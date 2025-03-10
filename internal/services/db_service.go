package services

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/0dragosh/simple-invoice/internal/models"
	_ "github.com/mattn/go-sqlite3"
)

// DBService provides methods for database operations
type DBService struct {
	db      *sql.DB
	dataDir string
	logger  *Logger
}

// NewDBService creates a new DBService
func NewDBService(dataDir string, logger *Logger) (*DBService, error) {
	logger.Info("Initializing database service...")

	// Ensure data directory exists
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		logger.Error("Failed to create data directory: %v", err)
		return nil, fmt.Errorf("failed to create data directory: %w", err)
	}
	logger.Debug("Data directory ensured: %s", dataDir)

	// Open database
	dbPath := filepath.Join(dataDir, "database.db")
	logger.Debug("Database path: %s", dbPath)

	// Check if database file exists and is locked
	if _, err := os.Stat(dbPath); err == nil {
		logger.Debug("Database file exists, checking for lock files")

		// Try to remove any stale lock files
		lockPath := dbPath + "-shm"
		if _, err := os.Stat(lockPath); err == nil {
			logger.Warn("Found stale SHM lock file, attempting to remove: %s", lockPath)
			if err := os.Remove(lockPath); err != nil {
				logger.Warn("Failed to remove SHM lock file: %v", err)
			}
		}

		lockPath = dbPath + "-wal"
		if _, err := os.Stat(lockPath); err == nil {
			logger.Warn("Found stale WAL lock file, attempting to remove: %s", lockPath)
			if err := os.Remove(lockPath); err != nil {
				logger.Warn("Failed to remove WAL lock file: %v", err)
			}
		}

		// If database file exists but might be corrupted, try to remove it
		logger.Debug("Checking if database file is accessible")
		file, err := os.Open(dbPath)
		if err != nil {
			logger.Warn("Database file exists but cannot be opened: %v", err)
			logger.Warn("Attempting to remove potentially corrupted database file")
			if err := os.Remove(dbPath); err != nil {
				logger.Error("Failed to remove database file: %v", err)
				return nil, fmt.Errorf("failed to remove corrupted database file: %w", err)
			}
			logger.Info("Removed potentially corrupted database file")
		} else {
			file.Close()
		}
	} else {
		logger.Debug("Database file does not exist, will be created")
	}

	// Use a connection with strict timeout and no journal
	logger.Debug("Opening database connection with timeout")
	db, err := sql.Open("sqlite3", dbPath+"?_timeout=5000&_journal=DELETE")
	if err != nil {
		logger.Error("Failed to open database: %v", err)
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Set connection parameters
	db.SetMaxOpenConns(1) // Restrict to a single connection to avoid locks
	db.SetConnMaxLifetime(30 * time.Second)
	db.SetMaxIdleConns(1)

	// Verify connection with timeout
	logger.Debug("Verifying database connection")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := db.PingContext(ctx); err != nil {
		db.Close()
		logger.Error("Failed to ping database: %v", err)

		// If database is locked or corrupted, try to remove it and recreate
		if strings.Contains(err.Error(), "database is locked") ||
			strings.Contains(err.Error(), "disk I/O error") ||
			strings.Contains(err.Error(), "database disk image is malformed") {
			logger.Warn("Database is locked or corrupted, attempting to remove and recreate")
			os.Remove(dbPath)
			os.Remove(dbPath + "-shm")
			os.Remove(dbPath + "-wal")

			// Reopen database with simpler settings
			logger.Debug("Reopening database after removal")
			db, err = sql.Open("sqlite3", dbPath)
			if err != nil {
				logger.Error("Failed to reopen database after lock: %v", err)
				return nil, fmt.Errorf("failed to reopen database after lock: %w", err)
			}

			// Verify connection again with timeout
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			if err := db.PingContext(ctx); err != nil {
				db.Close()
				logger.Error("Failed to ping database after recreation: %v", err)
				return nil, fmt.Errorf("failed to ping database after recreation: %w", err)
			}
			logger.Info("Successfully recreated database")
		} else {
			return nil, fmt.Errorf("failed to ping database: %w", err)
		}
	}
	logger.Debug("Database connection verified")

	service := &DBService{
		db:      db,
		dataDir: dataDir,
		logger:  logger,
	}

	// Initialize database with timeout
	logger.Debug("Initializing database schema")
	ctx2, cancel2 := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel2()

	initErr := make(chan error, 1)
	go func() {
		initErr <- service.initDB()
	}()

	select {
	case err := <-initErr:
		if err != nil {
			db.Close()
			logger.Error("Failed to initialize database: %v", err)
			return nil, fmt.Errorf("failed to initialize database: %w", err)
		}
	case <-ctx2.Done():
		db.Close()
		logger.Error("Database initialization timed out after 10 seconds")
		return nil, fmt.Errorf("database initialization timed out")
	}
	logger.Debug("Database schema initialized")

	// Ensure invoice_items table exists with timeout
	logger.Debug("Ensuring invoice_items table exists")
	ctx3, cancel3 := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel3()

	ensureErr := make(chan error, 1)
	go func() {
		ensureErr <- service.EnsureInvoiceItemsTable()
	}()

	select {
	case err := <-ensureErr:
		if err != nil {
			db.Close()
			logger.Error("Failed to ensure invoice_items table exists: %v", err)
			return nil, fmt.Errorf("failed to ensure invoice_items table exists: %w", err)
		}
	case <-ctx3.Done():
		db.Close()
		logger.Error("Ensuring invoice_items table timed out after 5 seconds")
		return nil, fmt.Errorf("ensuring invoice_items table timed out")
	}
	logger.Debug("Invoice items table ensured")

	logger.Info("Database service initialized successfully")
	return service, nil
}

