package models

import (
	"encoding/json"
	"testing"
	"time"
)

func TestInvoiceJSON(t *testing.T) {
	// Create a test invoice
	invoice := Invoice{
		ID:               1,
		InvoiceNumber:    "INV-001",
		BusinessID:       2,
		ClientID:         3,
		IssueDate:        time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC),
		DueDate:          time.Date(2023, 1, 31, 0, 0, 0, 0, time.UTC),
		HourlyRate:       50.0,
		HoursWorked:      40.0,
		TotalAmount:      2000.0,
		VatRate:          19.0,
		VatAmount:        380.0,
		ReverseChargeVat: false,
		Currency:         "EUR",
		Notes:            "Test invoice",
		Status:           "Draft",
	}

	// Marshal to JSON
	jsonData, err := json.Marshal(invoice)
	if err != nil {
		t.Fatalf("Failed to marshal invoice to JSON: %v", err)
	}

	// Unmarshal back to invoice
	var unmarshaledInvoice Invoice
	err = json.Unmarshal(jsonData, &unmarshaledInvoice)
	if err != nil {
		t.Fatalf("Failed to unmarshal invoice from JSON: %v", err)
	}

	// Check that the unmarshaled invoice matches the original
	if unmarshaledInvoice.ID != invoice.ID {
		t.Errorf("Expected ID %d, got %d", invoice.ID, unmarshaledInvoice.ID)
	}
	if unmarshaledInvoice.InvoiceNumber != invoice.InvoiceNumber {
		t.Errorf("Expected invoice number %s, got %s", invoice.InvoiceNumber, unmarshaledInvoice.InvoiceNumber)
	}
	if unmarshaledInvoice.TotalAmount != invoice.TotalAmount {
		t.Errorf("Expected total amount %f, got %f", invoice.TotalAmount, unmarshaledInvoice.TotalAmount)
	}
	if unmarshaledInvoice.Currency != invoice.Currency {
		t.Errorf("Expected currency %s, got %s", invoice.Currency, unmarshaledInvoice.Currency)
	}
}

func TestInvoiceItemJSON(t *testing.T) {
	// Create a test invoice item
	item := InvoiceItem{
		ID:          1,
		InvoiceID:   2,
		Description: "Test Item",
		Quantity:    10.0,
		UnitPrice:   50.0,
		Amount:      500.0,
	}

	// Marshal to JSON
	jsonData, err := json.Marshal(item)
	if err != nil {
		t.Fatalf("Failed to marshal invoice item to JSON: %v", err)
	}

	// Unmarshal back to invoice item
	var unmarshaledItem InvoiceItem
	err = json.Unmarshal(jsonData, &unmarshaledItem)
	if err != nil {
		t.Fatalf("Failed to unmarshal invoice item from JSON: %v", err)
	}

	// Check that the unmarshaled invoice item matches the original
	if unmarshaledItem.ID != item.ID {
		t.Errorf("Expected ID %d, got %d", item.ID, unmarshaledItem.ID)
	}
	if unmarshaledItem.Description != item.Description {
		t.Errorf("Expected description %s, got %s", item.Description, unmarshaledItem.Description)
	}
	if unmarshaledItem.Quantity != item.Quantity {
		t.Errorf("Expected quantity %f, got %f", item.Quantity, unmarshaledItem.Quantity)
	}
	if unmarshaledItem.UnitPrice != item.UnitPrice {
		t.Errorf("Expected unit price %f, got %f", item.UnitPrice, unmarshaledItem.UnitPrice)
	}
	if unmarshaledItem.Amount != item.Amount {
		t.Errorf("Expected amount %f, got %f", item.Amount, unmarshaledItem.Amount)
	}
}
