// @ts-check
const { defineConfig } = require('@playwright/test');

/**
 * @see https://playwright.dev/docs/test-configuration
 */
module.exports = defineConfig({
  testDir: './tests',
  // Maximum time one test can run for
  timeout: 60 * 1000,
  expect: {
    /**
     * Maximum time expect() should wait for the condition to be met
     */
    timeout: 10000
  },
  // Fail the build on CI if you accidentally left test.only in the source code
  forbidOnly: !!process.env.CI,
  // Retry on CI only
  retries: process.env.CI ? 2 : 0,
  // Workers parallelism
  workers: process.env.CI ? 1 : undefined,
  
  // Reporter to use
  reporter: process.env.CI ? 'github' : 'list',
  
  // Configure projects for browsers
  projects: [
    {
      name: 'chromium',
      use: {
        // Browser options
        headless: true,
        viewport: { width: 1280, height: 720 },
        ignoreHTTPSErrors: true,
        // Context options
        baseURL: process.env.BASE_URL || 'http://localhost:8080',
        // Artifacts
        video: process.env.CI ? 'on-first-retry' : 'off',
        screenshot: process.env.CI ? 'only-on-failure' : 'off',
        // Set a generous timeout for navigations
        navigationTimeout: 30000,
        // Set a generous timeout for actions
        actionTimeout: 15000,
      },
    },
  ],

  // Local dev server - only enable in CI
  webServer: process.env.CI ? {
    command: process.env.CI 
      ? '../app' 
      : 'cd .. && go run cmd/server/main.go',
    port: 8080,
    reuseExistingServer: true,
    timeout: 120 * 1000,
    env: {
      DATA_DIR: process.env.DATA_DIR || '../data',
      LOG_LEVEL: process.env.LOG_LEVEL || 'DEBUG',
    }
  } : undefined,
}); 