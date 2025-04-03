package services

import (
	"fmt"
	"image"
	"image/color"
	_ "image/jpeg" // Register JPEG format
	_ "image/png"  // Register PNG format
	"math"
	"os"
	"path/filepath"
	"strings"

	"github.com/0dragosh/simple-invoice/internal/models"
	"github.com/jung-kurt/gofpdf/v2"
)

// PDFService provides methods for generating PDF invoices
type PDFService struct {
	dataDir string
}

// NewPDFService creates a new PDFService
func NewPDFService(dataDir string) *PDFService {
	return &PDFService{
		dataDir: dataDir,
	}
}

// ThemeColors represents the primary and secondary colors for the invoice theme
type ThemeColors struct {
	Primary   color.RGBA
	Secondary color.RGBA
}

// RGBToHex converts RGB values to a hex color string
func RGBToHex(r, g, b uint8) string {
	return fmt.Sprintf("%02X%02X%02X", r, g, b)
}

// ExtractColorsFromImage extracts two dominant colors from an image
func ExtractColorsFromImage(imgPath string) (ThemeColors, error) {
	// Default colors if extraction fails - using modern teal and coral accent
	defaultTheme := ThemeColors{
		Primary:   color.RGBA{R: 0, G: 150, B: 136, A: 255},  // Teal
		Secondary: color.RGBA{R: 255, G: 111, B: 97, A: 255}, // Coral accent
	}

	// Open the image file
	file, err := os.Open(imgPath)
	if err != nil {
		return defaultTheme, err
	}
	defer file.Close()

	// Decode the image
	img, _, err := image.Decode(file)
	if err != nil {
		return defaultTheme, err
	}

	// Get image bounds
	bounds := img.Bounds()
	width, height := bounds.Max.X, bounds.Max.Y

	// Sample colors from the image
	colorMap := make(map[uint32]int)
	for y := 0; y < height; y += 5 { // Sample every 5 pixels
		for x := 0; x < width; x += 5 {
			r, g, b, _ := img.At(x, y).RGBA()
			// Convert to 8-bit color
			r8, g8, b8 := uint8(r>>8), uint8(g>>8), uint8(b>>8)

			// Skip very light (white) and very dark (black) colors
			brightness := (float64(r8) + float64(g8) + float64(b8)) / 3.0
			if brightness < 20 || brightness > 235 {
				continue
			}

			// Create a color key
			colorKey := uint32(r8)<<16 | uint32(g8)<<8 | uint32(b8)
			colorMap[colorKey]++
		}
	}

	// Find the most common color
	var mostCommonColor uint32
	var maxCount int
	for colorKey, count := range colorMap {
		if count > maxCount {
			maxCount = count
			mostCommonColor = colorKey
		}
	}

	// Extract RGB components from the most common color
	primaryR := uint8((mostCommonColor >> 16) & 0xFF)
	primaryG := uint8((mostCommonColor >> 8) & 0xFF)
	primaryB := uint8(mostCommonColor & 0xFF)

	// Find a contrasting color
	var secondaryR, secondaryG, secondaryB uint8
	maxDistance := 0.0

	for colorKey := range colorMap {
		r := uint8((colorKey >> 16) & 0xFF)
		g := uint8((colorKey >> 8) & 0xFF)
		b := uint8(colorKey & 0xFF)

		// Calculate color distance (simple Euclidean distance in RGB space)
		distance := math.Sqrt(
			math.Pow(float64(primaryR)-float64(r), 2) +
				math.Pow(float64(primaryG)-float64(g), 2) +
				math.Pow(float64(primaryB)-float64(b), 2),
		)

		if distance > maxDistance {
			maxDistance = distance
			secondaryR, secondaryG, secondaryB = r, g, b
		}
	}

	// If we couldn't find a good contrasting color, use a default
	if maxDistance < 100 {
		if (float64(primaryR)+float64(primaryG)+float64(primaryB))/3 > 128 {
			// If primary is light, use a dark secondary
			secondaryR, secondaryG, secondaryB = 41, 128, 185 // Blue
		} else {
			// If primary is dark, use a light secondary
			secondaryR, secondaryG, secondaryB = 231, 76, 60 // Red
		}
	}

	return ThemeColors{
		Primary:   color.RGBA{R: primaryR, G: primaryG, B: primaryB, A: 255},
		Secondary: color.RGBA{R: secondaryR, G: secondaryG, B: secondaryB, A: 255},
	}, nil
}

