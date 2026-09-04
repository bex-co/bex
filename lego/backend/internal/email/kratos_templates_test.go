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

package email

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// committedKratosTemplatesPath is the generated file's home, relative to this
// package (lego/backend/internal/email → repo root is four levels up). The
// operator's codegen reading ../types is the precedent for a test reaching
// across module boundaries inside the one repo checkout.
const committedKratosTemplatesPath = "../../../../deploy/gitops/base/values/kratos-email-templates.values.yaml"

// TestGenerateKratosCourierTemplates writes the values fragment when
// KRATOS_TEMPLATES_PATH is set (the SCHEMA_DUMP_PATH pattern) and is a no-op
// otherwise, so a plain `go test ./...` never touches the tree.
func TestGenerateKratosCourierTemplates(t *testing.T) {
	path := os.Getenv("KRATOS_TEMPLATES_PATH")
	if path == "" {
		t.Skip("KRATOS_TEMPLATES_PATH not set; generation not requested")
	}
	body, err := KratosCourierTemplatesYAML()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Clean(path), body, 0o644); err != nil {
		t.Fatal(err)
	}
	t.Logf("wrote %d bytes to %s", len(body), path)
}

// TestKratosCourierTemplatesInSync pins the committed GitOps values fragment
// to this package's output: a change to the layout, the brand tokens, or the
// courier copy that is not regenerated into the committed file fails here —
// the drift class the generator exists to close.
func TestKratosCourierTemplatesInSync(t *testing.T) {
	want, err := KratosCourierTemplatesYAML()
	if err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(committedKratosTemplatesPath)
	if err != nil {
		t.Fatalf("committed templates missing (%v) — regenerate with KRATOS_TEMPLATES_PATH, see kratos_templates.go", err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("deploy/gitops/base/values/kratos-email-templates.values.yaml is stale — regenerate with KRATOS_TEMPLATES_PATH, see kratos_templates.go (%d vs %d bytes)", len(got), len(want))
	}
}

// TestKratosCourierTemplatesShape guards the properties the rendered
// templates must hold regardless of copy changes.
func TestKratosCourierTemplatesShape(t *testing.T) {
	body, err := KratosCourierTemplatesYAML()
	if err != nil {
		t.Fatal(err)
	}
	s := string(body)

	// No sentinel may survive into the shipped templates.
	for _, sentinel := range []string{kratosCodeSentinel, kratosURLSentinel} {
		if strings.Contains(s, sentinel) {
			t.Fatalf("sentinel %q leaked into the rendered templates", sentinel)
		}
	}
	// html/template rejecting the URL sentinel would ship a broken button.
	if strings.Contains(s, "ZgotmplZ") {
		t.Fatal("html/template rejected a URL context — the sentinel swap broke")
	}
	// The Kratos actions each variant depends on must be present.
	for _, action := range []string{
		"{{ .VerificationCode }}",
		"{{ .VerificationURL }}",
		"{{ .RecoveryCode }}",
	} {
		if !strings.Contains(s, action) {
			t.Fatalf("rendered templates missing Kratos action %s", action)
		}
	}
	// The codes ship as the Code panel (kratos_templates.go uses Code, not
	// Reference, for them): each action is the sole content of its monospace
	// span, so what a reader copies is exactly the code.
	for _, action := range []string{"{{ .VerificationCode }}", "{{ .RecoveryCode }}"} {
		if !strings.Contains(s, ">"+action+"</span>") {
			t.Fatalf("%s is not rendered as an unbroken Code panel value", action)
		}
	}
	// Both branded halves are present: the HTML card and the override path.
	if !strings.Contains(s, "template_override_path: /conf/courier-templates") {
		t.Fatal("missing courier template_override_path")
	}
	if !strings.Contains(s, BrandPrimary) {
		t.Fatal("HTML bodies do not carry the brand palette")
	}
}
