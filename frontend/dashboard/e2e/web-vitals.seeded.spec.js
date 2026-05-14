const { test, expect } = require("playwright/test");
const { login } = require("./support/auth");

const TABLE_SETTLE_MS = 1000;
const PRIMARY_SEEDED_SITE_DOMAIN = "acme-analytics.io";

async function selectSeededSite(page, domain = PRIMARY_SEEDED_SITE_DOMAIN) {
    const combobox = page.locator('[role="combobox"]:visible').first();
    await expect(combobox).toBeVisible();

    const currentSite = ((await combobox.textContent()) || "").trim();
    if (currentSite.includes(domain)) {
        return;
    }

    await page.locator('[aria-label="Select a site to view stats"]:visible').first().click();

    const option = page.locator('[role="option"]:visible').filter({ hasText: domain }).first();
    await expect(option).toBeVisible();
    await option.click();

    await expect(combobox).toContainText(domain);
    await page.waitForTimeout(TABLE_SETTLE_MS);
}

test("web vitals dashboard renders seeded data and filters", async ({ page }) => {
    await login(page, "/web-vitals");
    await selectSeededSite(page);

    await expect(page.getByRole("button", { name: "Inspect LCP over time" })).toBeVisible();
    await expect(page.getByRole("button", { name: "Inspect INP over time" })).toBeVisible();
    await expect(page.getByRole("button", { name: "Inspect CLS over time" })).toBeVisible();
    await expect(page.getByText("Path: /")).toBeVisible();
    await expect(page.getByRole("heading", { name: "LCP p75 trend" })).toBeVisible();
    await expect(page.getByRole("heading", { name: "Rating mix" })).toBeVisible();
    await expect(page.getByRole("heading", { name: "Page breakdown" })).toBeVisible();
    await expect(page.locator(".web-vitals-table tbody tr").filter({ hasText: "/" }).first()).toBeVisible();

    await page.getByRole("button", { name: "Inspect INP over time" }).click();
    await expect(page.getByRole("heading", { name: "INP p75 trend" })).toBeVisible();

    await page.getByRole("button", { name: /^Good\s+\d+/ }).click();
    await expect(page.getByText("Rating: Good")).toBeVisible();

    await page.locator(".web-vitals-table tbody tr").filter({ hasText: "/pricing" }).first().click();
    await expect(page.getByText("Path: /pricing")).toBeVisible();
    await expect(page.locator(".web-vitals-table tbody tr").filter({ hasText: "/pricing" }).first()).toBeVisible();
    await page.getByRole("tab", { name: "Browsers" }).click();
    const browsersPanel = page.getByRole("tabpanel", { name: "Browsers" });
    await expect(browsersPanel).toBeVisible();
    await expect(
        browsersPanel
            .locator(".web-vitals-table tbody tr")
            .filter({ hasText: /Chrome|Safari|Firefox|Edge/ })
            .first()
    ).toBeVisible();

    await page.getByRole("button", { name: "Clear all" }).click();
    await expect(page.getByText("Path: /")).toBeVisible();
    await expect(page.getByText("Rating: Good")).toHaveCount(0);
});
