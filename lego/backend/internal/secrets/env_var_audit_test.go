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
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

// auditLogRows is what the workspace audit log shows of rows: every row but a
// feed_only one, as verb and keys.
func auditLogRows(rows []core.AuditEvent) []string {
	var out []string
	for _, ev := range rows {
		if !ev.FeedOnly {
			out = append(out, ev.Verb+" "+strings.Join(ev.EnvVarKeys, ","))
		}
	}
	return out
}

// One env var save is Render's per-kind rows (w9/m449): a patch that adds K1
// and changes K2 is one CreateEnvVarsEvent naming K1 and one
// UpdateEnvVarsEvent naming K2 in the workspace audit log, beside the save's
// own row, which the service's feed keeps. A save that changes nothing shows
// nothing there, a delete names what it removed, and no row carries a value.
func TestEnvVarWritesRecordRenderPerKindRows(t *testing.T) {
	const secret = "s3cr3t-value-never-audited"
	for _, tc := range []struct {
		name  string
		setup func(*Service) error
		write func(*Service) error
		want  []string
	}{{
		name: "a patch that adds one key and changes another",
		setup: func(s *Service) error {
			_, err := s.SetEnvVar(auditCtx(), "web", "K2", EnvVarWrite{Value: "old"})
			return err
		},
		write: func(s *Service) error {
			_, err := s.PatchEnvironment(auditCtx(), "web", EnvironmentPatch{SaveMode: SaveModeOnly, EnvVars: []EnvVarPatch{
				{Key: "K1", Value: secret}, {Key: "K2", Value: secret},
			}})
			return err
		},
		want: []string{"secrets.CreateEnvVars K1", "secrets.UpdateEnvVars K2"},
	}, {
		name: "a replace-all that adds, changes and removes",
		setup: func(s *Service) error {
			_, err := s.SetEnvVars(auditCtx(), "web", []EnvVarView{{Key: "KEEP", Value: "a"}, {Key: "EDIT", Value: "b"}, {Key: "GONE", Value: "c"}})
			return err
		},
		write: func(s *Service) error {
			_, err := s.SetEnvVars(auditCtx(), "web", []EnvVarView{{Key: "KEEP", Value: "a"}, {Key: "EDIT", Value: secret}, {Key: "NEW", Value: secret}})
			return err
		},
		want: []string{"secrets.CreateEnvVars NEW", "secrets.UpdateEnvVars EDIT", "secrets.DeleteEnvVars GONE"},
	}, {
		name: "a save of the value already set",
		setup: func(s *Service) error {
			_, err := s.SetEnvVar(auditCtx(), "web", "K", EnvVarWrite{Value: secret})
			return err
		},
		write: func(s *Service) error {
			_, err := s.SetEnvVar(auditCtx(), "web", "K", EnvVarWrite{Value: secret})
			return err
		},
	}, {
		name: "a patch deleting a key",
		setup: func(s *Service) error {
			_, err := s.SetEnvVar(auditCtx(), "web", "K", EnvVarWrite{Value: secret})
			return err
		},
		write: func(s *Service) error {
			_, err := s.PatchEnvironment(auditCtx(), "web", EnvironmentPatch{SaveMode: SaveModeOnly, EnvVars: []EnvVarPatch{{Key: "K", Delete: true}}})
			return err
		},
		want: []string{"secrets.DeleteEnvVars K"},
	}, {
		// Render has no secret-file event, so a save that changed a file keeps
		// its own row in the audit log beside the env var rows.
		name: "a patch changing a variable and a secret file",
		write: func(s *Service) error {
			_, err := s.PatchEnvironment(auditCtx(), "web", EnvironmentPatch{SaveMode: SaveModeOnly,
				EnvVars:     []EnvVarPatch{{Key: "K", Value: secret}},
				SecretFiles: []SecretFilePatch{{Name: "cert.pem", Content: secret}},
			})
			return err
		},
		want: []string{"secrets.PatchEnvironment ", "secrets.CreateEnvVars K"},
	}} {
		t.Run(tc.name, func(t *testing.T) {
			sink := &recordingAuditSink{}
			svc := auditedService(true, sink)
			if tc.setup != nil {
				if err := tc.setup(svc); err != nil {
					t.Fatalf("setup: %v", err)
				}
				sink.events = nil
			}
			if err := tc.write(svc); err != nil {
				t.Fatalf("write: %v", err)
			}
			if got := auditLogRows(sink.events); !slices.Equal(got, tc.want) {
				t.Errorf("audit log rows = %q, want %q", got, tc.want)
			}
			for _, ev := range sink.events {
				if dump := fmt.Sprintf("%+v", ev); strings.Contains(dump, secret) {
					t.Fatalf("an audit row carries a value: %s", dump)
				}
			}
		})
	}
}

// A variable a Blueprint dropped (a retained literal) was the service's before
// the write that moves it into the store: editing it is UpdateEnvVarsEvent,
// re-saving its value records nothing in the audit log (w9/m449).
func TestEditingARetainedLiteralIsAnUpdate(t *testing.T) {
	for _, tc := range []struct {
		value string
		want  []string
	}{{"changed", []string{"secrets.UpdateEnvVars MESSAGE"}}, {"hello from bex", nil}} {
		sink := &recordingAuditSink{}
		svc := newService(newFakeSecretStore(), retainedApp("bp"))
		svc.Authz = &fakeChecker{allow: true}
		svc.Audit = sink
		if _, err := svc.SetEnvVar(auditCtx(), "bp", "MESSAGE", EnvVarWrite{Value: tc.value}); err != nil {
			t.Fatalf("SetEnvVar(%q): %v", tc.value, err)
		}
		if got := auditLogRows(sink.events); !slices.Equal(got, tc.want) {
			t.Errorf("SetEnvVar(%q) audit log rows = %q, want %q", tc.value, got, tc.want)
		}
	}
}
