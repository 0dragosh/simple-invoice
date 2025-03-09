package models

// Client represents a client's details
type Client struct {
	ID            int    `json:"id"`
	Name          string `json:"name"`
	Address       string `json:"address"`
	City          string `json:"city"`
	PostalCode    string `json:"postal_code"`
	Country       string `json:"country"`
	VatID         string `json:"vat_id"`
	CompanyNumber string `json:"company_number"`
}
