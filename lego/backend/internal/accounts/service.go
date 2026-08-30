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

// Package accounts owns irreversible self-deletion. It deliberately exposes
// REST and GraphQL but no MCP registration.
package accounts

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/apikeys"
	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

const Confirmation = "delete my account"

const (
	CodeBlocked     = "ACCOUNT_DELETION_BLOCKED"
	CodeUnavailable = "ACCOUNT_DELETION_UNAVAILABLE"
)

type Store interface {
	PreviewAccountDeletion(context.Context, string, []string) ([]store.AccountWorkspaceDisposition, error)
	BeginAccountDeletion(context.Context, string, string, []string) (store.AccountDeletion, error)
	ClaimAccountDeletions(context.Context, int) ([]store.AccountDeletion, error)
	AdvanceAccountDeletion(context.Context, string, string, string) error
	FailAccountDeletion(context.Context, string, string) error
	CleanupAccountSubject(context.Context, string, string) error
}

type WorkspaceTeardown interface {
	Delete(context.Context, string) error
}

type MemberOffboarder interface {
	Remove(context.Context, string, string) error
}

type MachineCredentials interface {
	List(context.Context) ([]apikeys.APIKey, error)
	CleanupSubject(context.Context, string) error
}

type SubjectCleaner interface {
	CleanupSubject(context.Context, string) error
}

type IdentityCleaner interface {
	DeleteSessions(context.Context, string) error
	DeleteIdentity(context.Context, string) error
}

type Preview struct {
	Delete  []store.AccountWorkspaceDisposition `json:"delete"`
	Leave   []store.AccountWorkspaceDisposition `json:"leave"`
	Blocked []store.AccountWorkspaceDisposition `json:"blocked"`
}

type DeletionView struct {
	State string `json:"state"`
}

type Service struct {
	*core.Base
	Store      Store
	Workspaces WorkspaceTeardown
	Members    MemberOffboarder
	APIKeys    MachineCredentials
	OAuth      SubjectCleaner
	Kratos     IdentityCleaner
}

func directSession(ctx context.Context) (core.Identity, error) {
	id, ok := core.IdentityFrom(ctx)
	if !ok || id.Subject == "" || id.Method != "session" || !id.Human {
		return core.Identity{}, core.NewForbiddenError(
			"ACCOUNT_DELETION_SESSION_REQUIRED",
			"account deletion requires a direct signed-in browser session",
			nil,
		)
	}
	return id, nil
}

func (s *Service) Preview(ctx context.Context) (Preview, error) {
	if err := s.Authorize(ctx, core.RelCanView); err != nil {
		return Preview{}, err
	}
	id, err := directSession(ctx)
	if err != nil {
		return Preview{}, err
	}
	if s.Store == nil {
		return Preview{}, core.NewUnavailableError(CodeUnavailable, "account deletion is not configured", nil)
	}
	preview, _, err := s.previewForSubject(ctx, id.Subject)
	return preview, err
}

func (s *Service) previewForSubject(ctx context.Context, subject string) (Preview, []string, error) {
	if s.APIKeys == nil {
		return Preview{}, nil, core.NewUnavailableError(CodeUnavailable, "API-key inventory is not configured", nil)
	}
	keys, err := s.APIKeys.List(ctx)
	if err != nil {
		return Preview{}, nil, err
	}
	machineSubjects := make([]string, 0, len(keys))
	for _, key := range keys {
		machineSubjects = append(machineSubjects, key.ID)
	}
	rows, err := s.Store.PreviewAccountDeletion(ctx, subject, machineSubjects)
	if err != nil {
		return Preview{}, nil, err
	}
	var out Preview
	for _, row := range rows {
		switch row.Action {
		case store.AccountWorkspaceDelete:
			out.Delete = append(out.Delete, row)
		case store.AccountWorkspaceLeave:
			out.Leave = append(out.Leave, row)
		case store.AccountWorkspaceBlocked:
			out.Blocked = append(out.Blocked, row)
		}
	}
	return out, machineSubjects, nil
}

func blockedError(rows []store.AccountWorkspaceDisposition) error {
	workspaces := make([]map[string]string, 0, len(rows))
	for _, row := range rows {
		workspaces = append(workspaces, map[string]string{"id": row.ID, "name": row.Name})
	}
	return core.NewConflictError(CodeBlocked,
		"Promote another workspace member to admin, remove the other members, or delete the blocking workspace first.",
		map[string]any{"workspaces": workspaces})
}

