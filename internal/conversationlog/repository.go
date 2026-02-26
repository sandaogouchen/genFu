package conversationlog

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"

	"genFu/internal/db"
)

const (
	SceneAnalyze     = "analyze"
	SceneDecision    = "decision"
	SceneStockPicker = "stockpicker"
	SceneWorkflow    = "workflow"
	SceneChat        = "chat"
)

const defaultUserID = "default"

type Repository struct {
	db *db.DB
}

type Session struct {
	ID        string    `json:"id"`
	Scene     string    `json:"scene"`
	Title     string    `json:"title"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type Run struct {
	ID        int64           `json:"id"`
	SessionID string          `json:"session_id"`
	Prompt    string          `json:"prompt"`
	Request   json.RawMessage `json:"request"`
	Result    json.RawMessage `json:"result,omitempty"`
	Error     string          `json:"error,omitempty"`
	CreatedAt time.Time       `json:"created_at"`
}

type runPayload struct {
	Prompt  string          `json:"prompt"`
	Request json.RawMessage `json:"request"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   string          `json:"error,omitempty"`
}

func NewRepository(database *db.DB) *Repository {
	return &Repository{db: database}
}

func NormalizeScene(scene string) string {
	switch strings.ToLower(strings.TrimSpace(scene)) {
	case SceneAnalyze:
		return SceneAnalyze
	case SceneDecision:
		return SceneDecision
	case SceneStockPicker:
		return SceneStockPicker
	case SceneWorkflow:
		return SceneWorkflow
	case SceneChat:
		return SceneChat
	default:
		return ""
	}
}

func BuildSessionTitle(inputTitle, prompt, fallback string) string {
	if t := strings.TrimSpace(inputTitle); t != "" {
		return truncateRunes(t, 40)
	}
	if p := strings.TrimSpace(prompt); p != "" {
		return truncateRunes(p, 40)
	}
	if f := strings.TrimSpace(fallback); f != "" {
		return truncateRunes(f, 40)
	}
	return "未命名会话"
}

func truncateRunes(v string, maxRunes int) string {
	if maxRunes <= 0 || v == "" {
		return ""
	}
	if utf8.RuneCountInString(v) <= maxRunes {
		return v
	}
	rs := []rune(v)
	return string(rs[:maxRunes])
}

func ensureJSONOrNull(raw json.RawMessage) json.RawMessage {
	trimmed := strings.TrimSpace(string(raw))
	if trimmed == "" {
		return json.RawMessage("null")
	}
	if json.Valid([]byte(trimmed)) {
		return json.RawMessage(trimmed)
	}
	quoted, _ := json.Marshal(trimmed)
	return quoted
}

func (r *Repository) CreateSession(ctx context.Context, scene, title, userID string) (Session, error) {
	if r == nil || r.db == nil || r.db.DB == nil {
		return Session{}, errors.New("db_not_initialized")
	}
	scene = NormalizeScene(scene)
	if scene == "" {
		return Session{}, errors.New("invalid_scene")
	}
	if strings.TrimSpace(userID) == "" {
		userID = defaultUserID
	}
	id := uuid.NewString()
	title = strings.TrimSpace(title)
	_, err := r.db.ExecContext(ctx, `
		insert into conversation_sessions(id, user_id, scene, title)
		values (?, ?, ?, ?)
	`, id, userID, scene, title)
	if err != nil {
		return Session{}, err
	}
	return r.GetSession(ctx, id)
}

func (r *Repository) EnsureSession(ctx context.Context, sessionID, scene, title, userID string) (Session, error) {
	if r == nil || r.db == nil || r.db.DB == nil {
		return Session{}, errors.New("db_not_initialized")
	}
	scene = NormalizeScene(scene)
	if scene == "" {
		return Session{}, errors.New("invalid_scene")
	}
	if strings.TrimSpace(userID) == "" {
		userID = defaultUserID
	}
	sessionID = strings.TrimSpace(sessionID)
	title = strings.TrimSpace(title)
	if sessionID == "" {
		return r.CreateSession(ctx, scene, title, userID)
	}
	_, err := r.db.ExecContext(ctx, `
		insert or ignore into conversation_sessions(id, user_id, scene, title)
		values (?, ?, ?, ?)
	`, sessionID, userID, scene, title)
	if err != nil {
		return Session{}, err
	}
	if title != "" {
		_, _ = r.db.ExecContext(ctx, `
			update conversation_sessions
			set title = case
				when trim(title) = '' then ?
				else title
			end,
			updated_at = CURRENT_TIMESTAMP,
			user_id = ?
			where id = ? and deleted_at is null
		`, title, userID, sessionID)
	} else {
		_, _ = r.db.ExecContext(ctx, `
			update conversation_sessions
			set updated_at = CURRENT_TIMESTAMP, user_id = ?
			where id = ? and deleted_at is null
		`, userID, sessionID)
	}
	return r.GetSession(ctx, sessionID)
}

