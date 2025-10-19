# Pending Work Before Step 3

This list captures the outstanding tasks that must be completed during Steps 1 and 2 (test-first development and implementation/refinement) before advancing to Step 3 (documentation and review).

## Test Scaffolding (Step 1)
- [ ] Flesh out failing test skeletons for the Schedule application service methods (`CreateSchedule`, `UpdateSchedule`, `ListSchedules`, `DeleteSchedule`) that cover creator immutability, administrator overrides, validation of start/end windows and required fields (including web conference URL format), verifying participant and room existence, creating schedules on behalf of other users, and allowing hybrid meetings that simultaneously set physical room and web conference information.
- [ ] Extend Schedule service test scaffolds with cases that assert participant filters, multi-user list views, recurrence expansion hooks, propagation of conflict warnings from the detector, chronological ordering of returned schedules, cleanup of linked recurrences when schedules are updated or deleted, enforcement of JST-only scheduling, translation of day/week/month timeframe filters (`StartsAfter`/`EndsBefore`) into the correct result set, and success paths that still persist updates when conflicts are reported.
- [ ] Describe `ListSchedules` test scaffolds that ensure the default view (no explicit participant filter) returns only the authenticated user's schedules while explicit participant filters allow viewing colleagues, matching the "My schedule" vs. "Selected colleagues" requirement without leaking unintended records.
- [ ] Add Schedule service test scaffolds that cover authorization failures for non-creators attempting updates or deletes, administrator override success paths, and consistent `ErrUnauthorized`/`ErrNotFound` propagation for missing schedules.
- [ ] Add unit-test scaffolds covering `RoomService` CRUD behavior, including administrator-only access constraints and validation of required attributes (name, location, positive capacity).
- [ ] Capture `RoomService.ListRooms` scaffolds that assert read access is available to all authenticated employees (not only administrators) so schedule creation flows can surface the catalog.
- [ ] Create unit-test scaffolds for administrator-only user management service methods (`CreateUser`, `UpdateUser`, `ListUsers`, `DeleteUser`) that validate input handling and privilege enforcement.
- [ ] Draft unit-test scaffolds for the authentication service, including password hashing edge cases, lockout behavior (if retained), session/token issuance flows, and sentinel errors for invalid credentials or disabled accounts.
- [ ] Tighten conflict detection unit-test scaffolds in `internal/scheduler` to describe overlapping participant and room intervals, identical-ID short-circuit behavior, and non-overlap baselines before wiring services to the detector.
- [ ] Describe HTTP middleware/component tests that assert session token validation and propagation of the authenticated principal into handler contexts.
- [ ] Introduce recurrence engine test outlines that describe weekday selection, timezone handling, clipping generated occurrences to requested timeframes, and generated occurrence linking.
- [ ] Define persistence adapter test scaffolds (repositories for users, schedules, rooms, recurrences, and authentication data), including integration test placeholders using SQLite fixtures, coverage for foreign-key cascades, and translation of uniqueness/lookup violations into sentinel errors.
- [ ] Add persistence test scaffolds for session management repositories covering token creation, lookup, expiration, and revocation behavior.
- [ ] Outline component-test scaffolds for HTTP handlers (authentication, user management, schedules, rooms) to validate request validation, authorization (including 403 responses for non-creators), response shaping, login responses that set session tokens (cookie/header), logout flows that revoke sessions, surfacing of conflict warnings and recurrence-expanded results, and returning localized (Japanese) error and validation messages per the specification.
- [ ] Add HTTP handler test scaffolds for schedule listing that cover the default personal view, explicit colleague selections, translation of `ErrNotFound`/`ErrUnauthorized` sentinel errors into `404`/`403` responses for missing or forbidden schedule resources, and correct day/week/month query parameter mapping (including Monday-start week windows and default daily time spans) into service timeframe filters.
- [ ] Capture configuration loader test scaffolds that assert environment variable parsing, default fallback behavior, and validation of required settings (HTTP port, SQLite DSN, session secrets) so Step 2 work can proceed test-first.

