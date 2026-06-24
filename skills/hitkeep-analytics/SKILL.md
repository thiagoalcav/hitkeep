---
name: hitkeep-analytics
description: 'Official parent skill for analyzing HitKeep data through HitKeep MCP and official docs. Use whenever the user asks an AI assistant to analyze HitKeep traffic, events, ecommerce, Web Vitals, AI visibility, Search Console, opportunities, tracking health, or "Ask AI" style questions; when setting up or validating HitKeep MCP access or help; or when deciding whether to use MCP, the dashboard, REST API, open exports, or takeout. Prefer this spine skill alongside narrower HitKeep analytics skills when they are installed.'
license: MIT
---

# HitKeep Analytics

Use this as the spine skill for HitKeep analytics work. It teaches the agent how HitKeep expects analytics to be queried, interpreted, and bounded.

## Skill Pack Shape

This parent skill has logical child skills:

- `hitkeep-traffic-diagnosis` for traffic drops, spikes, and source changes.
- `hitkeep-ai-visibility-analyst` for AI crawler fetches, AI-referred visits, assistant families, citation yield, and fetch failure hotspots.
- `hitkeep-ecommerce-analyst` for revenue, products, sources, city/provider/ASN aggregates, and conversion context.
- `hitkeep-tracking-verifier` for tracker install checks, automatic events, WordPress-style installs, and "is tracking live?" questions.

The child skills are sibling skill directories in the official pack. Do not assume nested child `SKILL.md` files are discoverable by every client.

The pack also includes `hitkeep-i18n` for dashboard translation and locale-formatting changes. Use it for product UI copy work, not for analytics interpretation.

## Operating Principles

HitKeep is privacy-first web analytics with optional read-only MCP access for approved assistants.

Prefer this boundary:

- Use HitKeep MCP for scoped, read-only aggregate analytics and official docs lookup.
- Use the dashboard for normal human investigation, setup, admin, and visual review.
- Use the REST API for normal application automation and write workflows.
- Use open exports and takeout for portable owned files and raw export needs.

Do not ask MCP for raw visitor IPs, raw visitor rows, raw hit exports, dashboard cookies, billing changes, site administration, alert creation, or goal/funnel mutation. If the user needs those, route them to the dashboard, REST API, or takeout instead.

## Default Workflow

1. Identify the site, timeframe, comparison period, and metric. If any are missing, make a reasonable default: visible site if only one exists, last 30 days, and previous period comparison.
2. Check available HitKeep MCP tools. If MCP is unavailable, say which parts cannot be verified live and use official docs or user-provided data.
3. Use `hitkeep_list_sites` when site scope is ambiguous.
4. Use report-shaped MCP tools before trying to reconstruct analytics manually:
   - `hitkeep_get_site_overview` for traffic overview and comparisons.
   - `hitkeep_get_event_names` and `hitkeep_get_event_breakdown` for event activity.
   - `hitkeep_get_ecommerce` for revenue, products, sources, city/provider/ASN aggregate context.
   - `hitkeep_get_web_vitals` for page and visitor-context performance aggregates.
   - `hitkeep_get_ai_visibility` for AI crawler and AI-referred traffic.
   - `hitkeep_get_opportunities` for saved recommendations.
   - `hitkeep_get_search_console_status` and `hitkeep_get_search_console` for imported organic search aggregates.
   - `hitkeep_get_mcp_help` for token setup, date range, filter syntax, and privacy boundary guidance.
   - `hitkeep_search_docs`, `hitkeep_get_doc`, and `hitkeep_get_api_reference` for official documentation.
5. Triangulate before making a recommendation. Compare time shape, channel/source mix, event activity, Search Console signals, Web Vitals, and ecommerce outcomes where relevant.
6. Present findings with a verdict first, then evidence, next action, and caveats.

## Output Shape

For analytics investigations, use this compact structure:

```markdown
Verdict: <one-sentence answer>

Evidence:
- <metric and timeframe>
- <segment or comparison>
- <supporting check>

Next:
- <recommended action>

Caveats:
- <data limits, missing MCP scope, small sample, or unavailable surface>
```

For pure retrieval questions, answer directly and include the timeframe and site scope.

## Privacy Guardrails

Keep assistant access aggregate-only. Do not request broader tokens to make a question easier. Do not suggest dashboard-cookie reuse. Do not infer individual visitor identity from aggregate reports.

If a user asks for raw data, say that HitKeep MCP is not the raw export surface and point them to takeout/open exports.
