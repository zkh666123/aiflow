# Go Identity and Access Compatibility Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Implement the Go control-plane routes for users, applications, teams, API keys, application sharing, and three-level RBAC while preserving the frozen public HTTP contract and native Windows/WSL runtime.

**Architecture:** Gin owns strict HTTP validation and response envelopes, domain services own authorization and business rules, sqlc-backed repositories own all `control` schema access, and Redis owns distributed login-attempt state. JWT, API-key HMAC, and gRPC secrets remain separate; PostgreSQL stores password hashes and API-key digests but never plaintext credentials.

**Tech Stack:** Go 1.26, Gin, pgx v5, sqlc, Goose, go-redis, bcrypt, golang-jwt/jwt v5, PostgreSQL 16, Redis 7, Node contract tests, and native PowerShell lifecycle scripts.

---

## Contract Decisions

- Preserve all 34 legacy routes in this phase and add the approved compatibility route `GET /api/apps/:appId/share`, for 35 routes total.
- Preserve NestJS default status behavior: every successful `POST` in this phase returns HTTP 201; `GET`, `PATCH`, and `DELETE` return HTTP 200.
- Every non-streaming response uses `{success, code, message, data, timestamp}`; errors add `path` and set `data` to `null`.
- Registration accepts usernames matching `^[A-Za-z0-9_]{3,20}$` and passwords of at least six characters.
- Login locks a username after five failed attempts for 15 minutes. Attempt state is atomic in Redis and expires after one hour.
- JWT payloads retain `userId` and `username`; tokens use HS256 and expire after seven days.
- Global roles are `admin` and `member`; team roles are `owner`, `admin`, `editor`, and `viewer`; team application grants are `full_access`, `can_edit`, and `can_view`.
- Any team member can read team details, matching the effective legacy `TeamService` behavior; viewer/editor receive `team:read` even though the unused legacy decorator matrix omitted it.
- API keys contain 256 random bits, are shown once, and are stored as a display prefix plus HMAC-SHA256 digest. JWT, API-key HMAC, and gRPC secrets are distinct.
- Share management remains application-owner-only. Team `full_access` does not grant share-setting ownership.
- No legacy database data is copied. Goose creates a fresh schema and can roll the phase back cleanly.

## File Map

- `flowai-studio-control-plane/db/migrations/00002_identity_access.sql`: users, applications, teams, memberships, team grants, API keys, and shares.
- `flowai-studio-control-plane/db/schema/control.sql`: sqlc parser copy of the complete control schema.
- `flowai-studio-control-plane/sqlc.yaml`: reads the canonical schema copy once instead of reparsing migrations.
- `flowai-studio-control-plane/db/query/*.sql`: typed CRUD and authorization queries.
- `flowai-studio-control-plane/internal/auth/`: password hashing, JWT, Redis login lock, authenticated principal, and user service.
- `flowai-studio-control-plane/internal/rbac/`: permission constants and access resolution.
- `flowai-studio-control-plane/internal/applications/`: application CRUD and status transitions.
- `flowai-studio-control-plane/internal/teams/`: team, member, and team-application behavior.
- `flowai-studio-control-plane/internal/apikeys/`: HMAC API-key lifecycle.
- `flowai-studio-control-plane/internal/shares/`: owner-only share lifecycle and embed payload.
- `flowai-studio-control-plane/internal/httpapi/`: strict JSON decoding, errors, middleware, handlers, and route registration.
- `scripts/native/initialize-database.ps1`: ignored local JWT and API-key HMAC secret generation/backfill.
- `scripts/native/identity-access-contracts.test.ps1`: live PostgreSQL/Redis/API compatibility checks.

### Task 1: Add Runtime Secrets and Control Tables

**Files:**
- Modify: `flowai-studio-control-plane/internal/config/config_test.go`
- Modify: `flowai-studio-control-plane/internal/config/config.go`
- Modify: `scripts/native/environment-contracts.test.cjs`
- Modify: `scripts/native/initialize-database.ps1`
- Create: `flowai-studio-control-plane/db/migrations/00002_identity_access.sql`
- Modify: `flowai-studio-control-plane/db/schema/control.sql`
- Modify: `flowai-studio-control-plane/sqlc.yaml`
- Create: `flowai-studio-control-plane/db/query/users.sql`
- Create: `flowai-studio-control-plane/db/query/applications.sql`
- Create: `flowai-studio-control-plane/db/query/teams.sql`
- Create: `flowai-studio-control-plane/db/query/api_keys.sql`
- Create: `flowai-studio-control-plane/db/query/shares.sql`

