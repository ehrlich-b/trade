import { defineConfig } from '@playwright/test'

// Use a different port for tests to avoid conflicts with dev server
const TEST_PORT = 18088

export default defineConfig({
  testDir: './e2e',
  timeout: 30000,
  use: {
    baseURL: `http://localhost:${TEST_PORT}`,
    headless: true,
  },
  webServer: {
    command: `cd .. && rm -f test-*.db test-*.db-shm test-*.db-wal && make build && ./bin/trade -port ${TEST_PORT} -db test-${Date.now()}.db`,
    url: `http://localhost:${TEST_PORT}`,
    reuseExistingServer: false,
    timeout: 60000,
  },
})
