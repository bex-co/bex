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

package deploys

import (
	"context"
	"errors"
	"testing"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/bex-co/bex/lego/backend/internal/core"
	ids "github.com/bex-co/bex/lego/backend/internal/id"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

type deployHookAuditSink struct{ events []core.AuditEvent }

func (s *deployHookAuditSink) Record(_ context.Context, event core.AuditEvent) error {
	s.events = append(s.events, event)
	return nil
}

func TestRegenerateDeployHookAuditsEachSuccessfulRotation(t *testing.T) {
	serviceID := ids.New(ids.Service)
	svc, _ := newService(newFakeStore(), sampleApp("web", serviceID))
	first, err := svc.GetDeployHook(context.Background(), "web")
	if err != nil {
		t.Fatal(err)
	}
	sink := &deployHookAuditSink{}
	svc.Audit = sink
	previous := first.URL
	for i := range 2 {
		rotated, err := svc.RegenerateDeployHook(context.Background(), "web")
		if err != nil {
			t.Fatal(err)
		}
		if rotated.URL == previous {
			t.Fatal("rotation preserved the old deploy-hook credential")
		}
		previous = rotated.URL
		if len(sink.events) != i+1 {
			t.Fatalf("rotation %d recorded %d events, want %d", i+1, len(sink.events), i+1)
		}
		if event := sink.events[i]; event.Verb != core.AuditVerbRegenerateDeployHook || event.Outcome != core.AuditAllowed || event.Target != core.ServiceTarget(serviceID) {
			t.Fatalf("rotation event = %+v", event)
		}
	}
}

type deniedDeployHookChecker struct{}

func (deniedDeployHookChecker) Check(context.Context, string, string, string) (bool, error) {
	return false, nil
}

// Write authorization already bypasses the cache. Revocation between that
// initial decision and the verb's explicit fresh recheck must still be quiet.
type revokingDeployHookChecker struct{ checks int }

func (c *revokingDeployHookChecker) Check(context.Context, string, string, string) (bool, error) {
	c.checks++
	return c.checks == 1, nil
}

func (c *revokingDeployHookChecker) CheckFresh(ctx context.Context, subject, relation, object string) (bool, error) {
	return c.Check(ctx, subject, relation, object)
}

type failedDeployHookPatchClient struct {
	client.Client
	err error
}

func (c *failedDeployHookPatchClient) Patch(context.Context, client.Object, client.Patch, ...client.PatchOption) error {
	return c.err
}

func TestRegenerateDeployHookFailuresDoNotRecordRotation(t *testing.T) {
	for _, failure := range []string{"initial denial", "fresh revocation", "patch"} {
		t.Run(failure, func(t *testing.T) {
			svc, cl := newService(newFakeStore(), sampleApp("web", ids.New(ids.Service)))
			first, err := svc.GetDeployHook(context.Background(), "web")
			if err != nil {
				t.Fatal(err)
			}
			sink := &deployHookAuditSink{}
			svc.Audit = sink
			failureErr := core.ErrForbidden
			switch failure {
			case "initial denial":
				svc.Authz = deniedDeployHookChecker{}
			case "fresh revocation":
				svc.Authz = &revokingDeployHookChecker{}
			case "patch":
				failureErr = errors.New("injected deploy-hook patch failure")
				svc.Client = &failedDeployHookPatchClient{Client: cl, err: failureErr}
			}
			ctx := core.WithIdentity(context.Background(), core.Identity{Subject: "user-x", Method: "session"})
			if _, err := svc.RegenerateDeployHook(ctx, "web"); !errors.Is(err, failureErr) {
				t.Fatalf("rotation error = %v, want %v", err, failureErr)
			}
			var app appv1alpha1.App
			if err := cl.Get(context.Background(), client.ObjectKey{Namespace: "default", Name: "web"}, &app); err != nil {
				t.Fatal(err)
			}
			if app.Annotations[DeployHookTokenAnnotation] != deployHookTokenFromURL(t, first.URL) {
				t.Fatal("failed rotation changed the deploy-hook credential")
			}
			if failure == "initial denial" {
				if len(sink.events) != 1 || sink.events[0].Verb != core.AuditVerbRegenerateDeployHook || sink.events[0].Outcome != core.AuditDenied {
					t.Fatalf("denial audit = %+v, want one denied rotation", sink.events)
				}
			} else if len(sink.events) != 0 {
				t.Fatalf("failed rotation recorded events: %+v", sink.events)
			}
		})
	}
}
