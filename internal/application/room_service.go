package application

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/example/enterprise-scheduler/internal/persistence"
)

// RoomRepository captures the persistence operations needed by the service.
type RoomRepository interface {
	CreateRoom(ctx context.Context, room Room) (Room, error)
	GetRoom(ctx context.Context, id string) (Room, error)
	UpdateRoom(ctx context.Context, room Room) (Room, error)
	DeleteRoom(ctx context.Context, id string) error
	ListRooms(ctx context.Context) ([]Room, error)
}

// RoomService orchestrates validation, authorization, and persistence for rooms.
type RoomService struct {
	rooms       RoomRepository
	idGenerator func() string
	now         func() time.Time
}

// NewRoomService constructs a room service with the provided dependencies.
func NewRoomService(rooms RoomRepository, idGenerator func() string, now func() time.Time) *RoomService {
	if idGenerator == nil {
		idGenerator = func() string { return "" }
	}
	if now == nil {
		now = time.Now
	}
	return &RoomService{rooms: rooms, idGenerator: idGenerator, now: now}
}

// CreateRoom validates input and persists a new room for administrators.
func (s *RoomService) CreateRoom(ctx context.Context, params CreateRoomParams) (Room, error) {
	if s == nil {
		return Room{}, fmt.Errorf("RoomService is nil")
	}

	if !params.Principal.IsAdmin {
		return Room{}, ErrUnauthorized
	}

	vErr := validateRoomInput(params.Input)
	if vErr.HasErrors() {
		return Room{}, vErr
	}

	room := Room{
		ID:         s.idGenerator(),
		Name:       strings.TrimSpace(params.Input.Name),
		Location:   strings.TrimSpace(params.Input.Location),
		Capacity:   params.Input.Capacity,
		Facilities: normalizeOptionalString(params.Input.Facilities),
		CreatedAt:  s.now(),
	}
	room.UpdatedAt = room.CreatedAt

	if s.rooms == nil {
		return room, nil
	}

	persisted, err := s.rooms.CreateRoom(ctx, room)
	if err != nil {
		return Room{}, mapRoomRepoError(err)
	}

	return persisted, nil
}

// UpdateRoom validates input and updates an existing room for administrators.
func (s *RoomService) UpdateRoom(ctx context.Context, params UpdateRoomParams) (Room, error) {
	if s == nil {
		return Room{}, fmt.Errorf("RoomService is nil")
	}
	if !params.Principal.IsAdmin {
		return Room{}, ErrUnauthorized
	}
	if s.rooms == nil {
		return Room{}, fmt.Errorf("room repository not configured")
	}

	existing, err := s.rooms.GetRoom(ctx, params.RoomID)
	if err != nil {
		return Room{}, mapRoomRepoError(err)
	}

	vErr := validateRoomInput(params.Input)
	if vErr.HasErrors() {
		return Room{}, vErr
	}

	updated := existing
	updated.Name = strings.TrimSpace(params.Input.Name)
	updated.Location = strings.TrimSpace(params.Input.Location)
	updated.Capacity = params.Input.Capacity
	updated.Facilities = normalizeOptionalString(params.Input.Facilities)
	updated.UpdatedAt = s.now()

	persisted, err := s.rooms.UpdateRoom(ctx, updated)
	if err != nil {
		return Room{}, mapRoomRepoError(err)
	}

	return persisted, nil
}

// DeleteRoom removes an existing room when requested by an administrator.
func (s *RoomService) DeleteRoom(ctx context.Context, principal Principal, roomID string) error {
	if s == nil {
		return fmt.Errorf("RoomService is nil")
	}
	if !principal.IsAdmin {
		return ErrUnauthorized
	}
	if s.rooms == nil {
		return fmt.Errorf("room repository not configured")
	}

	if err := s.rooms.DeleteRoom(ctx, roomID); err != nil {
		return mapRoomRepoError(err)
	}

	return nil
}

// ListRooms returns the catalog of rooms for any authenticated user.
func (s *RoomService) ListRooms(ctx context.Context, principal Principal) ([]Room, error) {
	if s == nil {
		return nil, fmt.Errorf("RoomService is nil")
	}
	if s.rooms == nil {
		return nil, nil
	}

	rooms, err := s.rooms.ListRooms(ctx)
	if err != nil {
		return nil, err
	}

	out := make([]Room, len(rooms))
	copy(out, rooms)

	sort.Slice(out, func(i, j int) bool {
		if strings.EqualFold(out[i].Name, out[j].Name) {
			return out[i].ID < out[j].ID
		}
		return strings.ToLower(out[i].Name) < strings.ToLower(out[j].Name)
	})

	return out, nil
}

func validateRoomInput(input RoomInput) *ValidationError {
	vErr := &ValidationError{}

	if strings.TrimSpace(input.Name) == "" {
		vErr.add("name", "name is required")
	}
	if strings.TrimSpace(input.Location) == "" {
		vErr.add("location", "location is required")
	}
	if input.Capacity <= 0 {
		vErr.add("capacity", "capacity must be positive")
	}

	return vErr
}

func mapRoomRepoError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, ErrNotFound) {
		return ErrNotFound
	}
	if errors.Is(err, persistence.ErrNotFound) {
		return ErrNotFound
	}
	if errors.Is(err, persistence.ErrDuplicate) {
		return ErrAlreadyExists
	}
	if errors.Is(err, persistence.ErrConstraintViolation) {
		vErr := &ValidationError{}
		vErr.add("capacity", "capacity must be positive")
		return vErr
	}
	return err
}

func normalizeOptionalString(value *string) *string {
	if value == nil {
		return nil
	}
	trimmed := strings.TrimSpace(*value)
	if trimmed == "" {
		return nil
	}
	return &trimmed
}
