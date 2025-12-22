import { test, expect } from '@playwright/test'

// Helper to wait for order book to have liquidity (bots quoting)
// Global setup already ensures liquidity exists, so this just verifies it's still there
async function waitForLiquidity(page: import('@playwright/test').Page) {
  // Short wait for WebSocket to receive order book and for bots to re-quote
  // Bots requote every 500ms-2s depending on type
  let attempts = 0
  const maxAttempts = 20 // 10 seconds max (should be fast since global setup already waited)
  while (attempts < maxAttempts) {
    const bodyText = await page.evaluate(() => document.body.innerText)
    const match = bodyText.match(/Spread:\s*\$(\d+\.\d+)/)
    if (match && parseFloat(match[1]) > 0) {
      return // Found liquidity!
    }
    await page.waitForTimeout(500)
    attempts++
  }
  throw new Error('Timed out waiting for liquidity (bots may need to re-quote)')
}

test.describe('Open Orders', () => {
  test('limit order that fills immediately does not hang', async ({ page }) => {
    await page.goto('/')

    // Register
    const username = `testuser_${Date.now()}`
    await page.click('button:has-text("Register")')
    await page.fill('input[placeholder="Enter username"]', username)
    await page.fill('input[placeholder="Enter password"]', 'testpass123')
    await page.click('button[type="submit"]:has-text("Create Account")')
    await page.waitForSelector('text=PLACE ORDER', { timeout: 5000 })

    // Wait for bots to provide liquidity
    await waitForLiquidity(page)

    // Get the current mid price from the header
    const midPriceText = await page.locator('text=MID').locator('..').locator('span').last().textContent()
    const midPrice = parseFloat(midPriceText?.replace('$', '') || '100')

    // Place a limit BUY order at a price ABOVE mid (will fill immediately against asks)
    await page.click('button:has-text("BUY")')
    await page.click('button:has-text("Limit")')
    // Set price well above mid to ensure immediate fill
    const highPrice = (midPrice + 5).toFixed(2)
    await page.locator('input[type="number"][step="0.01"]').fill(highPrice)
    await page.locator('input[type="number"][step="1"]').fill('10')

    // Submit and verify it doesn't hang (should complete within 2 seconds)
    const submitButton = page.locator('button[type="submit"]')
    await submitButton.click()

    // Wait for the button to not be in "Submitting..." state
    await expect(submitButton).not.toHaveText(/Submitting/, { timeout: 3000 })

    // Order should NOT appear in open orders (it filled immediately)
    await page.waitForTimeout(500)
    const openOrdersPanel = page.locator('text=Open Orders').locator('..')
    await expect(openOrdersPanel.locator('text=No open orders')).toBeVisible()

    // Position should have updated
    const positionPanel = page.locator('text=POSITION DETAILS').locator('..')
    await expect(positionPanel.locator('text=+10')).toBeVisible()
  })

  test('limit order appears in open orders and can be cancelled', async ({ page }) => {
    await page.goto('/')

    // Register a unique user
    const username = `testuser_${Date.now()}`
    await page.click('button:has-text("Register")')
    await page.fill('input[placeholder="Enter username"]', username)
    await page.fill('input[placeholder="Enter password"]', 'testpass123')
    await page.click('button[type="submit"]:has-text("Create Account")')

    // Wait for main UI
    await page.waitForSelector('text=PLACE ORDER', { timeout: 5000 })

    // Place a limit BUY order that won't fill (price way below market)
    await page.click('button:has-text("BUY")')
    await page.click('button:has-text("Limit")')
    // Price input is the first number input with step=0.01
    await page.locator('input[type="number"][step="0.01"]').fill('50.00')
    // Quantity input is the one with step=1
    await page.locator('input[type="number"][step="1"]').fill('25')
    await page.click('button[type="submit"]')

    // Wait for order to be processed
    await page.waitForTimeout(500)

    // Verify order appears in Open Orders section
    const openOrdersPanel = page.locator('text=Open Orders').locator('..')
    await expect(openOrdersPanel.locator('text=BUY')).toBeVisible()
    await expect(openOrdersPanel.locator('text=$50.00')).toBeVisible()
    await expect(openOrdersPanel.locator('text=25/25')).toBeVisible()

    // Cancel the order
    await openOrdersPanel.locator('button:has-text("✕")').first().click()
    await page.waitForTimeout(500)

    // Verify order is removed
    await expect(openOrdersPanel.locator('text=No open orders')).toBeVisible()
  })

  test('multiple limit orders show correctly', async ({ page }) => {
    await page.goto('/')

    // Register
    const username = `testuser_${Date.now()}`
    await page.click('button:has-text("Register")')
    await page.fill('input[placeholder="Enter username"]', username)
    await page.fill('input[placeholder="Enter password"]', 'testpass123')
    await page.click('button[type="submit"]:has-text("Create Account")')
    await page.waitForSelector('text=PLACE ORDER', { timeout: 5000 })

    // Place first limit BUY order
    await page.click('button:has-text("BUY")')
    await page.click('button:has-text("Limit")')
    await page.locator('input[type="number"][step="0.01"]').fill('45.00')
    await page.locator('input[type="number"][step="1"]').fill('10')
    await page.click('button[type="submit"]')
    await page.waitForTimeout(300)

    // Place second limit SELL order (price ABOVE market so it won't fill immediately)
    // Market is around $480, so $550 should stay open
    await page.click('button:has-text("SELL")')
    await page.locator('input[type="number"][step="0.01"]').fill('550.00')
    await page.locator('input[type="number"][step="1"]').fill('15')
    await page.click('button[type="submit"]')
    await page.waitForTimeout(300)

    // Verify both orders appear
    const openOrdersPanel = page.locator('text=Open Orders').locator('..')
    await expect(openOrdersPanel.locator('text=BUY')).toBeVisible()
    await expect(openOrdersPanel.locator('text=SELL')).toBeVisible()
    await expect(openOrdersPanel.locator('text=$45.00')).toBeVisible()
    await expect(openOrdersPanel.locator('text=$550.00')).toBeVisible()
  })

  test('market orders do not appear in open orders (they fill immediately)', async ({ page }) => {
    await page.goto('/')

    // Register
    const username = `testuser_${Date.now()}`
    await page.click('button:has-text("Register")')
    await page.fill('input[placeholder="Enter username"]', username)
    await page.fill('input[placeholder="Enter password"]', 'testpass123')
    await page.click('button[type="submit"]:has-text("Create Account")')
    await page.waitForSelector('text=PLACE ORDER', { timeout: 5000 })

    // Wait for bots to provide liquidity
    await waitForLiquidity(page)

    // Place a market BUY order
    await page.click('button:has-text("BUY")')
    await page.click('button:has-text("Market")')
    await page.fill('input[type="number"]', '5')
    await page.click('button[type="submit"]')
    await page.waitForTimeout(500)

    // Market orders fill immediately, so should not be in open orders
    const openOrdersPanel = page.locator('text=Open Orders').locator('..')
    await expect(openOrdersPanel.locator('text=No open orders')).toBeVisible()

    // But position should update
    const positionPanel = page.locator('text=POSITION DETAILS').locator('..')
    await expect(positionPanel.locator('text=+5')).toBeVisible()
  })
})
