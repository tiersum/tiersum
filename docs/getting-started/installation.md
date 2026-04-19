# Installation Guide

## Prerequisites

- **Go 1.23+** (with CGO enabled for SQLite)
- **Database**: SQLite (default, zero-config) or PostgreSQL (optional)
- **LLM API Key**: OpenAI, Anthropic, or compatible provider

## Quick Install

```bash
# Clone repository
git clone https://github.com/tiersum/tiersum.git
cd tiersum

# Install Go dependencies
make deps

# Copy and edit configuration
cp configs/config.example.yaml configs/config.yaml

# Set required environment variables
export OPENAI_API_KEY="your-api-key"
# or
export ANTHROPIC_API_KEY="your-api-key"

# Build (includes embedded frontend)
make build
```

## Docker Compose (Recommended for Production)

TierSum provides a pre-built Docker image hosted on **Alibaba Cloud Container Registry (ACR)** Personal Edition.

### Pull and Run

```bash
# Login to ACR (use your Alibaba Cloud credentials)
docker login --username=your-alibaba-cloud-username crpi-gp9w4rgj2tki21xk.cn-hangzhou.personal.cr.aliyuncs.com

# Pull the image (replace <tag> with actual version, e.g., latest or v1.0.0)
docker pull crpi-gp9w4rgj2tki21xk.cn-hangzhou.personal.cr.aliyuncs.com/tiersum/tiersum:<tag>

# Run with Docker Compose
cd deployments/docker && docker-compose up -d
```

### Image Reference

| Registry | Repository | Tag |
|----------|------------|-----|
| `crpi-gp9w4rgj2tki21xk.cn-hangzhou.personal.cr.aliyuncs.com` | `tiersum/tiersum` | `<tag>` (e.g., `latest`, `v1.0.0`) |

**Note:** The image includes:
- Pre-built TierSum binary
- Embedded Vue 3 frontend
- ONNX Runtime and MiniLM model files (optional; can be mounted separately)
- Default SQLite configuration

Default setup uses SQLite with volume-mounted data directory. See `deployments/docker/README.md` for compose configuration details.

#### Troubleshooting: gojieba dictionary files

If you see `panic: Dictionary file does not exist: .../gojieba/.../jieba.dict.utf8` when starting the container, the gojieba Chinese tokenizer dictionary files are missing from the final image. This was fixed in recent Dockerfile versions; ensure your image includes:

```dockerfile
COPY --from=builder /go/pkg/mod/github.com/yanyiwu/gojieba@*/deps/cppjieba/dict /app/deps/cppjieba/dict
```

Rebuild the image if using a custom Dockerfile.

## Cold Document Embeddings (Optional)

Semantic vectors for the **cold** index use **all-MiniLM-L6-v2** ONNX files on disk plus the **ONNX Runtime** shared library:

```bash
make fetch-onnxruntime   # ONNX .so / .dylib per platform
make fetch-minilm        # model.onnx + tokenizer.json from Hugging Face
```

If MiniLM fails to load and `cold_index.embedding.provider` is `auto`, TierSum falls back to simple hash embeddings.

See [third_party/onnxruntime/README.md](../../third_party/onnxruntime/README.md) and [third_party/minilm/README.md](../../third_party/minilm/README.md).

## Start Server

```bash
# Run locally (backend + embedded frontend)
make run

# Or run binary directly
./build/tiersum --config configs/config.yaml

# Server ready
# Web UI:   http://localhost:8080/
# REST API: http://localhost:8080/api/v1
# BFF:      http://localhost:8080/bff/v1
# Health:   http://localhost:8080/health
# Metrics:  http://localhost:8080/metrics
# MCP SSE:  http://localhost:8080/mcp/sse
```

## First Launch (Bootstrap)

1. Open the web UI (`http://localhost:8080/`). If uninitialized, you are redirected to **`/init`**.
2. Submit **Initialize** with the desired **admin username**.
3. The response shows, **once**:
   - **Admin access token** (`ts_u_…`) — for browser login
   - **Initial API key** (`tsk_live_…`) — for scripts and automation

Store these safely; they cannot be retrieved again.

See [Auth and Permissions](../design/auth-and-permissions.md) for the full dual-track auth design.

---

