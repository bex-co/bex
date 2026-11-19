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
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// appEnumerationsThatMayIncludeBuildSources names every function that lists
// App CRs without filtering build source carriers out (w2/m254, ADR097), and
// why that is right. A new App enumeration must either call ServiceApps /
// IsBuildSource or be added here with its reason — the failure this guards is
// a carrier silently showing up as a service (a list, a count, a quota, a
// sweep).
var appEnumerationsThatMayIncludeBuildSources = map[string]string{
	"deploys/deployhook.go:BackfillDeployHookTokenDigests":      "a carrier has no deploy hook",
	"deploys/deployhook.go:appForDeployHookToken":               "a carrier has no deploy hook token",
	"deploys/restart_wait.go:runDeferredDeploy":                 "resolves the one App a deferred deploy row names",
	"registrycreds/pullsecret.go:appBoundToCredential":          "an image-backed build source binds a credential too",
	"diskusage/diskusage.go:SweepOnce":                          "a carrier has no disk",
	"environments/service.go:proxiedDomainsByEnvironment":       "a carrier is never in an environment",
	"usage/service.go:resolveAppCR":                             "resolves the App a usage row names, carrier builds included",
	"rollout/stage.go:List":                                     "env changes reach the Apps that link them; a carrier's are built in by its next build",
	"apps/container_policy_migration.go:appByServiceID":         "resolves one App by its id",
	"apps/purger.go:PurgeWorkspace":                             "a workspace purge deletes its build sources too",
	"apps/blueprint_ownership.go:clearBlueprintMarkers":         "clears markers on whatever a Blueprint owned",
	"apps/blueprint_ownership.go:clearBlueprintOwnership":       "clears markers on whatever a Blueprint owned",
	"notifications/alert_email_worker.go:AppNotificationPolicy": "resolves the App an alert names",
	"apps/domains.go:listHostClaims":                            "a carrier claims no host",
	"apps/domain_verification_worker.go:resolveApp":             "resolves the App a domain names",
	"apps/workspace_snapshot.go:workspaceSnapshot":              "the snapshot's field type; filled by workspaceSnapshot.apps",
	"apps/workspace_snapshot.go:workspaceSnapshot.apps":         "the cached list is filtered where it is filled",
	"apps/workspace_snapshot.go:workspaceSnapshot.allApps":      "the one listing that apps (services) and buildSources (carriers) each filter",
	"apps/checks_pass.go:serveCheckSuite":                       "a carrier opens no checked deploy",
	"apps/checks_worker.go:tick":                                "a carrier opens no checked deploy",
	"apps/nameavailable.go:tenantNames":                         "carrier names are bds- ids, never a service's",
	"billing/enforcement.go:Enforce":                            "an unpaid workspace's build sources stop too",
	"apps/build_minutes.go:List":                                "a carrier builds, so it stops when the workspace is out of build minutes",
	"store/reconciler.go:ReconcileOnce":                         "every App CR, carriers included, is projected",
	"store/app_status_watch.go:watchOnce":                       "a carrier's status changes kick the projector that closes its build's rows",
	"store/product_inventory.go:listResources":                  "product inventory counts build sources as their own type",
}

func TestEveryAppEnumerationDeclaresBuildSources(t *testing.T) {
	root := filepath.Join("..")
	var unclassified []string
	seen := map[string]bool{}
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return err
		}
		file, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(root, path)
		rel = filepath.ToSlash(rel)
		check := func(name string, body ast.Node) {
			if body == nil || !mentions(body, "AppList") {
				return
			}
			key := rel + ":" + name
			seen[key] = true
			if mentions(body, "ServiceApps") || mentions(body, "IsBuildSource") || mentions(body, "notServiceAppFor") || mentions(body, "serviceAppsFor") {
				return
			}
			if _, ok := appEnumerationsThatMayIncludeBuildSources[key]; !ok {
				unclassified = append(unclassified, key)
			}
		}
		for _, decl := range file.Decls {
			switch d := decl.(type) {
			case *ast.FuncDecl:
				name := d.Name.Name
				if d.Recv != nil && len(d.Recv.List) == 1 {
					if star, ok := d.Recv.List[0].Type.(*ast.StarExpr); ok {
						if id, ok := star.X.(*ast.Ident); ok && id.Name == "workspaceSnapshot" {
							name = id.Name + "." + name
						}
					}
				}
				check(name, d)
			case *ast.GenDecl:
				for _, spec := range d.Specs {
					switch sp := spec.(type) {
					case *ast.TypeSpec:
						check(sp.Name.Name, sp)
					case *ast.ValueSpec:
						if len(sp.Names) > 0 {
							check(sp.Names[0].Name, sp)
						}
					}
				}
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	sort.Strings(unclassified)
	for _, key := range unclassified {
		t.Errorf("%s lists App CRs without filtering build sources (ServiceApps) and is not in appEnumerationsThatMayIncludeBuildSources", key)
	}
	for key := range appEnumerationsThatMayIncludeBuildSources {
		if !seen[key] {
			t.Errorf("%s is listed as an App enumeration but no longer is one: drop it", key)
		}
	}
}

// mentions reports whether an identifier or selector named name occurs in n.
func mentions(n ast.Node, name string) bool {
	found := false
	ast.Inspect(n, func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.Ident:
			found = found || x.Name == name
		case *ast.SelectorExpr:
			found = found || x.Sel.Name == name
		}
		return !found
	})
	return found
}

// w2/m254: a build source carrier is never a service — no service verb
// resolves it by id, by CR name or by name, while a service beside it still
// resolves.
func TestServiceLookupsNeverResolveABuildSource(t *testing.T) {
	carrier := workspaceApp("tea-a", "bds-1", "bds-1")
	carrier.Spec.Type = appv1alpha1.TypeBuildSource
	web := workspaceApp("tea-a", "web", "srv-a")
	ctx := WithWorkspace(WithIdentity(context.Background(), Identity{Subject: "bob", Method: "session"}), "tea-a")
	base := &Base{Client: fakeAppClient(carrier, web), Namespace: "default", Workspace: multiWorkspace{"bob": {"tea-a"}}, Authz: &fakeAllowChecker{}}
	for _, target := range []string{"bds-1", carrier.Name} {
		if got, err := base.GetApp(ctx, RelCanView, target); !errors.Is(err, ErrNotFound) {
			t.Errorf("GetApp(%s) = (%v, %v), want not found", target, got, err)
		}
		if got, err := base.AuthorizeApp(ctx, RelCanOperate, target); !errors.Is(err, ErrNotFound) {
			t.Errorf("AuthorizeApp(%s) = (%v, %v), want not found", target, got, err)
		}
	}
	if got, err := base.GetApp(ctx, RelCanView, "srv-a"); err != nil || got.Name != web.Name {
		t.Fatalf("GetApp(srv-a) = (%v, %v), want the service", got, err)
	}
	// The build-source surface resolves its carrier through the same paths.
	scoped := WithBuildSources(ctx)
	if got, err := base.GetApp(scoped, RelCanView, "bds-1"); err != nil || got.Name != carrier.Name {
		t.Fatalf("GetApp(bds-1) with WithBuildSources = (%v, %v), want the carrier", got, err)
	}
	if got, err := base.AuthorizeApp(scoped, RelCanOperate, "bds-1"); err != nil || got.Name != carrier.Name {
		t.Fatalf("AuthorizeApp(bds-1) with WithBuildSources = (%v, %v), want the carrier", got, err)
	}
}
