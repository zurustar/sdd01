package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/example/enterprise-scheduler/internal/application"
	"github.com/example/enterprise-scheduler/internal/config"
	httptransport "github.com/example/enterprise-scheduler/internal/http"
	"github.com/example/enterprise-scheduler/internal/persistence"
	"github.com/example/enterprise-scheduler/internal/persistence/sqlite"
	"github.com/example/enterprise-scheduler/internal/persistence/sqlite/migration"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	cfg, err := config.Load()
	if err != nil {
		logger.Error("failed to load configuration", "error", err)
		os.Exit(1)
	}

	storage, err := sqlite.Open(cfg.SQLiteDSN)
	if err != nil {
		logger.Error("failed to open storage", "error", err)
		os.Exit(1)
	}
	defer func() {
		if cerr := storage.Close(); cerr != nil {
			logger.Error("failed to close storage", "error", cerr)
		}
	}()

	if err := runDatabaseMigrations(ctx, cfg.SQLiteDSN, logger); err != nil {
		logger.Error("failed to apply migrations", "error", err)
		os.Exit(1)
	}

	idGenerator := func() string { return randomHex(16) }
	tokenGenerator := func() string { return randomHex(32) }
	now := time.Now

	userRepo := newUserRepositoryAdapter(storage)
	roomRepo := newRoomRepositoryAdapter(storage)
	scheduleRepo := newScheduleRepositoryAdapter(storage)
	userDirectory := newUserDirectoryAdapter(storage)
	roomCatalog := newRoomCatalogAdapter(storage)
	recurrenceRepo := newRecurrenceRepositoryAdapter(storage, idGenerator)
	sessionRepo := newSessionRepositoryAdapter(storage)
	credentialStore := newCredentialStoreAdapter(storage)

	scheduleService := application.NewScheduleServiceWithLogger(scheduleRepo, userDirectory, roomCatalog, recurrenceRepo, idGenerator, now, logger)
	roomService := application.NewRoomServiceWithLogger(roomRepo, idGenerator, now, logger)
	userService := application.NewUserServiceWithLogger(userRepo, idGenerator, now, logger)
	authService := application.NewAuthServiceWithLogger(credentialStore, sessionRepo, nil, tokenGenerator, now, cfg.SessionTTL, logger)

	authHandler := httptransport.NewAuthHandler(authService, logger)
	userHandler := httptransport.NewUserHandler(userService, logger)
	roomHandler := httptransport.NewRoomHandler(roomService, logger)
	scheduleHandler := httptransport.NewScheduleHandler(scheduleService, logger)

	router := httptransport.NewRouter(httptransport.RouterConfig{
		Auth:      authHandler,
		Users:     userHandler,
		Rooms:     roomHandler,
		Schedules: scheduleHandler,
	})

	protected := httptransport.RequireSession(authService, logger)(router)
	handler := httptransport.RequestLogger(logger)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost && strings.EqualFold(r.URL.Path, "/sessions") {
			router.ServeHTTP(w, r)
			return
		}
		protected.ServeHTTP(w, r)
	}))

	server := &http.Server{
		Addr:              fmt.Sprintf(":%d", cfg.HTTPPort),
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := server.Shutdown(shutdownCtx); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("failed to shutdown server", "error", err)
		}
	}()

	logger.Info("scheduler API listening", "addr", server.Addr)
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		logger.Error("server encountered error", "error", err)
		os.Exit(1)
	}
}

