package chat

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"

	"genFu/internal/db"
)

type SessionMemory struct {
	SessionID  string
	Summary    string
	LastIntent string
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

type SessionMemoryRepository struct {
	db *db.DB
}

func NewSessionMemoryRepository(database *db.DB) *SessionMemoryRepository {
	return &SessionMemoryRepository{db: database}
}

func (r *SessionMemoryRepository) Get(ctx context.Context, sessionID string) (SessionMemory, error) {
	if r == nil || r.db == nil || r.db.DB == nil {
		return SessionMemory{}, errors.New("db_not_initialized")
	}
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return SessionMemory{}, errors.New("missing_session_id")
	}
	row := r.db.QueryRowContext(ctx, `
		select session_id, summary, last_intent, created_at, updated_at
		from conversation_session_memories
		where session_id = ?
	`, sessionID)
	var out SessionMemory
	var createdRaw sql.NullString
	var updatedRaw sql.NullString
	if err := row.Scan(&out.SessionID, &out.Summary, &out.LastIntent, &createdRaw, &updatedRaw); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return SessionMemory{SessionID: sessionID}, nil
		}
		return SessionMemory{}, err
	}
	if parsed, ok := db.ParseTime(createdRaw); ok {
		out.CreatedAt = parsed
	}
	if parsed, ok := db.ParseTime(updatedRaw); ok {
		out.UpdatedAt = parsed
	}
	return out, nil
}

func (r *SessionMemoryRepository) Upsert(ctx context.Context, sessionID string, summary string, lastIntent string) error {
	if r == nil || r.db == nil || r.db.DB == nil {
		return errors.New("db_not_initialized")
	}
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return errors.New("missing_session_id")
	}
	summary = strings.TrimSpace(summary)
	lastIntent = strings.TrimSpace(lastIntent)
	_, err := r.db.ExecContext(ctx, `
		insert into conversation_session_memories(session_id, summary, last_intent)
		values (?, ?, ?)
		on conflict(session_id) do update set
			summary = excluded.summary,
			last_intent = excluded.last_intent,
			updated_at = CURRENT_TIMESTAMP
	`, sessionID, summary, lastIntent)
	return err
}