- [x] **Step 1: Write failing configuration and environment tests**

Add assertions that `config.Load()` rejects missing or identical JWT/API-key secrets and accepts:

```go
t.Setenv("FLOWAI_JWT_SECRET", strings.Repeat("j", 32))
t.Setenv("FLOWAI_API_KEY_HMAC_SECRET", strings.Repeat("k", 32))
t.Setenv("FLOWAI_API_KEY_HMAC_PREVIOUS_SECRET", "")
t.Setenv("FLOWAI_FRONTEND_URL", "http://127.0.0.1:5173")
```

Add script-contract assertions that the ignored native environment contains both secret names and that existing environment files are backfilled without replacing existing values.

- [x] **Step 2: Run tests and verify RED**

Run:

```powershell
go test ./internal/config
node --test scripts/native/environment-contracts.test.cjs
```

Expected: FAIL because the new settings and initializer behavior do not exist.

- [x] **Step 3: Extend settings and native secret initialization**

Add these fields and validation:

```go
JWTSecret               string
APIKeyHMACSecret        string
APIKeyHMACPreviousSecret string
FrontendURL             string
JWTExpiration           time.Duration
```

Require current secrets to contain at least 32 non-blank characters, require them to differ from each other and the gRPC token, default `JWTExpiration` to `168h`, and validate `FrontendURL` as an absolute `http` or `https` URL. Update `initialize-database.ps1` to generate missing values with `New-Secret` while preserving an existing ignored environment file.

- [x] **Step 4: Add the reversible identity/access migration**

Create constrained tables with `uuid` IDs, `timestamptz`, foreign keys, and checks:

```sql
CREATE TABLE control.users (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    username varchar(20) NOT NULL UNIQUE,
    password_hash text NOT NULL,
    avatar text,
    global_role text NOT NULL DEFAULT 'member'
        CHECK (global_role IN ('admin', 'member')),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CHECK (username ~ '^[A-Za-z0-9_]{3,20}$')
);
```

Add `applications`, `teams`, `team_members`, `team_applications`, `api_keys`, and `app_shares` with the enum checks listed in Contract Decisions. Add indexes for ownership, membership, application grants, API-key prefix, and updated-time list ordering. The Goose down section drops child tables before parents.

- [x] **Step 5: Add typed sqlc queries and generate code**

Queries must cover exact business operations, including `CreateUser`, `GetUserByUsername`, `GetUserByID`, `UpdateUserProfile`, owned/team application lists, team membership and grant lookup, API-key owner operations, and application-share owner/public lookups. Use only sqlc parameters; no SQL string concatenation.

Run:

```powershell
sqlc generate -f flowai-studio-control-plane/sqlc.yaml
```

Expected: generated `internal/store/sqlc` code includes all new query methods.

- [x] **Step 6: Apply migrations and verify database ownership**

