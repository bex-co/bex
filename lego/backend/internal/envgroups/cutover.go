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

package envgroups

// cutover.go is the one-shot move of environment-group metadata from OpenBao
// into env_groups (w8/m114), run as `api env-group-cutover dry-run|apply|prune`
// from a production bex-api pod. It is the only code that still reads the KV
// layout — legacy-tenant copies, locators, tombstones, name claims, revision and
// op records, links stored as names — and it is deleted, with its tests and the
// cutover gate, once production has been pruned (m114 t008).

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"slices"
	"strconv"
	"strings"
	"time"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/secrets"
	"github.com/bex-co/bex/lego/backend/internal/store"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// CutoverStore is what the cutover needs from the control-plane store.
type CutoverStore interface {
	GroupStore
	ListTenants(ctx context.Context) ([]store.Tenant, error)
	GetEnvironment(ctx context.Context, id string) (store.Environment, error)
	CompleteEnvGroupCutover(ctx context.Context, at time.Time) error
}

// Cutover moves one platform's environment groups.
type Cutover struct {
	Secrets core.SecretKV
	Store   CutoverStore
	// Client lists a workspace's Apps, to resolve links stored as names.
	Client client.Client
	Now    func() time.Time
}

// CutoverReport is what a run found and did. It names groups by id only.
type CutoverReport struct {
	Mode            string   `json:"mode"`
	Prefixes        int      `json:"openBaoPrefixes"`
	ListedIDs       int      `json:"listedIds"`
	LiveGroups      int      `json:"liveGroups"`
	AlreadyMigrated int      `json:"alreadyMigrated"`
	Migrated        int      `json:"migrated"`
	ValuesCopied    int      `json:"valuesCopiedFromLegacy"`
	Ownerless       []string `json:"ownerlessAttributedToDefault,omitempty"`
	LinksResolved   int      `json:"linksResolvedFromNames"`
	LinksDropped    []string `json:"linksDropped,omitempty"`
	EnvironmentGone []string `json:"environmentCleared,omitempty"`
	TimestampsFixed []string `json:"timestampsDefaulted,omitempty"`
	NotGroups       int      `json:"staleOrFencedIds"`
	// LocatorsToUnknownWorkspaces are legacy locators naming a workspace with
	// no tenants row: a deleted workspace's leftovers, out of every prefix this
	// cutover reads. Reported, not moved.
	LocatorsToUnknownWorkspaces []string `json:"locatorsToUnknownWorkspaces,omitempty"`
	Refusals                    []string `json:"refusals,omitempty"`
	PrunedPaths                 int      `json:"prunedPaths"`
}

// cutoverGroup is one live group as the KV layout records it.
type cutoverGroup struct {
	row        store.EnvGroup
	env, files map[string]string
	// namedLinks: the metadata predates links being service ids (no
	// linksByID), so its links may still be names.
	namedLinks bool
	// fromLegacy: its maps are readable only at the legacy tenant, so apply
	// copies them to the group's own workspace tenant.
	fromLegacy  bool
	alreadyInDB bool
}

// Run performs mode ("dry-run", "apply" or "prune"). dry-run writes nothing.
// apply refuses, writing nothing, while any refusal stands; it inserts rows and
// copies legacy-resident maps to the workspace tenant, deletes nothing, writes
// the marker, and is a no-op when repeated. prune requires every live group to
// have its row, then deletes every OpenBao env-group path except live groups'
// maps and an active save's recovery maps.
func (c *Cutover) Run(ctx context.Context, mode string) (CutoverReport, error) {
	report := CutoverReport{Mode: mode}
	if mode != "dry-run" && mode != "apply" && mode != "prune" {
		return report, fmt.Errorf("unknown cutover mode %q", mode)
	}
	prefixes, groups, err := c.collect(ctx, &report)
	if err != nil {
		return report, err
	}
	switch mode {
	case "dry-run":
		return report, nil
	case "apply":
		if len(report.Refusals) > 0 {
			return report, errors.New("cutover refused; nothing was written")
		}
		return report, c.apply(ctx, groups, &report)
	}
	for _, g := range groups {
		if !g.alreadyInDB {
			report.Refusals = append(report.Refusals, fmt.Sprintf("group %s has no row; run apply first", g.row.ID))
		}
	}
	if len(report.Refusals) > 0 {
		return report, errors.New("prune refused; nothing was deleted")
	}
	return report, c.prune(ctx, prefixes, &report)
}

