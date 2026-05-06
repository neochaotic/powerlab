import { test, expect } from '@playwright/test';

test.describe('Orchestrator Smoke Test', () => {
  test('should load the orchestrator page and show the editor', async ({ page }) => {
    // 1. Navigate to the orchestrator
    await page.goto('/apps/new');

    // 2. Check if the header title is correct
    await expect(page.locator('h1')).toContainText('New Service');

    // 3. Check if the YAML editor textarea exists and has default content
    const yamlEditor = page.locator('textarea');
    await expect(yamlEditor).toBeVisible();
    await expect(yamlEditor).toContainText("version: '3'");

    // 4. Check if the Mac buttons (controls) are present
    const macControls = page.locator('.absolute.left-6.top-6.z-20');
    await expect(macControls).toBeVisible();

    // 5. Test view switching
    const yamlButton = page.locator('button:has-text("YAML")');
    await yamlButton.click();
    
    // In YAML mode, the editor should be full width (or at least visible)
    await expect(yamlEditor).toBeVisible();
  });
});
