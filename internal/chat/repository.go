package chat

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"

	"github.com/cloudwego/eino/schema"
	"github.com/google/uuid"

	"genFu/internal/db"
)

type Repository struct {
	db *db.DB
}

func NewRepository(database *db.DB) *Repository {
	return &Repository{db: database}
}

func (r *Repository) EnsureSession(ctx context.Context, sessionID string, userID string) (string, error) {
	if r == nil || r.db == nil || r.db.DB == nil {
		return "", errors.New("db_not_initialized")
	}
	if sessionID == "" {
		sessionID = uuid.NewString()
	}
	_, err := r.db.ExecContext(ctx, `insert or ignore into conversation_sessions(id, user_id) values (?, ?)`, sessionID, userID)
	if err != nil {
		return "", err
	}
	_, _ = r.db.ExecContext(ctx, `update conversation_sessions set updated_at = CURRENT_TIMESTAMP, user_id = ? where id = ?`, userID, sessionID)
	return sessionID, nil
}

func (r *Repository) AppendMessages(ctx context.Context, sessionID string, messages []*schema.Message) error {
	if r == nil || r.db == nil || r.db.DB == nil {
		return errors.New("db_not_initialized")
	}
	if sessionID == "" {
		return errors.New("missing_session_id")
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	for _, msg := range messages {
		if msg == nil {
			continue
		}
		payload, err := json.Marshal(msg)
		if err != nil {
			_ = tx.Rollback()
			return err
		}
		_, err = tx.ExecContext(ctx, `insert into conversation_messages(session_id, role, content, payload) values (?, ?, ?, ?)`,
			sessionID, msg.Role, msg.Content, string(payload))
		if err != nil {
			_ = tx.Rollback()
			return err
		}
	}
	if _, err := tx.ExecContext(ctx, `update conversation_sessions set updated_at = CURRENT_TIMESTAMP where id = ?`, sessionID); err != nil {
		_ = tx.Rollback()
		return err
	}
	return tx.Commit()
}

func (r *Repository) ListMessages(ctx context.Context, sessionID string, limit int) ([]*schema.Message, error) {
	if r == nil || r.db == nil || r.db.DB == nil {
		return nil, errors.New("db_not_initialized")
	}
	if sessionID == "" {
		return nil, errors.New("missing_session_id")
	}
	query := `select payload from conversation_messages where session_id = ? order by id asc`
	var rows *sql.Rows
	var err error
	if limit > 0 {
		query = query + ` limit ?`
		rows, err = r.db.QueryContext(ctx, query, sessionID, limit)
	} else {
		rows, err = r.db.QueryContext(ctx, query, sessionID)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]*schema.Message, 0)
	for rows.Next() {
		var payload string
		if err := rows.Scan(&payload); err != nil {
			return nil, err
		}
		var msg schema.Message
		if err := json.Unmarshal([]byte(payload), &msg); err != nil {
			return nil, err
		}
		result = append(result, &msg)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return result, nil
}
