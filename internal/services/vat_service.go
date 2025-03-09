package services

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/0dragosh/simple-invoice/internal/models"
)

// VatService provides methods for VAT ID validation and business info retrieval
type VatService struct {
	companiesHouseAPIKey string
	viesIdentifier       string
	viesKey              string
	logger               *Logger
}

// NewVatService creates a new VAT service
func NewVatService(logger *Logger) *VatService {
	// Get API keys from environment variables
	companiesHouseAPIKey := os.Getenv("COMPANIES_HOUSE_API_KEY")
	viesIdentifier := os.Getenv("VIES_IDENTIFIER")
	viesKey := os.Getenv("VIES_KEY")

	return &VatService{
		companiesHouseAPIKey: companiesHouseAPIKey,
		viesIdentifier:       viesIdentifier,
		viesKey:              viesKey,
		logger:               logger,
	}
}

// ValidateVatID validates a VAT ID and returns business information if available
func (s *VatService) ValidateVatID(vatID string) (*models.Client, error) {
	// Clean the VAT ID (remove spaces, make uppercase)
	vatID = strings.ToUpper(strings.ReplaceAll(vatID, " ", ""))

	s.logger.Info("Starting VAT ID validation for: %s", vatID)
	s.logger.Debug("VAT Validation - Input: Original VAT ID = %s", vatID)

	// Extract country code and number
	if len(vatID) < 3 {
		s.logger.Error("VAT Validation - Error: Invalid VAT ID format (too short)")
		return nil, fmt.Errorf("invalid VAT ID format")
	}

	countryCode := vatID[:2]
	number := vatID[2:]

	s.logger.Debug("VAT Validation - Parsed: Country Code = %s, Number = %s", countryCode, number)

	// Only use VIES API for validation - no fallbacks
	if s.viesIdentifier != "" && s.viesKey != "" {
		s.logger.Info("Using VIES API for VAT validation")
		s.logger.Debug("VAT Validation - Config: Using VIES API with identifier = %s", s.viesIdentifier)

		client, err := s.fetchFromVIES(countryCode, number)
		if err != nil {
			s.logger.Error("VIES API validation failed: %v", err)
			s.logger.Debug("VAT Validation - Error: Full error details = %v", err)
			return nil, fmt.Errorf("VAT validation failed: %w", err)
		}

		s.logger.Debug("VAT Validation - Success: Validated VAT ID %s for %s", vatID, client.Name)
		return client, nil
	}

	// If no VIES credentials, return an error
	s.logger.Error("VAT Validation - Error: VIES API credentials not configured")
	return nil, fmt.Errorf("VIES API credentials not configured")
}

// parseAddress parses a raw address string into address, city, and postal code
func parseAddress(rawAddress string, countryCode string) (address, city, postalCode string) {
	// Split the address into lines
	lines := strings.Split(rawAddress, "\n")

	// Clean up the lines
	for i, line := range lines {
		lines[i] = strings.TrimSpace(line)
	}

	// Remove empty lines
	var nonEmptyLines []string
	for _, line := range lines {
		if line != "" {
			nonEmptyLines = append(nonEmptyLines, line)
		}
	}
	lines = nonEmptyLines

	// If there are no lines, return empty strings
	if len(lines) == 0 {
		return "", "", ""
	}

	// If there's only one line, it's probably the address
	if len(lines) == 1 {
		return lines[0], "", ""
	}

	// The last line often contains the city and postal code
	lastLine := lines[len(lines)-1]

	// Try to extract city and postal code from the last line
	extractCityAndPostalCode(lastLine, countryCode, &city, &postalCode)

	// If we found a city or postal code, remove the last line from the address
	if city != "" || postalCode != "" {
		lines = lines[:len(lines)-1]
	}

	// If there are still lines left, join them to form the address
	address = strings.Join(lines, ", ")

	return address, city, postalCode
}

