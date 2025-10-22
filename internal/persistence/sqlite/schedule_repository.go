package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/example/enterprise-scheduler/internal/persistence"
)

// ScheduleRepository implements persistence.ScheduleRepository using SQLite
type ScheduleRepository struct {
	pool   *ConnectionPool
	helper *QueryHelper
	mapper *ErrorMapper
}

// NewScheduleRepository creates a new SQLite schedule repository
func NewScheduleRepository(pool *ConnectionPool) *ScheduleRepository {
	return &ScheduleRepository{
		pool:   pool,
		helper: NewQueryHelper(pool),
		mapper: NewErrorMapper(),
	}
}

// CreateSchedule inserts a new schedule with participants into the database
func (r *ScheduleRepository) CreateSchedule(ctx context.Context, schedule persistence.Schedule) error {
	if schedule.ID == "" {
		return persistence.ErrConstraintViolation
	}
	
	// Validate schedule constraints
	if err := r.validateSchedule(schedule); err != nil {
		return err
	}
	
	// Set timestamps
	now := time.Now().UTC()
	schedule.CreatedAt = now
	schedule.UpdatedAt = now
	
	return r.pool.WithTransaction(ctx, func(tx *sql.Tx) error {
		// Insert the schedule
		query := `
			INSERT INTO schedules (id, title, start_time, end_time, creator_id, room_id, memo, web_conference_url, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		`
		
		var roomID sql.NullString
		if schedule.RoomID != nil {
			roomID.String = *schedule.RoomID
			roomID.Valid = true
		}
		
		var memo sql.NullString
		if schedule.Memo != nil {
			memo.String = *schedule.Memo
			memo.Valid = true
		}
		
		var webConferenceURL sql.NullString
		if schedule.WebConferenceURL != nil {
			webConferenceURL.String = *schedule.WebConferenceURL
			webConferenceURL.Valid = true
		}
		
		_, err := r.helper.ExecTx(tx, query,
			schedule.ID,
			schedule.Title,
			schedule.Start.Format(time.RFC3339),
			schedule.End.Format(time.RFC3339),
			schedule.CreatorID,
			roomID,
			memo,
			webConferenceURL,
			schedule.CreatedAt.Format(time.RFC3339),
			schedule.UpdatedAt.Format(time.RFC3339),
		)
		
		if err != nil {
			return r.mapScheduleError(err)
		}
		
		// Insert participants
		if err := r.insertParticipants(tx, schedule.ID, schedule.Participants); err != nil {
			return err
		}
		
		return nil
	})
}

// UpdateSchedule updates an existing schedule and its participants
func (r *ScheduleRepository) UpdateSchedule(ctx context.Context, schedule persistence.Schedule) error {
	if schedule.ID == "" {
		return persistence.ErrConstraintViolation
	}
	
	// Validate schedule constraints
	if err := r.validateSchedule(schedule); err != nil {
		return err
	}
	
	// Set updated timestamp
	schedule.UpdatedAt = time.Now().UTC()
	
	return r.pool.WithTransaction(ctx, func(tx *sql.Tx) error {
		// Get the current creator_id (it should not be changed)
		var currentCreatorID string
		err := r.helper.QueryRowTx(tx, "SELECT creator_id FROM schedules WHERE id = ?", schedule.ID).Scan(&currentCreatorID)
		if err != nil {
			if err == sql.ErrNoRows {
				return persistence.ErrNotFound
			}
			return r.mapper.MapError(err)
		}
		
		// Use the current creator_id instead of the one from the input
		schedule.CreatorID = currentCreatorID
		
		// Update the schedule
		query := `
			UPDATE schedules 
			SET title = ?, start_time = ?, end_time = ?, room_id = ?, memo = ?, web_conference_url = ?, updated_at = ?
			WHERE id = ?
		`
		
		var roomID sql.NullString
		if schedule.RoomID != nil {
			roomID.String = *schedule.RoomID
			roomID.Valid = true
		}
		
		var memo sql.NullString
		if schedule.Memo != nil {
			memo.String = *schedule.Memo
			memo.Valid = true
		}
		
		var webConferenceURL sql.NullString
		if schedule.WebConferenceURL != nil {
			webConferenceURL.String = *schedule.WebConferenceURL
			webConferenceURL.Valid = true
		}
		
		result, err := r.helper.ExecTx(tx, query,
			schedule.Title,
			schedule.Start.Format(time.RFC3339),
			schedule.End.Format(time.RFC3339),
			roomID,
			memo,
			webConferenceURL,
			schedule.UpdatedAt.Format(time.RFC3339),
			schedule.ID,
		)
		
		if err != nil {
			return r.mapScheduleError(err)
		}
		
		rowsAffected, err := result.RowsAffected()
		if err != nil {
			return fmt.Errorf("failed to get rows affected: %w", err)
		}
		
		if rowsAffected == 0 {
			return persistence.ErrNotFound
		}
		
		// Update participants
		// First, delete existing participants
		_, err = r.helper.ExecTx(tx, "DELETE FROM schedule_participants WHERE schedule_id = ?", schedule.ID)
		if err != nil {
			return r.mapper.MapError(err)
		}
		
		// Then insert new participants
		if err := r.insertParticipants(tx, schedule.ID, schedule.Participants); err != nil {
			return err
		}
		
		return nil
	})
}

