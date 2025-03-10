package services

import (
	"time"
)

// CalculateWorkHoursForMonth calculates the total work hours for a given month
// excluding weekends (8 hours per day, 5 days per week)
func CalculateWorkHoursForMonth(year int, month time.Month) float64 {
	// Get the first day of the month
	firstDay := time.Date(year, month, 1, 0, 0, 0, 0, time.UTC)

	// Get the last day of the month
	lastDay := firstDay.AddDate(0, 1, -1)

	var totalHours float64 = 0

	// Iterate through each day of the month
	for day := firstDay; day.Before(lastDay.AddDate(0, 0, 1)); day = day.AddDate(0, 0, 1) {
		// Check if the day is a weekday (Monday to Friday)
		if day.Weekday() != time.Saturday && day.Weekday() != time.Sunday {
			totalHours += 8.0 // 8 hours per workday
		}
	}

	return totalHours
}

// CalculateWorkHoursForCurrentMonth calculates the total work hours for the current month
// excluding weekends (8 hours per day, 5 days per week)
func CalculateWorkHoursForCurrentMonth() float64 {
	now := time.Now()
	return CalculateWorkHoursForMonth(now.Year(), now.Month())
}

// GetCurrencyForCountry returns the currency code for a given country code
// For EU countries, it returns the appropriate currency (EUR for Eurozone, or local currency for non-Eurozone)
func GetCurrencyForCountry(countryCode string) string {
	// Default currency
	defaultCurrency := "EUR"

	// Map of EU countries to their currencies
	euCurrencies := map[string]string{
		// Eurozone countries (using EUR)
		"AT": "EUR", // Austria
		"BE": "EUR", // Belgium
		"CY": "EUR", // Cyprus
		"EE": "EUR", // Estonia
		"FI": "EUR", // Finland
		"FR": "EUR", // France
		"DE": "EUR", // Germany
		"GR": "EUR", // Greece
		"IE": "EUR", // Ireland
		"IT": "EUR", // Italy
		"LV": "EUR", // Latvia
		"LT": "EUR", // Lithuania
		"LU": "EUR", // Luxembourg
		"MT": "EUR", // Malta
		"NL": "EUR", // Netherlands
		"PT": "EUR", // Portugal
		"SK": "EUR", // Slovakia
		"SI": "EUR", // Slovenia
		"ES": "EUR", // Spain

		// Non-Eurozone EU countries
		"BG": "BGN", // Bulgaria - Bulgarian Lev
		"HR": "HRK", // Croatia - Croatian Kuna (Note: Croatia adopted EUR in 2023, but keeping HRK for backward compatibility)
		"CZ": "CZK", // Czech Republic - Czech Koruna
		"DK": "DKK", // Denmark - Danish Krone
		"HU": "HUF", // Hungary - Hungarian Forint
		"PL": "PLN", // Poland - Polish Złoty
		"RO": "RON", // Romania - Romanian Leu
		"SE": "SEK", // Sweden - Swedish Krona

		// Former EU member
		"GB": "GBP", // United Kingdom - British Pound
	}

	// Return the currency for the country code, or the default if not found
	if currency, exists := euCurrencies[countryCode]; exists {
		return currency
	}

	return defaultCurrency
}

// FormatCurrencySymbol returns the appropriate currency symbol for a given currency code
func FormatCurrencySymbol(currencyCode string) string {
	currencySymbols := map[string]string{
		"EUR": "EUR", // Using "EUR" instead of "€"
		"GBP": "GBP", // Using "GBP" instead of "£"
		"BGN": "BGN", // Using "BGN" instead of "лв"
		"HRK": "HRK", // Using "HRK" instead of "kn"
		"CZK": "CZK", // Using "CZK" instead of "Kč"
		"DKK": "DKK", // Using "DKK" instead of "kr"
		"HUF": "HUF", // Using "HUF" instead of "Ft"
		"PLN": "PLN", // Using "PLN" instead of "zł"
		"RON": "RON", // Using "RON" instead of "lei"
		"SEK": "SEK", // Using "SEK" instead of "kr"
		"USD": "USD", // Using "USD" instead of "$"
	}

	if symbol, exists := currencySymbols[currencyCode]; exists {
		return symbol
	}

	// Return the currency code if no symbol is found
	return currencyCode
}
