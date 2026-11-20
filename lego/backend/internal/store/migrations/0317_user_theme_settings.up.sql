-- 0317: an account's Dashboard Theme and Log Explorer Theme (w3/m531), kept on
-- the account the way Render keeps them on its User (`themeSetting`,
-- `logThemeSetting`; docs/render-artifacts/account-appearance.md), so a browser
-- that signs in for the first time opens in the account's theme.
--
--   subject            the account (the Kratos identity tenant_members keys
--                      on). The theme is the account's, not a workspace's, so
--                      it is keyed by subject alone.
--   theme_setting      Render's ThemeSetting. High Contrast Mode is not a
--                      column of its own: it is the `_HIGH_CONTRAST` twin of the
--                      base theme, as Render stores it.
--   log_theme_setting  Render's LogThemeSetting: MATCH_APP follows the
--                      dashboard, ALWAYS_LIGHT and ALWAYS_DARK force every log
--                      explorer into that theme.
--
-- An account with no row reads SYSTEM and MATCH_APP. The row is personal
-- state: account deletion deletes it (store.CleanupAccountSubject).
CREATE TABLE user_theme_settings (
    subject           text PRIMARY KEY,
    theme_setting     text NOT NULL DEFAULT 'SYSTEM'
        CHECK (theme_setting IN ('SYSTEM', 'LIGHT', 'DARK', 'SYSTEM_HIGH_CONTRAST', 'LIGHT_HIGH_CONTRAST', 'DARK_HIGH_CONTRAST')),
    log_theme_setting text NOT NULL DEFAULT 'MATCH_APP'
        CHECK (log_theme_setting IN ('MATCH_APP', 'ALWAYS_LIGHT', 'ALWAYS_DARK')),
    updated_at        timestamptz NOT NULL DEFAULT now()
);
