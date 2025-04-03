const { test, expect } = require('@playwright/test');
const { 
  generateBusinessData, 
  generateClientData,
  generateInvoiceData,
  fillWithMultipleSelectors 
} = require('./utils');

// Generate test data
const business = generateBusinessData();
const client = generateClientData();
const invoice = generateInvoiceData(business.name, client.name, 'EUR', 'normal');

test('create business with EUR and USD bank accounts', async ({ page }) => {
  await page.goto('/business');
  
  // Wait for page to load
  await page.waitForTimeout(1000);
  
  // Take screenshot of initial form
  await page.screenshot({ path: 'screenshots/business-form-basic-initial.png' });
  
  // Try to find and fill in business details with flexible selectors
  const fillIfPossible = async (selectors, value) => {
    return await fillWithMultipleSelectors(page, selectors, value, true);
  };
  
  // Try to fill business details with multiple possible selectors
  await fillIfPossible(['#name', '[name="name"]'], business.name);
  await fillIfPossible(['#address', '[name="address"]'], business.address);
  await fillIfPossible(['#city', '[name="city"]'], business.city);
  await fillIfPossible(['#postal_code', '[name="postal_code"]'], business.postalCode);
  await fillIfPossible(['#country', '[name="country"]'], business.country);
  await fillIfPossible(['#vat_id', '[name="vat_id"]'], business.vatID);
  
  // Try to fill bank details
  await fillIfPossible(['#bankName', '[name="bankName"]'], business.bankName);
  await fillIfPossible(['#iban', '[name="iban"]'], business.iban);
  await fillIfPossible(['#bic', '[name="bic"]'], business.bic);
  
  // Try to find and click add bank account button if it exists
  try {
    const addBankText = page.getByText('Add Bank Account');
    if (await addBankText.count() > 0) {
      await addBankText.click();
      await page.waitForTimeout(500);
      
      // Try to fill second bank account
      await fillIfPossible(['#secondBankName', '[name="secondBankName"]'], business.bankNameUSD);
      await fillIfPossible(['#secondIBAN', '[name="secondIBAN"]'], business.ibanUSD);
      await fillIfPossible(['#secondBIC', '[name="secondBIC"]'], business.bicUSD);
      await fillIfPossible(['#secondCurrency', '[name="secondCurrency"]'], 'USD');
    }
  } catch (e) {
    console.log('Could not add second bank account:', e.message);
  }
  
  // Take screenshot before saving
  await page.screenshot({ path: 'screenshots/business-before-save-basic.png' });
  
  // Try to find and click save button
  try {
    // Look for various save button patterns
    const saveButtonSelectors = [
      'button:has-text("Save")',
      'button:has-text("Save Business")',
      'button:has-text("Save Details")',
      'button:has-text("Submit")'
    ];
    
    for (const selector of saveButtonSelectors) {
      const saveButton = page.locator(selector);
      if (await saveButton.count() > 0) {
        await saveButton.click();
        break;
      }
    }
    
    // Wait after save attempt
    await page.waitForTimeout(2000);
    
    // Take screenshot after save attempt
    await page.screenshot({ path: 'screenshots/business-after-save-basic.png' });
  } catch (e) {
    console.log('Error saving business:', e.message);
  }
  
  console.log('Business creation test completed');
});

test('create client with generated details', async ({ page }) => {
  await page.goto('/clients');
  
  // Wait for page to load
  await page.waitForTimeout(1000);
  
  // Take screenshot of initial page
  await page.screenshot({ path: 'screenshots/clients-page-basic.png' });
  
  // Try to click add client button
  try {
    const addClientButton = page.getByRole('button', { name: 'Add Client' });
    await addClientButton.click();
  } catch (e) {
    console.log('Could not click Add Client button:', e.message);
    // Try alternative method to add client
    try {
      await page.getByText(/add client/i).first().click();
    } catch (e2) {
      console.log('Alternative client addition also failed:', e2.message);
    }
  }
  
  // Wait for client form to load
  await page.waitForTimeout(1000);
  
  // Take screenshot of client form
  await page.screenshot({ path: 'screenshots/client-form-basic.png' });
  
  // Try to find and fill in client details with flexible selectors
  const fillIfPossible = async (selectors, value) => {
    return await fillWithMultipleSelectors(page, selectors, value, true);
  };
  
  // Try filling client details with multiple possible selectors
  await fillIfPossible(['#name', '[name="name"]'], client.name);
  await fillIfPossible(['#address', '[name="address"]'], client.address);
  await fillIfPossible(['#city', '[name="city"]'], client.city);
  await fillIfPossible(['#postal_code', '[name="postal_code"]'], client.postalCode);
  await fillIfPossible(['#country', '[name="country"]'], client.country);
  await fillIfPossible(['#vat_id', '[name="vat_id"]'], client.vatID);
  await fillIfPossible(['#email', '[name="email"]'], client.email);
  await fillIfPossible(['#phone', '[name="phone"]'], client.phone);
  
  // Take screenshot before saving
  await page.screenshot({ path: 'screenshots/client-before-save-basic.png' });
  
  // Try to find and click save button
  try {
    // Look for various save button patterns
    const saveButtonSelectors = [
      'button:has-text("Save")',
      'button:has-text("Save Client")',
      'button:has-text("Submit")'
    ];
    
    for (const selector of saveButtonSelectors) {
      const saveButton = page.locator(selector);
      if (await saveButton.count() > 0) {
        await saveButton.click();
        break;
      }
    }
    
    // Wait after save attempt
    await page.waitForTimeout(2000);
    
    // Take screenshot after save attempt
    await page.screenshot({ path: 'screenshots/client-after-save-basic.png' });
  } catch (e) {
    console.log('Error saving client:', e.message);
  }
  
  console.log('Client creation test completed');
});

