# Authentication and permissions

TierSum separates **who can call the API** into two tracks: **human (browser)** and **program (integrations / MCP)**. Both are backed by the same database (`users`, `browser_sessions`, `api_keys`, `system_state`) and enforced in **`internal/api`** with **`internal/service`** (wired by `svcimpl.NewProgramAuth` + `svcimpl.NewAuthService`).

For **day-to-day usage** (bootstrap, login, keys, UI), see **[Access control and permissions (user guide)](../README.md#access-control-and-permissions-user-guide)** in the root **README**.

---

## 1. Goals and threat model

- **No shared static API key in config** for `/api/v1`: credentials live in the DB and are shown once at creation (bootstrap or admin UI).
- **Browser UI** uses **HttpOnly cookies** so JavaScript cannot read the session secret; CSRF posture is same-site lax cookie + same-origin UI.
- **Programmatic access** uses **scoped API keys** so read-only automation cannot ingest documents unless given `write` (or `admin`).
- **MCP** reuses the **same** API-key validation and scopes as REST (`TIERSUM_API_KEY` or `mcp.api_key` in config).

Public (no TierSum auth): **`GET /health`**, **`GET /metrics`** at the server root. **`/mcp/*`** is not gated by browser session but **is** gated by program-track API key when tools run.

---

## 2. Data model (schema v9+)

| Entity | Purpose |
| ------ | ------- |
| **`system_state`** | Singleton row; `initialized_at` set after first bootstrap. While uninitialized, protected `/api/v1` and `/bff/v1` reject with `SYSTEM_NOT_INITIALIZED` / `auth_state_unavailable` on DB errors. |
| **`users`** | Human identities: `username`, **`role`** `admin` \| `user`, hashed **access token** (`ts_u_*` plaintext shown once), optional sliding expiry, **`max_devices`**. |
| **`browser_sessions`** | Bound browsers: hashed session cookie, fingerprint hash, IP/UA hints, `device_alias`, sliding `expires_at` / `last_seen_at`. |
| **`api_keys`** | Service credentials: name, **`scope`** `read` \| `write` \| `admin`, hashed key (`tsk_live_*` / `tsk_admin_*` prefixes in plaintext), optional expiry, revoke, audit metadata. |
| **`api_key_audit`** | Optional usage rows for admin reporting (`CountsPerKeySince`). |

**Human access token** format: `ts_u_<random>` (stored as SHA-256 hex only).

**API key** format: `tsk_live_<random>` or `tsk_admin_<random>` depending on scope at creation (stored as hash only).

---

## 3. Human track (browser / BFF)

**Mount:** **`/bff/v1`**. Middleware: **`BFFSessionMiddleware`** (`internal/api/bff_session_middleware.go`) on the group except **public** paths:

- `GET /bff/v1/system/status`
- `POST /bff/v1/system/bootstrap`
- `POST /bff/v1/auth/login`
- `POST /bff/v1/auth/logout`

**Bootstrap** (`IAuthService.Bootstrap`): creates first **`admin`** user, one **read** API key, sets `system_state`. Returns plaintext secrets once.

**Login** (`POST /bff/v1/auth/login`): body includes **access token** + **fingerprint** (timezone + optional client signal). Service checks token hash, optional user token expiry (**slide** mode), then **device cap**: distinct active fingerprints vs `max_devices`; may evict same-fingerprint old session. Issues opaque **session cookie** (`tiersum_session`, HttpOnly).

**Session validation**: cookie → session row → user row; loose IP / user-agent consistency; sliding session TTL (`auth.browser.session_ttl`); may slide user token validity (`auth.browser.slide_user_token_ttl`).

**`/bff/v1/me/*`**: authenticated any signed-in human — profile, list/patch/delete **own** devices, revoke all own sessions. **Admins** may PATCH/DELETE **any** `session_id` (support / cross-user sign-out).

**`/bff/v1/admin/*`**: **`BFFRequireAdmin`** — `BrowserPrincipal.Role == admin`. Routes include users, reset token, **`GET /admin/devices`** (all sessions + usernames; static route registered **before** `/admin/users/:id/devices` to avoid path ambiguity), API keys CRUD/revoke, usage snapshot, **`GET /admin/config/snapshot`** (read-only redacted `viper` tree for ops — no secrets in plaintext; UI **Management → Configuration** at `/admin/config`).

**Implementation map:** `auth_bff_handlers.go` (handlers), `bff_session_middleware.go`, service auth implementations (wired from `internal/di`), `internal/storage/db/auth/*_repository_impl.go` (auth tables), `internal/service/interface.go` (facades) / `auth_entities.go`.

---

## 4. Program track (`/api/v1` and MCP)

**Mount:** **`/api/v1`**. Middleware: **`ProgramAuthMiddleware`** (`internal/api/program_auth_middleware.go`).

1. Require system initialized.
2. Read **`X-API-Key`** or **`Authorization: Bearer <key>`**.
3. **`ValidateAPIKey`** → principal (`key_id`, `scope`, `name`).
4. **`apiRouteRequiredScope(method, path)`** — today:
   - **`write`** for `POST /api/v1/documents`, `POST /api/v1/topics/regroup`
   - **`read`** for everything else under `/api/v1`
5. **`APIKeyMeetsScope`**: ordered ranks `read < write < admin` (admin satisfies write and read).
6. **`RecordAPIKeyUse`** for audit + last-used fields.

**MCP:** `mcpProgramGate` in **`internal/api/mcp_gate.go`** uses the same service with env **`TIERSUM_API_KEY`** or **`mcp.api_key`** from config; tools call read or write gate matching REST semantics.

---

## 5. Roles vs scopes (terminology)

| Concept | Values | Where used |
| ------- | ------ | ---------- |
| **Human role** | `admin`, `user` | `users.role`, BFF admin routes |
| **API key scope** | `read`, `write`, `admin` | `api_keys.scope`, `/api/v1`, MCP |

An **admin user** in the browser is not the same as an **admin-scoped API key**; the latter is a service credential with superset HTTP access, not UI login.

---

## 6. Configuration

See **`configs/config.example.yaml`**:

- **`auth.browser`**: `session_ttl`, `slide_user_token_ttl`, `default_max_devices` (new users / bootstrap).
- **`mcp.api_key`**: optional static key string for MCP when env `TIERSUM_API_KEY` is unset (still a DB key value, not a bypass).

There is **no** `security.api_key` single shared secret for REST.

---

## 7. Related documents

| Document | Content |
| -------- | ------- |
| [CORE_API_FLOWS.md](CORE_API_FLOWS.md) | Mount points, bootstrap, login, admin/me BFF summary, MCP pointer. |
| [FRONTEND.md](../cmd/web/FRONTEND.md) | Which UI screens call which BFF routes. |
| [AGENTS.md](../AGENTS.md) | Repo layout, security bullets, layer rules. |
