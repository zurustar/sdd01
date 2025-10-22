package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/example/enterprise-scheduler/internal/persistence"
)

// RecurrenceRepository implements persistence.RecurrenceRepository using SQLite
type RecurrenceRepository struct {
	pool   *ConnectionPool
	helper *QueryHelper
	mapper *ErrorMapper
}

// NewRecurrenceRepository creates a new SQLite recurrence repository
func NewRecurrenceRepository(pool *ConnectionPool) *RecurrenceRepository {
	return &RecurrenceRepository{
		pool:   pool,
		helper: NewQueryHelper(pool),
		mapper: NewErrorMapper(),
	}
}

// UpsertRecurrence creates or updates a recurrence rule
func (r *RecurrenceRepository) UpsertRecurrence(ctx context.Context, rule persistence.RecurrenceRule) error {
	if rule.ID == "" {
		return persistence.ErrConstraintViolation
	}
	if rule.ScheduleID == "" {
		return persistence.ErrConstraintViolation
	}
	
	// Validate recurrence rule
	if err := r.validateRecurrence(rule); err != nil {
		return err
	}
	
	// Normalize times to UTC
	rule.StartsOn = rule.StartsOn.UTC()
	if rule.EndsOn != nil {
		endsOn := rule.EndsOn.UTC()
		rule.EndsOn = &endsOn
	}
	
	// Set timestamps
	now := time.Now().UTC()
	rule.UpdatedAt = now
	
	return r.pool.WithTransaction(ctx, func(tx *sql.Tx) error {
		// Check if recurrence already exists
		var existingCreatedAt sql.NullString
		err := r.helper.QueryRowTx(tx, "SELECT created_at FROM recurrences WHERE id = ?", rule.ID).Scan(&existingCreatedAt)
		
		if err != nil && err != sql.ErrNoRows {
			return r.mapper.MapError(err)
		}
		
		// Set created_at timestamp
		if existingCreatedAt.Valid {
			// Update existing recurrence - keep original created_at
			if rule.CreatedAt, err = time.Parse(time.RFC3339, existingCreatedAt.String); err != nil {
				return fmt.Errorf("failed to parse existing created_at: %w", err)
			}
		} else {
			// Create new recurrence
			rule.CreatedAt = now
		}
		
		// Encode weekdays as bitmask
		weekdayMask := encodeWeekdays(rule.Weekdays)
		
		// Prepare nullable fields
		var endsOn sql.NullString
		if rule.EndsOn != nil {
			endsOn.String = rule.EndsOn.Format(time.RFC3339)
			endsOn.Valid = true
		}
		
		// Upsert the recurrence rule
		query := `
			INSERT OR REPLACE INTO recurrences 
			(id, schedule_id, frequency, interval_value, weekdays, starts_on, ends_on, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		`
		
		_, err = r.helper.ExecTx(tx, query,
			rule.ID,
			rule.ScheduleID,
			rule.Frequency,
			rule.Frequency, // Use frequency as interval_value since there's no separate Interval field
			weekdayMask,
			rule.StartsOn.Format(time.RFC3339),
			endsOn,
			rule.CreatedAt.Format(time.RFC3339),
			rule.UpdatedAt.Format(time.RFC3339),
		)
		
		if err != nil {
			return r.mapRecurrenceError(err)
		}
		
		return nil
	})
}

