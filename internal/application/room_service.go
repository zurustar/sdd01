package application

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
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
	logger      *slog.Logger
}

// NewRoomService constructs a room service with the provided dependencies.
func NewRoomService(rooms RoomRepository, idGenerator func() string, now func() time.Time) *RoomService {
	return NewRoomServiceWithLogger(rooms, idGenerator, now, nil)
}

// NewRoomServiceWithLogger constructs a room service with a specified logger.
func NewRoomServiceWithLogger(rooms RoomRepository, idGenerator func() string, now func() time.Time, logger *slog.Logger) *RoomService {
	if idGenerator == nil {
		idGenerator = func() string { return "" }
	}
	if now == nil {
		now = time.Now
	}
	return &RoomService{rooms: rooms, idGenerator: idGenerator, now: now, logger: defaultLogger(logger)}
}

func (s *RoomService) loggerWith(ctx context.Context, operation string, attrs ...any) *slog.Logger {
	return serviceLogger(ctx, s.logger, "RoomService", operation, attrs...)
}

// CreateRoom validates input and persists a new room for administrators.
func (s *RoomService) CreateRoom(ctx context.Context, params CreateRoomParams) (room Room, err error) {
	if s == nil {
		err = fmt.Errorf("RoomService is nil")
		return
	}

	logger := s.loggerWith(ctx, "CreateRoom",
		"principal_id", params.Principal.UserID,
	)
	defer func() {
		if err != nil {
			logger.ErrorContext(ctx, "failed to create room", "error", err, "error_kind", ErrorKind(err))
			return
		}
		logger.With("room_id", room.ID).InfoContext(ctx, "room created")
	}()

	if !params.Principal.IsAdmin {
		err = ErrUnauthorized
		return
	}

	vErr := validateRoomInput(params.Input)
	if vErr.HasErrors() {
		err = vErr
		return
	}

	room = Room{
		ID:         s.idGenerator(),
		Name:       strings.TrimSpace(params.Input.Name),
		Location:   strings.TrimSpace(params.Input.Location),
		Capacity:   params.Input.Capacity,
		Facilities: normalizeOptionalString(params.Input.Facilities),
		CreatedAt:  s.now(),
	}
	room.UpdatedAt = room.CreatedAt

	if s.rooms == nil {
		return
	}

	var persisted Room
	persisted, err = s.rooms.CreateRoom(ctx, room)
	if err != nil {
		err = mapRoomRepoError(err)
		return
	}

	room = persisted
	return
}

// UpdateRoom validates input and updates an existing room for administrators.
func (s *RoomService) UpdateRoom(ctx context.Context, params UpdateRoomParams) (room Room, err error) {
	if s == nil {
		err = fmt.Errorf("RoomService is nil")
		return
	}
	if !params.Principal.IsAdmin {
		err = ErrUnauthorized
		return
	}
	if s.rooms == nil {
		err = fmt.Errorf("room repository not configured")
		return
	}

	logger := s.loggerWith(ctx, "UpdateRoom",
		"principal_id", params.Principal.UserID,
		"room_id", params.RoomID,
	)
	defer func() {
		if err != nil {
			logger.ErrorContext(ctx, "failed to update room", "error", err, "error_kind", ErrorKind(err))
			return
		}
		logger.With("room_id", room.ID).InfoContext(ctx, "room updated")
	}()

	var existing Room
	existing, err = s.rooms.GetRoom(ctx, params.RoomID)
	if err != nil {
		err = mapRoomRepoError(err)
		return
	}

	vErr := validateRoomInput(params.Input)
	if vErr.HasErrors() {
		err = vErr
		return
	}

	updated := existing
	updated.Name = strings.TrimSpace(params.Input.Name)
	updated.Location = strings.TrimSpace(params.Input.Location)
	updated.Capacity = params.Input.Capacity
	updated.Facilities = normalizeOptionalString(params.Input.Facilities)
	updated.UpdatedAt = s.now()

	room, err = s.rooms.UpdateRoom(ctx, updated)
	if err != nil {
		err = mapRoomRepoError(err)
		return
	}

	return
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

	logger := s.loggerWith(ctx, "DeleteRoom",
		"principal_id", principal.UserID,
		"room_id", roomID,
	)

	if err := s.rooms.DeleteRoom(ctx, roomID); err != nil {
		err = mapRoomRepoError(err)
		logger.ErrorContext(ctx, "failed to delete room", "error", err, "error_kind", ErrorKind(err))
		return err
	}

	logger.InfoContext(ctx, "room deleted")
	return nil
}

// ListRooms returns the catalog of rooms for any authenticated user.
func (s *RoomService) ListRooms(ctx context.Context, principal Principal) (rooms []Room, err error) {
	if s == nil {
		err = fmt.Errorf("RoomService is nil")
		return
	}
	if s.rooms == nil {
		return nil, nil
	}

	logger := s.loggerWith(ctx, "ListRooms",
		"principal_id", principal.UserID,
	)
	defer func() {
		if err != nil {
			logger.ErrorContext(ctx, "failed to list rooms", "error", err, "error_kind", ErrorKind(err))
			return
		}
		logger.With("result_count", len(rooms)).InfoContext(ctx, "rooms listed")
	}()

	var raw []Room
	raw, err = s.rooms.ListRooms(ctx)
	if err != nil {
		return
	}

	rooms = make([]Room, len(raw))
	copy(rooms, raw)

	sort.Slice(rooms, func(i, j int) bool {
		if strings.EqualFold(rooms[i].Name, rooms[j].Name) {
			return rooms[i].ID < rooms[j].ID
		}
		return strings.ToLower(rooms[i].Name) < strings.ToLower(rooms[j].Name)
	})

	return
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
