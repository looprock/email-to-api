-- Create users table
CREATE TABLE IF NOT EXISTS users (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    email VARCHAR(255) NOT NULL UNIQUE,
    password_hash VARCHAR(255),
    role VARCHAR(10) NOT NULL CHECK (role IN ('admin', 'user')),
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    last_login DATETIME,
    is_active BOOLEAN DEFAULT TRUE
);

-- Create registration_tokens table
CREATE TABLE IF NOT EXISTS registration_tokens (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id INTEGER NOT NULL,
    token VARCHAR(64) NOT NULL UNIQUE,
    expires_at DATETIME NOT NULL,
    used_at DATETIME,
    FOREIGN KEY (user_id) REFERENCES users(id)
);

-- Create email_mappings table
CREATE TABLE IF NOT EXISTS email_mappings (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id INTEGER NOT NULL,
    headers TEXT DEFAULT '{}',
    generated_email VARCHAR(255) NOT NULL UNIQUE,
    endpoint_url TEXT NOT NULL,
    description TEXT,
    is_active BOOLEAN DEFAULT TRUE,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (user_id) REFERENCES users(id)
);

-- Create email_logs table for auditing
CREATE TABLE IF NOT EXISTS email_logs (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    mapping_id INTEGER NOT NULL,
    from_address TEXT NOT NULL,
    subject TEXT,
    headers TEXT DEFAULT '{}',
    body TEXT,
    processed_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    status TEXT NOT NULL,
    error_message TEXT,
    FOREIGN KEY (mapping_id) REFERENCES email_mappings(id)
);

-- Create indexes
CREATE INDEX IF NOT EXISTS idx_email_mappings_generated_email ON email_mappings(generated_email);
CREATE INDEX IF NOT EXISTS idx_email_logs_mapping_id ON email_logs(mapping_id);
CREATE INDEX IF NOT EXISTS idx_email_logs_processed_at ON email_logs(processed_at);
CREATE INDEX IF NOT EXISTS idx_registration_tokens_user_id ON registration_tokens(user_id); 

INSERT INTO users (email, password_hash, role, is_active)
VALUES (
  'admin@admin.com',
  '$2a$10$H4BrPBNinE7ejMx1vLHaie8OAYws8CXX5XitGaIzNXEfnYao7SUwW', -- bcrypt hash for 'admin'
  'admin',
  TRUE
)
ON CONFLICT(email) DO NOTHING; 