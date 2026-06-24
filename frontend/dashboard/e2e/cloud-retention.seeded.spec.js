const { test, expect } = require("playwright/test");
const { login } = require("./support/auth");

const TEAM_A_ID = "00000000-0000-0000-0000-00000000c0a1";
const TEAM_B_ID = "00000000-0000-0000-0000-00000000c0b2";

test("cloud free-plan retention notice dismisses per team and starts checkout", async ({ page }) => {
    let activeTeamID = TEAM_A_ID;
    let checkoutPayload = null;

    await stubCloudFreeBootstrap(page, () => activeTeamID);
    await page.route("**/api/cloud/billing/checkout", async (route) => {
        checkoutPayload = route.request().postDataJSON();
        await route.fulfill({
            contentType: "application/json",
            body: JSON.stringify({ url: "/dashboard?checkout=pro" })
        });
    });

    await login(page, "/dashboard");

    const notice = page.getByTestId("free-plan-retention-notice");
    await expect(notice).toContainText("Free plan data is retained for 60 days.");
    await expect(notice.getByRole("link", { name: "Compare plans" })).toHaveAttribute("href", "/admin/team");

    await notice.getByRole("button", { name: "Dismiss retention notice" }).click();
    await expect(notice).toBeHidden();
    await expect.poll(() => page.evaluate((key) => window.localStorage.getItem(key), `hitkeep.freeRetentionNotice.dismissed.${TEAM_A_ID}`)).toBe("dismissed");

    activeTeamID = TEAM_B_ID;
    await page.reload({ waitUntil: "domcontentloaded" });
    await expect(notice).toContainText("Free plan data is retained for 60 days.");

    activeTeamID = TEAM_A_ID;
    await page.reload({ waitUntil: "domcontentloaded" });
    await expect(notice).toBeHidden();

    activeTeamID = TEAM_B_ID;
    await page.reload({ waitUntil: "domcontentloaded" });
    await expect(notice).toBeVisible();

    await Promise.all([page.waitForRequest((request) => request.url().includes("/api/cloud/billing/checkout") && request.method() === "POST"), notice.getByRole("button", { name: "Upgrade to Pro" }).click()]);

    expect(checkoutPayload).toEqual({ plan_code: "pro", locale: "en" });
    await page.waitForURL(/\/dashboard\?checkout=pro$/);
});

async function stubCloudFreeBootstrap(page, activeTeamID) {
    await page.route("**/api/user/bootstrap", async (route) => {
        const response = await route.fetch();
        const bootstrap = await response.json();
        const sourceTeam = bootstrap.teams?.teams?.[0] || {};

        bootstrap.status = {
            ...(bootstrap.status || {}),
            cloud: {
                hosted: true,
                signup_enabled: false
            }
        };
        bootstrap.teams = {
            ...(bootstrap.teams || {}),
            active_team_id: activeTeamID(),
            teams: [cloudFreeTeam(sourceTeam, TEAM_A_ID, "Acme Analytics"), cloudFreeTeam(sourceTeam, TEAM_B_ID, "Beta Analytics")]
        };

        await route.fulfill({ response, json: bootstrap });
    });
}

function cloudFreeTeam(sourceTeam, id, name) {
    return {
        ...sourceTeam,
        id,
        name,
        role: "owner",
        usage: {
            current_sites: 1,
            current_members: 1,
            current_pending_invites: 0
        },
        entitlements: {
            max_sites_per_team: 3,
            max_team_members: 3,
            max_retention_days: 60,
            allow_sso: false,
            allow_custom_branding: false
        },
        plan: {
            code: "free",
            name: "Free"
        }
    };
}