// GetDataDir returns the data directory path
func (s *DBService) GetDataDir() string {
	return s.dataDir
}

// Close closes the database connection
func (s *DBService) Close() error {
	s.logger.Info("Closing database connection")

	// Execute PRAGMA to optimize database before closing
	_, err := s.db.Exec("PRAGMA optimize")
	if err != nil {
		s.logger.Warn("Failed to optimize database: %v", err)
	}

	// Execute VACUUM to compact the database
	_, err = s.db.Exec("VACUUM")
	if err != nil {
		s.logger.Warn("Failed to vacuum database: %v", err)
	}

	// Close the database connection
	if err := s.db.Close(); err != nil {
		s.logger.Error("Failed to close database: %v", err)
		return fmt.Errorf("failed to close database: %w", err)
	}

	return nil
}

// initDB initializes the database schema
func (s *DBService) initDB() error {
	s.logger.Debug("Starting database initialization")

	// Create businesses table
	s.logger.Debug("Creating businesses table if not exists")
	_, err := s.db.Exec(`
		CREATE TABLE IF NOT EXISTS businesses (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL,
			address TEXT NOT NULL,
			city TEXT NOT NULL,
			postal_code TEXT NOT NULL,
			country TEXT NOT NULL,
			vat_id TEXT NOT NULL,
			email TEXT NOT NULL,
			bank_name TEXT NOT NULL,
			bank_account TEXT NOT NULL,
			iban TEXT NOT NULL,
			bic TEXT NOT NULL,
			logo_path TEXT
		)
	`)
	if err != nil {
		s.logger.Error("Failed to create businesses table: %v", err)
		return fmt.Errorf("failed to create businesses table: %w", err)
	}

	// Create clients table
	s.logger.Debug("Creating clients table if not exists")
	_, err = s.db.Exec(`
		CREATE TABLE IF NOT EXISTS clients (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL,
			address TEXT NOT NULL,
			city TEXT NOT NULL,
			postal_code TEXT NOT NULL,
			country TEXT NOT NULL,
			vat_id TEXT NOT NULL,
			created_date TIMESTAMP,
			deleted INTEGER DEFAULT 0
		)
	`)
	if err != nil {
		s.logger.Error("Failed to create clients table: %v", err)
		return fmt.Errorf("failed to create clients table: %w", err)
	}

	// Check if we need to add the deleted column to the clients table
	s.logger.Debug("Checking if deleted column exists in clients table")
	var deletedColumnExists bool
	err = s.db.QueryRow(`
		SELECT COUNT(*) > 0
		FROM pragma_table_info('clients')
		WHERE name = 'deleted'
	`).Scan(&deletedColumnExists)
	if err != nil {
		s.logger.Error("Failed to check if deleted column exists: %v", err)
		return fmt.Errorf("failed to check if deleted column exists: %w", err)
	}

	if !deletedColumnExists {
		s.logger.Info("Adding deleted column to clients table")
		_, err = s.db.Exec(`ALTER TABLE clients ADD COLUMN deleted INTEGER DEFAULT 0`)
		if err != nil {
			s.logger.Error("Failed to add deleted column: %v", err)
			return fmt.Errorf("failed to add deleted column: %w", err)
		}
		s.logger.Info("Successfully added deleted column to clients table")
	}

	// Check if we need to remove the company_number column from the clients table
	s.logger.Debug("Checking if company_number column exists in clients table")
	var companyNumberColumnExists bool
	err = s.db.QueryRow(`
		SELECT COUNT(*) > 0
		FROM pragma_table_info('clients')
		WHERE name = 'company_number'
	`).Scan(&companyNumberColumnExists)
	if err != nil {
		s.logger.Error("Failed to check if company_number column exists: %v", err)
		return fmt.Errorf("failed to check if company_number column exists: %w", err)
	}

	if companyNumberColumnExists {
		s.logger.Info("Removing company_number column from clients table")

		// SQLite doesn't support DROP COLUMN directly, so we need to:
		// 1. Create a new table without the column
		// 2. Copy the data
		// 3. Drop the old table
		// 4. Rename the new table

		_, err = s.db.Exec(`
			BEGIN TRANSACTION;
			
			-- Create new table without company_number
			CREATE TABLE clients_new (
				id INTEGER PRIMARY KEY AUTOINCREMENT,
				name TEXT NOT NULL,
				address TEXT NOT NULL,
				city TEXT NOT NULL,
				postal_code TEXT NOT NULL,
				country TEXT NOT NULL,
				vat_id TEXT NOT NULL,
				created_date TIMESTAMP
			);
			
			-- Copy data from old table to new table
			INSERT INTO clients_new (id, name, address, city, postal_code, country, vat_id)
			SELECT id, name, address, city, postal_code, country, vat_id FROM clients;
			
			-- Drop old table
			DROP TABLE clients;
			
			-- Rename new table to original name
			ALTER TABLE clients_new RENAME TO clients;
			
			COMMIT;
		`)
		if err != nil {
			s.logger.Error("Failed to remove company_number column: %v", err)
			return fmt.Errorf("failed to remove company_number column: %w", err)
		}

		s.logger.Info("Successfully removed company_number column from clients table")
	}

	// Create invoices table
	s.logger.Debug("Creating invoices table if not exists")
	_, err = s.db.Exec(`
		CREATE TABLE IF NOT EXISTS invoices (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			invoice_number TEXT NOT NULL,
			business_id INTEGER NOT NULL,
			client_id INTEGER NOT NULL,
			issue_date TEXT NOT NULL,
			due_date TEXT NOT NULL,
			hourly_rate REAL NOT NULL,
			hours_worked REAL NOT NULL,
			total_amount REAL NOT NULL,
			vat_rate REAL NOT NULL,
			vat_amount REAL NOT NULL,
			reverse_charge_vat INTEGER NOT NULL,
			currency TEXT DEFAULT 'EUR',
			notes TEXT,
			status TEXT NOT NULL,
			FOREIGN KEY (business_id) REFERENCES businesses (id),
			FOREIGN KEY (client_id) REFERENCES clients (id)
		)
	`)
	if err != nil {
		s.logger.Error("Failed to create invoices table: %v", err)
		return fmt.Errorf("failed to create invoices table: %w", err)
	}

	// Create invoice_items table
	s.logger.Debug("Creating invoice_items table if not exists")
	_, err = s.db.Exec(`
		CREATE TABLE IF NOT EXISTS invoice_items (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			invoice_id INTEGER NOT NULL,
			description TEXT NOT NULL,
			quantity REAL NOT NULL,
			unit_price REAL NOT NULL,
			amount REAL NOT NULL,
			FOREIGN KEY (invoice_id) REFERENCES invoices (id) ON DELETE CASCADE
		)
	`)
	if err != nil {
		s.logger.Error("Failed to create invoice_items table: %v", err)
		return fmt.Errorf("failed to create invoice_items table: %w", err)
	}

	// Check if we need to add the currency column to the invoices table
	s.logger.Debug("Checking if currency column exists in invoices table")
	var currencyColumnExists bool
	err = s.db.QueryRow(`
		SELECT COUNT(*) > 0
		FROM pragma_table_info('invoices')
		WHERE name = 'currency'
	`).Scan(&currencyColumnExists)
	if err != nil {
		s.logger.Error("Failed to check if currency column exists: %v", err)
		return fmt.Errorf("failed to check if currency column exists: %w", err)
	}

	if !currencyColumnExists {
		s.logger.Info("Adding currency column to invoices table")
		_, err = s.db.Exec(`ALTER TABLE invoices ADD COLUMN currency TEXT DEFAULT 'EUR'`)
		if err != nil {
			s.logger.Error("Failed to add currency column: %v", err)
			return fmt.Errorf("failed to add currency column: %w", err)
		}
	}

	// Check if we need to add the email column to the businesses table
	s.logger.Debug("Checking if email column exists in businesses table")
	var emailColumnExists bool
	err = s.db.QueryRow(`
		SELECT COUNT(*) > 0
		FROM pragma_table_info('businesses')
		WHERE name = 'email'
	`).Scan(&emailColumnExists)
	if err != nil {
		s.logger.Error("Failed to check if email column exists: %v", err)
		return fmt.Errorf("failed to check if email column exists: %w", err)
	}

	if !emailColumnExists {
		s.logger.Info("Added email column to businesses table")
		_, err = s.db.Exec(`ALTER TABLE businesses ADD COLUMN email TEXT DEFAULT ''`)
		if err != nil {
			s.logger.Error("Failed to add email column: %v", err)
			return fmt.Errorf("failed to add email column: %w", err)
		}
	}

	s.logger.Debug("Database initialization completed successfully")
	return nil
}

