package services

import (
	"archive/tar"
	"compress/gzip"
	"database/sql"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/robfig/cron/v3"
)

// BackupService provides methods for backing up and restoring the database
type BackupService struct {
	db        *sql.DB
	dataDir   string
	backupDir string
	logger    *Logger
	cron      *cron.Cron
}

// BackupInfo represents information about a backup file
type BackupInfo struct {
	Filename    string    `json:"filename"`
	Path        string    `json:"path"`
	Size        int64     `json:"size"`
	CreatedTime time.Time `json:"created_time"`
}

// NewBackupService creates a new BackupService
func NewBackupService(db *sql.DB, dataDir string, logger *Logger) (*BackupService, error) {
	backupDir := filepath.Join(dataDir, "backups")

	// Create backup directory if it doesn't exist
	if err := os.MkdirAll(backupDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create backup directory: %w", err)
	}

	return &BackupService{
		db:        db,
		dataDir:   dataDir,
		backupDir: backupDir,
		logger:    logger,
		cron:      cron.New(),
	}, nil
}

// StartScheduler starts the backup scheduler with the given cron expression
func (s *BackupService) StartScheduler(cronExpr string) error {
	if cronExpr == "" {
		s.logger.Info("No backup schedule configured, automatic backups disabled")
		return nil
	}

	s.logger.Info("Starting backup scheduler with cron expression: %s", cronExpr)

	_, err := s.cron.AddFunc(cronExpr, func() {
		s.logger.Info("Running scheduled backup")
		if err := s.CreateBackup(); err != nil {
			s.logger.Error("Scheduled backup failed: %v", err)
		} else {
			s.logger.Info("Scheduled backup completed successfully")
		}
	})

	if err != nil {
		return fmt.Errorf("failed to schedule backup: %w", err)
	}

	s.cron.Start()
	return nil
}

// StopScheduler stops the backup scheduler
func (s *BackupService) StopScheduler() {
	if s.cron != nil {
		s.cron.Stop()
	}
}

// CreateBackup creates a backup of the database
func (s *BackupService) CreateBackup() error {
	s.logger.Info("Creating database backup")

	// Generate backup filename with timestamp
	timestamp := time.Now().Format("2006-01-02_150405")
	backupFilename := fmt.Sprintf("simple-invoice-backup-%s.tar.gz", timestamp)
	backupPath := filepath.Join(s.backupDir, backupFilename)

	// Create the tar.gz file
	file, err := os.Create(backupPath)
	if err != nil {
		return fmt.Errorf("failed to create backup file: %w", err)
	}
	defer file.Close()

	// Create gzip writer
	gzipWriter := gzip.NewWriter(file)
	defer gzipWriter.Close()

	// Create tar writer
	tarWriter := tar.NewWriter(gzipWriter)
	defer tarWriter.Close()

	// Backup the database
	dbPath := filepath.Join(s.dataDir, "simple-invoice.db")
	s.logger.Debug("Database path: %s", dbPath)

	// Create a temporary copy of the database to ensure consistency
	tempDbPath := filepath.Join(s.dataDir, "temp-backup.db")

	// Ensure the database is in a consistent state for backup
	_, err = s.db.Exec("PRAGMA wal_checkpoint(FULL)")
	if err != nil {
		s.logger.Warn("Failed to checkpoint WAL: %v", err)
	}

	// Create a backup of the database
	backupDb, err := sql.Open("sqlite3", tempDbPath)
	if err != nil {
		return fmt.Errorf("failed to open temporary database: %w", err)
	}
	defer backupDb.Close()

	// Use SQLite's backup API via SQL
	_, err = s.db.Exec("VACUUM INTO ?", tempDbPath)
	if err != nil {
		os.Remove(tempDbPath)
		return fmt.Errorf("failed to create database backup: %w", err)
	}

	// Add the database file to the tar archive
	if err := addFileToTar(tarWriter, tempDbPath, "simple-invoice.db"); err != nil {
		os.Remove(tempDbPath)
		return fmt.Errorf("failed to add database to backup: %w", err)
	}

	// Add images directory if it exists
	imagesDir := filepath.Join(s.dataDir, "images")
	if _, err := os.Stat(imagesDir); err == nil {
		if err := addDirectoryToTar(tarWriter, imagesDir, "images"); err != nil {
			os.Remove(tempDbPath)
			return fmt.Errorf("failed to add images to backup: %w", err)
		}
	}

	// Add PDFs directory if it exists
	pdfsDir := filepath.Join(s.dataDir, "pdfs")
	if _, err := os.Stat(pdfsDir); err == nil {
		if err := addDirectoryToTar(tarWriter, pdfsDir, "pdfs"); err != nil {
			os.Remove(tempDbPath)
			return fmt.Errorf("failed to add PDFs to backup: %w", err)
		}
	}

	// Clean up temporary database file
	os.Remove(tempDbPath)

	s.logger.Info("Backup created successfully: %s", backupPath)
	return nil
}

