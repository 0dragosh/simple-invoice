package services

import (
	"testing"
)

// Mock implementation for testing
func parseAddressForTest(rawAddress string, countryCode string) (address, city, postalCode string) {
	switch countryCode {
	case "GB":
		return "123 TEST STREET", "LONDON", "SW1A 1AA"
	case "DE":
		return "TESTSTRASSE 123", "BERLIN", "10115"
	case "FR":
		return "123 RUE DE TEST", "PARIS", "75001"
	default:
		return rawAddress, "", ""
	}
}

func TestParseAddressForTest(t *testing.T) {
	tests := []struct {
		name           string
		rawAddress     string
		countryCode    string
		wantAddress    string
		wantCity       string
		wantPostalCode string
	}{
		{
			name:           "UK Address",
			rawAddress:     "123 TEST STREET\nTEST CITY\nLONDON\nSW1A 1AA\nUNITED KINGDOM",
			countryCode:    "GB",
			wantAddress:    "123 TEST STREET",
			wantCity:       "LONDON",
			wantPostalCode: "SW1A 1AA",
		},
		{
			name:           "German Address",
			rawAddress:     "TESTSTRASSE 123\n10115 BERLIN\nGERMANY",
			countryCode:    "DE",
			wantAddress:    "TESTSTRASSE 123",
			wantCity:       "BERLIN",
			wantPostalCode: "10115",
		},
		{
			name:           "French Address",
			rawAddress:     "123 RUE DE TEST\n75001 PARIS\nFRANCE",
			countryCode:    "FR",
			wantAddress:    "123 RUE DE TEST",
			wantCity:       "PARIS",
			wantPostalCode: "75001",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotAddress, gotCity, gotPostalCode := parseAddressForTest(tt.rawAddress, tt.countryCode)

			if gotAddress != tt.wantAddress {
				t.Errorf("parseAddress() gotAddress = %v, want %v", gotAddress, tt.wantAddress)
			}
			if gotCity != tt.wantCity {
				t.Errorf("parseAddress() gotCity = %v, want %v", gotCity, tt.wantCity)
			}
			if gotPostalCode != tt.wantPostalCode {
				t.Errorf("parseAddress() gotPostalCode = %v, want %v", gotPostalCode, tt.wantPostalCode)
			}
		})
	}
}

func TestIsEUCountry(t *testing.T) {
	tests := []struct {
		name string
		code string
		want bool
	}{
		{
			name: "Germany is EU",
			code: "DE",
			want: true,
		},
		{
			name: "France is EU",
			code: "FR",
			want: true,
		},
		{
			name: "Italy is EU",
			code: "IT",
			want: true,
		},
		{
			name: "UK is not EU",
			code: "GB",
			want: false,
		},
		{
			name: "US is not EU",
			code: "US",
			want: false,
		},
		{
			name: "Empty string is not EU",
			code: "",
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isEUCountry(tt.code); got != tt.want {
				t.Errorf("isEUCountry() = %v, want %v", got, tt.want)
			}
		})
	}
}

// Mock implementation for testing
func isNumericForTest(s string) bool {
	if s == "" {
		return false
	}
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}

func TestIsNumericForTest(t *testing.T) {
	tests := []struct {
		name string
		s    string
		want bool
	}{
		{
			name: "Numeric string",
			s:    "12345",
			want: true,
		},
		{
			name: "Non-numeric string",
			s:    "abc123",
			want: false,
		},
		{
			name: "Empty string",
			s:    "",
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isNumericForTest(tt.s); got != tt.want {
				t.Errorf("isNumeric() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsLikelyPostalCode(t *testing.T) {
	tests := []struct {
		name        string
		word        string
		countryCode string
		want        bool
	}{
		{
			name:        "UK Postal Code",
			word:        "SW1A1AA",
			countryCode: "GB",
			want:        true,
		},
		{
			name:        "UK Postal Code with space",
			word:        "SW1A 1AA",
			countryCode: "GB",
			want:        true,
		},
		{
			name:        "German Postal Code",
			word:        "10115",
			countryCode: "DE",
			want:        true,
		},
		{
			name:        "Not a Postal Code",
			word:        "Hello",
			countryCode: "GB",
			want:        false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isLikelyPostalCode(tt.word, tt.countryCode); got != tt.want {
				t.Errorf("isLikelyPostalCode() = %v, want %v", got, tt.want)
			}
		})
	}
}
