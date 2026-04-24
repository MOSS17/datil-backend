# Backend Delivery Plan — Phased

This file is the running execution plan for the work outlined in `TODO-backend.md` and `TODO-security.md`. It exists so a fresh session can pick up mid-flight without needing the whole conversation history.

**Workflow**: one PR per phase, branched off `main`, named `phase-N-<slug>`. Each phase ships in two halves: a backend PR here, then a frontend wire-up PR in `../frontend/` (`MOSS17/datil-frontend`) that flips mocks for the new surface and runs a manual smoke. **A phase is not "done" until both PRs are merged and the smoke is green.** Don't start phase N+1 backend work before phase N's frontend cutover lands — drift compounds fast.

---

## Status

| Phase | Title | Backend | Backend PR | Frontend | Frontend PR |
|---|---|---|---|---|---|
| 0 | Foundation — response envelope, storage seam, model cleanup | **Merged** | [#1](https://github.com/MOSS17/datil-backend/pull/1) | **Merged** | [frontend#4](https://github.com/MOSS17/datil-frontend/pull/4) |
| 1 | Auth — signup / login / refresh with rotation | **Merged** | [#2](https://github.com/MOSS17/datil-backend/pull/2) | **Merged** | [frontend#4](https://github.com/MOSS17/datil-frontend/pull/4) |
| 2 | Logo + service extras (R2 wired) | **Merged** | [#3](https://github.com/MOSS17/datil-backend/pull/3) | **Merged** | [frontend#4](https://github.com/MOSS17/datil-frontend/pull/4) |
| 2.1 | Categories CRUD (services depend on it) | **Merged** | [#6](https://github.com/MOSS17/datil-backend/pull/6) | **Merged** | [frontend#4](https://github.com/MOSS17/datil-frontend/pull/4) |
| 3 | Public booking flow + availability algorithm | **Merged** | [#8](https://github.com/MOSS17/datil-backend/pull/8) | **Merged** | [frontend#5](https://github.com/MOSS17/datil-frontend/pull/5) |
| 4 | Owner dashboard + appointments CRUD | Not started | — | Not started (still mocked: `/dashboard`, `/appointments/*`) | — |
| 5 | Schedule config (workdays + personal time) | Not started | — | Not started (still mocked: `/schedule/*`) | — |
| 6 | Calendar integration (Google + Apple OAuth, two-way sync) | Not started | — | Not started (still mocked: `/calendar/*`) | — |
| 7 | Polish — startup migrations, non-root container, CI | Not started | — | n/a (no frontend surface) | — |

**Frontend wire-up batching**: phases 0–2.1 are covered by a single frontend PR ([frontend#4](https://github.com/MOSS17/datil-frontend/pull/4)) because mock-replacement was done in one pass. Future phases get their own frontend PRs.

**To pick up work**: check the row's *backend* state. If `Not started`, branch off `main` as `phase-N-<slug>`, execute the phase below, open a PR. Then in `../frontend/`, branch as `wire-phase-N`, replace the relevant mocks, run the smoke checklist, open a PR. Update both columns of this table in the same PR they describe.

**Phase completion criteria** (apply to every row before moving on):
1. Backend PR merged on `main`.
2. Frontend wire-up PR merged.
3. Manual end-to-end smoke against the local backend confirms the affected dashboard / booking pages render and accept input correctly.
4. No regressions on previously-wired surfaces (sign in still works after wiring services, etc.).

**API path convention**: every endpoint below is mounted under `/api/v1/`. The doc omits the prefix when describing route shapes (e.g. "POST /auth/signup") but real requests are `POST /api/v1/auth/signup`. Static dev-only file server at `/static/uploads/*` is *not* prefixed — it's not an API.

---

## Scope decisions (locked in)

- **Email verification dropped**: no `/auth/verify-email`, no `/auth/resend-code`. Users log in directly after signup. No `is_verified` / `phone` fields on User.
- **Password reset dropped**: no `/auth/forgot-password`, no `/auth/reset-password`, no `password_reset_tokens` table, no email provider in scope.
- **Refresh-token rotation kept**: per `TODO-security.md` item 5 — costs one table + ~30 lines, gives a real revocation story from day one.
- **Storage**: Cloudflare R2 (free tier, zero egress, S3-compatible SDK).
- **Naming**: `/auth/signup` (not register), `PUT /business/logo` (not `/businesses/:id/logo`), `AuthResponse` (not `LoginResponse`). `businesses.logo` column renamed to `logo_url` to match frontend.
- **Payment proof**: inline multipart inside `POST /book/{url}/reserve` (not a separate upload endpoint).

These decisions are closed. Do not revisit without flagging in a PR description.

---

## Phase 0 — Foundation ✅

**Goal**: cross-cutting changes that unblock every subsequent phase. No endpoint behavior changes.

### Shipped
- `internal/httpx/response.go` — `WriteJSON`, `WriteError(w, status, message, fields)`, `WriteNoContent`, `ReadJSON(w, r, dst)`. New envelope: raw JSON on success, `{message, errors?}` on errors.
- `internal/handler/response.go` — thin delegators over `httpx`.
- `internal/middleware/auth.go` — uses `httpx.WriteError` instead of inline `http.Error`.
- 35 handler stubs renamed `ErrorJSON(...)` → `WriteError(..., nil)`.
- `internal/model/models.go` — `LoginResponse` → `AuthResponse`; `Business.Logo` → `Business.LogoURL` (`json:"logo_url"`).
- `internal/storage/` — `Uploader` interface, `LocalDiskUploader` (dev), `DetectAndValidate` (magic-byte sniffing via `http.DetectContentType`), `EnforceSize`, table-driven tests.
- `internal/config/config.go` — `BcryptCost`, `StorageProvider` (`"local"|"r2"`), R2 creds, local upload config. Provider-aware validation at `Load()`.
- `.env.example` — documents new vars.
- `migrations/000013_alter_businesses_rename_logo.{up,down}.sql` — column rename.
- `TODO-backend.md`, `TODO-security.md` committed as contract spec.

### Verified
- `go build ./...` clean.
- `go vet ./...` clean.
- `go test ./internal/storage/...` green.

---

## Phase 1 — Auth (signup / login / refresh)

**Goal**: 3 endpoints the router already exposes, plus refresh-token rotation and rate limiting.

### New dependencies
- `golang.org/x/crypto/bcrypt` — `go get golang.org/x/crypto/bcrypt`
- `github.com/go-chi/httprate` — `go get github.com/go-chi/httprate`

### Migration
`migrations/000014_create_refresh_tokens.up.sql`:
```sql
CREATE TABLE refresh_tokens (
    jti UUID PRIMARY KEY,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    expires_at TIMESTAMPTZ NOT NULL,
    used_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_refresh_tokens_user ON refresh_tokens(user_id);
```
Paired `.down.sql`: `DROP TABLE refresh_tokens;`

### Repository implementations

**`internal/repository/user.go`** — fill bodies (signatures already defined):
- `GetByID(ctx, id) (*User, error)`
- `GetByEmail(ctx, email) (*User, error)`
- `Create(ctx, tx, u) error` — INSERT RETURNING id, created_at, updated_at; expects `u.BusinessID` set
- `Update(ctx, id, u) error`

**`internal/repository/business.go`** — only the ones Phase 1 needs:
- `Create(ctx, tx, b) error` — INSERT RETURNING …
- `GetByID(ctx, id) (*Business, error)`

Other business methods (`GetByURL`, `Update`, `UpdateBank`, `UpdateLogo`) land in Phase 2.

**`internal/repository/refresh.go`** (new file):
```go
type RefreshTokenRepository interface {
    Insert(ctx context.Context, jti uuid.UUID, userID uuid.UUID, expiresAt time.Time) error
    MarkUsed(ctx context.Context, jti uuid.UUID) (alreadyUsed bool, err error)
    DeleteAllForUser(ctx context.Context, userID uuid.UUID) error
}

func NewRefreshTokenRepository(pool *pgxpool.Pool) RefreshTokenRepository { /* … */ }
```

`MarkUsed` must be atomic: `UPDATE refresh_tokens SET used_at = NOW() WHERE jti = $1 AND used_at IS NULL RETURNING jti`. If the row doesn't return (already used or missing), return `alreadyUsed=true` — this is the theft signal.

### Middleware changes

**`internal/middleware/auth.go`**:
- Inside `GenerateTokenPair`, set `RegisteredClaims.ID = uuid.NewString()` on the **refresh** token. Don't add a custom claim; `jwt.RegisteredClaims.ID` is the JTI.
- Refactor signature to `GenerateTokenPair(...) (access, refresh string, refreshJTI uuid.UUID, err error)` so the handler can persist `refreshJTI`.
- Add `ParseRefreshToken(tokenString, secret string) (*Claims, error)` — same as JWT parse but doesn't reject `type=refresh` (the existing middleware does). Used by the `/auth/refresh` handler.

**`internal/middleware/ratelimit.go`** (new) — thin wrapper:
```go
func PerIP(requests int, window time.Duration) func(http.Handler) http.Handler {
    return httprate.LimitByIP(requests, window)
}
```

### Handler implementations — `internal/handler/auth.go`

**Signup** (`POST /auth/signup`):
1. `httpx.ReadJSON` into `SignupRequest`. Validate: name/email/password/business_name non-empty; email format; password ≥ 8 chars.
2. `bcrypt.GenerateFromPassword([]byte(password), cfg.BcryptCost)`.
3. `repository.WithTransaction`:
   - `businessRepo.Create(ctx, tx, &Business{Name: req.BusinessName, URL: slug(req.BusinessName)})` → returns id.
   - `userRepo.Create(ctx, tx, &User{BusinessID: businessID, Name: req.Name, Email: req.Email, Password: string(hash)})`.
4. `access, refresh, jti, _ := GenerateTokenPair(...)`.
5. `refreshRepo.Insert(ctx, jti, userID, now.Add(cfg.JWTRefreshExpiry))`.
6. `WriteJSON(w, 200, AuthResponse{...})`.

Uniqueness: users.email is already UNIQUE; catch pgx `23505` and return `{message: "Ese correo ya está registrado"}` with 409.

Slug for Business.URL: use something like `slug(business_name) + "-" + random-4-chars` to avoid collisions; prove uniqueness via UNIQUE constraint + retry once on 23505.

**Login** (`POST /auth/login`):
1. Read `LoginRequest`.
2. `user, err := userRepo.GetByEmail(email)`.
3. `bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.Password))`.
4. On any failure (user not found OR password mismatch): return 401 with generic `{message: "Correo o contraseña incorrectos"}` — uniform error to avoid enumeration.
5. Generate pair, insert jti, return `AuthResponse`.

**Refresh** (`POST /auth/refresh`):
1. Parse `{refresh_token}`.
2. `claims, err := ParseRefreshToken(req.RefreshToken, cfg.JWTSecret)`. Reject if `claims.TokenType != "refresh"`.
3. `jti, _ := uuid.Parse(claims.ID)`.
4. `alreadyUsed, err := refreshRepo.MarkUsed(ctx, jti)`.
5. If `alreadyUsed`: `refreshRepo.DeleteAllForUser(ctx, claims.UserID)` (theft signal) → 401.
6. Issue new pair, insert new jti, return `AuthResponse`.

### Router changes — `internal/router/router.go`

Wrap the `/auth` group with `httprate.LimitByIP(5, time.Minute)`.

### Wiring — `cmd/api/main.go`

- Instantiate `refreshRepo := repository.NewRefreshTokenRepository(pool)`.
- Update `NewAuthHandler` signature to accept it; pass through.

### Tests

Minimum bar: none required; the handler logic is thin enough and integration tests would need real Postgres infra. If time permits, add a `handler/auth_test.go` with table-driven signup validation tests using a mock `UserRepository` (there's already `golang/mock` in `go.mod`).

### Ship gate

Run the full flow manually:
1. `curl -X POST localhost:8080/api/v1/auth/signup -d '{"name":"M","email":"a@b.c","password":"pw123456","business_name":"Demo"}'` → receive `AuthResponse` with both tokens.
2. `curl -X GET localhost:8080/api/v1/dashboard -H "Authorization: Bearer $ACCESS"` → 200 (or 501 if dashboard still stubbed — the point is "not 401").
3. `curl -X POST localhost:8080/api/v1/auth/refresh -d '{"refresh_token":"'$REFRESH'"}'` → new pair.
4. Same refresh token again → 401. `SELECT used_at FROM refresh_tokens` → all user's tokens have `used_at IS NOT NULL`.

### Files to modify / create
- `go.mod` / `go.sum` — `go get` new deps
- `migrations/000014_create_refresh_tokens.{up,down}.sql` (new)
- `internal/repository/user.go` (fill bodies)
- `internal/repository/business.go` (fill `Create`, `GetByID`)
- `internal/repository/refresh.go` (new)
- `internal/middleware/auth.go` (jti in claims, `ParseRefreshToken`, signature change)
- `internal/middleware/ratelimit.go` (new)
- `internal/handler/auth.go` (implement 3 stubs)
- `internal/router/router.go` (rate-limit group)
- `cmd/api/main.go` (wire `refreshRepo`)

---

## Phase 2 — Logo upload + service extras

**Goal**: upload pipeline proven end-to-end against R2; service extras CRUD live.

### New dependencies
- `github.com/aws/aws-sdk-go-v2` + `github.com/aws/aws-sdk-go-v2/config` + `github.com/aws/aws-sdk-go-v2/service/s3` + `github.com/aws/aws-sdk-go-v2/credentials`

### Files

**`internal/storage/r2.go`** (new):
```go
type R2Uploader struct {
    client   *s3.Client
    bucket   string
    publicBaseURL string
}

func NewR2Uploader(cfg R2Config) (*R2Uploader, error)
func (u *R2Uploader) Upload(ctx, key, ct string, size int64, r io.Reader) (string, error)
```

Use `s3.NewFromConfig(awsCfg)` with custom endpoint `https://<account_id>.r2.cloudflarestorage.com` via `config.WithEndpointResolverWithOptions`. Credentials via `credentials.NewStaticCredentialsProvider`.

**`internal/handler/business.go`** — implement all four:
- `Get`: return `businessRepo.GetByID(ctx.BusinessID)`.
- `Update`: read body, validate, `businessRepo.Update`.
- `UpdateBank`: read body, `businessRepo.UpdateBank`.
- `UpdateLogo`:
  1. `r.ParseMultipartForm(2 << 20)` (2 MB).
  2. `file, hdr, err := r.FormFile("logo")`.
  3. `ct, body, err := storage.DetectAndValidate(file, []string{"image/png","image/jpeg","image/webp"}, 2<<20)`.
  4. `key := fmt.Sprintf("businesses/%s/logo-%s", businessID, uuid.NewString())`.
  5. `url, err := uploader.Upload(ctx, key, ct, hdr.Size, body)`.
  6. `businessRepo.UpdateLogo(ctx, businessID, url)`.
  7. Return updated business.

**`internal/handler/service.go`** — implement all seven stubs + new `ListExtras`:
- Every method must verify the service's `business_id` matches `middleware.BusinessIDFromContext(ctx)`. Return 403 on mismatch, 404 on missing.
- `LinkExtra`: parse `{extra_id}` body; load both services; verify both owned; insert.
- `ListExtras`: return `serviceRepo.ListExtras(serviceID)`.

**`internal/router/router.go`** — add `r.Get("/extras", serviceHandler.ListExtras)` inside the `/services/{id}` subroute.

### Repository implementations

- `internal/repository/business.go` — `GetByURL`, `Update`, `UpdateBank`, `UpdateLogo`.
- `internal/repository/service.go` — all methods: `List`, `GetByID`, `Create`, `Update`, `Delete`, `ListExtras`, `LinkExtra`, `UnlinkExtra`. `ListByBusinessURL` lands in Phase 3.

### Wiring — `cmd/api/main.go`

```go
var uploader storage.Uploader
switch cfg.StorageProvider {
case "r2":
    uploader, err = storage.NewR2Uploader(storage.R2Config{...})
case "local":
    uploader, err = storage.NewLocalDiskUploader(cfg.LocalUploadRoot, cfg.LocalPublicBaseURL)
}
```

Pass `uploader` to `NewBusinessHandler` (signature grows) and (Phase 3) `NewBookingHandler`. In dev (`cfg.Env == "development"`), mount a `/static/uploads/*` file server so local uploads are fetchable.

### Tests

Add an R2 round-trip test gated on `R2_TEST_BUCKET` env — skipped in CI unless explicitly enabled.

### Ship gate
- Upload logo via Postman → returns updated `Business` with real R2 URL → frontend dashboard displays it.
- Cross-business ownership: user A's token on user B's service extra → 403.

---

## Phase 3 — Public booking flow

**Goal**: `/book/{url}/*` live end-to-end. Includes the algorithmic heart of the app — availability computation.

### Files

**`internal/booking/availability.go`** (new, pure function — **this is the tested surface**):

```go
type TimeSlot struct { Start, End time.Time }

func ComputeSlots(
    workday model.Workday,             // with Hours []WorkHour already loaded
    personalTime []model.PersonalTime, // overlapping the target date
    appointments []model.Appointment,  // on the target date
    totalDurationMin int,              // sum of selected service durations
    date time.Time,                    // target date at 00:00 local
    slotStepMin int,                   // e.g. 15
) []TimeSlot
```

Algorithm:
1. Build `[]interval` from `workday.Hours` anchored to `date`.
2. Subtract overlapping personal-time intervals (intersect with date).
3. Subtract appointment intervals (`[StartTime, EndTime)`).
4. For each remaining interval, walk from `interval.Start` in `slotStepMin` increments, emitting `start` where `start + totalDuration ≤ interval.End`.

Keep it pure — no DB, no time.Now(). Inject `date` and all data. This is what the tests depend on.

**`internal/booking/availability_test.go`** — ≥6 table cases:
- Single window, no blocks
- Lunch break (two windows from two WorkHours)
- Appointment mid-day
- Personal-time half-day
- Duration exceeds all windows → empty
- Boundary: slot ending exactly at window close

### Handlers — `internal/handler/booking.go`

- `GetBusiness`: `businessRepo.GetByURL(url)` + `categoryRepo.List(businessID)`. 404 on miss.
- `GetServices`: `serviceRepo.ListByBusinessURL`; for each, `serviceRepo.ListExtras`; group by category.
- `GetAvailability`: parse `?date=YYYY-MM-DD&service_ids=...`; sum durations via `serviceRepo.GetByID` for each; load `workdayRepo.GetByDay(businessID, date.Weekday())`, personal-time overlapping date (via user owning business), appointments same date; call `booking.ComputeSlots`; return `[]TimeSlot`.
- `Reserve`: **multipart** (max 6 MB). Fields: `customer_name, customer_phone, customer_email, start_time, service_ids[], extra_ids[]`, optional file `payment_proof`.
  1. Parse multipart.
  2. `WithTransaction`:
     - `SELECT ... FOR UPDATE` on appointments for `date(start_time)` via `appointmentRepo.ListByDateRangeForUpdate` (new method) → re-check no overlap. **This is the race guard.**
     - If file present: `uploader.Upload(...)` → URL.
     - `appointmentRepo.Create(tx, appointment, services)` — atomically insert `appointments` + `appointment_services`.
  3. Post-commit: `go func(){ notifier.SendBookingConfirmation(bgCtx, phone, details) }()`. Log errors, don't fail the response.
  4. Return created appointment.

### Repository implementations

- `internal/repository/schedule.go` — `ListWorkdays(businessID)` joining `work_hours`; `ListPersonalTime(userID)`.
- `internal/repository/appointment.go` — `Create` (tx-scoped; inserts appointment + services), `ListByDateRange(businessID, from, to)`, `ListByDateRangeForUpdate` (same + `FOR UPDATE`).
- `internal/repository/service.go` — `ListByBusinessURL` (join through businesses).
- `internal/repository/category.go` — `List(businessID)`.

### Router
- `/book/{url}/*`: `httprate.LimitByIP(20, time.Minute)`.
- `POST /book/{url}/reserve`: tighter `5/min` (Twilio send cost).

### Ship gate (backend)
- `go test ./internal/booking/...` passes all 6+ table cases.
- `curl /api/v1/book/<slug>` with a real slug returns the business + categories.
- `curl /api/v1/book/<slug>/availability?date=YYYY-MM-DD&service_ids=…` returns slots for a workday with hours configured.
- `POST /api/v1/book/<slug>/reserve` with multipart payload + payment proof creates the appointment, returns it, and a few seconds later the owner's WhatsApp number receives the confirmation message (or the noop notifier logs a stub if Twilio isn't configured).
- Concurrency: fire two reserves at the same `start_time` against the same business in parallel — exactly one wins; the other returns 409 / overlap error. Proves the `SELECT ... FOR UPDATE` race guard works.

### Frontend cutover (separate PR in `../frontend/`)
Branch `wire-phase-3` off frontend `main`. The backend PR must be merged first so the routes exist for smoke testing.

1. **Mocks router** (`src/api/mocks/router.ts`):
   - Remove the `/^\/business\/slug\/([^/]+)$/` and `/^\/business\/([^/]+)$/` mock handlers — backend now serves business by slug at `/book/{url}`.
   - Remove the `/^\/services$/` and `/^\/services\/([^/]+)$/` *public* fallbacks (still gated by no-token), since the booking flow now reads through `/book/{url}/services`.
   - The dashboard's `/services` calls already hit real backend (covered in phase 2 via `AUTHED_REAL`).

2. **Booking-flow API hooks** (rewrite `useBusiness.ts:useBusinessBySlug` and friends, or add new `useBooking.ts`):
   - `useBusinessBySlug(slug)` → `GET /book/{slug}` (returns business + categories in one shot per phase 3 spec).
   - `useBookingServices(slug)` → `GET /book/{slug}/services` (services grouped by category, with extras).
   - `useAvailability(slug, date, serviceIds)` → `GET /book/{slug}/availability?date=...&service_ids=...`.
   - `useReserve(slug)` → `POST /book/{slug}/reserve` as multipart (`customer_name`, `customer_phone`, `customer_email`, `start_time`, `service_ids[]`, `extra_ids[]`, optional `payment_proof` file).

3. **Reconcile `Category` type**: drop the now-optional `description` and `display_order` from `Category` once the booking flow no longer reads them. Re-tighten `groupServicesByCategory` and `groupExtrasByCategory` sorts. Or: add `display_order` to the backend (a real product call — drag-to-reorder for owners) and keep them. Decide before this phase ships.

4. **Smoke checklist** (against local backend with `STORAGE_PROVIDER=local ENV=development`):
   - `/book/<your-business-slug>` loads (the slug is in the JWT after signup, or visible in the database).
   - Date picker populates only the dates with availability for the selected services.
   - Selecting a date shows the time slots that survived workday ∩ ¬personal-time ∩ ¬appointments.
   - Reserve flow accepts a customer name/phone/email, time slot, and image upload.
   - After reserve, refresh the owner dashboard → the new appointment appears in the day view.
   - Twilio sandbox / noop notifier fires post-commit (check server logs).
   - Concurrency check: open two browser tabs, race-click reserve on the same slot — only one succeeds; the other shows a friendly conflict error.

5. **Things that should still error gracefully** (not in scope for phase 3):
   - `/calendar/*` and `/appointments/*` are still mocked/half-mocked. Don't touch in this PR.

---

## Phase 4 — Owner dashboard + appointments CRUD

**Goal**: flip the authenticated owner-side off mocks. After this phase, signing in loads real appointment data and the owner can manage existing appointments.

### Scope decision to resolve first
Frontend has `APPOINTMENT_STATUS = pending | confirmed | cancelled | completed` (`src/lib/constants.ts`); backend `appointments` table has no status column. Either:
- **(a)** Add a `status` column via migration with default `'confirmed'` and backfill existing rows — owner gets a real state machine.
- **(b)** Drop the status concept entirely for now — every created appointment is implicitly live; cancellation = `DELETE`.

Pick (a) if you need "cancelled but kept for records"; pick (b) if that's a future feature. **This PR's author decides before starting; don't wait for the roadmap.**

### Migration (only if picking option a)
`migrations/0000NN_add_appointments_status.up.sql`:
```sql
ALTER TABLE appointments ADD COLUMN status VARCHAR(16) NOT NULL DEFAULT 'confirmed';
ALTER TABLE appointments ADD CONSTRAINT appointments_status_check
    CHECK (status IN ('pending', 'confirmed', 'cancelled', 'completed'));
```

### Repositories
- `AppointmentRepository` — already mostly filled in phase 3. Verify `List`, `GetByID`, `Update`, `Delete` still work; only `Create` had the tx-required signature from the booking flow, but owner-side manual-create can use a separate tx-less `CreateForOwner` helper or just open its own tx inline.
- `DashboardRepository` — new surface. Single SQL call returns `today_count`, `week_count`, `monthly_income` (sum of `total`), `upcoming` (next N future appointments), `latest` (last N past). All scoped through `users.business_id`. Keep it one aggregate query if feasible; otherwise one small query per field is fine.

### Handlers — `internal/handler/dashboard.go`
- `Get`: return `DashboardData` from a single repo call. Compute "today"/"week"/"month" in the caller's timezone (which, per the deferred-work note, is the server's `time.Local` until business-tz lands).

### Handlers — `internal/handler/appointment.go`
All require `middleware.BusinessIDFromContext` and scope queries through the authenticated user's `business_id`.
- `List`: query params `from=YYYY-MM-DD&to=YYYY-MM-DD` (default: today → today+30d). Returns the owner's appointments in the window.
- `Get`: 403 if the appointment's `user_id` belongs to another business; 404 if missing.
- `Create`: manual owner-initiated booking. Skip the `FOR UPDATE` race guard — owners are expected to know what they're double-booking.
- `Update`: edit customer info, notes, and (if option a) status.
- `Delete`: hard delete.

### Frontend cutover (separate `wire-phase-4` PR)
1. `mocks/router.ts`: add `/^\/dashboard$/` and `/^\/appointments(\/.*)?$/` to `AUTHED_REAL`. Remove the old `/appointments/*` mock handlers (they're now real).
2. Update `useAppointments.ts` return types to match whatever the backend actually returns (status field if you went with option a).
3. Dashboard page: drop `mockAppointments` import; use `useDashboard()`.
4. Smoke: signup a fresh user → zero appointments show on dashboard. Reserve via the public flow → refresh dashboard → appointment appears in today's list.

### Ship gate
- `go test`, `go vet` clean.
- Dashboard page against local backend shows real counts + real upcoming list.
- Owner can delete an appointment; deleting cascades to `appointment_services` (FK already has `ON DELETE CASCADE`).
- Cross-business 403: user A hits `PUT /appointments/<B's-id>` → 403.

---

## Phase 5 — Schedule config (workdays + personal time)

**Goal**: owners can configure their weekly hours and block personal time through the dashboard. Unblocks the phase-3 smoke step that currently requires raw SQL.

### Repositories
Already filled during phase 3. `ScheduleRepository.{ListWorkdays, UpsertWorkdays, ListPersonalTime, CreatePersonalTime, DeletePersonalTime}` are ready. Nothing to do here.

### Handlers — `internal/handler/schedule.go`
- `GetWorkdays`: `scheduleRepo.ListWorkdays(businessID)`. Returns all 7 days (disabled included) so the UI can render the full week.
- `UpdateWorkdays`: body is `[]model.Workday` — whole-week replacement. Handler validates: `day ∈ [0..6]`, each `WorkHour.StartTime < EndTime`, no overlapping hours within a day. Call `scheduleRepo.UpsertWorkdays`.
- `ListPersonalTime`: return the caller's personal time, scoped by `middleware.UserIDFromContext`.
- `CreatePersonalTime`: validate `end_date >= start_date`; if `start_time`/`end_time` are both set then `start_date == end_date` and `start_time < end_time` (mirror the CHECK constraint in the migration, but return 400 with a field-level message instead of a DB error).
- `DeletePersonalTime`: load first, 403 if `user_id` doesn't match the caller, then delete.

### Models
May need request types in `model/models.go` for `UpdateWorkdaysRequest` and `CreatePersonalTimeRequest` if the raw `Workday`/`PersonalTime` shapes don't match what the frontend sends (they likely do). Add only if necessary.

### Frontend cutover (separate `wire-phase-5` PR)
1. `mocks/router.ts`: add `/^\/schedule(\/.*)?$/` to `AUTHED_REAL`. Remove the old schedule mock handlers.
2. `useSchedule.ts`: already exists with the right route shapes (`/schedule/workdays`, `/schedule/personal-time`). Update response parsing if shapes differ.
3. Schedule settings page: verify edits persist + reload, deletes work, validation errors surface inline via `applyApiErrors`.
4. Smoke: save a workday → log out/in → hours persist. Block a Saturday as personal time → public booking flow's `/availability` respects it.

### Ship gate
- `go test`, `go vet` clean.
- Workday editor saves end-to-end.
- Personal-time block shows up as unavailable on the public booking page.
- Phase-3 smoke step 2 ("seed workday hours via SQL") is no longer necessary — the UI does it.

---

## Phase 6 — Calendar integration (Google + Apple OAuth, two-way sync)

**Goal**: bidirectional sync between the owner's external calendar and datil appointments. MVP-blocking but sequenced last because the surface is big and the other phases compose cleanly without it.

### This phase needs research before planning

Calendar integration is the only phase in the roadmap that isn't a straightforward CRUD + wiring exercise. Before starting, answer:

1. **OAuth app registration** — Google Cloud Console project + OAuth consent screen (verification review timeline: weeks if you request sensitive scopes; instant if you stay in "testing" mode with <100 users). Apple is trickier: requires an Apple Developer account + Sign in with Apple setup.
2. **Sync direction + source of truth**:
   - datil → external (push new appointments to Google Calendar): straightforward.
   - external → datil (pull blocked times from Google Calendar to subtract from availability): harder — requires webhook subscriptions or periodic polling.
   - MVP might be one-way (push only) with a "connected to calendar" badge; pull can come later.
3. **Token storage**: access tokens expire; refresh tokens need encryption at rest or a KMS call. `calendar_integrations` table has `access_token` and `refresh_token` columns but no encryption today.
4. **Provider behavior**: Google and Apple have different APIs. Consider whether Apple support is actually MVP (vs. Google-only for launch).

### Suggested staging
- **Phase 6a — Google push-only** (MVP): OAuth handshake, store tokens, post-commit goroutine in `Reserve` also creates a Google Calendar event. Best-effort; failures log.
- **Phase 6b — Google pull** (post-MVP): fetch external events, subtract from availability like personal time.
- **Phase 6c — Apple support** (post-MVP): same shape as 6a but Apple APIs.

Don't scope 6b and 6c until 6a is shipped and the API shape is proven.

### Handlers — `internal/handler/calendar.go`
All three handlers are currently stubs. Whoever picks this up rewrites them completely based on the research above. The repo methods (`GetByUserAndProvider`, `Upsert`, `Delete`) need to be filled too.

### Ship gate (for 6a at minimum)
- Owner clicks "Connect Google Calendar" → OAuth flow completes → `calendar_integrations` row appears.
- Reserve a new appointment → event appears on the owner's Google Calendar within 30 seconds.
- Disconnect → row deleted; future reserves don't attempt to push.

---

## Phase 7 — Polish (deploy hygiene)

**Goal**: last phase before public launch. Everything else must ship first — this is the gate.

### Changes
- `cmd/api/main.go` — startup migration: import `github.com/golang-migrate/migrate/v4`, run `m.Up()` before `ListenAndServe`. Idempotent; fatal on non-nil-non-NoChange error.
- `Dockerfile` — `RUN adduser -D -u 10001 app && chown -R app /app` + `USER app` in final stage.
- `.github/workflows/ci.yml` (new) — `go test ./...`, `go vet ./...`, `govulncheck ./...` (`go install golang.org/x/vuln/cmd/govulncheck@latest`).
- `internal/middleware/auth.go` — verify chi's default `Logger` doesn't emit Authorization headers or request bodies; document with a comment.
- In `refreshRepo.MarkUsed`, the `DeleteAllForUser` call on theft is already specified in Phase 1. Confirm wired and tested.

### Ship gate
- Railway deploy: container starts, migrations run automatically, runs as uid 10001, CI green on a PR, `govulncheck` reports no issues.
- `grep -rn "not implemented" internal/` returns zero hits (see "Keeping this doc honest").

---

## Cross-cutting notes

- **Repo implementations are interleaved, not a phase.** Each phase implements the repo methods its handlers consume. Prevents a monolithic "repo phase" that's impossible to verify.
- **JTI placement**: use `jwt.RegisteredClaims.ID = uuid.NewString()`. Don't add a custom claim.
- **Reserve race guard**: `SELECT ... FOR UPDATE` on the date's appointments inside the reserve tx is the simple fix. A `tstzrange` exclusion constraint is a later hardening.
- **Response shape**: raw JSON payloads on success; `{message, errors?}` on errors. Set by Phase 0 — don't regress.
- **Ownership checks**: every authenticated handler that takes an `{id}` must verify the referenced resource's `business_id` matches `middleware.BusinessIDFromContext(ctx)`.

---

## Keeping this doc honest

- When a phase ships: update the Status table (state + PR link). Do this in the same PR as the phase work.
- If a decision changes: update "Scope decisions" and note why in the PR description.
- If a phase's design drifts during implementation: update that phase section to match reality before merging.
- When all phases merge: move this file's body to an `archive/` directory and replace with a one-line pointer. The roadmap is done when `grep -rn "not implemented" internal/` returns zero hits.

---

## Deferred work (known, chosen to defer)

Items we've hit and consciously pushed off. Revisit when someone needs them, not before.

### Per-business timezone (availability rendering)

**Problem**: `internal/booking/availability.go::ComputeSlots` emits each `TimeSlot.Start` in the server's `time.Local`. Go's `time.Local` is derived from the container's `TZ` env var — if `TZ` is unset (the Docker default), `time.Local` is UTC. A Friday 9am–5pm workday configured in the DB then serializes as `09:00:00Z`, which a customer in America/Mexico_City sees rendered as "9 AM" (the frontend slices `HH:MM` off the string) but actually falls at 2 AM their time. Reservations still round-trip correctly because the same offset is sent back on submit — it's a display-layer lie, not a data bug.

**Workaround for now**: set `TZ=America/Mexico_City` on the Go container at deploy time so `time.Local` matches business reality. Good enough for a single-region MVP. Document the var in the production runbook env table when adding it.

**Real fix (deferred)**:
1. Add `businesses.timezone` column (IANA name, e.g. `America/Mexico_City`), defaulted at signup (infer from browser or ask).
2. Thread a `*time.Location` through `BookingHandler.GetAvailability` → `ComputeSlots`. Availability is then computed in each business's own tz, independent of server tz.
3. Update `work_hours`/`personal_time` consumers to treat their TIME columns as wall-clock in that tz.
4. Frontend stops needing to trust the offset in the response — it can render the timezone alongside the slot.

**Trigger to actually do it**: second business onboarded in a different timezone from the first, or a customer-support ticket about wrong times.

---

## Production setup (operator runbook)

Not a phase — this is the checklist for the first cutover from local-only to a live deployment. Skip until at least phases 1–3 are merged and you're ready to point a real frontend at a real backend. Local development with `STORAGE_PROVIDER=local` works without any of this.

### R2 (object storage)

Cloudflare R2 hosts business logos and (phase 3) customer payment proofs. Free tier covers everything below current scale.

1. **Create the bucket** in the Cloudflare dashboard → R2 → "Create bucket". Pick a name (e.g. `datil-prod`); region "Automatic" is fine. Repeat for `datil-staging` if you want a separate non-prod bucket.
2. **Make objects publicly readable**. R2 buckets are private by default. Two options:
   - **Custom domain** (recommended for prod): R2 → bucket → Settings → "Custom Domains" → connect a subdomain like `cdn.datil.app`. Cloudflare provisions the DNS record and TLS automatically.
   - **r2.dev subdomain** (fast for staging): bucket → Settings → "Public Access" → enable. URL shape: `https://pub-<hash>.r2.dev`. Rate-limited and not for production volume.
3. **Create an API token** scoped to this bucket: R2 → "Manage R2 API Tokens" → "Create API Token" → permissions "Object Read & Write", scope to the specific bucket(s). Save the Access Key ID and Secret Access Key — the secret is shown once.
4. **Grab the account ID** from the right-hand sidebar of the R2 overview page.

### Production env vars

Set on the deployment platform (Railway, Fly, etc.). The app validates these at startup and fails fast on missing R2 creds when `STORAGE_PROVIDER=r2`.

| Var | Example | Notes |
|---|---|---|
| `ENV` | `production` | Disables the `/static/uploads/*` dev mount. |
| `PORT` | `8080` | Railway sets this automatically. |
| `DATABASE_URL` | `postgres://…` | From the managed Postgres add-on. |
| `JWT_SECRET` | 32+ random bytes | `openssl rand -base64 48`. Never commit. Rotating it invalidates every issued token. |
| `JWT_ACCESS_EXPIRY` | `15m` | Default; raise only with a reason. |
| `JWT_REFRESH_EXPIRY` | `168h` | 7 days. |
| `BCRYPT_COST` | `12` | Default; bump to 13 if signup latency budget allows. |
| `CORS_ALLOWED_ORIGINS` | `https://app.datil.mx` | Comma-separated. No trailing slashes. Must include every frontend origin that calls this API. |
| `STORAGE_PROVIDER` | `r2` | Anything else falls back to local disk — wrong for prod (Railway disks are ephemeral). |
| `R2_ACCOUNT_ID` | `abc123…` | From dashboard sidebar. |
| `R2_ACCESS_KEY_ID` | `…` | From the API token. |
| `R2_SECRET_ACCESS_KEY` | `…` | From the API token. Treat as password-grade. |
| `R2_BUCKET` | `datil-prod` | Match the bucket created in step 1. |
| `R2_PUBLIC_BASE_URL` | `https://cdn.datil.app` | Custom domain or `https://pub-<hash>.r2.dev`. No trailing slash. |
| `TWILIO_ACCOUNT_SID` | `AC…` | Optional — booking confirmation SMS/WhatsApp. App degrades to noop notifier if unset. |
| `TWILIO_AUTH_TOKEN` | `…` | Required if SID is set. |
| `TWILIO_WHATSAPP_FROM` | `whatsapp:+1415…` | Twilio's sandbox or approved sender. |

### Sanity checks before flipping DNS

- `curl https://api.datil.mx/healthz` (when added) → 200.
- `curl -X POST https://api.datil.mx/api/v1/auth/signup …` round-trip works against the prod DB.
- `PUT /api/v1/business/logo` with a real PNG → response `logo_url` starts with `R2_PUBLIC_BASE_URL`. Open it in a browser → image renders. If it 403s, the bucket isn't public; revisit step 2.
- `R2_TEST_BUCKET=<staging-bucket> R2_TEST_PUBLIC_BASE_URL=… go test -run TestR2Roundtrip ./internal/storage/...` against the staging bucket as a one-off proof the credentials work.
- Frontend's production build pointed at the prod API — no CORS errors in console.

### What's deferred to Phase 4

Phase 4 covers the deployment-hygiene work that should land *before* the first public traffic: startup migrations (`migrate.Up()` at boot), non-root container, CI with `govulncheck`. Don't open prod to users until phase 4 ships.
