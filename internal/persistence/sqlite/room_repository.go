package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/example/enterprise-scheduler/internal/persistence"
)

// RoomRepository implements persistence.RoomRepository using SQLite
type RoomRepository struct {
	pool   *ConnectionPool
	helper *QueryHelper
	mapper *ErrorMapper
}

// NewRoomRepository creates a new SQLite room repository
func NewRoomRepository(pool *ConnectionPool) *RoomRepository {
	return &RoomRepository{
		pool:   pool,
		helper: NewQueryHelper(pool),
		mapper: NewErrorMapper(),
	}
}

// CreateRoom inserts a new room into the database
func (r *RoomRepository) CreateRoom(ctx context.Context, room persistence.Room) error {
	if room.ID == "" {
		return persistence.ErrConstraintViolation
	}
	if room.Capacity <= 0 {
		return persistence.ErrConstraintViolation
	}
	
	// Set timestamps
	now := time.Now().UTC()
	room.CreatedAt = now
	room.UpdatedAt = now
	
	query := `
		INSERT INTO rooms (id, name, capacity, location, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?)
	`
	
	_, err := r.helper.Exec(ctx, query,
		room.ID,
		room.Name,
		room.Capacity,
		room.Location,
		room.CreatedAt.Format(time.RFC3339),
		room.UpdatedAt.Format(time.RFC3339),
	)
	
	if err != nil {
		return r.mapRoomError(err)
	}
	
	return nil
}

// UpdateRoom updates an existing room in the database
func (r *RoomRepository) UpdateRoom(ctx context.Context, room persistence.Room) error {
	if room.ID == "" {
		return persistence.ErrConstraintViolation
	}
	if room.Capacity <= 0 {
		return persistence.ErrConstraintViolation
	}
	
	// Set updated timestamp
	room.UpdatedAt = time.Now().UTC()
	
	query := `
		UPDATE rooms 
		SET name = ?, capacity = ?, location = ?, updated_at = ?
		WHERE id = ?
	`
	
	result, err := r.helper.Exec(ctx, query,
		room.Name,
		room.Capacity,
		room.Location,
		room.UpdatedAt.Format(time.RFC3339),
		room.ID,
	)
	
	if err != nil {
		return r.mapRoomError(err)
	}
	
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}
	
	if rowsAffected == 0 {
		return persistence.ErrNotFound
	}
	
	return nil
}

// GetRoom retrieves a room by ID from the database
func (r *RoomRepository) GetRoom(ctx context.Context, id string) (persistence.Room, error) {
	if id == "" {
		return persistence.Room{}, persistence.ErrNotFound
	}
	
	query := `
		SELECT id, name, capacity, location, created_at, updated_at
		FROM rooms
		WHERE id = ?
	`
	
	var room persistence.Room
	var createdAtStr, updatedAtStr string
	var location sql.NullString
	
	err := r.helper.QueryRow(ctx, query, id).Scan(
		&room.ID,
		&room.Name,
		&room.Capacity,
		&location,
		&createdAtStr,
		&updatedAtStr,
	)
	
	if err != nil {
		if err == sql.ErrNoRows {
			return persistence.Room{}, persistence.ErrNotFound
		}
		return persistence.Room{}, r.mapper.MapError(err)
	}
	
	// Handle nullable location field
	if location.Valid {
		room.Location = location.String
	}
	
	// Parse timestamps
	if room.CreatedAt, err = time.Parse(time.RFC3339, createdAtStr); err != nil {
		return persistence.Room{}, fmt.Errorf("failed to parse created_at: %w", err)
	}
	if room.UpdatedAt, err = time.Parse(time.RFC3339, updatedAtStr); err != nil {
		return persistence.Room{}, fmt.Errorf("failed to parse updated_at: %w", err)
	}
	
	return room, nil
}

// ListRooms returns all rooms ordered by name then ID
func (r *RoomRepository) ListRooms(ctx context.Context) ([]persistence.Room, error) {
	query := `
		SELECT id, name, capacity, location, created_at, updated_at
		FROM rooms
		ORDER BY name ASC, id ASC
	`
	
	rows, err := r.helper.Query(ctx, query)
	if err != nil {
		return nil, r.mapper.MapError(err)
	}
	defer rows.Close()
	
	var rooms []persistence.Room
	
	for rows.Next() {
		var room persistence.Room
		var createdAtStr, updatedAtStr string
		var location sql.NullString
		
		err := rows.Scan(
			&room.ID,
			&room.Name,
			&room.Capacity,
			&location,
			&createdAtStr,
			&updatedAtStr,
		)
		
		if err != nil {
			return nil, r.mapper.MapError(err)
		}
		
		// Handle nullable location field
		if location.Valid {
			room.Location = location.String
		}
		
		// Parse timestamps
		if room.CreatedAt, err = time.Parse(time.RFC3339, createdAtStr); err != nil {
			return nil, fmt.Errorf("failed to parse created_at: %w", err)
		}
		if room.UpdatedAt, err = time.Parse(time.RFC3339, updatedAtStr); err != nil {
			return nil, fmt.Errorf("failed to parse updated_at: %w", err)
		}
		
		rooms = append(rooms, room)
	}
	
	if err := rows.Err(); err != nil {
		return nil, r.mapper.MapError(err)
	}
	
	return rooms, nil
}

// DeleteRoom removes a room by ID from the database
func (r *RoomRepository) DeleteRoom(ctx context.Context, id string) error {
	if id == "" {
		return persistence.ErrNotFound
	}
	
	return r.pool.WithTransaction(ctx, func(tx *sql.Tx) error {
		// Update schedules that reference this room to have no room
		_, err := r.helper.ExecTx(tx, "UPDATE schedules SET room_id = NULL WHERE room_id = ?", id)
		if err != nil {
			return r.mapper.MapError(err)
		}
		
		// Delete the room
		result, err := r.helper.ExecTx(tx, "DELETE FROM rooms WHERE id = ?", id)
		if err != nil {
			return r.mapper.MapError(err)
		}
		
		rowsAffected, err := result.RowsAffected()
		if err != nil {
			return fmt.Errorf("failed to get rows affected: %w", err)
		}
		
		if rowsAffected == 0 {
			return persistence.ErrNotFound
		}
		
		return nil
	})
}

// mapRoomError maps SQLite errors to appropriate persistence errors for room operations
func (r *RoomRepository) mapRoomError(err error) error {
	if err == nil {
		return nil
	}
	
	errStr := err.Error()
	
	// Handle unique constraint violations
	if containsAny(errStr, []string{"UNIQUE constraint failed"}) {
		if containsAny(errStr, []string{"rooms.name"}) {
			return persistence.ErrDuplicate
		}
		if containsAny(errStr, []string{"rooms.id", "PRIMARY KEY"}) {
			return persistence.ErrDuplicate
		}
		return persistence.ErrDuplicate
	}
	
	// Handle check constraint violations (e.g., capacity > 0)
	if containsAny(errStr, []string{"CHECK constraint failed"}) {
		return persistence.ErrConstraintViolation
	}
	
	// Use the general error mapper for other cases
	return r.mapper.MapError(err)
}