/*
Copyright 2026.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package store

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
)

// Render's ThemeSetting and LogThemeSetting (w3/m531,
// docs/render-artifacts/account-appearance.md). The migration's CHECKs hold
// the same lists.
const (
	ThemeSettingSystem             = "SYSTEM"
	ThemeSettingLight              = "LIGHT"
	ThemeSettingDark               = "DARK"
	ThemeSettingSystemHighContrast = "SYSTEM_HIGH_CONTRAST"
	ThemeSettingLightHighContrast  = "LIGHT_HIGH_CONTRAST"
	ThemeSettingDarkHighContrast   = "DARK_HIGH_CONTRAST"

	LogThemeSettingMatchApp    = "MATCH_APP"
	LogThemeSettingAlwaysLight = "ALWAYS_LIGHT"
	LogThemeSettingAlwaysDark  = "ALWAYS_DARK"
)

// ThemeSettings and LogThemeSettings list each enum in Render's order.
var (
	ThemeSettings = []string{
		ThemeSettingSystem, ThemeSettingLight, ThemeSettingDark,
		ThemeSettingSystemHighContrast, ThemeSettingLightHighContrast, ThemeSettingDarkHighContrast,
	}
	LogThemeSettings = []string{LogThemeSettingMatchApp, LogThemeSettingAlwaysLight, LogThemeSettingAlwaysDark}
)

// UserThemeSettings is an account's Dashboard Theme (with its high-contrast
// twin) and Log Explorer Theme.
type UserThemeSettings struct {
	ThemeSetting    string
	LogThemeSetting string
}

// DefaultUserThemeSettings is what an account that never chose reads.
var DefaultUserThemeSettings = UserThemeSettings{ThemeSetting: ThemeSettingSystem, LogThemeSetting: LogThemeSettingMatchApp}

// GetUserThemeSettings reads an account's theme settings, the defaults when
// it has no row.
func (s *PGStore) GetUserThemeSettings(ctx context.Context, subject string) (UserThemeSettings, error) {
	var out UserThemeSettings
	err := s.Pool.QueryRow(ctx,
		`SELECT theme_setting, log_theme_setting FROM user_theme_settings WHERE subject = $1`, subject,
	).Scan(&out.ThemeSetting, &out.LogThemeSetting)
	if errors.Is(err, pgx.ErrNoRows) {
		return DefaultUserThemeSettings, nil
	}
	if err != nil {
		return UserThemeSettings{}, classify("user theme settings", err)
	}
	return out, nil
}

// SetUserThemeSetting stores an account's Dashboard Theme and keeps its Log
// Explorer Theme.
func (s *PGStore) SetUserThemeSetting(ctx context.Context, subject, value string) (UserThemeSettings, error) {
	return s.upsertUserThemeSettings(ctx, subject, "theme_setting", value)
}

// SetUserLogThemeSetting stores an account's Log Explorer Theme and keeps its
// Dashboard Theme.
func (s *PGStore) SetUserLogThemeSetting(ctx context.Context, subject, value string) (UserThemeSettings, error) {
	return s.upsertUserThemeSettings(ctx, subject, "log_theme_setting", value)
}

// upsertUserThemeSettings writes one of the two columns, creating the row
// with the other at its default. It refuses an account whose deletion is
// pending, so a write cannot recreate the row its cleanup removes.
func (s *PGStore) upsertUserThemeSettings(ctx context.Context, subject, column, value string) (UserThemeSettings, error) {
	var out UserThemeSettings
	err := pgx.BeginFunc(ctx, s.Pool, func(tx pgx.Tx) error {
		if err := lockSubjectMembership(ctx, tx, subject); err != nil {
			return err
		}
		if err := refuseDeletingSubject(ctx, tx, subject); err != nil {
			return err
		}
		// column is one of the two names the setters above pass, never caller
		// input.
		return tx.QueryRow(ctx, fmt.Sprintf(`
			INSERT INTO user_theme_settings (subject, %[1]s) VALUES ($1, $2)
			ON CONFLICT (subject) DO UPDATE SET %[1]s = EXCLUDED.%[1]s, updated_at = now()
			RETURNING theme_setting, log_theme_setting`, column),
			subject, value,
		).Scan(&out.ThemeSetting, &out.LogThemeSetting)
	})
	if err != nil {
		// classify passes ErrAccountDeletionPending through as it is.
		return UserThemeSettings{}, classify("user theme settings", err)
	}
	return out, nil
}
