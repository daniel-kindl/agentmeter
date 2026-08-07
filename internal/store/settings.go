package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
)

// SettingLiveLimits records whether the operator has turned on authoritative
// limit fetching. It is stored rather than held in memory so that a choice made
// in the dashboard survives a restart.
const SettingLiveLimits = "limits.live"

// Setting returns a stored preference and whether it was present.
func (s *Store) Setting(ctx context.Context, key string) (string, bool, error) {
	var value string
	err := s.db.QueryRowContext(ctx, "SELECT value FROM settings WHERE key = ?", key).Scan(&value)
	if errors.Is(err, sql.ErrNoRows) {
		return "", false, nil
	}
	if err != nil {
		return "", false, fmt.Errorf("read setting %q: %w", key, err)
	}
	return value, true, nil
}

// SetSetting stores a preference, replacing any previous value.
func (s *Store) SetSetting(ctx context.Context, key, value string) error {
	if strings.TrimSpace(key) == "" {
		return errors.New("set setting: key is blank")
	}
	const upsert = `INSERT INTO settings (key, value) VALUES (?, ?)
ON CONFLICT(key) DO UPDATE SET value = excluded.value`
	if _, err := s.db.ExecContext(ctx, upsert, key, value); err != nil {
		return fmt.Errorf("write setting %q: %w", key, err)
	}
	return nil
}