// GetSchedule retrieves a schedule by ID from the database
func (r *ScheduleRepository) GetSchedule(ctx context.Context, id string) (persistence.Schedule, error) {
	if id == "" {
		return persistence.Schedule{}, persistence.ErrNotFound
	}
	
	query := `
		SELECT id, title, start_time, end_time, creator_id, room_id, memo, web_conference_url, created_at, updated_at
		FROM schedules
		WHERE id = ?
	`
	
	var schedule persistence.Schedule
	var createdAtStr, updatedAtStr, startTimeStr, endTimeStr string
	var roomID, memo, webConferenceURL sql.NullString
	
	err := r.helper.QueryRow(ctx, query, id).Scan(
		&schedule.ID,
		&schedule.Title,
		&startTimeStr,
		&endTimeStr,
		&schedule.CreatorID,
		&roomID,
		&memo,
		&webConferenceURL,
		&createdAtStr,
		&updatedAtStr,
	)
	
	if err != nil {
		if err == sql.ErrNoRows {
			return persistence.Schedule{}, persistence.ErrNotFound
		}
		return persistence.Schedule{}, r.mapper.MapError(err)
	}
	
	// Handle nullable fields
	if roomID.Valid {
		schedule.RoomID = &roomID.String
	}
	if memo.Valid {
		schedule.Memo = &memo.String
	}
	if webConferenceURL.Valid {
		schedule.WebConferenceURL = &webConferenceURL.String
	}
	
	// Parse timestamps
	if schedule.Start, err = time.Parse(time.RFC3339, startTimeStr); err != nil {
		return persistence.Schedule{}, fmt.Errorf("failed to parse start_time: %w", err)
	}
	if schedule.End, err = time.Parse(time.RFC3339, endTimeStr); err != nil {
		return persistence.Schedule{}, fmt.Errorf("failed to parse end_time: %w", err)
	}
	if schedule.CreatedAt, err = time.Parse(time.RFC3339, createdAtStr); err != nil {
		return persistence.Schedule{}, fmt.Errorf("failed to parse created_at: %w", err)
	}
	if schedule.UpdatedAt, err = time.Parse(time.RFC3339, updatedAtStr); err != nil {
		return persistence.Schedule{}, fmt.Errorf("failed to parse updated_at: %w", err)
	}
	
	// Load participants
	participants, err := r.loadParticipants(ctx, id)
	if err != nil {
		return persistence.Schedule{}, err
	}
	schedule.Participants = participants
	
	return schedule, nil
}

