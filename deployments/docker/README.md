# Docker image (TierSum)

- **Dockerfile:** `Dockerfile` in this directory (build context = **repository root**).
- **Local build** (from repo root): `make docker-build` or `docker build -f deployments/docker/Dockerfile -t tiersum:local .`
- **Compose:** from this directory, `docker compose build` uses the same Dockerfile.

## GitHub Actions → Alibaba Cloud ACR

Workflow: `.github/workflows/docker-acr.yml` (push to `main`, tags `v*`, or manual run).

Uses **two jobs** (`ubuntu-latest` for amd64, `ubuntu-24.04-arm64` for arm64): **native compile, no QEMU**.

**Tags pushed** (example namespace `myns`):

| Event | amd64 | arm64 |
|--------|--------|--------|
| push `main` | `.../myns/tiersum:latest-amd64` (+ `sha-<12>-amd64`) | `.../myns/tiersum:latest-arm64` (+ `sha-<12>-arm64`) |
| push tag `v1.2.3` | `.../myns/tiersum:v1.2.3-amd64` | `.../myns/tiersum:v1.2.3-arm64` |

There is **no** single multi-arch `latest` manifest; pull the tag that matches your CPU (or run amd64 everywhere).

Configure these **repository secrets**:

| Secret | Example / note |
|--------|----------------|
| `ACR_USERNAME` | `luodaijun-9030` |
| `ACR_PASSWORD` | ACR password or access token (do not commit) |
| `ACR_NAMESPACE` | Namespace name from ACR console (命名空间) |

Image: `crpi-gp9w4rgj2tki21xk.cn-hangzhou.personal.cr.aliyuncs.com/<ACR_NAMESPACE>/tiersum:<tag>`.

Local login:

```bash
docker login --username=YOUR_USER crpi-gp9w4rgj2tki21xk.cn-hangzhou.personal.cr.aliyuncs.com
```
