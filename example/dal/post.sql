CREATE TABLE posts (
    id BIGINT PRIMARY KEY AUTO_INCREMENT,
    version INT DEFAULT 0,
    deleted BOOLEAN NOT NULL,
    post VARCHAR(255) NOT NULL,
    target_age TINYINT NOT NULL,
    created TIMESTAMP,
    updated TIMESTAMP
) ENGINE=InnoDB;

# Unique indexes as they serve Get operations returning single entity

# Indexes that serve all the list operations
CREATE INDEX idx_deleted ON posts (deleted);
CREATE INDEX idx_created ON posts (created);
CREATE INDEX idx_target_age ON posts (target_age);

