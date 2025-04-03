# Simple Invoice E2E Test Suite

This directory contains the end-to-end test suite for the Simple Invoice application.

## Overview

The test suite uses Playwright to test the core functionality of the Simple Invoice application:

- Basic page navigation tests
- Business creation and management
- Client creation and management  
- Invoice creation with various configurations
  - Minimal invoices
  - Basic invoices
  - Flat-rate invoices
  - Full invoices with multiple items and tax options

## Test Files

- `basic-test.spec.js` - Base tests for core functionality
- `invoice-minimal.spec.js` - Basic invoice creation tests
- `invoice-basic.spec.js` - Standard invoice creation tests
- `invoice-flat.spec.js` - Flat-rate invoice tests
- `invoice-full.spec.js` - Comprehensive invoice tests with all features
- `simple.spec.js` - Basic page load tests
- `pages.spec.js` - Page navigation tests
- `all-tests.spec.js` - Runs all tests in sequence

## Running Tests

Make sure your application is running at http://localhost:8080 (or configure the BASE_URL environment variable).

```bash
# Run all tests
npm run test:all

# Run specific test suites
npm run test:basic   # Run basic invoice tests
npm run test:minimal # Run minimal invoice tests
npm run test:flat    # Run flat-rate invoice tests
npm run test:full    # Run comprehensive invoice tests
npm run test:simple  # Run page navigation tests

# Generate HTML report
npm run test:report

# View the HTML report
npm run show:report
```

## Test Design

The tests use a robust approach with:

1. **Multiple selector options** for filling forms to handle UI changes
2. **Screenshot captures** at critical points for debugging
3. **Flexible timing** with appropriate waits
4. **Graceful error handling** to prevent test failures from minor UI changes

## Debugging

Screenshots are saved to the `screenshots/` directory and can be used to debug test failures.

The HTML report provides a detailed view of test results, including:
- Test status (passed/failed)
- Screenshots
- Test duration
- Errors and stack traces 