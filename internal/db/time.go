package db

import (
	"database/sql"
	"time"
)

func ParseTime(value sql.NullString) (time.Time, bool) {
	if !value.Valid {
		return time.Time{}, false
	}
	text := value.String
	if text == "" {
		return time.Time{}, false
	}
	layouts := []string{
		time.RFC3339Nano,
		time.RFC3339,
		"2006-01-02 15:04:05",
	}
	for _, layout := range layouts {
		if parsed, err := time.Parse(layout, text); err == nil {
			return parsed, true
		}
	}
	return time.Time{}, false
}
