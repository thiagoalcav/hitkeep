# Official HitKeep Agent Skills

This directory contains official HitKeep Agent Skills for AI assistants and humans working with HitKeep analytics and contributor workflows.

The pack starts with one parent skill and several narrower child skills:

- `hitkeep-analytics` is the parent, or spine, skill. It routes analytics questions, explains the HitKeep MCP boundary, and tells agents when to use MCP, docs, REST API, open exports, or takeout.
- `hitkeep-traffic-diagnosis` handles traffic drops, spikes, and source changes.
- `hitkeep-ai-visibility-analyst` handles AI crawler fetches, AI-referred visits, assistant families, citation yield, and fetch failure hotspots.
- `hitkeep-ecommerce-analyst` handles revenue, products, sources, city/provider/ASN aggregates, and conversion context.
- `hitkeep-tracking-verifier` handles tracker install checks, WordPress-style installs, and automatic event verification.
- `hitkeep-i18n` handles dashboard UI strings, Transloco keys, locale files, PrimeNG locale behavior, and localized formatting.

The child skills are siblings on disk, not physically nested under the parent skill. Many skills-compatible clients discover skills by scanning direct child directories of a configured skills root, so nested `SKILL.md` files can be invisible. Treat them as one logical HitKeep Analytics pack.

The repository path is `skills/` so clients and directories that understand the conventional skills layout can discover the official HitKeep pack without extra mapping.

## Install

Install the full pack from the HitKeep repository when your client supports GitHub subdirectory skill installation:

```bash
npx skills add https://github.com/PascaleBeier/hitkeep/tree/main/skills/hitkeep-analytics
npx skills add https://github.com/PascaleBeier/hitkeep/tree/main/skills/hitkeep-traffic-diagnosis
npx skills add https://github.com/PascaleBeier/hitkeep/tree/main/skills/hitkeep-ai-visibility-analyst
npx skills add https://github.com/PascaleBeier/hitkeep/tree/main/skills/hitkeep-ecommerce-analyst
npx skills add https://github.com/PascaleBeier/hitkeep/tree/main/skills/hitkeep-tracking-verifier
npx skills add https://github.com/PascaleBeier/hitkeep/tree/main/skills/hitkeep-i18n
```

For manual installs, copy the skill directories you want into your agent's skills directory.

## Pair With HitKeep MCP

The analytics skills work best with the official HitKeep MCP server configured. They teach the agent how to reason about HitKeep data, while the MCP server provides scoped, read-only aggregate analytics and official docs tools. The `hitkeep-i18n` skill is for repository localization work and does not need MCP access.

The skills do not include scripts and do not store credentials. Keep HitKeep MCP tokens narrow: create API clients for the assistant, grant only the sites it needs, and revoke tokens when access is no longer required.
