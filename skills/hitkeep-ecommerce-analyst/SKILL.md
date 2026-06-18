---
name: hitkeep-ecommerce-analyst
description: Analyze HitKeep ecommerce and conversion data through aggregate reports. Use when the user asks about revenue, orders, average order value, products, sources, ecommerce funnels, conversion changes, city/provider/ASN aggregates, UTM performance, or whether traffic quality changed in revenue terms. Use with HitKeep MCP when available and keep analysis aggregate-only.
---

# HitKeep Ecommerce Analyst

Use this skill for revenue and conversion questions in HitKeep.

## Scope

HitKeep ecommerce MCP output is for aggregate analysis. It can support questions about:

- Revenue and order trends.
- Product performance.
- Source, channel, UTM, city, provider, and ASN aggregates where available.
- Whether a traffic change affected revenue quality.
- Checkout or conversion context alongside Web Vitals and events.

Do not request raw order rows, customer identities, visitor IPs, or raw hit exports through MCP.

## MCP Call Pattern

1. `hitkeep_list_sites` if site scope is unclear.
2. `hitkeep_get_site_overview` for overall traffic and conversion context.
3. `hitkeep_get_ecommerce` for revenue, products, sources, and aggregate context.
4. `hitkeep_get_event_names` and `hitkeep_get_event_breakdown` when checkout, forms, or custom conversion events matter.
5. `hitkeep_get_web_vitals` when page performance could explain conversion movement.
6. `hitkeep_get_ai_visibility` when AI-referred traffic may be part of the acquisition story.
7. `hitkeep_get_mcp_help` when token setup, date range, filter syntax, or MCP privacy boundaries are unclear.

Use current and comparison periods. Revenue without a baseline is usually just a number, not an insight.

## Analysis Checks

- Split revenue movement into traffic volume, conversion rate, average order value, and product mix.
- Check source mix before judging channel quality.
- Check product mix before judging campaign quality.
- If traffic rose and revenue fell, inspect low-intent sources, bot-like spikes, and product/page mix.
- If revenue rose and orders did not, inspect AOV and product mix before claiming more buyers.
- Treat city/provider/ASN aggregates as infrastructure and geographic context, not individual identity.

## Output

Use:

```markdown
Verdict: <what changed in revenue or conversion terms>

Evidence:
- <revenue/order/AOV comparison>
- <source/product/UTM finding>
- <traffic/event/performance context>

Next:
- <1-3 actions>

Caveats:
- <sample size, attribution limit, missing event, or aggregate-only caveat>
```

If the user asks for customer-level or order-level exports, route to takeout/open exports or the appropriate REST/export path instead of MCP.
