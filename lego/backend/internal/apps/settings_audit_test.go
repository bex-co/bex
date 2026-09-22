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

package apps

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/core"
	ids "github.com/bex-co/bex/lego/backend/internal/id"
)

func TestSetIdleTTLRefusalDoesNotRecordChange(t *testing.T) {
	app := sampleApp("web")
	app.Spec.IdleTTLSeconds = 60
	svc, cl := newService(nil, app)
	sink := &captureAuditSink{}
	svc.Audit = sink
	for _, seconds := range []int32{-1, MaxIdleTTLSeconds + 1} {
		if _, err := svc.SetIdleTTL(context.Background(), "web", seconds); !errors.Is(err, core.ErrBadRequest) {
			t.Fatalf("SetIdleTTL(%d) = %v, want ErrBadRequest", seconds, err)
		}
	}
	if len(sink.events) != 0 {
		t.Fatalf("refused idle timeout recorded events: %+v", sink.events)
	}
	if got := getApp(t, cl, "web").Spec.IdleTTLSeconds; got != 60 {
		t.Fatalf("refused idle timeout changed the stored value to %d", got)
	}
}

// The feed projects these allowed rows into past-tense changes. A no-op
// (including the display name's normalized spelling) must not manufacture one.
func TestOperationalSettingsAuditOnlySuccessfulChanges(t *testing.T) {
	for _, managed := range []bool{false, true} {
		mode := "bare CR"
		if managed {
			mode = "store managed"
		}
		t.Run(mode, func(t *testing.T) {
			for _, tc := range []struct {
				name string
				verb string
				call func(*Service, bool) (AppView, error)
			}{
				{"idle timeout", core.AuditVerbSetIdleTTL, func(s *Service, change bool) (AppView, error) {
					seconds := int32(60)
					if change {
						seconds = 120
					}
					return s.SetIdleTTL(context.Background(), "web", seconds)
				}},
				{"display name", core.AuditVerbSetDisplayName, func(s *Service, change bool) (AppView, error) {
					name := "  Original label  "
					if change {
						name = "  Renamed label  "
					}
					return s.SetDisplayName(context.Background(), "web", name)
				}},
			} {
				t.Run(tc.name, func(t *testing.T) {
					app := sampleApp("web")
					app.Spec.IdleTTLSeconds = 60
					app.Spec.DisplayName = "Original label"
					target := "web"
					var st IntentStore
					if managed {
						target = ids.New(ids.Service)
						app = manage(app, target)
						st = &recordingStore{}
					}
					svc, cl := newService(st, app)
					sink := &captureAuditSink{}
					svc.Audit = sink
					if _, err := tc.call(svc, false); err != nil {
						t.Fatalf("unchanged save: %v", err)
					}
					if len(sink.events) != 0 {
						t.Fatalf("unchanged save recorded events: %+v", sink.events)
					}
					if _, err := tc.call(svc, true); err != nil {
						t.Fatalf("changed save: %v", err)
					}
					if len(sink.events) != 1 {
						t.Fatalf("changed save recorded %d events, want 1", len(sink.events))
					}
					if event := sink.events[0]; event.Verb != tc.verb || event.Outcome != core.AuditAllowed || event.Target != core.ServiceTarget(target) {
						t.Fatalf("changed save event = %+v", event)
					}
					got := getApp(t, cl, "web")
					if tc.verb == core.AuditVerbSetIdleTTL && got.Spec.IdleTTLSeconds != 120 {
						t.Fatalf("changed save idle TTL = %d, want 120", got.Spec.IdleTTLSeconds)
					}
					if tc.verb == core.AuditVerbSetDisplayName && got.Spec.DisplayName != "Renamed label" {
						t.Fatalf("changed save label = %q, want trimmed label", got.Spec.DisplayName)
					}
					if _, err := tc.call(svc, true); err != nil {
						t.Fatalf("repeated save: %v", err)
					}
					if len(sink.events) != 1 {
						t.Fatalf("repeated save recorded another event: %+v", sink.events)
					}
				})
			}
		})
	}
}

func TestOperationalSettingsFailedAndDeniedWritesDoNotRecordChange(t *testing.T) {
	for _, tc := range []struct {
		name string
		verb string
		call func(*Service, context.Context) error
	}{
		{"idle timeout", core.AuditVerbSetIdleTTL, func(s *Service, ctx context.Context) error {
			_, err := s.SetIdleTTL(ctx, "web", 120)
			return err
		}},
		{"display name", core.AuditVerbSetDisplayName, func(s *Service, ctx context.Context) error {
			_, err := s.SetDisplayName(ctx, "web", "Renamed label")
			return err
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			for _, failure := range []string{"row", "patch", "denied"} {
				t.Run(failure, func(t *testing.T) {
					app := managedApp("web", ids.New(ids.Service))
					app.Spec.IdleTTLSeconds = 60
					app.Spec.DisplayName = "Original label"
					st := &recordingStore{}
					svc, cl := newService(st, app)
					patches := &auditPatchClient{Client: cl}
					svc.Client = patches
					sink := &captureAuditSink{}
					svc.Audit = sink
					failureErr := errors.New("injected write failure")
					switch failure {
					case "row":
						st.err = failureErr
					case "patch":
						patches.err = failureErr
					case "denied":
						svc.Authz = &fakeChecker{allow: false}
						failureErr = core.ErrForbidden
					}
					if err := tc.call(svc, ctxAs("user-x")); !errors.Is(err, failureErr) {
						t.Fatalf("call error = %v, want %v", err, failureErr)
					}
					if got := getApp(t, cl, "web").Spec; !reflect.DeepEqual(got, app.Spec) {
						t.Fatalf("failed operation changed the App spec: %+v", got)
					}
					if failure == "patch" && patches.patches != 1 || failure != "patch" && patches.patches != 0 {
						t.Fatalf("%s failure attempted %d CR patches", failure, patches.patches)
					}
					if failure == "denied" {
						if len(sink.events) != 1 || sink.events[0].Verb != tc.verb || sink.events[0].Outcome != core.AuditDenied {
							t.Fatalf("denial audit = %+v, want one denied %s", sink.events, tc.verb)
						}
					} else if len(sink.events) != 0 {
						t.Fatalf("failed write recorded events: %+v", sink.events)
					}
				})
			}
		})
	}
}
