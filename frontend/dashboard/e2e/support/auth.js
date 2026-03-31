const { expect } = require("playwright/test");

const E2E_EMAIL = process.env.HITKEEP_E2E_EMAIL || "demo@example.com";
const E2E_PASSWORD = process.env.HITKEEP_E2E_PASSWORD || "demo1234";
const E2E_SHARE_TOKEN = process.env.HITKEEP_E2E_SHARE_TOKEN || "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef";

async function login(page, returnPath = "/dashboard") {
    await page.goto(`/login?returnUrl=${encodeURIComponent(returnPath)}`, {
        waitUntil: "domcontentloaded"
    });
    await page.locator('input[type="email"], input[name="email"]').first().fill(E2E_EMAIL);
    await page.locator('input[type="password"]').first().fill(E2E_PASSWORD);
    await page.locator('button[type="submit"]').first().click();
    await page.waitForURL((url) => !url.pathname.includes("/login"), { timeout: 15_000 });
}

async function expectAuthPage(page, title) {
    await expect(page.getByRole("heading", { name: title })).toBeVisible();
}

module.exports = {
    E2E_EMAIL,
    E2E_PASSWORD,
    E2E_SHARE_TOKEN,
    expectAuthPage,
    login
};
