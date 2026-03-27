CREATE TABLE users (
    id BIGINT PRIMARY KEY AUTO_INCREMENT,
    version INT DEFAULT 0,
    age TINYINT NOT NULL,
    birthdate DATE,
    email VARCHAR(255) NOT NULL,
    meta JSON,
    status VARCHAR(255),
    uid VARCHAR(255) NOT NULL,
    created TIMESTAMP,
    updated TIMESTAMP
) ENGINE=InnoDB;

# Unique indexes as they serve Get operations returning single entity
CREATE UNIQUE INDEX idx_email ON users (email);
CREATE UNIQUE INDEX idx_uid ON users (uid);

# Indexes that serve all the list operations
CREATE INDEX idx_birthdate ON users (birthdate);
CREATE INDEX idx_age ON users (age);
CREATE INDEX idx_created ON users (created);
CREATE INDEX idx_status ON users (status);

