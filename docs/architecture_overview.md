# Enterprise Scheduler Architecture Overview

## Purpose
This document captures the Step 1 outcome for the Enterprise Scheduler MVP: a shared
understanding of the major components, their responsibilities, and how they
collaborate to satisfy the specification in `docs/enterprise_scheduler_spec.md`.
The intent is to guide implementation decisions and provide a touchpoint for
reviews before production code changes are introduced.

## High-Level Component Map

```
+------------------------+        +----------------------------+
|  Presentation Layer    |        | External Integrations      |
|                        |        | (deferred in MVP)          |
| - Web UI (SPA)         |        |                            |
| - HTTP API handlers    |        |                            |
+-----------+------------+        +-------------+--------------+
            |                                |
            v                                v
+-----------+------------+        +-------------+--------------+
| Application Services   |        | Authentication Provider    |
|                        |        | (internal module)          |
| - ScheduleService      |        | - Password hashing         |
| - RoomService          |        | - Session management       |
| - RecurrenceEngine     |        +----------------------------+
| - ConflictDetector     |
+-----------+------------+
            |
            v
+-----------+------------+
| Persistence Layer       |
|                        |
| - Repository interfaces |
|   (UserRepo, RoomRepo, |
|    ScheduleRepo,       |
|    RecurrenceRepo)     |
| - SQLite implementation |
+-----------+------------+
            |
            v
+-----------+------------+
| SQLite Storage          |
| - schema.sql            |
| - migration helper      |
+------------------------+
```

### Presentation Layer
* **Delivery mechanism:** RESTful HTTP API initially, designed to be consumed by a
  web front end. Later iterations can add GraphQL or gRPC without disturbing
  lower layers.
* **Responsibilities:** request validation, authentication/authorization checks,
  translating HTTP status codes, and orchestrating calls to application services.

### Application Services
* Encapsulate business rules and coordinate repository operations.
* Maintain invariants defined in the specification (creator immutability,
  conflict warning calculation, recurrence generation, access control around
  meeting rooms).
* Provide transaction boundaries when the persistence layer supports them.

### Persistence Layer
* Defines repository interfaces that abstract away concrete storage details.
* The initial implementation will target SQLite through a cgo-free driver. The
  specific package and migration helper are **not** implemented yet; they will
  be introduced during Steps 3 and 4 once the design in this document is
  reviewed.
* Enables swapping storage with minimal changes in the upper layers.

### Cross-Cutting Concerns
* **Logging:** Standard library `log/slog` with structured context, instrumented at
  service boundaries.
* **Configuration:** Environment-driven configuration loaded at startup to control
  DSN, HTTP port, security parameters, and feature flags.
* **Error handling:** Use sentinel errors for domain conditions (e.g.
  `ErrUnauthorized`, `ErrConflictDetected`) and wrap unexpected errors with
  contextual information.

## Data Flow Scenarios

### Schedule Creation
1. HTTP handler receives `POST /schedules` request and performs validation.
2. Handler authenticates the user and resolves authorization via
   `ScheduleService`.
3. `ScheduleService` persists the schedule through `ScheduleRepo`, generates
   recurrences via `RecurrenceEngine` if requested, and asks
   `ConflictDetector` for warnings.
4. `ScheduleRepo` writes to SQLite using prepared statements and returns the
   persisted entity.
5. Handler returns the created schedule with any conflict warnings.

### Meeting Room Management
1. Administrator calls `POST/PUT/DELETE /rooms`.
2. `RoomService` enforces admin-only access, then delegates to `RoomRepo`.
3. `RoomRepo` performs the SQL operation and returns the updated model.

### Viewing Calendars
1. Client requests `GET /schedules?participants=alice,bob`.
2. Handler forwards filters to `ScheduleService` which gathers occurrences from
   `ScheduleRepo` and recurrence projections.
3. Application layer materializes view models for the UI, including conflict
   annotations when relevant.

## Deployment Considerations
* Single Go binary containing HTTP server and background jobs.
* SQLite database stored on local disk with periodic backup strategy (out of MVP
  scope but considered in design for future extension).
* Containerization via Docker is anticipated; use multi-stage build to produce a
  minimal image.

## Open Decisions
* Exact HTTP routing structure and request/response DTOs.
* Whether to embed front-end assets in the Go binary or serve them separately.
* Detailed logging/metrics stack (e.g., OpenTelemetry vs. simple logs).

## Implementation Status
* Steps 1 and 2 (architecture and testing strategy) are documented here and in
  `docs/test_strategy.md`.
* No production Go code or database schema has been committed yet. Earlier
  experimental implementations were rolled back to keep the repository focused
  on the agreed planning activities before moving on to Steps 3 and 4.

