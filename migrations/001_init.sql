-- Initial schema reference (GORM AutoMigrate is primary in development).
-- Production deployments can apply this SQL or use AutoMigrate.

CREATE EXTENSION IF NOT EXISTS "pgcrypto";

CREATE INDEX IF NOT EXISTS idx_messages_conv_created ON messages (conversation_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_conversation_members_user ON conversation_members (user_id) WHERE left_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_messages_body_trgm ON messages USING gin (lower(body) gin_trgm_ops);
