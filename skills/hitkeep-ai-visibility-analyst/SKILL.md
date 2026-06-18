---
name: hitkeep-ai-visibility-analyst
description: Analyze HitKeep AI visibility data: AI crawler fetches, AI-referred visits, assistant names and families, citation yield, resource types, top paths, fetch failures, and Search Console correlation. Use when the user asks about GPTBot, ClaudeBot, ChatGPT, Claude, Perplexity, AI search, AI referrals, crawler access, LLM visibility, or whether AI assistants are discovering and sending traffic to a site.
---

# HitKeep AI Visibility Analyst

Use this skill when the analytics question is about AI systems discovering, fetching, citing, or referring traffic to a HitKeep-tracked site.

## Core Questions

Translate vague AI traffic questions into one of these:

- Which AI assistants fetched our content?
- Which assistant families fetched content, and how did those fetches correlate with AI-referred visits?
- Which paths are fetched but not followed by AI-referred visits?
- Which paths have high citation yield?
- Which assistants or paths have fetch failures?
- Did AI-origin traffic move alongside organic search or normal traffic?

## MCP Call Pattern

Use HitKeep MCP when available:

1. `hitkeep_list_sites` if the site is ambiguous.
2. `hitkeep_get_ai_visibility` for overview, timeseries, top assistants, top paths, citation yield, opportunity pages, and failure hotspots. Use `include_correlation` when the question is about fetched paths turning into AI-referred visits.
3. `hitkeep_get_site_overview` to compare AI-referred visits against overall traffic and source mix.
4. `hitkeep_get_search_console_status` and `hitkeep_get_search_console` when organic search context matters.
5. `hitkeep_get_mcp_help` when token setup, date range, filter syntax, or MCP privacy boundaries are unclear.
6. `hitkeep_get_doc` for setup questions such as AI fetch ingest, CloudFront AI crawler tracking, or tracking GPTBot and ClaudeBot.

Apply supported filters deliberately: `assistant_name`, `assistant_family`, and `resource_type` are analysis tools, not decoration. The MCP AI visibility tool returns path-level rows in top paths and correlation output; do not invent a `path` filter unless the live tool schema exposes one.

## Interpretation Rules

- Do not equate crawler fetches with human visits. Fetches show assistant access; AI-referred visits show downstream traffic.
- Treat `assistant_name` and `assistant_family` as fetch-side filters. In correlation output, `ai_referred_visits` means later AI-referred visits on the same path within the correlation window, not necessarily visits from that same assistant.
- For questions like "GPTBot fetched but ChatGPT or Perplexity did not visit", report what MCP can prove: GPTBot-fetched paths with low or zero correlated AI-referred visits, plus overall AI source context from site overview. Do not claim source-specific path attribution unless the live tool output exposes that source/path dimension.
- Treat citation yield as directional unless the sample is large enough and the tracked path mapping is clean.
- Separate assistant families from individual assistant names. Family-level movement can hide one bot replacing another.
- Fetch failures are often setup or access issues. Check status codes, resource type, and path patterns before assuming content quality.
- Search Console data in HitKeep MCP is imported aggregate data. It does not call Google live or trigger syncs.
- Keep analysis aggregate-only. Route raw hit exports, visitor identity questions, or dashboard/admin changes away from MCP.

## Output

Use:

```markdown
Verdict: <what AI visibility shows>

Evidence:
- <assistant/family/path finding>
- <fetch vs referred visit comparison>
- <Search Console or traffic context if relevant>

Next:
- <content, access, tracking, or follow-up query action>

Caveats:
- <sample size, sync status, imported-data limits, or unmapped site caveat>
```

Never claim that a specific AI product "ranked" a page unless the data actually shows that. HitKeep measures fetches, referrals, and configured AI visibility signals, not every black-box answer surface.