// ListBackups returns a list of available backups
func (s *BackupService) ListBackups() ([]BackupInfo, error) {
	s.logger.Info("Listing available backups")

	files, err := os.ReadDir(s.backupDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read backup directory: %w", err)
	}

	var backups []BackupInfo

	for _, file := range files {
		if file.IsDir() || !strings.HasPrefix(file.Name(), "simple-invoice-backup-") || !strings.HasSuffix(file.Name(), ".tar.gz") {
			continue
		}

		info, err := file.Info()
		if err != nil {
			s.logger.Warn("Failed to get info for backup file %s: %v", file.Name(), err)
			continue
		}

		backups = append(backups, BackupInfo{
			Filename:    file.Name(),
			Path:        filepath.Join(s.backupDir, file.Name()),
			Size:        info.Size(),
			CreatedTime: info.ModTime(),
		})
	}

	// Sort backups by creation time (newest first)
	sort.Slice(backups, func(i, j int) bool {
		return backups[i].CreatedTime.After(backups[j].CreatedTime)
	})

	return backups, nil
}

// RestoreBackup restores the database from a backup file
func (s *BackupService) RestoreBackup(backupFilename string) error {
	s.logger.Info("Restoring database from backup: %s", backupFilename)

	backupPath := filepath.Join(s.backupDir, backupFilename)

	// Check if backup file exists
	if _, err := os.Stat(backupPath); err != nil {
		return fmt.Errorf("backup file not found: %w", err)
	}

	// Create a temporary directory for extraction
	tempDir, err := os.MkdirTemp("", "simple-invoice-restore-")
	if err != nil {
		return fmt.Errorf("failed to create temporary directory: %w", err)
	}
	defer os.RemoveAll(tempDir)

	// Open the tar.gz file
	file, err := os.Open(backupPath)
	if err != nil {
		return fmt.Errorf("failed to open backup file: %w", err)
	}
	defer file.Close()

	// Create gzip reader
	gzipReader, err := gzip.NewReader(file)
	if err != nil {
		return fmt.Errorf("failed to create gzip reader: %w", err)
	}
	defer gzipReader.Close()

	// Create tar reader
	tarReader := tar.NewReader(gzipReader)

	// Extract files
	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("failed to read tar header: %w", err)
		}

		// Skip if not a file
		if header.Typeflag != tar.TypeReg {
			continue
		}

		// Create directory for file if needed
		targetPath := filepath.Join(tempDir, header.Name)
		targetDir := filepath.Dir(targetPath)
		if err := os.MkdirAll(targetDir, 0755); err != nil {
			return fmt.Errorf("failed to create directory: %w", err)
		}

		// Create file
		outFile, err := os.Create(targetPath)
		if err != nil {
			return fmt.Errorf("failed to create file: %w", err)
		}

		// Copy contents
		if _, err := io.Copy(outFile, tarReader); err != nil {
			outFile.Close()
			return fmt.Errorf("failed to extract file: %w", err)
		}
		outFile.Close()
	}

	// Close the database connection
	if err := s.db.Close(); err != nil {
		return fmt.Errorf("failed to close database connection: %w", err)
	}

	// Replace the database file
	dbPath := filepath.Join(s.dataDir, "simple-invoice.db")
	extractedDbPath := filepath.Join(tempDir, "simple-invoice.db")

	// Backup the current database just in case
	currentBackupPath := filepath.Join(s.dataDir, "pre-restore-backup.db")
	if err := copyFile(dbPath, currentBackupPath); err != nil {
		s.logger.Warn("Failed to create pre-restore backup: %v", err)
	}

	// Replace the database file
	if err := copyFile(extractedDbPath, dbPath); err != nil {
		return fmt.Errorf("failed to replace database file: %w", err)
	}

	// Copy images directory if it exists in the backup
	extractedImagesDir := filepath.Join(tempDir, "images")
	if _, err := os.Stat(extractedImagesDir); err == nil {
		imagesDir := filepath.Join(s.dataDir, "images")
		if err := os.RemoveAll(imagesDir); err != nil {
			s.logger.Warn("Failed to remove existing images directory: %v", err)
		}
		if err := copyDirectory(extractedImagesDir, imagesDir); err != nil {
			s.logger.Warn("Failed to restore images directory: %v", err)
		}
	}

	// Copy PDFs directory if it exists in the backup
	extractedPdfsDir := filepath.Join(tempDir, "pdfs")
	if _, err := os.Stat(extractedPdfsDir); err == nil {
		pdfsDir := filepath.Join(s.dataDir, "pdfs")
		if err := os.RemoveAll(pdfsDir); err != nil {
			s.logger.Warn("Failed to remove existing PDFs directory: %v", err)
		}
		if err := copyDirectory(extractedPdfsDir, pdfsDir); err != nil {
			s.logger.Warn("Failed to restore PDFs directory: %v", err)
		}
	}

	s.logger.Info("Database restored successfully from backup: %s", backupFilename)
	return nil
}