func randomHex(bytes int) string {
	if bytes <= 0 {
		bytes = 16
	}
	buf := make([]byte, bytes)
	if _, err := io.ReadFull(rand.Reader, buf); err != nil {
		return fmt.Sprintf("fallback-%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(buf)
}

func runDatabaseMigrations(ctx context.Context, databasePath string, logger *slog.Logger) error {
	logger.Info("initializing database migration system")
	
	// Configure SQLite connection for migrations
	sqliteConfig := migration.DefaultSQLiteConfig(databasePath)
	connectionManager := migration.NewConnectionManager(sqliteConfig)
	
	// Configure migration settings
	migrationConfig := migration.DefaultMigrationConfig("internal/persistence/sqlite/migrations")
	
	// Validate migration configuration
	if err := migration.ValidateMigrationConfig(migrationConfig); err != nil {
		logger.Error("invalid migration configuration", "error", err)
		return fmt.Errorf("migration configuration validation failed: %w", err)
	}
	
	// Get database connection
	db, err := connectionManager.GetConnection()
	if err != nil {
		logger.Error("failed to establish database connection for migrations", "error", err)
		return fmt.Errorf("failed to get database connection: %w", err)
	}
	defer func() {
		if cerr := db.Close(); cerr != nil {
			logger.Error("failed to close migration database connection", "error", cerr)
		}
	}()
	
	// Initialize migration components
	scanner := migration.NewFileScanner()
	executor := migration.NewSQLiteExecutor(db)
	migrationManager := migration.NewMigrationManager(scanner, executor, migrationConfig.MigrationDir)
	
	// Log migration system initialization
	logger.Info("migration system initialized", 
		"migration_dir", migrationConfig.MigrationDir,
		"database_path", databasePath)
	
	// Log current schema version before migration
	logger.Info("checking current database schema version")
	if err := migrationManager.LogCurrentSchemaVersion(ctx); err != nil {
		logger.Warn("could not determine current schema version", "error", err)
	}
	
	// Check and log pending migrations
	logger.Info("scanning for pending migrations")
	pendingMigrations, err := migrationManager.GetPendingMigrations(ctx)
	if err != nil {
		logger.Error("failed to scan for pending migrations", "error", err)
		return fmt.Errorf("failed to get pending migrations: %w", err)
	}
	
	if len(pendingMigrations) == 0 {
		logger.Info("database schema is up to date - no migrations pending")
		return nil
	}
	
	// Log migration execution progress
	logger.Info("migration execution starting", 
		"pending_count", len(pendingMigrations))
	
	for i, migration := range pendingMigrations {
		logger.Info("migration queued for execution", 
			"sequence", i+1,
			"total", len(pendingMigrations),
			"version", migration.Version,
			"description", migration.Description)
	}
	
	// Execute migrations with comprehensive error handling
	migrationStartTime := time.Now()
	logger.Info("executing database migrations")
	
	if err := migrationManager.RunMigrations(ctx); err != nil {
		logger.Error("migration execution failed", "error", err)
		return fmt.Errorf("migration execution failed: %w", err)
	}
	
	migrationDuration := time.Since(migrationStartTime)
	
	// Log successful completion with final schema version
	logger.Info("database migrations completed successfully", 
		"execution_time", migrationDuration,
		"migrations_applied", len(pendingMigrations))
	
	// Log final schema version
	logger.Info("verifying final database schema version")
	if err := migrationManager.LogCurrentSchemaVersion(ctx); err != nil {
		logger.Warn("could not verify final schema version", "error", err)
	}
	
	return nil
}

type userRepositoryAdapter struct {
	repo persistence.UserRepository
}

func newUserRepositoryAdapter(repo persistence.UserRepository) *userRepositoryAdapter {
	return &userRepositoryAdapter{repo: repo}
}

func (a *userRepositoryAdapter) CreateUser(ctx context.Context, user application.User) (application.User, error) {
	password := user.ID
	if password == "" {
		password = randomHex(12)
	}
	if err := a.repo.CreateUser(ctx, toPersistenceUser(user, password)); err != nil {
		return application.User{}, err
	}
	stored, err := a.repo.GetUser(ctx, user.ID)
	if err != nil {
		return application.User{}, err
	}
	return toApplicationUser(stored), nil
}

func (a *userRepositoryAdapter) GetUser(ctx context.Context, id string) (application.User, error) {
	stored, err := a.repo.GetUser(ctx, id)
	if err != nil {
		return application.User{}, err
	}
	return toApplicationUser(stored), nil
}

func (a *userRepositoryAdapter) UpdateUser(ctx context.Context, user application.User) (application.User, error) {
	current, err := a.repo.GetUser(ctx, user.ID)
	if err != nil {
		return application.User{}, err
	}
	if err := a.repo.UpdateUser(ctx, toPersistenceUser(user, current.PasswordHash)); err != nil {
		return application.User{}, err
	}
	stored, err := a.repo.GetUser(ctx, user.ID)
	if err != nil {
		return application.User{}, err
	}
	return toApplicationUser(stored), nil
}

func (a *userRepositoryAdapter) DeleteUser(ctx context.Context, id string) error {
	return a.repo.DeleteUser(ctx, id)
}

func (a *userRepositoryAdapter) ListUsers(ctx context.Context) ([]application.User, error) {
	models, err := a.repo.ListUsers(ctx)
	if err != nil {
		return nil, err
	}
	if len(models) == 0 {
		return nil, nil
	}
	users := make([]application.User, 0, len(models))
	for _, model := range models {
		users = append(users, toApplicationUser(model))
	}
	return users, nil
}

type roomRepositoryAdapter struct {
	repo persistence.RoomRepository
}

func newRoomRepositoryAdapter(repo persistence.RoomRepository) *roomRepositoryAdapter {
	return &roomRepositoryAdapter{repo: repo}
}

func (a *roomRepositoryAdapter) CreateRoom(ctx context.Context, room application.Room) (application.Room, error) {
	if err := a.repo.CreateRoom(ctx, toPersistenceRoom(room)); err != nil {
		return application.Room{}, err
	}
	stored, err := a.repo.GetRoom(ctx, room.ID)
	if err != nil {
		return application.Room{}, err
	}
	return toApplicationRoom(stored), nil
}

func (a *roomRepositoryAdapter) GetRoom(ctx context.Context, id string) (application.Room, error) {
	stored, err := a.repo.GetRoom(ctx, id)
	if err != nil {
		return application.Room{}, err
	}
	return toApplicationRoom(stored), nil
}

func (a *roomRepositoryAdapter) UpdateRoom(ctx context.Context, room application.Room) (application.Room, error) {
	if err := a.repo.UpdateRoom(ctx, toPersistenceRoom(room)); err != nil {
		return application.Room{}, err
	}
	stored, err := a.repo.GetRoom(ctx, room.ID)
	if err != nil {
		return application.Room{}, err
	}
	return toApplicationRoom(stored), nil
}

func (a *roomRepositoryAdapter) DeleteRoom(ctx context.Context, id string) error {
	return a.repo.DeleteRoom(ctx, id)
}

func (a *roomRepositoryAdapter) ListRooms(ctx context.Context) ([]application.Room, error) {
	models, err := a.repo.ListRooms(ctx)
	if err != nil {
		return nil, err
	}
	if len(models) == 0 {
		return nil, nil
	}
	rooms := make([]application.Room, 0, len(models))
	for _, model := range models {
		rooms = append(rooms, toApplicationRoom(model))
	}
	return rooms, nil
}

type scheduleRepositoryAdapter struct {
	repo persistence.ScheduleRepository
}

func newScheduleRepositoryAdapter(repo persistence.ScheduleRepository) *scheduleRepositoryAdapter {
	return &scheduleRepositoryAdapter{repo: repo}
}

func (a *scheduleRepositoryAdapter) CreateSchedule(ctx context.Context, schedule application.Schedule) (application.Schedule, error) {
	if err := a.repo.CreateSchedule(ctx, toPersistenceSchedule(schedule)); err != nil {
		return application.Schedule{}, err
	}
	stored, err := a.repo.GetSchedule(ctx, schedule.ID)
	if err != nil {
		return application.Schedule{}, err
	}
	return toApplicationSchedule(stored), nil
}

func (a *scheduleRepositoryAdapter) GetSchedule(ctx context.Context, id string) (application.Schedule, error) {
	stored, err := a.repo.GetSchedule(ctx, id)
	if err != nil {
		return application.Schedule{}, err
	}
	return toApplicationSchedule(stored), nil
}

func (a *scheduleRepositoryAdapter) UpdateSchedule(ctx context.Context, schedule application.Schedule) (application.Schedule, error) {
	if err := a.repo.UpdateSchedule(ctx, toPersistenceSchedule(schedule)); err != nil {
		return application.Schedule{}, err
	}
	stored, err := a.repo.GetSchedule(ctx, schedule.ID)
	if err != nil {
		return application.Schedule{}, err
	}
	return toApplicationSchedule(stored), nil
}

func (a *scheduleRepositoryAdapter) DeleteSchedule(ctx context.Context, id string) error {
	return a.repo.DeleteSchedule(ctx, id)
}

func (a *scheduleRepositoryAdapter) ListSchedules(ctx context.Context, filter application.ScheduleRepositoryFilter) ([]application.Schedule, error) {
	persistedFilter := persistence.ScheduleFilter{
		ParticipantIDs: append([]string(nil), filter.ParticipantIDs...),
		StartsAfter:    filter.StartsAfter,
		EndsBefore:     filter.EndsBefore,
	}
	models, err := a.repo.ListSchedules(ctx, persistedFilter)
	if err != nil {
		return nil, err
	}
	if len(models) == 0 {
		return nil, nil
	}
	schedules := make([]application.Schedule, 0, len(models))
	for _, model := range models {
		schedules = append(schedules, toApplicationSchedule(model))
	}
	return schedules, nil
}

type userDirectoryAdapter struct {
	repo persistence.UserRepository
}

func newUserDirectoryAdapter(repo persistence.UserRepository) *userDirectoryAdapter {
	return &userDirectoryAdapter{repo: repo}
}

func (a *userDirectoryAdapter) MissingUserIDs(ctx context.Context, ids []string) ([]string, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	missing := make([]string, 0)
	for _, id := range ids {
		if _, err := a.repo.GetUser(ctx, id); err != nil {
			if errors.Is(err, persistence.ErrNotFound) {
				missing = append(missing, id)
				continue
			}
			return nil, err
		}
	}
	if len(missing) == 0 {
		return nil, nil
	}
	return missing, nil
}

type roomCatalogAdapter struct {
	repo persistence.RoomRepository
}

func newRoomCatalogAdapter(repo persistence.RoomRepository) *roomCatalogAdapter {
	return &roomCatalogAdapter{repo: repo}
}

func (a *roomCatalogAdapter) RoomExists(ctx context.Context, id string) (bool, error) {
	if _, err := a.repo.GetRoom(ctx, id); err != nil {
		if errors.Is(err, persistence.ErrNotFound) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

type recurrenceRepositoryAdapter struct {
	repo        persistence.RecurrenceRepository
	idGenerator func() string
}

func newRecurrenceRepositoryAdapter(repo persistence.RecurrenceRepository, idGenerator func() string) *recurrenceRepositoryAdapter {
	return &recurrenceRepositoryAdapter{repo: repo, idGenerator: idGenerator}
}

func (a *recurrenceRepositoryAdapter) SaveRecurrence(ctx context.Context, scheduleID string, start time.Time, recurrence application.RecurrenceInput) error {
	weekdays := make([]time.Weekday, 0, len(recurrence.Weekdays))
	for _, day := range recurrence.Weekdays {
		weekdays = append(weekdays, toWeekday(day))
	}

	now := time.Now().UTC()
	rule := persistence.RecurrenceRule{
		ID:         a.idGenerator(),
		ScheduleID: scheduleID,
		Frequency:  toPersistenceFrequency(recurrence.Frequency),
		Weekdays:   weekdays,
		StartsOn:   start,
		EndsOn:     recurrence.Until,
		CreatedAt:  now,
		UpdatedAt:  now,
	}
	return a.repo.UpsertRecurrence(ctx, rule)
}

func (a *recurrenceRepositoryAdapter) ListRecurrencesForSchedules(ctx context.Context, scheduleIDs []string) (map[string][]application.RecurrenceRule, error) {
	// This is a simplified implementation for now.
	return nil, nil
}

func (a *recurrenceRepositoryAdapter) DeleteRecurrencesForSchedule(ctx context.Context, scheduleID string) error {
	return a.repo.DeleteRecurrencesForSchedule(ctx, scheduleID)
}

func toWeekday(day string) time.Weekday {
	switch strings.ToLower(day) {
	case "sunday":
		return time.Sunday
	case "monday":
		return time.Monday
	case "tuesday":
		return time.Tuesday
	case "wednesday":
		return time.Wednesday
	case "thursday":
		return time.Thursday
	case "friday":
		return time.Friday
	case "saturday":
		return time.Saturday
	}
	return time.Sunday // Default
}

func toPersistenceFrequency(freq string) int {
	switch strings.ToLower(freq) {
	case "weekly":
		return 1
	case "daily":
		return 0
	}
	return 1 // Default to weekly
}

type sessionRepositoryAdapter struct {
	repo persistence.SessionRepository
}

func newSessionRepositoryAdapter(repo persistence.SessionRepository) *sessionRepositoryAdapter {
	return &sessionRepositoryAdapter{repo: repo}
}

func (a *sessionRepositoryAdapter) CreateSession(ctx context.Context, session application.Session) (application.Session, error) {
	stored, err := a.repo.CreateSession(ctx, toPersistenceSession(session))
	if err != nil {
		return application.Session{}, err
	}
	return toApplicationSession(stored), nil
}

func (a *sessionRepositoryAdapter) GetSession(ctx context.Context, token string) (application.Session, error) {
	stored, err := a.repo.GetSession(ctx, token)
	if err != nil {
		return application.Session{}, err
	}
	return toApplicationSession(stored), nil
}

func (a *sessionRepositoryAdapter) UpdateSession(ctx context.Context, session application.Session) (application.Session, error) {
	stored, err := a.repo.UpdateSession(ctx, toPersistenceSession(session))
	if err != nil {
		return application.Session{}, err
	}
	return toApplicationSession(stored), nil
}

func (a *sessionRepositoryAdapter) RevokeSession(ctx context.Context, token string, revokedAt time.Time) (application.Session, error) {
	stored, err := a.repo.RevokeSession(ctx, token, revokedAt)
	if err != nil {
		return application.Session{}, err
	}
	return toApplicationSession(stored), nil
}

func (a *sessionRepositoryAdapter) DeleteExpiredSessions(ctx context.Context, reference time.Time) error {
	return a.repo.DeleteExpiredSessions(ctx, reference)
}

type credentialStoreAdapter struct {
	repo persistence.UserRepository
}

func newCredentialStoreAdapter(repo persistence.UserRepository) *credentialStoreAdapter {
	return &credentialStoreAdapter{repo: repo}
}

func (a *credentialStoreAdapter) GetUserCredentialsByEmail(ctx context.Context, email string) (application.UserCredentials, error) {
	stored, err := a.repo.GetUserByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, persistence.ErrNotFound) {
			return application.UserCredentials{}, application.ErrNotFound
		}
		return application.UserCredentials{}, err
	}
	return application.UserCredentials{
		User:         toApplicationUser(stored),
		PasswordHash: stored.PasswordHash,
	}, nil
}

func (a *credentialStoreAdapter) GetUser(ctx context.Context, id string) (application.User, error) {
	stored, err := a.repo.GetUser(ctx, id)
	if err != nil {
		return application.User{}, err
	}
	return toApplicationUser(stored), nil
}

func toApplicationUser(model persistence.User) application.User {
	return application.User{
		ID:          model.ID,
		Email:       model.Email,
		DisplayName: model.DisplayName,
		IsAdmin:     model.IsAdmin,
		CreatedAt:   model.CreatedAt,
		UpdatedAt:   model.UpdatedAt,
	}
}

func toPersistenceUser(user application.User, passwordHash string) persistence.User {
	if passwordHash == "" {
		passwordHash = user.ID
	}
	return persistence.User{
		ID:           user.ID,
		Email:        user.Email,
		DisplayName:  user.DisplayName,
		PasswordHash: passwordHash,
		IsAdmin:      user.IsAdmin,
		CreatedAt:    user.CreatedAt,
		UpdatedAt:    user.UpdatedAt,
	}
}

func toApplicationRoom(model persistence.Room) application.Room {
	return application.Room{
		ID:         model.ID,
		Name:       model.Name,
		Location:   model.Location,
		Capacity:   model.Capacity,
		Facilities: cloneString(model.Facilities),
		CreatedAt:  model.CreatedAt,
		UpdatedAt:  model.UpdatedAt,
	}
}

func toPersistenceRoom(room application.Room) persistence.Room {
	return persistence.Room{
		ID:         room.ID,
		Name:       room.Name,
		Location:   room.Location,
		Capacity:   room.Capacity,
		Facilities: cloneString(room.Facilities),
		CreatedAt:  room.CreatedAt,
		UpdatedAt:  room.UpdatedAt,
	}
}

func toApplicationSchedule(model persistence.Schedule) application.Schedule {
	description := ""
	if model.Memo != nil {
		description = *model.Memo
	}
	webURL := ""
	if model.WebConferenceURL != nil {
		webURL = *model.WebConferenceURL
	}
	return application.Schedule{
		ID:               model.ID,
		CreatorID:        model.CreatorID,
		Title:            model.Title,
		Description:      description,
		Start:            model.Start,
		End:              model.End,
		RoomID:           cloneString(model.RoomID),
		WebConferenceURL: webURL,
		ParticipantIDs:   append([]string(nil), model.Participants...),
		CreatedAt:        model.CreatedAt,
		UpdatedAt:        model.UpdatedAt,
	}
}

func toPersistenceSchedule(schedule application.Schedule) persistence.Schedule {
	var memo *string
	if strings.TrimSpace(schedule.Description) != "" {
		memo = cloneString(&schedule.Description)
	}
	var web *string
	if strings.TrimSpace(schedule.WebConferenceURL) != "" {
		web = cloneString(&schedule.WebConferenceURL)
	}
	return persistence.Schedule{
		ID:               schedule.ID,
		Title:            schedule.Title,
		Start:            schedule.Start,
		End:              schedule.End,
		CreatorID:        schedule.CreatorID,
		Memo:             memo,
		Participants:     append([]string(nil), schedule.ParticipantIDs...),
		RoomID:           cloneString(schedule.RoomID),
		WebConferenceURL: web,
		CreatedAt:        schedule.CreatedAt,
		UpdatedAt:        schedule.UpdatedAt,
	}
}

func toApplicationSession(model persistence.Session) application.Session {
	return application.Session{
		ID:          model.ID,
		UserID:      model.UserID,
		Token:       model.Token,
		Fingerprint: model.Fingerprint,
		ExpiresAt:   model.ExpiresAt,
		CreatedAt:   model.CreatedAt,
		UpdatedAt:   model.UpdatedAt,
		RevokedAt:   cloneTime(model.RevokedAt),
	}
}

func toPersistenceSession(session application.Session) persistence.Session {
	return persistence.Session{
		ID:          session.ID,
		UserID:      session.UserID,
		Token:       session.Token,
		Fingerprint: session.Fingerprint,
		ExpiresAt:   session.ExpiresAt,
		CreatedAt:   session.CreatedAt,
		UpdatedAt:   session.UpdatedAt,
		RevokedAt:   cloneTime(session.RevokedAt),
	}
}

func cloneString(value *string) *string {
	if value == nil {
		return nil
	}
	clone := *value
	return &clone
}

func cloneTime(value *time.Time) *time.Time {
	if value == nil {
		return nil
	}
	clone := *value
	return &clone
}
