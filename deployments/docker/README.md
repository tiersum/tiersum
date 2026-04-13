# Docker image (TierSum)

- **Dockerfile:** `Dockerfile` in this directory (build context = **repository root**).
- **Local build** (from repo root): `make docker-build` or `docker build -f deployments/docker/Dockerfile -t tiersum:local .`
- **Compose:** from this directory, `docker compose build` uses the same Dockerfile.

## GitHub Actions → Alibaba Cloud ACR

Workflow: `.github/workflows/docker-acr.yml` (**tags `v*` only**, or **Actions → Run workflow** manual run). Pushes to `main` do **not** trigger image builds.

Uses **two jobs** (`ubuntu-latest` for amd64, `ubuntu-24.04-arm64` for arm64): **native compile, no QEMU**.

**Tags pushed** (example namespace `myns`):

| Event | amd64 | arm64 |
|--------|--------|--------|
| push tag `v1.2.3` | `.../myns/tiersum:v1.2.3-amd64` (+ `sha-<12>-amd64`) | `.../myns/tiersum:v1.2.3-arm64` (+ `sha-<12>-arm64`) |
| **workflow_dispatch** | `.../myns/tiersum:manual-<12hexsha>-amd64` | `.../myns/tiersum:manual-<12hexsha>-arm64` |

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

### Pull from the registry

After login, pull by **namespace** + **image** + **tag** (tag must match your CPU: `-amd64` vs `-arm64`; there is no single `latest`):

```bash
docker pull crpi-gp9w4rgj2tki21xk.cn-hangzhou.personal.cr.aliyuncs.com/<ACR_NAMESPACE>/tiersum:<tag>
```

Example if your namespace is `tiersum` and you released `v1.2.3` on amd64:

```bash
docker pull crpi-gp9w4rgj2tki21xk.cn-hangzhou.personal.cr.aliyuncs.com/tiersum/tiersum:v1.2.3-amd64
```

Replace `<tag>` with a value from the Actions run (e.g. `v1.2.3-amd64`, `manual-abcdef123456-amd64`, or `sha-abcdef123456-amd64`).
