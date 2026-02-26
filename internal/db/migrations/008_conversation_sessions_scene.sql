ALTER TABLE conversation_sessions ADD COLUMN scene TEXT NOT NULL DEFAULT 'chat';
ALTER TABLE conversation_sessions ADD COLUMN title TEXT NOT NULL DEFAULT '';
ALTER TABLE conversation_sessions ADD COLUMN deleted_at TEXT;

CREATE INDEX IF NOT EXISTS idx_conversation_sessions_scene_updated_at
  ON conversation_sessions (scene, updated_at DESC);

CREATE INDEX IF NOT EXISTS idx_conversation_sessions_deleted_at
  ON conversation_sessions (deleted_at);