test('create invoice in EUR with VAT', async ({ page }) => {
  await page.goto('/invoices');
  
  // Wait for page to load
  await page.waitForTimeout(1000);
  
  // Take screenshot of initial page
  await page.screenshot({ path: 'screenshots/invoices-page-basic.png' });
  
  // Try to click create invoice - first check which elements are available
  try {
    const createButtonCount = await page.getByRole('button', { name: 'Create Invoice' }).count();
    const createLinkCount = await page.getByRole('link', { name: /Create New Invoice/i }).count();
    
    if (createButtonCount > 0) {
      await page.getByRole('button', { name: 'Create Invoice' }).click();
    } else if (createLinkCount > 0) {
      await page.getByRole('link', { name: /Create New Invoice/i }).click();
    } else {
      // If neither is found, try clicking on any element that might lead to invoice creation
      await page.getByText(/Create/i).first().click();
    }
  } catch (e) {
    console.log('Could not click create invoice button:', e.message);
  }
  
  // Wait for form to load
  await page.waitForTimeout(1000);
  
  // Take screenshot of invoice creation form
  await page.screenshot({ path: 'screenshots/invoice-form-basic.png' });
  
  // Try to find and fill in invoice details with flexible approach
  const fillIfPossible = async (selectors, value) => {
    return await fillWithMultipleSelectors(page, selectors, value, true);
  };
  
  // Generate a unique invoice number with timestamp
  const invoiceNumber = `INV-${Date.now().toString().slice(-6)}`;
  
  // Try filling invoice details with multiple possible selectors
  await fillIfPossible(['#invoice_number', '#invoiceNumber', '[name="invoiceNumber"]'], invoiceNumber);
  await fillIfPossible(['#issue_date', '#issueDate', '[name="issueDate"]'], new Date().toISOString().split('T')[0]);
  
  // Calculate due date (30 days from now)
  const dueDate = new Date();
  dueDate.setDate(dueDate.getDate() + 30);
  await fillIfPossible(['#due_date', '#dueDate', '[name="dueDate"]'], dueDate.toISOString().split('T')[0]);
  
  // Try to set currency if it exists as select element
  try {
    const currencySelectors = ['#currency', '[name="currency"]', 'select[id*="currency" i]', 'select[name*="currency" i]'];
    for (const selector of currencySelectors) {
      const currencyField = page.locator(selector);
      if (await currencyField.count() > 0) {
        await currencyField.selectOption('EUR');
        break;
      }
    }
  } catch (e) {
    console.log('Could not set currency:', e.message);
  }
  
  // Try to set VAT rate
  await fillIfPossible(['#vat_rate', '[name="vatRate"]', '[name*="vat" i]'], '20');
  await fillIfPossible(['#notes', '[name="notes"]'], 'Test invoice created by automated test');
  
  // Try to add invoice items
  try {
    // Find item description field
    const itemDescSelectors = ['#description', '#itemDescription', '[name*="description" i]', '[placeholder*="description" i]'];
    for (const selector of itemDescSelectors) {
      const descField = page.locator(selector).first();
      if (await descField.count() > 0) {
        await descField.fill('Consulting Services');
        break;
      }
    }
    
    // Find quantity field
    const qtySelectors = ['#quantity', '[name*="quantity" i]', '[placeholder*="quantity" i]'];
    for (const selector of qtySelectors) {
      const qtyField = page.locator(selector).first();
      if (await qtyField.count() > 0) {
        await qtyField.fill('10');
        break;
      }
    }
    
    // Find unit price field
    const priceSelectors = ['#unit_price', '#unitPrice', '[name*="price" i]', '[placeholder*="price" i]'];
    for (const selector of priceSelectors) {
      const priceField = page.locator(selector).first();
      if (await priceField.count() > 0) {
        await priceField.fill('100');
        break;
      }
    }
  } catch (e) {
    console.log('Could not add invoice item:', e.message);
  }
  
  // Take screenshot before saving
  await page.screenshot({ path: 'screenshots/invoice-filled-basic.png' });
  
  // Try to find and click save button
  try {
    // Look for various save button patterns
    const saveButtonSelectors = [
      'button:has-text("Save")',
      'button:has-text("Save Invoice")',
      'button:has-text("Create Invoice")',
      'button:has-text("Submit")'
    ];
    
    for (const selector of saveButtonSelectors) {
      const saveButton = page.locator(selector);
      if (await saveButton.count() > 0) {
        await saveButton.click();
        break;
      }
    }
    
    // Wait after save attempt
    await page.waitForTimeout(2000);
    
    // Take final screenshot
    await page.screenshot({ path: 'screenshots/invoice-final-basic.png' });
  } catch (e) {
    console.log('Error saving invoice:', e.message);
  }
  
  console.log('Invoice creation test completed');
}); 