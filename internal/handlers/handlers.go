package handlers

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/0dragosh/simple-invoice/internal/models"
	"github.com/0dragosh/simple-invoice/internal/services"
)

// AppHandler handles HTTP requests
type AppHandler struct {
	dbService     *services.DBService
	vatService    *services.VatService
	pdfService    *services.PDFService
	backupService *services.BackupService
	templates     map[string]*template.Template
	dataDir       string
	logger        *services.Logger
}

// NewAppHandler creates a new AppHandler
func NewAppHandler(dataDir string, logger *services.Logger) (*AppHandler, error) {
	// Create DB service
	dbService, err := services.NewDBService(dataDir, logger)
	if err != nil {
		return nil, fmt.Errorf("failed to create DB service: %w", err)
	}

	// Create VAT service
	vatService := services.NewVatService(logger)

	// Create PDF service
	pdfService := services.NewPDFService(dataDir)

	// Create Backup service
	backupService, err := services.NewBackupService(dbService.GetDB(), dataDir, logger)
	if err != nil {
		return nil, fmt.Errorf("failed to create backup service: %w", err)
	}

	// Start backup scheduler if BACKUP_CRON is set
	backupCron := os.Getenv("BACKUP_CRON")
	if backupCron != "" {
		if err := backupService.StartScheduler(backupCron); err != nil {
			logger.Warn("Failed to start backup scheduler: %v", err)
		}
	}

	// Parse templates
	templates, err := parseTemplates(logger)
	if err != nil {
		return nil, fmt.Errorf("failed to parse templates: %w", err)
	}

	return &AppHandler{
		dbService:     dbService,
		vatService:    vatService,
		pdfService:    pdfService,
		backupService: backupService,
		templates:     templates,
		dataDir:       dataDir,
		logger:        logger,
	}, nil
}

// Helper function to format dates
func formatDate(t time.Time) string {
	return t.Format("2006-01-02")
}

// Helper function to format money
func formatMoney(amount float64) string {
	return fmt.Sprintf("%.2f", amount)
}

// Helper function to format file sizes
func formatFileSize(size int64) string {
	const unit = 1024
	if size < unit {
		return fmt.Sprintf("%d B", size)
	}
	div, exp := int64(unit), 0
	for n := size / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(size)/float64(div), "KMGTPE"[exp])
}

// formatCurrency formats a float as a currency value with 2 decimal places
func formatCurrency(amount float64) string {
	return fmt.Sprintf("%.2f", amount)
}

// currencySymbol returns the symbol for a given currency code
func currencySymbol(currency string) string {
	return services.FormatCurrencySymbol(currency)
}

// add adds two float64 values
func add(a, b float64) float64 {
	return a + b
}

// parseTemplates parses all HTML templates
func parseTemplates(logger *services.Logger) (map[string]*template.Template, error) {
	templates := make(map[string]*template.Template)

	// Define template functions
	funcMap := template.FuncMap{
		"formatDate":     formatDate,
		"formatMoney":    formatMoney,
		"formatFileSize": formatFileSize,
		"formatCurrency": formatCurrency,
		"currencySymbol": currencySymbol,
		"add":            add,
	}

	// Parse base template
	baseTemplate, err := template.New("layout.html").Funcs(funcMap).ParseFiles("internal/templates/layout.html")
	if err != nil {
		return nil, fmt.Errorf("failed to parse base template: %w", err)
	}

	// Parse content templates
	contentTemplates := []string{
		"internal/templates/index.html",
		"internal/templates/business.html",
		"internal/templates/clients.html",
		"internal/templates/invoices.html",
		"internal/templates/create-invoice.html",
		"internal/templates/view-invoice.html",
		"internal/templates/backups.html",
	}

	for _, tmpl := range contentTemplates {
		// Clone the base template
		t, err := baseTemplate.Clone()
		if err != nil {
			return nil, fmt.Errorf("failed to clone base template: %w", err)
		}

		// Parse the content template
		t, err = t.ParseFiles(tmpl)
		if err != nil {
			return nil, fmt.Errorf("failed to parse template %s: %w", tmpl, err)
		}

		// Add to templates map
		name := filepath.Base(tmpl)
		// Remove the .html extension
		name = strings.TrimSuffix(name, ".html")
		templates[name] = t
		logger.Debug("Parsed template: %s", name)
	}

	return templates, nil
}

