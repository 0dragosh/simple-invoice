package models

// Business represents the consultant's business details
type Business struct {
	ID          int    `json:"id"`
	Name        string `json:"name"`
	Address     string `json:"address"`
	City        string `json:"city"`
	PostalCode  string `json:"postal_code"`
	Country     string `json:"country"`
	VatID       string `json:"vat_id"`
	Email       string `json:"email"`
	BankName    string `json:"bank_name"`
	BankAccount string `json:"bank_account"`
	IBAN        string `json:"iban"`
	BIC         string `json:"bic"`
	LogoPath    string `json:"logo_path"`
}
