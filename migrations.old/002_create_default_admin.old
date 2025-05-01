-- Migration: Create default admin user
-- Password is 'admin', bcrypt-hashed

INSERT INTO users (email, password_hash, role, is_active)
VALUES (
  'admin@admin.com',
  '$2a$10$H4BrPBNinE7ejMx1vLHaie8OAYws8CXX5XitGaIzNXEfnYao7SUwW', -- bcrypt hash for 'admin'
  'admin',
  TRUE
)
ON CONFLICT(email) DO NOTHING; 