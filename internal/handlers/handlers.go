package handlers

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/0dragosh/simple-invoice/internal/models"
	"github.com/0dragosh/simple-invoice/internal/services"
)

// AppHandler holds the services and templates for the application
type AppHandler struct {
	dbService  *services.DBService
	vatService *services.VatService
	pdfService *services.PDFService
	templates  map[string]*template.Template
	dataDir    string
	logger     *services.Logger
}

// NewAppHandler creates a new AppHandler
func NewAppHandler(dataDir string, logger *services.Logger) (*AppHandler, error) {
	// Ensure data directory exists
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create data directory: %w", err)
	}

	// Create images directory
	imagesDir := filepath.Join(dataDir, "images")
	if err := os.MkdirAll(imagesDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create images directory: %w", err)
	}

	// Create pdfs directory
	pdfsDir := filepath.Join(dataDir, "pdfs")
	if err := os.MkdirAll(pdfsDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create pdfs directory: %w", err)
	}

	logger.Info("Initializing application handler")

	// Initialize services
	dbService, err := services.NewDBService(dataDir, logger)
	if err != nil {
		logger.Error("Failed to initialize database service: %v", err)
		return nil, fmt.Errorf("failed to initialize database service: %w", err)
	}

	vatService := services.NewVatService(logger)
	pdfService := services.NewPDFService(dataDir)

	// Parse templates
	templates := make(map[string]*template.Template)
	templatesDir := "internal/templates"

	// Define template functions
	funcMap := template.FuncMap{
		"add": func(a, b float64) float64 {
			return a + b
		},
		"formatDate": func(t time.Time) string {
			return t.Format("2006-01-02")
		},
		"formatCurrency": func(amount float64) string {
			return fmt.Sprintf("%.2f", amount)
		},
		"currencySymbol": func(currencyCode string) string {
			return services.FormatCurrencySymbol(currencyCode)
		},
		"filepath": filepath.Base,
	}

	// Load base layout
	baseLayout := filepath.Join(templatesDir, "layout.html")

	// Load page templates
	pages := []string{"index", "business", "clients", "invoices", "create-invoice", "view-invoice"}
	for _, page := range pages {
		tmpl, err := template.New("layout").Funcs(funcMap).ParseFiles(baseLayout, filepath.Join(templatesDir, page+".html"))
		if err != nil {
			logger.Error("Failed to parse template %s: %v", page, err)
			return nil, fmt.Errorf("failed to parse template %s: %w", page, err)
		}
		templates[page] = tmpl
	}

	logger.Info("Application handler initialized successfully")

	return &AppHandler{
		dbService:  dbService,
		vatService: vatService,
		pdfService: pdfService,
		templates:  templates,
		dataDir:    dataDir,
		logger:     logger,
	}, nil
}

// RegisterHandlers registers all HTTP handlers
func RegisterHandlers(mux *http.ServeMux, dataDir string, logger *services.Logger) (*AppHandler, error) {
	handler, err := NewAppHandler(dataDir, logger)
	if err != nil {
		logger.Fatal("Failed to create application handler: %v", err)
		return nil, fmt.Errorf("failed to create application handler: %w", err)
	}

	// Pages
	mux.HandleFunc("/", handler.IndexHandler)
	mux.HandleFunc("/business", handler.BusinessHandler)
	mux.HandleFunc("/clients", handler.ClientsHandler)
	mux.HandleFunc("/invoices", handler.InvoicesHandler)
	mux.HandleFunc("/invoices/create", handler.CreateInvoiceHandler)
	mux.HandleFunc("/invoices/view/", handler.ViewInvoiceHandler)

	// API endpoints - register more specific routes first
	mux.HandleFunc("/api/clients/vat-lookup", handler.VatLookupHandler)
	mux.HandleFunc("/api/clients/uk-company-lookup", handler.UKCompanyLookupHandler)
	mux.HandleFunc("/api/invoices/generate-pdf/", handler.GeneratePDFHandler)
	mux.HandleFunc("/api/invoices/", handler.InvoiceByIDHandler)

	// Then register the more general routes
	mux.HandleFunc("/api/business", handler.BusinessAPIHandler)
	mux.HandleFunc("/api/clients/", handler.ClientsAPIHandler)
	mux.HandleFunc("/api/clients", handler.ClientsAPIHandler)
	mux.HandleFunc("/api/invoices", handler.InvoicesAPIHandler)
	mux.HandleFunc("/api/upload-logo", handler.UploadLogoHandler)

	// Serve static files from data directory
	fs := http.FileServer(http.Dir(dataDir))
	mux.Handle("/data/", http.StripPrefix("/data/", fs))

	// Log the data directory path for debugging
	logger.Info("Serving static files from: %s", dataDir)
	logger.Info("PDF files will be available at: %s/pdfs/", dataDir)

	logger.Info("All handlers registered successfully")

	return handler, nil
}

