package services

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/0dragosh/simple-invoice/internal/models"
)

func setupTestPDFService(t *testing.T) (*PDFService, string, func()) {
	// Create a temporary directory for the test data
	tempDir, err := os.MkdirTemp("", "simple-invoice-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}

	// Create subdirectories
	os.MkdirAll(filepath.Join(tempDir, "images"), 0755)
	os.MkdirAll(filepath.Join(tempDir, "pdfs"), 0755)

	// Create a new PDF service with the temp directory
	pdfService := NewPDFService(tempDir)

	// Return the service, temp dir, and a cleanup function
	cleanup := func() {
		os.RemoveAll(tempDir)
	}

	return pdfService, tempDir, cleanup
}

func TestRGBToHex(t *testing.T) {
	tests := []struct {
		name     string
		r, g, b  uint8
		expected string
	}{
		{
			name:     "Black",
			r:        0,
			g:        0,
			b:        0,
			expected: "000000",
		},
		{
			name:     "White",
			r:        255,
			g:        255,
			b:        255,
			expected: "FFFFFF",
		},
		{
			name:     "Red",
			r:        255,
			g:        0,
			b:        0,
			expected: "FF0000",
		},
		{
			name:     "Green",
			r:        0,
			g:        255,
			b:        0,
			expected: "00FF00",
		},
		{
			name:     "Blue",
			r:        0,
			g:        0,
			b:        255,
			expected: "0000FF",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := RGBToHex(tt.r, tt.g, tt.b)
			if result != tt.expected {
				t.Errorf("RGBToHex(%d, %d, %d) = %s, want %s", tt.r, tt.g, tt.b, result, tt.expected)
			}
		})
	}
}

// Skip the TestGenerateInvoice test for now as it requires more setup
func TestGenerateInvoiceSkip(t *testing.T) {
	t.Skip("Skipping TestGenerateInvoice as it requires more setup")
}

func TestGenerateInvoice(t *testing.T) {
	// Create a temporary directory for testing
	tempDir := filepath.Join(os.TempDir(), "simple-invoice-test")
	if err := os.MkdirAll(tempDir, 0755); err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create pdfs directory
	pdfsDir := filepath.Join(tempDir, "pdfs")
	if err := os.MkdirAll(pdfsDir, 0755); err != nil {
		t.Fatalf("Failed to create pdfs directory: %v", err)
	}

	// Create images directory
	imagesDir := filepath.Join(tempDir, "images")
	if err := os.MkdirAll(imagesDir, 0755); err != nil {
		t.Fatalf("Failed to create images directory: %v", err)
	}

	// Create a new PDF service
	pdfService := NewPDFService(tempDir)

	// Create test data
	invoice := &models.Invoice{
		ID:            1,
		InvoiceNumber: "INV-001",
		BusinessID:    1,
		ClientID:      1,
		IssueDate:     time.Now(),
		DueDate:       time.Now().AddDate(0, 0, 30),
		TotalAmount:   120.0,
		Currency:      "EUR",
		Notes:         "Test invoice",
		VatRate:       20.0,
		VatAmount:     20.0,
	}

	business := &models.Business{
		ID:         1,
		Name:       "Test Business",
		Address:    "123 Business St",
		City:       "Business City",
		PostalCode: "12345",
		Country:    "Test Country",
		VatID:      "TEST123456",
		Email:      "test@business.com",
		BankName:   "Test Bank",
		IBAN:       "TEST1234567890",
		BIC:        "TESTBIC",
	}

	client := &models.Client{
		ID:         1,
		Name:       "Test Client",
		Address:    "456 Client St",
		City:       "Client City",
		PostalCode: "67890",
		Country:    "Test Country",
		VatID:      "CLIENT123456",
	}

	items := []models.InvoiceItem{
		{
			ID:          1,
			InvoiceID:   1,
			Description: "Test Item 1",
			Quantity:    1,
			UnitPrice:   50.0,
			Amount:      50.0,
		},
		{
			ID:          2,
			InvoiceID:   1,
			Description: "Test Item 2",
			Quantity:    1,
			UnitPrice:   50.0,
			Amount:      50.0,
		},
	}

	// Generate the PDF
	pdfPath, err := pdfService.GenerateInvoice(invoice, business, client, items)
	if err != nil {
		t.Fatalf("Failed to generate PDF: %v", err)
	}

	// Check if the PDF file exists
	if _, err := os.Stat(pdfPath); os.IsNotExist(err) {
		t.Errorf("PDF file was not created at %s", pdfPath)
	}

	// Check if the PDF file has content
	fileInfo, err := os.Stat(pdfPath)
	if err != nil {
		t.Fatalf("Failed to get file info: %v", err)
	}

	if fileInfo.Size() == 0 {
		t.Error("PDF file is empty")
	}
}