// extractCityAndPostalCode attempts to extract city and postal code from a string
func extractCityAndPostalCode(line string, countryCode string, city, postalCode *string) {
	// Common patterns:
	// "12345 City Name"
	// "City Name 12345"
	// "12345-City Name"

	line = strings.TrimSpace(line)

	// Try to find postal code at the beginning
	if postalCodePattern := getPostalCodePattern(countryCode); postalCodePattern != nil {
		if matches := postalCodePattern.FindStringSubmatch(line); len(matches) > 1 {
			*postalCode = matches[1]
			*city = strings.TrimSpace(line[len(matches[0]):])
			return
		}
	}

	// Try to find postal code at the end
	words := strings.Fields(line)
	if len(words) > 1 {
		lastWord := words[len(words)-1]
		if isLikelyPostalCode(lastWord, countryCode) {
			*postalCode = lastWord
			*city = strings.TrimSpace(strings.Join(words[:len(words)-1], " "))
			return
		}

		// Try second to last word (for cases like "City 12345 Country")
		if len(words) > 2 {
			secondLastWord := words[len(words)-2]
			if isLikelyPostalCode(secondLastWord, countryCode) {
				*postalCode = secondLastWord
				*city = strings.TrimSpace(strings.Join(append(words[:len(words)-2], words[len(words)-1]), " "))
				return
			}
		}
	}

	// If we couldn't extract a postal code, just use the whole line as the city
	*city = line
}

// extractPostalCode attempts to extract a postal code from a string
func extractPostalCode(line string, countryCode string, postalCode *string) {
	line = strings.TrimSpace(line)

	// Try to find postal code using regex pattern
	if postalCodePattern := getPostalCodePattern(countryCode); postalCodePattern != nil {
		if matches := postalCodePattern.FindStringSubmatch(line); len(matches) > 1 {
			*postalCode = matches[1]
			return
		}
	}

	// Try to find postal code by looking for digits
	words := strings.Fields(line)
	for _, word := range words {
		if isLikelyPostalCode(word, countryCode) {
			*postalCode = word
			return
		}
	}
}

// containsPostalCode checks if a string likely contains a postal code
func containsPostalCode(line string, countryCode string) bool {
	// Check using regex pattern
	if postalCodePattern := getPostalCodePattern(countryCode); postalCodePattern != nil {
		return postalCodePattern.MatchString(line)
	}

	// Check for digits as fallback
	words := strings.Fields(line)
	for _, word := range words {
		if isLikelyPostalCode(word, countryCode) {
			return true
		}
	}

	return false
}

// isLikelyPostalCode checks if a string is likely a postal code
func isLikelyPostalCode(word string, countryCode string) bool {
	// Most postal codes contain digits
	hasDigit := false
	for _, c := range word {
		if c >= '0' && c <= '9' {
			hasDigit = true
			break
		}
	}

	// Country-specific checks
	switch countryCode {
	case "CZ", "SE":
		// Czech and Swedish postal codes: 3 digits + 2 digits, possibly with space
		// Check for formats like "123 45" or "12345"
		if len(word) >= 5 && len(word) <= 6 && isNumeric(strings.ReplaceAll(word, " ", "")) {
			return true
		}
	case "PL":
		// Polish postal codes: 2 digits + dash + 3 digits
		// Check for format like "12-345"
		if len(word) == 6 && word[2] == '-' &&
			isNumeric(word[:2]) && isNumeric(word[3:]) {
			return true
		}
	case "RO":
		// Romanian postal codes are 6 digits
		if len(word) == 6 && isNumeric(word) {
			return true
		}
	case "DK", "NO":
		// Danish and Norwegian postal codes are 4 digits
		if len(word) == 4 && isNumeric(word) {
			return true
		}
	case "GB":
		// UK postal codes have specific formats like "AB12 3CD"
		if len(word) >= 5 && len(word) <= 8 && strings.ContainsAny(word, "0123456789") {
			return true
		}
	default:
		// Generic check: if it has digits and is reasonably sized
		return hasDigit && len(word) >= 4 && len(word) <= 10
	}

	return false
}

