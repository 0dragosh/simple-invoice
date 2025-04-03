const { test, expect } = require('@playwright/test');

// Test navigation to all main pages
test.describe('Navigation Tests', () => {
  
  test('can navigate to home page', async ({ page }) => {
    await page.goto('/');
    await expect(page.getByRole('heading', { name: 'Simple Invoice', level: 1 })).toBeVisible();
    await page.screenshot({ path: 'screenshots/home.png' });
  });
  
  test('can navigate to business page', async ({ page }) => {
    await page.goto('/business');
    await expect(page.getByRole('heading', { name: 'Business Details', level: 2 })).toBeVisible();
    await expect(page.locator('#name')).toBeVisible();
    await page.screenshot({ path: 'screenshots/business.png' });
  });
  
  test('can navigate to clients page', async ({ page }) => {
    await page.goto('/clients');
    await expect(page.getByRole('heading', { name: 'Clients', level: 2 })).toBeVisible();
    await expect(page.getByRole('button', { name: 'Add Client' })).toBeVisible();
    await page.screenshot({ path: 'screenshots/clients.png' });
  });
  
  test('can navigate to invoices page', async ({ page }) => {
    await page.goto('/invoices');
    await expect(page.getByRole('heading', { name: 'Invoices', level: 2 })).toBeVisible();
    await expect(page.getByRole('link', { name: 'Create New Invoice' })).toBeVisible();
    await page.screenshot({ path: 'screenshots/invoices.png' });
  });
}); 