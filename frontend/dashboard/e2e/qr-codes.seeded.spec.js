const { test, expect } = require("playwright/test");
const { login } = require("./support/auth");

const PRIMARY_SEEDED_SITE_DOMAIN = "acme-analytics.io";
const SEEDED_QR_NAME = "Conference booth poster";

test("QR campaigns render seeded analytics, exports, redirect attribution, and read-only shares", async ({ page, browser }) => {
    await login(page, "/dashboard");

    const site = await loadSeededSite(page);
    await page.evaluate((siteId) => localStorage.setItem("hk_last_site_id", siteId), site.id);

    const qrs = await loadSeededQRCodes(page, site.id);
    expect(qrs.length).toBeGreaterThanOrEqual(3);
    const qr = qrs.find((item) => item.name === SEEDED_QR_NAME) || qrs[0];
    expect(qr.redirect_url || "").toContain("/q/");
    expect(qr.has_asset).toBeTruthy();

    const summary = await loadQRSummary(page, site.id, qr.id);
    expect(summary.open_count).toBeGreaterThan(0);
    expect(summary.pageviews).toBeGreaterThan(0);
    expect(summary.visitors).toBeGreaterThan(0);

    await page.goto("/utm/qr-codes", { waitUntil: "domcontentloaded" });
    await expect(page.getByRole("heading", { name: "QR campaigns" })).toBeVisible();
    await page.getByPlaceholder("Search...").fill("Conference");
    await expect(page.getByRole("link", { name: qr.name })).toBeVisible();
    await page.getByRole("link", { name: qr.name }).click();

    await expect(page.getByRole("heading", { name: qr.name })).toBeVisible();
    await expect(page.getByRole("heading", { name: "QR opens", exact: true })).toBeVisible();
    await expect(page.getByText("Full analytics")).toBeVisible();
    await expect(page.getByText("Redirect URL")).toBeVisible();
    await expect(page.getByText("Final tracked URL")).toBeVisible();
    await expect(page.getByText("berlin-analytics-summit").first()).toBeVisible();
    await expect(page.locator("app-kpi-card").filter({ hasText: "Opens" }).first()).toBeVisible();

    await expect(page.locator("app-qr-code-preview").first()).toBeVisible();
    const svgDownloadPromise = page.waitForEvent("download");
    await page.getByRole("button", { name: "Export artwork" }).click();
    const svgDownload = await svgDownloadPromise;
    expect(svgDownload.suggestedFilename()).toMatch(/acme-analytics-io-conference-booth-poster-print-vector\.svg$/);

    const pngDownloadPromise = page.waitForEvent("download");
    await page.locator(".qr-codes-preview-card__actions .p-splitbutton-dropdown").click();
    await page.getByRole("menuitem", { name: "PNG 2048 px" }).click();
    const pngDownload = await pngDownloadPromise;
    expect(pngDownload.suggestedFilename()).toMatch(/acme-analytics-io-conference-booth-poster-print-2048px\.png$/);

    for (const format of ["csv", "xlsx", "parquet", "json", "ndjson"]) {
        const response = await page.request.get(`/api/sites/${site.id}/qr-codes/${qr.id}/takeout?format=${format}`);
        expect(response.ok(), `QR takeout ${format} returned ${response.status()}`).toBeTruthy();
        expect(response.headers()["content-disposition"] || "").toContain(`.${format}`);
    }

    const redirectResponse = await page.request.get(qr.redirect_url, { maxRedirects: 0 });
    expect([301, 302, 303, 307, 308]).toContain(redirectResponse.status());
    const redirectLocation = redirectResponse.headers().location || "";
    const redirectedURL = new URL(redirectLocation);
    expect(redirectedURL.searchParams.get("hk_qr")).toBe(qr.id);
    expect(redirectedURL.searchParams.get("utm_campaign")).toBe("berlin-analytics-summit");
    expect(redirectedURL.searchParams.get("utm_medium")).toBe("qr");

    let createdShare;
    try {
        const shareResponsePromise = page.waitForResponse((response) => response.request().method() === "POST" && response.url().includes(`/api/sites/${site.id}/qr-codes/${qr.id}/share`) && response.status() === 201);
        await page.getByRole("button", { name: "Create QR share link" }).first().click();
        await expect(page.getByRole("dialog", { name: "QR share links" })).toBeVisible();
        await page.getByRole("dialog", { name: "QR share links" }).getByRole("button", { name: "Create QR share link" }).click();
        const shareResponse = await shareResponsePromise;
        createdShare = await shareResponse.json();

        await expect(page.getByText("QR share link created.")).toBeVisible();
        await expect(page.locator(".qr-codes-share-url-input").last()).toHaveValue(createdShare.url);

        const shareURL = new URL(createdShare.url || `/qr-share/${createdShare.token}`, page.url()).toString();
        const shareContext = await browser.newContext();
        const sharePage = await shareContext.newPage();
        try {
            await sharePage.goto(shareURL, { waitUntil: "domcontentloaded" });
            await expect(sharePage.getByText(/read-only qr analytics/i)).toBeVisible();
            await expect(sharePage.getByRole("heading", { name: qr.name })).toBeVisible();
            await expect(sharePage.getByText("Shared stats")).toBeVisible();
            await expect(sharePage.getByRole("heading", { name: "QR opens", exact: true })).toBeVisible();
            await expect(sharePage.getByRole("button", { name: "Create QR share link" })).toHaveCount(0);
            await expect(sharePage.getByRole("button", { name: /^Edit$/ })).toHaveCount(0);
        } finally {
            await shareContext.close();
        }
    } finally {
        if (createdShare?.id) {
            await page.request.delete(`/api/sites/${site.id}/qr-codes/${qr.id}/share/${createdShare.id}`, {
                headers: originHeaders(page)
            });
        }
    }
});

async function loadSeededSite(page) {
    const response = await page.request.get("/api/sites");
    const body = await response.text();
    expect(response.ok(), `list sites returned ${response.status()}: ${body}`).toBeTruthy();
    const sites = JSON.parse(body);
    const site = sites.find((item) => item.domain === PRIMARY_SEEDED_SITE_DOMAIN);
    expect(site, `expected seeded site ${PRIMARY_SEEDED_SITE_DOMAIN}`).toBeTruthy();
    return site;
}

async function loadSeededQRCodes(page, siteId) {
    const response = await page.request.get(`/api/sites/${siteId}/qr-codes`);
    const body = await response.text();
    expect(response.ok(), `list QR codes returned ${response.status()}: ${body}`).toBeTruthy();
    return JSON.parse(body);
}

async function loadQRSummary(page, siteId, qrId) {
    const response = await page.request.get(`/api/sites/${siteId}/qr-codes/${qrId}/summary`);
    const body = await response.text();
    expect(response.ok(), `load QR summary returned ${response.status()}: ${body}`).toBeTruthy();
    return JSON.parse(body);
}

function originHeaders(page) {
    return { Origin: new URL(page.url()).origin };
}