// isNumeric checks if a string contains only digits
func isNumeric(s string) bool {
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}

// getPostalCodePattern returns a regex pattern for postal codes based on country
func getPostalCodePattern(countryCode string) *regexp.Regexp {
	patterns := map[string]*regexp.Regexp{
		// Updated patterns for the requested countries
		"CZ": regexp.MustCompile(`(\d{3}\s*\d{2})`), // Czech postal codes: 3 digits, space, 2 digits (e.g., 123 45)
		"PL": regexp.MustCompile(`(\d{2}-\d{3})`),   // Polish postal codes: 2 digits, dash, 3 digits (e.g., 12-345)
		"RO": regexp.MustCompile(`(\d{6})`),         // Romanian postal codes: 6 digits (e.g., 123456)
		"DK": regexp.MustCompile(`(\d{4})`),         // Danish postal codes: 4 digits (e.g., 1234)
		"SE": regexp.MustCompile(`(\d{3}\s*\d{2})`), // Swedish postal codes: 3 digits, space, 2 digits (e.g., 123 45)
		"NO": regexp.MustCompile(`(\d{4})`),         // Norwegian postal codes: 4 digits (e.g., 1234)

		// Existing patterns
		"GB": regexp.MustCompile(`([A-Z]{1,2}[0-9][A-Z0-9]? ?[0-9][A-Z]{2})`), // UK postal codes
		"DE": regexp.MustCompile(`(\d{5})`),                                   // German postal codes: 5 digits
		"FR": regexp.MustCompile(`(\d{5})`),                                   // French postal codes: 5 digits
		"IT": regexp.MustCompile(`(\d{5})`),                                   // Italian postal codes: 5 digits
		"ES": regexp.MustCompile(`(\d{5})`),                                   // Spanish postal codes: 5 digits
		"NL": regexp.MustCompile(`(\d{4} ?[A-Z]{2})`),                         // Dutch postal codes: 4 digits + 2 letters
		"BE": regexp.MustCompile(`(\d{4})`),                                   // Belgian postal codes: 4 digits
		"AT": regexp.MustCompile(`(\d{4})`),                                   // Austrian postal codes: 4 digits
	}

	if pattern, exists := patterns[countryCode]; exists {
		return pattern
	}

	// Generic pattern for postal codes: 4-10 characters with at least one digit
	return regexp.MustCompile(`(\b[A-Z0-9]{4,10}\b)`)
}

// parseAddressForCountry parses an address based on country-specific formats
func parseAddressForCountry(rawAddress string, countryCode string) (address, city, postalCode string) {
	// Default to generic parsing
	address, city, postalCode = parseAddress(rawAddress, countryCode)

	// If we couldn't extract a postal code, try country-specific extraction
	if postalCode == "" {
		switch countryCode {
		case "CZ", "SE":
			// Look for patterns like "123 45" or "12345"
			re := regexp.MustCompile(`\b\d{3}\s*\d{2}\b`)
			if match := re.FindString(rawAddress); match != "" {
				postalCode = match
			}
		case "PL":
			// Look for patterns like "12-345"
			re := regexp.MustCompile(`\b\d{2}-\d{3}\b`)
			if match := re.FindString(rawAddress); match != "" {
				postalCode = match
			}
		case "RO":
			// Look for 6 consecutive digits
			re := regexp.MustCompile(`\b\d{6}\b`)
			if match := re.FindString(rawAddress); match != "" {
				postalCode = match
			}
		case "DK", "NO":
			// Look for 4 consecutive digits
			re := regexp.MustCompile(`\b\d{4}\b`)
			if match := re.FindString(rawAddress); match != "" {
				postalCode = match
			}
		}
	}

	return address, city, postalCode
}

