---
name: hitkeep-traffic-diagnosis
description: Diagnose HitKeep traffic drops, spikes, source shifts, and suspicious changes. Use when the user asks why traffic changed, whether a deploy hurt traffic, whether organic/search/referral/direct traffic moved, whether a spike is real or bot-like, or what happened to sessions, visitors, pageviews, events, or conversions in HitKeep. Use with HitKeep MCP when available and follow the parent hitkeep-analytics skill when it is also loaded.
---

# HitKeep Traffic Diagnosis

Use this skill to investigate traffic changes without jumping to the first plausible story.

## Opening Moves

1. Follow `hitkeep-analytics` if it is already loaded; otherwise keep this workflow self-contained.
2. Identify the site, metric, affected period, comparison period, and suspected change date.
3. Use HitKeep MCP if available. If not, ask for exported data or screenshots and state that live verification is unavailable.
4. Start with measurement checks before interpreting user behavior.

## Measurement First

Check the cheap failure modes first:

- Did total traffic change but key events stay stable?
- Did one path or page family change more than the rest?
- Did the change start like a cliff near a deploy or config change?
- Did Search Console disagree with HitKeep organic traffic?
- Did Web Vitals or page errors worsen around the same time?
- Did bot/spam filtering, exclusions, or trusted proxy configuration change?

Do not diagnose SEO decay, campaign performance, or UX until the data itself looks trustworthy.

## MCP Call Pattern

Use this sequence when the tools are available:

1. `hitkeep_list_sites` if site scope is unclear.
2. `hitkeep_get_site_overview` for current and comparison periods.
3. `hitkeep_get_event_names` and `hitkeep_get_event_breakdown` to compare event-to-traffic ratios.
4. `hitkeep_get_search_console_status` and `hitkeep_get_search_console` when organic search may be involved.
5. `hitkeep_get_web_vitals` when a deploy, performance regression, or page-level drop is suspected.
6. `hitkeep_get_ai_visibility` when AI crawler or AI-referred traffic may explain the change.
7. `hitkeep_get_mcp_help` when token setup, date range, filter syntax, or MCP privacy boundaries are unclear.
8. `hitkeep_get_doc` or `hitkeep_search_docs` when the question becomes setup/configuration rather than data interpretation.

## Diagnosis Tree

Walk in this order:

1. Measurement: tracker missing, blocked script, changed exclusions, import lag, bot filtering, wrong site, wrong date range.
2. Time shape: cliff, ramp, spike, seasonality, missing data window.
3. Source mix: direct, referral, organic, paid/UTM, AI-referred traffic.
4. Content or page: one route, section, hostname, or template moved.
5. Experience: performance, checkout, forms, errors, page layout, deploy fallout.
6. External: campaign changes, search updates, news, seasonal behavior.

## Output

Use:

```markdown
Verdict: <what most likely changed and why>

Evidence:
- <traffic overview comparison>
- <segment or page/source finding>
- <measurement sanity check>

Next:
- <1-3 concrete actions>

Caveats:
- <sample size, missing data, MCP scope, or unverified deploy context>
```

If the sample is small, say so plainly and prefer weekly or longer comparisons.

Keep analysis aggregate-only. Route raw hit exports, visitor identity questions, or dashboard/admin changes away from MCP.
