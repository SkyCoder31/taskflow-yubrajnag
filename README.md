# TaskFlow

## 1. Overview

TaskFlow is a task management REST API built as a backend engineering take-home assignment. It supports user authentication, project management, and task tracking with filtering, pagination, and per-project statistics.

**Tech stack:**

| Layer | Choice |
|---|---|
| Language | Go 1.23 |
| HTTP framework | Gin |
| Database | PostgreSQL 16 |
| Driver | pgx/v5 with pgxpool (no ORM) |
| Migrations | golang-migrate (embedded into binary) |
| Auth | Native JWT — HMAC-SHA256 via `crypto/hmac`|
| Password hashing | bcrypt |
| Logging | `log/slog` (stdlib, structured JSON) |
| Containerisation | Docker (multi-stage build) + Docker Compose |

---

## 2. Architecture Decisions

### Layered architecture: Handler → Service → Repository

Each layer has a single job. Handlers parse HTTP and write responses. Services enforce business rules and authorization. Repositories talk to the database. Domain errors (e.g. `ErrNotFound`, `ErrForbidden`) are defined once and translated to HTTP status codes at the handler boundary — no HTTP concepts leak into services or repositories.

### Design patterns and why they fit here

**Factory Method — `domain.NewUser()`**
A `User` can never exist with a plaintext password. The factory runs validation and bcrypt hashing before returning a value. Constructing a `User{}` struct directly is impossible from outside the package — the invariant is enforced at the type level, not by convention.

**Builder — `domain.NewTaskBuilder()`**
Tasks have several optional fields (assignee, due date) and two fields with meaningful defaults (status=todo, priority=medium). A plain constructor with 7 parameters would be unclear at the call site. The builder makes which fields are being set explicit and returns a `ValidationError` from `Build()` if required fields are missing.

**Builder — `postgres.NewQueryBuilder()`**
List endpoints accept optional filters (status, assignee) and always need a matching COUNT query for pagination. A separate method per filter combination would produce an explosion of functions. The builder accumulates `WHERE` clauses with `WhereIf(condition, clause, arg)` and returns both the data query and the count query from a single `Build()` call, keeping the WHERE conditions in sync.

**Repository with Interface Segregation**
Each aggregate (User, Project, Task) has its interface split into `Reader` and `Writer`. A service that only reads projects depends on `ProjectReader`, not the full `ProjectRepository`. This makes dependencies explicit and keeps test doubles smaller.

**Decorator — `logging.NewUserRepo(postgres.NewUserRepo())`**
Structured logging is added to every repository call without touching the PostgreSQL implementation. The decorator wraps the interface, logs method, duration, and error level (ErrNotFound is DEBUG, everything else is ERROR), then delegates to the real repo. Swapping the underlying store doesn't affect logging.

**Functional Options — `config.WithBcryptCost()`**
Config has sensible defaults for everything except `JWT_SECRET`. Functional options let integration tests override bcrypt cost (4 instead of 12) without exposing a sprawling constructor. The pattern also makes it clear at the call site which default is being overridden and why.

**Chain of Responsibility — Gin middleware**
`AuthMiddleware` either extracts and validates the JWT and calls `c.Next()`, or writes a 401 and calls `c.Abort()`. The chain stops at the first failure. This is the natural fit for Gin's middleware model.

**Strategy (functional) — `handler.mapError()`**
Each domain error maps to a different HTTP status code. A switch with `errors.Is` / `errors.As` keeps all HTTP-to-domain mappings in one place. Adding a new domain error means adding one case, not touching every handler.

### Key technical decisions and tradeoffs

**Native JWT over a library**
HMAC-SHA256 JWT is ~60 lines of `crypto/hmac` + `crypto/sha256` + base64. Using a library (golang-jwt) adds a transitive dependency for something that is fundamentally "hash a JSON payload.The implementation uses `hmac.Equal` for constant-time signature comparison to prevent timing attacks. Tradeoff: no RS256 or JWKS support, which would be needed for multi-service auth.

**CHECK constraints over PostgreSQL ENUMs**
`CHECK (status IN ('todo','in_progress','done'))` is plain DDL — adding or removing a value is a single `ALTER TABLE` that can run inside a transaction. PostgreSQL ENUMs cannot be modified transactionally and require a multi-step migration. Tradeoff: the constraint is not enforced at the Go type level (only at the DB level), so an invalid string reaches Postgres before being rejected.