// fetchFromVIES fetches business information from the VIES API
func (s *VatService) fetchFromVIES(countryCode, number string) (*models.Client, error) {
	// Check if we have VIES API credentials
	if s.viesIdentifier == "" || s.viesKey == "" {
		return nil, fmt.Errorf("VIES API credentials not configured")
	}

	// Construct the full VAT number
	fullVatNumber := countryCode + number

	// Use the correct API endpoint according to the documentation
	path := fmt.Sprintf("/api/get/vies/euvat/%s", fullVatNumber)
	url := fmt.Sprintf("https://viesapi.eu%s", path)

	s.logger.Debug("VAT Validation - Query: Sending request to %s", url)
	s.logger.Debug("VAT Validation - Query: VAT ID = %s, Country Code = %s, Number = %s",
		fullVatNumber, countryCode, number)

	// Create the request
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		s.logger.Error("Failed to create VIES API request: %v", err)
		return nil, err
	}

	// Calculate timestamp and nonce for MAC authentication
	timestamp := fmt.Sprintf("%d", time.Now().Unix())
	nonce := generateRandomString(12)

	// Calculate MAC value
	macValue := calculateMAC(timestamp, nonce, "GET", path, "viesapi.eu", "443", s.viesKey)

	// Set MAC authentication header
	authHeader := fmt.Sprintf(`MAC id="%s", ts="%s", nonce="%s", mac="%s"`,
		s.viesIdentifier, timestamp, nonce, macValue)
	req.Header.Set("Authorization", authHeader)

	s.logger.Debug("VAT Validation - Query: Authorization header set with timestamp=%s, nonce=%s",
		timestamp, nonce)

	// Set User-Agent as recommended in docs
	req.Header.Set("User-Agent", "SimpleInvoice/1.0.0 Go/1.20")
	req.Header.Set("Accept", "application/json")

	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	s.logger.Debug("VAT Validation - Query: Sending request with headers: %v", req.Header)

	resp, err := client.Do(req)
	if err != nil {
		s.logger.Error("VIES API request failed: %v", err)
		return nil, err
	}
	defer resp.Body.Close()

	// Read the body into a buffer so we can use it multiple times if needed
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		s.logger.Error("Failed to read VIES API response: %v", err)
		return nil, err
	}

	s.logger.Debug("VAT Validation - Response: Status code = %d", resp.StatusCode)
	s.logger.Debug("VAT Validation - Response: Headers = %v", resp.Header)
	s.logger.Debug("VAT Validation - Response: Body = %s", string(bodyBytes))

	if resp.StatusCode != http.StatusOK {
		errMsg := fmt.Sprintf("VIES API error: %s - %s", resp.Status, string(bodyBytes))
		s.logger.Error(errMsg)
		return nil, fmt.Errorf(errMsg)
	}

	// Parse VIES API response according to the documented structure
	var result struct {
		Uid               string `json:"uid"`
		CountryCode       string `json:"countryCode"`
		VatNumber         string `json:"vatNumber"`
		Valid             bool   `json:"valid"`
		TraderName        string `json:"traderName"`
		TraderCompanyType string `json:"traderCompanyType"`
		TraderAddress     string `json:"traderAddress"`
		ID                string `json:"id"`
		Date              string `json:"date"`
		Source            string `json:"source"`
	}

	if err := json.Unmarshal(bodyBytes, &result); err != nil {
		s.logger.Error("Failed to decode VIES API response: %v", err)
		return nil, err
	}

	s.logger.Debug("VAT Validation - Parsed Response: Valid = %t, Name = %s, Address = %s",
		result.Valid, result.TraderName, result.TraderAddress)

	if !result.Valid {
		s.logger.Error("Invalid VAT ID according to VIES API: %s", fullVatNumber)
		return nil, fmt.Errorf("invalid VAT ID")
	}

	s.logger.Info("Successfully validated VAT ID with VIES: %s", fullVatNumber)
	s.logger.Debug("VIES response: Name=%s, Address=%s", result.TraderName, result.TraderAddress)

	// Parse address based on country code
	address, city, postalCode := parseAddressForCountry(result.TraderAddress, countryCode)

	// Normalize postal code
	postalCode = normalizePostalCode(postalCode, countryCode)

	s.logger.Debug("VAT Validation - Parsed Address: Address = %s, City = %s, PostalCode = %s",
		address, city, postalCode)

	return &models.Client{
		Name:       result.TraderName,
		Address:    address,
		City:       city,
		PostalCode: postalCode,
		Country:    countryCode,
		VatID:      fullVatNumber,
	}, nil
}

