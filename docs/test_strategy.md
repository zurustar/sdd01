# Enterprise Scheduler Test Strategy

This document records Step 2: aligning the specification with the test
approach, identifying layers to cover, and defining interface contracts that
make tests possible from the outset.

## Testing Principles
* **Specification to test mapping:** Every acceptance criterion in
  `docs/enterprise_scheduler_spec.md` maps to at least one automated test. The
  level (unit/integration/e2e) is chosen based on the breadth of behavior.
* **Red-Green development:** Write failing tests that describe behavior before
  implementing the corresponding production code.
* **Deterministic data:** Use deterministic fixtures and factory helpers to keep
  tests reproducible.

## Test Levels and Responsibilities

| Level | Scope | Primary Targets | Tooling |
| ----- | ----- | --------------- | ------- |
| Unit | Single function or method | Application services, conflict detection logic, recurrence engine, repository adapters with sqlite mock | `testing` + table-driven tests |
| Integration | Real SQLite database with migration helper (to be implemented) | Repository implementations, transaction boundaries, schema constraints | `testing` + temporary filesystem for DB |
| API (Component) | HTTP handlers with in-memory dependencies | Request validation, auth middleware, response formatting | `net/http/httptest`, dependency injection |
| End-to-End (post-MVP) | Full stack inc. UI | Manual or automated browser tests (future) | Playwright/Cypress (backlog) |

## Key Modules and Interfaces

### Authentication
* `AuthService` exposes `Authenticate(email, password)` returning a session token
  or domain error.
* Tests cover password hashing edge cases, account lockout (if added later), and
  session issuance.

### Schedule Management
* `ScheduleService`
  - `CreateSchedule(ctx, CreateScheduleParams) (Schedule, []ConflictWarning, error)`
  - `UpdateSchedule(ctx, id, UpdateScheduleParams) (Schedule, []ConflictWarning, error)`
  - `ListSchedules(ctx, Filter) ([]Schedule, error)`
  - `DeleteSchedule(ctx, id) error`
* Tests assert creator immutability, participant assignment, recurrence
  generation hooks, and conflict warnings.

### Meeting Room Catalog
* `RoomService`
  - `CreateRoom(ctx, RoomInput) (Room, error)`
  - `UpdateRoom(ctx, id, RoomInput) (Room, error)`
  - `DeleteRoom(ctx, id) error`
  - `ListRooms(ctx) ([]Room, error)`
* Tests verify admin-only access (with mocks for auth), validation rules, and
  persistence interaction.

### Recurrence Engine
* Pure functions translating recurrence rules to occurrences.
* Table-driven tests covering combinations of weekdays, start/end boundaries,
  and timezone handling (JST only).

### Conflict Detection
* Function `DetectConflicts(existing []Schedule, candidate Schedule) []Conflict`.
* Unit tests for overlapping intervals by participant and room (skeleton currently describes participant, room, and non-overlap scenarios).
* Implementation pending; the `internal/scheduler` package hosts the test scaffolding that will be completed during the Green phase.

## Test Data and Fixtures
* Use `internal/testfixtures` package for reusable builders (e.g., `NewUser()`,
  `NewSchedule()`), ensuring tests remain expressive.
* For integration tests, we will leverage temporary directories and run
  migrations via the SQLite helper introduced during implementation. The exact
  package path will be finalized when Step 3 begins.

## Tooling and Automation
* CI executes `go test ./...` with race detector (`-race`) in nightly builds once
  Go packages exist.
* Static analysis via `golangci-lint` (configuration TBD) ensures style and
  error handling consistency.
* Coverage thresholds: maintain ≥80% statement coverage for application and
  persistence packages (excluding generated code).

## Open Questions
* Whether to introduce BDD-level tests mirroring the Gherkin scenarios directly.
* Decision on mock framework vs. manual fakes (default to manual fakes for now).
* Handling of time-dependent tests (consider using `clock` abstraction).