// Business methods

// SaveBusiness saves a business to the database
func (s *DBService) SaveBusiness(business *models.Business) error {
	if business.ID == 0 {
		// Insert new business
		result, err := s.db.Exec(`
			INSERT INTO businesses (name, address, city, postal_code, country, vat_id, email, bank_name, bank_account, iban, bic, logo_path)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		`,
			business.Name, business.Address, business.City, business.PostalCode, business.Country,
			business.VatID, business.Email, business.BankName, business.BankAccount, business.IBAN, business.BIC, business.LogoPath,
		)
		if err != nil {
			return err
		}

		id, err := result.LastInsertId()
		if err != nil {
			return err
		}

		business.ID = int(id)
	} else {
		// Update existing business
		_, err := s.db.Exec(`
			UPDATE businesses
			SET name = ?, address = ?, city = ?, postal_code = ?, country = ?, vat_id = ?, email = ?, bank_name = ?, bank_account = ?, iban = ?, bic = ?, logo_path = ?
			WHERE id = ?
		`,
			business.Name, business.Address, business.City, business.PostalCode, business.Country,
			business.VatID, business.Email, business.BankName, business.BankAccount, business.IBAN, business.BIC, business.LogoPath,
			business.ID,
		)
		if err != nil {
			return err
		}
	}

	return nil
}

