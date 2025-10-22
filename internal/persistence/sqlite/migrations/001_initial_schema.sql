-- Migration: 001_initial_schema.sql
-- Description: Create initial database schema with users, rooms, schedules, recurrences, and sessions

PRAGMA foreign_keys = ON;

CREATE TABLE IF NOT EXISTS users (
    id TEXT PRIMARY KEY,
    email TEXT NOT NULL UNIQUE,
    display_name TEXT NOT NULL,
    password_hash TEXT NOT NULL,
    is_admin INTEGER NOT NULL DEFAULT 0,
    created_at DATETIME NOT NULL,
    updated_at DATETIME NOT NULL
);

CREATE TABLE IF NOT EXISTS rooms (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    location TEXT NOT NULL,
    capacity INTEGER NOT NULL CHECK (capacity > 0),
    facilities TEXT,
    created_at DATETIME NOT NULL,
    updated_at DATETIME NOT NULL
);

CREATE TABLE IF NOT EXISTS schedules (
    id TEXT PRIMARY KEY,
    title TEXT NOT NULL,
    start_at DATETIME NOT NULL,
    end_at DATETIME NOT NULL,
    creator_id TEXT NOT NULL,
    memo TEXT,
    room_id TEXT,
    web_conference_url TEXT,
    created_at DATETIME NOT NULL,
    updated_at DATETIME NOT NULL,
    CHECK (end_at > start_at),
    FOREIGN KEY (creator_id) REFERENCES users(id) ON DELETE RESTRICT,
    FOREIGN KEY (room_id) REFERENCES rooms(id) ON DELETE SET NULL
);

CREATE TABLE IF NOT EXISTS schedule_participants (
    schedule_id TEXT NOT NULL,
    participant_id TEXT NOT NULL,
    PRIMARY KEY (schedule_id, participant_id),
    FOREIGN KEY (schedule_id) REFERENCES schedules(id) ON DELETE CASCADE,
    FOREIGN KEY (participant_id) REFERENCES users(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS recurrences (
    id TEXT PRIMARY KEY,
    schedule_id TEXT NOT NULL,
    frequency INTEGER NOT NULL,
    weekdays INTEGER NOT NULL,
    starts_on DATE NOT NULL,
    ends_on DATE,
    created_at DATETIME NOT NULL,
    updated_at DATETIME NOT NULL,
    CHECK (ends_on IS NULL OR ends_on >= starts_on),
    FOREIGN KEY (schedule_id) REFERENCES schedules(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_schedule_participants_participant_schedule ON schedule_participants(participant_id, schedule_id);
CREATE INDEX IF NOT EXISTS idx_schedules_start_end ON schedules(start_at, end_at);

CREATE TABLE IF NOT EXISTS sessions (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL,
    token TEXT NOT NULL UNIQUE,
    fingerprint TEXT,
    expires_at DATETIME NOT NULL,
    created_at DATETIME NOT NULL,
    updated_at DATETIME NOT NULL,
    revoked_at DATETIME,
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_sessions_expires_at ON sessions(expires_at);
CREATE INDEX IF NOT EXISTS idx_sessions_user_id ON sessions(user_id);