**Embedded migrations**
SQL files are compiled into the binary via `go:embed`. The server runs `migrations.Up()` before accepting traffic. Tradeoff: the binary is slightly larger and migrations can't be run independently without the binary.

**Manual dependency injection in `main.go`**
All wiring (repos → decorators → services → handlers → router) is explicit in `main.go`. With ~12 dependencies this is readable and debuggable. A DI framework (wire, fx) adds indirection that isn't justified at this scale. Tradeoff: as the graph grows, `main.go` becomes the place everything accumulates.

**Anti-enumeration on login for attackers and regular users**
Both "email not found" and "wrong password" return `401 Unauthorized` with the same message. An attacker cannot determine whether an email is registered. This is a deliberate tradeoff against slightly worse developer experience when debugging.

### What was intentionally left out

- **Refresh tokens** — The JWT has a 24h expiry with no refresh mechanism. A proper implementation needs short-lived access tokens + long-lived refresh tokens stored server-side.
- **Rate limiting** — Login and register are unprotected against brute force. A token-bucket rate limiter per IP would be the first thing added.
- **Role-based access control** — Authorization is binary: you own the project or you don't. Real systems need roles (admin, member, viewer) with a join table.
- **Request ID propagation** — Logs don't include a request ID, making it hard to correlate entries for a single request across middleware and handler logs.

---

## 3. Running Locally

Requires only Docker. No Go, no PostgreSQL, no other tooling.

```bash
git clone https://github.com/yubrajnag/taskflow.git
cd taskflow
cp .env.example .env
docker compose up --build
```

The app is available at **http://localhost:8080**

Verify it's running:

```bash
curl http://localhost:8080/health
# {"data":{"status":"ok"}}
```

What happens on startup:
1. PostgreSQL starts and passes its healthcheck
2. The Go binary starts, connects to Postgres, and runs any pending migrations automatically
3. The HTTP server begins accepting traffic

To stop:

```bash
docker compose down        # stop containers, keep data
docker compose down -v     # stop containers, delete data volume
```

---

## 4. Running Migrations

Migrations run **automatically on startup**. No manual step is required.

SQL migration files are embedded into the Go binary via `go:embed`. On each start, `golang-migrate` applies any pending migrations and exits cleanly if the schema is already up to date. Confirm in the startup logs:

```
{"level":"INFO","msg":"migrations complete"}
```

To roll back all migrations manually (requires Go 1.23 installed):

```bash
cd backend
go run ./cmd/server migrate-down  # not implemented — use psql directly if needed
```

Or drop and recreate via Docker:

```bash
docker compose down -v   # wipes the pgdata volume
docker compose up        # migrations re-run from scratch on next start
```

---

## 5. Test Credentials

Load seed data after first `docker compose up`:

```bash
docker compose exec db psql -U taskflow -d taskflow -f /seed/seed.sql
```

| | |
|---|---|
| **Email** | `test@example.com` |
| **Password** | `password123` |

The seed creates:
- 1 user (above credentials)
- 1 project: "TaskFlow MVP"
- 3 tasks in different statuses: `done`, `in_progress`, `todo`

---

## 6. API Reference

A Postman collection is included in the repo: **[`taskflow.postman_collection.json`](taskflow.postman_collection.json)**

**How to use it:**
1. Open Postman → click **Import** → select `taskflow.postman_collection.json` from the project root
2. Run **Auth → POST /auth/login** (uses the seed credentials by default)
3. The token is saved automatically into the `token` collection variable — all other requests are ready to fire
4. Creating a project saves its ID into `project_id`; creating a task saves into `task_id` — no copy-pasting needed

To change the base URL (e.g. staging), edit the `base_url` collection variable (default: `http://localhost:8080`).

**Base URL:** `http://localhost:8080`

### Response envelope

```json
{ "data": { ... } }
{ "data": [...], "meta": { "page": 1, "limit": 20, "total": 42 } }
{ "error": "not found" }
{ "error": "validation failed", "fields": { "email": "is required" } }
```

### Endpoints