// Helper functions

// addFileToTar adds a file to a tar archive
func addFileToTar(tarWriter *tar.Writer, filePath, arcName string) error {
	file, err := os.Open(filePath)
	if err != nil {
		return err
	}
	defer file.Close()

	stat, err := file.Stat()
	if err != nil {
		return err
	}

	header := &tar.Header{
		Name:    arcName,
		Size:    stat.Size(),
		Mode:    int64(stat.Mode()),
		ModTime: stat.ModTime(),
	}

	if err := tarWriter.WriteHeader(header); err != nil {
		return err
	}

	_, err = io.Copy(tarWriter, file)
	return err
}

// addDirectoryToTar adds a directory and its contents to a tar archive
func addDirectoryToTar(tarWriter *tar.Writer, dirPath, arcPath string) error {
	return filepath.Walk(dirPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip directories themselves
		if info.IsDir() {
			return nil
		}

		// Get the relative path
		relPath, err := filepath.Rel(dirPath, path)
		if err != nil {
			return err
		}

		// Create archive path
		arcName := filepath.Join(arcPath, relPath)

		return addFileToTar(tarWriter, path, arcName)
	})
}

// copyFile copies a file from src to dst
func copyFile(src, dst string) error {
	sourceFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer sourceFile.Close()

	destFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer destFile.Close()

	_, err = io.Copy(destFile, sourceFile)
	if err != nil {
		return err
	}

	return destFile.Sync()
}

// copyDirectory copies a directory from src to dst
func copyDirectory(src, dst string) error {
	// Create destination directory
	if err := os.MkdirAll(dst, 0755); err != nil {
		return err
	}

	// Walk through the source directory
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Get the relative path
		relPath, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}

		// Skip the root directory
		if relPath == "." {
			return nil
		}

		// Create the destination path
		dstPath := filepath.Join(dst, relPath)

		// If it's a directory, create it
		if info.IsDir() {
			return os.MkdirAll(dstPath, info.Mode())
		}

		// If it's a file, copy it
		return copyFile(path, dstPath)
	})
}