func (r *Repository) GetSession(ctx context.Context, sessionID string) (Session, error) {
	if r == nil || r.db == nil || r.db.DB == nil {
		return Session{}, errors.New("db_not_initialized")
	}
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return Session{}, errors.New("missing_session_id")
	}
	row := r.db.QueryRowContext(ctx, `
		select id, scene, title, created_at, updated_at
		from conversation_sessions
		where id = ? and deleted_at is null
	`, sessionID)
	var out Session
	var createdRaw, updatedRaw sql.NullString
	if err := row.Scan(&out.ID, &out.Scene, &out.Title, &createdRaw, &updatedRaw); err != nil {
		return Session{}, err
	}
	if parsed, ok := db.ParseTime(createdRaw); ok {
		out.CreatedAt = parsed
	}
	if parsed, ok := db.ParseTime(updatedRaw); ok {
		out.UpdatedAt = parsed
	}
	return out, nil
}

func (r *Repository) ListSessions(ctx context.Context, scene string, limit, offset int) ([]Session, error) {
	if r == nil || r.db == nil || r.db.DB == nil {
		return nil, errors.New("db_not_initialized")
	}
	scene = NormalizeScene(scene)
	if scene == "" {
		return nil, errors.New("invalid_scene")
	}
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}
	rows, err := r.db.QueryContext(ctx, `
		select id, scene, title, created_at, updated_at
		from conversation_sessions
		where scene = ? and deleted_at is null
		order by updated_at desc
		limit ? offset ?
	`, scene, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]Session, 0)
	for rows.Next() {
		var item Session
		var createdRaw, updatedRaw sql.NullString
		if err := rows.Scan(&item.ID, &item.Scene, &item.Title, &createdRaw, &updatedRaw); err != nil {
			return nil, err
		}
		if parsed, ok := db.ParseTime(createdRaw); ok {
			item.CreatedAt = parsed
		}
		if parsed, ok := db.ParseTime(updatedRaw); ok {
			item.UpdatedAt = parsed
		}
		out = append(out, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if out == nil {
		out = []Session{}
	}
	return out, nil
}

func (r *Repository) RenameSession(ctx context.Context, sessionID, title string) error {
	if r == nil || r.db == nil || r.db.DB == nil {
		return errors.New("db_not_initialized")
	}
	sessionID = strings.TrimSpace(sessionID)
	title = strings.TrimSpace(title)
	if sessionID == "" {
		return errors.New("missing_session_id")
	}
	res, err := r.db.ExecContext(ctx, `
		update conversation_sessions
		set title = ?, updated_at = CURRENT_TIMESTAMP
		where id = ? and deleted_at is null
	`, title, sessionID)
	if err != nil {
		return err
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func (r *Repository) SoftDeleteSession(ctx context.Context, sessionID string) error {
	if r == nil || r.db == nil || r.db.DB == nil {
		return errors.New("db_not_initialized")
	}
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return errors.New("missing_session_id")
	}
	res, err := r.db.ExecContext(ctx, `
		update conversation_sessions
		set deleted_at = CURRENT_TIMESTAMP, updated_at = CURRENT_TIMESTAMP
		where id = ? and deleted_at is null
	`, sessionID)
	if err != nil {
		return err
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func (r *Repository) AppendRun(ctx context.Context, sessionID, prompt string, requestJSON, resultJSON json.RawMessage, errorText string) error {
	if r == nil || r.db == nil || r.db.DB == nil {
		return errors.New("db_not_initialized")
	}
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return errors.New("missing_session_id")
	}
	if _, err := r.GetSession(ctx, sessionID); err != nil {
		return err
	}
	payload := runPayload{
		Prompt:  strings.TrimSpace(prompt),
		Request: ensureJSONOrNull(requestJSON),
		Result:  ensureJSONOrNull(resultJSON),
		Error:   strings.TrimSpace(errorText),
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `
		insert into conversation_messages(session_id, role, content, payload)
		values (?, ?, ?, ?)
	`, sessionID, "run", payload.Prompt, string(raw))
	if err != nil {
		_ = tx.Rollback()
		return err
	}
	_, err = tx.ExecContext(ctx, `
		update conversation_sessions
		set updated_at = CURRENT_TIMESTAMP
		where id = ? and deleted_at is null
	`, sessionID)
	if err != nil {
		_ = tx.Rollback()
		return err
	}
	return tx.Commit()
}

func (r *Repository) ListRuns(ctx context.Context, sessionID string, limit int) ([]Run, error) {
	if r == nil || r.db == nil || r.db.DB == nil {
		return nil, errors.New("db_not_initialized")
	}
	if _, err := r.GetSession(ctx, sessionID); err != nil {
		return nil, err
	}
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	rows, err := r.db.QueryContext(ctx, `
		select id, session_id, payload, created_at
		from conversation_messages
		where session_id = ? and role = ?
		order by id asc
		limit ?
	`, sessionID, "run", limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]Run, 0)
	for rows.Next() {
		var item Run
		var payloadRaw string
		var createdRaw sql.NullString
		if err := rows.Scan(&item.ID, &item.SessionID, &payloadRaw, &createdRaw); err != nil {
			return nil, err
		}
		var payload runPayload
		if err := json.Unmarshal([]byte(payloadRaw), &payload); err != nil {
			return nil, err
		}
		item.Prompt = payload.Prompt
		item.Request = ensureJSONOrNull(payload.Request)
		if strings.TrimSpace(string(payload.Result)) != "" {
			item.Result = ensureJSONOrNull(payload.Result)
		}
		item.Error = payload.Error
		if parsed, ok := db.ParseTime(createdRaw); ok {
			item.CreatedAt = parsed
		}
		out = append(out, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if out == nil {
		out = []Run{}
	}
	return out, nil
}
