# HitKeep Helm Chart

This chart deploys the HitKeep single-binary service on Kubernetes.

## Kubernetes Guide

### Rationale

HitKeep follows the standard Helm pattern used by major charts: the chart emits a Service and (optionally) an Ingress, while the reverse proxy/ingress controller is provided by the cluster. This keeps the chart simple, avoids bundling a proxy, and fits most Kubernetes setups.

### Install (OCI via GHCR)

```
helm install hitkeep oci://ghcr.io/pascalebeier/charts/hitkeep --version <x.y.z>
```

### Minimal values

```
image:
  repository: ghcr.io/pascalebeier/hitkeep
  tag: v1.0.0

env:
  HITKEEP_PUBLIC_URL: "https://analytics.example.com"
```

### Core configuration

- `domain`: convenience host for basic ingress (used when `ingress.hosts` is empty)
- `service.type`: `ClusterIP` (default), `LoadBalancer`, or `NodePort`
- `ingress.enabled`: creates an Ingress resource for your cluster's controller
- `ingress.className`: leave empty to use the cluster default
- `ingress.annotations`: controller-specific settings (cert-manager, auth, etc.)
- `env.HITKEEP_JWT_SECRET`: set for stable auth sessions
- `persistence.*`: PVC settings for `/var/lib/hitkeep/data`
- Probes: liveness uses `/healthz`, readiness uses `/readyz`

### Scaling & clustering

Set `replicaCount` to 2+ to enable clustering. The chart uses a StatefulSet with stable pod names and configures gossip discovery via a headless service.

```
replicaCount: 3
```

When clustered, the chart sets `HITKEEP_BIND_ADDR` to the pod IP and joins via the headless service.

### Ingress (standard cluster setup)

```
domain: analytics.example.com

ingress:
  enabled: true
  className: "" # use cluster default
```

### Advanced Ingress (multiple hosts/paths)

```
ingress:
  enabled: true
  hosts:
    - host: analytics.example.com
      paths:
        - path: /
          pathType: Prefix
```

### LoadBalancer (no ingress controller)

```
service:
  type: LoadBalancer

ingress:
  enabled: false
```

### TLS with cert-manager (Let’s Encrypt)

This assumes cert-manager is installed and a ClusterIssuer exists.

```
ingress:
  enabled: true
  annotations:
    cert-manager.io/cluster-issuer: "letsencrypt-prod"
  tls:
    - hosts:
        - analytics.example.com
      secretName: hitkeep-tls
```

### Persistence

The chart mounts a PVC at `/var/lib/hitkeep/data` by default.

```
persistence:
  enabled: true
  size: 10Gi
  accessMode: ReadWriteOnce
```