// IndexHandler handles the home page
func (h *AppHandler) IndexHandler(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	data := map[string]interface{}{
		"Title":       "Simple Invoice",
		"CurrentYear": time.Now().Year(),
	}

	h.renderTemplate(w, "index", data)
}

// BusinessHandler handles the business details page
func (h *AppHandler) BusinessHandler(w http.ResponseWriter, r *http.Request) {
	businesses, err := h.dbService.GetBusinesses()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	var business models.Business
	if len(businesses) > 0 {
		business = businesses[0]
	}

	data := map[string]interface{}{
		"Title":       "Business Details",
		"Business":    business,
		"CurrentYear": time.Now().Year(),
	}

	h.renderTemplate(w, "business", data)
}

// ClientsHandler handles the clients page
func (h *AppHandler) ClientsHandler(w http.ResponseWriter, r *http.Request) {
	clients, err := h.dbService.GetClients()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	data := map[string]interface{}{
		"Title":       "Clients",
		"Clients":     clients,
		"CurrentYear": time.Now().Year(),
	}

	h.renderTemplate(w, "clients", data)
}

// InvoicesHandler handles the invoices page
func (h *AppHandler) InvoicesHandler(w http.ResponseWriter, r *http.Request) {
	invoices, err := h.dbService.GetInvoices()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Fetch client information for each invoice
	type InvoiceWithClient struct {
		models.Invoice
		ClientName string
	}

	invoicesWithClients := make([]InvoiceWithClient, 0, len(invoices))
	for _, invoice := range invoices {
		client, err := h.dbService.GetClient(invoice.ClientID)
		if err != nil {
			// If client not found, use a placeholder
			invoicesWithClients = append(invoicesWithClients, InvoiceWithClient{
				Invoice:    invoice,
				ClientName: "Unknown Client",
			})
			continue
		}

		invoicesWithClients = append(invoicesWithClients, InvoiceWithClient{
			Invoice:    invoice,
			ClientName: client.Name,
		})
	}

	data := map[string]interface{}{
		"Title":       "Invoices",
		"Invoices":    invoicesWithClients,
		"CurrentYear": time.Now().Year(),
	}

	h.renderTemplate(w, "invoices", data)
}

// CreateInvoiceHandler handles the create invoice page
func (h *AppHandler) CreateInvoiceHandler(w http.ResponseWriter, r *http.Request) {
	clients, err := h.dbService.GetClients()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	businesses, err := h.dbService.GetBusinesses()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	var business models.Business
	if len(businesses) > 0 {
		business = businesses[0]
	}

	// Calculate work hours for the current month
	workHours := services.CalculateWorkHoursForCurrentMonth()

	data := map[string]interface{}{
		"Title":       "Create Invoice",
		"Clients":     clients,
		"Business":    business,
		"IssueDate":   time.Now().Format("2006-01-02"),
		"DueDate":     time.Now().AddDate(0, 0, 30).Format("2006-01-02"), // Due in 30 days
		"CurrentYear": time.Now().Year(),
		"WorkHours":   workHours, // Add work hours for the current month
	}

	h.renderTemplate(w, "create-invoice", data)
}

