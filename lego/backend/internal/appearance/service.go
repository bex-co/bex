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

// Package appearance owns an account's Dashboard Theme (with High Contrast
// Mode, its `_HIGH_CONTRAST` twin) and Log Explorer Theme, kept on the account
// the way Render keeps them on its User (w3/m531,
// docs/render-artifacts/account-appearance.md). GraphQL serves Render's own
// `user { id themeSetting logThemeSetting }`, `updateUserThemeSetting` and
// `updateUserLogThemeSetting`; REST serves the same two values as a bex
// extension. Render's MCP server has no theme tool, so there is none here.
package appearance

import (
	"context"
	"errors"
	"slices"

	"github.com/bex-co/bex/lego/backend/internal/core"
	ids "github.com/bex-co/bex/lego/backend/internal/id"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// Store is the account's settings row and the owner id its public user id
// spells; *store.PGStore satisfies it.
type Store interface {
	GetUserThemeSettings(ctx context.Context, subject string) (store.UserThemeSettings, error)
	SetUserThemeSetting(ctx context.Context, subject, value string) (store.UserThemeSettings, error)
	SetUserLogThemeSetting(ctx context.Context, subject, value string) (store.UserThemeSettings, error)
	OwnerIDForSubject(ctx context.Context, subject string) (string, error)
}

type Service struct {
	*core.Base
	Store Store
}

// Settings is the REST body: the account's two values in Render's names.
type Settings struct {
	ThemeSetting    string `json:"themeSetting"`
	LogThemeSetting string `json:"logThemeSetting"`
}

// Patch is a write: a field left nil keeps its stored value. It is REST's
// PATCH body as it comes.
type Patch struct {
	ThemeSetting    *string `json:"themeSetting"`
	LogThemeSetting *string `json:"logThemeSetting"`
}

// User is GraphQL's `User`: the caller's public user id (the usr- id GET
// /v1/users answers) and the two settings.
type User struct {
	ID string
	Settings
}

const codeUnavailable = "THEME_SETTINGS_UNAVAILABLE"

// caller authorizes a verb on the caller's own account. The settings are the
// account's, not a workspace's, so the check is the plain signed-in one. Only
// a person has a theme: an API key's machine identity is refused.
func (s *Service) caller(ctx context.Context) (string, error) {
	if err := s.Authorize(ctx, core.RelCanView); err != nil {
		return "", err
	}
	id, ok := core.IdentityFrom(ctx)
	if !ok || id.Subject == "" || !id.Human {
		return "", core.NewForbiddenError("THEME_SETTINGS_PERSON_REQUIRED",
			"theme settings belong to a signed-in person, not an API key", nil)
	}
	if s.Store == nil {
		return "", core.NewUnavailableError(codeUnavailable, "theme settings are not configured", nil)
	}
	return id.Subject, nil
}

// Get reads the caller's settings: SYSTEM and MATCH_APP when they never chose.
func (s *Service) Get(ctx context.Context) (Settings, error) {
	subject, err := s.caller(ctx)
	if err != nil {
		return Settings{}, err
	}
	row, err := s.Store.GetUserThemeSettings(ctx, subject)
	return view(row), err
}

// GetUser is Get with the caller's public user id, GraphQL's `user`.
func (s *Service) GetUser(ctx context.Context) (User, error) {
	subject, err := s.caller(ctx)
	if err != nil {
		return User{}, err
	}
	userID, err := s.userID(ctx, subject)
	if err != nil {
		return User{}, err
	}
	row, err := s.Store.GetUserThemeSettings(ctx, subject)
	if err != nil {
		return User{}, err
	}
	return User{ID: userID, Settings: view(row)}, nil
}

// Update writes whichever of the two the patch carries, each checked against
// its enum, and returns both. A patch with neither changes nothing.
func (s *Service) Update(ctx context.Context, patch Patch) (Settings, error) {
	subject, err := s.caller(ctx)
	if err != nil {
		return Settings{}, err
	}
	return s.update(ctx, subject, patch)
}

func (s *Service) update(ctx context.Context, subject string, patch Patch) (Settings, error) {
	if patch.ThemeSetting != nil && !slices.Contains(store.ThemeSettings, *patch.ThemeSetting) {
		return Settings{}, invalidValue("themeSetting", store.ThemeSettings)
	}
	if patch.LogThemeSetting != nil && !slices.Contains(store.LogThemeSettings, *patch.LogThemeSetting) {
		return Settings{}, invalidValue("logThemeSetting", store.LogThemeSettings)
	}
	if patch.ThemeSetting == nil && patch.LogThemeSetting == nil {
		row, err := s.Store.GetUserThemeSettings(ctx, subject)
		return view(row), err
	}
	// Each write returns the whole row; the last one is the answer.
	var row store.UserThemeSettings
	var err error
	if patch.ThemeSetting != nil {
		if row, err = s.Store.SetUserThemeSetting(ctx, subject, *patch.ThemeSetting); err != nil {
			return Settings{}, mapStoreErr(err)
		}
	}
	if patch.LogThemeSetting != nil {
		if row, err = s.Store.SetUserLogThemeSetting(ctx, subject, *patch.LogThemeSetting); err != nil {
			return Settings{}, mapStoreErr(err)
		}
	}
	return view(row), nil
}

// UpdateUser is GraphQL's two mutations: Render's dashboard names the user it
// writes, and the only user a caller can write is themselves, so any other id
// reads as a user that does not exist.
func (s *Service) UpdateUser(ctx context.Context, userID string, patch Patch) (User, error) {
	subject, err := s.caller(ctx)
	if err != nil {
		return User{}, err
	}
	own, err := s.userID(ctx, subject)
	if err != nil {
		return User{}, err
	}
	if userID != own {
		return User{}, core.NotFound("user")
	}
	settings, err := s.update(ctx, subject, patch)
	if err != nil {
		return User{}, err
	}
	return User{ID: own, Settings: settings}, nil
}

func (s *Service) userID(ctx context.Context, subject string) (string, error) {
	ownID, err := s.Store.OwnerIDForSubject(ctx, subject)
	return ids.UserPublicID(ownID), err
}

func view(row store.UserThemeSettings) Settings {
	return Settings{ThemeSetting: row.ThemeSetting, LogThemeSetting: row.LogThemeSetting}
}

func invalidValue(field string, allowed []string) error {
	return core.NewBadRequestError("INVALID_THEME_SETTING", field+" must be one of the listed values",
		map[string]any{"field": field, "allowed": allowed})
}

// mapStoreErr answers a write refused because the account is being deleted.
// A value outside the enums never reaches the store: update checks it first.
func mapStoreErr(err error) error {
	if errors.Is(err, store.ErrAccountDeletionPending) {
		return core.NewAccountDeletionPendingError()
	}
	return err
}
