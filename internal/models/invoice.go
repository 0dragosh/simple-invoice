package models

import (
	"time"
)

// Invoice represents an invoice
type Invoice struct {
	ID               int       `json:"id"`
	InvoiceNumber    string    `json:"invoice_number"`
	BusinessID       int       `json:"business_id"`
	ClientID         int       `json:"client_id"`
	IssueDate        time.Time `json:"issue_date"`
	DueDate          time.Time `json:"due_date"`
	HourlyRate       float64   `json:"hourly_rate"`
	HoursWorked      float64   `json:"hours_worked"`
	TotalAmount      float64   `json:"total_amount"`
	VatRate          float64   `json:"vat_rate"`
	VatAmount        float64   `json:"vat_amount"`
	ReverseChargeVat bool      `json:"reverse_charge_vat"`
	Currency         string    `json:"currency"`
	Notes            string    `json:"notes"`
	Status           string    `json:"status"` // draft, sent, paid
}

// InvoiceItem represents a line item on an invoice
type InvoiceItem struct {
	ID          int     `json:"id"`
	InvoiceID   int     `json:"invoice_id"`
	Description string  `json:"description"`
	Quantity    float64 `json:"quantity"`
	UnitPrice   float64 `json:"unit_price"`
	Amount      float64 `json:"amount"`
}
