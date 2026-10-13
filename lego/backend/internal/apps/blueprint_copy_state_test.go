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
	"strings"
	"testing"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/envgroups"
	"github.com/bex-co/bex/lego/backend/internal/secrets"
)

// w8/m64: the envVarKey copy index assumed a first create. An existing
// source keeps its current group links and appends newly declared ones, and
// keeps an edited seed, so its effective value can differ from the declared
// guess. Through the real env-group and secrets services, a copy now equals
// the source's effective value, or is refused at the caller's entry before
// any write when that value is not knowable from the Blueprint.
func TestBlueprintCopiesFollowTheSourcesEffectiveConfiguration(t *testing.T) {
	const image = `"runtime":"image","image":{"url":"nginx:1"}`
	groups := `"envVarGroups":[{"name":"a","envVars":[{"key":"VALUE","value":"group a"}]},{"name":"b","envVars":[{"key":"VALUE","value":"group b"}]}]`
	doc := func(sourceEnv string) string {
		return `{` + groups + `,"services":[
  {"name":"consumer","type":"web",` + image + `,"envVars":[{"key":"COPY","fromService":{"type":"web","name":"source","envVarKey":"VALUE"}}]},
  {"name":"source","type":"web",` + image + `,"envVars":[` + sourceEnv + `]}]}`
	}
	link := func(names ...string) string {
		var out []string
		for _, n := range names {
			out = append(out, `{"fromGroup":"`+n+`"}`)
		}
		return strings.Join(out, ",")
	}
	newServices := func() (*Service, *envgroups.Service) {
		base := &core.Base{Client: fakeClient(), Namespace: "default", Clock: func() time.Time { return time.Unix(1_000_000, 0).UTC() }}
		kv := newMemKV()
		eg := &envgroups.Service{Base: base, Store: kv}
		sec := &secrets.Service{Base: base, Store: kv}
		return &Service{Base: base, EnvGroups: eg, EnvSeeder: sec, EnvNames: sec}, eg
	}
	copyOf := func(t *testing.T, svc *Service) string {
		t.Helper()
		for _, e := range getApp(t, svc.Client, "consumer").Spec.Env {
			if e.Name == "COPY" {
				return e.Value
			}
		}
		t.Fatal("consumer has no COPY")
		return ""
	}
	for _, tc := range []struct {
		name          string
		first, second string // the second apply runs against the first's state
		want          string
	}{
		{"fresh duplicate links", doc(link("a", "b", "a")), "", "group b"},
		{"reapply in reverse order keeps current links", doc(link("a", "b")), doc(link("b", "a")), "group b"},
		{"new link appends after the current one", doc(link("b")), doc(link("a", "b")), "group a"},
		{"omitted link stays linked", doc(link("a", "b")), doc(link("a")), "group b"},
		{"unchanged links", doc(link("a", "b")), doc(link("a", "b")), "group b"},
		{"own literal overrides groups", doc(link("a", "b") + `,{"key":"VALUE","value":"own"}`), doc(link("b", "a") + `,{"key":"VALUE","value":"own"}`), "own"},
		{"own empty overrides groups", doc(link("a") + `,{"key":"VALUE","value":""}`), doc(link("a") + `,{"key":"VALUE","value":""}`), ""},
		{"first-create seed copies", doc(`{"key":"VALUE","value":"seed","sync":false}`), "", "seed"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			svc, _ := newServices()
			ctx := context.Background()
			if _, err := svc.DeployStack(ctx, DeployRequest{Manifest: tc.first}); err != nil {
				t.Fatalf("first apply: %v", err)
			}
			if tc.second != "" {
				if v, err := svc.ValidateBlueprint(ctx, "", tc.second, ""); err != nil || !v.Valid {
					t.Fatalf("validation against current state = %+v, %v; want valid", v, err)
				}
				if _, err := svc.DeployStack(ctx, DeployRequest{Manifest: tc.second}); err != nil {
					t.Fatalf("second apply: %v", err)
				}
			}
			if got := copyOf(t, svc); got != tc.want {
				t.Fatalf("COPY = %q, want the source's effective %q", got, tc.want)
			}
		})
	}

	// A link the file no longer declares keeps projecting values this parse
	// cannot know, so a copy from that source is refused.
	t.Run("retained link to an undeclared group", func(t *testing.T) {
		svc, _ := newServices()
		ctx := context.Background()
		if _, err := svc.DeployStack(ctx, DeployRequest{Manifest: doc(link("a", "b"))}); err != nil {
			t.Fatalf("first apply: %v", err)
		}
		onlyA := strings.Replace(doc(link("a")), `,{"name":"b","envVars":[{"key":"VALUE","value":"group b"}]}`, "", 1)
		if _, err := svc.DeployStack(ctx, DeployRequest{Manifest: onlyA}); err == nil || !strings.Contains(err.Error(), `links group "b" that this file does not declare`) {
			t.Fatalf("reapply = %v, want the undeclared-link refusal", err)
		}
	})

	// An existing source's seed was seeded once and may since have been
	// edited: the copy is refused, located, and the reapply writes nothing.
	t.Run("existing seed is not knowable", func(t *testing.T) {
		svc, _ := newServices()
		ctx := context.Background()
		seeded := doc(`{"key":"VALUE","value":"seed","sync":false}`)
		if _, err := svc.DeployStack(ctx, DeployRequest{Manifest: seeded}); err != nil {
			t.Fatalf("first apply: %v", err)
		}
		before := getApp(t, svc.Client, "consumer").ResourceVersion
		_, err := svc.DeployStack(ctx, DeployRequest{Manifest: strings.Replace(seeded, `"value":"seed"`, `"value":"changed"`, 1)})
		if !errors.Is(err, core.ErrBadRequest) || !strings.Contains(err.Error(), "seeded once") {
			t.Fatalf("reapply = %v, want the seed refusal", err)
		}
		if after := getApp(t, svc.Client, "consumer").ResourceVersion; after != before {
			t.Fatalf("a refused reapply wrote the consumer (%s -> %s)", before, after)
		}
		v, err := svc.ValidateBlueprint(ctx, "", seeded, "")
		if err != nil || v.Valid || len(v.Errors) != 1 || v.Errors[0].Path == nil || *v.Errors[0].Path != "services[0].envVars[0]" {
			t.Fatalf("validation = %+v, %v; want one refusal at the consumer's COPY entry", v, err)
		}
	})

	// t007: the source's own store projects after its groups, so a same-key
	// variable there, even empty, wins; its value is not read, so the copy is
	// refused. An unrelated own key keeps the copy. (The store refuses a write to
	// a manifest-owned key, so a manifest literal cannot be shadowed there.)
	for _, tc := range []struct {
		name, ownKey, ownValue, sourceEnv, want string
	}{
		{"edited own override", "VALUE", "edited", link("a"), ""},
		{"empty own override", "VALUE", "", link("a"), ""},
		{"unrelated own key", "OTHER", "x", link("a"), "group a"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			svc, _ := newServices()
			ctx := context.Background()
			manifest := doc(tc.sourceEnv)
			if _, err := svc.DeployStack(ctx, DeployRequest{Manifest: manifest}); err != nil {
				t.Fatalf("first apply: %v", err)
			}
			if _, err := svc.EnvSeeder.(*secrets.Service).SetEnvVar(ctx, "source", tc.ownKey, secrets.EnvVarWrite{Value: tc.ownValue}); err != nil {
				t.Fatalf("own override: %v", err)
			}
			before := getApp(t, svc.Client, "consumer").ResourceVersion
			_, err := svc.DeployStack(ctx, DeployRequest{Manifest: manifest})
			if tc.want == "" {
				if !errors.Is(err, core.ErrBadRequest) || !strings.Contains(err.Error(), "its own environment also sets") {
					t.Fatalf("reapply = %v, want the own-override refusal", err)
				}
				if after := getApp(t, svc.Client, "consumer").ResourceVersion; after != before {
					t.Fatalf("a refused reapply wrote the consumer")
				}
				return
			}
			if err != nil {
				t.Fatalf("reapply: %v", err)
			}
			if got := copyOf(t, svc); got != tc.want {
				t.Fatalf("COPY = %q, want %q", got, tc.want)
			}
		})
	}

	// t008: a manifest literal is container env and wins over a same-key
	// sync:false seed in either entry order, so the copy is the literal on
	// first apply and stays eligible on reapply.
	for _, tc := range []struct{ name, sourceEnv, want string }{
		{"literal then seed", `{"key":"VALUE","value":"literal"},{"key":"VALUE","value":"seed","sync":false}`, "literal"},
		{"seed then literal", `{"key":"VALUE","value":"seed","sync":false},{"key":"VALUE","value":"literal"}`, "literal"},
		{"empty literal then seed", `{"key":"VALUE","value":""},{"key":"VALUE","value":"seed","sync":false}`, ""},
		{"seed then empty literal", `{"key":"VALUE","value":"seed","sync":false},{"key":"VALUE","value":""}`, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			svc, _ := newServices()
			ctx := context.Background()
			manifest := doc(tc.sourceEnv)
			for i := 0; i < 2; i++ {
				if _, err := svc.DeployStack(ctx, DeployRequest{Manifest: manifest}); err != nil {
					t.Fatalf("apply %d: %v", i+1, err)
				}
				if got := copyOf(t, svc); got != tc.want {
					t.Fatalf("apply %d COPY = %q, want the literal %q", i+1, got, tc.want)
				}
			}
		})
	}
}