// GenerateInvoice generates a PDF invoice
func (s *PDFService) GenerateInvoice(invoice *models.Invoice, business *models.Business, client *models.Client, items []models.InvoiceItem) (string, error) {
	// Create a new PDF with UTF-8 encoding
	pdf := gofpdf.New("P", "mm", "A4", "")
	pdf.SetMargins(15, 15, 15)

	// Enable UTF-8 encoding
	pdf.SetAuthor("Simple Invoice", true)
	pdf.SetCreator("Simple Invoice", true)

	// Use core fonts with encoding for currency symbols
	pdf.AddPage()

	// Set default font
	pdf.SetFont("Helvetica", "", 10)

	// Define theme colors
	var theme ThemeColors
	var useColors bool = false

	// Helper function to format currency values
	formatCurrency := func(amount float64) string {
		// Use currency code instead of symbol to avoid encoding issues
		return fmt.Sprintf("%.2f %s", amount, invoice.Currency)
	}

	// Check if business has a logo
	if business.LogoPath != "" {
		useColors = true
		// Extract colors from logo if available
		var logoPath string
		// Check if the logo path already includes the data directory
		if strings.HasPrefix(business.LogoPath, s.dataDir) {
			// Logo path already includes the data directory
			logoPath = business.LogoPath
		} else if strings.HasPrefix(business.LogoPath, "/app/data") {
			// Logo path includes the container data directory
			// Extract just the filename
			logoPath = filepath.Join(s.dataDir, "images", filepath.Base(business.LogoPath))
		} else {
			// Logo path is just the filename
			logoPath = filepath.Join(s.dataDir, "images", business.LogoPath)
		}

		fmt.Printf("Checking for logo at path: %s\n", logoPath)
		if fileExists(logoPath) {
			fmt.Printf("Logo file exists, extracting colors\n")
			var err error
			theme, err = ExtractColorsFromImage(logoPath)
			if err != nil {
				fmt.Printf("Failed to extract colors from logo: %v\n", err)
				// Use default colors if extraction fails
				theme = ThemeColors{
					Primary:   color.RGBA{R: 0, G: 150, B: 136, A: 255},  // Teal
					Secondary: color.RGBA{R: 255, G: 111, B: 97, A: 255}, // Coral accent
				}
			}
		} else {
			fmt.Printf("Logo file does not exist at path: %s\n", logoPath)
			// Try alternative paths
			alternativePaths := []string{
				filepath.Join(s.dataDir, "images", filepath.Base(business.LogoPath)),
				filepath.Join("/app/data/images", filepath.Base(business.LogoPath)),
				business.LogoPath,
			}

			for _, altPath := range alternativePaths {
				if altPath != logoPath {
					fmt.Printf("Trying alternative path: %s\n", altPath)
					if fileExists(altPath) {
						logoPath = altPath
						fmt.Printf("Found logo at alternative path: %s\n", logoPath)
						var err error
						theme, err = ExtractColorsFromImage(logoPath)
						if err != nil {
							fmt.Printf("Failed to extract colors from logo: %v\n", err)
							// Use default colors if extraction fails
							theme = ThemeColors{
								Primary:   color.RGBA{R: 0, G: 150, B: 136, A: 255},  // Teal
								Secondary: color.RGBA{R: 255, G: 111, B: 97, A: 255}, // Coral accent
							}
						}
						break
					}
				}
			}

			if !fileExists(logoPath) {
				// Use default colors if logo file doesn't exist
				theme = ThemeColors{
					Primary:   color.RGBA{R: 0, G: 150, B: 136, A: 255},  // Teal
					Secondary: color.RGBA{R: 255, G: 111, B: 97, A: 255}, // Coral accent
				}
			}
		}
	} else {
		fmt.Printf("No logo path specified for business\n")
		// Default black/gray colors when no logo
		theme = ThemeColors{
			Primary:   color.RGBA{R: 50, G: 50, B: 50, A: 255},
			Secondary: color.RGBA{R: 100, G: 100, B: 100, A: 255},
		}
	}

	// Convert colors to hex for PDF
	primaryColor := RGBToHex(theme.Primary.R, theme.Primary.G, theme.Primary.B)
	secondaryColor := RGBToHex(theme.Secondary.R, theme.Secondary.G, theme.Secondary.B)

	// Add logo if available
	if business.LogoPath != "" {
		var logoPath string
		// Check if the logo path already includes the data directory
		if strings.HasPrefix(business.LogoPath, s.dataDir) {
			// Logo path already includes the data directory
			logoPath = business.LogoPath
		} else if strings.HasPrefix(business.LogoPath, "/app/data") {
			// Logo path includes the container data directory
			// Extract just the filename
			logoPath = filepath.Join(s.dataDir, "images", filepath.Base(business.LogoPath))
		} else {
			// Logo path is just the filename
			logoPath = filepath.Join(s.dataDir, "images", business.LogoPath)
		}

		fmt.Printf("Adding logo to PDF from path: %s\n", logoPath)
		if fileExists(logoPath) {
			fmt.Printf("Logo file exists, adding to PDF\n")
			pdf.Image(logoPath, 15, 15, 40, 0, false, "", 0, "")
		} else {
			// Try alternative paths
			alternativePaths := []string{
				filepath.Join(s.dataDir, "images", filepath.Base(business.LogoPath)),
				filepath.Join("/app/data/images", filepath.Base(business.LogoPath)),
				business.LogoPath,
			}

			for _, altPath := range alternativePaths {
				if altPath != logoPath {
					fmt.Printf("Trying alternative path for logo: %s\n", altPath)
					if fileExists(altPath) {
						fmt.Printf("Found logo at alternative path, adding to PDF: %s\n", altPath)
						pdf.Image(altPath, 15, 15, 40, 0, false, "", 0, "")
						break
					}
				}
			}

			if !fileExists(logoPath) {
				fmt.Printf("Logo file does not exist at path: %s, skipping logo\n", logoPath)
			}
		}
	}

	// Modern header with clean typography
	pdf.SetFont("Helvetica", "B", 24)
	if useColors {
		pdf.SetTextColor(hexToR(primaryColor), hexToG(primaryColor), hexToB(primaryColor))
	} else {
		pdf.SetTextColor(50, 50, 50)
	}
	pdf.SetY(15)
	pdf.SetX(60)
	pdf.Cell(0, 10, "INVOICE")

	// Add invoice number with secondary color
	pdf.SetFont("Helvetica", "", 12)
	if useColors {
		pdf.SetTextColor(hexToR(secondaryColor), hexToG(secondaryColor), hexToB(secondaryColor))
	} else {
		pdf.SetTextColor(100, 100, 100)
	}
	pdf.SetY(25)
	pdf.SetX(60)
	pdf.Cell(0, 10, "#"+invoice.InvoiceNumber)

	// Add a subtle divider line
	pdf.SetDrawColor(230, 230, 230)
	pdf.Line(15, 40, 195, 40)

	// Business and client information in a modern two-column layout
	pdf.SetY(45)
	pdf.SetFont("Helvetica", "B", 10)
	pdf.SetTextColor(80, 80, 80)
	pdf.Cell(90, 6, "FROM")
	pdf.SetX(105)
	pdf.Cell(90, 6, "TO")

	// Business details
	pdf.SetY(52)
	pdf.SetFont("Helvetica", "B", 11)
	pdf.SetTextColor(50, 50, 50)
	pdf.Cell(90, 6, business.Name)

	// Client details
	pdf.SetX(105)
	pdf.SetFont("Helvetica", "B", 11)
	pdf.Cell(90, 6, client.Name)

	// Business address
	pdf.SetY(58)
	pdf.SetFont("Helvetica", "", 9)
	pdf.SetTextColor(100, 100, 100)
	pdf.MultiCell(90, 5, business.Address+"\n"+business.City+", "+business.PostalCode+"\n"+business.Country, "", "", false)

	// Add VAT ID and other business details
	y := pdf.GetY() + 2
	pdf.SetY(y)
	pdf.SetFont("Helvetica", "", 9)
	pdf.Cell(90, 5, "VAT ID: "+business.VatID)
	y += 5
	pdf.SetY(y)
	pdf.Cell(90, 5, "Email: "+business.Email)

	// Client address
	pdf.SetY(58)
	pdf.SetX(105)
	pdf.SetFont("Helvetica", "", 9)
	pdf.MultiCell(90, 5, client.Address+"\n"+client.City+", "+client.PostalCode+"\n"+client.Country, "", "", false)

	// Add VAT ID for client
	y = pdf.GetY() + 2
	pdf.SetY(y)
	pdf.SetX(105)
	pdf.Cell(90, 5, "VAT ID: "+client.VatID)

	// Add dates in a modern, clean format
	y = pdf.GetY() + 15
	pdf.SetY(y)
	pdf.SetFont("Helvetica", "B", 10)
	pdf.SetTextColor(80, 80, 80)
	pdf.Cell(60, 6, "ISSUE DATE")
	pdf.SetX(75)
	pdf.Cell(60, 6, "DUE DATE")

	// Date values
	pdf.SetY(y + 6)
	pdf.SetFont("Helvetica", "", 10)
	pdf.SetTextColor(50, 50, 50)
	pdf.Cell(60, 6, invoice.IssueDate.Format("Jan 02, 2006"))
	pdf.SetX(75)
	pdf.Cell(60, 6, invoice.DueDate.Format("Jan 02, 2006"))

	// Add a subtle divider line
	pdf.SetDrawColor(230, 230, 230)
	pdf.Line(15, y+16, 195, y+16)

	// Add invoice items table with modern styling
	y = y + 25
	pdf.SetY(y)

	// Table headers with clean design
	pdf.SetFont("Helvetica", "B", 10)
	pdf.SetFillColor(245, 245, 245)
	pdf.SetTextColor(80, 80, 80)

	// Modern table header with subtle background
	pdf.Rect(15, y, 180, 8, "F")
	pdf.Cell(90, 8, "  DESCRIPTION")
	pdf.SetX(105)
	pdf.Cell(30, 8, "QUANTITY")
	pdf.SetX(135)
	pdf.Cell(30, 8, "UNIT PRICE")
	pdf.SetX(165)
	pdf.Cell(30, 8, "AMOUNT")

	// Table rows
	y += 8
	pdf.SetFont("Helvetica", "", 9)
	pdf.SetTextColor(70, 70, 70)

	// Alternating row colors for better readability
	alternate := false

	for _, item := range items {
		if alternate {
			pdf.SetFillColor(250, 250, 250)
			pdf.Rect(15, y, 180, 8, "F")
		}
		alternate = !alternate

		pdf.SetY(y)
		pdf.SetX(15)
		pdf.MultiCell(90, 8, item.Description, "", "", false)

		// Make sure we're at the right Y position after the multi-line description
		currentY := pdf.GetY()
		if currentY > y {
			y = currentY
		}

		pdf.SetY(y - 8) // Go back to the start of this row
		pdf.SetX(105)
		pdf.Cell(30, 8, fmt.Sprintf("%.2f", item.Quantity))
		pdf.SetX(135)
		pdf.Cell(30, 8, formatCurrency(item.UnitPrice))
		pdf.SetX(165)
		pdf.Cell(30, 8, formatCurrency(item.Amount))

		y += 8
	}

	// Add a subtle divider line
	pdf.SetDrawColor(230, 230, 230)
	pdf.Line(15, y+2, 195, y+2)

	// Add totals with modern styling
	y += 10
	pdf.SetY(y)
	pdf.SetFont("Helvetica", "", 10)
	pdf.SetTextColor(80, 80, 80)
	pdf.SetX(135)
	pdf.Cell(30, 6, "Subtotal:")
	pdf.SetX(165)
	pdf.Cell(30, 6, formatCurrency(invoice.TotalAmount-invoice.VatAmount))

	y += 6
	pdf.SetY(y)
	pdf.SetX(135)

	// VAT line
	if invoice.ReverseChargeVat {
		pdf.Cell(30, 6, fmt.Sprintf("VAT (%.1f%%):", invoice.VatRate))
		pdf.SetX(165)
		pdf.Cell(30, 6, "Reverse Charge")
	} else {
		pdf.Cell(30, 6, fmt.Sprintf("VAT (%.1f%%):", invoice.VatRate))
		pdf.SetX(165)
		pdf.Cell(30, 6, formatCurrency(invoice.VatAmount))
	}

	// Total with emphasis
	y += 8
	pdf.SetY(y)
	pdf.SetFont("Helvetica", "B", 12)
	if useColors {
		pdf.SetTextColor(hexToR(primaryColor), hexToG(primaryColor), hexToB(primaryColor))
	} else {
		pdf.SetTextColor(50, 50, 50)
	}
	pdf.SetX(135)
	pdf.Cell(30, 8, "TOTAL:")
	pdf.SetX(165)
	pdf.Cell(30, 8, formatCurrency(invoice.TotalAmount))

	// Add notes section with subtle styling
	if invoice.Notes != "" {
		y += 20
		pdf.SetY(y)
		pdf.SetFont("Helvetica", "B", 10)
		pdf.SetTextColor(80, 80, 80)
		pdf.Cell(30, 6, "NOTES:")

		y += 6
		pdf.SetY(y)
		pdf.SetFont("Helvetica", "", 9)
		pdf.SetTextColor(100, 100, 100)
		pdf.MultiCell(180, 5, invoice.Notes, "", "", false)
	}

	// Add payment information only if bank details are provided
	if business.BankName != "" && business.IBAN != "" && business.BIC != "" {
		y = pdf.GetY() + 10
		pdf.SetY(y)
		pdf.SetFont("Helvetica", "B", 10)
		pdf.SetTextColor(80, 80, 80)
		pdf.Cell(90, 6, "PAYMENT INFORMATION")

		y += 6
		pdf.SetY(y)
		pdf.SetFont("Helvetica", "", 9)
		pdf.SetTextColor(100, 100, 100)
		pdf.Cell(30, 5, "Bank Name:")
		pdf.SetX(45)
		pdf.Cell(90, 5, business.BankName)

		y += 5
		pdf.SetY(y)
		pdf.Cell(30, 5, "IBAN:")
		pdf.SetX(45)
		pdf.Cell(90, 5, business.IBAN)

		y += 5
		pdf.SetY(y)
		pdf.Cell(30, 5, "BIC:")
		pdf.SetX(45)
		pdf.Cell(90, 5, business.BIC)
	}

	// Generate PDF file path
	pdfFileName := fmt.Sprintf("invoice-%s.pdf", invoice.InvoiceNumber)
	pdfPath := filepath.Join(s.dataDir, "pdfs", pdfFileName)

	// Ensure the pdfs directory exists
	pdfsDir := filepath.Join(s.dataDir, "pdfs")
	if err := os.MkdirAll(pdfsDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create pdfs directory: %w", err)
	}

	// Save PDF to file
	err := pdf.OutputFileAndClose(pdfPath)
	if err != nil {
		return "", fmt.Errorf("failed to save PDF file: %w", err)
	}

	return pdfPath, nil
}

