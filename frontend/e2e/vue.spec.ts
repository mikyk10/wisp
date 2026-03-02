import { test, expect } from '@playwright/test'

// These tests run against the actual dev/preview server (mock mode — no backend).
// They verify the most important user-facing flows using public/mock-data/photos.ndjson.

test.describe('App startup', () => {
  test('loads and shows the app bar with title', async ({ page }) => {
    await page.goto('/')

    // App bar title
    await expect(page.getByText('WiSP')).toBeVisible()
  })

  test('shows the catalog selector in the app bar', async ({ page }) => {
    await page.goto('/')

    // Vuetify v-select for catalog selection
    const select = page.locator('.v-select')
    await expect(select).toBeVisible()
  })

  test('loads photos and shows the photo count chip', async ({ page }) => {
    await page.goto('/')

    // Wait for the streaming to start and at least one photo to appear
    // The chip shows "N枚" once photos are loaded
    const countChip = page.locator('.v-chip').filter({ hasText: '枚' }).first()
    await expect(countChip).toBeVisible({ timeout: 15_000 })
  })
})

test.describe('Photo grid', () => {
  test('renders photo cards inside the grid', async ({ page }) => {
    await page.goto('/')

    // Wait for at least one photo card to appear
    const card = page.locator('.photo-item').first()
    await expect(card).toBeVisible({ timeout: 15_000 })
  })

})

test.describe('Photo selection', () => {
  test('clicking a photo enters selection mode', async ({ page }) => {
    await page.goto('/')

    // Wait for at least one card
    const firstCard = page.locator('.photo-item').first()
    await expect(firstCard).toBeVisible({ timeout: 15_000 })

    await firstCard.click()

    // Selection toolbar slides in from the bottom
    const toolbar = page.locator('.selection-toolbar')
    await expect(toolbar).toBeVisible()

    // Selection count chip in the app bar shows "1枚選択"
    const selectionChip = page.locator('.v-chip').filter({ hasText: '枚選択' })
    await expect(selectionChip).toBeVisible()
  })

  test('Cancel button clears selection', async ({ page }) => {
    await page.goto('/')

    const firstCard = page.locator('.photo-item').first()
    await expect(firstCard).toBeVisible({ timeout: 15_000 })
    await firstCard.click()

    const toolbar = page.locator('.selection-toolbar')
    await expect(toolbar).toBeVisible()

    // Click the Cancel button
    await toolbar.getByText('キャンセル').click()

    // Toolbar should disappear
    await expect(toolbar).not.toBeVisible()
  })

  test('Timeline sidebar is visible', async ({ page }) => {
    await page.goto('/')

    const timeline = page.locator('.timeline-scrollbar')
    await expect(timeline).toBeVisible()

    // At least one month entry should appear once photos are loaded
    const entry = page.locator('.timeline-entry').first()
    await expect(entry).toBeVisible({ timeout: 15_000 })
  })
})
