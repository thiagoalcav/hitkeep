---
name: hitkeep-i18n
description: 'Maintain HitKeep dashboard localization. Use whenever an AI assistant or contributor adds or changes user-visible dashboard text, Transloco keys, locale JSON files, PrimeNG locale behavior, language switching, translated labels, date/number/duration formatting, or supported dashboard languages.'
license: MIT
---

# HitKeep i18n

Use this skill when changing user-visible dashboard text or localized formatting in HitKeep.

HitKeep's dashboard uses Transloco JSON files for UI copy and `@jsverse/transloco-locale` plus browser `Intl` APIs for locale-aware formatting. PrimeNG labels are synchronized from the active Transloco language.

## Public Surface

Dashboard translation files live in:

- `frontend/dashboard/public/i18n/en.json`
- `frontend/dashboard/public/i18n/de.json`
- `frontend/dashboard/public/i18n/es.json`
- `frontend/dashboard/public/i18n/fr.json`
- `frontend/dashboard/public/i18n/it.json`
- `frontend/dashboard/public/i18n/nl.json`
- `frontend/dashboard/public/i18n/pt.json`

The dashboard currently exposes these language IDs:

```text
en, de, es, fr, it, nl, pt
```

Formatting locale mappings are configured in `frontend/dashboard/src/app/app.config.ts`:

```text
en -> en-US
de -> de-DE
es -> es-ES
fr -> fr-FR
it -> it-IT
nl -> nl-NL
pt -> pt-BR
```

Do not infer a new supported language from an Angular locale helper alone. Add or change a supported dashboard language only when the Transloco config, locale JSON file, formatting mapping, and product docs all agree.

## Workflow

1. Search for the existing feature key before creating a new key.
2. Put user-visible Angular template text behind `TranslocoPipe`.
3. Use `TranslocoService` for computed labels in TypeScript.
4. Add the same key path to all seven locale JSON files.
5. Keep key names semantic and grouped by feature.
6. Preserve interpolation variables exactly. Do not translate variable names or change placeholder syntax.
7. For computed option labels, depend on the active language so labels recompute after language switches.
8. For dates, numbers, percentages, and durations, prefer the existing locale services or helpers instead of manual string formatting.
9. Let PrimeNG locale text flow through the existing sync service instead of hardcoding component labels.
10. Keep labels short enough for buttons, tabs, chips, table columns, and mobile layouts.

Useful places to inspect:

- `frontend/dashboard/src/app/app.config.ts`
- `frontend/dashboard/src/app/transloco-loader.ts`
- `frontend/dashboard/src/app/core/i18n/`
- `frontend/dashboard/scripts/check-i18n-locales.mjs`
- `frontend/dashboard/scripts/sync-i18n-locales.mjs`

## Translation Quality Bar

Translate product UI, not individual English words.

- German: use natural product German and correct characters such as `Ä`, `Ö`, `Ü`, `ä`, `ö`, `ü`, and `ß`.
- Spanish: use accents, `ñ`, and opening punctuation where required, for example `¿Seguro?`.
- French: use accents and apostrophes naturally, for example `paramètres`, `sécurité`, and `l'adresse e-mail`.
- Italian: use accents where required, especially final-stress words such as `attività`, `città`, `perché`, `può`, and `è`.
- Dutch: keep dashboard terminology natural and concise. Reuse existing terms for site, team, goal, funnel, share link, and API client.
- Portuguese: use the existing Brazilian Portuguese formatting context (`pt-BR`) unless the product explicitly changes the locale target. Use natural Portuguese with correct accents.

Keep analytics terminology consistent across all languages. If the repo already uses a term for "site", "team", "event", "goal", "funnel", "share link", "API client", "QR campaign", or "Opportunity", reuse it.

## Checks

Run the locale check after translation changes:

```bash
cd frontend/dashboard && npm run i18n:check
```

If key shape drift is suspected, run the sync script and review the resulting changes before keeping them:

```bash
cd frontend/dashboard && node scripts/sync-i18n-locales.mjs
```

Run formatting checks for dashboard files:

```bash
cd frontend/dashboard && npm run fmt:check
```

For UI behavior that depends on language switching, add or update focused tests near the component or service that owns the behavior.

## Output For Reviews

When reporting an i18n change, include:

```markdown
Changed keys:
- <key path>

Locales updated:
- en, de, es, fr, it, nl, pt

Checks:
- <command and result>

Caveats:
- <layout, wording, or review notes>
```

Do not claim a translation was reviewed by a native speaker unless that actually happened.
