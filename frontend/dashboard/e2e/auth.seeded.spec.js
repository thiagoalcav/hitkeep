const { test, expect } = require("playwright/test");
const { E2E_EMAIL, E2E_PASSWORD, expectAuthPage, login } = require("./support/auth");

test("protected routes redirect to login and return after authentication", async ({ page }) => {
    await page.goto("/events", { waitUntil: "domcontentloaded" });
    await expect(page).toHaveURL(/\/login\?returnUrl=%2Fevents/);
    await expectAuthPage(page, "Sign in");

    await page.locator('input[type="email"], input[name="email"]').first().fill(E2E_EMAIL);
    await page.locator('input[type="password"]').first().fill(E2E_PASSWORD);
    await page.locator('button[type="submit"]').first().click();

    await expect(page).toHaveURL(/\/events(?:\?.*)?$/);
    await expect(page.getByText("Event activity")).toBeVisible();
});

test("login shows an error for invalid credentials", async ({ page }) => {
    await page.goto("/login", { waitUntil: "domcontentloaded" });
    await page.locator('input[type="email"], input[name="email"]').first().fill(E2E_EMAIL);
    await page.locator('input[type="password"]').first().fill("definitely-wrong-password");
    await page.locator('button[type="submit"]').first().click();

    await expect(page).toHaveURL(/\/login/);
    await expect(page.getByRole("alert")).toContainText("Invalid email or password.");
});

test("signed-in users can sign out from the user menu", async ({ page }) => {
    await login(page, "/dashboard");
    await expect(page).toHaveURL(/\/dashboard(?:\?.*)?$/);

    await page.getByRole("button", { name: "Open user menu" }).click();
    await page.getByRole("menuitem", { name: "Sign out" }).click();

    await expect(page).toHaveURL(/\/login(?:\?.*)?$/);
    await expectAuthPage(page, "Sign in");
});

test("forgot password keeps the response generic for unknown accounts", async ({ page }) => {
    await page.goto("/forgot-password", { waitUntil: "domcontentloaded" });
    await expectAuthPage(page, "Reset password");

    await page.locator('input[type="email"]').first().fill("nobody@example.invalid");
    await page.locator('button[type="submit"]').first().click();

    await expect(page.getByRole("status")).toContainText("Check your inbox");
    await expect(page.getByText("If an account exists for nobody@example.invalid, we have sent a reset link.")).toBeVisible();
});

test("invalid mfa email links bounce back to login with a clear error", async ({ page }) => {
    const response = await page.request.get("/api/auth/mfa/email-link/verify?token=not-a-valid-token", {
        maxRedirects: 0
    });
    expect(response.status()).toBe(303);
    expect(response.headers().location).toContain("/login?error=mfa_link_invalid");

    await page.goto("/login?error=mfa_link_invalid", {
        waitUntil: "domcontentloaded"
    });

    await expectAuthPage(page, "Sign in");
    await expect(page.getByRole("alert")).toContainText("This sign-in link is invalid or has expired.");
});
