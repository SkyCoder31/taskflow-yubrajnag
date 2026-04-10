# TaskFlow — Deep Dive Technical Reference

This document explains every file and every non-trivial line in the codebase.
Use it to prepare for a code review conversation.

---

## Table of Contents

1. [Project Layout](#1-project-layout)
2. [Entry Point — cmd/server/main.go](#2-entry-point)
3. [Config — internal/config/config.go](#3-config)
4. [Domain Layer](#4-domain-layer)
   - [errors.go](#41-errorsgo)
   - [user.go](#42-usergo)
   - [project.go](#43-projectgo)
   - [task.go](#44-taskgo)
5. [Auth — internal/auth/jwt.go](#5-auth--jwt)
6. [Repository Interfaces — repository/interfaces.go](#6-repository-interfaces)
7. [PostgreSQL Implementations](#7-postgresql-implementations)
   - [errors.go](#71-errorsgo)
   - [user.go](#72-usergo)
   - [project.go](#73-projectgo)
   - [task.go](#74-taskgo)
   - [query_builder.go](#75-query_buildergo)
8. [Logging Decorators](#8-logging-decorators)
9. [Service Layer](#9-service-layer)
   - [auth.go](#91-authgo)
   - [project.go](#92-projectgo)
   - [task.go](#93-taskgo)
10. [Handler Layer](#10-handler-layer)
    - [response.go](#101-responsego)
    - [middleware.go](#102-middlewarego)
    - [router.go](#103-routergo)
    - [auth.go](#104-authgo)
    - [project.go](#105-projectgo)
    - [task.go](#106-taskgo)
11. [Migrations](#11-migrations)
12. [Docker & Compose](#12-docker--compose)
13. [Integration Tests](#13-integration-tests)
14. [Design Pattern Summary](#14-design-pattern-summary)
15. [Likely Interview Questions](#15-likely-interview-questions)

---

## 1. Project Layout

```
taskflow/
├── Dockerfile                          # Multi-stage build
├── docker-compose.yml                  # Full stack orchestration
├── .env.example                        # All environment variables documented
├── taskflow.postman_collection.json    # Ready-to-import API collection
└── backend/
    ├── go.mod                          # Module: github.com/yubrajnag/taskflow/backend
    ├── cmd/server/main.go              # Entry point — wires everything together
    ├── internal/
    │   ├── config/config.go            # Env-based config + functional options
    │   ├── domain/                     # Pure business types — no frameworks, no DB
    │   │   ├── errors.go              # Sentinel errors + ValidationError
    │   │   ├── user.go                # User factory (bcrypt at creation)
    │   │   ├── project.go             # Project factory
    │   │   └── task.go                # Task builder (fluent API + defaults)
    │   ├── auth/jwt.go                # Native JWT — HMAC-SHA256, no library
    │   ├── repository/
    │   │   ├── interfaces.go          # Segregated interfaces: Reader/Writer splits
    │   │   ├── postgres/              # PostgreSQL implementations
    │   │   │   ├── errors.go          # pgx error → domain error translation
    │   │   │   ├── user.go
    │   │   │   ├── project.go
    │   │   │   ├── task.go
    │   │   │   └── query_builder.go   # Dynamic SQL with positional params
    │   │   └── logging/               # Decorator: adds structured logging
    │   │       ├── user.go
    │   │       ├── project.go
    │   │       └── task.go
    │   ├── service/                   # Business logic + authorization checks
    │   │   ├── auth.go
    │   │   ├── project.go
    │   │   └── task.go
    │   └── handler/                   # HTTP layer — Gin handlers
    │       ├── router.go              # Route registration + request logger
    │       ├── middleware.go          # JWT auth middleware
    │       ├── response.go            # Consistent JSON envelope + error mapping
    │       ├── auth.go
    │       ├── project.go
    │       └── task.go
    ├── migrations/
    │   ├── migrations.go              # go:embed + golang-migrate runner
    │   ├── 001_create_users.up.sql
    │   ├── 001_create_users.down.sql
    │   ├── 002_create_projects.up.sql
    │   ├── 002_create_projects.down.sql
    │   ├── 003_create_tasks.up.sql
    │   └── 003_create_tasks.down.sql
    ├── seed/seed.sql                   # Idempotent test data (ON CONFLICT DO NOTHING)
    ├── scripts/init-test-db.sql        # Creates taskflow_test DB if absent
    └── tests/integration/             # Integration tests against real Postgres
        ├── setup_test.go
        ├── helpers_test.go
        └── auth_test.go
```

**Why `internal/`?**
Go enforces that packages inside `internal/` can only be imported by code rooted at the parent of `internal/`. This prevents any external tool or test from reaching into implementation packages by accident.

---

## 2. Entry Point

**File:** `backend/cmd/server/main.go`

### `main()` → `run()`

```go
func main() {
    if err := run(); err != nil {
        fmt.Fprintf(os.Stderr, "error: %v\n", err)
        os.Exit(1)
    }
}
```

`main` is a one-liner that delegates to `run()`. This matters for testing: `run()` returns an `error`, so startup failures are descriptive strings, not panic stack traces. `os.Exit(1)` is called only in `main` — never inside library code — because `os.Exit` skips deferred functions.

### Config loading

```go
cfg, err := config.Load()
```

`config.Load()` reads all environment variables and returns an error if `JWT_SECRET` is empty. The server cannot start without a secret — there is no insecure default.

### Logger setup

```go
logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
    Level: slog.LevelInfo,
}))
slog.SetDefault(logger)
```

`slog` is the structured logging package added to Go's standard library in 1.21. `NewJSONHandler` writes each log line as a single JSON object to stdout — this is what log aggregators (Datadog, CloudWatch, etc.) expect. `slog.SetDefault` makes this logger the package-level default so any code calling `slog.Info(...)` directly also uses JSON output.

### `connectDB` — retry with linear backoff

```go
for attempt := 1; attempt <= 5; attempt++ {
    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    pool, err = pgxpool.New(ctx, dbCfg.DSN())
    if err == nil {
        err = pool.Ping(ctx)
    }
    cancel()
    if err == nil {
        return pool, nil
    }
    time.Sleep(time.Duration(attempt) * time.Second)
}
```

Docker Compose starts containers in parallel. Even with the `healthcheck` on the `db` service, there's a window between "Postgres is accepting connections" and "Postgres has finished initializing the database." The retry loop (5 attempts, 1s/2s/3s/4s/5s waits) bridges that window. `cancel()` is called in every iteration to avoid context leak — the defer-in-loop anti-pattern would only cancel on function return.

### Dependency injection (manual)

```go
userRepo := logging.NewUserRepo(postgres.NewUserRepo(pool), logger)
projectRepo := logging.NewProjectRepo(postgres.NewProjectRepo(pool), logger)
taskRepo := logging.NewTaskRepo(postgres.NewTaskRepo(pool), logger)

tokenService := auth.NewTokenService(cfg.Auth.JWTSecret, cfg.Auth.JWTExpiry)
authService := service.NewAuthService(userRepo, tokenService, cfg.Auth.BcryptCost)
projectService := service.NewProjectService(projectRepo)
taskService := service.NewTaskService(taskRepo, projectRepo)
```

This is explicit, manual dependency injection — no framework, no reflection. Each dependency is constructed and passed to its consumer. The `logging.NewXxxRepo(postgres.NewXxxRepo(pool), logger)` pattern is the Decorator: the postgres implementation is wrapped in a logger that satisfies the same interface. From the service's perspective, it is calling a `repository.UserRepository` — it has no idea the call is being timed and logged first.

### Graceful shutdown

```go
errCh := make(chan error, 1)
go func() {
    if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
        errCh <- err
    }
    close(errCh)
}()

quit := make(chan os.Signal, 1)
signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

select {
case sig := <-quit:
    ...
case err := <-errCh:
    return fmt.Errorf("server error: %w", err)
}

ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
defer cancel()
srv.Shutdown(ctx)
```

`ListenAndServe` blocks until the server stops. It returns `http.ErrServerClosed` when `Shutdown` is called — that is not an error, so it is filtered out. The `select` waits for either a signal (clean shutdown) or a real server error (immediate exit). `srv.Shutdown` stops accepting new connections, then waits for in-flight requests to complete (up to 10 seconds). The Dockerfile uses exec-form `ENTRYPOINT ["/server"]`, which means the Go binary is PID 1 and receives `SIGTERM` directly from Docker — there is no shell wrapper swallowing the signal.

---

## 3. Config

**File:** `backend/internal/config/config.go`

### Functional Options pattern

```go
type Option func(*Config)

func WithBcryptCost(cost int) Option {
    return func(c *Config) { c.Auth.BcryptCost = cost }
}
```

`Option` is a function type that mutates a `*Config`. `Load(opts ...Option)` builds a config from environment variables and then applies each option. This lets callers override specific defaults:

```go
// In integration tests:
auth.NewAuthService(userRepo, tokenService, 4) // cost 4, not 12
```

The alternative — a struct with every field as a constructor parameter — breaks every caller when a new field is added. The alternative — a big options struct — requires nil-checking every field. Functional options avoid both problems.

### Required secret enforcement

```go
if cfg.Auth.JWTSecret == "" {
    return nil, fmt.Errorf("JWT_SECRET environment variable is required")
}
```

The empty string is the zero value for `string`, and `envStr("JWT_SECRET", "")` returns `""` if the variable is unset. This guard means there is no scenario where the server runs with an empty signing key. It fails at startup, not at the first login request.

### `DSN()` method

```go
func (d DatabaseConfig) DSN() string {
    return fmt.Sprintf("postgres://%s:%s@%s:%d/%s?sslmode=%s",
        d.User, d.Password, d.Host, d.Port, d.Name, d.SSLMode)
}
```

The DSN is assembled from individual fields rather than stored as a single string. This keeps the config struct readable and lets the `DB_PASSWORD` be changed without reformatting a connection string.

---

## 4. Domain Layer

The domain layer contains pure Go — no Gin, no pgx, no JWT. It can be tested without any infrastructure.

### 4.1 errors.go

```go
var (
    ErrNotFound      = errors.New("not found")
    ErrAlreadyExists = errors.New("already exists")
    ErrUnauthorized  = errors.New("unauthorized")
    ErrForbidden     = errors.New("forbidden")
)
```

**Sentinel errors** are package-level variables that can be compared with `errors.Is`. They are defined once in the domain layer and used at every layer above. The handler translates them to HTTP status codes with `mapError()`. The advantage over string comparisons: you cannot misspell a sentinel error and have it silently fail — the compiler will catch an unknown identifier.

```go
type ValidationError struct {
    Fields map[string]string
}

func (e *ValidationError) Error() string { return "validation failed" }
func (e *ValidationError) Add(field, message string) { e.Fields[field] = message }
func (e *ValidationError) HasErrors() bool { return len(e.Fields) > 0 }
```

`ValidationError` is a typed error carrying a map of field → message. Because it implements the `error` interface, it can be returned from any function that returns `error`. In the handler, `errors.As(err, &ve)` extracts it and serializes the `Fields` map as JSON, giving the API consumer a structured validation response like `{"error":"validation failed","fields":{"email":"is required"}}`.

### 4.2 user.go — Factory Method

```go
func NewUser(name, email, plainPassword string, bcryptCost int) (*User, error) {
    ve := NewValidationError()

    name = strings.TrimSpace(name)
    if name == "" { ve.Add("name", "is required") }

    email = strings.TrimSpace(strings.ToLower(email))
    if email == "" { ve.Add("email", "is required") }
    else if _, err := mail.ParseAddress(email); err != nil {
        ve.Add("email", "is not a valid email address")
    }

    if plainPassword == "" { ve.Add("password", "is required") }
    else if len(plainPassword) < 8 { ve.Add("password", "must be at least 8 characters") }

    if ve.HasErrors() { return nil, ve }

    hashed, err := bcrypt.GenerateFromPassword([]byte(plainPassword), bcryptCost)
    ...
    return &User{ID: uuid.New(), ..., Password: string(hashed)}, nil
}
```

This is the **Factory Method** pattern. Key decisions:

- Validation is collected into `ve` rather than returning on the first error. The API consumer gets all problems at once, not one at a time.
- `bcrypt.GenerateFromPassword` is called *inside* the factory. A `User` struct can never be constructed with a plaintext password from outside this package — the invariant is structural, not documentary.
- `email` is normalized to lowercase before storage. `strings.ToLower("A@B.com")` → `"a@b.com"`. Without this, the same email could register twice as "User@example.com" and "user@example.com".
- `mail.ParseAddress` is used for email validation rather than a regex. It validates against RFC 5322, which handles edge cases (quoted strings, comments) that simple regexes miss.
- `bcryptCost` is a parameter so tests can pass `4` (the minimum) instead of `12`. bcrypt at cost 12 takes ~250ms; at cost 4 it takes ~1ms. With dozens of tests, this difference matters.

```go
func (u *User) CheckPassword(plainPassword string) bool {
    return bcrypt.CompareHashAndPassword([]byte(u.Password), []byte(plainPassword)) == nil
}
```

`bcrypt.CompareHashAndPassword` extracts the salt and cost from the stored hash and re-hashes the input. It is designed to be constant-time (it always runs the full bcrypt rounds regardless of where a mismatch occurs) to prevent timing attacks on password comparison.

### 4.3 project.go

```go
func NewProject(name, description string, ownerID uuid.UUID) (*Project, error) {
    ...
    if ownerID == uuid.Nil {
        ve.Add("owner_id", "is required")
    }
    ...
    return &Project{ID: uuid.New(), Name: name, Description: strings.TrimSpace(description), OwnerID: ownerID, CreatedAt: time.Now().UTC()}, nil
}
```

`uuid.Nil` is the zero value for `uuid.UUID` (`00000000-0000-0000-0000-000000000000`). Checking for it guards against a programming error where `GetUserID()` returns a zero value due to a missing middleware. `time.Now().UTC()` stores timestamps in UTC — without `.UTC()`, the stored time would be in the local timezone of the machine running the binary, which would be different in Docker vs. a developer's laptop.

### 4.4 task.go — Builder Pattern

```go
type TaskBuilder struct {
    title       string
    description string
    status      TaskStatus     // default: StatusTodo
    priority    TaskPriority   // default: PriorityMedium
    projectID   uuid.UUID
    assigneeID  *uuid.UUID     // optional
    dueDate     *time.Time     // optional
}

func NewTaskBuilder() *TaskBuilder {
    return &TaskBuilder{status: StatusTodo, priority: PriorityMedium}
}
```

The **Builder** pattern is used here because:
1. Tasks have two optional fields (`assigneeID`, `dueDate`) — pointer types because `nil` means "not set", distinct from "zero value".
2. Two fields have meaningful defaults (`status=todo`, `priority=medium`) — callers should not have to specify these if they want the defaults.
3. A function `NewTask(title, desc, status, priority, projectID string, assigneeID *uuid.UUID, dueDate *time.Time)` with 7 parameters would be unreadable at the call site.

```go
func (b *TaskBuilder) AssigneeID(id uuid.UUID) *TaskBuilder {
    b.assigneeID = &id   // takes the address of the parameter copy
    return b
}
```

Each setter returns `*TaskBuilder` for method chaining. `&id` takes the address of the local copy of `id`, which is safe because Go escape analysis will heap-allocate it.

```go
func (b *TaskBuilder) Build() (*Task, error) {
    ve := NewValidationError()
    title := strings.TrimSpace(b.title)
    if title == "" { ve.Add("title", "is required") }
    if b.projectID == uuid.Nil { ve.Add("project_id", "is required") }
    if !b.status.IsValid() { ve.Add("status", "must be one of: todo, in_progress, done") }
    if !b.priority.IsValid() { ve.Add("priority", "must be one of: low, medium, high") }
    if ve.HasErrors() { return nil, ve }
    ...
}
```

`Build()` is the only place where a `Task` value is produced. The `IsValid()` methods on `TaskStatus` and `TaskPriority` are switch statements that enumerate the valid constants — they catch cases where the handler passed a raw string that wasn't one of the known values.

---

## 5. Auth — JWT

**File:** `backend/internal/auth/jwt.go`

### Why no JWT library?

A JWT is `base64url(header) + "." + base64url(payload) + "." + base64url(HMAC(header.payload))`. The entire implementation requires `crypto/hmac`, `crypto/sha256`, `encoding/base64`, and `encoding/json` — all in the standard library. Adding `golang-jwt/jwt` would be a dependency for ~60 lines of code.

### Pre-encoded header

```go
var headerEncoded = base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"HS256","typ":"JWT"}`))
```

The JWT header is always the same (`{"alg":"HS256","typ":"JWT"}`), so it is computed once at package initialization rather than on every `Generate` call. `RawURLEncoding` is base64url without padding (`=`) — the JWT specification requires this.

### `Generate`

```go
func (t *TokenService) Generate(userID uuid.UUID, email string) (string, error) {
    now := time.Now().UTC()
    claims := Claims{
        UserID: userID,
        Email:  email,
        Exp:    now.Add(t.expiry).Unix(),
        Iat:    now.Unix(),
    }
    payloadJSON, _ := json.Marshal(claims)
    payloadEncoded := base64.RawURLEncoding.EncodeToString(payloadJSON)

    signingInput := headerEncoded + "." + payloadEncoded
    signature := t.sign([]byte(signingInput))
    signatureEncoded := base64.RawURLEncoding.EncodeToString(signature)

    return signingInput + "." + signatureEncoded, nil
}
```

`Exp` is Unix timestamp (seconds since epoch) — the JWT standard field for expiry. `Iat` ("issued at") records when the token was created. The `signingInput` is `header.payload` — exactly what the JWT spec says to sign.

### `Verify`

```go
func (t *TokenService) Verify(tokenStr string) (*Claims, error) {
    parts := strings.Split(tokenStr, ".")
    if len(parts) != 3 { return nil, ErrInvalidToken }

    signingInput := parts[0] + "." + parts[1]
    expectedSig := t.sign([]byte(signingInput))

    actualSig, err := base64.RawURLEncoding.DecodeString(parts[2])
    if err != nil { return nil, ErrInvalidToken }

    // Constant-time comparison to prevent timing attacks
    if !hmac.Equal(expectedSig, actualSig) { return nil, ErrInvalidToken }

    ...
    if time.Now().UTC().Unix() > claims.Exp { return nil, ErrExpiredToken }

    return &claims, nil
}
```

**Why `hmac.Equal` instead of `bytes.Equal`?**
`bytes.Equal` short-circuits on the first mismatched byte. An attacker who can measure response time could brute-force a valid signature one byte at a time by observing that tokens with the first N correct bytes take slightly longer to reject. `hmac.Equal` always compares all bytes regardless of where the mismatch is — it is constant-time with respect to the content.

**Why check expiry after the signature?** If the signature is invalid, the claims are untrusted and should not be read. Checking expiry first would mean parsing and trusting the payload of a forged token.

### `sign`

```go
func (t *TokenService) sign(message []byte) []byte {
    mac := hmac.New(sha256.New, t.secret)
    mac.Write(message)
    return mac.Sum(nil)
}
```

`hmac.New` creates an HMAC instance keyed with `t.secret`. `mac.Write` feeds the message. `mac.Sum(nil)` returns the raw HMAC-SHA256 bytes (32 bytes). `nil` means "append to a new slice" rather than an existing one.

---

## 6. Repository Interfaces

**File:** `backend/internal/repository/interfaces.go`

### Interface Segregation

```go
type UserReader interface {
    GetByID(ctx context.Context, id uuid.UUID) (*domain.User, error)
    GetByEmail(ctx context.Context, email string) (*domain.User, error)
}

type UserWriter interface {
    Create(ctx context.Context, user *domain.User) error
}

type UserRepository interface {
    UserReader
    UserWriter
}
```

Each aggregate is split into `Reader` and `Writer`. A service that only needs to look up users depends on `UserReader` — it cannot accidentally call `Create`. This is the **Interface Segregation Principle**: consumers depend on the smallest interface they actually need.

In practice, all three services receive the full `UserRepository` today — but the split pays off in tests, where a mock for a read-only service only needs to implement two methods instead of three.

### `TaskFilter`

```go
type TaskFilter struct {
    Status   domain.TaskStatus
    Assignee uuid.UUID
    Page     int
    Limit    int
}
```

Zero values mean "no filter". `domain.TaskStatus("")` is an empty string — the query builder's `WhereIf(filter.Status != "", ...)` treats it as "no status filter". `uuid.Nil` (all zeros) means "no assignee filter". This avoids the alternative of using pointers (`*domain.TaskStatus`) which would require nil-checking at every call site.

### `TaskStatsResult`

```go
type TaskStatsResult struct {
    ByStatus   map[domain.TaskStatus]int `json:"by_status"`
    ByAssignee map[uuid.UUID]int         `json:"by_assignee"`
}
```

The `json:` tags are required because without them, `ByStatus` would serialize as `"ByStatus"` (PascalCase) instead of `"by_status"` (snake_case). The API contract uses snake_case. The `TaskStatsResult` is defined in the repository package rather than the domain package because it is a query result shape — it exists to serve the API, not to model a business entity.

---

## 7. PostgreSQL Implementations

### 7.1 errors.go

```go
const uniqueViolation = "23505"

func isDuplicateKeyError(err error) bool {
    var pgErr *pgconn.PgError
    if errors.As(err, &pgErr) {
        return pgErr.Code == uniqueViolation
    }
    return false
}
```

`"23505"` is the PostgreSQL error code for `unique_violation`. It is a constant from the SQL standard that does not change across Postgres versions. `errors.As` unwraps the error chain to find a `*pgconn.PgError` — pgx wraps errors, so a direct type assertion would fail. When this returns `true`, the repo translates the error to `domain.ErrAlreadyExists` — the service and handler never see a pgx type.

### 7.2 user.go

```go
var _ repository.UserRepository = (*userRepo)(nil)
```

This is a **compile-time interface check**. It assigns the `nil` pointer of type `*userRepo` to a variable of type `repository.UserRepository`. If `*userRepo` does not implement all methods of `UserRepository`, the file will not compile. This catches missing methods at compile time instead of at runtime when the server starts.

```go
func NewUserRepo(pool *pgxpool.Pool) repository.UserRepository {
    return &userRepo{pool: pool}
}
```

The constructor returns the **interface**, not the concrete type. Callers see `repository.UserRepository` — they cannot access any method that is not part of the interface, even accidentally.

```go
func scanUser(row pgx.Row) (*domain.User, error) {
    var u domain.User
    err := row.Scan(&u.ID, &u.Name, &u.Email, &u.Password, &u.CreatedAt)
    if err != nil {
        if errors.Is(err, pgx.ErrNoRows) {
            return nil, domain.ErrNotFound
        }
        return nil, err
    }
    return &u, nil
}
```

`scanUser` is a shared helper used by both `GetByID` and `GetByEmail` — they have different queries but identical scan logic. `pgx.ErrNoRows` is translated to `domain.ErrNotFound` here, at the boundary between pgx and domain. The service layer never imports pgx.

### 7.3 project.go

```go
func (r *projectRepo) Update(ctx context.Context, project *domain.Project) error {
    tag, err := r.pool.Exec(ctx, `UPDATE projects SET name = $1, description = $2 WHERE id = $3`, ...)
    if err != nil { return err }
    if tag.RowsAffected() == 0 { return domain.ErrNotFound }
    return nil
}
```

`RowsAffected() == 0` means the `WHERE id = $3` matched no rows — the project was deleted between the service's `GetByID` check and the `Update` call (TOCTOU race). Returning `ErrNotFound` here handles this edge case cleanly.

```go
func (r *projectRepo) ListByUser(...) {
    ...
    SELECT DISTINCT p.id, p.name, p.description, p.owner_id, p.created_at
    FROM projects p
    LEFT JOIN tasks t ON t.project_id = p.id AND t.assignee_id = $1
    WHERE p.owner_id = $1 OR t.assignee_id = $1
    ORDER BY p.created_at DESC
    LIMIT $2 OFFSET $3
```

This query returns projects that the user either **owns** or **has tasks assigned to**. The `LEFT JOIN` with `t.assignee_id = $1` in the join condition (not the WHERE clause) ensures that projects with no matching tasks still appear — they just have `NULL` for the task columns. `DISTINCT` is required because a project with multiple tasks assigned to the user would otherwise appear multiple times. The same `$1` parameter appears twice — pgx handles this correctly.

### 7.4 task.go

```go
func (r *taskRepo) ListByProject(...) {
    qb := NewQueryBuilder(
        "SELECT id, title, ... FROM tasks",
        "SELECT COUNT(*) FROM tasks",
    ).
        Where("project_id = %s", projectID).
        WhereIf(filter.Status != "", "status = %s", string(filter.Status)).
        WhereIf(filter.Assignee != uuid.Nil, "assignee_id = %s", filter.Assignee).
        OrderBy("created_at DESC").
        Paginate(page, limit)

    query, queryArgs, countQuery, countArgs := qb.Build()
```

The builder produces both the data query and the count query from the same set of conditions. If you wrote them separately, any time you added a filter you'd have to update two places — and they'd inevitably drift.

```go
func (r *taskRepo) Stats(ctx context.Context, projectID uuid.UUID) (*repository.TaskStatsResult, error) {
    result := &repository.TaskStatsResult{
        ByStatus:   make(map[domain.TaskStatus]int),
        ByAssignee: make(map[uuid.UUID]int),
    }

    rows, err := r.pool.Query(ctx,
        `SELECT status, COUNT(*) FROM tasks WHERE project_id = $1 GROUP BY status`, projectID)
    ...
    for rows.Next() {
        var status domain.TaskStatus
        var count int
        rows.Scan(&status, &count)
        result.ByStatus[status] = count
    }
```

Two separate queries: one groups by status, one groups by assignee (filtering out NULLs). They could be combined into one query with conditional aggregation, but two simple queries are easier to read and debug. The maps are pre-initialized with `make` — without this, a nil map would panic on write.

### 7.5 query_builder.go

```go
func (qb *QueryBuilder) Where(clause string, arg any) *QueryBuilder {
    parameterized := strings.Replace(clause, "%s", fmt.Sprintf("$%d", qb.paramIndex), 1)
    qb.conditions = append(qb.conditions, parameterized)
    qb.args = append(qb.args, arg)
    qb.paramIndex++
    return qb
}
```

`clause` is written with `%s` as a placeholder (e.g., `"status = %s"`). `Where` replaces it with `$N` where N is the next parameter index. This is because PostgreSQL uses positional parameters (`$1`, `$2`, ...) rather than `?`. The builder tracks the index so each call increments it correctly — `$1`, `$2`, `$3`, etc.

```go
func (qb *QueryBuilder) Build() (query string, queryArgs []any, countQuery string, countArgs []any) {
    // Count query: base + WHERE (no ORDER BY, no LIMIT)
    countQuery = countBase + whereClause
    countArgs = make([]any, len(qb.args))
    copy(countArgs, qb.args)  // defensive copy

    // Data query: base + WHERE + ORDER BY + LIMIT/OFFSET
    ...
    if qb.limit > 0 {
        offset := (qb.page - 1) * qb.limit
        sb.WriteString(fmt.Sprintf(" LIMIT $%d OFFSET $%d", qb.paramIndex, qb.paramIndex+1))
        queryArgs = append(queryArgs, qb.limit, offset)
    }
```

`copy(countArgs, qb.args)` is a defensive copy — if the same `qb.args` slice were shared between count and data queries, appending LIMIT/OFFSET args to one would affect the other. The count query never gets `LIMIT` or `OFFSET` — it counts all matching rows, not just the current page.

---

## 8. Logging Decorators

**Files:** `backend/internal/repository/logging/`

```go
type userRepoLogger struct {
    next   repository.UserRepository
    logger *slog.Logger
}

func NewUserRepo(next repository.UserRepository, logger *slog.Logger) repository.UserRepository {
    return &userRepoLogger{next: next, logger: logger.With("component", "repository.user")}
}
```

This is the **Decorator** pattern. `userRepoLogger` holds a reference to the real `repository.UserRepository` (`next`) and satisfies the same interface. `logger.With("component", "repository.user")` creates a child logger that includes `"component": "repository.user"` in every log line it produces — without having to specify it in each method.

```go
func (r *userRepoLogger) Create(ctx context.Context, user *domain.User) error {
    start := time.Now()
    err := r.next.Create(ctx, user)
    r.logger.LogAttrs(ctx, logLevel(err), "Create",
        slog.String("user_id", user.ID.String()),
        slog.Duration("duration", time.Since(start)),
        errAttr(err),
    )
    return err
}
```

The pattern is the same in every method: record the start time, call the real implementation, log the result with duration, return the original error unchanged. The decorator is transparent — it does not alter the behavior, only observes it.

```go
func logLevel(err error) slog.Level {
    if err != nil && err != domain.ErrNotFound {
        return slog.LevelError
    }
    return slog.LevelInfo
}
```

`ErrNotFound` is a normal miss (e.g., "check if this email exists before inserting"). Logging it as ERROR would create noisy alerts for routine cache-miss-like operations. Everything else (connection errors, constraint violations, unexpected errors) is ERROR.

```go
func errAttr(err error) slog.Attr {
    if err != nil {
        return slog.String("error", err.Error())
    }
    return slog.Attr{}  // empty attribute — slog omits it
}
```

`slog.Attr{}` (zero value) is silently omitted by slog. This avoids a `"error": null` field appearing in every success log line.

---

## 9. Service Layer

### 9.1 auth.go

```go
func (s *AuthService) Login(ctx context.Context, email, password string) (string, error) {
    user, err := s.users.GetByEmail(ctx, email)
    if err != nil {
        if err == domain.ErrNotFound {
            return "", domain.ErrUnauthorized
        }
        return "", err
    }
    if !user.CheckPassword(password) {
        return "", domain.ErrUnauthorized
    }
    return s.tokens.Generate(user.ID, user.Email)
}
```

Both "email not found" and "wrong password" return `domain.ErrUnauthorized` — the same error, the same message. This prevents **user enumeration**: an attacker cannot determine whether an email is registered by comparing error messages. The handler translates `ErrUnauthorized` to `401 Unauthorized` with `{"error":"unauthorized"}` in both cases.

### 9.2 project.go

```go
func (s *ProjectService) Update(ctx context.Context, id uuid.UUID, name, description string, callerID uuid.UUID) (*domain.Project, error) {
    project, err := s.projects.GetByID(ctx, id)
    if err != nil { return nil, err }

    if project.OwnerID != callerID {
        return nil, domain.ErrForbidden
    }

    if name != "" { project.Name = name }
    if description != "" { project.Description = description }

    if err := s.projects.Update(ctx, project); err != nil { return nil, err }
    return project, nil
}
```

Authorization is checked **in the service layer**, not the handler. This means the check runs regardless of which HTTP verb, which route group, or which middleware was used. The handler is responsible for parsing HTTP — the service is responsible for enforcing who can do what.

The `if name != ""` guards implement partial updates: if the caller sends `{"description":"new desc"}` without a `name` field, the handler passes an empty string for `name`, and the service leaves the existing name unchanged.

### 9.3 task.go

```go
func (s *TaskService) Delete(ctx context.Context, id uuid.UUID, callerID uuid.UUID) error {
    task, err := s.tasks.GetByID(ctx, id)
    if err != nil { return err }

    project, err := s.projects.GetByID(ctx, task.ProjectID)
    if err != nil { return err }

    if project.OwnerID != callerID {
        return domain.ErrForbidden
    }

    return s.tasks.Delete(ctx, id)
}
```

Task deletion requires fetching both the task (to get `ProjectID`) and the project (to check `OwnerID`). Only the project owner can delete tasks — not just the task's assignee. This is why `TaskService` depends on *both* `TaskRepository` and `ProjectRepository`.

```go
func (s *TaskService) Update(..., title, description *string, status *domain.TaskStatus, ...) (*domain.Task, error) {
    task, err := s.tasks.GetByID(ctx, id)
    ...
    if title != nil { task.Title = *title }
    if description != nil { task.Description = *description }
    if status != nil {
        if !status.IsValid() {
            ve := domain.NewValidationError()
            ve.Add("status", "must be one of: todo, in_progress, done")
            return nil, ve
        }
        task.Status = *status
    }
```

Pointer parameters enable partial updates. A `nil` pointer means "caller did not provide this field — leave it unchanged." A non-nil pointer means "caller wants to change this field to this value." This is why `updateTaskRequest` in the handler uses `*string` for all fields.

---

## 10. Handler Layer

### 10.1 response.go

```go
func Error(c *gin.Context, err error) {
    var ve *domain.ValidationError
    if errors.As(err, &ve) {
        c.JSON(http.StatusBadRequest, gin.H{
            "error":  "validation failed",
            "fields": ve.Fields,
        })
        return
    }
    status := mapError(err)
    c.JSON(status, gin.H{"error": err.Error()})
}
```

`errors.As(err, &ve)` checks if `err` or any error in its chain is a `*domain.ValidationError`. This is important because errors can be wrapped (`fmt.Errorf("creating user: %w", ve)`) — `errors.As` unwraps the chain. If it is a `ValidationError`, the response includes the field map. Otherwise, `mapError` looks up the HTTP status code.

```go
func mapError(err error) int {
    switch {
    case errors.Is(err, domain.ErrNotFound):      return http.StatusNotFound
    case errors.Is(err, domain.ErrAlreadyExists): return http.StatusConflict
    case errors.Is(err, domain.ErrUnauthorized):  return http.StatusUnauthorized
    case errors.Is(err, domain.ErrForbidden):     return http.StatusForbidden
    default:                                       return http.StatusInternalServerError
    }
}
```

This is the **Strategy** pattern — each domain error maps to a different HTTP status via a dispatch table. `errors.Is` checks equality and also unwraps chains. The `default` case returning 500 means unexpected errors (DB connection drops, etc.) produce a 500 without leaking internal details.

### 10.2 middleware.go

```go
func AuthMiddleware(tokens *auth.TokenService) gin.HandlerFunc {
    return func(c *gin.Context) {
        header := c.GetHeader("Authorization")
        if header == "" {
            c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "missing authorization header"})
            return
        }
        parts := strings.SplitN(header, " ", 2)
        if len(parts) != 2 || parts[0] != "Bearer" {
            c.AbortWithStatusJSON(...)
            return
        }
        claims, err := tokens.Verify(parts[1])
        if err != nil {
            c.AbortWithStatusJSON(...)
            return
        }
        c.Set(claimsKey, claims)
        c.Next()
    }
}
```

`strings.SplitN(header, " ", 2)` splits into at most 2 parts — if the token itself contains spaces (it shouldn't, but malformed headers do), the token is still captured whole as `parts[1]`. `c.AbortWithStatusJSON` writes the response and stops the middleware chain — handlers registered after this middleware will not be called. `c.Set(claimsKey, claims)` stores the verified claims in the request context. `c.Next()` hands control to the next handler in the chain. This is **Chain of Responsibility**.

```go
func GetUserID(c *gin.Context) uuid.UUID {
    return getClaims(c).UserID
}

func getClaims(c *gin.Context) *auth.Claims {
    v, exists := c.Get(claimsKey)
    if !exists {
        panic("getClaims called without AuthMiddleware")
    }
    return v.(*auth.Claims)
}
```

`getClaims` panics if called on an unprotected route. This is intentional — it is a **programmer error**, not a user error. If a handler calls `GetUserID` but is registered without `AuthMiddleware`, that is a bug in the routing code that should be caught immediately (and loudly) during development, not silently return a zero UUID in production.

### 10.3 router.go

```go
gin.SetMode(gin.ReleaseMode)
r := gin.New()
r.Use(requestLogger(logger))
r.Use(gin.Recovery())
```

`gin.SetMode(gin.ReleaseMode)` suppresses Gin's debug-mode console output (which prints every registered route and is noisy in production). `gin.New()` creates a router with no middleware — unlike `gin.Default()` which includes its own logger and recovery. We add our own structured logger instead of Gin's default colored console logger.

`gin.Recovery()` catches panics in handlers, writes a 500 response, and logs the panic. Without it, a panic would crash the entire server process.

```go
protected := r.Group("/", AuthMiddleware(tokens))
projects := protected.Group("/projects")
```

The `AuthMiddleware` is applied to the entire `protected` group. Every route registered under `protected` requires a valid JWT. Routes under `authGroup` do not — they are public. This is more maintainable than adding the middleware to each route individually.

```go
func requestLogger(logger *slog.Logger) gin.HandlerFunc {
    return func(c *gin.Context) {
        start := time.Now()
        c.Next()  // run the actual handler
        logger.LogAttrs(c.Request.Context(), slog.LevelInfo, "HTTP",
            slog.String("method", c.Request.Method),
            slog.String("path", c.Request.URL.Path),
            slog.Int("status", c.Writer.Status()),
            slog.Duration("duration", time.Since(start)),
            slog.String("client_ip", c.ClientIP()),
        )
    }
}
```

The logger calls `c.Next()` first, then logs. This is **post-middleware** pattern — the log line is written after the handler returns, so `c.Writer.Status()` has the final status code (set by the handler) not the default 200.

### 10.4 auth.go

```go
func (h *AuthHandler) Register(c *gin.Context) {
    var req registerRequest
    if err := c.ShouldBindJSON(&req); err != nil {
        BadRequest(c, err)
        return
    }
    token, err := h.auth.Register(c.Request.Context(), req.Name, req.Email, req.Password)
    if err != nil {
        Error(c, err)
        return
    }
    Created(c, authResponse{Token: token})
}
```

`ShouldBindJSON` returns an error if the body is not valid JSON (malformed, EOF, wrong Content-Type). This error is a Gin/stdlib error, not a domain error — it goes through `BadRequest()` (400) directly, not through `Error()` which would hit `mapError` and return 500 for an unknown error type. `c.Request.Context()` passes the HTTP request's context to the service — this context carries the request's deadline and cancellation. If the client disconnects, the context is cancelled, and any in-progress DB query using that context will be cancelled too.

### 10.5 project.go

```go
type projectResponse struct {
    ID          string `json:"id"`
    Name        string `json:"name"`
    Description string `json:"description"`
    OwnerID     string `json:"owner_id"`
    CreatedAt   string `json:"created_at"`
}

func toProjectResponse(p *domain.Project) projectResponse {
    return projectResponse{
        ID:        p.ID.String(),
        ...
        CreatedAt: p.CreatedAt.Format(time.RFC3339),
    }
}
```

`toProjectResponse` is a DTO (Data Transfer Object) converter. It converts the domain `Project` (which uses `uuid.UUID` and `time.Time`) into a response struct with plain strings. `uuid.UUID.String()` produces the standard `8-4-4-4-12` UUID format. `time.RFC3339` formats as `2006-01-02T15:04:05Z07:00` — the standard JSON date format. Why not serialize `uuid.UUID` directly? The `json` package would serialize it as an array of 16 bytes unless a custom `MarshalJSON` method is defined. Converting to `string` at the boundary is explicit and predictable.

### 10.6 task.go

```go
type updateTaskRequest struct {
    Title       *string `json:"title"`
    Description *string `json:"description"`
    Status      *string `json:"status"`
    ...
}
```

All fields are pointers (`*string`). When `ShouldBindJSON` parses `{"status":"done"}`, `req.Status` will be a non-nil pointer to `"done"`, and `req.Title` will be `nil`. The service checks `if title != nil` — only non-nil fields are updated. If they were plain `string`, there would be no way to distinguish "caller sent empty string" from "caller did not include the field".

```go
var assigneeID *uuid.UUID
if req.AssigneeID != nil {
    parsed, err := uuid.Parse(*req.AssigneeID)
    if err != nil {
        ve := domain.NewValidationError()
        ve.Add("assignee_id", "must be a valid UUID")
        Error(c, ve)
        return
    }
    assigneeID = &parsed
}
```

UUID parsing happens in the handler, before calling the service. If the UUID string is malformed, it is caught here with a structured validation error. The service receives either `nil` (no change) or a valid `*uuid.UUID` — it never receives an invalid UUID string.

---

## 11. Migrations

**File:** `backend/migrations/migrations.go`

```go
//go:embed *.sql
var fs embed.FS
```

`//go:embed *.sql` is a compiler directive that reads all `.sql` files from the `migrations/` directory at compile time and embeds them into the binary as a virtual filesystem. The binary is self-contained — it does not need the SQL files on disk at runtime. `fs` is unexported (lowercase) — only `Run` is accessible from outside.

```go
func Run(dsn string) error {
    source, err := iofs.New(fs, ".")
    m, err := migrate.NewWithSourceInstance("iofs", source, dsn)
    defer m.Close()

    if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
        return err
    }
    return nil
}
```

`iofs.New` adapts the embedded filesystem to the `golang-migrate` source interface. `m.Up()` applies all pending migrations in order. `migrate.ErrNoChange` is returned when the schema is already up to date — this is not an error, it is the normal case on every restart after the first. The `migrate` table in Postgres tracks which migrations have been applied.

### SQL Migrations

**001_create_users.up.sql:**
```sql
CREATE TABLE IF NOT EXISTS users (
    id         UUID PRIMARY KEY,
    email      VARCHAR(255) NOT NULL,
    password   VARCHAR(255) NOT NULL,
    created_at TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    CONSTRAINT users_email_unique UNIQUE (email)
);
```

`UUID PRIMARY KEY` — UUIDs are generated by the application (not the DB) so there is no `SERIAL` or `GENERATED ALWAYS AS IDENTITY`. This means IDs are known before insertion, which simplifies returning the created entity without a second query. `TIMESTAMPTZ` ("timestamp with time zone") stores timestamps as UTC internally and converts to the session timezone on display — always use this over `TIMESTAMP` (which is timezone-naive and causes bugs when servers move between regions).

**002_create_projects.up.sql:**
```sql
owner_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE
CREATE INDEX idx_projects_owner_id ON projects(owner_id);
```

`ON DELETE CASCADE` — deleting a user deletes all their projects. The index on `owner_id` makes the `WHERE owner_id = $1` query in `ListByUser` efficient (index scan instead of sequential scan).

**003_create_tasks.up.sql:**
```sql
CONSTRAINT tasks_status_check CHECK (status IN ('todo', 'in_progress', 'done')),
CONSTRAINT tasks_priority_check CHECK (priority IN ('low', 'medium', 'high'))

CREATE INDEX idx_tasks_project_status ON tasks(project_id, status);
CREATE INDEX idx_tasks_assignee_id ON tasks(assignee_id);

CREATE TRIGGER tasks_updated_at
    BEFORE UPDATE ON tasks
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();
```

**CHECK constraints vs ENUMs:** `CHECK` constraints are plain DDL — adding a new status value is `ALTER TABLE tasks DROP CONSTRAINT tasks_status_check, ADD CONSTRAINT tasks_status_check CHECK (status IN ('todo', 'in_progress', 'done', 'blocked'))`, which runs in a transaction. PostgreSQL ENUMs require `ALTER TYPE` which cannot be done inside a transaction and involves multiple steps.

**Composite index `(project_id, status)`:** The most common query is "tasks in project X with status Y". A composite index on `(project_id, status)` serves both `WHERE project_id = $1` (index scan on leftmost column) and `WHERE project_id = $1 AND status = $2` (full composite use). The order matters — `(status, project_id)` would not serve the project-only query as efficiently.

**`updated_at` trigger:** The trigger fires `BEFORE UPDATE` on every row update, setting `NEW.updated_at = NOW()`. Application code does not need to set `updated_at` — it is always correct regardless of which code path performs the update.

---

## 12. Docker & Compose

### Dockerfile

```dockerfile
FROM golang:1.23-alpine AS builder
WORKDIR /src
COPY backend/ .
RUN go mod tidy && CGO_ENABLED=0 GOOS=linux go build -o /server ./cmd/server
```

`AS builder` names this stage. Only the output (`/server`) is carried to the next stage — all Go tooling, source code, and build cache stay in this stage.

`go mod tidy` is called at build time because there is no `go.sum` committed to the repo. In a production setup, `go.sum` would be committed and `go mod download` would be used instead (faster, reproducible builds). The tradeoff: `go mod tidy` checks network on every build. `CGO_ENABLED=0` disables cgo — the resulting binary is statically linked and does not depend on any shared libraries. `GOOS=linux` ensures the binary targets Linux even if built on macOS or Windows. `-o /server` places the binary at `/server` in the builder stage.

```dockerfile
FROM alpine:3.20
RUN adduser -D -g '' appuser
COPY --from=builder /server /server
USER appuser
EXPOSE 8080
ENTRYPOINT ["/server"]
```

`alpine:3.20` is a minimal Linux distribution (~5MB). The builder stage (`golang:1.23-alpine`) is ~600MB — only the statically compiled binary is copied into the runtime image. `adduser -D` creates a user with no password, no home directory, no login shell. `USER appuser` drops root privileges before running the server. `EXPOSE 8080` is documentation — it does not actually publish the port. `ENTRYPOINT ["/server"]` (exec form, square brackets) makes the Go binary PID 1 directly. If it were `ENTRYPOINT /server` (shell form), Docker would run `/bin/sh -c /server`, making `sh` PID 1 — `sh` does not forward signals, so `docker stop` would kill the container with SIGKILL after a timeout instead of cleanly shutting down the server.

### docker-compose.yml

```yaml
db:
  healthcheck:
    test: ["CMD-SHELL", "pg_isready -U ${DB_USER:-taskflow}"]
    interval: 2s
    timeout: 5s
    retries: 5

app:
  depends_on:
    db:
      condition: service_healthy
```

`service_healthy` means the `app` container does not start until `db` has passed its healthcheck. `pg_isready` checks that Postgres is accepting connections on the given user. Even with this, there is a race between "Postgres accepts connections" and "Postgres has initialized the database", which is why `connectDB` in `main.go` retries 5 times.

```yaml
environment:
  JWT_SECRET: ${JWT_SECRET:-super-secret-change-me-in-production}
```

`${JWT_SECRET:-default}` uses the `JWT_SECRET` variable from the environment or `.env` file if set, and falls back to the default string if not. The config's `if cfg.Auth.JWTSecret == ""` check does not trigger because this default is non-empty. The intent: the app starts in development without requiring a `.env` file, but the `.env.example` makes it clear that the default must be changed before production use.

---

## 13. Integration Tests

**File:** `backend/tests/integration/setup_test.go`

```go
func setupRouter(t *testing.T) *gin.Engine {
    dsn := testDSN()
    pool, err := pgxpool.New(ctx, dsn)
    ...
    if err := migrations.Run(dsn); err != nil { t.Fatalf(...) }
    _, err = pool.Exec(ctx, "TRUNCATE tasks, projects, users CASCADE")
    ...
    tokenService := auth.NewTokenService("test-secret-for-integration-tests", 1*time.Hour)
    authService := service.NewAuthService(userRepo, tokenService, 4)  // bcrypt cost 4
```

`TRUNCATE ... CASCADE` removes all rows from all three tables in one statement, respecting foreign keys (tasks first, then projects, then users — or CASCADE handles the order). This runs before each test via `setupRouter`, giving each test a clean database.

`bcrypt cost 4` is the minimum allowed by golang.org/x/crypto/bcrypt. At cost 12 (production), bcrypt takes ~250ms per hash. At cost 4, it takes ~1ms. Integration tests hash passwords on register — at cost 12, a single test would take 500ms+. At cost 4, the full test suite runs in under 1 second.

```go
func testDSN() string {
    if v := os.Getenv("TEST_DATABASE_URL"); v != "" { return v }
    return "postgres://taskflow:taskflow@localhost:5432/taskflow_test?sslmode=disable"
}
```

`TEST_DATABASE_URL` allows CI environments (GitHub Actions, etc.) to point the tests at any database. The default `localhost:5432` matches `docker compose up db -d` on a developer machine.

```go
func registerAndLogin(t *testing.T, router *gin.Engine, name, email, password string) string {
    body := fmt.Sprintf(`{"name":"%s","email":"%s","password":"%s"}`, name, email, password)
    w := doRequest(router, "POST", "/auth/register", body, "")
    if w.Code != 201 { t.Fatalf(...) }
    return extractToken(t, w)
}
```

Tests use `httptest.NewRecorder` — no actual HTTP server is started. The router is called directly with a synthetic request and response recorder. This is faster than spinning up a real server and sending real HTTP requests.

---

## 14. Design Pattern Summary

| Pattern | File | Implementation |
|---|---|---|
| **Factory Method** | `domain/user.go`, `domain/project.go` | `NewUser()`, `NewProject()` validate and produce fully-initialized values; raw struct construction is impossible from outside the package |
| **Builder** | `domain/task.go` | `NewTaskBuilder()` with fluent setters, defaults at construction, validation in `Build()` |
| **Builder** | `repository/postgres/query_builder.go` | `NewQueryBuilder()` assembles SQL with dynamic WHERE clauses; `Build()` returns synchronized data + count queries |
| **Repository** | `repository/interfaces.go` + all postgres/*.go | Data access behind interfaces; services depend on interfaces, not concrete types |
| **Interface Segregation** | `repository/interfaces.go` | `UserReader`/`UserWriter` split; consumers depend on only the methods they use |
| **Decorator** | `repository/logging/` | Logging wraps postgres implementations with the same interface; behavior is unchanged, observability is added |
| **Functional Options** | `config/config.go` | `WithBcryptCost()`, `WithJWTExpiry()`, `WithServerPort()` override specific defaults |
| **Chain of Responsibility** | `handler/middleware.go` + `handler/router.go` | `AuthMiddleware` aborts or calls `c.Next()`; protected routes are a chain: middleware → handler |
| **Strategy (functional)** | `handler/response.go` | `mapError()` dispatches domain errors to HTTP status codes via a switch |

---

## 15. Likely Interview Questions

**Q: Why did you use Go for this?**
Go produces a statically-linked binary, which is ideal for containers — the final image is ~12MB. The standard library covers structured logging (`slog`), HTTP (`net/http`), cryptography (`crypto/hmac`), and JSON (`encoding/json`). The concurrency model (goroutines + channels) is a natural fit for handling many concurrent HTTP requests.

**Q: Why Gin over the standard library's `net/http`?**
Gin adds path parameter parsing (`:id`), route grouping for middleware application, and `c.ShouldBindJSON`. These are all possible with `net/http` directly, but require boilerplate. Gin adds ~350KB to the binary and no transitive DB or auth dependencies. For an assignment, it is the pragmatic choice.

**Q: Why `pgx/v5` over `database/sql`?**
pgx is PostgreSQL-specific — it supports native types (UUID, TIMESTAMPTZ, arrays) without conversion overhead. `database/sql` is a generic interface that adds an abstraction layer; every call goes through reflection-based scanning. pgx's `pgxpool` also provides connection pooling with fine-grained control over pool size and health.

**Q: Why implement JWT yourself instead of using `golang-jwt`?**
HMAC-SHA256 JWT is 60 lines using only the standard library. `golang-jwt` would be a dependency for exactly this functionality. The implementation covers the assignment's requirements (generate + verify with expiry). What it doesn't cover: RS256 (needed for multi-service auth where you don't want to share the secret), JWKS endpoints, key rotation. For this scope, the tradeoff is correct.

**Q: How does the partial update on tasks work?**
`updateTaskRequest` uses `*string` (pointer) for every field. When `ShouldBindJSON` parses the body, a field present in JSON is set to a non-nil pointer; an absent field remains `nil`. The service checks `if title != nil { task.Title = *title }` — only non-nil fields update the existing task. This avoids the common bug where omitting a field zeroes it out.

**Q: Why CHECK constraints instead of PostgreSQL ENUMs?**
PostgreSQL ENUMs cannot be modified inside a transaction. Adding a new status value requires: `ALTER TYPE task_status ADD VALUE 'blocked'`, which is DDL that cannot be rolled back if something goes wrong during migration. CHECK constraints are plain DDL that can be added, dropped, and recreated inside a transaction. The tradeoff is that CHECK constraints are not enforced at the Go type level — invalid strings reach Postgres before being rejected.

**Q: How does graceful shutdown work?**
`signal.Notify` registers the process to receive `SIGINT` (Ctrl+C) and `SIGTERM` (Docker stop). When either arrives, `srv.Shutdown(ctx)` is called with a 10-second timeout. `Shutdown` stops accepting new connections and waits for in-flight requests to complete. Requests that take longer than 10 seconds are cancelled. The binary is PID 1 in Docker (exec-form ENTRYPOINT) so it receives SIGTERM directly — no shell wrapper intercepts it.

**Q: What is the `var _ repository.UserRepository = (*userRepo)(nil)` line?**
A compile-time interface check. It attempts to assign a nil `*userRepo` to a variable of type `repository.UserRepository`. If `*userRepo` is missing any method required by the interface, the compiler produces an error at that line — immediately, not at runtime. This is a Go idiom for asserting interface compliance without allocating a real value.

**Q: Why does `GetUserID` panic instead of returning an error?**
Missing middleware on a route is a programming error, not a runtime condition. If `GetUserID` returned `(uuid.UUID, error)`, every handler would have to handle an error that should never occur in a correctly-configured application. A panic during development (before the route is tested) is more useful than a silent zero-UUID in production. The panic message `"getClaims called without AuthMiddleware"` makes the cause immediately obvious.

**Q: What would you change if this were going to production?**
Rate limiting on auth endpoints, refresh tokens, request ID propagation for log correlation, structured error codes (not just strings), connection pool tuning, and role-based access control. All are in the "What I'd Do With More Time" section of the README with specific reasoning for each.
