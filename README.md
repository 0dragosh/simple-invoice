# Simple Invoice

A simple invoicing application for consultants built with Go. All invoice applications I found were too complex so, naturally, I wrote my own.

## Features

- Generate PDF invoices with your logo (optional)
- Store business details (name, address, bank account (optional), VAT ID)
- Support for reverse charge VAT
- Auto-fetch client details from VAT ID (VIES/public databases)
- Auto-fetch UK business details from company name or VAT ID
- Currency hardcoded to Euros

## Setup

### Running with Docker (Recommended)

1. Clone the repository
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
- `COMPANIES_HOUSE_API_KEY`: API key for Companies House (optional, for UK company lookups)
- `VIES_IDENTIFIER`: VIES API identifier (optional, for enhanced EU VAT validation)
- `VIES_KEY`: VIES API key (optional, for enhanced EU VAT validation)
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

1. **EU VAT Validation (VIES)**: For enhanced EU VAT validation, you can register for VIES API credentials:
   - Register at [VIES API Portal](https://viesapi.eu/portal/register.php)
   - Set the `VIES_IDENTIFIER` and `VIES_KEY` environment variables:
     ```
     export VIES_IDENTIFIER=your_identifier
     export VIES_KEY=your_key
     ```
     or in docker-compose.yml:
     ```yaml
     environment:
       - VIES_IDENTIFIER=your_identifier
       - VIES_KEY=your_key
     ```

2. **UK Company Lookup**: To enable UK company lookup by name, you need to:
   - Obtain a Companies House API key from [Companies House API](https://developer.company-information.service.gov.uk/)
   - Set the `COMPANIES_HOUSE_API_KEY` environment variable:
     ```
     export COMPANIES_HOUSE_API_KEY=your_api_key
     ```
     or in docker-compose.yml:
     ```yaml
     environment:
       - COMPANIES_HOUSE_API_KEY=your_api_key
     ```

## Development

### Building the Docker Image

```bash
docker build -t simple-invoice .
```

### Running the Docker Container

```bash
docker run -p 8080:8080 -v $(pwd)/data:/app/data simple-invoice
```

## CI/CD

This project uses GitHub Actions for continuous integration and deployment:

### CI Workflow

The CI workflow runs on every push to the main branch and on pull requests:
- Builds the application
- Runs tests
- Builds the Docker image (without pushing)

### Container Publishing

The container publishing workflow:
- Runs on pushes to the main branch and on tag creation (v*)
- Builds and pushes the Docker image to GitHub Container Registry (ghcr.io)
- Tags the image with:
  - Latest tag for main branch
  - Semantic version tags for releases (v1.0.0, v1.0, etc.)
  - Short SHA for all pushes

### Using the Container Image

```bash
docker pull ghcr.io/0dragosh/simple-invoice:latest
docker run -p 8080:8080 -v /path/to/data:/app/data ghcr.io/0dragosh/simple-invoice:latest
``` 