// GetBusiness retrieves a business from the database
func (s *DBService) GetBusiness(id int) (*models.Business, error) {
	// Create a context with timeout for database operations
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	s.logger.Info("Fetching business with ID: %d", id)

	var business models.Business
	err := s.db.QueryRowContext(ctx, `
		SELECT id, name, address, city, postal_code, country, vat_id, email, bank_name, bank_account, iban, bic, logo_path
		FROM businesses
		WHERE id = ?
	`, id).Scan(
		&business.ID,
		&business.Name,
		&business.Address,
		&business.City,
		&business.PostalCode,
		&business.Country,
		&business.VatID,
		&business.Email,
		&business.BankName,
		&business.BankAccount,
		&business.IBAN,
		&business.BIC,
		&business.LogoPath,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			s.logger.Warn("No business found with ID: %d", id)
		} else {
			s.logger.Error("Database error when fetching business ID %d: %v", id, err)
		}
		return nil, err
	}

	s.logger.Debug("Successfully fetched business: %s (ID: %d)", business.Name, business.ID)
	return &business, nil
}

// GetBusinesses retrieves all businesses from the database
func (s *DBService) GetBusinesses() ([]models.Business, error) {
	rows, err := s.db.Query(`
		SELECT id, name, address, city, postal_code, country, vat_id, email, bank_name, bank_account, iban, bic, logo_path
		FROM businesses
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var businesses []models.Business
	for rows.Next() {
		var business models.Business
		err := rows.Scan(
			&business.ID, &business.Name, &business.Address, &business.City, &business.PostalCode,
			&business.Country, &business.VatID, &business.Email, &business.BankName, &business.BankAccount,
			&business.IBAN, &business.BIC, &business.LogoPath,
		)
		if err != nil {
			return nil, err
		}
		businesses = append(businesses, business)
	}

	return businesses, nil
}

// Client methods

// SaveClient saves a client to the database
func (s *DBService) SaveClient(client *models.Client) error {
	// No validation for VAT ID - accept as provided
	s.logger.Debug("SaveClient called with client: %+v", client)

	// Ensure created_date is not nil
	if client.CreatedDate == nil {
		now := time.Now()
		client.CreatedDate = &now
		s.logger.Debug("Created date was nil, using current time: %v", now)
	}

	if client.ID == 0 {
		// Insert new client
		s.logger.Debug("Inserting new client: %s", client.Name)
		result, err := s.db.Exec(`
			INSERT INTO clients (name, address, city, postal_code, country, vat_id, created_date, deleted)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		`, client.Name, client.Address, client.City, client.PostalCode, client.Country, client.VatID, client.CreatedDate, boolToInt(client.Deleted))
		if err != nil {
			s.logger.Error("Failed to insert client: %v", err)
			return err
		}

		id, err := result.LastInsertId()
		if err != nil {
			s.logger.Error("Failed to get last insert ID: %v", err)
			return err
		}

		client.ID = int(id)
		s.logger.Info("Successfully inserted client with ID: %d", client.ID)
	} else {
		// Update existing client
		s.logger.Debug("Updating existing client with ID: %d", client.ID)
		_, err := s.db.Exec(`
			UPDATE clients
			SET name = ?, address = ?, city = ?, postal_code = ?, country = ?, vat_id = ?, created_date = ?, deleted = ?
			WHERE id = ?
		`, client.Name, client.Address, client.City, client.PostalCode, client.Country, client.VatID, client.CreatedDate, boolToInt(client.Deleted), client.ID)
		if err != nil {
			s.logger.Error("Failed to update client: %v", err)
			return err
		}
		s.logger.Info("Successfully updated client with ID: %d", client.ID)
	}

	return nil
}

// GetClient retrieves a client from the database
func (s *DBService) GetClient(id int) (*models.Client, error) {
	// Create a context with timeout for database operations
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	s.logger.Info("Fetching client with ID: %d from database", id)

	var client models.Client
	query := `
		SELECT id, name, address, city, postal_code, country, vat_id, created_date, deleted
		FROM clients
		WHERE id = ?
	`

	s.logger.Debug("Executing query: %s with ID: %d", query, id)
	err := s.db.QueryRowContext(ctx, query, id).Scan(
		&client.ID,
		&client.Name,
		&client.Address,
		&client.City,
		&client.PostalCode,
		&client.Country,
		&client.VatID,
		&client.CreatedDate,
		&client.Deleted,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			s.logger.Warn("No client found with ID: %d", id)
		} else {
			s.logger.Error("Database error when fetching client ID %d: %v", id, err)
		}
		return nil, err
	}

	s.logger.Debug("Successfully fetched client: %s (ID: %d)", client.Name, client.ID)
	return &client, nil
}

// GetClients retrieves all clients from the database
func (s *DBService) GetClients() ([]models.Client, error) {
	rows, err := s.db.Query(`
		SELECT id, name, address, city, postal_code, country, vat_id, created_date, deleted
		FROM clients
		WHERE deleted = 0
		ORDER BY name
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var clients []models.Client
	for rows.Next() {
		var client models.Client
		if err := rows.Scan(&client.ID, &client.Name, &client.Address, &client.City, &client.PostalCode, &client.Country, &client.VatID, &client.CreatedDate, &client.Deleted); err != nil {
			return nil, err
		}
		clients = append(clients, client)
	}

	return clients, nil
}

// DeleteClient marks a client as deleted
func (s *DBService) DeleteClient(id int) error {
	_, err := s.db.Exec(`
		UPDATE clients
		SET deleted = 1
		WHERE id = ?
	`, id)
	return err
}

// Invoice methods

// SaveInvoice saves an invoice and its items to the database
func (s *DBService) SaveInvoice(invoice *models.Invoice, items []models.InvoiceItem) error {
	s.logger.Info("Starting transaction to save invoice")

	// Create a context with timeout for database operations
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Ensure the invoice_items table exists
	if err := s.EnsureInvoiceItemsTable(); err != nil {
		s.logger.Error("Failed to ensure invoice_items table exists: %v", err)
		return fmt.Errorf("failed to ensure invoice_items table exists: %w", err)
	}

	// Start a transaction
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		s.logger.Error("Failed to begin transaction: %v", err)
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if err != nil {
			s.logger.Warn("Rolling back transaction due to error")
			tx.Rollback()
		}
	}()

	// If no currency is provided, set a default based on the client's country
	if invoice.Currency == "" {
		// Get the client to determine the country
		client, err := s.GetClient(invoice.ClientID)
		if err == nil && client != nil {
			// Set currency based on client's country
			invoice.Currency = GetCurrencyForCountry(client.Country)
			s.logger.Info("Set currency to %s based on client's country %s", invoice.Currency, client.Country)
		} else {
			// Default to EUR if client can't be found
			invoice.Currency = "EUR"
			s.logger.Info("Set default currency to EUR")
		}
	}

	// Generate invoice number if not provided
	if invoice.InvoiceNumber == "" {
		// Get the current year
		currentYear := time.Now().Year()

		// Count existing invoices for this year
		var count int
		err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM invoices WHERE strftime('%Y', issue_date) = ?",
			strconv.Itoa(currentYear)).Scan(&count)
		if err != nil {
			s.logger.Error("Failed to count invoices for year %d: %v", currentYear, err)
			return fmt.Errorf("failed to count invoices: %w", err)
		}

		// Generate invoice number in format: INV-YYYY-XXXX
		invoice.InvoiceNumber = fmt.Sprintf("INV-%d-%04d", currentYear, count+1)
		s.logger.Info("Generated invoice number: %s", invoice.InvoiceNumber)
	}

	if invoice.ID == 0 {
		// Insert new invoice
		s.logger.Info("Creating new invoice with number: %s", invoice.InvoiceNumber)

		// Log the invoice data for debugging
		s.logger.Debug("Invoice data: ClientID=%d, BusinessID=%d, IssueDate=%s, DueDate=%s, Total=%f, Currency=%s",
			invoice.ClientID, invoice.BusinessID, invoice.IssueDate.Format("2006-01-02"),
			invoice.DueDate.Format("2006-01-02"), invoice.TotalAmount, invoice.Currency)

		result, err := tx.ExecContext(ctx, `
			INSERT INTO invoices (invoice_number, business_id, client_id, issue_date, due_date, hourly_rate, hours_worked, total_amount, vat_rate, vat_amount, reverse_charge_vat, currency, notes, status)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		`, invoice.InvoiceNumber, invoice.BusinessID, invoice.ClientID, invoice.IssueDate.Format("2006-01-02"), invoice.DueDate.Format("2006-01-02"),
			invoice.HourlyRate, invoice.HoursWorked, invoice.TotalAmount, invoice.VatRate, invoice.VatAmount, boolToInt(invoice.ReverseChargeVat), invoice.Currency, invoice.Notes, invoice.Status)
		if err != nil {
			s.logger.Error("Failed to insert invoice: %v", err)
			return fmt.Errorf("failed to insert invoice: %w", err)
		}

		id, err := result.LastInsertId()
		if err != nil {
			s.logger.Error("Failed to get last insert ID: %v", err)
			return fmt.Errorf("failed to get last insert ID: %w", err)
		}
		invoice.ID = int(id)
		s.logger.Info("Created new invoice with ID: %d", invoice.ID)
	} else {
		// Update existing invoice
		s.logger.Info("Updating existing invoice with ID: %d", invoice.ID)
		_, err := tx.ExecContext(ctx, `
			UPDATE invoices
			SET invoice_number = ?, business_id = ?, client_id = ?, issue_date = ?, due_date = ?, hourly_rate = ?, hours_worked = ?, total_amount = ?, vat_rate = ?, vat_amount = ?, reverse_charge_vat = ?, currency = ?, notes = ?, status = ?
			WHERE id = ?
		`, invoice.InvoiceNumber, invoice.BusinessID, invoice.ClientID, invoice.IssueDate.Format("2006-01-02"), invoice.DueDate.Format("2006-01-02"),
			invoice.HourlyRate, invoice.HoursWorked, invoice.TotalAmount, invoice.VatRate, invoice.VatAmount, boolToInt(invoice.ReverseChargeVat), invoice.Currency, invoice.Notes, invoice.Status, invoice.ID)
		if err != nil {
			s.logger.Error("Failed to update invoice: %v", err)
			return fmt.Errorf("failed to update invoice: %w", err)
		}

		// Delete existing items
		s.logger.Info("Deleting existing invoice items for invoice ID: %d", invoice.ID)
		_, err = tx.ExecContext(ctx, `DELETE FROM invoice_items WHERE invoice_id = ?`, invoice.ID)
		if err != nil {
			s.logger.Error("Failed to delete existing invoice items: %v", err)
			return fmt.Errorf("failed to delete existing invoice items: %w", err)
		}
	}

	// Insert invoice items
	s.logger.Info("Inserting %d invoice items", len(items))
	for i := range items {
		items[i].InvoiceID = invoice.ID
		_, err := tx.ExecContext(ctx, `
			INSERT INTO invoice_items (invoice_id, description, quantity, unit_price, amount)
			VALUES (?, ?, ?, ?, ?)
		`, items[i].InvoiceID, items[i].Description, items[i].Quantity, items[i].UnitPrice, items[i].Amount)
		if err != nil {
			s.logger.Error("Failed to insert invoice item %d: %v", i, err)
			return fmt.Errorf("failed to insert invoice item: %w", err)
		}
	}

	s.logger.Info("Committing transaction")
	if err := tx.Commit(); err != nil {
		s.logger.Error("Failed to commit transaction: %v", err)
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	s.logger.Info("Successfully saved invoice and %d items", len(items))
	return nil
}

