package application

import "testing"

func TestRoomService_CreateRoom(t *testing.T) {
	t.Parallel()

	t.Run("requires administrator privileges", func(t *testing.T) {
		t.Parallel()
		t.Skip("TODO: expect ErrUnauthorized when non-admin attempts to create a room")
	})

	t.Run("validates required attributes", func(t *testing.T) {
		t.Parallel()
		t.Skip("TODO: ensure CreateRoom checks name, location, and positive capacity")
	})

	t.Run("persists rooms for administrators", func(t *testing.T) {
		t.Parallel()
		t.Skip("TODO: assert CreateRoom succeeds for administrators with valid input")
	})

	t.Run("maps repository errors to sentinel failures", func(t *testing.T) {
		t.Parallel()
		t.Skip("TODO: ensure storage failures convert to domain errors")
	})
}

func TestRoomService_UpdateRoom(t *testing.T) {
	t.Parallel()

	t.Run("requires administrator privileges", func(t *testing.T) {
		t.Parallel()
		t.Skip("TODO: expect ErrUnauthorized when non-admin attempts to update a room")
	})

	t.Run("validates required attributes", func(t *testing.T) {
		t.Parallel()
		t.Skip("TODO: ensure UpdateRoom enforces name, location, and positive capacity")
	})

	t.Run("propagates ErrNotFound when the room is missing", func(t *testing.T) {
		t.Parallel()
		t.Skip("TODO: expect UpdateRoom to surface ErrNotFound from repository")
	})

	t.Run("persists updated attributes for administrators", func(t *testing.T) {
		t.Parallel()
		t.Skip("TODO: assert UpdateRoom writes changes when validation passes")
	})
}

func TestRoomService_DeleteRoom(t *testing.T) {
	t.Parallel()

	t.Run("requires administrator privileges", func(t *testing.T) {
		t.Parallel()
		t.Skip("TODO: expect ErrUnauthorized when non-admin attempts to delete a room")
	})

	t.Run("propagates ErrNotFound when the room is missing", func(t *testing.T) {
		t.Parallel()
		t.Skip("TODO: expect DeleteRoom to surface ErrNotFound from repository")
	})

	t.Run("allows administrators to delete rooms", func(t *testing.T) {
		t.Parallel()
		t.Skip("TODO: assert DeleteRoom succeeds for administrators")
	})
}

func TestRoomService_ListRooms(t *testing.T) {
	t.Parallel()

	t.Run("is accessible to all authenticated employees", func(t *testing.T) {
		t.Parallel()
		t.Skip("TODO: ensure ListRooms allows read access without admin role")
	})

	t.Run("returns rooms in deterministic order", func(t *testing.T) {
		t.Parallel()
		t.Skip("TODO: assert ListRooms sorts by name or identifier predictably")
	})
}
