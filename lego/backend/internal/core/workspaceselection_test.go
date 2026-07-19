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

package core

import (
	"context"
	"errors"
	"sync"
	"testing"
)

type fakeMCPWorkspaceSelectionStore struct {
	mu     sync.Mutex
	byKey  map[string]string
	getErr error
	setErr error
}

func (s *fakeMCPWorkspaceSelectionStore) GetMCPWorkspaceSelection(_ context.Context, sessionID, subject string) (string, bool, error) {
	if s.getErr != nil {
		return "", false, s.getErr
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	id, ok := s.byKey[sessionID+"\x00"+subject]
	return id, ok, nil
}

func (s *fakeMCPWorkspaceSelectionStore) SetMCPWorkspaceSelection(_ context.Context, sessionID, subject, workspaceID string) error {
	if s.setErr != nil {
		return s.setErr
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.byKey[sessionID+"\x00"+subject] = workspaceID
	return nil
}

func TestWorkspaceSelectionsSharedStoreScopesBySubject(t *testing.T) {
	shared := &fakeMCPWorkspaceSelectionStore{byKey: map[string]string{}}
	selections := NewWorkspaceSelections(shared)
	alice := WithIdentity(context.Background(), Identity{Subject: "alice"})
	bob := WithIdentity(context.Background(), Identity{Subject: "bob"})

	if err := selections.Set(alice, "session-1", "tea-alice"); err != nil {
		t.Fatalf("Set alice: %v", err)
	}
	if got, ok, err := selections.Get(alice, "session-1"); err != nil || !ok || got != "tea-alice" {
		t.Fatalf("Get alice = %q, %v, %v", got, ok, err)
	}
	if got, ok, err := selections.Get(bob, "session-1"); err != nil || ok || got != "" {
		t.Fatalf("Get bob = %q, %v, %v; want no cross-subject selection", got, ok, err)
	}
}

func TestWorkspaceSelectionsSharedStoreFailsClosed(t *testing.T) {
	backendErr := errors.New("database unavailable")
	shared := &fakeMCPWorkspaceSelectionStore{byKey: map[string]string{}, getErr: backendErr}
	selections := NewWorkspaceSelections(shared)
	ctx := WithIdentity(context.Background(), Identity{Subject: "alice"})

	if _, _, err := selections.Get(ctx, "session-1"); !errors.Is(err, backendErr) {
		t.Fatalf("Get error = %v, want wrapped backend error", err)
	}
	if err := selections.Set(context.Background(), "session-1", "tea-alice"); !errors.Is(err, ErrForbidden) {
		t.Fatalf("Set without identity = %v, want ErrForbidden", err)
	}
}

func TestWorkspaceSelectionsLocalFallbackNeedsNoIdentity(t *testing.T) {
	selections := NewWorkspaceSelections(nil)
	ctx := context.Background()
	if err := selections.Set(ctx, "", "tea-local"); err != nil {
		t.Fatalf("Set local: %v", err)
	}
	if got, ok, err := selections.Get(ctx, ""); err != nil || !ok || got != "tea-local" {
		t.Fatalf("Get local = %q, %v, %v", got, ok, err)
	}
}
