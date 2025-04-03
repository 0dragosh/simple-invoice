const { test, expect } = require('@playwright/test');
const { 
  generateBusinessData, 
  generateClientData,
  generateInvoiceData
} = require('./utils');

// Generate test data
const business = generateBusinessData();
const client = generateClientData();

test('business page test', async ({ page }) => {
  // Navigate to business page
  await page.goto('/business');
  
  // Check that we're on the right page
  await expect(page.getByRole('heading', { name: 'Business Details', level: 1 })).toBeVisible();
  
  // Fill business name and address
  await page.locator('#name').fill(business.name);
  await page.locator('#address').fill(business.address);
  
  // Fill other fields if they exist
  const fillIfExists = async (selector, value) => {
    const count = await page.locator(selector).count();
    if (count > 0) {
      await page.locator(selector).fill(value);
    }
  };
  
  await fillIfExists('#city', business.city);
  await fillIfExists('#postal_code', business.postalCode);
  await fillIfExists('#country', business.country);
  await fillIfExists('#vat_id', business.vatID);
  await fillIfExists('#email', business.email);
  
  // Fill bank details
  await fillIfExists('#bankName', business.bankName);
  await fillIfExists('#iban', business.iban);
  await fillIfExists('#bic', business.bic);
  
  // Take a screenshot before saving
  await page.screenshot({ path: 'screenshots/business-before-save.png' });
  
  // Get the save button
  const saveButton = page.getByRole('button', { name: /Save/i });
  
  // Check if the button is visible and enabled
  await expect(saveButton).toBeVisible();
  
  // Click the save button
  await saveButton.click();
  
  // Wait a moment
  await page.waitForTimeout(2000);
  
  // Take a screenshot after saving
  await page.screenshot({ path: 'screenshots/business-after-save.png' });
});

test('client page test', async ({ page }) => {
  // Navigate to clients page
  await page.goto('/clients');
  
  // Check that we're on the right page
  await expect(page.getByRole('heading', { name: 'Clients', level: 1 })).toBeVisible();
  
  // Click add client button
  await page.getByRole('button', { name: 'Add Client' }).click();
  
  // Fill required fields
  const fillIfExists = async (label, value) => {
    const el = page.getByLabel(new RegExp(label, 'i'));
    const count = await el.count();
    if (count > 0) {
      await el.fill(value);
    }
  };
  
  await fillIfExists('Client Name', client.name);
  await fillIfExists('Address', client.address);
  await fillIfExists('City', client.city);
  await fillIfExists('Postal Code', client.postalCode);
  await fillIfExists('Country', client.country);
  await fillIfExists('VAT ID', client.vatID);
  await fillIfExists('Email', client.email);
  await fillIfExists('Phone', client.phone);
  
  // Take a screenshot before saving
  await page.screenshot({ path: 'screenshots/client-before-save.png' });
  
  // Get the save button
  const saveButton = page.getByRole('button', { name: /Save/i });
  
  // Check if the button is visible and enabled
  await expect(saveButton).toBeVisible();
  
  // Click the save button
  await saveButton.click();
  
  // Wait a moment
  await page.waitForTimeout(2000);
  
  // Take a screenshot after saving
  await page.screenshot({ path: 'screenshots/client-after-save.png' });
});

test('invoice page test', async ({ page }) => {
  // Navigate to invoices page
  await page.goto('/invoices');
  
  // Check that we're on the right page
  await expect(page.getByRole('heading', { name: 'Invoices', level: 1 })).toBeVisible();
  
  // Click create invoice button (either a button or a link)
  const createButton = page.getByRole('button', { name: /Create Invoice/i });
  const createLink = page.getByRole('link', { name: /Create New Invoice/i });
  
  if (await createButton.count() > 0) {
    await createButton.click();
  } else {
    await createLink.click();
  }
  
  // Wait a moment for the page to load
  await page.waitForTimeout(2000);
  
  // Take a screenshot of the invoice creation page
  await page.screenshot({ path: 'screenshots/invoice-creation.png' });
}); 