Run:

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File scripts/native/initialize-database.ps1
powershell -NoProfile -ExecutionPolicy Bypass -File scripts/native/database-contracts.test.ps1
```

Expected: migrations apply idempotently; `flowai_control` can perform DML in `control` but cannot create tables or write `ai`.

- [x] **Step 7: Run tests and commit the schema slice**

Run `go test ./internal/config`, the native environment tests, `sqlc generate`, and `git diff --check`. Commit only Task 1 files with message `feat: add control identity schema and secrets`.

### Task 2: Freeze HTTP Error, Validation, and Authentication Behavior

**Files:**
- Modify: `flowai-studio-control-plane/internal/httpapi/envelope.go`
- Create: `flowai-studio-control-plane/internal/httpapi/errors.go`
- Create: `flowai-studio-control-plane/internal/httpapi/json.go`
- Create: `flowai-studio-control-plane/internal/httpapi/middleware.go`
- Create: `flowai-studio-control-plane/internal/httpapi/authentication.go`
- Create: `flowai-studio-control-plane/internal/httpapi/authentication_test.go`
- Create: `flowai-studio-control-plane/internal/httpapi/compatibility_test.go`
- Create: `flowai-studio-control-plane/internal/auth/jwt.go`
- Create: `flowai-studio-control-plane/internal/auth/jwt_test.go`

- [x] **Step 1: Write failing envelope and strict-decoder tests**

Test success and error payloads against the frozen keys, verify POST handlers can emit 201, reject unknown JSON fields and trailing JSON values with 400 `BAD_REQUEST`, and verify panics become 500 `INTERNAL_ERROR` without stack traces.

- [x] **Step 2: Write failing JWT tests**

Tests create a seven-day HS256 token containing `userId` and `username`, reject wrong algorithms, expired tokens, malformed Bearer headers, and tokens signed by another secret.

- [x] **Step 3: Run focused tests and verify RED**

Run:

```powershell
go test ./internal/httpapi ./internal/auth
```

Expected: FAIL because strict decoding, API errors, and JWT services do not exist.

- [x] **Step 4: Implement boundary helpers**

Define:

```go
type APIError struct {
    Status  int
    Code    string
    Message string
    Cause   error
}

