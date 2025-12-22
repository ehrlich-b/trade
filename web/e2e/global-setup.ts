import { chromium } from '@playwright/test'

const TEST_PORT = 18088

async function globalSetup() {
  console.log('Waiting for server to have liquidity...')

  const browser = await chromium.launch()
  const page = await browser.newPage()

  try {
    // Navigate to the app
    await page.goto(`http://localhost:${TEST_PORT}`)

    // Register a test user to access the trading UI
    await page.click('button:has-text("Register")')
    await page.fill('input[placeholder="Enter username"]', `setup_${Date.now()}`)
    await page.fill('input[placeholder="Enter password"]', 'testpass123')
    await page.click('button[type="submit"]:has-text("Create Account")')
    await page.waitForSelector('text=PLACE ORDER', { timeout: 10000 })

    // Wait for spread to appear (indicates bots are quoting)
    // This can take up to 60+ seconds as we wait for TRADING state
    let attempts = 0
    const maxAttempts = 180 // 90 seconds max
    while (attempts < maxAttempts) {
      const bodyText = await page.evaluate(() => document.body.innerText)
      const match = bodyText.match(/Spread:\s*\$(\d+\.\d+)/)
      if (match && parseFloat(match[1]) > 0) {
        console.log(`Found liquidity after ${attempts * 500}ms`)
        break
      }
      await page.waitForTimeout(500)
      attempts++
    }

    if (attempts >= maxAttempts) {
      console.warn('Global setup: Could not find liquidity, tests may fail')
    }
  } finally {
    await browser.close()
  }
}

export default globalSetup
