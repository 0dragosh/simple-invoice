const { test, expect } = require('@playwright/test');

test('homepage loads', async ({ page }) => {
  await page.goto('/');
  await expect(page.getByRole('heading', { name: 'Simple Invoice', level: 1 })).toBeVisible();
  await page.screenshot({ path: 'homepage.png' });
}); 