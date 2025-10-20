package application

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/example/enterprise-scheduler/internal/persistence"
)

type roomRepoStub struct {
	createErr error
	created   Room

	getRoom Room
	getErr  error

	updateErr error
	updated   Room

	deleteErr error
	deletedID string

	list    []Room
	listErr error
}

func (r *roomRepoStub) CreateRoom(ctx context.Context, room Room) (Room, error) {
	if r.createErr != nil {
		return Room{}, r.createErr
	}
	r.created = room
	return room, nil
}

func (r *roomRepoStub) GetRoom(ctx context.Context, id string) (Room, error) {
	if r.getErr != nil {
		return Room{}, r.getErr
	}
	if r.getRoom.ID == "" {
		return Room{}, ErrNotFound
	}
	return r.getRoom, nil
}

func (r *roomRepoStub) UpdateRoom(ctx context.Context, room Room) (Room, error) {
	if r.updateErr != nil {
		return Room{}, r.updateErr
	}
	r.updated = room
	return room, nil
}

func (r *roomRepoStub) DeleteRoom(ctx context.Context, id string) error {
	if r.deleteErr != nil {
		return r.deleteErr
	}
	r.deletedID = id
	return nil
}

func (r *roomRepoStub) ListRooms(ctx context.Context) ([]Room, error) {
	if r.listErr != nil {
		return nil, r.listErr
	}
	if len(r.list) == 0 {
		return nil, nil
	}
	out := make([]Room, len(r.list))
	copy(out, r.list)
	return out, nil
}

func TestRoomService_CreateRoom(t *testing.T) {
	t.Run("requires administrator privileges", func(t *testing.T) {
		svc := NewRoomService(nil, nil, nil)

		_, err := svc.CreateRoom(context.Background(), CreateRoomParams{
			Principal: Principal{IsAdmin: false},
			Input: RoomInput{
				Name:     "Conference Room",
				Location: "Floor 10",
				Capacity: 10,
			},
		})

		if !errors.Is(err, ErrUnauthorized) {
			t.Fatalf("expected ErrUnauthorized, got %v", err)
		}
	})

	t.Run("validates required attributes", func(t *testing.T) {
		svc := NewRoomService(nil, nil, nil)

		_, err := svc.CreateRoom(context.Background(), CreateRoomParams{
			Principal: Principal{IsAdmin: true},
			Input: RoomInput{
				Name:     "   ",
				Location: "",
				Capacity: 0,
			},
		})

		var vErr *ValidationError
		if !errors.As(err, &vErr) {
			t.Fatalf("expected ValidationError, got %v", err)
		}

		if _, ok := vErr.FieldErrors["name"]; !ok {
			t.Fatalf("expected name validation error, got %v", vErr.FieldErrors)
		}
		if _, ok := vErr.FieldErrors["location"]; !ok {
			t.Fatalf("expected location validation error, got %v", vErr.FieldErrors)
		}
		if _, ok := vErr.FieldErrors["capacity"]; !ok {
			t.Fatalf("expected capacity validation error, got %v", vErr.FieldErrors)
		}
	})

	t.Run("persists rooms for administrators", func(t *testing.T) {
		repo := &roomRepoStub{}
		now := time.Date(2024, time.March, 14, 9, 0, 0, 0, time.UTC)
		facility := "  Projector  "
		svc := NewRoomService(repo, func() string { return "room-1" }, func() time.Time { return now })

		created, err := svc.CreateRoom(context.Background(), CreateRoomParams{
			Principal: Principal{IsAdmin: true},
			Input: RoomInput{
				Name:       "  Sakura Hall  ",
				Location:   "  10F  ",
				Capacity:   25,
				Facilities: &facility,
			},
		})
		if err != nil {
			t.Fatalf("expected success, got %v", err)
		}

		if repo.created.ID != "room-1" {
			t.Fatalf("expected repository to receive generated ID, got %q", repo.created.ID)
		}
		if repo.created.Name != "Sakura Hall" {
			t.Fatalf("expected name to be trimmed, got %q", repo.created.Name)
		}
		if repo.created.Location != "10F" {
			t.Fatalf("expected location to be trimmed, got %q", repo.created.Location)
		}
		if repo.created.Capacity != 25 {
			t.Fatalf("expected capacity to be 25, got %d", repo.created.Capacity)
		}
		if repo.created.Facilities == nil || *repo.created.Facilities != "Projector" {
			t.Fatalf("expected facilities to be trimmed, got %v", repo.created.Facilities)
		}
		if !repo.created.CreatedAt.Equal(now) || !repo.created.UpdatedAt.Equal(now) {
			t.Fatalf("expected timestamps to use injected clock, got created=%v updated=%v", repo.created.CreatedAt, repo.created.UpdatedAt)
		}

		if created.ID != "room-1" {
			t.Fatalf("expected returned room to include generated ID, got %q", created.ID)
		}
	})

	t.Run("maps repository errors to sentinel failures", func(t *testing.T) {
		repo := &roomRepoStub{createErr: persistence.ErrDuplicate}
		svc := NewRoomService(repo, nil, nil)

		_, err := svc.CreateRoom(context.Background(), CreateRoomParams{
			Principal: Principal{IsAdmin: true},
			Input: RoomInput{
				Name:     "Conf Room",
				Location: "HQ",
				Capacity: 10,
			},
		})

		if !errors.Is(err, ErrAlreadyExists) {
			t.Fatalf("expected ErrAlreadyExists, got %v", err)
		}
	})
}

