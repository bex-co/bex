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

package secrets

import (
	"context"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

const m66AppID = "srv-c185th5c2rvvnhbfiltg"

// idAddressableApp is a store-managed App exactly as the control plane projects
// one: metadata.name is the tenant-prefixed CRName, the public service name and
// the stable srv- id live on labels. Every official adapter accepts either the
// id or the public name as the {serviceId} path token.
func idAddressableApp() *appv1alpha1.App {
	a := sampleApp("tea-a-web")
	a.Labels = map[string]string{
		core.LabelTenant:      "tea-a",
		core.LabelServiceName: "web",
		core.LabelAppID:       m66AppID,
	}
	return a
}

// TestSetSecretFileCanonicalizesStorageKey is the w1/m66 F10 regression. Before
// the fix SetSecretFile was the only verb in this package that skipped
// storeServiceName, so a write addressed by the stable srv- id landed under
// services/<srv-id>/files — a path no read, delete, or purge path consults. The
// file materialized into the pod once and then vanished from the API.
// The addressing forms a caller can use against this fixture without a resolved
// workspace: the stable srv- id (LabelAppID) and the CR's metadata.name. The
// public-name fallback additionally needs an acting workspace (Base.Workspace),
// which the store-off unit fixture does not have; the existing suite covers it
// where metadata.name IS the public name.
var m66Addressings = []string{m66AppID, "tea-a-web"}

func TestSetSecretFileCanonicalizesStorageKey(t *testing.T) {
	for _, addressedAs := range m66Addressings {
		t.Run(addressedAs, func(t *testing.T) {
			store := newFakeSecretStore()
			svc := newService(store, idAddressableApp())
			ctx := context.Background()

			if _, err := svc.SetSecretFile(ctx, addressedAs, "ca.pem", "----CERT----"); err != nil {
				t.Fatalf("SetSecretFile(%s): %v", addressedAs, err)
			}

			// One canonical key, whatever the caller addressed.
			if got := store.m[filesPath("web")]["ca.pem"]; got != "----CERT----" {
				t.Fatalf("canonical path not written: %+v", store.m)
			}
			if _, ok := store.m[filesPath(m66AppID)]; ok {
				t.Fatalf("id-keyed path must not exist: %+v", store.m)
			}
			if _, ok := store.m[filesPath("tea-a-web")]; ok {
				t.Fatalf("CR-name-keyed path must not exist: %+v", store.m)
			}

			// And it is readable through every addressing form, which is the part
			// that was actually broken for users: a write by id then disappeared.
			for _, readAs := range m66Addressings {
				one, err := svc.GetSecretFile(ctx, readAs, "ca.pem")
				if err != nil || one.Content != "----CERT----" {
					t.Fatalf("GetSecretFile(%s) after write(%s) = %+v err=%v", readAs, addressedAs, one, err)
				}
				list, err := svc.ListSecretFiles(ctx, readAs)
				if err != nil || len(list) != 1 || list[0].Name != "ca.pem" {
					t.Fatalf("ListSecretFiles(%s) = %+v err=%v", readAs, list, err)
				}
			}
		})
	}
}

// TestPurgeAppRemovesLegacyIDKeyedPaths proves the cleanup half: data written by
// the pre-fix code under services/<srv-id>/… is destroyed when the service is
// deleted. Without it, a srv- id (DNS-safe and well under the 30-char service
// name limit) could be re-used as a NEW service's name in the same workspace and
// would read the dead service's secrets.
func TestPurgeAppRemovesLegacyIDKeyedPaths(t *testing.T) {
	store := newTenantFakeSecretStore()
	a := idAddressableApp()
	svc := newService(store, a)
	tenantCtx := withTenant(context.Background(), "tea-a")

	// Canonical (post-fix) data plus the legacy id-keyed data a pre-fix write left.
	if err := store.Put(tenantCtx, filesPath("web"), map[string]string{"ca.pem": "live"}); err != nil {
		t.Fatalf("seed canonical files: %v", err)
	}
	if err := store.Put(tenantCtx, filesPath(m66AppID), map[string]string{"ca.pem": "stale"}); err != nil {
		t.Fatalf("seed legacy files: %v", err)
	}
	if err := store.Put(tenantCtx, envPath(m66AppID), map[string]string{"TOKEN": "stale"}); err != nil {
		t.Fatalf("seed legacy env: %v", err)
	}

	if err := (&WorkspacePurger{Service: svc}).PurgeApp(context.Background(), a); err != nil {
		t.Fatalf("PurgeApp: %v", err)
	}

	for _, path := range []string{
		"tea-a/" + filesPath("web"),
		"tea-a/" + filesPath(m66AppID),
		"tea-a/" + envPath(m66AppID),
	} {
		if _, ok := store.m[path]; ok {
			t.Errorf("path %q survived the purge: %+v", path, store.m)
		}
	}
}

// TestEveryStoreKeyIsCanonicalized sweeps the package so the F10 divergence
// cannot come back in a new verb: any function that authorizes an App and then
// builds a store key from a caller-supplied service token must canonicalize it
// through storeServiceName first. This is the class check — the fix above is one
// instance of it.
func TestEveryStoreKeyIsCanonicalized(t *testing.T) {
	fset := token.NewFileSet()
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("read package dir: %v", err)
	}
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		file, err := parser.ParseFile(fset, filepath.Join(".", name), nil, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", name, err)
		}
		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Body == nil || !takesServiceParam(fn) {
				continue
			}
			var authorizes, canonicalizes, buildsKey bool
			ast.Inspect(fn.Body, func(n ast.Node) bool {
				call, ok := n.(*ast.CallExpr)
				if !ok {
					return true
				}
				switch f := call.Fun.(type) {
				case *ast.SelectorExpr:
					if strings.HasPrefix(f.Sel.Name, "AuthorizeApp") {
						authorizes = true
					}
				case *ast.Ident:
					switch f.Name {
					case "storeServiceName":
						canonicalizes = true
					case "envPath", "filesPath":
						// Only a key built from the service token matters; a key built
						// from an already-canonical local is fine either way.
						if len(call.Args) == 1 {
							if id, ok := call.Args[0].(*ast.Ident); ok && id.Name == "service" {
								buildsKey = true
							}
						}
					}
				}
				return true
			})
			if authorizes && buildsKey && !canonicalizes {
				t.Errorf("%s: %s authorizes an App and builds a store key from the raw request token — assign service = storeServiceName(a, service) first (w1/m66 F10)",
					name, fn.Name.Name)
			}
		}
	}
}

func takesServiceParam(fn *ast.FuncDecl) bool {
	if fn.Type.Params == nil {
		return false
	}
	for _, p := range fn.Type.Params.List {
		for _, n := range p.Names {
			if n.Name == "service" {
				return true
			}
		}
	}
	return false
}
