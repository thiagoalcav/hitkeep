---
name: hitkeep-tracking-verifier
description: 'Verify HitKeep tracking setup and automatic event collection. Use when the user asks whether HitKeep is installed, whether tracking is live, why pageviews or events are missing, how to validate WordPress or script installs, how automatic outbound click, file download, or form submit events should appear, or how to distinguish tracker setup problems from analytics interpretation problems.'
license: MIT
---

# HitKeep Tracking Verifier

Use this skill before deeper analytics when the user may not have trustworthy tracking data yet.

## First Principle

Do not diagnose traffic, SEO, ecommerce, or AI visibility until the tracker surface is plausible. A broken tracker can look like a product or campaign problem.

## What To Check

1. Site scope: make sure the user is looking at the intended HitKeep site.
2. Install path: normal script snippet, WordPress plugin, server-side tracking, or another first-party integration.
3. Hostname: make sure traffic is coming from the expected domain.
4. First and last hit: confirm the site has accepted recent pageviews.
5. Automatic events: check whether `outbound_click`, `file_download`, and `form_submit` are expected and firing.
6. Exclusions: check IP exclusions, global exclusions, DNT behavior, spam/bot filters, and trusted proxy setup if counts look wrong.
7. Environment: localhost and preview/staging behavior may differ from production depending on setup.

## Tool And Surface Routing

- Use the dashboard tracking verifier when the user is in the HitKeep UI.
- Use REST API or dashboard status surfaces when install status needs operational proof.
- Use HitKeep MCP for aggregate analytics and docs lookup, not raw tracker debugging.
- Use `hitkeep_get_mcp_help` when token setup, date range, filter syntax, or MCP privacy boundaries are unclear.
- Use `hitkeep_search_docs` or `hitkeep_get_doc` for official install and tracker behavior guidance when MCP docs tools are configured.

If MCP shows no recent aggregate data, do not conclude the site has no visitors. Say that tracking must be verified through install status, network requests, dashboard verifier, server logs, or REST/dashboard surfaces.

## Verification Flow

Ask for or inspect:

- The site domain.
- The install method.
- The page where the tracker should load.
- Whether the user has ad blockers, DNT, CSP, or consent tools in the test browser.
- Whether a production deployment has happened.

Then verify:

- The tracker script loads.
- The ingest request succeeds.
- The site receives a pageview.
- Expected automatic events fire after real user actions.
- The dashboard or aggregate report updates after processing.

## Output

Use:

```markdown
Status: <live, partially live, not verified, or likely broken>

Evidence:
- <install/status clue>
- <hit/event clue>
- <configuration clue>

Next:
- <specific fix or verification action>

Caveats:
- <what could not be checked from the current tools>
```

Avoid telling the user tracking is fixed until a hit or expected event is visible in HitKeep.
