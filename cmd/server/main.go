package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/0dragosh/simple-invoice/internal/handlers"
	"github.com/0dragosh/simple-invoice/internal/services"
)

func main() {
	// Parse command-line flags
	resetDB := flag.Bool("reset-db", false, "Reset the database before starting")
	flag.Parse()

	// Get configuration from environment variables
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	dataDir := os.Getenv("DATA_DIR")
	if dataDir == "" {
		dataDir = "./data"
	}

	// Set up logging
	logLevelStr := os.Getenv("LOG_LEVEL")
	var logLevel services.LogLevel
	switch strings.ToUpper(logLevelStr) {
	case "DEBUG":
		logLevel = services.DEBUG
	case "INFO":
		logLevel = services.INFO
	case "WARN":
		logLevel = services.WARN
	case "ERROR":
		logLevel = services.ERROR
	case "FATAL":
		logLevel = services.FATAL
	default:
		// Use DEBUG level by default for better diagnostics
		logLevel = services.DEBUG
	}

	logger := services.NewLogger(logLevel)
	logger.Info("Starting application with log level: %s", logLevelStr)

	// Ensure data directories exist
	if err := ensureDir(dataDir, logger); err != nil {
		logger.Fatal("Failed to create data directory: %v", err)
	}

	// Reset database if requested
	if *resetDB {
		logger.Warn("Database reset requested via command-line flag")
		if err := services.RemoveDatabase(dataDir, logger); err != nil {
			logger.Error("Failed to reset database: %v", err)
		} else {
			logger.Info("Database reset successful")
		}
	}

	// Create and configure the HTTP server
	mux := http.NewServeMux()
	appHandler, err := handlers.RegisterHandlers(mux, dataDir, logger)
	if err != nil {
		logger.Fatal("Failed to register handlers: %v", err)
	}

	// Ensure cleanup on exit
	defer func() {
		logger.Info("Shutting down application...")
		if err := appHandler.Cleanup(); err != nil {
			logger.Error("Error during cleanup: %v", err)
		}
	}()

	// Create server with timeout settings
	server := &http.Server{
		Addr:         fmt.Sprintf(":%s", port),
		Handler:      mux,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Start the server in a goroutine
	go func() {
		logger.Info("Starting server on %s", server.Addr)
		logger.Info("Data directory: %s", dataDir)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatal("Server error: %v", err)
		}
	}()

	// Set up graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	logger.Info("Shutting down server...")

	// Create a deadline for shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Attempt graceful shutdown
	if err := server.Shutdown(ctx); err != nil {
		logger.Error("Server forced to shutdown: %v", err)
	}

	logger.Info("Server exited gracefully")
}

func ensureDir(dirName string, logger *services.Logger) error {
	if _, err := os.Stat(dirName); os.IsNotExist(err) {
		err = os.MkdirAll(dirName, 0755)
		if err != nil {
			return fmt.Errorf("failed to create directory: %w", err)
		}
		logger.Info("Created directory: %s", dirName)
	}
	return nil
}