func TestRoomService_UpdateRoom(t *testing.T) {
	t.Run("requires administrator privileges", func(t *testing.T) {
		svc := NewRoomService(nil, nil, nil)

		_, err := svc.UpdateRoom(context.Background(), UpdateRoomParams{
			Principal: Principal{IsAdmin: false},
			RoomID:    "room-1",
			Input: RoomInput{
				Name:       "Room",
				Location:   "HQ",
				Capacity:   10,
				Facilities: nil,
			},
		})

		if !errors.Is(err, ErrUnauthorized) {
			t.Fatalf("expected ErrUnauthorized, got %v", err)
		}
	})

	t.Run("validates required attributes", func(t *testing.T) {
		repo := &roomRepoStub{getRoom: Room{ID: "room-1", Name: "Sakura", Location: "10F", Capacity: 20}}
		svc := NewRoomService(repo, nil, nil)

		_, err := svc.UpdateRoom(context.Background(), UpdateRoomParams{
			Principal: Principal{IsAdmin: true},
			RoomID:    "room-1",
			Input: RoomInput{
				Name:     "",
				Location: " ",
				Capacity: -1,
			},
		})

		var vErr *ValidationError
		if !errors.As(err, &vErr) {
			t.Fatalf("expected ValidationError, got %v", err)
		}
		if _, ok := vErr.FieldErrors["name"]; !ok {
			t.Fatalf("expected name validation error, got %v", vErr.FieldErrors)
		}
		if _, ok := vErr.FieldErrors["location"]; !ok {
			t.Fatalf("expected location validation error, got %v", vErr.FieldErrors)
		}
		if _, ok := vErr.FieldErrors["capacity"]; !ok {
			t.Fatalf("expected capacity validation error, got %v", vErr.FieldErrors)
		}
	})

	t.Run("propagates ErrNotFound when the room is missing", func(t *testing.T) {
		repo := &roomRepoStub{getErr: persistence.ErrNotFound}
		svc := NewRoomService(repo, nil, nil)

		_, err := svc.UpdateRoom(context.Background(), UpdateRoomParams{
			Principal: Principal{IsAdmin: true},
			RoomID:    "missing",
			Input: RoomInput{
				Name:       "Room",
				Location:   "HQ",
				Capacity:   10,
				Facilities: nil,
			},
		})

		if !errors.Is(err, ErrNotFound) {
			t.Fatalf("expected ErrNotFound, got %v", err)
		}
	})

	t.Run("persists updated attributes for administrators", func(t *testing.T) {
		existing := Room{ID: "room-1", Name: "Sakura", Location: "10F", Capacity: 20, CreatedAt: time.Now(), UpdatedAt: time.Now()}
		repo := &roomRepoStub{getRoom: existing}
		now := time.Date(2024, time.March, 15, 9, 0, 0, 0, time.UTC)
		facility := "  Whiteboard  "
		svc := NewRoomService(repo, nil, func() time.Time { return now })

		updated, err := svc.UpdateRoom(context.Background(), UpdateRoomParams{
			Principal: Principal{IsAdmin: true},
			RoomID:    "room-1",
			Input: RoomInput{
				Name:       "  Maple ",
				Location:   "  11F",
				Capacity:   30,
				Facilities: &facility,
			},
		})
		if err != nil {
			t.Fatalf("expected success, got %v", err)
		}

		if repo.updated.Name != "Maple" {
			t.Fatalf("expected name to be trimmed, got %q", repo.updated.Name)
		}
		if repo.updated.Location != "11F" {
			t.Fatalf("expected location to be trimmed, got %q", repo.updated.Location)
		}
		if repo.updated.Capacity != 30 {
			t.Fatalf("expected capacity to be updated, got %d", repo.updated.Capacity)
		}
		if repo.updated.Facilities == nil || *repo.updated.Facilities != "Whiteboard" {
			t.Fatalf("expected facilities to be trimmed, got %v", repo.updated.Facilities)
		}
		if !repo.updated.UpdatedAt.Equal(now) {
			t.Fatalf("expected updated timestamp to use injected clock, got %v", repo.updated.UpdatedAt)
		}
		if repo.updated.CreatedAt != existing.CreatedAt {
			t.Fatalf("expected created timestamp to remain unchanged")
		}

		if updated.ID != existing.ID {
			t.Fatalf("expected returned room to include ID, got %q", updated.ID)
		}
	})
}

