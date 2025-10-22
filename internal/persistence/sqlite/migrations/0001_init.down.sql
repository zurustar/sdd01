DROP INDEX IF EXISTS idx_sessions_expires;
DROP INDEX IF EXISTS idx_sessions_user;
DROP INDEX IF EXISTS idx_participants_user;
DROP INDEX IF EXISTS idx_schedules_room;
DROP INDEX IF EXISTS idx_schedules_start;

DROP TABLE IF EXISTS sessions;
DROP TABLE IF EXISTS recurrences;
DROP TABLE IF EXISTS schedule_participants;
DROP TABLE IF EXISTS schedules;
DROP TABLE IF EXISTS rooms;
DROP TABLE IF EXISTS users;
DROP TABLE IF EXISTS schema_migrations;
