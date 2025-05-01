-- Create users table
CREATE TABLE IF NOT EXISTS users (
    id SERIAL PRIMARY KEY,
    email VARCHAR(255) NOT NULL UNIQUE,
    password_hash VARCHAR(255),
    role VARCHAR(10) NOT NULL CHECK (role IN ('admin', 'user')),
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    last_login TIMESTAMP,
    is_active BOOLEAN DEFAULT TRUE
);

-- Create the trigger function to update the `updated_at` column
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
  NEW.updated_at = CURRENT_TIMESTAMP;
  RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Drop the trigger if it already exists
DROP TRIGGER IF EXISTS users_updated_at ON users;

-- Create the trigger to use the function on UPDATE
CREATE TRIGGER users_updated_at
BEFORE UPDATE ON users
FOR EACH ROW
EXECUTE FUNCTION update_updated_at_column();

-- Create registration_tokens table
CREATE TABLE IF NOT EXISTS registration_tokens (
    id SERIAL PRIMARY KEY,
    user_id INTEGER NOT NULL,
    token VARCHAR(64) NOT NULL UNIQUE,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    expires_at TIMESTAMP NOT NULL,
    used_at TIMESTAMP,
    FOREIGN KEY (user_id) REFERENCES users(id)
);

-- Create email_mappings table
CREATE TABLE IF NOT EXISTS email_mappings (
    id SERIAL PRIMARY KEY,
    user_id INTEGER NOT NULL,
    headers TEXT DEFAULT '{}',
    generated_email VARCHAR(255) NOT NULL UNIQUE,
    endpoint_url TEXT NOT NULL,
    description TEXT,
    is_active BOOLEAN DEFAULT TRUE,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (user_id) REFERENCES users(id)
);

-- Create email_logs table for auditing
CREATE TABLE IF NOT EXISTS email_logs (
    id SERIAL PRIMARY KEY,
    mapping_id INTEGER NOT NULL,
    from_address TEXT NOT NULL,
    subject TEXT,
    headers TEXT DEFAULT '{}',
    body TEXT,
    processed_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    status TEXT NOT NULL,
    error_message TEXT,
    FOREIGN KEY (mapping_id) REFERENCES email_mappings(id)
);

-- Create indexes
CREATE INDEX IF NOT EXISTS idx_email_mappings_generated_email ON email_mappings(generated_email);
CREATE INDEX IF NOT EXISTS idx_email_logs_mapping_id ON email_logs(mapping_id);
CREATE INDEX IF NOT EXISTS idx_email_logs_processed_at ON email_logs(processed_at);
CREATE INDEX IF NOT EXISTS idx_registration_tokens_user_id ON registration_tokens(user_id); 

-- Seed admin user (if not already present)
INSERT INTO users (email, password_hash, role, is_active)
VALUES (
  'admin@admin.com',
  '$2a$10$H4BrPBNinE7ejMx1vLHaie8OAYws8CXX5XitGaIzNXEfnYao7SUwW', -- bcrypt hash for 'admin'
  'admin',
  TRUE
)
ON CONFLICT (email) DO NOTHING;