// ListSchedules lists schedules filtered by the provided filter
func (r *ScheduleRepository) ListSchedules(ctx context.Context, filter persistence.ScheduleFilter) ([]persistence.Schedule, error) {
	query, args := r.buildListQuery(filter)
	
	rows, err := r.helper.Query(ctx, query, args...)
	if err != nil {
		return nil, r.mapper.MapError(err)
	}
	defer rows.Close()
	
	var schedules []persistence.Schedule
	
	for rows.Next() {
		var schedule persistence.Schedule
		var createdAtStr, updatedAtStr, startTimeStr, endTimeStr string
		var roomID, memo, webConferenceURL sql.NullString
		
		err := rows.Scan(
			&schedule.ID,
			&schedule.Title,
			&startTimeStr,
			&endTimeStr,
			&schedule.CreatorID,
			&roomID,
			&memo,
			&webConferenceURL,
			&createdAtStr,
			&updatedAtStr,
		)
		
		if err != nil {
			return nil, r.mapper.MapError(err)
		}
		
		// Handle nullable fields
		if roomID.Valid {
			schedule.RoomID = &roomID.String
		}
		if memo.Valid {
			schedule.Memo = &memo.String
		}
		if webConferenceURL.Valid {
			schedule.WebConferenceURL = &webConferenceURL.String
		}
		
		// Parse timestamps
		if schedule.Start, err = time.Parse(time.RFC3339, startTimeStr); err != nil {
			return nil, fmt.Errorf("failed to parse start_time: %w", err)
		}
		if schedule.End, err = time.Parse(time.RFC3339, endTimeStr); err != nil {
			return nil, fmt.Errorf("failed to parse end_time: %w", err)
		}
		if schedule.CreatedAt, err = time.Parse(time.RFC3339, createdAtStr); err != nil {
			return nil, fmt.Errorf("failed to parse created_at: %w", err)
		}
		if schedule.UpdatedAt, err = time.Parse(time.RFC3339, updatedAtStr); err != nil {
			return nil, fmt.Errorf("failed to parse updated_at: %w", err)
		}
		
		// Load participants for each schedule
		participants, err := r.loadParticipants(ctx, schedule.ID)
		if err != nil {
			return nil, err
		}
		schedule.Participants = participants
		
		schedules = append(schedules, schedule)
	}
	
	if err := rows.Err(); err != nil {
		return nil, r.mapper.MapError(err)
	}
	
	return schedules, nil
}

