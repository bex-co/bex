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
	"strings"
)

// BranchHeads reads a session branch's head on GitHub through the workspace's
// connection for the repository's owner, the installation the session's git
// token is minted from (w5/m382). The completer records this head rather
// than the one the sandbox reports about itself.
type BranchHeads struct {
	GitHub interface {
		BranchHead(ctx context.Context, installationID int64, owner, repo, branch string) (string, error)
	}
	Connections ConnectionStore
}

// Head returns branch's head commit in workspaceID's repository.
func (b BranchHeads) Head(ctx context.Context, workspaceID, repository, branch string) (string, error) {
	repository, err := NormalizeRepository(repository)
	if err != nil {
		return "", err
	}
	owner, name, _ := strings.Cut(repository, "/")
	connection, err := b.Connections.GetGitConnectionByOwner(ctx, workspaceID, owner)
	if err != nil {
		return "", err
	}
	return b.GitHub.BranchHead(ctx, connection.InstallationID, owner, name, branch)
}
