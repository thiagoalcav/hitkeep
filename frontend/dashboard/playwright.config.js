const path = require("node:path");
const { defineConfig } = require("playwright/test");

const port = Number(process.env.HITKEEP_E2E_PORT || 8098);
const publicPath = normalizePublicPath(process.env.HITKEEP_E2E_PUBLIC_PATH || "/");
const rootBaseURL = process.env.HITKEEP_BASE_URL || `http://127.0.0.1:${port}`;
const baseURL = trimTrailingSlash(joinPublicURL(rootBaseURL, publicPath));
const repoRoot = path.resolve(__dirname, "../..");
const workers = Number(process.env.HITKEEP_E2E_WORKERS || 1);

module.exports = defineConfig({
    testDir: "./e2e",
    timeout: 60_000,
    expect: {
        timeout: 10_000
    },
    fullyParallel: false,
    workers,
    retries: process.env.CI ? 1 : 0,
    reporter: [["list"], ["html", { open: "never" }]],
    use: {
        baseURL,
        trace: "retain-on-failure",
        screenshot: "only-on-failure",
        video: "retain-on-failure"
    },
    webServer: process.env.HITKEEP_BASE_URL
        ? undefined
        : {
              command: "bash tests/e2e/serve-dashboard.sh",
              cwd: repoRoot,
              url: `${baseURL}/healthz`,
              timeout: 300_000,
              reuseExistingServer: !process.env.CI,
              stdout: "pipe",
              stderr: "pipe"
          }
});

function normalizePublicPath(value) {
    const trimmed = (value || "/").trim();
    if (!trimmed || trimmed === "/") {
        return "/";
    }
    const withLeadingSlash = trimmed.startsWith("/") ? trimmed : `/${trimmed}`;
    return withLeadingSlash.endsWith("/") ? withLeadingSlash : `${withLeadingSlash}/`;
}

function joinPublicURL(base, pathPrefix) {
    if (pathPrefix === "/") {
        return base;
    }

    const url = new URL(base);
    const basePath = normalizePublicPath(url.pathname);
    const nextPath = pathPrefix.replace(/\/$/, "");

    if (basePath !== "/" && nextPath === basePath.replace(/\/$/, "")) {
        return url.toString();
    }

    url.pathname = `${basePath === "/" ? "" : basePath.replace(/\/$/, "")}${nextPath}`;
    return url.toString();
}

function trimTrailingSlash(url) {
    return url.endsWith("/") ? url.slice(0, -1) : url;
}
