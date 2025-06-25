CREATE TABLE users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email VARCHAR(255) UNIQUE NOT NULL,
    username VARCHAR(500) NOT NULL,
    password_hash BYTEA NOT NULL,
    first_name VARCHAR(255),
    last_name VARCHAR(255),
    mobile_number VARCHAR(20),
    avatar_url VARCHAR(255),
    role VARCHAR(50) NOT NULL,
    authtype VARCHAR(50),
    has_completed_onboarding BOOLEAN DEFAULT FALSE,
    activated BOOLEAN DEFAULT FALSE,
    version INT DEFAULT 1,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);