// fetchFromUKVAT fetches business information from the UK VAT API
func (s *VatService) fetchFromUKVAT(number string) (*models.Client, error) {
	// Use the UK government's Check VAT API
	apiURL := "https://api.service.hmrc.gov.uk/organisations/vat/check-vat-number/lookup"

	// Create request body
	data := url.Values{}
	data.Set("target", number)

	// Create request
	req, err := http.NewRequest("POST", apiURL, strings.NewReader(data.Encode()))
	if err != nil {
		return nil, err
	}

	// Set headers
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	// Send request
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	// Read the body into a buffer so we can use it multiple times if needed
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		s.logger.Error("Failed to read UK VAT API response: %v", err)
		return nil, err
	}

	// Check response status
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API error: %s - %s", resp.Status, string(bodyBytes))
	}

	// Parse response
	var result struct {
		Target  string `json:"target"`
		Name    string `json:"name"`
		Address struct {
			Line1       string `json:"line1"`
			Line2       string `json:"line2"`
			Line3       string `json:"line3"`
			Line4       string `json:"line4"`
			PostalCode  string `json:"postalCode"`
			CountryCode string `json:"countryCode"`
		} `json:"address"`
	}

	if err := json.Unmarshal(bodyBytes, &result); err != nil {
		s.logger.Error("Failed to decode UK VAT API response: %v", err)
		return nil, err
	}

	if result.Target != number {
		s.logger.Error("Invalid VAT ID according to UK VAT API: %s", number)
		return nil, fmt.Errorf("invalid VAT ID")
	}

	s.logger.Info("Successfully validated VAT ID with UK VAT API: %s", number)
	s.logger.Debug("UK VAT API response: Name=%s, Address=%s", result.Name, result.Address.Line1)

	// Parse address based on country code
	var address, city, postalCode string

	if result.Address.CountryCode == "GB" {
		// Use specialized UK address parser
		address, city, postalCode = parseAddress(result.Address.Line1, result.Address.CountryCode)
	} else {
		// Use generic address parser for other countries
		address, city, postalCode = parseAddress(result.Address.Line1, result.Address.CountryCode)
	}

	// Normalize postal code
	postalCode = normalizePostalCode(postalCode, result.Address.CountryCode)

	return &models.Client{
		Name:       result.Name,
		Address:    address,
		City:       city,
		PostalCode: postalCode,
		Country:    result.Address.CountryCode,
		VatID:      result.Address.CountryCode + number,
	}, nil
}