// GetInvoice retrieves an invoice from the database
func (s *DBService) GetInvoice(id int) (*models.Invoice, []models.InvoiceItem, error) {
	// Create a context with timeout for database operations
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	s.logger.Info("Fetching invoice with ID: %d", id)

	// Get invoice
	var invoice models.Invoice
	var issueDate, dueDate string
	var reverseChargeVat int
	var currency sql.NullString // Use sql.NullString to handle NULL values

	err := s.db.QueryRowContext(ctx, `
		SELECT id, invoice_number, business_id, client_id, issue_date, due_date, hourly_rate, hours_worked, total_amount, vat_rate, vat_amount, reverse_charge_vat, currency, notes, status
		FROM invoices
		WHERE id = ?
	`, id).Scan(
		&invoice.ID,
		&invoice.InvoiceNumber,
		&invoice.BusinessID,
		&invoice.ClientID,
		&issueDate,
		&dueDate,
		&invoice.HourlyRate,
		&invoice.HoursWorked,
		&invoice.TotalAmount,
		&invoice.VatRate,
		&invoice.VatAmount,
		&reverseChargeVat,
		&currency,
		&invoice.Notes,
		&invoice.Status,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			s.logger.Warn("No invoice found with ID: %d", id)
		} else {
			s.logger.Error("Database error when fetching invoice ID %d: %v", id, err)
		}
		return nil, nil, err
	}

	// Parse dates
	invoice.IssueDate, err = time.Parse("2006-01-02", issueDate)
	if err != nil {
		s.logger.Error("Failed to parse issue date: %v", err)
		return nil, nil, fmt.Errorf("failed to parse issue date: %w", err)
	}

	invoice.DueDate, err = time.Parse("2006-01-02", dueDate)
	if err != nil {
		s.logger.Error("Failed to parse due date: %v", err)
		return nil, nil, fmt.Errorf("failed to parse due date: %w", err)
	}

	// Convert reverseChargeVat to bool
	invoice.ReverseChargeVat = intToBool(reverseChargeVat)

	// Handle currency
	if currency.Valid {
		invoice.Currency = currency.String
	} else {
		invoice.Currency = "EUR" // Default to EUR if NULL
	}

	// Get invoice items
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, invoice_id, description, quantity, unit_price, amount
		FROM invoice_items
		WHERE invoice_id = ?
	`, id)
	if err != nil {
		s.logger.Error("Failed to fetch invoice items: %v", err)
		return nil, nil, fmt.Errorf("failed to fetch invoice items: %w", err)
	}
	defer rows.Close()

	var items []models.InvoiceItem
	for rows.Next() {
		var item models.InvoiceItem
		if err := rows.Scan(
			&item.ID,
			&item.InvoiceID,
			&item.Description,
			&item.Quantity,
			&item.UnitPrice,
			&item.Amount,
		); err != nil {
			s.logger.Error("Failed to scan invoice item: %v", err)
			return nil, nil, fmt.Errorf("failed to scan invoice item: %w", err)
		}
		items = append(items, item)
	}

	if err := rows.Err(); err != nil {
		s.logger.Error("Error iterating invoice items: %v", err)
		return nil, nil, fmt.Errorf("error iterating invoice items: %w", err)
	}

	s.logger.Info("Successfully fetched invoice #%s with %d items", invoice.InvoiceNumber, len(items))
	return &invoice, items, nil
}

// GetInvoices retrieves all invoices from the database
func (s *DBService) GetInvoices() ([]models.Invoice, error) {
	rows, err := s.db.Query(`
		SELECT id, invoice_number, business_id, client_id, issue_date, due_date, hourly_rate, hours_worked, total_amount, vat_rate, vat_amount, reverse_charge_vat, currency, notes, status
		FROM invoices
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var invoices []models.Invoice
	for rows.Next() {
		var invoice models.Invoice
		var issueDate, dueDate string
		var reverseChargeVat int
		var currency sql.NullString // Use sql.NullString to handle NULL values
		err := rows.Scan(
			&invoice.ID, &invoice.InvoiceNumber, &invoice.BusinessID, &invoice.ClientID, &issueDate, &dueDate,
			&invoice.HourlyRate, &invoice.HoursWorked, &invoice.TotalAmount, &invoice.VatRate, &invoice.VatAmount,
			&reverseChargeVat, &currency, &invoice.Notes, &invoice.Status,
		)
		if err != nil {
			return nil, err
		}

		// Parse dates
		invoice.IssueDate, _ = time.Parse("2006-01-02", issueDate)
		invoice.DueDate, _ = time.Parse("2006-01-02", dueDate)
		invoice.ReverseChargeVat = intToBool(reverseChargeVat)

		// Set currency, default to EUR if NULL
		if currency.Valid {
			invoice.Currency = currency.String
		} else {
			invoice.Currency = "EUR"
		}

		invoices = append(invoices, invoice)
	}

	return invoices, nil
}

