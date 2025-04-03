// @ts-check
const { defineConfig } = require('@playwright/test');

/**
 * @see https://playwright.dev/docs/test-configuration
 */
module.exports = defineConfig({
  testDir: './tests',
  // Maximum time one test can run for
  timeout: 30 * 1000,
  expect: {
    /**
     * Maximum time expect() should wait for the condition to be met
     */
    timeout: 5000
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
      },
    },
  ],

  // Local dev server
  webServer: {
    command: 'cd .. && go run cmd/server/main.go',
    port: 8080,
    reuseExistingServer: !process.env.CI,
    timeout: 60 * 1000,
  },
}); 