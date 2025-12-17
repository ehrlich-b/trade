import { test, expect } from '@playwright/test'

test.describe('Position Details', () => {
  test('position quantity updates correctly after buy', async ({ page }) => {
    await page.goto('/')

    // Register
    const username = `testuser_${Date.now()}`
    await page.click('button:has-text("Register")')
    await page.fill('input[placeholder="Enter username"]', username)
    await page.fill('input[placeholder="Enter password"]', 'testpass123')
    await page.click('button[type="submit"]:has-text("Create Account")')
    await page.waitForSelector('text=PLACE ORDER', { timeout: 5000 })

    // Check initial position is 0
    const positionPanel = page.locator('text=POSITION DETAILS').locator('..')
    const qtyElement = positionPanel.locator('text=Quantity').locator('..').locator('span').last()
    expect(await qtyElement.textContent()).toBe('0')

    // Buy 10 shares
    await page.click('button:has-text("BUY")')
    await page.click('button:has-text("Market")')
    await page.fill('input[type="number"]', '10')
    await page.click('button[type="submit"]')
    await page.waitForTimeout(1000)

    // Verify position updated
    expect(await qtyElement.textContent()).toBe('+10')

    // Verify avg price is set
    const avgPriceElement = positionPanel.locator('text=Avg Price').locator('..').locator('span').last()
    const avgPrice = await avgPriceElement.textContent()
    expect(avgPrice).not.toBe('—')
    expect(avgPrice).toContain('$')
  })

  test('position quantity updates correctly after sell', async ({ page }) => {
    await page.goto('/')

    // Register
    const username = `testuser_${Date.now()}`
    await page.click('button:has-text("Register")')
    await page.fill('input[placeholder="Enter username"]', username)
    await page.fill('input[placeholder="Enter password"]', 'testpass123')
    await page.click('button[type="submit"]:has-text("Create Account")')
    await page.waitForSelector('text=PLACE ORDER', { timeout: 5000 })

    const positionPanel = page.locator('text=POSITION DETAILS').locator('..')
    const qtyElement = positionPanel.locator('text=Quantity').locator('..').locator('span').last()

    // Sell 10 shares (short position)
    await page.click('button:has-text("SELL")')
    await page.click('button:has-text("Market")')
    await page.fill('input[type="number"]', '10')
    await page.click('button[type="submit"]')
    await page.waitForTimeout(1000)

    // Verify short position
    expect(await qtyElement.textContent()).toBe('-10')
  })

  test('position accumulates across multiple trades', async ({ page }) => {
    await page.goto('/')

    // Register
    const username = `testuser_${Date.now()}`
    await page.click('button:has-text("Register")')
    await page.fill('input[placeholder="Enter username"]', username)
    await page.fill('input[placeholder="Enter password"]', 'testpass123')
    await page.click('button[type="submit"]:has-text("Create Account")')
    await page.waitForSelector('text=PLACE ORDER', { timeout: 5000 })

    const positionPanel = page.locator('text=POSITION DETAILS').locator('..')
    const qtyElement = positionPanel.locator('text=Quantity').locator('..').locator('span').last()

    // Buy 10
    await page.click('button:has-text("BUY")')
    await page.click('button:has-text("Market")')
    await page.fill('input[type="number"]', '10')
    await page.click('button[type="submit"]')
    await page.waitForTimeout(500)

    // Buy another 15
    await page.fill('input[type="number"]', '15')
    await page.click('button[type="submit"]')
    await page.waitForTimeout(1000)

    // Should be 25
    expect(await qtyElement.textContent()).toBe('+25')

    // Sell 10
    await page.click('button:has-text("SELL")')
    await page.fill('input[type="number"]', '10')
    await page.click('button[type="submit"]')
    await page.waitForTimeout(1000)

    // Should be 15
    expect(await qtyElement.textContent()).toBe('+15')
  })

  test('cash balance updates after trades', async ({ page }) => {
    await page.goto('/')

    // Register
    const username = `testuser_${Date.now()}`
    await page.click('button:has-text("Register")')
    await page.fill('input[placeholder="Enter username"]', username)
    await page.fill('input[placeholder="Enter password"]', 'testpass123')
    await page.click('button[type="submit"]:has-text("Create Account")')
    await page.waitForSelector('text=PLACE ORDER', { timeout: 5000 })

    // Get initial cash from header
    const cashElement = page.locator('text=CASH').locator('..').locator('span').last()
    const initialCash = await cashElement.textContent()
    expect(initialCash).toBe('$1.00M') // $1M starting balance

    // Buy shares
    await page.click('button:has-text("BUY")')
    await page.click('button:has-text("Market")')
    await page.fill('input[type="number"]', '100')
    await page.click('button[type="submit"]')
    await page.waitForTimeout(1000)

    // Cash should decrease
    const newCash = await cashElement.textContent()
    expect(newCash).not.toBe('$1.00M')
  })

  test('P&L displays correctly in header and position details', async ({ page }) => {
    await page.goto('/')

    // Register
    const username = `testuser_${Date.now()}`
    await page.click('button:has-text("Register")')
    await page.fill('input[placeholder="Enter username"]', username)
    await page.fill('input[placeholder="Enter password"]', 'testpass123')
    await page.click('button[type="submit"]:has-text("Create Account")')
    await page.waitForSelector('text=PLACE ORDER', { timeout: 5000 })

    // Buy shares
    await page.click('button:has-text("BUY")')
    await page.click('button:has-text("Market")')
    await page.fill('input[type="number"]', '10')
    await page.click('button[type="submit"]')
    await page.waitForTimeout(1000)

    // P&L should be visible in header
    const headerPnl = page.locator('header').locator('text=P&L').locator('..').locator('span').last()
    const pnlText = await headerPnl.textContent()
    expect(pnlText).toContain('$')

    // Realized P&L should be displayed (may be $0 for new positions, or small amount from spread)
    const positionPanel = page.locator('text=POSITION DETAILS').locator('..')
    const realizedPnl = positionPanel.locator('text=Realized P&L').locator('..').locator('span').last()
    const realizedText = await realizedPnl.textContent()
    // Just verify it contains a dollar amount (may be $0.00 or small amount from prior trades)
    expect(realizedText).toContain('$')

    // Unrealized P&L should be visible and contain $ sign
    // It may be positive or negative depending on spread/market movement
    const unrealizedPnl = positionPanel.locator('text=Unrealized P&L').locator('..').locator('span').last()
    const unrealizedText = await unrealizedPnl.textContent()
    expect(unrealizedText).toContain('$')
  })
})