// RegisterHandlers registers all HTTP handlers
func RegisterHandlers(mux *http.ServeMux, dataDir string, logger *services.Logger) (*AppHandler, error) {
	handler, err := NewAppHandler(dataDir, logger)
	if err != nil {
		return nil, err
	}

	// Register page handlers
	mux.HandleFunc("/", handler.IndexHandler)
	mux.HandleFunc("/business", handler.BusinessHandler)
	mux.HandleFunc("/clients", handler.ClientsHandler)
	mux.HandleFunc("/invoices", handler.InvoicesHandler)
	mux.HandleFunc("/invoices/create", handler.CreateInvoiceHandler)
	mux.HandleFunc("/invoices/view/", handler.ViewInvoiceHandler)
	mux.HandleFunc("/backups", handler.BackupsHandler)

	// Register API handlers
	mux.HandleFunc("/api/business", handler.BusinessAPIHandler)
	mux.HandleFunc("/api/clients", handler.ClientsAPIHandler)
	mux.HandleFunc("/api/clients/", handler.ClientsAPIHandler)
	mux.HandleFunc("/api/clients/vat-lookup", handler.VatLookupHandler)
	mux.HandleFunc("/api/clients/uk-company-lookup", handler.UKCompanyLookupHandler)
	mux.HandleFunc("/api/invoices", handler.InvoicesAPIHandler)
	mux.HandleFunc("/api/invoices/", handler.InvoiceByIDHandler)
	mux.HandleFunc("/api/invoices/generate-pdf/", handler.GeneratePDFHandler)
	mux.HandleFunc("/api/invoices/preview-pdf", handler.PreviewPDFHandler)
	mux.HandleFunc("/api/upload/logo", handler.UploadLogoHandler)
	mux.HandleFunc("/api/backups", handler.BackupsAPIHandler)
	mux.HandleFunc("/api/backups/restore", handler.RestoreBackupHandler)

	// Register static file handler
	fileServer := http.FileServer(http.Dir(dataDir))
	mux.Handle("/data/", http.StripPrefix("/data/", fileServer))

	// Log the data directory and static file paths
	logger.Info("Data directory: %s", dataDir)
	logger.Info("Static files will be served from: %s", dataDir)
	logger.Info("PDFs will be available at: /data/pdfs/")

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

		// Handle DELETE request for a specific client
		if r.Method == http.MethodDelete {
			h.logger.Info("Received request to delete client with ID: %d", clientID)

			if err := h.dbService.DeleteClient(clientID); err != nil {
				h.logger.Error("Failed to delete client: %v", err)
				http.Error(w, fmt.Sprintf("Failed to delete client: %v", err), http.StatusInternalServerError)
				return
			}

			h.logger.Info("Successfully deleted client with ID: %d", clientID)
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]string{"message": "Client deleted successfully"})
			return
		}

		// Handle GET request for a specific client
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
		w.Header().Set("Content-Type", "application/json")

		var client models.Client
		h.logger.Debug("Decoding client JSON from request body")
		if err := json.NewDecoder(r.Body).Decode(&client); err != nil {
			h.logger.Error("Failed to decode client JSON: %v", err)
			http.Error(w, fmt.Sprintf("Invalid client data: %v", err), http.StatusBadRequest)
			return
		}

		// If no created date is provided, use current date
		if client.CreatedDate == nil {
			now := time.Now()
			client.CreatedDate = &now
			h.logger.Debug("No created date provided, using current time: %v", now)
		}

		h.logger.Info("Processing client with ID: %d, Name: %s, VAT ID: %s, Country: %s",
			client.ID, client.Name, client.VatID, client.Country)

		// Special handling for UK VAT IDs
		if strings.HasPrefix(strings.ToUpper(client.VatID), "GB") {
			h.logger.Info("UK VAT ID detected: %s", client.VatID)

			// Ensure country is set to GB for UK VAT IDs
			if client.Country != "GB" {
				h.logger.Info("Setting country to GB for UK VAT ID")
				client.Country = "GB"
			}
		}

		h.logger.Debug("Saving client to database: %+v", client)
		if err := h.dbService.SaveClient(&client); err != nil {
			h.logger.Error("Failed to save client: %v", err)
			http.Error(w, fmt.Sprintf("Failed to save client: %v", err), http.StatusInternalServerError)
			return
		}

		h.logger.Info("Successfully saved client: %s with ID: %d", client.Name, client.ID)
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

		// Read the request body for logging
		bodyBytes, err := io.ReadAll(r.Body)
		if err != nil {
			h.logger.Error("Failed to read request body: %v", err)
			http.Error(w, fmt.Sprintf("Failed to read request body: %v", err), http.StatusBadRequest)
			return
		}

		// Log the raw request body for debugging
		h.logger.Debug("Raw request body: %s", string(bodyBytes))

		// Create a new reader from the bytes for further processing
		r.Body = io.NopCloser(bytes.NewReader(bodyBytes))

		// First, decode the raw JSON to handle date strings manually
		var rawRequest map[string]json.RawMessage
		if err := json.NewDecoder(r.Body).Decode(&rawRequest); err != nil {
			h.logger.Error("Failed to decode invoice JSON: %v", err)
			http.Error(w, fmt.Sprintf("Invalid invoice data: %v", err), http.StatusBadRequest)
			return
		}

		// Check if required fields exist in the request
		if _, ok := rawRequest["invoice"]; !ok {
			h.logger.Error("Missing 'invoice' field in request")
			http.Error(w, "Missing 'invoice' field in request", http.StatusBadRequest)
			return
		}

		if _, ok := rawRequest["items"]; !ok {
			h.logger.Error("Missing 'items' field in request")
			http.Error(w, "Missing 'items' field in request", http.StatusBadRequest)
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

		// Log the parsed data for debugging
		h.logger.Debug("Parsed invoice data: %+v", rawInvoice)
		h.logger.Debug("Parsed items: %+v", items)

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
			Currency:         rawInvoice["currency"].(string),
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

		// Automatically generate PDF for the new invoice
		go func() {
			// Create a context with timeout for PDF generation
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()

			// Create a channel to receive errors
			errCh := make(chan error, 1)

			// Run PDF generation in a goroutine
			go func() {
				h.logger.Info("Automatically generating PDF for invoice ID: %d", invoice.ID)

				// Get the necessary data for PDF generation
				savedInvoice, savedItems, err := h.dbService.GetInvoice(invoice.ID)
				if err != nil {
					h.logger.Error("Failed to get invoice for automatic PDF generation: %v", err)
					errCh <- fmt.Errorf("failed to get invoice: %w", err)
					return
				}

				business, err := h.dbService.GetBusiness(savedInvoice.BusinessID)
				if err != nil {
					h.logger.Error("Failed to get business for automatic PDF generation: %v", err)
					errCh <- fmt.Errorf("failed to get business: %w", err)
					return
				}

				client, err := h.dbService.GetClient(savedInvoice.ClientID)
				if err != nil {
					h.logger.Error("Failed to get client for automatic PDF generation: %v", err)
					errCh <- fmt.Errorf("failed to get client: %w", err)
					return
				}

				// Ensure the pdfs directory exists
				pdfsDir := filepath.Join(h.dataDir, "pdfs")
				if err := os.MkdirAll(pdfsDir, 0755); err != nil {
					h.logger.Error("Failed to create pdfs directory for automatic generation: %v", err)
					errCh <- fmt.Errorf("failed to create pdfs directory: %w", err)
					return
				}

				// Generate the PDF
				pdfPath, err := h.pdfService.GenerateInvoice(savedInvoice, business, client, savedItems)
				if err != nil {
					h.logger.Error("Failed to automatically generate PDF: %v", err)
					errCh <- fmt.Errorf("failed to generate PDF: %w", err)
					return
				}

				// Verify the file exists and is accessible
				if _, err := os.Stat(pdfPath); os.IsNotExist(err) {
					h.logger.Error("Generated PDF file does not exist: %s", pdfPath)
					errCh <- fmt.Errorf("generated PDF file not found: %s", pdfPath)
					return
				}

				// Extract just the filename from the full path
				pdfFilename := filepath.Base(pdfPath)
				h.logger.Info("Successfully generated PDF: %s at path: %s", pdfFilename, pdfPath)
				errCh <- nil
			}()

			// Wait for either the PDF generation to complete or the context to timeout
			select {
			case err := <-errCh:
				if err != nil {
					h.logger.Error("PDF generation failed: %v", err)
				}
			case <-ctx.Done():
				h.logger.Error("PDF generation timed out after 30 seconds")
			}
		}()

		// Return the created invoice to the client
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

	// Set the correct URL for the PDF file
	pdfURL := fmt.Sprintf("/data/pdfs/%s", pdfFilename)
	h.logger.Debug("PDF URL: %s", pdfURL)

	w.Header().Set("Content-Type", "application/json")
	response := map[string]string{
		"filename": pdfFilename,
		"url":      pdfURL,
	}
	h.logger.Debug("Sending PDF response: %v", response)

	if err := json.NewEncoder(w).Encode(response); err != nil {
		h.logger.Error("Failed to encode PDF response: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
}

// PreviewPDFHandler generates a PDF preview based on form data
func (h *AppHandler) PreviewPDFHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		h.logger.Warn("Method not allowed for PDF preview: %s", r.Method)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	h.logger.Info("Generating PDF preview")

	// First parse the raw JSON data to access the date fields
	var rawData map[string]json.RawMessage
	if err := json.NewDecoder(r.Body).Decode(&rawData); err != nil {
		h.logger.Error("Failed to decode raw preview data: %v", err)
		http.Error(w, "Invalid request data", http.StatusBadRequest)
		return
	}

	// Parse the invoice data to access date fields
	var rawInvoice map[string]interface{}
	if err := json.Unmarshal(rawData["invoice"], &rawInvoice); err != nil {
		h.logger.Error("Failed to parse invoice data: %v", err)
		http.Error(w, "Invalid invoice data", http.StatusBadRequest)
		return
	}

	// Create the preview data structure
	var previewData struct {
		Invoice  models.Invoice       `json:"invoice"`
		Items    []models.InvoiceItem `json:"items"`
		Business models.Business      `json:"business"`
		Client   models.Client        `json:"client"`
	}

	// Parse the rest of the data
	if err := json.Unmarshal(rawData["business"], &previewData.Business); err != nil {
		h.logger.Error("Failed to decode business data: %v", err)
		http.Error(w, "Invalid business data", http.StatusBadRequest)
		return
	}

	if err := json.Unmarshal(rawData["client"], &previewData.Client); err != nil {
		h.logger.Error("Failed to decode client data: %v", err)
		http.Error(w, "Invalid client data", http.StatusBadRequest)
		return
	}

	if err := json.Unmarshal(rawData["items"], &previewData.Items); err != nil {
		h.logger.Error("Failed to decode items data: %v", err)
		http.Error(w, "Invalid items data", http.StatusBadRequest)
		return
	}

	// Fill in the invoice fields
	previewData.Invoice.ID = int(rawInvoice["id"].(float64))
	previewData.Invoice.InvoiceNumber = rawInvoice["invoice_number"].(string)
	previewData.Invoice.BusinessID = int(rawInvoice["business_id"].(float64))
	previewData.Invoice.ClientID = int(rawInvoice["client_id"].(float64))
	previewData.Invoice.HourlyRate = rawInvoice["hourly_rate"].(float64)
	previewData.Invoice.HoursWorked = rawInvoice["hours_worked"].(float64)
	previewData.Invoice.TotalAmount = rawInvoice["total_amount"].(float64)
	previewData.Invoice.VatRate = rawInvoice["vat_rate"].(float64)
	previewData.Invoice.VatAmount = rawInvoice["vat_amount"].(float64)
	previewData.Invoice.ReverseChargeVat = rawInvoice["reverse_charge_vat"].(bool)
	previewData.Invoice.Currency = rawInvoice["currency"].(string)
	previewData.Invoice.Notes = rawInvoice["notes"].(string)
	previewData.Invoice.Status = rawInvoice["status"].(string)

	// Handle date parsing
	issueDateStr, ok := rawInvoice["issue_date"].(string)
	if !ok {
		h.logger.Error("Issue date is missing or not a string")
		http.Error(w, "Issue date is required", http.StatusBadRequest)
		return
	}

	// Try parsing with the simple date format first
	issueDate, err := time.Parse("2006-01-02", issueDateStr)
	if err != nil {
		// If that fails, try the full timestamp format
		issueDate, err = time.Parse(time.RFC3339, issueDateStr)
		if err != nil {
			h.logger.Error("Failed to parse issue date: %v", err)
			http.Error(w, "Invalid issue date format", http.StatusBadRequest)
			return
		}
	}
	previewData.Invoice.IssueDate = issueDate

	dueDateStr, ok := rawInvoice["due_date"].(string)
	if !ok {
		h.logger.Error("Due date is missing or not a string")
		http.Error(w, "Due date is required", http.StatusBadRequest)
		return
	}

	// Try parsing with the simple date format first
	dueDate, err := time.Parse("2006-01-02", dueDateStr)
	if err != nil {
		// If that fails, try the full timestamp format
		dueDate, err = time.Parse(time.RFC3339, dueDateStr)
		if err != nil {
			h.logger.Error("Failed to parse due date: %v", err)
			http.Error(w, "Invalid due date format", http.StatusBadRequest)
			return
		}
	}
	previewData.Invoice.DueDate = dueDate

	// Ensure the pdfs directory exists
	pdfsDir := filepath.Join(h.dataDir, "pdfs", "previews")
	if err := os.MkdirAll(pdfsDir, 0755); err != nil {
		h.logger.Error("Failed to create pdfs preview directory: %v", err)
		http.Error(w, "Failed to create preview directory", http.StatusInternalServerError)
		return
	}

	// Create a unique preview filename using a timestamp
	previewID := fmt.Sprintf("preview-%d", time.Now().UnixNano())
	previewData.Invoice.InvoiceNumber = previewID

	// Generate the PDF
	pdfPath, err := h.pdfService.GenerateInvoice(&previewData.Invoice, &previewData.Business, &previewData.Client, previewData.Items)
	if err != nil {
		h.logger.Error("Failed to generate preview PDF: %v", err)
		http.Error(w, "Failed to generate preview", http.StatusInternalServerError)
		return
	}

	// Extract just the filename from the full path
	pdfFilename := filepath.Base(pdfPath)
	pdfURL := fmt.Sprintf("/data/pdfs/%s", pdfFilename)
	h.logger.Info("Generated preview PDF: %s", pdfURL)

	// Return the PDF URL
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"url": pdfURL,
	})
}

