PRAGMA foreign_keys = ON;

CREATE TABLE IF NOT EXISTS schema_migrations (
    version TEXT PRIMARY KEY,
    applied_at DATETIME NOT NULL
);

CREATE TABLE IF NOT EXISTS users (
    id TEXT PRIMARY KEY,
    email TEXT NOT NULL UNIQUE,
    display_name TEXT NOT NULL,
    role TEXT NOT NULL CHECK (role IN ('employee','administrator')),
    password_hash BLOB NOT NULL,
    created_at DATETIME NOT NULL,
    updated_at DATETIME NOT NULL
);

CREATE TABLE IF NOT EXISTS rooms (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL UNIQUE,
    location TEXT NOT NULL,
    capacity INTEGER NOT NULL CHECK (capacity > 0),
    facilities TEXT,
    created_at DATETIME NOT NULL,
    updated_at DATETIME NOT NULL
);

CREATE TABLE IF NOT EXISTS schedules (
    id TEXT PRIMARY KEY,
    title TEXT NOT NULL,
    start_time DATETIME NOT NULL,
    end_time DATETIME NOT NULL,
    creator_id TEXT NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    memo TEXT,
    room_id TEXT REFERENCES rooms(id) ON DELETE SET NULL,
    online_url TEXT,
    created_at DATETIME NOT NULL,
    updated_at DATETIME NOT NULL,
    CHECK (end_time > start_time)
);

CREATE TABLE IF NOT EXISTS schedule_participants (
    schedule_id TEXT NOT NULL REFERENCES schedules(id) ON DELETE CASCADE,
    user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    PRIMARY KEY (schedule_id, user_id)
);

CREATE TABLE IF NOT EXISTS recurrences (
    id TEXT PRIMARY KEY,
    schedule_id TEXT NOT NULL REFERENCES schedules(id) ON DELETE CASCADE,
    frequency INTEGER NOT NULL,
    weekdays TEXT NOT NULL,
    starts_on DATE NOT NULL,
    ends_on DATE,
    created_at DATETIME NOT NULL,
    updated_at DATETIME NOT NULL,
    CHECK (ends_on IS NULL OR ends_on >= starts_on)
);

CREATE TABLE IF NOT EXISTS sessions (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token TEXT NOT NULL UNIQUE,
    fingerprint TEXT,
    expires_at DATETIME NOT NULL,
    created_at DATETIME NOT NULL,
    updated_at DATETIME NOT NULL,
    revoked_at DATETIME
);

CREATE INDEX IF NOT EXISTS idx_schedules_start ON schedules(start_time);
CREATE INDEX IF NOT EXISTS idx_schedules_room ON schedules(room_id, start_time);
CREATE INDEX IF NOT EXISTS idx_participants_user ON schedule_participants(user_id);
CREATE INDEX IF NOT EXISTS idx_sessions_user ON sessions(user_id);
CREATE INDEX IF NOT EXISTS idx_sessions_expires ON sessions(expires_at);
