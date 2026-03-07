# HitKeep

> **Web Analytics in a single binary.**

[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)
[![Go Version](https://img.shields.io/badge/Go-1.26.1-00ADD8?logo=go)](https://go.dev/)
[![Docker Image (GHCR)](https://img.shields.io/badge/Docker-ghcr.io-blue?logo=docker)](https://github.com/pascalebeier/hitkeep/pkgs/container/hitkeep)
[![Docker Image (Hub)](https://img.shields.io/badge/Docker-Docker_Hub-2496ED?logo=docker)](https://hub.docker.com/r/pascalebeier/hitkeep)
[![Documentation](https://img.shields.io/badge/📖_Documentation-hitkeep.com-33d399)](https://hitkeep.com)
[![OpenSSF Best Practices](https://www.bestpractices.dev/projects/11990/badge)](https://www.bestpractices.dev/projects/11990)

HitKeep is a self-hostable, privacy-first web analytics platform designed for **radical simplicity** without sacrificing performance.

Unlike other solutions that require you to manage a complex stack (PostgreSQL, Redis, ClickHouse, Nginx), HitKeep runs as a **single, self-contained executable**. It embeds a high-performance OLAP database (DuckDB) and a distributed message queue (NSQ) directly into the binary.

![HitKeep analytics dashboard — traffic overview, geographic breakdown, goals, funnels, and UTM attribution](./.github/assets/dashboard-overview.png)

<details>
<summary>More screenshots</summary>

### Login
![HitKeep login page](./.github/assets/page-login.png)

### Share Dashboard
![HitKeep shareable read-only dashboard link](./.github/assets/feature-share-dashboard.png)

### Events
![HitKeep custom event analytics with timeseries chart and property breakdown](./.github/assets/analytics-events.png)

### Event Audience Breakdown
![HitKeep event audience breakdown — top pages, referrers, devices, and countries for a selected event](./.github/assets/analytics-events-audience.png)

### Goals & Conversion Tracking
![HitKeep goals and conversion tracking](./.github/assets/analytics-goals.png)

### Multi-Step Funnels
![HitKeep multi-step funnel analytics](./.github/assets/analytics-funnels.png)

### UTM Campaign Attribution
![HitKeep UTM tracking and campaign attribution](./.github/assets/analytics-utm.png)

### UTM Link Builder
![HitKeep built-in UTM link builder](./.github/assets/tools-utm-builder.png)

### Email Reports
![HitKeep scheduled email reports](./.github/assets/feature-email-reports.png)

### Weekly Report Email
![HitKeep weekly analytics report email](./.github/assets/weekly-report.png)

### Digest Report Email
![HitKeep digest report email with multi-site summary](./.github/assets/digest-report.png)

### Profile & Settings
![HitKeep user profile and settings](./.github/assets/settings-profile.png)

### TOTP & WebAuthn / Passkeys
![HitKeep two-factor authentication setup — TOTP and WebAuthn Passkeys](./.github/assets/security-2fa-setup.png)

### API Clients & Bearer Tokens
![HitKeep API client management](./.github/assets/security-api-clients.png)

### API Reference
![HitKeep built-in OpenAPI reference](./.github/assets/integration-api-reference.png)

### Admin — User Management
![HitKeep admin panel — user management](./.github/assets/admin-users.png)

### Admin — Site Management
![HitKeep admin panel — site management](./.github/assets/admin-sites-list.png)

### Teams — Switcher
![HitKeep team switcher with multiple teams](./.github/assets/feature-team-switcher.png)

### Teams — Create Team
![HitKeep create team dialog](./.github/assets/feature-create-team.png)

### Teams — Transfer Site
![HitKeep site transfer between teams](./.github/assets/feature-site-transfer.png)

### Teams — Overview
![HitKeep team administration overview](./.github/assets/admin-team-overview.png)

### Teams — Members
![HitKeep team member management](./.github/assets/admin-team-members.png)

### Teams — Settings
![HitKeep team settings](./.github/assets/admin-team-settings.png)

### Teams — Activity
![HitKeep team audit activity log](./.github/assets/admin-team-audit.png)

</details>

> **HitKeep Cloud is coming!**
>
> Prefer a managed solution and funding Open Source? Join the **Early Access Waitlist** for fully managed, data-sovereign and privacy-first analytics in the EU or US.
>
> [**Join the Waitlist →**](https://hitkeep.com/cloud)

## Documentation

Visit **[hitkeep.com](https://hitkeep.com)** for the complete documentation, including:

- [Installation Guides (Docker, K8s, Systemd)](https://hitkeep.com/guides/installation/)
- [Configuration Reference](https://hitkeep.com/reference/configuration/)
- [REST API Reference](https://hitkeep.com/api/)

---

## Features

- **Single Binary Runtime:** No external database, queue, or cache to provision.
- **Embedded DuckDB + NSQ:** Columnar analytics storage with in-process burst buffering.
- **Privacy-First Tracking:** Cookie-less by default, bot filtering, DNT support, optional `sendBeacon` disable.
- **Analytics Coverage:** Traffic overview, raw hits, events, goals, funnels, and UTM attribution fields.
- **Teams & Multitenancy:** Shared control plane plus isolated per-team analytics databases, team invites, ownership transfer, and cross-team site moves.
- **Security & Auth:** JWT sessions, remember-me tokens, password reset, TOTP MFA, and WebAuthn passkeys.
- **RBAC & Team Management:** Instance roles and per-site roles with delegated permissions.
- **API Clients:** Create scoped API tokens for automation and integrations.
- **Share Links:** Read-only dashboard sharing for stakeholders.
- **Data Lifecycle:** Per-site retention, scheduled archival to Parquet, and on-demand user/site takeout exports.
- **Ops Endpoints:** Health/readiness probes and versioned OpenAPI endpoint.
- **Cluster Support:** Optional leader/follower topology via memberlist gossip.

## Quick Start

### Binary

Head over to [Releases](https://github.com/PascaleBeier/hitkeep/releases) and download the binary for your system, for example:

```bash
$ wget https://github.com/PascaleBeier/hitkeep/releases/latest/download/hitkeep-linux-arm64
$ chmod +x hitkeep-linux-arm64
```

#### Running

> **Security Tip:** Avoid passing secrets (like `JWT_SECRET`) via flags in production, as they appear in process lists. Use the `HITKEEP_JWT_SECRET` environment variable instead.

```bash
# Set secret via ENV, config via flags
$ export HITKEEP_JWT_SECRET="your-secure-random-string"
$ ./hitkeep-linux-arm64 -public-url="https://analytics.example.org"
# to use your public ip
$ ./hitkeep-linux-arm64 -public-url="http://1.2.3.4:8080"
```

### Docker

Also see [examples](./examples/).

Images are published to both registries on every release:

| Registry | Image |
| :--- | :--- |
| GitHub Container Registry | `ghcr.io/pascalebeier/hitkeep` |
| Docker Hub | `pascalebeier/hitkeep` |

1.  Create a `compose.yml` file:

```yaml
services:
  hitkeep:
    # or: ghcr.io/pascalebeier/hitkeep:latest
    image: pascalebeier/hitkeep:latest
    container_name: hitkeep
    restart: unless-stopped
    ports:
      - "8080:8080"
    volumes:
      - hitkeep_data:/var/lib/hitkeep/data
    environment:
      # Securely pass secrets via ENV
      - HITKEEP_JWT_SECRET=replace-this-with-a-long-random-string
    command:
      # IMPORTANT: Set this to your actual public domain in production
      - "-public-url=http://localhost:8080"

volumes:
  hitkeep_data: {}
```

2.  Run it:

    ```bash
    docker compose up -d
    ```

3.  Open `http://localhost:8080` to create your admin account.

## Tracking Snippet

Once your instance is running and you have created a website in the dashboard, add this script to the `<head>` of your website:

```html
<script async src="https://your-hitkeep-instance.com/hk.js"></script>
```

### Do-Not-Track

To ignore DNT:

```html
<script
  async
  src="https://your-hitkeep-instance.com/hk.js"
  data-collect-dnt="true"
></script>
```

### fetch over sendBeacon

To use fetch over navigator.sendBeacon:

```html
<script
  async
  src="https://your-hitkeep-instance.com/hk.js"
  data-collect-dnt="true"
  data-disable-beacon="true"
></script>
```

### Custom events

Emit custom events from your app:

```html
<script>
  window.hk = window.hk || {};
  window.hk.event?.("signup", { plan: "pro", source: "landing-page" });
</script>
```

## Configuration

HitKeep is configured via command-line flags or environment variables. Flags take precedence.

### Most Relevant Production Settings

| Flag               | Environment Variable      | Why it matters most in production |
| :----------------- | :------------------------ | :-------------------------------- |
| `-public-url`      | `HITKEEP_PUBLIC_URL`      | Used for JWT issuer/audience and CORS behavior. |
| `-jwt-secret`      | `HITKEEP_JWT_SECRET`      | Signs auth tokens; must be stable and secret across restarts. |
| `-db`              | `HITKEEP_DB_PATH`         | Controls where `hitkeep.db` is stored/persisted. |
| `-data-path`       | `HITKEEP_DATA_PATH`       | Base directory for tenant databases and other local state. Back up this directory, not only `hitkeep.db`. |
| `-archive-path`    | `HITKEEP_ARCHIVE_PATH`    | Stores takeout and retention archives. |
| `-trusted-proxies` | `HITKEEP_TRUSTED_PROXIES` | Controls whether forwarded headers are trusted for real IP and GeoIP. |
| `-retention-days`  | `HITKEEP_DATA_RETENTION_DAYS` | Default retention policy for new sites. |

### Notes on Recent/Important Defaults

- `-trusted-proxies` now defaults to `*` (trust-all CIDRs). Set this explicitly in production to your reverse proxy/load balancer CIDRs.
- `-jwt-secret` is auto-generated if omitted. That is fine for local dev but will invalidate sessions on restart unless you persist a fixed secret.
- In multiteam installs, the backup boundary is the whole `-data-path` tree. The shared control plane stays at `{data-path}/hitkeep.db`; non-default team analytics live under `{data-path}/tenants/{team_id}/hitkeep.db`.

### General Settings

| Flag           | Environment Variable | Default                 | Description                                                                       |
| :------------- | :------------------- | :---------------------- | :-------------------------------------------------------------------------------- |
| `-public-url`  | `HITKEEP_PUBLIC_URL` | `http://localhost:8080` | Public URL for JWT issuer/audience and CORS. Set this to your real public domain. |
| `-jwt-secret`  | `HITKEEP_JWT_SECRET` | _(random)_              | Secret key for signing auth tokens. Use a fixed strong value in production.       |
| `-http`        | `HITKEEP_HTTP_ADDR`  | `:8080`                 | Address to bind the HTTP server to.                                               |
| `-healthcheck` | -                    | `false`                 | Run one-shot healthcheck mode and exit (useful for container probes).             |
| `-db`          | `HITKEEP_DB_PATH`    | `hitkeep.db`            | Path to the DuckDB database file.                                                 |
| `-log-level`   | `HITKEEP_LOG_LEVEL`  | `info`                  | Logging verbosity (`debug`, `info`, `warn`, `error`).                             |

### Data Management

| Flag              | Environment Variable          | Default   | Description                                                   |
| :---------------- | :---------------------------- | :-------- | :------------------------------------------------------------ |
| `-archive-path`   | `HITKEEP_ARCHIVE_PATH`        | `archive` | Directory for exports, rollups, and archival artifacts.       |
| `-retention-days` | `HITKEEP_DATA_RETENTION_DAYS` | `365`     | Default data retention window (days) for newly created sites. |

### Mailer (SMTP)

| Flag                         | Environment Variable                | Default             | Description                                                     |
| :--------------------------- | :---------------------------------- | :------------------ | :-------------------------------------------------------------- |
| `-mail-driver`               | `HITKEEP_MAIL_DRIVER`               | `smtp`              | Mail driver to use (`smtp` or `log`).                           |
| `-mail-host`                 | `HITKEEP_MAIL_HOST`                 |                     | SMTP Server Hostname (e.g., `smtp.postmarkapp.com`).            |
| `-mail-port`                 | `HITKEEP_MAIL_PORT`                 | `587`               | SMTP Server Port.                                               |
| `-mail-username`             | `HITKEEP_MAIL_USERNAME`             |                     | SMTP Username.                                                  |
| `-mail-password`             | `HITKEEP_MAIL_PASSWORD`             |                     | SMTP Password.                                                  |
| `-mail-encryption`           | `HITKEEP_MAIL_ENCRYPTION`           | `tls`               | Encryption mode: `tls` (STARTTLS), `ssl` (Implicit), or `none`. |
| `-mail-insecure-skip-verify` | `HITKEEP_MAIL_INSECURE_SKIP_VERIFY` | `false`             | Skip TLS certificate validation (useful for self-signed certs). |
| `-mail-from-address`         | `HITKEEP_MAIL_FROM_ADDRESS`         | `hitkeep@localhost` | The email address messages are sent from.                       |
| `-mail-from-name`            | `HITKEEP_MAIL_FROM_NAME`            | `HitKeep`           | The name displayed to the recipient.                            |

### Rate Limiting

| Flag            | Environment Variable        | Default | Description                                        |
| :-------------- | :-------------------------- | :------ | :------------------------------------------------- |
| `-ingest-rate`  | `HITKEEP_INGEST_RATE_LIMIT` | `20.0`  | Rate limit for `/ingest` (req/sec/ip).             |
| `-ingest-burst` | `HITKEEP_INGEST_BURST`      | `40`    | Burst size for `/ingest`.                          |
| `-api-rate`     | `HITKEEP_API_RATE_LIMIT`    | `10.0`  | Rate limit for general API endpoints (req/sec/ip). |
| `-api-burst`    | `HITKEEP_API_BURST`         | `20`    | Burst size for general API.                        |
| `-auth-rate`    | `HITKEEP_AUTH_RATE_LIMIT`   | `2.0`   | Rate limit for login/signup (req/sec/ip).          |
| `-auth-burst`   | `HITKEEP_AUTH_BURST`        | `5`     | Burst size for login/signup.                       |

### Trusted Proxies

Use this when HitKeep is behind a reverse proxy or load balancer and you want to trust forwarded headers.
This affects both **rate limiting** and **GeoIP** resolution.

| Flag               | Environment Variable      | Default | Description                                                                               |
| :----------------- | :------------------------ | :------ | :---------------------------------------------------------------------------------------- |
| `-trusted-proxies` | `HITKEEP_TRUSTED_PROXIES` | `"*"`   | Comma-separated trusted proxy CIDRs or `*` (trust all). Used for client IP and GeoIP resolution. |

Behavior:

- `*` trusts forwarding headers from any direct peer.
- CIDR list trusts forwarding headers only when the direct connection IP is in the trusted list.
- Empty disables trusted proxy behavior and uses the direct remote address.

### Clustering & Internals

| Flag                | Environment Variable       | Default              | Description                                   |
| :------------------ | :------------------------- | :------------------- | :-------------------------------------------- |
| `-name`             | `HITKEEP_NODE_NAME`        | `hostname-timestamp` | Unique name for this node in the cluster.     |
| `-bind`             | `HITKEEP_BIND_ADDR`        | `0.0.0.0:7946`       | Bind address for cluster gossip (Memberlist). |
| `-join`             | `HITKEEP_JOIN_ADDR`        | `""`                 | Address of a peer node to join.               |
| `-nsq-tcp-address`  | `HITKEEP_NSQ_TCP_ADDRESS`  | `127.0.0.1:4150`     | Address of the internal embedded NSQ TCP.     |
| `-nsq-http-address` | `HITKEEP_NSQ_HTTP_ADDRESS` | `127.0.0.1:4151`     | Address of the internal embedded NSQ HTTP.    |

## FAQ

### How much data storage will I need?

As of now, without any parqueting, you can expect to store 1 Million Raw Hits per ~120MB.

## Architecture

HitKeep bridges the gap between simple log analyzers (like GoAccess) and enterprise analytics (like Umami/Plausible).

1.  **Ingestion:** Requests hit the Go HTTP server.
2.  **Buffering:** Events are published to an **embedded NSQ** topic (`hits`) in memory. This decouples the API from the database write speed.
3.  **Storage:** An internal consumer creates micro-batches and writes them to **DuckDB**, a columnar database that lives in a single file but offers OLAP performance comparable to ClickHouse.
4.  **Clustering:** Nodes communicate via Gossip protocol. The **Leader** node handles database writes, while **Follower** nodes proxy ingestion traffic to the leader.

## Development

### Prerequisites

- Go 1.26+
- Node.js 24+
- Make

### Build from source

```bash
# Clone the repo
git clone https://github.com/pascalebeier/hitkeep.git
cd hitkeep

# Build frontend and backend
make build

# Run the binary
./hitkeep
```

## Changelog

We use SemVer and Conventional Commits.

See [CHANGELOG.md](./CHANGELOG.md).

## License

Distributed under the MIT License. See `LICENSE` for more information.
