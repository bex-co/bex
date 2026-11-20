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
	"testing"
)

// An account's theme settings (w3/m531): defaults with no row, each column
// written alone keeps the other, the CHECKs hold Render's enums, and account
// cleanup removes the row.
func TestUserThemeSettingsPG(t *testing.T) {
	st := newReplayTestStore(t)
	ctx := context.Background()
	const subject = "identity-theme-a"
	if _, err := st.Pool.Exec(ctx, `DELETE FROM user_theme_settings WHERE subject IN ($1, 'identity-theme-b')`, subject); err != nil {
		t.Fatal(err)
	}

	got, err := st.GetUserThemeSettings(ctx, subject)
	if err != nil || got != DefaultUserThemeSettings {
		t.Fatalf("never chose = %+v, %v; want SYSTEM and MATCH_APP", got, err)
	}

	got, err = st.SetUserLogThemeSetting(ctx, subject, LogThemeSettingAlwaysDark)
	if err != nil || got != (UserThemeSettings{ThemeSetting: ThemeSettingSystem, LogThemeSetting: LogThemeSettingAlwaysDark}) {
		t.Fatalf("log theme first = %+v, %v", got, err)
	}
	got, err = st.SetUserThemeSetting(ctx, subject, ThemeSettingDarkHighContrast)
	if err != nil || got != (UserThemeSettings{ThemeSetting: ThemeSettingDarkHighContrast, LogThemeSetting: LogThemeSettingAlwaysDark}) {
		t.Fatalf("theme kept the log theme = %+v, %v", got, err)
	}
	got, err = st.SetUserLogThemeSetting(ctx, subject, LogThemeSettingMatchApp)
	if err != nil || got != (UserThemeSettings{ThemeSetting: ThemeSettingDarkHighContrast, LogThemeSetting: LogThemeSettingMatchApp}) {
		t.Fatalf("log theme kept the theme = %+v, %v", got, err)
	}
	if read, err := st.GetUserThemeSettings(ctx, subject); err != nil || read != got {
		t.Fatalf("read back = %+v, %v; want %+v", read, err, got)
	}
	// Another account's settings are its own.
	if other, err := st.GetUserThemeSettings(ctx, "identity-theme-b"); err != nil || other != DefaultUserThemeSettings {
		t.Fatalf("other account = %+v, %v", other, err)
	}

	for _, write := range []func() (UserThemeSettings, error){
		func() (UserThemeSettings, error) { return st.SetUserThemeSetting(ctx, subject, "dark") },
		func() (UserThemeSettings, error) { return st.SetUserThemeSetting(ctx, subject, "HIGH_CONTRAST") },
		func() (UserThemeSettings, error) { return st.SetUserLogThemeSetting(ctx, subject, "DARK") },
	} {
		if _, err := write(); !errors.Is(err, ErrInvalid) {
			t.Fatalf("a value outside the enum = %v, want ErrInvalid", err)
		}
	}
	if read, _ := st.GetUserThemeSettings(ctx, subject); read != got {
		t.Fatalf("a refused write changed the row: %+v", read)
	}

	if err := st.CleanupAccountSubject(ctx, subject, "deleted-theme-marker"); err != nil {
		t.Fatalf("cleanup: %v", err)
	}
	var rows int
	if err := st.Pool.QueryRow(ctx, `SELECT count(*) FROM user_theme_settings WHERE subject = $1`, subject).Scan(&rows); err != nil {
		t.Fatal(err)
	}
	if rows != 0 {
		t.Fatalf("theme settings survived account cleanup: %d rows", rows)
	}
}

// A write while the account's deletion is pending is refused, so it cannot
// recreate the row the deletion's cleanup removes.
func TestUserThemeSettingsRefusedWhileDeletionPendingPG(t *testing.T) {
	st := newReplayTestStore(t)
	ctx := context.Background()
	const subject = "identity-theme-deleting"
	if _, err := st.Pool.Exec(ctx, `DELETE FROM account_deletions WHERE subject = $1`, subject); err != nil {
		t.Fatal(err)
	}
	if _, err := st.BeginAccountDeletion(ctx, subject, "deleting@example.com", nil); err != nil {
		t.Fatalf("begin deletion: %v", err)
	}
	t.Cleanup(func() {
		_, _ = st.Pool.Exec(context.Background(), `DELETE FROM account_deletions WHERE subject = $1`, subject)
	})
	if _, err := st.SetUserThemeSetting(ctx, subject, ThemeSettingDark); !errors.Is(err, ErrAccountDeletionPending) {
		t.Fatalf("theme write after the tombstone = %v, want ErrAccountDeletionPending", err)
	}
	if _, err := st.SetUserLogThemeSetting(ctx, subject, LogThemeSettingAlwaysDark); !errors.Is(err, ErrAccountDeletionPending) {
		t.Fatalf("log theme write after the tombstone = %v, want ErrAccountDeletionPending", err)
	}
}
