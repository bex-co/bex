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
	"errors"
	"fmt"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

// paging_test.go holds the two sensitive list verbs to ONE paging policy.
//
// ListEnvVarsPage and ListSecretFilesPage carried the same four policy
// branches verbatim — absent-both ⇒ the whole list, absent-limit-with-cursor ⇒
// DefaultPageLimit, negative ⇒ 400, oversized ⇒ clamp — and only the
// secret-files side was ever tested (files_test.go's boundary case). So the
// env-vars copy could have lost any branch silently: a dropped negative check
// there means a mistyped limit reaches core.Page's `start + limit` and returns
// an EMPTY page, which reads as "this service has no env vars".
//
// The table runs both verbs through every branch, so the shared applyPageLimits
// cannot regress on one side only, and neither can a future third list that
// reimplements it.

// pagedLister adapts one list verb to a common shape: seed n items, then page.
type pagedLister struct {
	name string
	// seed writes n items and returns their cursors in list order.
	seed func(t *testing.T, svc *Service, ctx context.Context, n int) []string
	// page invokes the verb, returning the page's cursors.
	page func(svc *Service, ctx context.Context, after string, limit int) ([]string, error)
}

func pagedListers() []pagedLister {
	return []pagedLister{
		{
			name: "ListEnvVarsPage",
			seed: func(t *testing.T, svc *Service, ctx context.Context, n int) []string {
				t.Helper()
				vars := make([]EnvVarView, 0, n)
				keys := make([]string, 0, n)
				for _, k := range firstNKeys(n) {
					vars = append(vars, EnvVarView{Key: k, Value: "v"})
					keys = append(keys, k)
				}
				if _, err := svc.SetEnvVars(ctx, "web", vars); err != nil {
					t.Fatalf("seed env vars: %v", err)
				}
				return keys
			},
			page: func(svc *Service, ctx context.Context, after string, limit int) ([]string, error) {
				vars, err := svc.ListEnvVarsPage(ctx, "web", after, limit)
				if err != nil {
					return nil, err
				}
				out := make([]string, len(vars))
				for i, v := range vars {
					out[i] = v.Key
				}
				return out, nil
			},
		},
		{
			name: "ListSecretFilesPage",
			// Seeded through SeedSecretFiles (one store write + one
			// materialize), the structural mirror of SetEnvVars on the other
			// lister. The obvious n× SetSecretFile loop costs ~73× the time and
			// ~78× the memory for an identical resulting list, because each
			// call repeats a read-modify-write plus an App patch.
			seed: func(t *testing.T, svc *Service, ctx context.Context, n int) []string {
				t.Helper()
				files := make([]core.SecretFile, 0, n)
				names := make([]string, 0, n)
				for _, k := range firstNKeys(n) {
					files = append(files, core.SecretFile{Name: k + ".pem", Content: "body"})
					names = append(names, k+".pem")
				}
				if err := svc.SeedSecretFiles(ctx, "web", files); err != nil {
					t.Fatalf("seed secret files: %v", err)
				}
				return names
			},
			page: func(svc *Service, ctx context.Context, after string, limit int) ([]string, error) {
				files, err := svc.ListSecretFilesPage(ctx, "web", after, limit)
				if err != nil {
					return nil, err
				}
				out := make([]string, len(files))
				for i, f := range files {
					out[i] = f.Name
				}
				return out, nil
			},
		},
	}
}

// firstNKeys returns n lexicographically sorted keys, so seeded order == list
// order. Zero-padded because the list is sorted as strings, and "K10" would
// otherwise sort before "K9".
func firstNKeys(n int) []string {
	out := make([]string, 0, n)
	for i := range n {
		out = append(out, fmt.Sprintf("K%03d", i))
	}
	return out
}

// seedSize is deliberately LARGER than both page bounds. At a smaller size
// every branch of the policy returns the same thing — the whole list, the
// default page, and the clamped page are all "the 5 items that exist" — so the
// negative controls for the whole-list and clamp branches did not fire until
// the fixture outgrew them. Sized so each branch has a distinct observable
// count: whole list = seedSize, default page = DefaultPageLimit, clamped page
// = MaxPageLimit.
const seedSize = core.MaxPageLimit + 5

func TestBothSensitiveListsShareOnePagingPolicy(t *testing.T) {
	for _, lister := range pagedListers() {
		t.Run(lister.name, func(t *testing.T) {
			svc, ctx := pagingHarness(t)
			seeded := lister.seed(t, svc, ctx, seedSize)

			t.Run("neither cursor nor limit returns the whole list", func(t *testing.T) {
				got, err := lister.page(svc, ctx, "", 0)
				if err != nil {
					t.Fatalf("page: %v", err)
				}
				if len(got) != seedSize {
					t.Fatalf("got %d items, want all %d — the pre-pagination contract; "+
						"%d would mean it fell through to the default page", len(got), seedSize, core.DefaultPageLimit)
				}
			})

			t.Run("a negative limit is a 400, never an empty page", func(t *testing.T) {
				got, err := lister.page(svc, ctx, "", -1)
				if !errors.Is(err, core.ErrLimitNotPositive) {
					t.Fatalf("err = %v (got %d items), want ErrLimitNotPositive — an empty page here reads as "+
						"'this service has none', which is invisible to the caller", err, len(got))
				}
			})

			t.Run("an oversized limit clamps to MaxPageLimit", func(t *testing.T) {
				got, err := lister.page(svc, ctx, "", core.MaxPageLimit+50)
				if err != nil {
					t.Fatalf("page: %v", err)
				}
				if len(got) != core.MaxPageLimit {
					t.Fatalf("got %d items, want the clamp at %d (seeded %d)", len(got), core.MaxPageLimit, seedSize)
				}
			})

			t.Run("a cursor with no limit uses DefaultPageLimit", func(t *testing.T) {
				got, err := lister.page(svc, ctx, seeded[0], 0)
				if err != nil {
					t.Fatalf("page: %v", err)
				}
				if len(got) != core.DefaultPageLimit {
					t.Fatalf("tail after %q = %d items, want DefaultPageLimit (%d)",
						seeded[0], len(got), core.DefaultPageLimit)
				}
			})

			t.Run("an unknown cursor yields an empty page", func(t *testing.T) {
				got, err := lister.page(svc, ctx, "zzz-nonexistent", 10)
				if err != nil || len(got) != 0 {
					t.Fatalf("unknown cursor = %d items, %v; want an empty page", len(got), err)
				}
			})
		})
	}
}

func pagingHarness(t *testing.T) (*Service, context.Context) {
	t.Helper()
	return newService(newFakeSecretStore(), sampleApp("web")), context.Background()
}