## Implementations to Unblock Green Phase (Step 2)
- [ ] Wire the existing conflict-detection logic into schedule creation/update flows so conflict warnings surface in service and handler responses while still persisting changes when warnings are present, matching the "warnings don't block" rule.
- [ ] Implement the application services (`ScheduleService`, `RoomService`, `UserService`, authentication service) as described in the specification, aligning signatures with the test scaffolding.
- [ ] Ensure application services enforce domain validations (required fields, JST time windows, participant existence, admin-only operations) and return sentinel errors that handlers can translate consistently, including unauthorized attempts to update/delete schedules created by other users.
- [ ] Implement the persistence layer (`internal/persistence` package) with repositories and migration helpers to satisfy integration tests, covering users, schedules, rooms, recurrences, and session storage, including maintaining the participant join table, participant-based filtering for schedules, and cascading cleanup of recurrence/session rows.
- [ ] Ensure `ScheduleRepository.ListSchedules` combines participant filters and timeframe constraints so multi-user views and day/week/month queries return the correct data set, orders results chronologically, and clips recurrence expansions to the requested window.
- [ ] Implement the default "my schedule" semantics in `ScheduleService.ListSchedules` so requests without explicit participant filters only return the authenticated user's schedules while honoring colleague selections when provided.
- [ ] Provide recurrence engine logic that satisfies the outlined recurrence tests.
- [ ] Extend persistence and application layers with session storage/revocation to back the authentication service's token issuance and expiration semantics that middleware will enforce.
- [ ] Persist password hashes for users (schema + repository updates) so that the authentication service can verify credentials securely.
- [ ] Implement HTTP API handlers and routing that pass the planned component tests, covering authentication (login issuing tokens + logout revocation), user management, schedule CRUD, and room management endpoints, including conflict warning serialization, recurrence-expanded payloads for schedule listing endpoints, translation of sentinel errors into 401/403/404/409 responses per test expectations, and localized Japanese messaging for user-facing errors.
- [ ] Finalize the HTTP routing map and request/response DTO definitions so handler implementations and tests share stable contracts before production wiring begins.
- [ ] Implement an environment-driven configuration module (with validation and defaults) consumed by the entry point and dependency wiring, covering HTTP server settings, SQLite DSN, and authentication/session secrets.
- [ ] Instrument application services and HTTP handlers with structured logging via `log/slog`, including contextual request identifiers and error surfaces, matching the architecture guidance.
- [ ] Ensure room listing handlers expose catalog data to all authenticated users so schedule creation clients can populate selection lists without administrator privileges.
- [ ] Wire authentication middleware that consumes the session repository and surfaces domain-specific authorization errors for handlers to translate into HTTP responses, rejecting expired sessions, and returning consistent 401/403 responses per test expectations.
- [ ] Provide an executable entry point (e.g., `cmd/scheduler`) that wires configuration, repositories, services, and HTTP routing so the API can run end-to-end before documentation begins.
- [ ] Select and integrate the cgo-free SQLite driver referenced in the architecture overview, ensuring migrations and repository implementations can run in CI environments without CGO dependencies.
- [ ] Produce a Dockerfile with a multi-stage build that outputs the single-scheduler binary described in the architecture plan to smooth later deployment and manual verification.

## Supporting Infrastructure
- [ ] Create reusable deterministic fixtures/builders in `internal/testfixtures` to support the upcoming tests.
- [ ] Establish a temporary SQLite helper for integration tests (migrations, cleanup) referenced by the persistence test scaffolding.
- [ ] Introduce dependency injection wiring (even minimal) so that application services can be instantiated in tests without production infrastructure.
- [ ] Provide a controllable clock/test time helper so schedule, recurrence, and session expiry logic remain deterministic under test.
- [ ] Establish CI automation (e.g., GitHub Actions) that runs `go test ./...` with the race detector and `golangci-lint`, and commit a baseline `.golangci.yml` aligned with the agreed test strategy while pinning Go 1.22 to match `go.mod`.
- [ ] Extend CI automation to fail the build when statement coverage for application and persistence packages drops below 80%, matching the coverage target in the test strategy document.

## Status Tracking
- Update this checklist as work progresses to ensure Step 3 only begins after the above items are implemented and all tests are green.
