# Docker image (TierSum)

- **Dockerfile:** `Dockerfile` in this directory (build context = **repository root**).
- **Local build** (from repo root): `make docker-build` or `docker build -f deployments/docker/Dockerfile -t tiersum:local .`
- **Compose:** from this directory, `docker compose build` uses the same Dockerfile.

## GitHub Actions → Alibaba Cloud ACR

Workflow: `.github/workflows/docker-acr.yml` (push to `main`, tags `v*`, or manual run).

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
