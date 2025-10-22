-- Migration: 002_test_posts.sql
-- Description: Create test posts table with foreign key to users

CREATE TABLE IF NOT EXISTS test_posts (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL,
    title TEXT NOT NULL,
    content TEXT,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (user_id) REFERENCES test_users(id) ON DELETE CASCADE
);

-- Create index for better query performance
CREATE INDEX IF NOT EXISTS idx_test_posts_user_id ON test_posts(user_id);

-- Insert some test data
INSERT INTO test_posts (id, user_id, title, content) VALUES 
    ('post1', 'user1', 'First Test Post', 'This is the content of the first test post'),
    ('post2', 'user1', 'Second Test Post', 'This is the content of the second test post'),
    ('post3', 'user2', 'User 2 Post', 'This is a post by user 2');