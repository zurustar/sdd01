-- Migration: 003_test_comments.sql
-- Description: Create test comments table with foreign keys

CREATE TABLE IF NOT EXISTS test_comments (
    id TEXT PRIMARY KEY,
    post_id TEXT NOT NULL,
    user_id TEXT NOT NULL,
    content TEXT NOT NULL,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (post_id) REFERENCES test_posts(id) ON DELETE CASCADE,
    FOREIGN KEY (user_id) REFERENCES test_users(id) ON DELETE CASCADE
);

-- Create indexes for better query performance
CREATE INDEX IF NOT EXISTS idx_test_comments_post_id ON test_comments(post_id);
CREATE INDEX IF NOT EXISTS idx_test_comments_user_id ON test_comments(user_id);

-- Insert some test data
INSERT INTO test_comments (id, post_id, user_id, content) VALUES 
    ('comment1', 'post1', 'user2', 'Great post!'),
    ('comment2', 'post1', 'user1', 'Thanks for the feedback'),
    ('comment3', 'post2', 'user2', 'Interesting perspective'),
    ('comment4', 'post3', 'user1', 'Nice work!');