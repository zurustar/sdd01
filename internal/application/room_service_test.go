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
}

func TestRoomService_DeleteRoom(t *testing.T) {
	t.Parallel()

	t.Run("requires administrator privileges", func(t *testing.T) {
		t.Parallel()
		t.Skip("TODO: expect ErrUnauthorized when non-admin attempts to delete a room")
	})
}

func TestRoomService_ListRooms(t *testing.T) {
	t.Parallel()

	t.Run("is accessible to all authenticated employees", func(t *testing.T) {
		t.Parallel()
		t.Skip("TODO: ensure ListRooms allows read access without admin role")
	})
}
