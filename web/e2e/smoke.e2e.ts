import { expect, test } from '@playwright/test'

// Nadège Pluviose in the seed's made-up family (internal/database/seed.sql).
const nadege = '019a0000-0000-7000-8000-000000000008'

test('an admin signs in, adds a relative, and finds them on the tree', async ({ page }) => {
  await page.goto('/auth/login')
  await page.getByLabel('Email').fill('maya.pluviose@example.com')
  await page.getByLabel('Password', { exact: true }).fill('sample-family-dev')
  await page.getByRole('button', { name: 'Sign in' }).click()
  await expect(page.getByRole('heading', { level: 1, name: /Maya$/ })).toBeVisible()

  await page.goto(`/people/${nadege}`)
  await page.getByRole('button', { name: 'Add a child' }).click()
  const sheet = page.getByRole('dialog')
  await sheet.getByRole('searchbox', { name: 'Name' }).fill('Paolo Smoketest')
  await sheet.getByRole('button', { name: /^Add “Paolo Smoketest” as someone new/ }).click()
  await sheet.getByRole('button', { name: 'Born to Nadège' }).click()
  await sheet.getByRole('button', { name: /^Someone else/ }).click()
  await sheet.getByRole('button', { name: 'Add Paolo' }).click()
  await expect(page.getByText('Paolo added as Nadège’s child.')).toBeVisible()

  await page.getByRole('link', { name: 'Center the tree on Nadège' }).click()
  await expect(page.getByRole('button', { name: /^Paolo Smoketest/ })).toBeVisible()
})
