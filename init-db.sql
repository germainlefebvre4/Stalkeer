-- Database initialization script for Stalkeer
-- This script is automatically executed when PostgreSQL container starts for the first time

-- Create extensions if needed
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

-- Note: Actual table creation is handled by GORM migrations in the application
-- This script is here for any additional database setup needed at initialization

-- Grant permissions to the stalkeer user (if using a non-default user)
-- GRANT ALL PRIVILEGES ON DATABASE stalkeer TO stalkeer_user;

-- Create any custom functions, triggers, or indexes here if needed in the future
