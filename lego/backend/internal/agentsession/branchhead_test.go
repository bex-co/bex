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

package agentsession

import (
	"context"
	"errors"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/store"
)

type headReader struct{ calls []string }

func (h *headReader) BranchHead(_ context.Context, installationID int64, owner, repo, branch string) (string, error) {
	h.calls = append(h.calls, owner+"/"+repo+" "+branch)
	if installationID != 42 {
		return "", errors.New("wrong installation")
	}
	return "0123456789abcdef0123456789abcdef01234567", nil
}

// The head is read from the installation the workspace connected for the
// repository's owner, the one the session's git token comes from; a workspace
// with no connection for that owner reads nothing.
func TestBranchHeadsReadsThroughTheOwnersConnection(t *testing.T) {
	gh := &headReader{}
	heads := BranchHeads{GitHub: gh, Connections: fakeConnections{row: store.GitConnection{AccountLogin: "Octo", InstallationID: 42}}}
	head, err := heads.Head(context.Background(), "tea-a", "Octo/repo", "bex-agent/s1")
	if err != nil || head != "0123456789abcdef0123456789abcdef01234567" || len(gh.calls) != 1 || gh.calls[0] != "octo/repo bex-agent/s1" {
		t.Fatalf("head = %q %v, calls %v", head, err, gh.calls)
	}
	if _, err := heads.Head(context.Background(), "tea-a", "mallory/repo", "bex-agent/s1"); !errors.Is(err, store.ErrNotFound) || len(gh.calls) != 1 {
		t.Fatalf("another owner's repository = %v after %d reads, want ErrNotFound and no read", err, len(gh.calls))
	}
}
