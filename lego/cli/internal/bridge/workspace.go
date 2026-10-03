package bridge

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/render-oss/cli/pkg/client"
	"github.com/render-oss/cli/pkg/owner"
)

// workspaceIDPrefix is the bex workspace id kind (Render's team prefix).
const workspaceIDPrefix = "tea-"

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
// it always has. Call it after Apply and InstallControlPlaneHeaders.
func ResolveWorkspaceName() error {
	if !workspaceFromBex {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), workspaceResolveTimeout)
	defer cancel()
	return resolveWorkspaceName(ctx, os.LookupEnv, os.Setenv, listOwnersByName)
}

// workspaceFromBex records that Apply copied BEX_WORKSPACE into an unset
// RENDER_WORKSPACE.
var workspaceFromBex bool

func resolveWorkspaceName(ctx context.Context, lookup lookupEnv, set setEnv, list ownerLister) error {
	name, _ := lookup(renderWorkspace)
	if name == "" || strings.HasPrefix(name, workspaceIDPrefix) {
		return nil
	}
	owners, err := list(ctx, name)
	if err != nil {
		return nil
	}
	var ids []string
	for _, o := range owners {
		if o != nil && o.Name == name {
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
// including its token refresh. The name filter narrows the page; the exact
// match above still decides.
func listOwnersByName(ctx context.Context, name string) ([]*client.Owner, error) {
	c, err := client.NewDefaultClient()
	if err != nil {
		return nil, err
	}
	return owner.NewRepo(c).ListOwners(ctx, owner.ListInput{Name: name})
}
