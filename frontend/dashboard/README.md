# Dashboard

HitKeep's dashboard is an Angular 21 application that also builds the lightweight tracking snippet (`hk.js`).

## Development server

To start a local development server, run:

```bash
ng serve
```

Once the server is running, open your browser and navigate to `http://localhost:4200/`. The application will automatically reload whenever you modify any of the source files.

From the repo root, `make dev` starts both the Go backend and this dashboard together. `make dev-seed` does the same with seeded demo data.

## Code scaffolding

Angular CLI includes powerful code scaffolding tools. To generate a new component, run:

```bash
ng generate component component-name
```

For a complete list of available schematics (such as `components`, `directives`, or `pipes`), run:

```bash
ng generate --help
```

## Building

To build the dashboard only, run:

```bash
ng build
```

This will compile your project and store the build artifacts in the `dist/` directory. By default, the production build optimizes your application for performance and speed.

To build the production dashboard bundle, optimize translations, rebuild `hk.js`, and sync the result into the Go app's embedded `public/` directory, run:

```bash
npm run build:prod
```

The Scalar API Reference runtime (`vendor/scalar/standalone.js`) is copied into the build output from `node_modules/@scalar/api-reference/dist/browser/standalone.js` via Angular assets configuration, so it always matches the installed npm package version.

## Running unit tests

To execute unit tests, use:

```bash
npm run test:ci
```

This runs Angular's Vitest-backed unit tests in non-watch headless mode. For a focused spec, use `npm run test -- --watch=false --no-progress --include <spec>`. Avoid bare `ng test` unless you intentionally want watch-mode debugging.

## Running end-to-end tests

For the real seeded end-to-end suite, run:

```bash
npm run e2e
```

This is the same browser contract CI runs. The launcher:

- builds the production dashboard bundle
- builds the Go binary
- seeds realistic demo data
- starts disposable local HitKeep instances
- runs the browser journeys against the real app
- runs the deployment smoke under `/hitkeep`

Angular 22 still supports `ng e2e`, but HitKeep uses Playwright directly so there is only one maintained E2E entrypoint. For a focused spec while iterating, pass Playwright arguments through `npm run e2e`:

```bash
npm run e2e -- e2e/auth.seeded.spec.js --workers=1
```

The full command includes a subdirectory smoke run with `HITKEEP_E2E_PUBLIC_PATH=/hitkeep` and verifies the dashboard, authenticated route refreshes, app-owned API/resource/static image paths, the API reference iframe, tracker bundles, and ingest preflight under that prefix.

On a fresh machine, install the browser dependency first:

```bash
npx playwright install --with-deps chromium
```

## Additional Resources

For more information on using the Angular CLI, including detailed command references, visit the [Angular CLI Overview and Command Reference](https://angular.dev/tools/cli) page.
