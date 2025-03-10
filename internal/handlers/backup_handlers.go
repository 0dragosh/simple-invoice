package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
)

// BackupsHandler handles the backups page
func (h *AppHandler) BackupsHandler(w http.ResponseWriter, r *http.Request) {
	backups, err := h.backupService.ListBackups()
	if err != nil {
		h.logger.Error("Failed to list backups: %v", err)
		http.Error(w, "Failed to list backups", http.StatusInternalServerError)
		return
	}

	// Get backup directory relative to data directory
	backupDir := filepath.Join(h.dataDir, "backups")
	relBackupDir, err := filepath.Rel(h.dataDir, backupDir)
	if err != nil {
		relBackupDir = "backups"
	}

	// Get backup cron schedule
	backupCron := os.Getenv("BACKUP_CRON")

	data := map[string]interface{}{
		"Title":      "Backups",
		"Backups":    backups,
		"BackupDir":  relBackupDir,
		"BackupCron": backupCron,
	}

	h.renderTemplate(w, "backups", data)
}

// BackupsAPIHandler handles backup API requests
func (h *AppHandler) BackupsAPIHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	switch r.Method {
	case http.MethodGet:
		// List backups
		backups, err := h.backupService.ListBackups()
		if err != nil {
			h.logger.Error("Failed to list backups: %v", err)
			http.Error(w, fmt.Sprintf("Failed to list backups: %v", err), http.StatusInternalServerError)
			return
		}

		json.NewEncoder(w).Encode(backups)

	case http.MethodPost:
		// Create backup
		h.logger.Info("Creating backup")
		if err := h.backupService.CreateBackup(); err != nil {
			h.logger.Error("Failed to create backup: %v", err)
			http.Error(w, fmt.Sprintf("Failed to create backup: %v", err), http.StatusInternalServerError)
			return
		}

		h.logger.Info("Backup created successfully")
		json.NewEncoder(w).Encode(map[string]string{"message": "Backup created successfully"})

	case http.MethodDelete:
		// Delete backup
		filename := r.URL.Query().Get("filename")
		if filename == "" {
			h.logger.Warn("No filename provided for backup deletion")
			http.Error(w, "Filename is required", http.StatusBadRequest)
			return
		}

		h.logger.Info("Deleting backup: %s", filename)
		backupPath := filepath.Join(h.dataDir, "backups", filename)

		// Check if file exists
		if _, err := os.Stat(backupPath); os.IsNotExist(err) {
			h.logger.Warn("Backup file not found: %s", backupPath)
			http.Error(w, "Backup file not found", http.StatusNotFound)
			return
		}

		// Delete file
		if err := os.Remove(backupPath); err != nil {
			h.logger.Error("Failed to delete backup: %v", err)
			http.Error(w, fmt.Sprintf("Failed to delete backup: %v", err), http.StatusInternalServerError)
			return
		}

		h.logger.Info("Backup deleted successfully: %s", filename)
		json.NewEncoder(w).Encode(map[string]string{"message": "Backup deleted successfully"})

	default:
		h.logger.Warn("Method not allowed: %s", r.Method)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// RestoreBackupHandler handles backup restoration
func (h *AppHandler) RestoreBackupHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if r.Method != http.MethodPost {
		h.logger.Warn("Method not allowed: %s", r.Method)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	filename := r.URL.Query().Get("filename")
	if filename == "" {
		h.logger.Warn("No filename provided for backup restoration")
		http.Error(w, "Filename is required", http.StatusBadRequest)
		return
	}

	h.logger.Info("Restoring backup: %s", filename)
	if err := h.backupService.RestoreBackup(filename); err != nil {
		h.logger.Error("Failed to restore backup: %v", err)
		http.Error(w, fmt.Sprintf("Failed to restore backup: %v", err), http.StatusInternalServerError)
		return
	}

	// Check if the database needs to be reopened
	if h.backupService.NeedsReopen() {
		h.logger.Info("Database needs to be reopened after restore")

		// Reopen the database connection
		if err := h.dbService.ReopenConnection(); err != nil {
			h.logger.Error("Failed to reopen database connection: %v", err)
			http.Error(w, fmt.Sprintf("Backup restored but failed to reopen database connection: %v", err), http.StatusInternalServerError)
			return
		}

		// Mark the database as reopened
		h.backupService.SetReopened()

		h.logger.Info("Database connection reopened successfully")
	}

	h.logger.Info("Backup restored successfully: %s", filename)
	json.NewEncoder(w).Encode(map[string]string{"message": "Backup restored successfully"})
}