// Helper functions for color conversion
func hexToR(h string) int {
	if len(h) < 2 {
		return 0
	}
	r := h[0:2]
	ret := 0
	fmt.Sscanf(r, "%X", &ret)
	return ret
}

func hexToG(h string) int {
	if len(h) < 4 {
		return 0
	}
	r := h[2:4]
	ret := 0
	fmt.Sscanf(r, "%X", &ret)
	return ret
}

func hexToB(h string) int {
	if len(h) < 6 {
		return 0
	}
	r := h[4:6]
	ret := 0
	fmt.Sscanf(r, "%X", &ret)
	return ret
}

// fileExists checks if a file exists and is accessible
func fileExists(filename string) bool {
	info, err := os.Stat(filename)
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Printf("File does not exist: %s\n", filename)
		} else {
			fmt.Printf("Error checking file: %s - %v\n", filename, err)
		}
		return false
	}

	if info.IsDir() {
		fmt.Printf("Path is a directory, not a file: %s\n", filename)
		return false
	}

	// Check if file is readable
	file, err := os.Open(filename)
	if err != nil {
		fmt.Printf("File exists but cannot be opened: %s - %v\n", filename, err)
		return false
	}
	file.Close()

	fmt.Printf("File exists and is accessible: %s (size: %d bytes)\n", filename, info.Size())
	return true
}
