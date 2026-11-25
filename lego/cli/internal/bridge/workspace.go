package bridge

import (
	"context"
	"fmt"
	"os"
	"regexp"
	"strings"
	"sync/atomic"
	"time"

	"github.com/render-oss/cli/pkg/client"
	"github.com/render-oss/cli/pkg/owner"
)

// workspaceID is the shape of a bex workspace id: Render's team prefix and a
// 20-character xid, as the server checks it before it reads a value as an id
// rather than a name (lego/backend/internal/id). The prefix alone does not
// make an id: a workspace named "tea-house" is resolved like any other name
// (w32/012).
var workspaceID = regexp.MustCompile(`^tea-[0-9a-v]{20}$`)

// workspaceResolveTimeout bounds the one extra owners request a workspace
// name costs, so a slow API cannot hold every command at startup.
const workspaceResolveTimeout = 10 * time.Second

// ownerLister lists the caller's workspaces filtered by name. Any error means
// the name could not be checked (no login, API unreachable).
type ownerLister func(ctx context.Context, name string) ([]*client.Owner, error)

// ResolveWorkspaceName turns a BEX_WORKSPACE name into the workspace id the
// upstream CLI needs (w8/038). Upstream reads RENDER_WORKSPACE as an id for
// every command (ownerId=, owners/{id}, Blueprint validate), so a name used
// to fail everywhere with a misleading "not allowed".
//
// It acts only when the bridge mapped BEX_WORKSPACE (an explicit
// RENDER_WORKSPACE stays the untouched escape hatch) and the value is not
// already an id, so an id costs no request. Without a usable login or API
// the name is passed through unchanged and upstream reports that failure as
// it always has. Call it after Apply and InstallControlPlaneHeaders, once the
// invoked command is known (w8/064): use says whether that command consumes
// the active workspace, so a stale name never blocks help, inventory, identity,
// explicit selection, completion or logout — the very commands that recover
// from it.
func ResolveWorkspaceName(use WorkspaceUse) error {
	if filledFromBex[renderWorkspace] == "" || use == WorkspaceUnused {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), workspaceResolveTimeout)
	defer cancel()
	err := resolveWorkspaceName(ctx, os.LookupEnv, os.Setenv, listOwnersByName)
	if use == WorkspaceIfResolvable {
		if err != nil {
			// The raw name stays in RENDER_WORKSPACE so the direct read
			// proceeds, but it must never reach the API as an ownerId —
			// including through an id-or-name fallback search a 404
			// triggers. Bar it at the control-plane transport (w32/117),
			// keeping the startup error so ambiguous names stay distinct
			// from unknown ones.
			if name, _ := os.LookupEnv(renderWorkspace); name != "" {
				blockedOwner.Store(blockedOwnerInfo{name: name, reason: err.Error()})
			}
		}
		return nil
	}
	return err
}

// blockedOwnerInfo pairs a barred RENDER_WORKSPACE name with the startup
// resolution error (unknown or ambiguous workspace) to report if a fallback
// search attempts to send it.
type blockedOwnerInfo struct {
	name   string
	reason string
}

// blockedOwner is the RENDER_WORKSPACE value left in place after an
// unresolvable lookup for a direct-ID command (see ResolveWorkspaceName).
// One command runs per process, so it is stored at most once outside tests.
var blockedOwner atomic.Value

// loadBlockedOwner returns the barred workspace name and refusal reason, or
// the zero value when none was barred.
func loadBlockedOwner() blockedOwnerInfo {
	info, _ := blockedOwner.Load().(blockedOwnerInfo)
	return info
}

// WorkspaceUse classifies how the invoked command uses the active workspace.
type WorkspaceUse int

const (
	// WorkspaceRequired: the command reads the active workspace, so an unknown
	// or ambiguous name is refused before it can act on the wrong tenant.
	WorkspaceRequired WorkspaceUse = iota
	// WorkspaceIfResolvable: the command addresses a typed resource id, which
	// upstream reads directly; a resolvable name is still mapped (an id
	// selector's not-found fallback searches the mapped workspace), but an
	// unresolvable one neither blocks the read nor reaches the API —
	// ResolveWorkspaceName bars it at the control-plane transport.
	WorkspaceIfResolvable
	// WorkspaceUnused: the command never reads the active workspace — help,
	// inventory, identity, an explicit selection, completion, auth.
	WorkspaceUnused
)

func resolveWorkspaceName(ctx context.Context, lookup lookupEnv, set setEnv, list ownerLister) error {
	name, _ := lookup(renderWorkspace)
	if name == "" || workspaceID.MatchString(name) {
		return nil
	}
	owners, err := list(ctx, name)
	if err != nil {
		return nil
	}
	// bex's ?name= filter decides (w3/m246): an exact match on the unique slug
	// wins outright, else every exact display-name match comes back. The
	// owner's `name` is the display name, so it cannot be re-checked here.
	var ids []string
	for _, o := range owners {
		if o != nil {
			ids = append(ids, o.Id)
		}
	}
	switch len(ids) {
	case 0:
		return fmt.Errorf("%s: no workspace named %q; run `bex workspaces` to list yours", bexWorkspace, name)
	case 1:
		if err := set(renderWorkspace, ids[0]); err != nil {
			return fmt.Errorf("set %s: %w", renderWorkspace, err)
		}
		return nil
	default:
		return fmt.Errorf("%s: several workspaces are named %q (%s); use the id", bexWorkspace, name, strings.Join(ids, ", "))
	}
}

// listOwnersByName uses the upstream client exactly as a command would,
// including its token refresh.
func listOwnersByName(ctx context.Context, name string) ([]*client.Owner, error) {
	c, err := client.NewDefaultClient()
	if err != nil {
		return nil, err
	}
	return owner.NewRepo(c).ListOwners(ctx, owner.ListInput{Name: name})
}
