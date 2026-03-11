# HitKeep

> Privacy-first web analytics you can self-host or run in managed EU/US cloud.

[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)
[![Go Version](https://img.shields.io/badge/Go-1.26.1-00ADD8?logo=go)](https://go.dev/)
[![Docker Image (GHCR)](https://img.shields.io/badge/Docker-ghcr.io-blue?logo=docker)](https://github.com/pascalebeier/hitkeep/pkgs/container/hitkeep)
[![Docker Image (Hub)](https://img.shields.io/badge/Docker-Docker_Hub-2496ED?logo=docker)](https://hub.docker.com/r/pascalebeier/hitkeep)
[![Documentation](https://img.shields.io/badge/Docs-hitkeep.com-0ea5e9)](https://hitkeep.com)
[![OpenSSF Best Practices](https://www.bestpractices.dev/projects/11990/badge)](https://www.bestpractices.dev/projects/11990)

HitKeep is an open source web analytics platform built for people who want a simpler stack than the usual PostgreSQL, Redis, ClickHouse, and reverse-proxy pileup.

- Single binary runtime
- Embedded DuckDB and NSQ
- Privacy-first tracking
- Goals, funnels, ecommerce, email reports, and API clients
- Self-hosted or managed cloud with EU/US region choice

[Website](https://hitkeep.com) · [Cloud](https://hitkeep.com/cloud) · [Docs](https://hitkeep.com/guides/introduction/) · [API](https://hitkeep.com/api/) · [Releases](https://github.com/PascaleBeier/hitkeep/releases)

![HitKeep analytics dashboard — traffic overview, geographic breakdown, goals, funnels, and UTM attribution](./.github/assets/dashboard-overview.png)

## Why HitKeep

HitKeep is for teams that want product analytics without adopting a full analytics platform stack.

- **Simple to run:** one binary, one data directory, no external database required
- **Privacy-first by default:** cookie-less tracking, Do Not Track support, focused data collection
- **Useful out of the box:** traffic analytics, custom events, goals, funnels, ecommerce, UTM attribution, and scheduled email reports
- **Built for teams:** passkeys, TOTP, site and team permissions, API clients, share links, and audit visibility
- **Flexible deployment:** self-host it yourself or use HitKeep Cloud and still keep the migration path open

## Quick Start

### Binary

Download the latest release for your system:

```bash
wget https://github.com/PascaleBeier/hitkeep/releases/latest/download/hitkeep-linux-arm64
chmod +x hitkeep-linux-arm64
export HITKEEP_JWT_SECRET="replace-this-with-a-long-random-string"
./hitkeep-linux-arm64 -public-url="http://localhost:8080"
```

Open `http://localhost:8080` and create your first account.

### Docker

```yaml
services:
  hitkeep:
    image: pascalebeier/hitkeep:latest
    restart: unless-stopped
    ports:
      - "8080:8080"
    volumes:
      - hitkeep_data:/var/lib/hitkeep/data
    environment:
      HITKEEP_JWT_SECRET: replace-this-with-a-long-random-string
    command:
      - "-public-url=http://localhost:8080"

volumes:
  hitkeep_data: {}
```

```bash
docker compose up -d
```

For production setup, reverse proxies, SMTP, systemd, Kubernetes, S3 archiving, and every configuration flag, use the docs instead of this README:

- [Installation guides](https://hitkeep.com/guides/installation/)
- [Configuration reference](https://hitkeep.com/reference/configuration/)
- [Cloud documentation](https://hitkeep.com/cloud)

## Tracking Snippet

Once your instance is running and a site is created, add:

```html
<script async src="https://your-hitkeep-instance.com/hk.js"></script>
```

Custom event example:

```html
<script>
  window.hk = window.hk || {};
  window.hk.event?.("signup", { plan: "pro", source: "landing-page" });
</script>
```

Tracker options, ecommerce events, custom events, and advanced tracking examples live here:

- [Tracking docs](https://hitkeep.com/guides/tracking/)
- [Custom events](https://hitkeep.com/guides/tracking/custom-events/)
- [Ecommerce analytics](https://hitkeep.com/guides/analytics/ecommerce/)

## Product Tour

<details>
<summary>See more screenshots</summary>

### Events
![HitKeep custom event analytics with timeseries chart and property breakdown](./.github/assets/analytics-events.png)

### Goals
![HitKeep goals and conversion tracking](./.github/assets/analytics-goals.png)

### Funnels
![HitKeep multi-step funnel analytics](./.github/assets/analytics-funnels.png)

### Ecommerce
![HitKeep ecommerce analytics with revenue KPIs, chart, top products, and revenue sources](./.github/assets/analytics-ecommerce.png)

### Email Reports
![HitKeep scheduled email reports](./.github/assets/feature-email-reports.png)

### API Clients
![HitKeep API client management](./.github/assets/security-api-clients.png)

### Team Overview
![HitKeep team administration overview](./.github/assets/admin-team-overview.png)

</details>

## Documentation

The maintained reference lives on `hitkeep.com`.

- [Getting started](https://hitkeep.com/guides/introduction/)
- [Installation](https://hitkeep.com/guides/installation/)
- [Configuration](https://hitkeep.com/reference/configuration/)
- [REST API reference](https://hitkeep.com/api/)
- [Compliance](https://hitkeep.com/compliance/overview/)
- [Privacy policy for cloud](https://hitkeep.com/legal/privacy-policy/)
- [Terms of service](https://hitkeep.com/legal/terms-of-service/)
- [Comparison pages](https://hitkeep.com/vs/)

## Cloud

HitKeep Cloud is live.

If you want the same product without running it yourself, start here:

- [Start in the EU](https://cloud.hitkeep.eu/signup)
- [Start in the US](https://cloud.hitkeep.com/signup)
- [Cloud overview](https://hitkeep.com/cloud)

## Development

Prerequisites:

- Go 1.26+
- Node.js 24+
- Make

Build from source:

```bash
git clone https://github.com/pascalebeier/hitkeep.git
cd hitkeep
make build
./hitkeep
```

Contributor docs and project guides:

- [Development docs](https://hitkeep.com/guides/development/)
- [Changelog](./CHANGELOG.md)

## License

Distributed under the MIT License. See [LICENSE](./LICENSE).
