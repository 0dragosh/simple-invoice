# Simple Invoice

A simple invoicing application for consultants built with Go. All invoice applications I found were too complex so, naturally, I wrote my own.

![Simple Invoice Screenshot](docs/screenshot.png)

## Features

- Generate PDF invoices with your logo (optional)
- Store business details (name, address, bank account (optional), VAT ID)
- Support for reverse charge VAT
- Auto-fetch client details from VAT ID (VIES/public databases)
- Auto-fetch UK business details from company name or VAT ID
- Support for multiple currencies:
  - Euro (EUR) and all European currencies (GBP, BGN, HRK, CZK, DKK, HUF, PLN, RON, SEK)
  - US Dollar (USD)
  - Swiss Franc (CHF)
- Automatic currency selection based on client's country
- Create and manage invoices

## Setup

### Running with Docker (Recommended)

1. Create a `docker-compose.yml` file with the following content:

```yaml
services:
  simple-invoice:
    image: ghcr.io/0dragosh/simple-invoice:latest
    ports:
      - "8080:8080"
    volumes:
      - ./data:/app/data
    environment:
      - PORT=8080
      - DATA_DIR=/app/data
      - COMPANIES_HOUSE_API_KEY=${COMPANIES_HOUSE_API_KEY:-}
      - LOG_LEVEL=${LOG_LEVEL:-INFO}
    restart: unless-stopped 
```

2. Run `docker-compose up -d` to start the application
3. Access the application at http://localhost:8080

### Running Locally

1. Clone the repository
2. Run `go mod tidy` to install dependencies
3. Run `go run cmd/server/main.go` to start the server
4. Access the application at http://localhost:8080

## Configuration

### Environment Variables

- `PORT`: The port to run the server on (default: 8080)
- `DATA_DIR`: The directory to store data in (default: /app/data)
- `COMPANIES_HOUSE_API_KEY`: Companies House API key (optional, required only for UK company lookups)
- `LOG_LEVEL`: Logging level (DEBUG, INFO, WARN, ERROR, FATAL) (default: INFO)

### Data Directory Structure

All persistent data is stored in the `/app/data` directory:

- `/app/data/images`: Logo images (optional)
- `/app/data/pdfs`: Generated PDF invoices
- `/app/data/simple-invoice.db`: SQLite database

## Usage

1. Configure your business details (can be auto-filled using VAT ID lookup)
   - Bank account details and logo are optional
2. Add clients (manually, via VAT ID lookup, or UK company name lookup)
3. Create invoices for your clients
4. Generate and download PDF invoices

### VAT ID Validation

The application supports VAT ID validation and company information retrieval for EU and UK companies:

1. **EU VAT Validation (VIES)**: The application uses the official VIES SOAP API from the European Commission for EU VAT validation.

2. **UK Company Lookup**: For UK companies, the application uses the Companies House API to look up company details by name or company number. Note that for UK companies, the VAT ID needs to be entered manually as it cannot be automatically validated.

   - To use the Companies House API, you need to register for an API key at [Companies House API](https://developer.company-information.service.gov.uk/)
   - Set the `COMPANIES_HOUSE_API_KEY` environment variable with your API key
   - Without this API key, UK company lookups will not work

Note: UK VAT numbers cannot be automatically validated through the application. Users will need to manually enter the VAT ID for UK companies.

## Development

### Building the Docker Image

```bash
docker build -t simple-invoice .
```

### Running the Docker Container

```bash
docker run -p 8080:8080 -v $(pwd)/data:/app/data simple-invoice
```