func DecodeJSON[T any](c *gin.Context) (T, *APIError)
func WriteSuccess(c *gin.Context, status int, data any)
func WriteError(c *gin.Context, err *APIError)
```

Use `json.Decoder.DisallowUnknownFields`, require one JSON value, preserve the request URI in error `path`, and keep internal causes out of the response.

- [x] **Step 5: Implement JWT service and middleware**

Define an authenticated principal:

```go
type Principal struct {
    UserID   string
    Username string
}
```

The Gin middleware accepts only `Authorization: Bearer <token>`, verifies HS256 and expiry, stores the principal in context, and emits stable 401 envelopes for missing/invalid tokens.

- [x] **Step 6: Run tests and commit the HTTP/auth boundary slice**

Run `go test ./internal/httpapi ./internal/auth`, `go vet ./...`, and `git diff --check`. Commit Task 2 files with message `feat: add compatible HTTP and JWT boundaries`.

### Task 3: Implement Users and Distributed Login Locking

**Files:**
- Create: `flowai-studio-control-plane/internal/auth/login_limiter.go`
- Create: `flowai-studio-control-plane/internal/auth/login_limiter_test.go`
- Create: `flowai-studio-control-plane/internal/auth/service.go`
- Create: `flowai-studio-control-plane/internal/auth/service_test.go`
- Create: `flowai-studio-control-plane/internal/store/user_repository.go`
- Create: `flowai-studio-control-plane/internal/store/user_repository_test.go`
- Create: `flowai-studio-control-plane/internal/httpapi/users.go`
- Create: `flowai-studio-control-plane/internal/httpapi/users_test.go`
- Modify: `flowai-studio-control-plane/internal/httpapi/router.go`
- Modify: `flowai-studio-control-plane/cmd/api/main.go`
- Create: `scripts/native/user-contracts.test.ps1`

- [x] **Step 1: Write failing Redis login-limiter tests**

Test five failures, remaining-attempt counts, 15-minute lock TTL, successful-login reset, lock expiry, and SHA-256-derived Redis keys that do not contain the username. The production limiter uses atomic Redis Lua scripts for check/fail/reset.

- [x] **Step 2: Write failing user-service tests**

Cover registration validation, duplicate username conflict, bcrypt cost 12 hashes, invalid-user and invalid-password paths sharing the same 401 response, lockout before password work, successful reset, profile lookup, and profile updates that never expose `password_hash`.

- [x] **Step 3: Run focused tests and verify RED**

Run:

```powershell
go test ./internal/auth ./internal/httpapi -run 'Test(Login|Register|Profile|Limiter)'
```

Expected: FAIL because the user service and routes do not exist.

- [x] **Step 4: Implement user repository and service**

Use `bcrypt.GenerateFromPassword(..., 12)` and constant-time bcrypt comparison. Validate the username before querying. Return typed domain errors that map to 400, 401, 404, 409, or 500; do not expose whether an unknown username or password was wrong.

- [x] **Step 5: Add all four user routes**

Register:

```text
POST /api/users/register -> 201
```

Login:

```text
POST /api/users/login -> 201 {user:{id,username},token}
```

Profile routes return 200 and require JWT. Register/profile responses include only `id`, `username`, optional `avatar`, and the legacy timestamp fields appropriate to each route.

- [x] **Step 6: Run tests and commit the user slice**

Run all Go tests, `go vet ./...`, and a live register/login/profile smoke test. Commit only Task 3 files with message `feat: migrate user authentication to Go`.

### Task 4: Implement the Three-Level RBAC Resolver

**Files:**
- Create: `flowai-studio-control-plane/internal/rbac/permissions.go`
- Create: `flowai-studio-control-plane/internal/rbac/permissions_test.go`
- Create: `flowai-studio-control-plane/internal/rbac/authorizer.go`
- Create: `flowai-studio-control-plane/internal/rbac/authorizer_test.go`
- Create: `flowai-studio-control-plane/internal/store/access_repository.go`
- Create: `flowai-studio-control-plane/internal/store/access_repository_test.go`

- [x] **Step 1: Write failing pure permission-matrix tests**

Freeze all legacy permission strings and test role/grant coverage. In particular, `viewer` can read, `editor` cannot delete applications, `can_edit` cannot delete/publish/share, and `full_access` covers all application/workflow operations.

- [x] **Step 2: Write failing resolution-order tests**

Use fakes to prove this order:

```text
global admin -> application owner -> team role OR team-app grant -> deny
```

Require every requested permission, return not-found separately from forbidden, and ensure one unrelated team grant cannot authorize another application.

- [x] **Step 3: Run tests and verify RED**

Run `go test ./internal/rbac` and expect missing symbols.

- [x] **Step 4: Implement permission types and authorizer**

Use typed constants rather than free-form strings. The authorizer reads only `control` through sqlc-backed queries and has explicit `AuthorizeApplication` and `AuthorizeTeam` methods.

- [x] **Step 5: Run tests and commit the RBAC slice**

Run `go test ./internal/rbac ./...`, `go vet ./...`, and commit Task 4 files with message `feat: add three-level RBAC authorization`.

### Task 5: Implement Application CRUD and Status Routes

**Files:**
- Create: `flowai-studio-control-plane/internal/applications/service.go`
- Create: `flowai-studio-control-plane/internal/applications/service_test.go`
- Create: `flowai-studio-control-plane/internal/store/application_repository.go`
- Create: `flowai-studio-control-plane/internal/store/application_repository_test.go`
- Create: `flowai-studio-control-plane/internal/httpapi/applications.go`
- Create: `flowai-studio-control-plane/internal/httpapi/applications_test.go`
- Modify: `flowai-studio-control-plane/cmd/api/main.go`
- Create: `scripts/native/application-contracts.test.ps1`

- [x] **Step 1: Write failing service tests**

Cover name/description/status validation, owner creation, updated-time ordering, owned plus team-access lists with `accessType`, not-found, `can_view` reads, `can_edit` updates and publish transitions, and `full_access` delete/archive transitions.

- [x] **Step 2: Write failing HTTP compatibility tests**

Freeze the nine routes, exact methods, JWT requirement, POST 201, other success 200, envelope fields, response camelCase names, and 400/401/403/404 mappings.

- [x] **Step 3: Run tests and verify RED**

Run `go test ./internal/applications ./internal/httpapi -run 'TestApplication'` and expect failures for missing implementation.

- [x] **Step 4: Implement the application slice**

Expose:

```text
POST   /api/apps
GET    /api/apps
GET    /api/apps/:id
PATCH  /api/apps/:id
DELETE /api/apps/:id
PATCH  /api/apps/:id/publish
PATCH  /api/apps/:id/unpublish
PATCH  /api/apps/:id/archive
PATCH  /api/apps/:id/unarchive
```

All authorization goes through the RBAC authorizer. SQL list queries deduplicate applications reachable through multiple teams and choose the strongest team grant.

- [x] **Step 5: Run tests and commit the application slice**

Run all Go tests, `go vet ./...`, and live owner/view/edit/delete checks. Commit Task 5 files with message `feat: migrate application APIs to Go`.

### Task 6: Implement Teams, Members, and Team Application Grants

**Files:**
- Create: `flowai-studio-control-plane/internal/teams/service.go`
- Create: `flowai-studio-control-plane/internal/teams/service_test.go`
- Create: `flowai-studio-control-plane/internal/store/team_repository.go`
- Create: `flowai-studio-control-plane/internal/httpapi/teams.go`
- Create: `flowai-studio-control-plane/internal/httpapi/teams_test.go`

- [ ] **Step 1: Write failing team-service tests**

Cover transactional team+owner creation, list counts and `myRole`, member-only detail access, owner/admin management, owner-only delete, duplicate/self member rejection, immutable owner role, owner removal/leave rejection, application-owner-only team sharing, and grant updates/removal scoped to the requested team.

- [ ] **Step 2: Write failing route compatibility tests**

Freeze all 12 team routes, POST 201 semantics, UUID validation, strict role/grant enums, response shapes used by `teamApi.ts`, and error status mapping.

- [ ] **Step 3: Run tests and verify RED**

Run `go test ./internal/teams ./internal/httpapi -run 'TestTeam'` and expect missing behavior.

- [ ] **Step 4: Implement transactions and routes**

Use pgx transactions for team+owner membership creation and any multi-row invariant. Re-check membership/ownership inside the transaction before writes so concurrent requests cannot bypass owner protections.

- [ ] **Step 5: Run tests and commit the team slice**

Run all Go tests, `go vet ./...`, live owner/admin/viewer checks, and commit Task 6 files with message `feat: migrate team and membership APIs to Go`.

### Task 7: Implement HMAC API Keys

**Files:**
- Create: `flowai-studio-control-plane/internal/apikeys/service.go`
- Create: `flowai-studio-control-plane/internal/apikeys/service_test.go`
- Create: `flowai-studio-control-plane/internal/store/api_key_repository.go`
- Create: `flowai-studio-control-plane/internal/httpapi/api_keys.go`
- Create: `flowai-studio-control-plane/internal/httpapi/api_keys_test.go`

- [ ] **Step 1: Write failing cryptographic lifecycle tests**

Assert 32 random bytes, `sk-` plus 64 lowercase hex characters, a seven-character prefix, HMAC-SHA256 rather than plain SHA-256, no plaintext persistence, one-time plaintext response, current/previous-secret validation, constant-time digest comparison, expiry, inactive keys, and owner isolation.

- [ ] **Step 2: Write failing HTTP tests**

Freeze POST/GET/DELETE/PATCH routes, default scopes, optional application filter, strict scope validation, dates, POST 201, and list responses that never contain `key` or `keyHash`.

- [ ] **Step 3: Run tests and verify RED**

Run `go test ./internal/apikeys ./internal/httpapi -run 'TestAPIKey'` and expect failures.

- [ ] **Step 4: Implement API-key service and handlers**

Use:

```go
mac := hmac.New(sha256.New, secret)
_, _ = mac.Write([]byte(rawKey))
digest := mac.Sum(nil)
```

Validate an application-scoped key belongs to the creating user. Persist scopes as `jsonb`. Update `last_used_at` only after successful authentication.

- [ ] **Step 5: Run tests and commit the API-key slice**

Run all Go tests, `go vet ./...`, scan the diff for secrets/plaintext keys, and commit Task 7 files with message `feat: add HMAC API key management`.

### Task 8: Implement Application Sharing and Public Access

**Files:**
- Create: `flowai-studio-control-plane/internal/shares/service.go`
- Create: `flowai-studio-control-plane/internal/shares/service_test.go`
- Create: `flowai-studio-control-plane/internal/store/share_repository.go`
- Create: `flowai-studio-control-plane/internal/httpapi/shares.go`
- Create: `flowai-studio-control-plane/internal/httpapi/shares_test.go`

- [ ] **Step 1: Write failing share-service tests**

Cover owner-only management, idempotent generation, approved GET compatibility behavior, public/disabled lookup, access-count increment, embed config JSON, revoke idempotency, and frontend URL construction.

- [ ] **Step 2: Write failing HTTP compatibility tests**

Freeze:

```text
POST   /api/apps/:appId/share
GET    /api/apps/:appId/share
PATCH  /api/apps/:appId/share
DELETE /api/apps/:appId/share
GET    /api/apps/:appId/embed
GET    /api/share/:shareLink
```

The public route has no JWT; every management route does. Ensure generated HTML is derived only from server-owned base URL, validated share identifiers, and validated theme values.

- [ ] **Step 3: Run tests and verify RED**

Run `go test ./internal/shares ./internal/httpapi -run 'TestShare'` and expect failures.

- [ ] **Step 4: Implement share repository, service, and handlers**

Generate `share-` plus 128 random bits. Store embed config as `jsonb`, validate allowed origins as absolute HTTP(S) origins, and return both `scriptTag` and the frontend-compatible `scriptCode` alias during migration.

- [ ] **Step 5: Run tests and commit the share slice**

Run all Go tests, `go vet ./...`, live public/private share checks, and commit Task 8 files with message `feat: migrate application sharing to Go`.

### Task 9: Wire the Router and Add Live Compatibility Coverage

**Files:**
- Modify: `flowai-studio-control-plane/internal/httpapi/router.go`
- Modify: `flowai-studio-control-plane/internal/httpapi/router_test.go`
- Modify: `flowai-studio-control-plane/cmd/api/main.go`
- Create: `scripts/native/identity-access-contracts.test.ps1`
- Modify: `scripts/native/check-services.ps1`

- [ ] **Step 1: Write failing router inventory test**

Build the Gin engine with fake services and compare registered method/path pairs to the 34 legacy entries plus `GET /api/apps/:appId/share`. Verify protected/public route placement and custom recovery middleware.

- [ ] **Step 2: Write the live native contract test**

The PowerShell test resets only phase-owned rows, creates multiple users, logs in, exercises owner/admin/editor/viewer and team-grant paths, verifies one-time API-key plaintext, tests sharing, and asserts status/envelope fields. It must not read or print local secrets.

- [ ] **Step 3: Run tests and verify RED**

Run the router test and native script; expect missing dependency wiring/routes.

- [ ] **Step 4: Wire repositories, services, handlers, and routes**

Construct one sqlc query set from the pgx pool, one Redis login limiter, one JWT service, one authorizer, and domain services in `main.go`. Register public user/share routes separately from JWT-protected route groups.

- [ ] **Step 5: Run full phase verification**

Run:

```powershell
go test ./...
go vet ./...
node --test scripts/contracts/*.test.cjs
node scripts/contracts/check-contracts.cjs
powershell -NoProfile -ExecutionPolicy Bypass -File scripts/native/database-contracts.test.ps1
powershell -NoProfile -ExecutionPolicy Bypass -File scripts/native/identity-access-contracts.test.ps1
powershell -NoProfile -ExecutionPolicy Bypass -File scripts/native/check-services.ps1
git diff --check
```

Expected: all deterministic tests pass; PostgreSQL, Redis, and AI runtime are healthy; control plane may remain `degraded` only because the WASI guest is not yet ready.

- [ ] **Step 6: Commit wiring and live contract coverage**

Commit Task 9 files with message `test: verify Go identity and access compatibility`.

### Task 10: Phase Acceptance Review

**Files:**
- Modify: `docs/superpowers/plans/2026-07-13-go-identity-access-compatibility-implementation.md`

- [ ] **Step 1: Check off completed plan steps**

Mark only evidence-backed steps complete and add a completion record with exact test counts and live service status.

- [ ] **Step 2: Review security and scope**

Verify no plaintext password/API key/secret enters logs, responses, command lines, git diffs, or database columns; every protected route checks authentication and domain authorization; every query is parameterized; and no user-staged legacy/frontend file entered migration commits.

- [ ] **Step 3: Record the next implementation boundary**

The next plan is `Go workflows, templates, versions, traces, DAG execution, SSE, Redis controls, and cache`. Do not implement those systems inside this phase.