// collect reads the whole KV layout: every OpenBao tenant prefix that may hold
// env groups (each workspace's, and the legacy shared one) and every id listed
// under any of them.
func (c *Cutover) collect(ctx context.Context, report *CutoverReport) ([]string, []cutoverGroup, error) {
	tenants, err := c.Store.ListTenants(ctx)
	if err != nil {
		return nil, nil, err
	}
	known := map[string]bool{}
	prefixes := []string{secrets.LegacyTenant}
	for _, t := range tenants {
		known[t.ID] = true
		if t.ID != secrets.LegacyTenant {
			prefixes = append(prefixes, t.ID)
		}
	}
	report.Prefixes = len(prefixes)
	listedAt := map[string][]string{} // id -> prefixes listing it
	for _, p := range prefixes {
		ids, err := c.Secrets.List(secrets.WithTenant(ctx, p), "env-groups")
		if err != nil {
			return nil, nil, fmt.Errorf("list %s env groups: %w", p, err)
		}
		for _, gid := range ids {
			listedAt[gid] = append(listedAt[gid], p)
		}
	}
	report.ListedIDs = len(listedAt)

	appsByTenant := map[string][]appv1alpha1.App{}
	var groups []cutoverGroup
	byName := map[string]string{} // workspace\x00name -> id
	for _, gid := range slices.Sorted(maps.Keys(listedAt)) {
		g, live, err := c.readGroup(ctx, gid, listedAt[gid], report)
		if err != nil {
			return nil, nil, err
		}
		if !live {
			report.NotGroups++
			if legacy, err := c.Secrets.Get(secrets.WithTenant(ctx, secrets.LegacyTenant), cutoverMetaPath(gid)); err != nil {
				return nil, nil, err
			} else if ws := legacy["workspace"]; legacy["locator"] == "1" && ws != "" && !known[ws] {
				report.LocatorsToUnknownWorkspaces = append(report.LocatorsToUnknownWorkspaces, gid)
			}
			continue
		}
		report.LiveGroups++
		ws := g.row.TenantID
		if !known[ws] {
			report.Refusals = append(report.Refusals, fmt.Sprintf("group %s belongs to workspace %q, which has no tenants row", gid, ws))
		}
		key := ws + "\x00" + g.row.Name
		if other, dup := byName[key]; dup {
			report.Refusals = append(report.Refusals, fmt.Sprintf("groups %s and %s share name %q in workspace %s", other, gid, g.row.Name, ws))
		}
		byName[key] = gid
		if err := c.resolveLinks(ctx, &g, appsByTenant, report); err != nil {
			return nil, nil, err
		}
		if g.row.EnvironmentID != "" {
			env, err := c.Store.GetEnvironment(ctx, g.row.EnvironmentID)
			if errors.Is(err, store.ErrNotFound) || (err == nil && env.TenantID != ws) {
				report.EnvironmentGone = append(report.EnvironmentGone, gid)
				g.row.EnvironmentID = ""
			} else if err != nil {
				return nil, nil, err
			}
		}
		if existing, err := c.Store.GetEnvGroup(ctx, gid); err == nil {
			g.alreadyInDB = true
			if existing.TenantID != ws {
				report.Refusals = append(report.Refusals, fmt.Sprintf("group %s already has a row in workspace %s, not %s", gid, existing.TenantID, ws))
			}
			report.AlreadyMigrated++
		} else if !errors.Is(err, store.ErrNotFound) {
			return nil, nil, err
		}
		groups = append(groups, g)
	}
	return prefixes, groups, nil
}

