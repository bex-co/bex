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

package controller

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestAppPodDisablesServiceLinks is w4/m123. Kubernetes' default
// (enableServiceLinks: true) makes the kubelet inject a legacy Docker-link set
// — <NAME>_SERVICE_HOST, <NAME>_SERVICE_PORT, <NAME>_PORT_<n>_TCP_* — for every
// Service in the pod's namespace. Live, a tenant web service with four
// configured variables received 185, of which 174 were these: every sibling
// resource in the workspace, managed Postgres primaries by id, and
// cert-manager's own ACME solver Service.
//
// The footgun is not the count. A <NAME>_PORT variable SHADOWS the name an
// application expects to configure itself with, so a process reading REDIS_PORT
// or API_PORT is handed `tcp://10.x.x.x:6379` by the platform and fails in a
// way that looks like an application bug.
func TestAppPodDisablesServiceLinks(t *testing.T) {
	dep := project(projectionApp(), webParams())
	links := dep.Spec.Template.Spec.EnableServiceLinks
	if links == nil {
		t.Fatal("enableServiceLinks is unset, so Kubernetes defaults it to TRUE and injects a link var per sibling Service")
	}
	if *links {
		t.Fatal("enableServiceLinks is true — every Service in the workspace is enumerated into the container")
	}
}

// TestEveryPodSpecDisablesServiceLinks is the placement guard t003 owes: a NEW
// pod-producing projection must make the choice deliberately rather than
// inherit the default. It reads the operator's own source, so a site added
// later fails here instead of silently shipping 174 env vars.
//
// Source-level rather than behavioral because the eleven sites are spread over
// build, pre-deploy, cron, backup, export and publish paths with no shared
// constructor to assert against — the absence of one is exactly why the default
// leaked this far.
func TestEveryPodSpecDisablesServiceLinks(t *testing.T) {
	root := ".."
	var checked int
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return err
		}
		src, readErr := os.ReadFile(path)
		if readErr != nil {
			return readErr
		}
		fset := token.NewFileSet()
		file, parseErr := parser.ParseFile(fset, path, src, parser.ParseComments)
		if parseErr != nil {
			return parseErr
		}
		ast.Inspect(file, func(n ast.Node) bool {
			lit, ok := n.(*ast.CompositeLit)
			if !ok {
				return true
			}
			sel, ok := lit.Type.(*ast.SelectorExpr)
			if !ok || sel.Sel.Name != "PodSpec" {
				return true
			}
			checked++
			for _, elt := range lit.Elts {
				kv, ok := elt.(*ast.KeyValueExpr)
				if !ok {
					continue
				}
				if id, ok := kv.Key.(*ast.Ident); ok && id.Name == "EnableServiceLinks" {
					return true
				}
			}
			// The App Deployment assigns the field after building the literal
			// (the projection mutates a fetched object rather than replacing
			// the whole PodSpec), so accept a nearby assignment in the same
			// file as the deliberate choice.
			if strings.Contains(string(src), "EnableServiceLinks = new(false)") {
				return true
			}
			t.Errorf("%s:%d builds a PodSpec without setting EnableServiceLinks — Kubernetes defaults it to true and injects a Docker-link env var for every Service in the namespace (w4/m123)",
				path, fset.Position(lit.Pos()).Line)
			return true
		})
		return nil
	})
	if err != nil {
		t.Fatalf("walk: %v", err)
	}
	// Anti-vacuity: the sweep must actually find the sites it claims to guard.
	if checked < 11 {
		t.Fatalf("only %d PodSpec literals examined — the sweep has gone vacuous; teach it where pods are now built", checked)
	}
}