// UpdateInvoiceStatus updates the status of an invoice
func (s *DBService) UpdateInvoiceStatus(id int, status string) error {
	_, err := s.db.Exec("UPDATE invoices SET status = ? WHERE id = ?", status, id)
	return err
}

// DeleteInvoice deletes an invoice and its items from the database
func (s *DBService) DeleteInvoice(id int) error {
	// Start a transaction
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Delete invoice items first (due to foreign key constraint)
	_, err = tx.Exec("DELETE FROM invoice_items WHERE invoice_id = ?", id)
	if err != nil {
		return err
	}

	// Delete the invoice
	result, err := tx.Exec("DELETE FROM invoices WHERE id = ?", id)
	if err != nil {
		return err
	}

	// Check if the invoice was found and deleted
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rowsAffected == 0 {
		return fmt.Errorf("invoice with ID %d not found", id)
	}

	// Commit the transaction
	return tx.Commit()
}

// EnsureInvoiceItemsTable checks if the invoice_items table exists and creates it if it doesn't
func (s *DBService) EnsureInvoiceItemsTable() error {
	s.logger.Debug("Checking if invoice_items table exists")

	// Check if the invoice_items table exists
	var tableExists bool
	err := s.db.QueryRow(`
		SELECT COUNT(*) > 0 FROM sqlite_master 
		WHERE type='table' AND name='invoice_items'
	`).Scan(&tableExists)

	if err != nil {
		s.logger.Error("Failed to check if invoice_items table exists: %v", err)
		return fmt.Errorf("failed to check if invoice_items table exists: %w", err)
	}

	// If the table doesn't exist, create it
	if !tableExists {
		s.logger.Info("Creating invoice_items table")
		_, err := s.db.Exec(`
			CREATE TABLE invoice_items (
				id INTEGER PRIMARY KEY AUTOINCREMENT,
				invoice_id INTEGER NOT NULL,
				description TEXT NOT NULL,
				quantity REAL NOT NULL,
				unit_price REAL NOT NULL,
				amount REAL NOT NULL,
				FOREIGN KEY (invoice_id) REFERENCES invoices (id) ON DELETE CASCADE
			)
		`)

		if err != nil {
			s.logger.Error("Failed to create invoice_items table: %v", err)
			return fmt.Errorf("failed to create invoice_items table: %w", err)
		}

		s.logger.Info("Invoice items table created successfully")
	} else {
		s.logger.Debug("Invoice items table already exists")
	}

	return nil
}

