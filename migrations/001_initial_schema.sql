-- Create users table
CREATE TABLE users (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    email VARCHAR(255) NOT NULL UNIQUE,
    password_hash VARCHAR(255),
    role VARCHAR(10) NOT NULL CHECK (role IN ('admin', 'user')),
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    last_login DATETIME,
    is_active BOOLEAN DEFAULT TRUE
);

-- Create registration_tokens table
CREATE TABLE registration_tokens (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id INTEGER NOT NULL,
    token VARCHAR(64) NOT NULL UNIQUE,
    expires_at DATETIME NOT NULL,
    used_at DATETIME,
    FOREIGN KEY (user_id) REFERENCES users(id)
);

-- Create email_mappings table
CREATE TABLE email_mappings (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id INTEGER NOT NULL,
    generated_email VARCHAR(255) NOT NULL UNIQUE,
    endpoint_url TEXT NOT NULL,
    description TEXT,
    is_active BOOLEAN DEFAULT TRUE,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (user_id) REFERENCES users(id)
);

-- Create email_logs table for auditing
CREATE TABLE email_logs (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    mapping_id INTEGER NOT NULL,
    from_address TEXT NOT NULL,
    subject TEXT,
    body TEXT,
    processed_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    status TEXT NOT NULL,
    error_message TEXT,
    FOREIGN KEY (mapping_id) REFERENCES email_mappings(id)
);

-- Create indexes
CREATE INDEX idx_email_mappings_generated_email ON email_mappings(generated_email);
CREATE INDEX idx_email_logs_mapping_id ON email_logs(mapping_id);
CREATE INDEX idx_email_logs_processed_at ON email_logs(processed_at);
CREATE INDEX idx_registration_tokens_user_id ON registration_tokens(user_id); 