// DeleteSchedule removes a schedule by ID from the database
func (r *ScheduleRepository) DeleteSchedule(ctx context.Context, id string) error {
	if id == "" {
		return persistence.ErrNotFound
	}
	
	return r.pool.WithTransaction(ctx, func(tx *sql.Tx) error {
		// Delete participants first
		_, err := r.helper.ExecTx(tx, "DELETE FROM schedule_participants WHERE schedule_id = ?", id)
		if err != nil {
			return r.mapper.MapError(err)
		}
		
		// Delete recurrences for this schedule
		_, err = r.helper.ExecTx(tx, "DELETE FROM recurrences WHERE schedule_id = ?", id)
		if err != nil {
			return r.mapper.MapError(err)
		}
		
		// Delete the schedule
		result, err := r.helper.ExecTx(tx, "DELETE FROM schedules WHERE id = ?", id)
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

// validateSchedule validates schedule business rules
func (r *ScheduleRepository) validateSchedule(schedule persistence.Schedule) error {
	// Check time constraints
	if schedule.End.Before(schedule.Start) || schedule.End.Equal(schedule.Start) {
		return persistence.ErrConstraintViolation
	}
	
	// Normalize times to UTC
	schedule.Start = schedule.Start.UTC()
	schedule.End = schedule.End.UTC()
	
	return nil
}

// insertParticipants inserts participants for a schedule within a transaction
func (r *ScheduleRepository) insertParticipants(tx *sql.Tx, scheduleID string, participants []string) error {
	if len(participants) == 0 {
		return nil
	}
	
	// Remove duplicates and empty strings
	uniqueParticipants := make(map[string]struct{})
	for _, participant := range participants {
		participant = strings.TrimSpace(participant)
		if participant != "" {
			uniqueParticipants[participant] = struct{}{}
		}
	}
	
	// Insert each unique participant
	for participant := range uniqueParticipants {
		_, err := r.helper.ExecTx(tx, 
			"INSERT INTO schedule_participants (schedule_id, user_id) VALUES (?, ?)",
			scheduleID, participant)
		if err != nil {
			return r.mapper.MapError(err)
		}
	}
	
	return nil
}

// loadParticipants loads participants for a schedule
func (r *ScheduleRepository) loadParticipants(ctx context.Context, scheduleID string) ([]string, error) {
	query := `
		SELECT user_id 
		FROM schedule_participants 
		WHERE schedule_id = ?
		ORDER BY user_id ASC
	`
	
	rows, err := r.helper.Query(ctx, query, scheduleID)
	if err != nil {
		return nil, r.mapper.MapError(err)
	}
	defer rows.Close()
	
	var participants []string
	
	for rows.Next() {
		var userID string
		if err := rows.Scan(&userID); err != nil {
			return nil, r.mapper.MapError(err)
		}
		participants = append(participants, userID)
	}
	
	if err := rows.Err(); err != nil {
		return nil, r.mapper.MapError(err)
	}
	
	return participants, nil
}

// buildListQuery builds the SQL query for listing schedules with filters
func (r *ScheduleRepository) buildListQuery(filter persistence.ScheduleFilter) (string, []interface{}) {
	baseQuery := `
		SELECT DISTINCT s.id, s.title, s.start_time, s.end_time, s.creator_id, s.room_id, s.memo, s.web_conference_url, s.created_at, s.updated_at
		FROM schedules s
	`
	
	var conditions []string
	var args []interface{}
	
	// Add participant filter if specified
	if len(filter.ParticipantIDs) > 0 {
		baseQuery += " LEFT JOIN schedule_participants sp ON s.id = sp.schedule_id"
		
		// Create placeholders for participant IDs
		placeholders := make([]string, len(filter.ParticipantIDs))
		for i, participantID := range filter.ParticipantIDs {
			placeholders[i] = "?"
			args = append(args, participantID)
		}
		
		// Include schedules where the user is a participant OR the creator
		participantCondition := fmt.Sprintf("(sp.user_id IN (%s) OR s.creator_id IN (%s))", 
			strings.Join(placeholders, ","), strings.Join(placeholders, ","))
		conditions = append(conditions, participantCondition)
		
		// Add the participant IDs again for the creator condition
		for _, participantID := range filter.ParticipantIDs {
			args = append(args, participantID)
		}
	}
	
	// Add time range filters
	if filter.StartsAfter != nil {
		conditions = append(conditions, "s.end_time > ?")
		args = append(args, filter.StartsAfter.UTC().Format(time.RFC3339))
	}
	
	if filter.EndsBefore != nil {
		conditions = append(conditions, "s.start_time < ?")
		args = append(args, filter.EndsBefore.UTC().Format(time.RFC3339))
	}
	
	// Add WHERE clause if there are conditions
	if len(conditions) > 0 {
		baseQuery += " WHERE " + strings.Join(conditions, " AND ")
	}
	
	// Add ordering
	baseQuery += " ORDER BY s.start_time ASC, s.id ASC"
	
	return baseQuery, args
}

// mapScheduleError maps SQLite errors to appropriate persistence errors for schedule operations
func (r *ScheduleRepository) mapScheduleError(err error) error {
	if err == nil {
		return nil
	}
	
	errStr := err.Error()
	
	// Handle unique constraint violations
	if containsAny(errStr, []string{"UNIQUE constraint failed"}) {
		if containsAny(errStr, []string{"schedules.id", "PRIMARY KEY"}) {
			return persistence.ErrDuplicate
		}
		return persistence.ErrDuplicate
	}
	
	// Handle foreign key violations
	if containsAny(errStr, []string{"FOREIGN KEY constraint failed"}) {
		return persistence.ErrForeignKeyViolation
	}
	
	// Handle check constraint violations
	if containsAny(errStr, []string{"CHECK constraint failed"}) {
		return persistence.ErrConstraintViolation
	}
	
	// Use the general error mapper for other cases
	return r.mapper.MapError(err)
}