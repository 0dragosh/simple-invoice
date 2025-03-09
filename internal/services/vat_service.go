package services

import (
	"encoding/json"
	"fmt"
	"io"
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
	logger               *Logger
}

// NewVatService creates a new VatService
func NewVatService(logger *Logger) *VatService {
	// Get API key from environment variable
	companiesHouseAPIKey := os.Getenv("COMPANIES_HOUSE_API_KEY")

	// Log the API key status (masked for security)
	if companiesHouseAPIKey != "" {
		maskedKey := ""
		if len(companiesHouseAPIKey) >= 8 {
			maskedKey = companiesHouseAPIKey[:4] + "..." + companiesHouseAPIKey[len(companiesHouseAPIKey)-4:]
		} else if len(companiesHouseAPIKey) > 0 {
			maskedKey = "[too short to mask]"
		}
		logger.Debug("Companies House API Key loaded: %s (length: %d)", maskedKey, len(companiesHouseAPIKey))
	} else {
		logger.Warn("Companies House API Key not set - UK company lookups will not work")
	}

	return &VatService{
		companiesHouseAPIKey: companiesHouseAPIKey,
		logger:               logger,
	}
}

// ValidateVatID validates a VAT ID and returns business information if available
func (s *VatService) ValidateVatID(vatID string) (*models.Client, error) {
	// Clean the VAT ID (remove spaces, make uppercase)
	vatID = strings.ToUpper(strings.ReplaceAll(vatID, " ", ""))

	s.logger.Debug("VAT Validation - Input: Original VAT ID = %s", vatID)

	// Check if the VAT ID is valid (should be at least 3 characters)
	if len(vatID) < 3 {
		return nil, fmt.Errorf("invalid VAT ID format")
	}

	// Extract the country code and number
	countryCode := vatID[:2]
	number := vatID[2:]

	s.logger.Debug("VAT Validation - Parsed: Country Code = %s, Number = %s", countryCode, number)

	// Validate based on country code
	if isEUCountry(countryCode) {
		s.logger.Info("Using EU VIES API for VAT validation")
		return s.fetchFromVIES(countryCode, number)
	} else if countryCode == "GB" {
		s.logger.Info("UK VAT validation requires manual entry - VAT ID cannot be automatically validated")
		// Return a special error for UK VAT IDs that can be handled differently
		return nil, fmt.Errorf("UK_VAT_MANUAL_ENTRY: UK VAT validation requires manual entry - please enter company details manually or use Companies House lookup")
	} else {
		return nil, fmt.Errorf("unsupported country code: %s", countryCode)
	}
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
		case "GB", "UK":
			// UK postal code pattern: 1-2 letters, 1-2 digits, optional space, 1 digit, 2 letters
			// Examples: SW1A 1AA, M1 1AA, B1 1AA, etc.
			re := regexp.MustCompile(`\b[A-Z]{1,2}[0-9][0-9A-Z]?\s*[0-9][A-Z]{2}\b`)
			if match := re.FindString(rawAddress); match != "" {
				postalCode = match
				// Normalize the postal code format (ensure there's a space in the right place)
				postalCode = normalizePostalCode(postalCode, countryCode)
			}

			// Try to extract city for UK addresses
			// Common UK cities
			ukCities := []string{
				"London", "Manchester", "Birmingham", "Liverpool", "Leeds", "Glasgow", "Edinburgh",
				"Bristol", "Sheffield", "Newcastle", "Nottingham", "Cardiff", "Belfast", "Leicester",
				"Coventry", "Bradford", "Stoke-on-Trent", "Wolverhampton", "Plymouth", "Derby",
				"Southampton", "Brighton", "Hull", "Reading", "Preston", "York", "Swansea",
				"Aberdeen", "Cambridge", "Exeter", "Oxford", "Sunderland", "Norwich", "Bath",
				"Portsmouth", "Bournemouth", "Middlesbrough", "Peterborough", "Blackpool",
				"Dundee", "Gloucester", "Huddersfield", "Ipswich", "Luton", "Northampton",
				"Poole", "Stockport", "Swindon", "Watford", "Wigan", "Blackburn", "Bolton",
				"Colchester", "Eastbourne", "Worthing", "Basingstoke", "Cheltenham", "Crawley",
				"Dudley", "Gillingham", "Hartlepool", "Rochdale", "Southport", "Woking",
				"Birkenhead", "Grimsby", "Hastings", "Maidstone", "Oldham", "Warrington",
				"Carlisle", "Darlington", "Guildford", "Harrogate", "Lincoln", "Stevenage",
				"Walsall", "Burnley", "Chatham", "Halifax", "Slough", "Southend-on-Sea",
				"Stockton-on-Tees", "Wakefield", "Chester", "Chesterfield", "Doncaster",
				"Mansfield", "Milton Keynes", "Rotherham", "Telford", "Weston-super-Mare",
				"Barnsley", "Bedford", "Harlow", "Hemel Hempstead", "Redditch", "Scarborough",
				"Scunthorpe", "Shrewsbury", "Weymouth", "Worcester", "Ashford", "Bognor Regis",
				"Canterbury", "Folkestone", "Hereford", "Kidderminster", "Leamington Spa",
				"Loughborough", "Nuneaton", "Rugby", "Stafford", "Taunton", "Torquay",
				"Wellingborough", "Bangor", "Barry", "Bridgend", "Caerphilly", "Llanelli",
				"Merthyr Tydfil", "Newport", "Pontypool", "Port Talbot", "Rhondda", "Wrexham",
				"Ayr", "Cumbernauld", "Dumfries", "East Kilbride", "Falkirk", "Greenock",
				"Hamilton", "Inverness", "Kilmarnock", "Kirkcaldy", "Livingston", "Motherwell",
				"Paisley", "Perth", "Stirling", "Armagh", "Bangor", "Coleraine", "Craigavon",
				"Derry", "Lisburn", "Newry", "Newtownabbey", "Omagh", "Didsbury",
			}

			// Check if any of the common UK cities are in the address
			for _, ukCity := range ukCities {
				if strings.Contains(rawAddress, ukCity) {
					city = ukCity
					break
				}
			}

			// If we found a city and postal code, remove them from the address
			if city != "" && postalCode != "" {
				// Remove the city and postal code from the address
				addressWithoutCity := strings.Replace(rawAddress, city, "", -1)
				addressWithoutPostal := strings.Replace(addressWithoutCity, postalCode, "", -1)

				// Clean up the address
				address = strings.TrimSpace(addressWithoutPostal)
				address = strings.Trim(address, ",")
				address = strings.TrimSpace(address)

				// Remove any trailing commas
				for strings.HasSuffix(address, ",") {
					address = strings.TrimSuffix(address, ",")
					address = strings.TrimSpace(address)
				}
			}
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

// fetchFromVIES fetches business information from the official VIES SOAP API
func (s *VatService) fetchFromVIES(countryCode, number string) (*models.Client, error) {
	// Construct the full VAT number
	fullVatNumber := countryCode + number

	// Use the official VIES SOAP API endpoint
	url := "https://ec.europa.eu/taxation_customs/vies/services/checkVatService"

	s.logger.Debug("VAT Validation - Query: Sending request to %s", url)
	s.logger.Debug("VAT Validation - Query: VAT ID = %s, Country Code = %s, Number = %s",
		fullVatNumber, countryCode, number)

	// Create the SOAP request body
	soapEnvelope := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<soapenv:Envelope xmlns:soapenv="http://schemas.xmlsoap.org/soap/envelope/" xmlns:urn="urn:ec.europa.eu:taxud:vies:services:checkVat:types">
   <soapenv:Header/>
   <soapenv:Body>
      <urn:checkVat>
         <urn:countryCode>%s</urn:countryCode>
         <urn:vatNumber>%s</urn:vatNumber>
      </urn:checkVat>
   </soapenv:Body>
</soapenv:Envelope>`, countryCode, number)

	// Create the request
	req, err := http.NewRequest("POST", url, strings.NewReader(soapEnvelope))
	if err != nil {
		s.logger.Error("Failed to create VIES API request: %v", err)
		return nil, err
	}

	// Set headers
	req.Header.Set("Content-Type", "text/xml;charset=UTF-8")
	req.Header.Set("SOAPAction", "")
	req.Header.Set("User-Agent", "SimpleInvoice/1.0.0 Go/1.20")

	s.logger.Debug("VAT Validation - Query: Sending request with headers: %v", req.Header)

	client := &http.Client{
		Timeout: 10 * time.Second,
	}

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

	// Parse the SOAP response
	responseStr := string(bodyBytes)

	// Check if the VAT number is valid
	valid := strings.Contains(responseStr, "<ns2:valid>true</ns2:valid>")

	// Extract the name
	var name, address string

	nameTag := "<ns2:name>"
	nameEndTag := "</ns2:name>"
	nameStart := strings.Index(responseStr, nameTag)
	nameEnd := strings.Index(responseStr, nameEndTag)
	if nameStart != -1 && nameEnd != -1 {
		name = responseStr[nameStart+len(nameTag) : nameEnd]
	}

	// Extract the address
	addressTag := "<ns2:address>"
	addressEndTag := "</ns2:address>"
	addressStart := strings.Index(responseStr, addressTag)
	addressEnd := strings.Index(responseStr, addressEndTag)
	if addressStart != -1 && addressEnd != -1 {
		address = responseStr[addressStart+len(addressTag) : addressEnd]
	}

	// Clean up XML entities
	name = strings.ReplaceAll(name, "&lt;", "<")
	name = strings.ReplaceAll(name, "&gt;", ">")
	name = strings.ReplaceAll(name, "&amp;", "&")
	name = strings.ReplaceAll(name, "&quot;", "\"")
	name = strings.ReplaceAll(name, "&apos;", "'")

	address = strings.ReplaceAll(address, "&lt;", "<")
	address = strings.ReplaceAll(address, "&gt;", ">")
	address = strings.ReplaceAll(address, "&amp;", "&")
	address = strings.ReplaceAll(address, "&quot;", "\"")
	address = strings.ReplaceAll(address, "&apos;", "'")

	s.logger.Debug("VAT Validation - Parsed Response: Valid = %t, Name = %s, Address = %s",
		valid, name, address)

	if !valid {
		s.logger.Error("Invalid VAT ID according to VIES API: %s", fullVatNumber)
		return nil, fmt.Errorf("invalid VAT ID")
	}

	s.logger.Info("Successfully validated VAT ID with VIES: %s", fullVatNumber)
	s.logger.Debug("VIES response: Name=%s, Address=%s", name, address)

	// Parse address based on country code
	parsedAddress, city, postalCode := parseAddressForCountry(address, countryCode)

	// Normalize postal code
	postalCode = normalizePostalCode(postalCode, countryCode)

	s.logger.Debug("VAT Validation - Parsed Address: Address = %s, City = %s, PostalCode = %s",
		parsedAddress, city, postalCode)

	return &models.Client{
		Name:       name,
		Address:    parsedAddress,
		City:       city,
		PostalCode: postalCode,
		Country:    countryCode,
		VatID:      fullVatNumber,
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

// normalizePostalCode formats a postal code according to country standards
func normalizePostalCode(postalCode string, countryCode string) string {
	// Remove all spaces first
	postalCode = strings.ReplaceAll(postalCode, " ", "")

	switch countryCode {
	case "GB", "UK":
		// Format UK postal codes as "XX XX" or "XXX XXX"
		// UK postal codes are in the format:
		// Area (1-2 characters) + District (1-2 characters) + Space + Sector (1 character) + Unit (2 characters)
		// Examples: SW1A 1AA, M1 1AA, B1 1AA, etc.
		if len(postalCode) >= 5 && len(postalCode) <= 7 {
			// Find the position to insert the space
			// The space always comes before the last 3 characters
			insertPos := len(postalCode) - 3
			return postalCode[:insertPos] + " " + postalCode[insertPos:]
		}
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

// LookupUKCompany looks up a UK company by name using the Companies House API
func (s *VatService) LookupUKCompany(name string) ([]*models.Client, error) {
	if s.companiesHouseAPIKey == "" {
		return nil, fmt.Errorf("Companies House API key not configured. Please set the COMPANIES_HOUSE_API_KEY environment variable")
	}

	// Use the Companies House API to search for companies
	apiURL := fmt.Sprintf("https://api.company-information.service.gov.uk/search/companies?q=%s", url.QueryEscape(name))

	s.logger.Debug("Companies House - Query: Sending request to %s", apiURL)
	s.logger.Debug("Companies House - Query: Company Name = %s", name)

	// Create the request
	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		s.logger.Error("Failed to create Companies House request: %v", err)
		return nil, err
	}

	// Set headers
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "SimpleInvoice/1.0.0 Go/1.20")

	// Set basic auth with API key
	req.SetBasicAuth(s.companiesHouseAPIKey, "")

	s.logger.Debug("Companies House - Query: Sending request with headers: %v", req.Header)

	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	resp, err := client.Do(req)
	if err != nil {
		s.logger.Error("Companies House request failed: %v", err)
		return nil, err
	}
	defer resp.Body.Close()

	// Read the body into a buffer so we can use it multiple times if needed
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		s.logger.Error("Failed to read Companies House response: %v", err)
		return nil, err
	}

	s.logger.Debug("Companies House - Response: Status code = %d", resp.StatusCode)
	s.logger.Debug("Companies House - Response: Headers = %v", resp.Header)
	s.logger.Debug("Companies House - Response: Body = %s", string(bodyBytes))

	// Check for error responses
	if resp.StatusCode != http.StatusOK {
		errMsg := fmt.Sprintf("Companies House API error: %s - %s", resp.Status, string(bodyBytes))
		s.logger.Error(errMsg)
		return nil, fmt.Errorf(errMsg)
	}

	// Parse the response
	var result struct {
		Items []struct {
			CompanyNumber  string `json:"company_number"`
			Title          string `json:"title"`
			AddressSnippet string `json:"address_snippet"`
			Kind           string `json:"company_type"`
		} `json:"items"`
	}

	if err := json.Unmarshal(bodyBytes, &result); err != nil {
		s.logger.Error("Failed to decode Companies House response: %v", err)
		return nil, err
	}

	// Convert the results to clients
	clients := make([]*models.Client, 0, len(result.Items))
	for _, item := range result.Items {
		// Parse the address to extract city and postal code
		address, city, postalCode := parseAddressForCountry(item.AddressSnippet, "GB")

		client := &models.Client{
			Name:       item.Title,
			Address:    address,
			City:       city,
			PostalCode: postalCode,
			Country:    "GB",
			// Note: VAT ID needs to be entered manually
		}

		clients = append(clients, client)
	}

	s.logger.Info("Successfully found %d UK companies matching '%s'", len(clients), name)
	return clients, nil
}

// LookupUKCompanyByNumber looks up a UK company by company number using the Companies House API
func (s *VatService) LookupUKCompanyByNumber(number string) (*models.Client, error) {
	if s.companiesHouseAPIKey == "" {
		return nil, fmt.Errorf("Companies House API key not configured. Please set the COMPANIES_HOUSE_API_KEY environment variable")
	}

	// Use the Companies House API to get company details
	apiURL := fmt.Sprintf("https://api.company-information.service.gov.uk/company/%s", url.QueryEscape(number))

	s.logger.Debug("Companies House - Query: Sending request to %s", apiURL)
	s.logger.Debug("Companies House - Query: Company Number = %s", number)

	// Create the request
	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		s.logger.Error("Failed to create Companies House request: %v", err)
		return nil, err
	}

	// Set headers
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "SimpleInvoice/1.0.0 Go/1.20")

	// Set basic auth with API key
	req.SetBasicAuth(s.companiesHouseAPIKey, "")

	s.logger.Debug("Companies House - Query: Sending request with headers: %v", req.Header)

	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	resp, err := client.Do(req)
	if err != nil {
		s.logger.Error("Companies House request failed: %v", err)
		return nil, err
	}
	defer resp.Body.Close()

	// Read the body into a buffer so we can use it multiple times if needed
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		s.logger.Error("Failed to read Companies House response: %v", err)
		return nil, err
	}

	s.logger.Debug("Companies House - Response: Status code = %d", resp.StatusCode)
	s.logger.Debug("Companies House - Response: Headers = %v", resp.Header)
	s.logger.Debug("Companies House - Response: Body = %s", string(bodyBytes))

	// Check for error responses
	if resp.StatusCode != http.StatusOK {
		errMsg := fmt.Sprintf("Companies House API error: %s - %s", resp.Status, string(bodyBytes))
		s.logger.Error(errMsg)
		return nil, fmt.Errorf(errMsg)
	}

	// Parse the response
	var result struct {
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

	if err := json.Unmarshal(bodyBytes, &result); err != nil {
		s.logger.Error("Failed to decode Companies House response: %v", err)
		return nil, err
	}

	// Build the address
	addressParts := []string{}
	if result.RegisteredOfficeAddress.AddressLine1 != "" {
		addressParts = append(addressParts, result.RegisteredOfficeAddress.AddressLine1)
	}
	if result.RegisteredOfficeAddress.AddressLine2 != "" {
		addressParts = append(addressParts, result.RegisteredOfficeAddress.AddressLine2)
	}

	address := strings.Join(addressParts, ", ")
	city := result.RegisteredOfficeAddress.Locality
	postalCode := result.RegisteredOfficeAddress.PostalCode
	country := "GB"
	if result.RegisteredOfficeAddress.Country != "" {
		country = result.RegisteredOfficeAddress.Country
	}

	s.logger.Info("Successfully found UK company with number '%s'", number)

	return &models.Client{
		Name:       result.CompanyName,
		Address:    address,
		City:       city,
		PostalCode: postalCode,
		Country:    country,
		// Note: VAT ID needs to be entered manually
	}, nil
}
