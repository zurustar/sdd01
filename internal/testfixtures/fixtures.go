package testfixtures

import (
	"fmt"
	"sync/atomic"
	"time"

	"github.com/example/enterprise-scheduler/internal/application"
	"github.com/example/enterprise-scheduler/internal/persistence"
	"github.com/example/enterprise-scheduler/internal/scheduler"
)

var (
	userCounter       uint64
	roomCounter       uint64
	scheduleCounter   uint64
	sessionCounter    uint64
	recurrenceCounter uint64
)

var referenceTime = time.Date(2024, time.January, 2, 15, 4, 5, 0, time.UTC)

// ReferenceTime returns the canonical baseline timestamp used by fixtures.
func ReferenceTime() time.Time {
	return referenceTime
}

// ----------------------------- User fixtures -----------------------------

// UserFixture represents a deterministic user record that can be materialised
// for application or persistence tests.
type UserFixture struct {
	ID           string
	Email        string
	DisplayName  string
	PasswordHash string
	IsAdmin      bool
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// UserOption configures the generated user fixture.
type UserOption func(*UserFixture)

// NewUserFixture returns a deterministic user fixture with optional overrides.
func NewUserFixture(opts ...UserOption) UserFixture {
	idx := atomic.AddUint64(&userCounter, 1)
	id := fmt.Sprintf("user-%03d", idx)
	created := referenceTime.Add(time.Duration(idx) * time.Minute)
	fixture := UserFixture{
		ID:           id,
		Email:        fmt.Sprintf("%s@example.com", id),
		DisplayName:  fmt.Sprintf("User %03d", idx),
		PasswordHash: fmt.Sprintf("hash-%03d", idx),
		IsAdmin:      false,
		CreatedAt:    created,
		UpdatedAt:    created,
	}
	for _, opt := range opts {
		opt(&fixture)
	}
	return fixture
}

// WithUserID overrides the generated user ID.
func WithUserID(id string) UserOption {
	return func(f *UserFixture) {
		f.ID = id
	}
}

// WithUserEmail overrides the generated email address.
func WithUserEmail(email string) UserOption {
	return func(f *UserFixture) {
		f.Email = email
	}
}

// WithUserDisplayName overrides the generated display name.
func WithUserDisplayName(name string) UserOption {
	return func(f *UserFixture) {
		f.DisplayName = name
	}
}

// WithUserPasswordHash overrides the generated password hash.
func WithUserPasswordHash(hash string) UserOption {
	return func(f *UserFixture) {
		f.PasswordHash = hash
	}
}

// WithUserAdmin sets the admin flag on the generated fixture.
func WithUserAdmin(isAdmin bool) UserOption {
	return func(f *UserFixture) {
		f.IsAdmin = isAdmin
	}
}

// WithUserCreatedAt sets the created timestamp on the fixture.
func WithUserCreatedAt(t time.Time) UserOption {
	return func(f *UserFixture) {
		f.CreatedAt = t
	}
}

// WithUserUpdatedAt sets the updated timestamp on the fixture.
func WithUserUpdatedAt(t time.Time) UserOption {
	return func(f *UserFixture) {
		f.UpdatedAt = t
	}
}

// WithUserTimestamps sets both created and updated timestamps on the fixture.
func WithUserTimestamps(created, updated time.Time) UserOption {
	return func(f *UserFixture) {
		f.CreatedAt = created
		f.UpdatedAt = updated
	}
}

// Application returns the fixture as an application.User value.
func (f UserFixture) Application() application.User {
	return application.User{
		ID:          f.ID,
		Email:       f.Email,
		DisplayName: f.DisplayName,
		IsAdmin:     f.IsAdmin,
		CreatedAt:   f.CreatedAt,
		UpdatedAt:   f.UpdatedAt,
	}
}

// Credentials returns the fixture as application.UserCredentials.
func (f UserFixture) Credentials() application.UserCredentials {
	creds := f.Application()
	return application.UserCredentials{
		User:         creds,
		PasswordHash: f.PasswordHash,
	}
}

// Principal returns an application.Principal derived from the fixture.
func (f UserFixture) Principal() application.Principal {
	return application.Principal{UserID: f.ID, IsAdmin: f.IsAdmin}
}

// Persistence returns the fixture as a persistence.User value.
func (f UserFixture) Persistence() persistence.User {
	return persistence.User{
		ID:           f.ID,
		Email:        f.Email,
		DisplayName:  f.DisplayName,
		PasswordHash: f.PasswordHash,
		IsAdmin:      f.IsAdmin,
		CreatedAt:    f.CreatedAt,
		UpdatedAt:    f.UpdatedAt,
	}
}

// Input returns the fixture as an application.UserInput.
func (f UserFixture) Input() application.UserInput {
	return application.UserInput{
		Email:       f.Email,
		DisplayName: f.DisplayName,
		IsAdmin:     f.IsAdmin,
	}
}

// ----------------------------- Room fixtures -----------------------------

// RoomFixture represents a deterministic meeting room record.
type RoomFixture struct {
	ID         string
	Name       string
	Location   string
	Capacity   int
	Facilities *string
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

// RoomOption configures the generated room fixture.
type RoomOption func(*RoomFixture)

// NewRoomFixture returns a deterministic room fixture with optional overrides.
func NewRoomFixture(opts ...RoomOption) RoomFixture {
	idx := atomic.AddUint64(&roomCounter, 1)
	id := fmt.Sprintf("room-%03d", idx)
	created := referenceTime.Add(time.Duration(idx) * time.Hour)
	fixture := RoomFixture{
		ID:        id,
		Name:      fmt.Sprintf("Room %03d", idx),
		Location:  "Main Office",
		Capacity:  int(4 + idx%4),
		CreatedAt: created,
		UpdatedAt: created,
	}
	for _, opt := range opts {
		opt(&fixture)
	}
	return fixture
}

// WithRoomID overrides the generated room ID.
func WithRoomID(id string) RoomOption {
	return func(f *RoomFixture) {
		f.ID = id
	}
}

// WithRoomName overrides the generated room name.
func WithRoomName(name string) RoomOption {
	return func(f *RoomFixture) {
		f.Name = name
	}
}

// WithRoomLocation overrides the generated location.
func WithRoomLocation(location string) RoomOption {
	return func(f *RoomFixture) {
		f.Location = location
	}
}

// WithRoomCapacity overrides the generated capacity.
func WithRoomCapacity(capacity int) RoomOption {
	return func(f *RoomFixture) {
		f.Capacity = capacity
	}
}

// WithRoomFacilities sets the facilities description on the fixture.
func WithRoomFacilities(facility string) RoomOption {
	return func(fx *RoomFixture) {
		value := facility
		fx.Facilities = &value
	}
}

// WithRoomFacilitiesPtr sets the facilities pointer directly.
func WithRoomFacilitiesPtr(facility *string) RoomOption {
	return func(fx *RoomFixture) {
		if facility == nil {
			fx.Facilities = nil
			return
		}
		value := *facility
		fx.Facilities = &value
	}
}

// WithoutRoomFacilities clears any facilities on the fixture.
func WithoutRoomFacilities() RoomOption {
	return func(f *RoomFixture) {
		f.Facilities = nil
	}
}

// WithRoomCreatedAt sets the created timestamp on the fixture.
func WithRoomCreatedAt(t time.Time) RoomOption {
	return func(f *RoomFixture) {
		f.CreatedAt = t
	}
}

// WithRoomUpdatedAt sets the updated timestamp on the fixture.
func WithRoomUpdatedAt(t time.Time) RoomOption {
	return func(f *RoomFixture) {
		f.UpdatedAt = t
	}
}

// WithRoomTimestamps sets both created and updated timestamps.
func WithRoomTimestamps(created, updated time.Time) RoomOption {
	return func(f *RoomFixture) {
		f.CreatedAt = created
		f.UpdatedAt = updated
	}
}

// Application returns the fixture as an application.Room value.
func (f RoomFixture) Application() application.Room {
	return application.Room{
		ID:         f.ID,
		Name:       f.Name,
		Location:   f.Location,
		Capacity:   f.Capacity,
		Facilities: copyStringPtr(f.Facilities),
		CreatedAt:  f.CreatedAt,
		UpdatedAt:  f.UpdatedAt,
	}
}

// Persistence returns the fixture as a persistence.Room value.
func (f RoomFixture) Persistence() persistence.Room {
	return persistence.Room{
		ID:         f.ID,
		Name:       f.Name,
		Location:   f.Location,
		Capacity:   f.Capacity,
		Facilities: copyStringPtr(f.Facilities),
		CreatedAt:  f.CreatedAt,
		UpdatedAt:  f.UpdatedAt,
	}
}

// Input returns the fixture as an application.RoomInput.
func (f RoomFixture) Input() application.RoomInput {
	return application.RoomInput{
		Name:       f.Name,
		Location:   f.Location,
		Capacity:   f.Capacity,
		Facilities: copyStringPtr(f.Facilities),
	}
}

// --------------------------- Schedule fixtures ---------------------------

// ScheduleFixture represents a deterministic schedule record.
type ScheduleFixture struct {
	ID               string
	CreatorID        string
	Title            string
	Description      string
	Start            time.Time
	End              time.Time
	ParticipantIDs   []string
	RoomID           *string
	WebConferenceURL string
	CreatedAt        time.Time
	UpdatedAt        time.Time
	Occurrences      []application.ScheduleOccurrence
}

// ScheduleOption configures the generated schedule fixture.
type ScheduleOption func(*ScheduleFixture)

// NewScheduleFixture returns a deterministic schedule fixture with optional overrides.
func NewScheduleFixture(opts ...ScheduleOption) ScheduleFixture {
	idx := atomic.AddUint64(&scheduleCounter, 1)
	id := fmt.Sprintf("schedule-%03d", idx)
	start := referenceTime.Add(time.Duration(idx) * time.Hour)
	end := start.Add(time.Hour)
	creator := fmt.Sprintf("user-%03d", idx)
	fixture := ScheduleFixture{
		ID:             id,
		CreatorID:      creator,
		Title:          fmt.Sprintf("Schedule %03d", idx),
		Description:    "",
		Start:          start,
		End:            end,
		ParticipantIDs: []string{creator},
		CreatedAt:      referenceTime,
		UpdatedAt:      referenceTime,
	}
	for _, opt := range opts {
		opt(&fixture)
	}
	return fixture
}

// WithScheduleID overrides the schedule ID.
func WithScheduleID(id string) ScheduleOption {
	return func(f *ScheduleFixture) {
		f.ID = id
	}
}

// WithScheduleCreator sets the creator ID.
func WithScheduleCreator(id string) ScheduleOption {
	return func(f *ScheduleFixture) {
		f.CreatorID = id
	}
}

// WithScheduleTitle overrides the title.
func WithScheduleTitle(title string) ScheduleOption {
	return func(f *ScheduleFixture) {
		f.Title = title
	}
}

// WithScheduleDescription sets the description/memo field.
func WithScheduleDescription(description string) ScheduleOption {
	return func(f *ScheduleFixture) {
		f.Description = description
	}
}

// WithScheduleStartEnd sets the start and end times.
func WithScheduleStartEnd(start, end time.Time) ScheduleOption {
	return func(f *ScheduleFixture) {
		f.Start = start
		f.End = end
	}
}

// WithScheduleParticipants sets the participant IDs.
func WithScheduleParticipants(participants ...string) ScheduleOption {
	return func(f *ScheduleFixture) {
		f.ParticipantIDs = append([]string(nil), participants...)
	}
}

// WithScheduleRoomID sets the optional room ID.
func WithScheduleRoomID(roomID string) ScheduleOption {
	return func(f *ScheduleFixture) {
		id := roomID
		f.RoomID = &id
	}
}

// WithoutScheduleRoom clears the room ID.
func WithoutScheduleRoom() ScheduleOption {
	return func(f *ScheduleFixture) {
		f.RoomID = nil
	}
}

// WithScheduleWebURL sets the web conference URL.
func WithScheduleWebURL(url string) ScheduleOption {
	return func(f *ScheduleFixture) {
		f.WebConferenceURL = url
	}
}

// WithoutScheduleWebURL clears the web conference URL.
func WithoutScheduleWebURL() ScheduleOption {
	return func(f *ScheduleFixture) {
		f.WebConferenceURL = ""
	}
}

// WithScheduleCreatedAt sets the created timestamp.
func WithScheduleCreatedAt(t time.Time) ScheduleOption {
	return func(f *ScheduleFixture) {
		f.CreatedAt = t
	}
}

// WithScheduleUpdatedAt sets the updated timestamp.
func WithScheduleUpdatedAt(t time.Time) ScheduleOption {
	return func(f *ScheduleFixture) {
		f.UpdatedAt = t
	}
}

// WithScheduleTimestamps sets both created and updated timestamps.
func WithScheduleTimestamps(created, updated time.Time) ScheduleOption {
	return func(f *ScheduleFixture) {
		f.CreatedAt = created
		f.UpdatedAt = updated
	}
}

// WithScheduleOccurrences sets the expanded occurrences on the fixture.
func WithScheduleOccurrences(occurrences []application.ScheduleOccurrence) ScheduleOption {
	return func(f *ScheduleFixture) {
		f.Occurrences = append([]application.ScheduleOccurrence(nil), occurrences...)
	}
}

// Application returns the fixture as an application.Schedule value.
func (f ScheduleFixture) Application() application.Schedule {
	var roomID *string
	if f.RoomID != nil {
		id := *f.RoomID
		roomID = &id
	}
	return application.Schedule{
		ID:               f.ID,
		CreatorID:        f.CreatorID,
		Title:            f.Title,
		Description:      f.Description,
		Start:            f.Start,
		End:              f.End,
		RoomID:           roomID,
		WebConferenceURL: f.WebConferenceURL,
		ParticipantIDs:   append([]string(nil), f.ParticipantIDs...),
		CreatedAt:        f.CreatedAt,
		UpdatedAt:        f.UpdatedAt,
		Occurrences:      append([]application.ScheduleOccurrence(nil), f.Occurrences...),
	}
}

// Input returns the fixture as an application.ScheduleInput.
func (f ScheduleFixture) Input() application.ScheduleInput {
	var roomID *string
	if f.RoomID != nil {
		id := *f.RoomID
		roomID = &id
	}
	return application.ScheduleInput{
		CreatorID:        f.CreatorID,
		Title:            f.Title,
		Description:      f.Description,
		Start:            f.Start,
		End:              f.End,
		RoomID:           roomID,
		WebConferenceURL: f.WebConferenceURL,
		ParticipantIDs:   append([]string(nil), f.ParticipantIDs...),
	}
}

// Persistence returns the fixture as a persistence.Schedule value.
func (f ScheduleFixture) Persistence() persistence.Schedule {
	var memo *string
	if f.Description != "" {
		desc := f.Description
		memo = &desc
	}
	var roomID *string
	if f.RoomID != nil {
		id := *f.RoomID
		roomID = &id
	}
	var webURL *string
	if f.WebConferenceURL != "" {
		url := f.WebConferenceURL
		webURL = &url
	}
	return persistence.Schedule{
		ID:               f.ID,
		Title:            f.Title,
		Start:            f.Start,
		End:              f.End,
		CreatorID:        f.CreatorID,
		Memo:             memo,
		Participants:     append([]string(nil), f.ParticipantIDs...),
		RoomID:           roomID,
		WebConferenceURL: webURL,
		CreatedAt:        f.CreatedAt,
		UpdatedAt:        f.UpdatedAt,
	}
}

// Scheduler returns the fixture as a scheduler.Schedule value.
func (f ScheduleFixture) Scheduler() scheduler.Schedule {
	var roomID *string
	if f.RoomID != nil {
		id := *f.RoomID
		roomID = &id
	}
	return scheduler.Schedule{
		ID:           f.ID,
		Participants: append([]string(nil), f.ParticipantIDs...),
		RoomID:       roomID,
		Start:        f.Start,
		End:          f.End,
	}
}

// --------------------------- Recurrence fixtures -------------------------

// RecurrenceFixture represents a deterministic recurrence rule.
type RecurrenceFixture struct {
	ID         string
	ScheduleID string
	Frequency  int
	Weekdays   []time.Weekday
	StartsOn   time.Time
	EndsOn     *time.Time
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

// RecurrenceOption configures the generated recurrence fixture.
type RecurrenceOption func(*RecurrenceFixture)

// NewRecurrenceFixture returns a deterministic recurrence fixture with optional overrides.
func NewRecurrenceFixture(opts ...RecurrenceOption) RecurrenceFixture {
	idx := atomic.AddUint64(&recurrenceCounter, 1)
	id := fmt.Sprintf("recurrence-%03d", idx)
	startsOn := referenceTime.Truncate(24 * time.Hour)
	fixture := RecurrenceFixture{
		ID:         id,
		ScheduleID: fmt.Sprintf("schedule-%03d", idx),
		Frequency:  1,
		Weekdays:   []time.Weekday{time.Monday},
		StartsOn:   startsOn,
		CreatedAt:  referenceTime,
		UpdatedAt:  referenceTime,
	}
	for _, opt := range opts {
		opt(&fixture)
	}
	return fixture
}

// WithRecurrenceID overrides the recurrence ID.
func WithRecurrenceID(id string) RecurrenceOption {
	return func(f *RecurrenceFixture) {
		f.ID = id
	}
}

// WithRecurrenceScheduleID sets the associated schedule ID.
func WithRecurrenceScheduleID(id string) RecurrenceOption {
	return func(f *RecurrenceFixture) {
		f.ScheduleID = id
	}
}

// WithRecurrenceFrequency sets the recurrence frequency.
func WithRecurrenceFrequency(freq int) RecurrenceOption {
	return func(f *RecurrenceFixture) {
		f.Frequency = freq
	}
}

// WithRecurrenceWeekdays sets the recurrence weekdays.
func WithRecurrenceWeekdays(days ...time.Weekday) RecurrenceOption {
	return func(f *RecurrenceFixture) {
		f.Weekdays = append([]time.Weekday(nil), days...)
	}
}

// WithRecurrenceStartsOn sets the start date for the recurrence.
func WithRecurrenceStartsOn(t time.Time) RecurrenceOption {
	return func(f *RecurrenceFixture) {
		f.StartsOn = t
	}
}

// WithRecurrenceEndsOn sets the optional end date.
func WithRecurrenceEndsOn(t time.Time) RecurrenceOption {
	return func(f *RecurrenceFixture) {
		end := t
		f.EndsOn = &end
	}
}

// WithoutRecurrenceEndsOn clears any end date on the fixture.
func WithoutRecurrenceEndsOn() RecurrenceOption {
	return func(f *RecurrenceFixture) {
		f.EndsOn = nil
	}
}

// WithRecurrenceCreatedAt sets the created timestamp.
func WithRecurrenceCreatedAt(t time.Time) RecurrenceOption {
	return func(f *RecurrenceFixture) {
		f.CreatedAt = t
	}
}

// WithRecurrenceUpdatedAt sets the updated timestamp.
func WithRecurrenceUpdatedAt(t time.Time) RecurrenceOption {
	return func(f *RecurrenceFixture) {
		f.UpdatedAt = t
	}
}

// WithRecurrenceTimestamps sets both created and updated timestamps.
func WithRecurrenceTimestamps(created, updated time.Time) RecurrenceOption {
	return func(f *RecurrenceFixture) {
		f.CreatedAt = created
		f.UpdatedAt = updated
	}
}

// Persistence returns the fixture as a persistence.RecurrenceRule value.
func (f RecurrenceFixture) Persistence() persistence.RecurrenceRule {
	var endsOn *time.Time
	if f.EndsOn != nil {
		end := *f.EndsOn
		endsOn = &end
	}
	return persistence.RecurrenceRule{
		ID:         f.ID,
		ScheduleID: f.ScheduleID,
		Frequency:  f.Frequency,
		Weekdays:   append([]time.Weekday(nil), f.Weekdays...),
		StartsOn:   f.StartsOn,
		EndsOn:     endsOn,
		CreatedAt:  f.CreatedAt,
		UpdatedAt:  f.UpdatedAt,
	}
}

// ----------------------------- Session fixtures -------------------------

// SessionFixture represents a deterministic session record.
type SessionFixture struct {
	ID          string
	UserID      string
	Token       string
	Fingerprint string
	ExpiresAt   time.Time
	CreatedAt   time.Time
	UpdatedAt   time.Time
	RevokedAt   *time.Time
}

// SessionOption configures the generated session fixture.
type SessionOption func(*SessionFixture)

// NewSessionFixture returns a deterministic session fixture with optional overrides.
func NewSessionFixture(opts ...SessionOption) SessionFixture {
	idx := atomic.AddUint64(&sessionCounter, 1)
	id := fmt.Sprintf("session-%03d", idx)
	userID := fmt.Sprintf("user-%03d", idx)
	created := referenceTime
	fixture := SessionFixture{
		ID:          id,
		UserID:      userID,
		Token:       fmt.Sprintf("token-%03d", idx),
		Fingerprint: fmt.Sprintf("fingerprint-%03d", idx),
		ExpiresAt:   created.Add(8 * time.Hour),
		CreatedAt:   created,
		UpdatedAt:   created,
	}
	for _, opt := range opts {
		opt(&fixture)
	}
	return fixture
}

// WithSessionID overrides the session ID.
func WithSessionID(id string) SessionOption {
	return func(f *SessionFixture) {
		f.ID = id
	}
}

// WithSessionUserID sets the user ID.
func WithSessionUserID(id string) SessionOption {
	return func(f *SessionFixture) {
		f.UserID = id
	}
}

// WithSessionToken overrides the token value.
func WithSessionToken(token string) SessionOption {
	return func(f *SessionFixture) {
		f.Token = token
	}
}

// WithSessionFingerprint sets the session fingerprint.
func WithSessionFingerprint(fp string) SessionOption {
	return func(f *SessionFixture) {
		f.Fingerprint = fp
	}
}

// WithSessionExpiresAt sets the expiration timestamp.
func WithSessionExpiresAt(t time.Time) SessionOption {
	return func(f *SessionFixture) {
		f.ExpiresAt = t
	}
}

// WithSessionCreatedAt sets the created timestamp.
func WithSessionCreatedAt(t time.Time) SessionOption {
	return func(f *SessionFixture) {
		f.CreatedAt = t
	}
}

// WithSessionUpdatedAt sets the updated timestamp.
func WithSessionUpdatedAt(t time.Time) SessionOption {
	return func(f *SessionFixture) {
		f.UpdatedAt = t
	}
}

// WithSessionTimestamps sets both created and updated timestamps.
func WithSessionTimestamps(created, updated time.Time) SessionOption {
	return func(f *SessionFixture) {
		f.CreatedAt = created
		f.UpdatedAt = updated
	}
}

// WithSessionRevokedAt sets the optional revoked timestamp.
func WithSessionRevokedAt(t time.Time) SessionOption {
	return func(f *SessionFixture) {
		revoked := t
		f.RevokedAt = &revoked
	}
}

// WithoutSessionRevoked clears any revoked timestamp.
func WithoutSessionRevoked() SessionOption {
	return func(f *SessionFixture) {
		f.RevokedAt = nil
	}
}

// Application returns the fixture as an application.Session value.
func (f SessionFixture) Application() application.Session {
	var revoked *time.Time
	if f.RevokedAt != nil {
		t := *f.RevokedAt
		revoked = &t
	}
	return application.Session{
		ID:          f.ID,
		UserID:      f.UserID,
		Token:       f.Token,
		Fingerprint: f.Fingerprint,
		ExpiresAt:   f.ExpiresAt,
		CreatedAt:   f.CreatedAt,
		UpdatedAt:   f.UpdatedAt,
		RevokedAt:   revoked,
	}
}

// Persistence returns the fixture as a persistence.Session value.
func (f SessionFixture) Persistence() persistence.Session {
	var revoked *time.Time
	if f.RevokedAt != nil {
		t := *f.RevokedAt
		revoked = &t
	}
	return persistence.Session{
		ID:          f.ID,
		UserID:      f.UserID,
		Token:       f.Token,
		Fingerprint: f.Fingerprint,
		ExpiresAt:   f.ExpiresAt,
		CreatedAt:   f.CreatedAt,
		UpdatedAt:   f.UpdatedAt,
		RevokedAt:   revoked,
	}
}

// helper to deep copy optional strings.
func copyStringPtr(src *string) *string {
	if src == nil {
		return nil
	}
	value := *src
	return &value
}