func (s *Service) Request(ctx context.Context, confirmation string) (DeletionView, error) {
	if err := s.Authorize(ctx, core.RelCanView); err != nil {
		return DeletionView{}, err
	}
	id, err := directSession(ctx)
	if err != nil {
		return DeletionView{}, err
	}
	if confirmation != Confirmation {
		return DeletionView{}, core.NewBadRequestError("ACCOUNT_DELETION_CONFIRMATION",
			fmt.Sprintf("confirmation must be %q", Confirmation), nil)
	}
	if s.Store == nil {
		return DeletionView{}, core.NewUnavailableError(CodeUnavailable, "account deletion is not configured", nil)
	}
	preview, machineSubjects, err := s.previewForSubject(ctx, id.Subject)
	if err != nil {
		return DeletionView{}, err
	}
	if len(preview.Blocked) > 0 {
		return DeletionView{}, blockedError(preview.Blocked)
	}
	d, err := s.Store.BeginAccountDeletion(ctx, id.Subject, id.Email, machineSubjects)
	var blocked *store.AccountDeletionBlockedError
	if errors.As(err, &blocked) {
		return DeletionView{}, blockedError(blocked.Workspaces)
	}
	if err != nil {
		return DeletionView{}, err
	}
	return DeletionView{State: d.State}, nil
}

func (s *Service) process(ctx context.Context, d store.AccountDeletion) error {
	for {
		switch d.State {
		case store.AccountDeletionPending:
			if s.APIKeys == nil {
				return core.NewUnavailableError(CodeUnavailable, "API-key cleanup is not configured", nil)
			}
			if err := s.APIKeys.CleanupSubject(ctx, d.Subject); err != nil {
				return err
			}
			if s.OAuth == nil {
				return core.NewUnavailableError(CodeUnavailable, "OAuth cleanup is not configured", nil)
			}
			if err := s.OAuth.CleanupSubject(ctx, d.Subject); err != nil {
				return err
			}
			if err := s.Store.AdvanceAccountDeletion(ctx, d.Subject, d.State, store.AccountDeletionCleaning); err != nil {
				return err
			}
			d.State = store.AccountDeletionCleaning
		case store.AccountDeletionCleaning:
			if s.Workspaces == nil || s.Members == nil {
				return core.NewUnavailableError(CodeUnavailable, "workspace cleanup is not configured", nil)
			}
			for _, workspace := range d.Workspaces {
				switch workspace.Action {
				case store.AccountWorkspaceDelete:
					if err := s.Workspaces.Delete(ctx, workspace.ID); err != nil {
						return err
					}
				case store.AccountWorkspaceLeave:
					if err := s.Members.Remove(ctx, workspace.ID, d.Subject); err != nil {
						return err
					}
				}
			}
			if err := s.Store.CleanupAccountSubject(ctx, d.Subject, d.DeletedMarker); err != nil {
				return err
			}
			if err := s.Store.AdvanceAccountDeletion(ctx, d.Subject, d.State, store.AccountDeletionIdentity); err != nil {
				return err
			}
			d.State = store.AccountDeletionIdentity
		case store.AccountDeletionIdentity:
			if s.Kratos == nil {
				return core.NewUnavailableError(CodeUnavailable, "identity cleanup is not configured", nil)
			}
			if err := s.Kratos.DeleteSessions(ctx, d.Subject); err != nil {
				return err
			}
			if err := s.Kratos.DeleteIdentity(ctx, d.Subject); err != nil {
				return err
			}
			return s.Store.AdvanceAccountDeletion(ctx, d.Subject, d.State, store.AccountDeletionDone)
		case store.AccountDeletionDone:
			return nil
		default:
			return fmt.Errorf("unknown account deletion state %q", d.State)
		}
	}
}

func (s *Service) processPending(ctx context.Context, limit int) error {
	if s.Store == nil {
		return nil
	}
	rows, err := s.Store.ClaimAccountDeletions(ctx, limit)
	if err != nil {
		return err
	}
	for _, row := range rows {
		if err := s.process(ctx, row); err != nil {
			_ = s.Store.FailAccountDeletion(ctx, row.Subject, err.Error())
			if errors.Is(err, context.Canceled) {
				return err
			}
		}
	}
	return nil
}

func (s *Service) Run(ctx context.Context) {
	if s.Store == nil {
		return
	}
	core.Poll(ctx, "accounts: deletion sweep", 5*time.Second,
		func(ctx context.Context) error { return s.processPending(ctx, 1) })
}
