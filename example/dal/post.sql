CREATE TABLE posts (
    id BIGINT PRIMARY KEY AUTO_INCREMENT,
    version INT DEFAULT 0,
    deleted BOOLEAN NOT NULL,
    expires_at DATETIME NOT NULL,
    language_id VARCHAR(255) NOT NULL,
    post text NOT NULL,
    revoked BOOLEAN NOT NULL,
    story_uid VARCHAR(255) NOT NULL,
    target_age TINYINT NOT NULL,
    user_id VARCHAR(255) NOT NULL,
    
    created TIMESTAMP,
    updated TIMESTAMP
) ENGINE=InnoDB;

# Unique indexes as they serve Get operations returning single entity

# Indexes that serve all the list operations
CREATE INDEX idx_deleted ON posts (deleted);
CREATE INDEX idx_target_age ON posts (target_age);
CREATE INDEX idx_created ON posts (created);
CREATE INDEX idx_language_id ON posts (language_id);
CREATE INDEX idx_story_uid ON posts (story_uid);
CREATE INDEX idx_user_id ON posts (user_id);


