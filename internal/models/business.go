package models

// Business represents the consultant's business details
type Business struct {
	ID                  int    `json:"id"`
	Name                string `json:"name"`
	Address             string `json:"address"`
	City                string `json:"city"`
	PostalCode          string `json:"postal_code"`
	Country             string `json:"country"`
	VatID               string `json:"vat_id"`
	Email               string `json:"email"`
	BankName            string `json:"bank_name"`
	BankAccount         string `json:"bank_account"`
	IBAN                string `json:"iban"`
	BIC                 string `json:"bic"`
	Currency            string `json:"currency"`
	SecondBankName      string `json:"second_bank_name"`
	SecondIBAN          string `json:"second_iban"`
	SecondBIC           string `json:"second_bic"`
	SecondCurrency      string `json:"second_currency"`
	ExtraBusinessDetail string `json:"extra_business_detail"`
	LogoPath            string `json:"logo_path"`
	LogoURL             string `json:"logo_url"` // URL to display the logo, without the /app prefix
}

// GetLogoURL returns the correct URL to display the logo
func (b *Business) GetLogoURL() string {
	if b.LogoPath == "" {
		return ""
	}

	// Strip the /app prefix if it exists
	logoPath := b.LogoPath
	if len(logoPath) >= 4 && logoPath[:4] == "/app" {
		logoPath = logoPath[4:]
	}

	return logoPath
}
