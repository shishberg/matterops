import { test, expect } from '@playwright/test';

test.describe('MatterOps Dashboard', () => {
  test('shows page title', async ({ page }) => {
    await page.goto('/');
    await expect(page.locator('h1')).toHaveText('MatterOps');
  });

  test('shows service table', async ({ page }) => {
    await page.goto('/');
    await expect(page.locator('table')).toBeVisible();
    await expect(page.locator('th').first()).toHaveText('Service');
  });

  test('shows service status', async ({ page }) => {
    await page.goto('/');
    await expect(page.locator('td strong')).toContainText(['echo']);
  });

  test('has JSON API endpoint', async ({ request }) => {
    const response = await request.get('/api/status');
    expect(response.ok()).toBeTruthy();
    const data = await response.json();
    expect(data).toHaveProperty('echo');
  });

  test('clicking a service row toggles output', async ({ page }) => {
    await page.goto('/');
    const detailRow = page.locator('.detail-row').first();
    await expect(detailRow).not.toBeVisible();

    await page.locator('.expand-btn').first().click();
    await expect(detailRow).toBeVisible();

    await page.locator('.expand-btn').first().click();
    await expect(detailRow).not.toBeVisible();
  });
});
