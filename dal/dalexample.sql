CREATE TABLE users (
    id INT PRIMARY KEY AUTO_INCREMENT,
    
    age TINYINT NOT NULL,
    
    birthdate DATE,
    
    email VARCHAR(255) NOT NULL,
    
    created TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP
) ENGINE=InnoDB;


# Unique indexes as they serve Get operations returning single entity
CREATE UNIQUE INDEX idx_email ON users (email);

