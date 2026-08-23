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

  test('loads photos and shows the count in the filter bar', async ({ page }) => {
    await page.goto('/')

    // The count used to be a chip in the app bar. It moved into the filter bar
    // because the bar has no width to spare on a narrow screen — see
    // TagFilterBar — so this is where "N photos" is now.
    const bar = page.locator('.tag-filter-bar')
    await expect(bar).toBeVisible({ timeout: 15_000 })
    await expect(bar).toContainText('photos')
    await expect(bar).not.toHaveText('0 photos')
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

    // Selection count chip in the app bar shows "1 selected"
    const selectionChip = page.locator('.v-chip').filter({ hasText: 'selected' })
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
    await toolbar.getByText('Cancel').click()

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

test.describe('Tags', () => {
  test('a tagged photo carries a badge that opens its tags', async ({ page }) => {
    await page.goto('/')

    // The badge is the only route to a photo's tags on a touch screen, so it
    // has to be there without hovering anything.
    const badge = page.locator('.tag-badge').first()
    await expect(badge).toBeVisible({ timeout: 15_000 })

    await badge.click()

    const sheet = page.locator('.photo-tags-sheet')
    await expect(sheet).toBeVisible()
    await expect(sheet).toContainText('tag')

    // Tapping the badge must not also select the photo — the card does that.
    await expect(page.locator('.selection-toolbar')).not.toBeVisible()
  })

  test('the filter picker opens from the app bar', async ({ page }) => {
    await page.goto('/')

    await page.locator('#tag-filter-activator').click()

    await expect(page.locator('.tag-picker')).toBeVisible()
    await expect(page.locator('.tag-picker-search')).toBeVisible()
  })
})
