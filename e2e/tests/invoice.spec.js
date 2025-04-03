const { test, expect } = require('@playwright/test');
const { 
  generateBusinessData, 
  generateClientData,
  generateInvoiceData,
  validatePDF
} = require('./utils');

// Test suite for the invoice application
test.describe('Invoice Application E2E Tests', () => {
  // Store test data for use across tests
  const testData = {
    business: null,
    client: null,
    invoices: {
      eur: null,
      eurReverseCharge: null,
      usd: null
    },
    invoiceIds: {
      eur: null,
      eurReverseCharge: null,
      usd: null
    }
  };

  // Before all tests, set up the browser context
  test.beforeAll(async ({ browser }) => {
    // Generate test data
    testData.business = generateBusinessData();
    testData.client = generateClientData();
  });

  // Test: Create a new business with two bank accounts (EUR and USD)
  test('should create a business with EUR and USD bank accounts', async ({ page }) => {
    // Navigate to business page
    await page.goto('/business');
    
    // Fill in business details
    await page.getByLabel('Business Name').fill(testData.business.name);
    await page.getByLabel('Address').fill(testData.business.address);
    await page.getByLabel('City').fill(testData.business.city);
    await page.getByLabel('Postal Code').fill(testData.business.postalCode);
    await page.getByLabel('Country').fill(testData.business.country);
    await page.getByLabel('VAT ID').fill(testData.business.vatID);
    await page.getByLabel('Email').fill(testData.business.email);
    
    // Fill in EUR bank details
    await page.getByLabel('Bank Name').fill(testData.business.bankName);
    await page.getByLabel('IBAN').fill(testData.business.iban);
    await page.getByLabel('BIC/SWIFT').fill(testData.business.bic);
    
    // Add another bank account (USD)
    await page.getByText('Add Bank Account').click();
    
    // Fill in USD bank details
    await page.getByLabel('Bank Name', { exact: true }).nth(1).fill(testData.business.bankNameUSD);
    await page.getByLabel('IBAN', { exact: true }).nth(1).fill(testData.business.ibanUSD);
    await page.getByLabel('BIC/SWIFT', { exact: true }).nth(1).fill(testData.business.bicUSD);
    await page.getByLabel('Currency', { exact: true }).nth(1).selectOption('USD');
    
    // Save business details
    await page.getByRole('button', { name: 'Save Business Details' }).click();
    
    // Verify success message
    await expect(page.getByText('Business details saved successfully')).toBeVisible();
    
    // Verify business name is displayed
    await expect(page.getByText(testData.business.name)).toBeVisible();
  });

  // Test: Create a new client
  test('should create a new client', async ({ page }) => {
    // Navigate to clients page
    await page.goto('/clients');
    
    // Click add new client button
    await page.getByRole('button', { name: 'Add Client' }).click();
    
    // Fill in client details
    await page.getByLabel('Client Name').fill(testData.client.name);
    await page.getByLabel('Address').fill(testData.client.address);
    await page.getByLabel('City').fill(testData.client.city);
    await page.getByLabel('Postal Code').fill(testData.client.postalCode);
    await page.getByLabel('Country').fill(testData.client.country);
    await page.getByLabel('VAT ID').fill(testData.client.vatID);
    await page.getByLabel('Email').fill(testData.client.email);
    await page.getByLabel('Phone').fill(testData.client.phone);
    
    // Save client
    await page.getByRole('button', { name: 'Save' }).click();
    
    // Verify success message
    await expect(page.getByText('Client saved successfully')).toBeVisible();
    
    // Verify client name is displayed in the list
    await expect(page.getByText(testData.client.name)).toBeVisible();
  });

  // Test: Create invoice in EUR with VAT
  test('should create an invoice in EUR with VAT', async ({ page }) => {
    // Generate EUR invoice data with VAT
    testData.invoices.eur = generateInvoiceData(
      testData.business.name, 
      testData.client.name, 
      'EUR', 
      'normal'
    );
    
    // Navigate to invoices page
    await page.goto('/invoices');
    
    // Click create invoice button
    await page.getByRole('button', { name: 'Create Invoice' }).click();
    
    // Select the client
    await page.getByLabel('Client').selectOption({ label: testData.client.name });
    
    // Fill in invoice details
    await page.getByLabel('Invoice Number').fill(testData.invoices.eur.invoiceNumber);
    await page.getByLabel('Issue Date').fill(testData.invoices.eur.issueDate);
    await page.getByLabel('Due Date').fill(testData.invoices.eur.dueDate);
    await page.getByLabel('Currency').selectOption('EUR');
    await page.getByLabel('VAT Rate (%)').fill(testData.invoices.eur.vatRate.toString());
    await page.getByLabel('Notes').fill(testData.invoices.eur.notes);
    
    // Add invoice items
    for (const [index, item] of testData.invoices.eur.items.entries()) {
      if (index > 0) {
        await page.getByRole('button', { name: 'Add Item' }).click();
      }
      
      // Fill in item details
      await page.getByLabel('Description').nth(index).fill(item.description);
      await page.getByLabel('Quantity').nth(index).fill(item.quantity.toString());
      await page.getByLabel('Unit Price').nth(index).fill(item.unitPrice.toString());
    }
    
    // Save invoice
    await page.getByRole('button', { name: 'Save Invoice' }).click();
    
    // Verify success message
    await expect(page.getByText('Invoice created successfully')).toBeVisible();
    
    // Store the invoice ID for PDF validation
    // Extract the ID from the URL: /invoices/view/ID
    const url = page.url();
    testData.invoiceIds.eur = url.split('/').pop();
    
    // Verify invoice number is displayed
    await expect(page.getByText(testData.invoices.eur.invoiceNumber)).toBeVisible();
    
    // Wait for PDF to be generated
    await page.waitForTimeout(2000);
  });

  // Test: Create invoice in EUR with reverse charge VAT
  test('should create an invoice in EUR with reverse charge VAT', async ({ page }) => {
    // Generate EUR invoice data with reverse charge VAT
    testData.invoices.eurReverseCharge = generateInvoiceData(
      testData.business.name, 
      testData.client.name, 
      'EUR', 
      'reverse-charge'
    );
    
    // Navigate to invoices page
    await page.goto('/invoices');
    
    // Click create invoice button
    await page.getByRole('button', { name: 'Create Invoice' }).click();
    
    // Select the client
    await page.getByLabel('Client').selectOption({ label: testData.client.name });
    
    // Fill in invoice details
    await page.getByLabel('Invoice Number').fill(testData.invoices.eurReverseCharge.invoiceNumber);
    await page.getByLabel('Issue Date').fill(testData.invoices.eurReverseCharge.issueDate);
    await page.getByLabel('Due Date').fill(testData.invoices.eurReverseCharge.dueDate);
    await page.getByLabel('Currency').selectOption('EUR');
    await page.getByLabel('VAT Rate (%)').fill('0'); // Reverse charge VAT is 0%
    await page.getByLabel('Notes').fill(testData.invoices.eurReverseCharge.notes);
    
    // Add invoice items
    for (const [index, item] of testData.invoices.eurReverseCharge.items.entries()) {
      if (index > 0) {
        await page.getByRole('button', { name: 'Add Item' }).click();
      }
      
      // Fill in item details
      await page.getByLabel('Description').nth(index).fill(item.description);
      await page.getByLabel('Quantity').nth(index).fill(item.quantity.toString());
      await page.getByLabel('Unit Price').nth(index).fill(item.unitPrice.toString());
    }
    
    // Save invoice
    await page.getByRole('button', { name: 'Save Invoice' }).click();
    
    // Verify success message
    await expect(page.getByText('Invoice created successfully')).toBeVisible();
    
    // Store the invoice ID for PDF validation
    const url = page.url();
    testData.invoiceIds.eurReverseCharge = url.split('/').pop();
    
    // Verify invoice number is displayed
    await expect(page.getByText(testData.invoices.eurReverseCharge.invoiceNumber)).toBeVisible();
    
    // Wait for PDF to be generated
    await page.waitForTimeout(2000);
  });

  // Test: Create invoice in USD with no VAT
  test('should create an invoice in USD with no VAT', async ({ page }) => {
    // Generate USD invoice data with no VAT
    testData.invoices.usd = generateInvoiceData(
      testData.business.name, 
      testData.client.name, 
      'USD', 
      'none'
    );
    
    // Navigate to invoices page
    await page.goto('/invoices');
    
    // Click create invoice button
    await page.getByRole('button', { name: 'Create Invoice' }).click();
    
    // Select the client
    await page.getByLabel('Client').selectOption({ label: testData.client.name });
    
    // Fill in invoice details
    await page.getByLabel('Invoice Number').fill(testData.invoices.usd.invoiceNumber);
    await page.getByLabel('Issue Date').fill(testData.invoices.usd.issueDate);
    await page.getByLabel('Due Date').fill(testData.invoices.usd.dueDate);
    await page.getByLabel('Currency').selectOption('USD');
    await page.getByLabel('VAT Rate (%)').fill('0'); // No VAT
    await page.getByLabel('Notes').fill(testData.invoices.usd.notes);
    
    // Add invoice items
    for (const [index, item] of testData.invoices.usd.items.entries()) {
      if (index > 0) {
        await page.getByRole('button', { name: 'Add Item' }).click();
      }
      
      // Fill in item details
      await page.getByLabel('Description').nth(index).fill(item.description);
      await page.getByLabel('Quantity').nth(index).fill(item.quantity.toString());
      await page.getByLabel('Unit Price').nth(index).fill(item.unitPrice.toString());
    }
    
    // Save invoice
    await page.getByRole('button', { name: 'Save Invoice' }).click();
    
    // Verify success message
    await expect(page.getByText('Invoice created successfully')).toBeVisible();
    
    // Store the invoice ID for PDF validation
    const url = page.url();
    testData.invoiceIds.usd = url.split('/').pop();
    
    // Verify invoice number is displayed
    await expect(page.getByText(testData.invoices.usd.invoiceNumber)).toBeVisible();
    
    // Wait for PDF to be generated
    await page.waitForTimeout(2000);
  });

  // Test: Validate the EUR invoice PDF
  test('should validate the EUR invoice PDF content', async ({ page }) => {
    // Skip in non-CI environment if no invoices were created
    test.skip(!process.env.CI && !testData.invoiceIds.eur, 'Skipping PDF validation in non-CI environment');
    
    // Navigate to the invoice
    await page.goto(`/invoices/view/${testData.invoiceIds.eur}`);
    
    // Get the PDF file path
    const pdfPath = `../data/pdfs/invoice_${testData.invoiceIds.eur}.pdf`;
    
    // Define expected values in the PDF
    const expectedValues = {
      businessName: testData.business.name,
      clientName: testData.client.name,
      invoiceNumber: testData.invoices.eur.invoiceNumber,
      vatRate: `${testData.invoices.eur.vatRate}%`,
      currency: '€', // EUR symbol
      item1: testData.invoices.eur.items[0].description,
      item2: testData.invoices.eur.items[1].description
    };
    
    // Validate the PDF content
    const validationResult = await validatePDF(pdfPath, expectedValues);
    
    // Assert that validation succeeded
    expect(validationResult.success, `PDF validation failed: ${validationResult.errors.join(', ')}`).toBeTruthy();
  });
  
  // Test: Validate the EUR invoice with reverse charge PDF
  test('should validate the EUR reverse charge invoice PDF content', async ({ page }) => {
    // Skip in non-CI environment if no invoices were created
    test.skip(!process.env.CI && !testData.invoiceIds.eurReverseCharge, 'Skipping PDF validation in non-CI environment');
    
    // Navigate to the invoice
    await page.goto(`/invoices/view/${testData.invoiceIds.eurReverseCharge}`);
    
    // Get the PDF file path
    const pdfPath = `../data/pdfs/invoice_${testData.invoiceIds.eurReverseCharge}.pdf`;
    
    // Define expected values in the PDF
    const expectedValues = {
      businessName: testData.business.name,
      clientName: testData.client.name,
      invoiceNumber: testData.invoices.eurReverseCharge.invoiceNumber,
      reverseCharge: 'Reverse charge',
      currency: '€', // EUR symbol
      item1: testData.invoices.eurReverseCharge.items[0].description,
      item2: testData.invoices.eurReverseCharge.items[1].description
    };
    
    // Validate the PDF content
    const validationResult = await validatePDF(pdfPath, expectedValues);
    
    // Assert that validation succeeded
    expect(validationResult.success, `PDF validation failed: ${validationResult.errors.join(', ')}`).toBeTruthy();
  });
  
  // Test: Validate the USD invoice PDF
  test('should validate the USD invoice PDF content', async ({ page }) => {
    // Skip in non-CI environment if no invoices were created
    test.skip(!process.env.CI && !testData.invoiceIds.usd, 'Skipping PDF validation in non-CI environment');
    
    // Navigate to the invoice
    await page.goto(`/invoices/view/${testData.invoiceIds.usd}`);
    
    // Get the PDF file path
    const pdfPath = `../data/pdfs/invoice_${testData.invoiceIds.usd}.pdf`;
    
    // Define expected values in the PDF
    const expectedValues = {
      businessName: testData.business.name,
      clientName: testData.client.name,
      invoiceNumber: testData.invoices.usd.invoiceNumber,
      currency: '$', // USD symbol
      bankName: testData.business.bankNameUSD,
      iban: testData.business.ibanUSD,
      item1: testData.invoices.usd.items[0].description,
      item2: testData.invoices.usd.items[1].description
    };
    
    // Validate the PDF content
    const validationResult = await validatePDF(pdfPath, expectedValues);
    
    // Assert that validation succeeded
    expect(validationResult.success, `PDF validation failed: ${validationResult.errors.join(', ')}`).toBeTruthy();
  });
}); 