// readGroup resolves one listed id the way the KV readers did: the workspace
// copy of its metadata is authoritative, the legacy full copy stands in for an
// unmigrated group, and a locator, tombstone or empty fence is no group at all.
func (c *Cutover) readGroup(ctx context.Context, gid string, listedAt []string, report *CutoverReport) (cutoverGroup, bool, error) {
	var raw map[string]string
	home := ""
	for _, p := range listedAt {
		if p == secrets.LegacyTenant {
			continue
		}
		m, err := c.Secrets.Get(secrets.WithTenant(ctx, p), cutoverMetaPath(gid))
		if err != nil {
			return cutoverGroup{}, false, err
		}
		if cutoverEditable(m) {
			raw, home = m, p
			break
		}
	}
	if raw == nil {
		m, err := c.Secrets.Get(secrets.WithTenant(ctx, secrets.LegacyTenant), cutoverMetaPath(gid))
		if err != nil {
			return cutoverGroup{}, false, err
		}
		if !cutoverEditable(m) {
			return cutoverGroup{}, false, nil
		}
		raw = m
		home = raw["workspace"]
		if home == "" {
			home = core.DefaultTenant
			report.Ownerless = append(report.Ownerless, gid)
		}
	}
	g := cutoverGroup{row: store.EnvGroup{
		ID: gid, TenantID: home, Name: raw["name"], EnvironmentID: raw["environment"],
		ServiceLinks: cutoverLinks(raw["links"]),
	}}
	now := c.Now().UTC()
	var fixed bool
	g.row.CreatedAt, fixed = cutoverTime(raw["createdAt"], now)
	g.row.UpdatedAt, _ = cutoverTime(raw["updatedAt"], g.row.CreatedAt)
	if fixed {
		report.TimestampsFixed = append(report.TimestampsFixed, gid)
	}
	g.namedLinks = raw["linksByID"] != "1" && len(g.row.ServiceLinks) > 0

	revision, err := c.dualRead(ctx, home, gid, "revision")
	if err != nil {
		return cutoverGroup{}, false, err
	}
	if state := revision["state"]; state != "" && state != store.EnvGroupIdle {
		report.Refusals = append(report.Refusals, fmt.Sprintf("group %s is %s; it must be idle to move", gid, state))
	}
	generation, _ := strconv.ParseUint(revision["generation"], 10, 64)
	if generation == 0 {
		generation = 1
	}
	g.row.Content.Generation = generation

	if g.env, err = c.dualRead(ctx, home, gid, "env"); err != nil {
		return cutoverGroup{}, false, err
	}
	if g.files, err = c.dualRead(ctx, home, gid, "files"); err != nil {
		return cutoverGroup{}, false, err
	}
	ownEnv, err := c.Secrets.Get(secrets.WithTenant(ctx, home), envPath(gid))
	if err != nil {
		return cutoverGroup{}, false, err
	}
	ownFiles, err := c.Secrets.Get(secrets.WithTenant(ctx, home), filesPath(gid))
	if err != nil {
		return cutoverGroup{}, false, err
	}
	g.fromLegacy = (len(ownEnv) == 0 && len(g.env) > 0) || (len(ownFiles) == 0 && len(g.files) > 0)
	g.row.EnvKeys = slices.Sorted(maps.Keys(g.env))
	g.row.FileNames = slices.Sorted(maps.Keys(g.files))
	return g, true, nil
}

// resolveLinks rewrites a link set that may hold names, as the KV readers'
// lazy link migration did: the linked services are the workspace's Apps that
// mount the group; a name one of them answers to becomes its id, a mounting
// service the names no longer reach is added, and a name no mounting service
// answers to — a deleted service's — is dropped and reported.
func (c *Cutover) resolveLinks(ctx context.Context, g *cutoverGroup, cache map[string][]appv1alpha1.App, report *CutoverReport) error {
	if !g.namedLinks {
		return nil
	}
	links := g.row.ServiceLinks
	apps, ok := cache[g.row.TenantID]
	if !ok {
		var list appv1alpha1.AppList
		if err := c.Client.List(ctx, &list, client.MatchingLabels{core.LabelTenant: g.row.TenantID}); err != nil {
			return err
		}
		apps = list.Items
		cache[g.row.TenantID] = apps
	}
	var mounting []*appv1alpha1.App
	for i := range apps {
		if mountsGroup(&apps[i], g.row.ID) {
			mounting = append(mounting, &apps[i])
		}
	}
	var out []string
	for _, entry := range links {
		if isServiceID(entry) {
			out = addString(out, entry)
			continue
		}
		resolved := ""
		for _, a := range mounting {
			if slices.Contains([]string{core.AppPublicID(a), a.Labels[core.LabelServiceName], a.Name, a.Spec.DisplayName}, entry) {
				resolved = core.AppPublicID(a)
				break
			}
		}
		if resolved == "" {
			report.LinksDropped = append(report.LinksDropped, g.row.ID)
			continue
		}
		report.LinksResolved++
		out = addString(out, resolved)
	}
	for _, a := range mounting {
		out = addString(out, core.AppPublicID(a))
	}
	g.row.ServiceLinks = out
	return nil
}

// dualRead reads one of a group's KV paths the way the KV readers did: the
// group's workspace tenant first, then the legacy tenant.
func (c *Cutover) dualRead(ctx context.Context, home, gid, leaf string) (map[string]string, error) {
	path := "env-groups/" + gid + "/" + leaf
	own, err := c.Secrets.Get(secrets.WithTenant(ctx, home), path)
	if err != nil || len(own) > 0 || home == secrets.LegacyTenant {
		return own, err
	}
	return c.Secrets.Get(secrets.WithTenant(ctx, secrets.LegacyTenant), path)
}

func (c *Cutover) apply(ctx context.Context, groups []cutoverGroup, report *CutoverReport) error {
	for _, g := range groups {
		if g.alreadyInDB {
			continue
		}
		if g.fromLegacy {
			for path, data := range map[string]map[string]string{envPath(g.row.ID): g.env, filesPath(g.row.ID): g.files} {
				if len(data) == 0 {
					continue
				}
				if err := c.Secrets.Put(secrets.WithTenant(ctx, g.row.TenantID), path, data); err != nil {
					return fmt.Errorf("copy %s maps: %w", g.row.ID, err)
				}
			}
			report.ValuesCopied++
		}
		if _, err := c.Store.InsertEnvGroup(ctx, g.row); err != nil {
			return fmt.Errorf("insert %s: %w", g.row.ID, err)
		}
		report.Migrated++
	}
	return c.Store.CompleteEnvGroupCutover(ctx, c.Now().UTC())
}