| Method | Path | Auth | Description |
|--------|------|:----:|-------------|
| `GET` | `/health` | — | Liveness check |
| `POST` | `/auth/register` | — | Register → returns JWT |
| `POST` | `/auth/login` | — | Login → returns JWT |
| `POST` | `/projects` | ✓ | Create project |
| `GET` | `/projects` | ✓ | List projects owned by or assigned to the caller (`?page=1&limit=20`) |
| `GET` | `/projects/:id` | ✓ | Get project by ID |
| `PUT` | `/projects/:id` | ✓ owner | Update project name/description |
| `DELETE` | `/projects/:id` | ✓ owner | Delete project |
| `GET` | `/projects/:id/stats` | ✓ | Task counts by status and assignee |
| `POST` | `/projects/:id/tasks` | ✓ | Create task in project |
| `GET` | `/projects/:id/tasks` | ✓ | List tasks (`?status=todo&assignee=<uuid>&page=1&limit=20`) |
| `GET` | `/tasks/:id` | ✓ | Get task by ID |
| `PUT` | `/tasks/:id` | ✓ | Partial update — send only the fields to change |
| `DELETE` | `/tasks/:id` | ✓ owner | Delete task (project owner only) |

**Task fields:** `title` (required), `description`, `status` (`todo` \| `in_progress` \| `done`, default `todo`), `priority` (`low` \| `medium` \| `high`, default `medium`), `assignee_id` (UUID), `due_date` (RFC3339).

### Example: Register and create a project

```bash
# Register
curl -X POST http://localhost:8080/auth/register \
  -H "Content-Type: application/json" \
  -d '{"name":"Jane","email":"jane@example.com","password":"secret123"}'
# → {"data":{"token":"eyJ..."}}

# Create a project (use token from above)
curl -X POST http://localhost:8080/projects \
  -H "Authorization: Bearer eyJ..." \
  -H "Content-Type: application/json" \
  -d '{"name":"My Project","description":"First project"}'
# → {"data":{"id":"...","name":"My Project",...}}
```

---

## 7. What I'd Do With More Time

**Rate limiting on auth endpoints**
Login and register are the most abuse-prone endpoints. A token-bucket rate limiter (per-IP for login, global for register) as Gin middleware would be the first addition. Without it, the anti-enumeration protection on login is weakened by brute-force viability.

**Refresh tokens**
The current JWT has a 24h expiry with no refresh mechanism. The correct approach: short-lived access tokens (15 min) + long-lived refresh tokens stored server-side in a `sessions` table, with `POST /auth/refresh` and `POST /auth/logout`. This also enables server-side token revocation.

**Structured error codes**
The API returns human-readable strings (`"not found"`, `"forbidden"`). A production API should return machine-readable codes (`ERR_NOT_FOUND`, `ERR_FORBIDDEN`) alongside messages so clients can switch on codes rather than parsing strings. This is a one-time schema decision that becomes expensive to change later.

**More integration tests**
The current suite covers auth flows (register, login, duplicate email, wrong password). Missing: project CRUD with ownership enforcement, task filtering and pagination edge cases, the stats endpoint, and expired/tampered JWT rejection. I'd also add a test that verifies the partial update on tasks doesn't overwrite unset fields.

**Service-layer unit tests with mocked repos**
Integration tests cover the full stack but are slow (~100ms each, DB round-trips). Unit tests against the mock interfaces would cover authorization branches (ErrForbidden paths) and validation logic without Docker. The interfaces are already segregated — the mocks would be small.

**Role-based access control**
Authorization is currently binary: own the project or don't. A real system needs a `project_members` join table with roles (admin, member, viewer). Members can create tasks; only admins can delete the project. I left this out because the assignment scope is CRUD, not access management, and adding it would have required a fourth migration and significant service-layer changes.

**Connection pool tuning**
pgxpool uses sensible defaults but I haven't tuned `MaxConns`, `MinConns`, or `MaxConnIdleTime` against expected load. For production, these should be derived from Postgres's `max_connections` and the expected request concurrency.

**Request ID middleware**
Logs don't include a request ID. Under any real traffic, correlating the three or four log lines produced by a single request requires grepping by timestamp — fragile. A UUID injected at the middleware layer and threaded through via `context` would make distributed tracing possible later.
