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

package sshgateway

import (
	"context"
	"strings"

	"github.com/bex-co/bex/lego/backend/internal/apps"
	ids "github.com/bex-co/bex/lego/backend/internal/id"
)

// CompositeResolver routes an SSH username to the resolver that owns its id kind
// (ADR054 D1): a well-formed agent-session id (`ags-<xid>`) goes to the
// agent-session sandbox resolver; everything else — App ids (`srv-<xid>` /
// `srv-<xid>-<instance>`) AND malformed usernames — goes to Apps, whose
// ResolveSSHSession keeps its authorize-before-parse contract unchanged. Routing
// only a POSITIVELY-identified ags id away from Apps is what makes the srv-…
// path byte-identical to before this feature.
type CompositeResolver struct {
	Apps          TargetResolver
	AgentSessions TargetResolver
}

func (c CompositeResolver) ResolveSSHSession(ctx context.Context, username string) (apps.SSHInstanceTarget, error) {
	if c.AgentSessions != nil {
		if kind, ok := ids.KindOf(idPrefix(username)); ok && kind == ids.AgentSession {
			return c.AgentSessions.ResolveSSHSession(ctx, username)
		}
	}
	return c.Apps.ResolveSSHSession(ctx, username)
}

// idPrefix returns the `<prefix>-<xid>` head of an SSH username, discarding any
// instance suffix (srv-…-<instance>) so KindOf sees a well-formed id. An input
// with fewer than two hyphen-separated segments returns "" (no id kind), which
// routes to Apps and its existing bad-request handling.
func idPrefix(username string) string {
	parts := strings.SplitN(strings.TrimSpace(username), "-", 3)
	if len(parts) < 2 {
		return ""
	}
	return parts[0] + "-" + parts[1]
}