// prune deletes, under every prefix, every env-group path that is not a live
// group's own env or files map or an in-flight save's recovery maps, and every
// name claim.
func (c *Cutover) prune(ctx context.Context, prefixes []string, report *CutoverReport) error {
	for _, p := range prefixes {
		pctx := secrets.WithTenant(ctx, p)
		claims, err := c.Secrets.List(pctx, "env-group-name-claims")
		if err != nil {
			return err
		}
		for _, claim := range claims {
			if err := c.delete(pctx, "env-group-name-claims/"+claim, report); err != nil {
				return err
			}
		}
		ids, err := c.Secrets.List(pctx, "env-groups")
		if err != nil {
			return err
		}
		for _, gid := range ids {
			row, err := c.Store.GetEnvGroup(ctx, gid)
			if err != nil && !errors.Is(err, store.ErrNotFound) {
				return err
			}
			if err != nil || row.TenantID != p {
				// No live group lives here: a legacy copy, a stale id or a fence
				// — or a create still in flight, which has written its maps but
				// not yet its row. Only the KV layout ever wrote meta, revision
				// or op records, so an id with none of them is left alone.
				leaves, err := c.Secrets.List(pctx, "env-groups/"+gid)
				if err != nil {
					return err
				}
				if !slices.ContainsFunc(leaves, func(leaf string) bool { return leaf == "meta" || leaf == "revision" || leaf == "op" }) {
					continue
				}
				if err := c.deleteTree(pctx, "env-groups/"+gid, report); err != nil {
					return err
				}
				continue
			}
			leaves, err := c.Secrets.List(pctx, "env-groups/"+gid)
			if err != nil {
				return err
			}
			for _, leaf := range []string{"meta", "revision"} {
				if slices.Contains(leaves, leaf) {
					if err := c.delete(pctx, "env-groups/"+gid+"/"+leaf, report); err != nil {
						return err
					}
				}
			}
			if !slices.Contains(leaves, "op") {
				continue
			}
			// "op" was both the KV layout's operation record and the folder of
			// every operation's recovery maps.
			if record, err := c.Secrets.Get(pctx, opDir(gid)); err != nil {
				return err
			} else if len(record) > 0 {
				if err := c.delete(pctx, opDir(gid), report); err != nil {
					return err
				}
			}
			ops, err := c.Secrets.List(pctx, opDir(gid))
			if err != nil {
				return err
			}
			for _, opID := range ops {
				if row.Content.Op != nil && row.Content.Op.ID == opID {
					continue // an active save's recovery evidence
				}
				if err := c.deleteTree(pctx, opDir(gid)+"/"+opID, report); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func (c *Cutover) delete(ctx context.Context, path string, report *CutoverReport) error {
	if err := c.Secrets.Delete(ctx, path); err != nil {
		return fmt.Errorf("delete %s: %w", path, err)
	}
	report.PrunedPaths++
	return nil
}

// deleteTree deletes path and everything listed beneath it. A KV v2 list does
// not tell a leaf from a folder, so every listed entry is both descended into
// and deleted; a delete where there is nothing is a no-op. Only leaves count.
func (c *Cutover) deleteTree(ctx context.Context, path string, report *CutoverReport) error {
	children, err := c.Secrets.List(ctx, path)
	if err != nil {
		return err
	}
	for _, child := range children {
		if err := c.deleteTree(ctx, path+"/"+child, report); err != nil {
			return err
		}
	}
	if err := c.Secrets.Delete(ctx, path); err != nil {
		return fmt.Errorf("delete %s: %w", path, err)
	}
	if len(children) == 0 {
		report.PrunedPaths++
	}
	return nil
}

func cutoverMetaPath(gid string) string { return "env-groups/" + gid + "/meta" }

// cutoverEditable reports whether raw is a group's full metadata rather than a
// locator, a tombstone or an empty post-delete fence.
func cutoverEditable(raw map[string]string) bool {
	return len(raw) > 0 && raw["locator"] != "1" && raw["tombstoned"] != "1" && raw["name"] != ""
}

func cutoverLinks(raw string) []string {
	var out []string
	for _, entry := range strings.Split(strings.TrimSpace(raw), ",") {
		if entry = strings.TrimSpace(entry); entry != "" {
			out = addString(out, entry)
		}
	}
	return out
}

// cutoverTime parses a stored timestamp, reporting whether it had to default.
func cutoverTime(raw string, fallback time.Time) (time.Time, bool) {
	for _, layout := range []string{time.RFC3339Nano, time.RFC3339} {
		if t, err := time.Parse(layout, raw); err == nil {
			return t.UTC(), false
		}
	}
	return fallback, true
}