## Run as Systemd Service (Ubuntu / Debian / CentOS / RHEL)

For production deployments on Linux distributions using **systemd** (Ubuntu 20.04+, Debian 10+, CentOS 7+, RHEL 7+), running TierSum as a system service provides automatic startup, restart on failure, and log management via `journalctl`.

### 1. Create a dedicated user (recommended)

```bash
sudo useradd --system --home /opt/tiersum --shell /bin/false tiersum
```

### 2. Install the binary and config

```bash
# Build on the target machine (or copy from build machine)
sudo mkdir -p /opt/tiersum
sudo cp -r build configs third_party /opt/tiersum/
sudo chown -R tiersum:tiersum /opt/tiersum
```

### 3. Create the systemd service file

Create `/etc/systemd/system/tiersum.service`:

```ini
[Unit]
Description=TierSum Hierarchical Summary Knowledge Base
Documentation=https://github.com/tiersum/tiersum
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
User=tiersum
Group=tiersum
WorkingDirectory=/opt/tiersum

# Method A: Inline environment variables (simple but visible in `ps`)
Environment="OPENAI_API_KEY=sk-your-key-here"
# Environment="ANTHROPIC_API_KEY=sk-ant-your-key-here"

# Method B: Environment file (recommended for production)
# Create /opt/tiersum/.env first (see format below)
# EnvironmentFile=/opt/tiersum/.env

ExecStart=/opt/tiersum/build/tiersum --config /opt/tiersum/configs/config.yaml
ExecReload=/bin/kill -HUP $MAINPID

Restart=on-failure
RestartSec=5
StartLimitInterval=60s
StartLimitBurst=3

[Install]
WantedBy=multi-user.target
```

#### Environment file format

If using `EnvironmentFile`, create `/opt/tiersum/.env`:

```bash
sudo tee /opt/tiersum/.env << 'EOF'
OPENAI_API_KEY=sk-your-actual-key-here
EOF

# Must be readable by the tiersum user
sudo chmod 600 /opt/tiersum/.env
sudo chown tiersum:tiersum /opt/tiersum/.env
```

**Important:** Do **not** add quotes around values in systemd env files.
`OPENAI_API_KEY=sk-...` is correct; `OPENAI_API_KEY="sk-..."` is wrong.

#### Data directory (SQLite default)

If using SQLite (the default), ensure the data directory exists:

```bash
sudo mkdir -p /opt/tiersum/data
sudo chown -R tiersum:tiersum /opt/tiersum/data
```

> **Note on security hardening:** Options like `ProtectSystem=strict` or `NoNewPrivileges=true` can cause `Failed to set up mount namespacing` errors if directories are missing. Only add these after confirming the basic service starts successfully.

### 4. Reload systemd and start the service

```bash
sudo systemctl daemon-reload
sudo systemctl enable tiersum
sudo systemctl start tiersum
```

### 5. Verify status and logs

```bash
# Check service status
sudo systemctl status tiersum

# View logs
sudo journalctl -u tiersum -f

# View logs since last boot
sudo journalctl -u tiersum --since today
```

### 6. Service management commands

```bash
# Start
sudo systemctl start tiersum

# Stop
sudo systemctl stop tiersum

# Restart (e.g. after config change)
sudo systemctl restart tiersum

# Reload config without restarting (if supported)
sudo systemctl reload tiersum

# Disable auto-start on boot
sudo systemctl disable tiersum

# Re-enable auto-start
sudo systemctl enable tiersum
```

### 7. Nginx reverse proxy (recommended for public access)

When running behind Nginx on the same host:

1. Set `server.host: "127.0.0.1"` in `config.yaml` so TierSum only accepts local connections.
2. Configure Nginx with TLS termination:

```nginx
server {
    listen 443 ssl http2;
    server_name tiersum.example.com;

    ssl_certificate /etc/nginx/ssl/tiersum.crt;
    ssl_certificate_key /etc/nginx/ssl/tiersum.key;

    location / {
        proxy_pass http://127.0.0.1:8080;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }
}
```

3. Set `auth.browser.trust_proxy_headers: true` and `auth.browser.cookie_secure_mode: auto` (or `always`) in `config.yaml` for correct cookie behavior behind HTTPS.