// UploadLogoHandler handles logo uploads
func (h *AppHandler) UploadLogoHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		h.logger.Warn("Method not allowed for logo upload: %s", r.Method)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse the multipart form
	err := r.ParseMultipartForm(10 << 20) // 10 MB max
	if err != nil {
		h.logger.Error("Failed to parse multipart form: %v", err)
		http.Error(w, fmt.Sprintf("Failed to parse form: %v", err), http.StatusBadRequest)
		return
	}

	// Get the file from the form
	file, handler, err := r.FormFile("logo")
	if err != nil {
		h.logger.Error("Failed to get logo file from form: %v", err)
		http.Error(w, fmt.Sprintf("Failed to get logo file: %v", err), http.StatusBadRequest)
		return
	}
	defer file.Close()

	h.logger.Debug("Received logo upload: %s, size: %d bytes, content type: %s",
		handler.Filename, handler.Size, handler.Header.Get("Content-Type"))

	// Validate file type
	contentType := handler.Header.Get("Content-Type")
	if !strings.HasPrefix(contentType, "image/") {
		h.logger.Error("Invalid file type: %s", contentType)
		http.Error(w, "Invalid file type. Only images are allowed.", http.StatusBadRequest)
		return
	}

	// Create the uploads directory if it doesn't exist
	uploadsDir := filepath.Join(h.dataDir, "images")
	h.logger.Debug("Ensuring uploads directory exists: %s", uploadsDir)
	if err := os.MkdirAll(uploadsDir, 0755); err != nil {
		h.logger.Error("Failed to create uploads directory: %v", err)
		http.Error(w, fmt.Sprintf("Failed to create uploads directory: %v", err), http.StatusInternalServerError)
		return
	}

	// Create a new file in the uploads directory
	filename := filepath.Join(uploadsDir, handler.Filename)
	h.logger.Debug("Creating destination file: %s", filename)
	dst, err := os.Create(filename)
	if err != nil {
		h.logger.Error("Failed to create destination file: %v", err)
		http.Error(w, fmt.Sprintf("Failed to create destination file: %v", err), http.StatusInternalServerError)
		return
	}
	defer dst.Close()

	// Copy the uploaded file to the destination file
	h.logger.Debug("Copying uploaded file to destination")
	bytesWritten, err := io.Copy(dst, file)
	if err != nil {
		h.logger.Error("Failed to copy uploaded file: %v", err)
		http.Error(w, fmt.Sprintf("Failed to save uploaded file: %v", err), http.StatusInternalServerError)
		return
	}

	h.logger.Info("Successfully saved logo to: %s (%d bytes written)", filename, bytesWritten)

	// Verify the file exists and is readable
	if _, err := os.Stat(filename); os.IsNotExist(err) {
		h.logger.Error("File was not saved correctly: %v", err)
		http.Error(w, "Failed to save logo file", http.StatusInternalServerError)
		return
	}

	// Update the business logo path
	businesses, err := h.dbService.GetBusinesses()
	if err != nil {
		h.logger.Error("Failed to get businesses: %v", err)
		http.Error(w, fmt.Sprintf("Failed to get business details: %v", err), http.StatusInternalServerError)
		return
	}

	if len(businesses) > 0 {
		business := businesses[0]
		// Store only the filename, not the full path
		business.LogoPath = filepath.Base(handler.Filename)
		h.logger.Debug("Updating business with logo path: %s", business.LogoPath)
		if err := h.dbService.SaveBusiness(&business); err != nil {
			h.logger.Error("Failed to save business with logo: %v", err)
			http.Error(w, fmt.Sprintf("Failed to update business with logo: %v", err), http.StatusInternalServerError)
			return
		}
		h.logger.Info("Updated business with logo path: %s", business.LogoPath)
	} else {
		h.logger.Warn("No business found to update with logo")
	}

	// Return success response
	w.Header().Set("Content-Type", "application/json")
	response := map[string]string{
		"filename": handler.Filename,
		"path":     filename,
		"url":      "/data/images/" + filepath.Base(handler.Filename),
		"message":  "Logo uploaded successfully",
	}
	h.logger.Debug("Sending logo upload response: %v", response)

	if err := json.NewEncoder(w).Encode(response); err != nil {
		h.logger.Error("Failed to encode response: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
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
	// Get the template
	t, ok := h.templates[tmpl]
	if !ok {
		h.logger.Error("Template not found: %s", tmpl)
		http.Error(w, fmt.Sprintf("Template not found: %s", tmpl), http.StatusInternalServerError)
		return
	}

	// Render the template
	if err := t.ExecuteTemplate(w, "layout", data); err != nil {
		h.logger.Error("Failed to render template: %v", err)
		http.Error(w, fmt.Sprintf("Failed to render template: %v", err), http.StatusInternalServerError)
		return
	}
}

// Cleanup performs cleanup tasks before application shutdown
func (h *AppHandler) Cleanup() error {
	h.logger.Info("Performing cleanup tasks")

	// Stop the backup scheduler
	if h.backupService != nil {
		h.backupService.StopScheduler()
	}

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