// Helper functions

// boolToInt converts a boolean to an integer (1 for true, 0 for false)
func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

// intToBool converts an integer to a boolean (true for non-zero, false for zero)
func intToBool(i int) bool {
	return i != 0
}

// RemoveDatabase completely removes the database file and its associated files
func RemoveDatabase(dataDir string, logger *Logger) error {
	logger.Warn("Removing database file and associated files")

	dbPath := filepath.Join(dataDir, "database.db")

	// Remove the main database file
	if err := os.Remove(dbPath); err != nil && !os.IsNotExist(err) {
		logger.Error("Failed to remove database file: %v", err)
		return fmt.Errorf("failed to remove database file: %w", err)
	}

	// Remove associated WAL file
	walPath := dbPath + "-wal"
	if err := os.Remove(walPath); err != nil && !os.IsNotExist(err) {
		logger.Error("Failed to remove WAL file: %v", err)
		return fmt.Errorf("failed to remove WAL file: %w", err)
	}

	// Remove associated SHM file
	shmPath := dbPath + "-shm"
	if err := os.Remove(shmPath); err != nil && !os.IsNotExist(err) {
		logger.Error("Failed to remove SHM file: %v", err)
		return fmt.Errorf("failed to remove SHM file: %w", err)
	}

	logger.Info("Database files removed successfully")
	return nil
}

// GetDB returns the database connection
func (s *DBService) GetDB() *sql.DB {
	return s.db
}
