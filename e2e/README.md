# Simple Invoice E2E Tests

End-to-end tests for the Simple Invoice application using Playwright.

## Tests Coverage

The E2E tests cover the following scenarios:

1. Creating a business with multiple bank accounts (EUR and USD)
2. Creating a client with generated details
3. Creating invoices with different configurations:
   - EUR invoice with VAT
   - EUR invoice with reverse charge VAT
   - USD invoice with no VAT
4. Validating PDF content for each invoice type

## Running Tests Locally

### Prerequisites

- Node.js (v16+)
- npm
- Go (to run the Simple Invoice application)

### Setup

1. Install dependencies:

```bash
cd e2e
npm install
npx playwright install
```

2. Run tests:

```bash
# Run in headless mode
npm test

# Run in debug mode (with browser visible)
npm run test:debug
```

## Test Structure

- `tests/invoice.spec.js` - The main test file with all test scenarios
- `tests/utils.js` - Utility functions for data generation and PDF validation

## CI Integration

These tests run automatically in GitHub CI when:
- Pushing to the main branch
- Creating a new tag
- Opening a pull request to the main branch

Test artifacts (test reports and generated PDFs) are uploaded as GitHub artifacts and can be viewed for each workflow run. 