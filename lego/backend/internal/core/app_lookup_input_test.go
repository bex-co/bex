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
	"fmt"
	"strings"
	"testing"

	"k8s.io/apimachinery/pkg/api/validation/path"
	"k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// The fake client does not apply the apiserver's selector validation. Parse the
// actual outgoing selector so invalid input reproduces the production failure.
type selectorCheckingClient struct {
	client.Client
	listErr error
}

func (c selectorCheckingClient) Get(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
	// client-go's request builder rejects these before reaching the server.
	if problems := path.IsValidPathSegmentName(key.Name); len(problems) > 0 {
		return fmt.Errorf("invalid resource name: %v", problems)
	}
	return c.Client.Get(ctx, key, obj, opts...)
}

func (c selectorCheckingClient) List(ctx context.Context, list client.ObjectList, opts ...client.ListOption) error {
	options := (&client.ListOptions{}).ApplyOptions(opts)
	if options.LabelSelector != nil {
		if _, err := labels.Parse(options.LabelSelector.String()); err != nil {
			return err
		}
	}
	if c.listErr != nil {
		return c.listErr
	}
	return c.Client.List(ctx, list, opts...)
}

func TestAppLookupRejectsMalformedSelectorsWithoutLosingLegacyNames(t *testing.T) {
	for _, lookup := range []struct {
		name string
		call func(*Base, context.Context, string, string) (*appv1alpha1.App, error)
	}{
		{"authorize", (*Base).AuthorizeApp},
		{"get", (*Base).GetApp},
	} {
		t.Run(lookup.name, func(t *testing.T) {
			app := sampleApp("web", "tea-a")
			app.Labels[LabelAppID] = "srv-c185th5c2rvvnhbfiltg"
			longName := strings.Repeat("a", 70) // a valid CR name, too long for a label value
			longApp := sampleApp(longName, "tea-a")
			longApp.Namespace = "tea-a"
			b := resolvingBase(app, longApp)
			b.Client = selectorCheckingClient{Client: b.Client}
			for _, name := range []string{"srv-", "srv-!!", "srv- x", "not-an-id!!", ".", "..", "srv-foo/bar", "srv-100%", strings.Repeat("z", 64), strings.Repeat("z", 254), "missing", "srv-c185th5c2rvvnhbfilth"} {
				if _, err := lookup.call(b, actingCtx(), RelCanView, name); !errors.Is(err, ErrNotFound) {
					t.Errorf("lookup(%q) = %v, want not found", name, err)
				}
			}
			for _, name := range []string{"web", app.Labels[LabelAppID], longName} {
				if got, err := lookup.call(b, actingCtx(), RelCanView, name); err != nil || got == nil {
					t.Errorf("lookup(%q) = %v, %v, want existing service", name, got, err)
				}
			}
			outage := errors.New("apiserver unavailable")
			b.Client = selectorCheckingClient{Client: b.Client, listErr: outage}
			if _, err := lookup.call(b, actingCtx(), RelCanView, app.Labels[LabelAppID]); !errors.Is(err, outage) {
				t.Errorf("real List failure = %v, want %v", err, outage)
			}
		})
	}
}

func TestMalformedAppLookupPreservesAuthorizationDenial(t *testing.T) {
	b := resolvingBase()
	b.Client = selectorCheckingClient{Client: b.Client}
	b.Authz = fakeDenyChecker{}
	if _, err := b.AuthorizeApp(actingCtx(), RelCanView, "srv-!!"); !errors.Is(err, ErrForbidden) {
		t.Fatalf("denied malformed lookup = %v, want forbidden", err)
	}
}
