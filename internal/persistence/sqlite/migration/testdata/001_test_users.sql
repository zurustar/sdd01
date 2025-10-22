-- Migration: 001_test_users.sql
-- Description: Create test users table for testing scenarios

CREATE TABLE IF NOT EXISTS test_users (
    id TEXT PRIMARY KEY,
    username TEXT NOT NULL UNIQUE,
    email TEXT NOT NULL UNIQUE,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Insert some test data
INSERT INTO test_users (id, username, email) VALUES 
    ('user1', 'testuser1', 'test1@example.com'),
    ('user2', 'testuser2', 'test2@example.com');