// LookupUKCompany looks up a UK company by name or number
func (s *VatService) LookupUKCompany(query string) (*models.Client, error) {
	// Check if Companies House API key is available
	if s.companiesHouseAPIKey == "" {
		return nil, fmt.Errorf("Companies House API key not configured. Please set the COMPANIES_HOUSE_API_KEY environment variable")
	}

	// Check if the query is a company number (alphanumeric, typically 8 characters)
	isCompanyNumber := regexp.MustCompile(`^[A-Z0-9]{6,8}$`).MatchString(query)

	var companyNumber string

	if isCompanyNumber {
		// If it's a company number, use it directly
		companyNumber = query
		s.logger.Info("Looking up UK company by number: %s", companyNumber)
	} else {
		// Otherwise, search for the company by name
		s.logger.Info("Searching for UK company by name: %s", query)

		// Use the Companies House API to search for a company
		apiURL := fmt.Sprintf("https://api.companieshouse.gov.uk/search/companies?q=%s", url.QueryEscape(query))

		// Create request
		req, err := http.NewRequest("GET", apiURL, nil)
		if err != nil {
			return nil, err
		}

		// Set headers - Companies House API requires Basic Auth with API key as username and empty password
		req.SetBasicAuth(s.companiesHouseAPIKey, "")

		// Send request
		client := &http.Client{}
		resp, err := client.Do(req)
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()

		// Check response status
		if resp.StatusCode != http.StatusOK {
			bodyBytes, _ := io.ReadAll(resp.Body)
			return nil, fmt.Errorf("API error: %s - %s", resp.Status, string(bodyBytes))
		}

		// Parse response
		var result struct {
			Items []struct {
				CompanyNumber  string `json:"company_number"`
				Title          string `json:"title"`
				AddressSnippet string `json:"address_snippet"`
				CompanyStatus  string `json:"company_status"`
			} `json:"items"`
		}

		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			return nil, err
		}

		if len(result.Items) == 0 {
			return nil, fmt.Errorf("no companies found with name: %s", query)
		}

		// Filter for active companies
		var activeCompanies []struct {
			CompanyNumber  string `json:"company_number"`
			Title          string `json:"title"`
			AddressSnippet string `json:"address_snippet"`
			CompanyStatus  string `json:"company_status"`
		}

		for _, company := range result.Items {
			if company.CompanyStatus == "active" {
				activeCompanies = append(activeCompanies, company)
			}
		}

		if len(activeCompanies) == 0 {
			// If no active companies, use the first result
			companyNumber = result.Items[0].CompanyNumber
			s.logger.Warn("No active companies found, using first result: %s (%s)", result.Items[0].Title, companyNumber)
		} else {
			// Find the best match by comparing the name
			bestMatch := activeCompanies[0]
			bestScore := 0

			for _, company := range activeCompanies {
				// Simple scoring based on substring match
				score := 0
				if strings.Contains(strings.ToLower(company.Title), strings.ToLower(query)) {
					score += 5
				}

				// Exact word matches
				queryWords := strings.Fields(strings.ToLower(query))
				titleWords := strings.Fields(strings.ToLower(company.Title))

				for _, qw := range queryWords {
					for _, tw := range titleWords {
						if qw == tw {
							score += 3
						}
					}
				}

				if score > bestScore {
					bestScore = score
					bestMatch = company
				}
			}

			companyNumber = bestMatch.CompanyNumber
			s.logger.Info("Selected company: %s (%s)", bestMatch.Title, companyNumber)
		}
	}

	// Now get the company details
	detailsURL := fmt.Sprintf("https://api.companieshouse.gov.uk/company/%s", companyNumber)

	// Create request
	req, err := http.NewRequest("GET", detailsURL, nil)
	if err != nil {
		return nil, err
	}

	// Set headers
	req.SetBasicAuth(s.companiesHouseAPIKey, "")

	// Send request
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	// Check response status
	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API error: %s - %s", resp.Status, string(bodyBytes))
	}

	// Parse response
	var details struct {
		CompanyName             string `json:"company_name"`
		CompanyNumber           string `json:"company_number"`
		RegisteredOfficeAddress struct {
			AddressLine1 string `json:"address_line_1"`
			AddressLine2 string `json:"address_line_2"`
			Locality     string `json:"locality"`
			PostalCode   string `json:"postal_code"`
			Country      string `json:"country"`
		} `json:"registered_office_address"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&details); err != nil {
		return nil, err
	}

	// Build full address string
	addressParts := []string{}
	if details.RegisteredOfficeAddress.AddressLine1 != "" {
		addressParts = append(addressParts, details.RegisteredOfficeAddress.AddressLine1)
	}
	if details.RegisteredOfficeAddress.AddressLine2 != "" {
		addressParts = append(addressParts, details.RegisteredOfficeAddress.AddressLine2)
	}

	fullAddress := strings.Join(addressParts, ", ")

	// For UK addresses, we already have structured data, so we can use it directly
	// But we'll still use our parser for consistency if the locality or postal code is missing
	city := ""
	if details.RegisteredOfficeAddress.Locality != "" {
		city = details.RegisteredOfficeAddress.Locality
	}

	postalCode := details.RegisteredOfficeAddress.PostalCode

	// If city is still empty, try to extract it from the address
	if city == "" || postalCode == "" {
		_, extractedCity, extractedPostalCode := parseAddress(fullAddress, "GB")

		if city == "" {
			city = extractedCity
		}

		if postalCode == "" {
			postalCode = extractedPostalCode
		}
	}

	// Normalize postal code
	postalCode = normalizePostalCode(postalCode, "GB")

	// Use a placeholder for VAT ID - it needs to be filled in manually
	vatID := "GB" // Placeholder, would need to be filled in manually
	s.logger.Info("Using placeholder VAT ID for %s, it needs to be filled in manually", details.CompanyName)

	return &models.Client{
		Name:          details.CompanyName,
		Address:       fullAddress,
		City:          city,
		PostalCode:    postalCode,
		Country:       "GB",
		VatID:         vatID,
		CompanyNumber: companyNumber,
	}, nil
}

// isEUCountry checks if a country code is an EU member state
func isEUCountry(code string) bool {
	euCountries := map[string]bool{
		"AT": true, // Austria
		"BE": true, // Belgium
		"BG": true, // Bulgaria
		"HR": true, // Croatia
		"CY": true, // Cyprus
		"CZ": true, // Czech Republic
		"DK": true, // Denmark
		"EE": true, // Estonia
		"FI": true, // Finland
		"FR": true, // France
		"DE": true, // Germany
		"GR": true, // Greece
		"HU": true, // Hungary
		"IE": true, // Ireland
		"IT": true, // Italy
		"LV": true, // Latvia
		"LT": true, // Lithuania
		"LU": true, // Luxembourg
		"MT": true, // Malta
		"NL": true, // Netherlands
		"PL": true, // Poland
		"PT": true, // Portugal
		"RO": true, // Romania
		"SK": true, // Slovakia
		"SI": true, // Slovenia
		"ES": true, // Spain
		"SE": true, // Sweden
	}

	return euCountries[code]
}

// Add these helper functions for MAC authentication

// generateRandomString generates a random string of specified length
func generateRandomString(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, length)
	for i := range b {
		b[i] = charset[rand.Intn(len(charset))]
	}
	return string(b)
}

// calculateMAC calculates the HMAC-SHA256 for VIES API authentication
func calculateMAC(timestamp, nonce, method, path, host, port, key string) string {
	// Create the input string according to the documentation
	input := fmt.Sprintf("%s\n%s\n%s\n%s\n%s\n%s\n\n",
		timestamp, nonce, method, path, host, port)

	// Calculate HMAC-SHA256
	h := hmac.New(sha256.New, []byte(key))
	h.Write([]byte(input))

	// Return Base64 encoded result
	return base64.StdEncoding.EncodeToString(h.Sum(nil))
}

// normalizePostalCode formats a postal code according to country standards
func normalizePostalCode(postalCode string, countryCode string) string {
	// Remove all spaces first
	postalCode = strings.ReplaceAll(postalCode, " ", "")

	switch countryCode {
	case "CZ", "SE":
		// Format as "XXX XX"
		if len(postalCode) == 5 {
			return postalCode[:3] + " " + postalCode[3:]
		}
	case "PL":
		// Format as "XX-XXX"
		if len(postalCode) == 5 && !strings.Contains(postalCode, "-") {
			return postalCode[:2] + "-" + postalCode[2:]
		}
	}

	return postalCode
}
