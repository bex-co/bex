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

// Package identity is the one derivation for workspace-scoped shared-sink
// names (w2/m75, docs/ADR074-workspace-scoped-artifact-identity.md): Zot
// repository, htpasswd user, pull Secret, static-site prefix, and the
// future build-cache repository. Call sites must not concatenate App.Name
// into those identities themselves.
package identity

import (
	"crypto/sha256"
	"fmt"
	"strings"
)

const (
	// AnnotTombstone on an App means the migration tool has copied+verified
	// this App's artifacts onto the workspace-scoped scheme and dual-read of
	// the legacy location is closed. Value is TombstoneValue.
	AnnotTombstone = "app.bex.co/identity-tombstone"
	TombstoneValue = "true"

	// TombstoneTag is the marker tag left on a migrated legacy Zot repository.
	// It points at an already-copied digest so it creates no new blob.
	TombstoneTag = "bex-tombstone"

	// TombstoneObject is the marker object left at the legacy S3 prefix root.
	TombstoneObject = ".bex-tombstone"

	maxSecretName = 253
	hashLength    = 12
)

// Identity is one App's shared-sink naming inputs. Workspace empty selects
// the legacy (pre-m75) column. Tombstoned closes dual-read of the legacy
// location after a verified migration.
type Identity struct {
	Name       string
	Workspace  string
	Tombstoned bool
}

// ForApp builds an Identity from metadata.name and the app.bex.co/workspace
// label value. An empty workspace is the unlabeled/legacy path.
func ForApp(name, workspace string) Identity {
	return Identity{Name: name, Workspace: workspace}
}

// Key is the process-local map key for activation/cred state. Unlabeled Apps
// keep using the bare name so existing memoization stays byte-identical.
func (id Identity) Key() string {
	if id.Workspace == "" {
		return id.Name
	}
	return id.Workspace + "/" + id.Name
}

// Scoped reports whether this Identity uses the workspace-scoped column.
func (id Identity) Scoped() bool { return id.Workspace != "" }

// Repo is the OCI repository path (no registry host, no tag).
func (id Identity) Repo() string {
	if !id.Scoped() {
		return id.Name
	}
	return id.Workspace + "/" + id.Name
}

// LegacyRepo is the pre-m75 repository (always the bare App name). Dual-read
// and tombstone target this path.
func (id Identity) LegacyRepo() string { return id.Name }

// CacheRepo is the w7/m86 build-cache repository: the last path component of
// Repo() plus "-cache". Not minted by m75; the helper exists so m86 cannot
// invent a second scheme.
func (id Identity) CacheRepo() string { return id.Repo() + "-cache" }

// ZotUsername is the htpasswd user for this App's repository ACL.
func (id Identity) ZotUsername() string {
	if !id.Scoped() {
		return "app-" + id.Name
	}
	return "app-" + id.Workspace + "-" + id.Name
}

// LegacyCacheRepo is the build cache of the pre-m75 repository. It exists only
// for teardown: an App that built while unlabeled owns a "<name>-cache" grant,
// and without this the entry outlives the App in a document shared by the whole
// estate. Nothing mints it — CacheRepo is the only cache a live App uses.
func (id Identity) LegacyCacheRepo() string { return id.LegacyRepo() + "-cache" }

// LegacyZotUsername is the pre-m75 htpasswd user.
func (id Identity) LegacyZotUsername() string { return "app-" + id.Name }

// PullSecretName is the kubernetes.io/dockerconfigjson Secret in the App
// namespace (and its build-namespace mirror). Truncated to 253 with a stable
// hash suffix when the concatenated name would overflow.
func (id Identity) PullSecretName() string {
	raw := "reg-pull-" + id.Name
	if id.Scoped() {
		raw = "reg-pull-" + id.Workspace + "-" + id.Name
	}
	return secretName(raw, id.Workspace, id.Name)
}

// LegacyPullSecretName is the pre-m75 Secret name.
func (id Identity) LegacyPullSecretName() string {
	return secretName("reg-pull-"+id.Name, "", id.Name)
}

// StaticAppPrefix is the object-key prefix root (no revision, no trailing
// slash) this App publishes under.
func (id Identity) StaticAppPrefix() string {
	if !id.Scoped() {
		return id.Name
	}
	return id.Workspace + "/" + id.Name
}

// LegacyStaticAppPrefix is the pre-m75 prefix root.
func (id Identity) LegacyStaticAppPrefix() string { return id.Name }

// StaticPrefix is the full object-key prefix a revision publishes under,
// including the trailing slash: "<app-prefix>/<revision>/".
func (id Identity) StaticPrefix(revision string) string {
	if revision == "" {
		revision = "latest"
	}
	return id.StaticAppPrefix() + "/" + revision + "/"
}

// LegacyStaticPrefix is the pre-m75 full prefix for revision.
func (id Identity) LegacyStaticPrefix(revision string) string {
	if revision == "" {
		revision = "latest"
	}
	return id.LegacyStaticAppPrefix() + "/" + revision + "/"
}

// DualRead reports whether the operator should still grant the new user
// read on the legacy repo (labeled, not yet tombstoned, and the two repos
// actually differ).
func (id Identity) DualRead() bool {
	return id.Scoped() && !id.Tombstoned && id.Repo() != id.LegacyRepo()
}

// AppLevelPrefix strips the trailing revision segment from a full static
// prefix ("tea-x/web/rev-7/" → "tea-x/web/", "web/rev-7/" → "web/").
func AppLevelPrefix(revisionPrefix string) string {
	p := strings.Trim(revisionPrefix, "/")
	if p == "" {
		return ""
	}
	if i := strings.LastIndex(p, "/"); i >= 0 {
		return p[:i] + "/"
	}
	return p + "/"
}

// PurgePrefixes are the S3 prefix roots a delete-time purge must remove:
// the scheme-computed app prefix, the recorded published prefix's app
// level (so a dual-read site is fully reclaimed), and — only when
// tombstoned — the legacy prefix this App owned.
func (id Identity) PurgePrefixes(publishedPrefix string) []string {
	seen := map[string]struct{}{}
	var out []string
	add := func(p string) {
		p = strings.TrimSpace(p)
		if p == "" {
			return
		}
		if !strings.HasSuffix(p, "/") {
			p += "/"
		}
		if _, ok := seen[p]; ok {
			return
		}
		seen[p] = struct{}{}
		out = append(out, p)
	}
	add(id.StaticAppPrefix() + "/")
	if publishedPrefix != "" {
		add(AppLevelPrefix(publishedPrefix))
	}
	if id.Tombstoned {
		add(id.LegacyStaticAppPrefix() + "/")
	}
	return out
}

func secretName(raw string, parts ...string) string {
	raw = strings.ToLower(raw)
	if len(raw) <= maxSecretName && isDNSSubdomain(raw) {
		return raw
	}
	sum := sha256.Sum256([]byte(strings.Join(parts, "\x00")))
	suffix := fmt.Sprintf("%x", sum[:hashLength/2])
	maxBase := max(maxSecretName-1-len(suffix), 1)
	if len(raw) > maxBase {
		raw = raw[:maxBase]
	}
	raw = strings.TrimRight(raw, "-.")
	return raw + "-" + suffix
}

func isDNSSubdomain(s string) bool {
	if s == "" || len(s) > maxSecretName {
		return false
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c >= 'a' && c <= 'z', c >= '0' && c <= '9':
		case c == '-' && i > 0 && i < len(s)-1:
		default:
			return false
		}
	}
	return true
}