// ListRecurrencesForSchedule lists recurrence rules for a schedule ordered by creation time
func (r *RecurrenceRepository) ListRecurrencesForSchedule(ctx context.Context, scheduleID string) ([]persistence.RecurrenceRule, error) {
	if scheduleID == "" {
		return []persistence.RecurrenceRule{}, nil
	}
	
	query := `
		SELECT id, schedule_id, frequency, interval_value, weekdays, starts_on, ends_on, created_at, updated_at
		FROM recurrences
		WHERE schedule_id = ?
		ORDER BY created_at ASC, id ASC
	`
	
	rows, err := r.helper.Query(ctx, query, scheduleID)
	if err != nil {
		return nil, r.mapper.MapError(err)
	}
	defer rows.Close()
	
	var rules []persistence.RecurrenceRule
	
	for rows.Next() {
		var rule persistence.RecurrenceRule
		var createdAtStr, updatedAtStr, startsOnStr string
		var endsOn sql.NullString
		var weekdayMask int64
		var intervalValue int // We'll ignore this since the model doesn't have Interval field
		
		err := rows.Scan(
			&rule.ID,
			&rule.ScheduleID,
			&rule.Frequency,
			&intervalValue, // Read but ignore
			&weekdayMask,
			&startsOnStr,
			&endsOn,
			&createdAtStr,
			&updatedAtStr,
		)
		
		if err != nil {
			return nil, r.mapper.MapError(err)
		}
		
		// Decode weekdays from bitmask
		rule.Weekdays = decodeWeekdays(weekdayMask)
		
		// Handle nullable ends_on field
		if endsOn.Valid {
			if rule.EndsOn, err = parseTimePtr(endsOn.String); err != nil {
				return nil, fmt.Errorf("failed to parse ends_on: %w", err)
			}
		}
		
		// Parse timestamps
		if rule.StartsOn, err = time.Parse(time.RFC3339, startsOnStr); err != nil {
			return nil, fmt.Errorf("failed to parse starts_on: %w", err)
		}
		if rule.CreatedAt, err = time.Parse(time.RFC3339, createdAtStr); err != nil {
			return nil, fmt.Errorf("failed to parse created_at: %w", err)
		}
		if rule.UpdatedAt, err = time.Parse(time.RFC3339, updatedAtStr); err != nil {
			return nil, fmt.Errorf("failed to parse updated_at: %w", err)
		}
		
		rules = append(rules, rule)
	}
	
	if err := rows.Err(); err != nil {
		return nil, r.mapper.MapError(err)
	}
	
	return rules, nil
}

// DeleteRecurrence deletes a recurrence by ID
func (r *RecurrenceRepository) DeleteRecurrence(ctx context.Context, id string) error {
	if id == "" {
		return persistence.ErrNotFound
	}
	
	query := "DELETE FROM recurrences WHERE id = ?"
	
	result, err := r.helper.Exec(ctx, query, id)
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
}

// DeleteRecurrencesForSchedule deletes all recurrences for a schedule
func (r *RecurrenceRepository) DeleteRecurrencesForSchedule(ctx context.Context, scheduleID string) error {
	if scheduleID == "" {
		return nil
	}
	
	query := "DELETE FROM recurrences WHERE schedule_id = ?"
	
	_, err := r.helper.Exec(ctx, query, scheduleID)
	if err != nil {
		return r.mapper.MapError(err)
	}
	
	return nil
}

// validateRecurrence validates recurrence rule business rules
func (r *RecurrenceRepository) validateRecurrence(rule persistence.RecurrenceRule) error {
	// Check that ends_on is after starts_on if specified
	if rule.EndsOn != nil && rule.EndsOn.Before(rule.StartsOn) {
		return persistence.ErrConstraintViolation
	}
	
	// Validate frequency is positive
	if rule.Frequency <= 0 {
		return persistence.ErrConstraintViolation
	}
	
	return nil
}

// mapRecurrenceError maps SQLite errors to appropriate persistence errors for recurrence operations
func (r *RecurrenceRepository) mapRecurrenceError(err error) error {
	if err == nil {
		return nil
	}
	
	errStr := err.Error()
	
	// Handle unique constraint violations
	if containsAny(errStr, []string{"UNIQUE constraint failed"}) {
		if containsAny(errStr, []string{"recurrences.id", "PRIMARY KEY"}) {
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

// parseTimePtr parses a time string and returns a pointer to the time
func parseTimePtr(timeStr string) (*time.Time, error) {
	t, err := time.Parse(time.RFC3339, timeStr)
	if err != nil {
		return nil, err
	}
	return &t, nil
}

// encodeWeekdays encodes weekdays as a bitmask for storage
func encodeWeekdays(weekdays []time.Weekday) int64 {
	var mask int64
	for _, day := range weekdays {
		if day >= time.Sunday && day <= time.Saturday {
			mask |= 1 << uint(day)
		}
	}
	return mask
}

// decodeWeekdays decodes weekdays from a bitmask
func decodeWeekdays(mask int64) []time.Weekday {
	var weekdays []time.Weekday
	for day := time.Sunday; day <= time.Saturday; day++ {
		if mask&(1<<uint(day)) != 0 {
			weekdays = append(weekdays, day)
		}
	}
	return weekdays
}