func TestRoomService_DeleteRoom(t *testing.T) {
	t.Run("requires administrator privileges", func(t *testing.T) {
		svc := NewRoomService(nil, nil, nil)

		err := svc.DeleteRoom(context.Background(), Principal{IsAdmin: false}, "room-1")
		if !errors.Is(err, ErrUnauthorized) {
			t.Fatalf("expected ErrUnauthorized, got %v", err)
		}
	})

	t.Run("propagates ErrNotFound when the room is missing", func(t *testing.T) {
		repo := &roomRepoStub{deleteErr: persistence.ErrNotFound}
		svc := NewRoomService(repo, nil, nil)

		err := svc.DeleteRoom(context.Background(), Principal{IsAdmin: true}, "missing")
		if !errors.Is(err, ErrNotFound) {
			t.Fatalf("expected ErrNotFound, got %v", err)
		}
	})

	t.Run("allows administrators to delete rooms", func(t *testing.T) {
		repo := &roomRepoStub{}
		svc := NewRoomService(repo, nil, nil)

		if err := svc.DeleteRoom(context.Background(), Principal{IsAdmin: true}, "room-1"); err != nil {
			t.Fatalf("expected success, got %v", err)
		}

		if repo.deletedID != "room-1" {
			t.Fatalf("expected repository to receive room ID, got %q", repo.deletedID)
		}
	})
}

func TestRoomService_ListRooms(t *testing.T) {
	t.Run("is accessible to all authenticated employees", func(t *testing.T) {
		rooms := []Room{{ID: "room-1", Name: "A", Location: "1F", Capacity: 5}}
		repo := &roomRepoStub{list: rooms}
		svc := NewRoomService(repo, nil, nil)

		got, err := svc.ListRooms(context.Background(), Principal{UserID: "user-1", IsAdmin: false})
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		if len(got) != 1 || got[0].ID != "room-1" {
			t.Fatalf("expected rooms to be returned, got %v", got)
		}
	})

	t.Run("returns rooms in deterministic order", func(t *testing.T) {
		repo := &roomRepoStub{list: []Room{
			{ID: "room-2", Name: "Beta", Location: "2F", Capacity: 10},
			{ID: "room-3", Name: "alpha", Location: "3F", Capacity: 8},
			{ID: "room-1", Name: "Alpha", Location: "1F", Capacity: 6},
		}}
		svc := NewRoomService(repo, nil, nil)

		got, err := svc.ListRooms(context.Background(), Principal{UserID: "user-1"})
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		if len(got) != 3 {
			t.Fatalf("expected three rooms, got %d", len(got))
		}

		if got[0].ID != "room-1" || got[1].ID != "room-3" || got[2].ID != "room-2" {
			t.Fatalf("expected case-insensitive ordering, got %+v", got)
		}
	})
}

func TestMapRoomRepoError(t *testing.T) {
	unexpected := errors.New("boom")

	tests := map[string]struct {
		err      error
		expected error
	}{
		"nil":                   {err: nil, expected: nil},
		"application not found": {err: ErrNotFound, expected: ErrNotFound},
		"persistence not found": {err: persistence.ErrNotFound, expected: ErrNotFound},
		"duplicate":             {err: persistence.ErrDuplicate, expected: ErrAlreadyExists},
		"constraint":            {err: persistence.ErrConstraintViolation, expected: &ValidationError{}},
		"unexpected":            {err: unexpected, expected: unexpected},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			result := mapRoomRepoError(tc.err)

			switch expected := tc.expected.(type) {
			case nil:
				if result != nil {
					t.Fatalf("expected nil, got %v", result)
				}
			case *ValidationError:
				vErr, ok := result.(*ValidationError)
				if !ok {
					t.Fatalf("expected ValidationError, got %T", result)
				}
				if msg, ok := vErr.FieldErrors["capacity"]; !ok || msg == "" {
					t.Fatalf("expected capacity validation message, got %v", vErr.FieldErrors)
				}
			default:
				if !errors.Is(result, expected) {
					t.Fatalf("expected %v, got %v", expected, result)
				}
			}
		})
	}
}
