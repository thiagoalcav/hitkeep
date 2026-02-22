# Contributing to HitKeep

HitKeep is an open-source project and contributions are welcome. This guide covers everything you need to set up a local development environment and submit a change.

## Table of Contents

- [Prerequisites](#prerequisites)
- [Getting Started](#getting-started)
- [Development Workflow](#development-workflow)
- [Project Structure](#project-structure)
- [Commit Conventions](#commit-conventions)
- [Submitting a Pull Request](#submitting-a-pull-request)
- [Reporting Bugs & Security Issues](#reporting-bugs--security-issues)

---

## Prerequisites

| Tool                  | Version                  | Purpose                                |
|:----------------------|:-------------------------|:---------------------------------------|
| **Go**                | 1.26+                    | Backend compilation                    |
| **CGo / C toolchain** | system default           | Required by DuckDB's Go bindings       |
| **Air**               | latest                   | Go live-reload for backend development |
| **Node.js + npm **    | 24+ (LTS)                | Angular dashboard and tracker snippet  |

### macOS

```bash
# Go (via Homebrew or https://go.dev/dl/)
brew install go

# C toolchain (required for CGo / DuckDB)
xcode-select --install

# Air — live-reload for Go
go install github.com/air-verse/air@latest

# Node.js (via fnm, nvm, or Homebrew)
brew install fnm
fnm install 24
fnm use 24
```

### Linux (Ubuntu / Debian)

```bash
# Go — download from https://go.dev/dl/ and follow official instructions
# Or via snap:
sudo snap install go --classic

# C toolchain
sudo apt-get install -y gcc g++ make

# Air
go install github.com/air-verse/air@latest

# Node.js (via fnm)
curl -fsSL https://fnm.vercel.app/install | bash
fnm install 24 && fnm use 24
```

---

## Getting Started

**1. Clone the repository:**

```bash
git clone https://github.com/pascalebeier/hitkeep.git
cd hitkeep
```

**2. Start the full development stack:**

```bash
make dev
```

This runs two processes in parallel:
- **Backend:** Air watches `internal/`, `cmd/`, and `*.go` files. On any change, it recompiles and restarts the Go server.
- **Frontend:** `ng serve` starts the Angular dev server with hot module replacement on `http://localhost:4200`.

The backend serves the API on `:8080`. The Angular dev server proxies `/api/*` and `/ingest` to `:8080` automatically.

Open `http://localhost:4200` in your browser.

---

## Development Workflow

### Backend Only

```bash
make dev-backend
```

The `.air.toml` configures Air to watch `*.go`, `*.sql`, `*.html`, `*.tpl`, and `*.tmpl` files. It excludes `frontend/`, `public/`, and `node_modules/`.

When you change a Go file, Air recompiles and restarts in ~1–2 seconds.

### Frontend Only

```bash
make dev-frontend
```

This starts `ng serve` on `http://localhost:4200`. The Angular proxy config forwards API calls to the backend.

### Full Build (Production Artifacts)

```bash
make build
```

This:
1. Runs `npm ci && npm run build` for the tracker snippet (output: `frontend/tracker/dist/`)
2. Runs `npm ci && ng build --configuration production` for the dashboard (output: `frontend/dashboard/dist/`)
3. Copies the dashboard bundle to `public/`
4. Compiles the Go binary: `go build -o hitkeep ./cmd/hitkeep/main.go`

The binary embeds the `public/` directory, so the build order matters.

### Running Tests

```bash
# Go tests
go test ./...

# Angular unit tests
cd frontend/dashboard && npm test
```

### Cleanup

```bash
make clean
# Removes: ./hitkeep binary, public/, frontend/*/dist/, frontend/*/node_modules/
```

---

## Project Structure

```
hitkeep/
├── cmd/hitkeep/           # Application entry point & flag definitions
│   ├── main.go            # Boots the modular registry
│   └── mod_*.go           # Module registration files (build-tag gated)
├── internal/
│   ├── modular/           # Plugin kernel — Registry and Module interface
│   ├── modules/           # Feature modules (auth, analytics, ingest, ...)
│   ├── database/          # DuckDB store — all SQL queries live here
│   ├── server/            # HTTP server setup, middleware, shared handlers
│   └── worker/            # Background workers (retention, rollups, reports)
├── frontend/
│   ├── dashboard/         # Angular v21 SPA (npm, @angular/cli)
│   └── tracker/           # hk.js tracking snippet (Rolldown)
├── public/                # Embedded static assets (built frontend output)
├── scripts/               # Developer tooling scripts
├── Makefile               # Development and build commands
└── .air.toml              # Air (live-reload) configuration
```

## Commit Conventions

HitKeep uses [Conventional Commits](https://www.conventionalcommits.org/) and [Release Please](https://github.com/googleapis/release-please) for automated changelog generation and version bumping.

**Format:** `type(scope): description`

| Type       | When to use                                             |
|:-----------|:--------------------------------------------------------|
| `feat`     | New user-facing feature                                 |
| `fix`      | Bug fix                                                 |
| `chore`    | Maintenance, dependency updates, tooling                |
| `docs`     | Documentation changes                                   |
| `refactor` | Code change that neither adds a feature nor fixes a bug |
| `test`     | Adding or updating tests                                |
| `perf`     | Performance improvement                                 |

**Examples:**

```
feat(ingest): add support for custom event properties
fix(auth): correct JWT expiry calculation for TOTP sessions
chore(deps): update duckdb-go to v2.5.5
docs(api): document /api/events endpoint
```

Breaking changes must include `BREAKING CHANGE:` in the commit body or footer:

```
feat(auth)!: remove legacy password-only login endpoint

BREAKING CHANGE: The /api/login endpoint now requires 2FA if enabled.
```

---

## Submitting a Pull Request

1. **Fork** the repository and create a branch from `main`:
   ```bash
   git checkout -b feat/my-new-feature
   ```

2. **Write your code** following the patterns above.

3. **Test your changes:**
   ```bash
   go test ./...
   ```

4. **Commit** using Conventional Commits format.

5. **Push** your branch and open a PR against `main`.

6. **PR description** should include:
   - What problem this solves
   - Any relevant issue numbers (`Closes #123`)
   - Testing steps

---

## Reporting Bugs & Security Issues

- **Bugs and feature requests:** [GitHub Issues](https://github.com/pascalebeier/hitkeep/issues)
- **Security vulnerabilities:** [GitHub Security Advisories](https://github.com/pascalebeier/hitkeep/security/advisories) — do not open a public issue for security vulnerabilities

See [SECURITY.md](./SECURITY.md) for the full vulnerability disclosure policy.
