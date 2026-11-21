import { defineConfig } from '@playwright/test'

const origin = 'http://localhost:8090'

// The one smoke test (architecture §15), against the real server on a fresh, seeded database: run it with
// `make e2e`, which prepares both and passes the server its settings.
export default defineConfig({
  testDir: 'e2e',
  testMatch: '*.e2e.ts',
  reporter: 'list',
  forbidOnly: true,
  // The Chrome already installed, here and on CI's runners: nothing to download.
  use: { baseURL: origin, channel: 'chrome' },
  // DATABASE_URL comes from `make e2e`. Its request log shows in the output (it's on stderr, with any crash),
  // which helps when a run fails on CI.
  webServer: {
    command: '../bin/family-hub',
    url: `${origin}/health`,
    reuseExistingServer: false,
    env: { HTTP_ADDR: `:${new URL(origin).port}`, BASE_URL: origin, COOKIE_SECURE: 'false' },
  },
})
