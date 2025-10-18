# Pending Work Before Step 3

This list captures the outstanding tasks that must be completed during Steps 1 and 2 (test-first development and implementation/refinement) before advancing to Step 3 (documentation and review).

## Test Scaffolding (Step 1)
- [ ] Flesh out failing test skeletons for the Schedule application service methods (`CreateSchedule`, `UpdateSchedule`, `ListSchedules`, `DeleteSchedule`).
- [ ] Add unit-test scaffolds covering `RoomService` CRUD behavior, including administrator-only access constraints.
- [ ] Introduce recurrence engine test outlines that describe weekday selection, timezone handling, and generated occurrence linking.
- [ ] Define persistence adapter test scaffolds (repositories for schedules, rooms, and authentication data), including integration test placeholders using SQLite fixtures.

## Implementations to Unblock Green Phase (Step 2)
- [ ] Replace the `DetectConflicts` panic with the real conflict-detection algorithm driven by the pending tests.
- [ ] Implement the application services (`ScheduleService`, `RoomService`, authentication service) as described in the specification, aligning signatures with the test scaffolding.
- [ ] Implement the persistence layer (`internal/persistence` package) with repositories and migration helpers to satisfy integration tests.
- [ ] Provide recurrence engine logic that satisfies the outlined recurrence tests.

## Supporting Infrastructure
- [ ] Create reusable deterministic fixtures/builders in `internal/testfixtures` to support the upcoming tests.
- [ ] Establish a temporary SQLite helper for integration tests (migrations, cleanup) referenced by the persistence test scaffolding.
- [ ] Introduce dependency injection wiring (even minimal) so that application services can be instantiated in tests without production infrastructure.

## Status Tracking
- Update this checklist as work progresses to ensure Step 3 only begins after the above items are implemented and all tests are green.