// ViewInvoiceHandler handles the view invoice page
func (h *AppHandler) ViewInvoiceHandler(w http.ResponseWriter, r *http.Request) {
	idStr := r.URL.Path[len("/invoices/view/"):]
	id, err := strconv.Atoi(idStr)
	if err != nil {
		http.Error(w, "Invalid invoice ID", http.StatusBadRequest)
		return
	}

	invoice, items, err := h.dbService.GetInvoice(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	business, err := h.dbService.GetBusiness(invoice.BusinessID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	client, err := h.dbService.GetClient(invoice.ClientID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	data := map[string]interface{}{
		"Title":       fmt.Sprintf("Invoice #%s", invoice.InvoiceNumber),
		"Invoice":     invoice,
		"Items":       items,
		"Business":    business,
		"Client":      client,
		"CurrentYear": time.Now().Year(),
	}

	h.renderTemplate(w, "view-invoice", data)
}

// BusinessAPIHandler handles business API requests
func (h *AppHandler) BusinessAPIHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	switch r.Method {
	case http.MethodGet:
		businesses, err := h.dbService.GetBusinesses()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		var business models.Business
		if len(businesses) > 0 {
			business = businesses[0]
		}

		json.NewEncoder(w).Encode(business)

	case http.MethodPost:
		var business models.Business
		if err := json.NewDecoder(r.Body).Decode(&business); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		if err := h.dbService.SaveBusiness(&business); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		json.NewEncoder(w).Encode(business)

	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// ClientsAPIHandler handles clients API requests
func (h *AppHandler) ClientsAPIHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	// Check if we're fetching a specific client by ID
	path := r.URL.Path
	h.logger.Debug("ClientsAPIHandler received request for path: %s", path)

	// Skip the vat-lookup endpoint which is handled by VatLookupHandler
	if strings.HasSuffix(path, "/vat-lookup") {
		h.logger.Debug("Skipping vat-lookup request in ClientsAPIHandler")
		return
	}

	pathParts := strings.Split(path, "/")
	if len(pathParts) > 3 && pathParts[3] != "" {
		// Path format: /api/clients/{id}
		h.logger.Info("Received request to fetch client by ID: %s", pathParts[3])

		clientID, err := strconv.Atoi(pathParts[3])
		if err != nil {
			h.logger.Error("Invalid client ID format: %s - %v", pathParts[3], err)
			http.Error(w, fmt.Sprintf("Invalid client ID format: %s", pathParts[3]), http.StatusBadRequest)
			return
		}

		h.logger.Info("Looking up client with ID: %d", clientID)
		client, err := h.dbService.GetClient(clientID)
		if err != nil {
			if err == sql.ErrNoRows {
				h.logger.Error("Client not found with ID: %d", clientID)
				http.Error(w, fmt.Sprintf("Client not found with ID: %d", clientID), http.StatusNotFound)
			} else {
				h.logger.Error("Database error when looking up client ID %d: %v", clientID, err)
				http.Error(w, fmt.Sprintf("Failed to lookup client: %v", err), http.StatusInternalServerError)
			}
			return
		}

		h.logger.Info("Successfully found client: %s (ID: %d)", client.Name, client.ID)
		json.NewEncoder(w).Encode(client)
		return
	}

	switch r.Method {
	case http.MethodGet:
		clients, err := h.dbService.GetClients()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		json.NewEncoder(w).Encode(clients)

	case http.MethodPost:
		h.logger.Info("Received request to create/update client")

		// Decode the client directly
		var client models.Client
		if err := json.NewDecoder(r.Body).Decode(&client); err != nil {
			h.logger.Error("Failed to decode client JSON: %v", err)
			http.Error(w, fmt.Sprintf("Invalid client data: %v", err), http.StatusBadRequest)
			return
		}

		// If no created date is provided, use current date
		if client.CreatedDate == nil {
			now := time.Now()
			client.CreatedDate = &now
		}

		h.logger.Info("Processing client with ID: %d", client.ID)

		if err := h.dbService.SaveClient(&client); err != nil {
			h.logger.Error("Failed to save client: %v", err)
			http.Error(w, fmt.Sprintf("Failed to save client: %v", err), http.StatusInternalServerError)
			return
		}

		h.logger.Info("Successfully saved client: %s", client.Name)
		json.NewEncoder(w).Encode(client)

	default:
		h.logger.Warn("Method not allowed: %s", r.Method)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// VatLookupHandler handles VAT ID lookup requests
func (h *AppHandler) VatLookupHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if r.Method != http.MethodGet {
		h.logger.Warn("Method not allowed: %s", r.Method)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	vatID := r.URL.Query().Get("vat_id")

	if vatID == "" {
		h.logger.Warn("VAT ID is required")
		http.Error(w, "VAT ID is required", http.StatusBadRequest)
		return
	}

	var client *models.Client
	var err error

	h.logger.Info("Looking up VAT ID: %s", vatID)
	client, err = h.vatService.ValidateVatID(vatID)

	if err != nil {
		h.logger.Error("VAT lookup failed: %v", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	h.logger.Info("Successfully looked up client: %s", client.Name)
	json.NewEncoder(w).Encode(client)
}

// UKCompanyLookupHandler handles UK company lookup requests
func (h *AppHandler) UKCompanyLookupHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if r.Method != http.MethodGet {
		h.logger.Warn("Method not allowed: %s", r.Method)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	companyName := r.URL.Query().Get("name")
	companyNumber := r.URL.Query().Get("number")

	if companyName == "" && companyNumber == "" {
		h.logger.Warn("Either company name or number is required")
		http.Error(w, "Either company name or number is required", http.StatusBadRequest)
		return
	}

	var clients []*models.Client
	var client *models.Client
	var err error

	if companyNumber != "" {
		// Lookup by company number
		h.logger.Info("Looking up UK company by number: %s", companyNumber)
		client, err = h.vatService.LookupUKCompanyByNumber(companyNumber)
		if err != nil {
			h.logger.Error("UK company lookup by number failed: %v", err)
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		clients = []*models.Client{client}
	} else {
		// Lookup by company name
		h.logger.Info("Looking up UK company by name: %s", companyName)
		clients, err = h.vatService.LookupUKCompany(companyName)
		if err != nil {
			h.logger.Error("UK company lookup by name failed: %v", err)
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
	}

	if len(clients) == 0 {
		h.logger.Warn("No UK companies found")
		http.Error(w, "No UK companies found", http.StatusNotFound)
		return
	}

	h.logger.Info("Successfully looked up %d UK companies", len(clients))
	json.NewEncoder(w).Encode(clients)
}

// InvoicesAPIHandler handles invoices API requests
func (h *AppHandler) InvoicesAPIHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	switch r.Method {
	case http.MethodGet:
		h.logger.Info("Fetching all invoices")
		invoices, err := h.dbService.GetInvoices()
		if err != nil {
			h.logger.Error("Failed to fetch invoices: %v", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		h.logger.Info("Successfully fetched %d invoices", len(invoices))
		json.NewEncoder(w).Encode(invoices)

	case http.MethodPost:
		h.logger.Info("Received request to create/update invoice")

		// First, decode the raw JSON to handle date strings manually
		var rawRequest map[string]json.RawMessage
		if err := json.NewDecoder(r.Body).Decode(&rawRequest); err != nil {
			h.logger.Error("Failed to decode invoice JSON: %v", err)
			http.Error(w, fmt.Sprintf("Invalid invoice data: %v", err), http.StatusBadRequest)
			return
		}

		// Extract and parse the invoice part
		var rawInvoice map[string]interface{}
		if err := json.Unmarshal(rawRequest["invoice"], &rawInvoice); err != nil {
			h.logger.Error("Failed to parse invoice data: %v", err)
			http.Error(w, fmt.Sprintf("Invalid invoice data: %v", err), http.StatusBadRequest)
			return
		}

		// Extract and parse the items part
		var items []models.InvoiceItem
		if err := json.Unmarshal(rawRequest["items"], &items); err != nil {
			h.logger.Error("Failed to parse invoice items: %v", err)
			http.Error(w, fmt.Sprintf("Invalid invoice items: %v", err), http.StatusBadRequest)
			return
		}

		// Create the invoice object
		invoice := models.Invoice{
			ID:               int(rawInvoice["id"].(float64)),
			InvoiceNumber:    rawInvoice["invoice_number"].(string),
			BusinessID:       int(rawInvoice["business_id"].(float64)),
			ClientID:         int(rawInvoice["client_id"].(float64)),
			HourlyRate:       rawInvoice["hourly_rate"].(float64),
			HoursWorked:      rawInvoice["hours_worked"].(float64),
			TotalAmount:      rawInvoice["total_amount"].(float64),
			VatRate:          rawInvoice["vat_rate"].(float64),
			VatAmount:        rawInvoice["vat_amount"].(float64),
			ReverseChargeVat: rawInvoice["reverse_charge_vat"].(bool),
			Notes:            rawInvoice["notes"].(string),
			Status:           rawInvoice["status"].(string),
		}

		// Parse the date strings
		issueDateStr, ok := rawInvoice["issue_date"].(string)
		if !ok {
			h.logger.Error("Issue date is missing or not a string")
			http.Error(w, "Issue date is required and must be a string in YYYY-MM-DD format", http.StatusBadRequest)
			return
		}

		issueDate, err := time.Parse("2006-01-02", issueDateStr)
		if err != nil {
			h.logger.Error("Failed to parse issue date: %v", err)
			http.Error(w, fmt.Sprintf("Invalid issue date format. Expected YYYY-MM-DD, got: %s", issueDateStr), http.StatusBadRequest)
			return
		}
		invoice.IssueDate = issueDate

		dueDateStr, ok := rawInvoice["due_date"].(string)
		if !ok {
			h.logger.Error("Due date is missing or not a string")
			http.Error(w, "Due date is required and must be a string in YYYY-MM-DD format", http.StatusBadRequest)
			return
		}

		dueDate, err := time.Parse("2006-01-02", dueDateStr)
		if err != nil {
			h.logger.Error("Failed to parse due date: %v", err)
			http.Error(w, fmt.Sprintf("Invalid due date format. Expected YYYY-MM-DD, got: %s", dueDateStr), http.StatusBadRequest)
			return
		}
		invoice.DueDate = dueDate

		h.logger.Info("Processing invoice with %d items, client ID: %d, business ID: %d",
			len(items), invoice.ClientID, invoice.BusinessID)

		h.logger.Debug("Invoice dates: issue_date=%s, due_date=%s",
			invoice.IssueDate.Format("2006-01-02"), invoice.DueDate.Format("2006-01-02"))

		// Validate required fields
		if invoice.ClientID == 0 {
			h.logger.Error("Missing client ID in invoice data")
			http.Error(w, "Client ID is required", http.StatusBadRequest)
			return
		}

		if invoice.BusinessID == 0 {
			h.logger.Error("Missing business ID in invoice data")
			http.Error(w, "Business ID is required", http.StatusBadRequest)
			return
		}

		if len(items) == 0 {
			h.logger.Error("No invoice items provided")
			http.Error(w, "At least one invoice item is required", http.StatusBadRequest)
			return
		}

		if err := h.dbService.SaveInvoice(&invoice, items); err != nil {
			h.logger.Error("Failed to save invoice: %v", err)
			http.Error(w, fmt.Sprintf("Failed to save invoice: %v", err), http.StatusInternalServerError)
			return
		}

		h.logger.Info("Successfully saved invoice #%s with ID: %d", invoice.InvoiceNumber, invoice.ID)
		json.NewEncoder(w).Encode(invoice)

	default:
		h.logger.Warn("Method not allowed: %s", r.Method)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// GeneratePDFHandler generates a PDF invoice
func (h *AppHandler) GeneratePDFHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		h.logger.Warn("Method not allowed for PDF generation: %s", r.Method)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	idStr := r.URL.Path[len("/api/invoices/generate-pdf/"):]
	h.logger.Debug("PDF generation requested for invoice ID: %s", idStr)

	id, err := strconv.Atoi(idStr)
	if err != nil {
		h.logger.Error("Invalid invoice ID for PDF generation: %s - %v", idStr, err)
		http.Error(w, "Invalid invoice ID", http.StatusBadRequest)
		return
	}

	h.logger.Info("Generating PDF for invoice ID: %d", id)

	invoice, items, err := h.dbService.GetInvoice(id)
	if err != nil {
		h.logger.Error("Failed to get invoice for PDF generation: %v", err)
		http.Error(w, fmt.Sprintf("Failed to get invoice: %v", err), http.StatusInternalServerError)
		return
	}
	h.logger.Debug("Retrieved invoice #%s with %d items", invoice.InvoiceNumber, len(items))

	business, err := h.dbService.GetBusiness(invoice.BusinessID)
	if err != nil {
		h.logger.Error("Failed to get business for PDF generation: %v", err)
		http.Error(w, fmt.Sprintf("Failed to get business details: %v", err), http.StatusInternalServerError)
		return
	}
	h.logger.Debug("Retrieved business details: %s", business.Name)

	client, err := h.dbService.GetClient(invoice.ClientID)
	if err != nil {
		h.logger.Error("Failed to get client for PDF generation: %v", err)
		http.Error(w, fmt.Sprintf("Failed to get client details: %v", err), http.StatusInternalServerError)
		return
	}
	h.logger.Debug("Retrieved client details: %s", client.Name)

	// Ensure the pdfs directory exists
	pdfsDir := filepath.Join(h.dataDir, "pdfs")
	if err := os.MkdirAll(pdfsDir, 0755); err != nil {
		h.logger.Error("Failed to create pdfs directory: %v", err)
		http.Error(w, fmt.Sprintf("Failed to create pdfs directory: %v", err), http.StatusInternalServerError)
		return
	}
	h.logger.Debug("Ensured pdfs directory exists: %s", pdfsDir)

	h.logger.Debug("Calling PDF service to generate invoice PDF")
	pdfPath, err := h.pdfService.GenerateInvoice(invoice, business, client, items)
	if err != nil {
		h.logger.Error("Failed to generate PDF: %v", err)
		http.Error(w, fmt.Sprintf("Failed to generate PDF: %v", err), http.StatusInternalServerError)
		return
	}

	// Extract just the filename from the full path
	pdfFilename := filepath.Base(pdfPath)
	h.logger.Info("Successfully generated PDF: %s at path: %s", pdfFilename, pdfPath)

	// Verify the file exists and is accessible
	if _, err := os.Stat(pdfPath); os.IsNotExist(err) {
		h.logger.Error("Generated PDF file does not exist: %s", pdfPath)
		http.Error(w, "Generated PDF file not found", http.StatusInternalServerError)
		return
	}

	// Check file permissions
	fileInfo, err := os.Stat(pdfPath)
	if err != nil {
		h.logger.Error("Error checking PDF file: %v", err)
	} else {
		h.logger.Debug("PDF file permissions: %v, size: %d bytes", fileInfo.Mode(), fileInfo.Size())
	}

	w.Header().Set("Content-Type", "application/json")
	response := map[string]string{
		"filename": pdfFilename,
		"url":      "/data/pdfs/" + pdfFilename,
	}
	h.logger.Debug("Sending PDF response: %v", response)

	if err := json.NewEncoder(w).Encode(response); err != nil {
		h.logger.Error("Failed to encode PDF response: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
}

// UploadLogoHandler handles logo uploads
func (h *AppHandler) UploadLogoHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse the multipart form
	err := r.ParseMultipartForm(10 << 20) // 10 MB max
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Get the file from the form
	file, handler, err := r.FormFile("logo")
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	defer file.Close()

	// Create the uploads directory if it doesn't exist
	uploadsDir := filepath.Join(h.dataDir, "images")
	if err := os.MkdirAll(uploadsDir, 0755); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Create a new file in the uploads directory
	filename := filepath.Join(uploadsDir, handler.Filename)
	dst, err := os.Create(filename)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer dst.Close()

	// Copy the uploaded file to the destination file
	_, err = dst.ReadFrom(file)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Update the business logo path
	businesses, err := h.dbService.GetBusinesses()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if len(businesses) > 0 {
		business := businesses[0]
		business.LogoPath = filename
		if err := h.dbService.SaveBusiness(&business); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"filename": handler.Filename,
		"path":     filename,
		"url":      "/data/images/" + handler.Filename,
	})
}

// InvoiceByIDHandler handles operations on a specific invoice by ID
func (h *AppHandler) InvoiceByIDHandler(w http.ResponseWriter, r *http.Request) {
	// Extract the invoice ID from the URL
	path := r.URL.Path
	idStr := path[len("/api/invoices/"):]
	id, err := strconv.Atoi(idStr)
	if err != nil {
		http.Error(w, "Invalid invoice ID", http.StatusBadRequest)
		return
	}

	// Handle DELETE requests for deleting invoices
	if r.Method == http.MethodDelete {
		h.logger.Info("Deleting invoice with ID: %d", id)

		if err := h.dbService.DeleteInvoice(id); err != nil {
			h.logger.Error("Failed to delete invoice: %v", err)
			http.Error(w, fmt.Sprintf("Failed to delete invoice: %v", err), http.StatusInternalServerError)
			return
		}

		// Return success response
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": true,
			"message": fmt.Sprintf("Invoice %d deleted successfully", id),
		})
		return
	}

	// Handle PATCH requests for updating invoice status
	if r.Method == http.MethodPatch {
		h.logger.Info("Updating invoice status for invoice ID: %d", id)

		// Parse the request body
		var updateData struct {
			Status string `json:"status"`
		}

		if err := json.NewDecoder(r.Body).Decode(&updateData); err != nil {
			h.logger.Error("Failed to decode status update request: %v", err)
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}

		// Validate the status
		status := updateData.Status
		if status != "draft" && status != "sent" && status != "paid" {
			h.logger.Error("Invalid status value: %s", status)
			http.Error(w, "Invalid status value. Must be 'draft', 'sent', or 'paid'", http.StatusBadRequest)
			return
		}

		// Update the invoice status in the database
		if err := h.dbService.UpdateInvoiceStatus(id, status); err != nil {
			h.logger.Error("Failed to update invoice status: %v", err)
			http.Error(w, "Failed to update invoice status", http.StatusInternalServerError)
			return
		}

		// Return success response
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"id":     id,
			"status": status,
		})
		return
	}

	// Method not allowed for other HTTP methods
	http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
}

// renderTemplate renders a template with the given data
func (h *AppHandler) renderTemplate(w http.ResponseWriter, tmpl string, data map[string]interface{}) {
	t, ok := h.templates[tmpl]
	if !ok {
		http.Error(w, fmt.Sprintf("Template %s not found", tmpl), http.StatusInternalServerError)
		return
	}

	err := t.ExecuteTemplate(w, "layout", data)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// Cleanup performs cleanup operations before shutdown
func (h *AppHandler) Cleanup() error {
	h.logger.Info("Cleaning up resources...")

	// Close database connection
	if h.dbService != nil {
		if err := h.dbService.Close(); err != nil {
			h.logger.Error("Failed to close database: %v", err)
			return fmt.Errorf("failed to close database: %w", err)
		}
		h.logger.Info("Database connection closed")
	}

	return nil
}
