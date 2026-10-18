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

package envgroups

import (
	"context"
	"testing"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/core/coretest"
	"github.com/bex-co/bex/lego/backend/internal/envgroups/envgroupstest"
)

// mustTime parses an RFC3339 test timestamp.
func mustTime(raw string) time.Time {
	t, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		panic(err)
	}
	return t.UTC()
}

// fake returns the fakeStore a test SecretKV is or embeds, so its groups can
// be found from Service.Store.
func (f *fakeStore) fake() *fakeStore { return f }

// groupsOf is the fakeGroups that travels with a test's fake OpenBao store:
// services built over one store share its groups, as two bex-api replicas
// share one database.
func groupsOf(kv core.SecretKV) *envgroupstest.Groups {
	if f, ok := kv.(interface{ fake() *fakeStore }); ok {
		return f.fake().groupStore()
	}
	return envgroupstest.New()
}

func (f *fakeStore) groupStore() *envgroupstest.Groups {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.groups == nil {
		f.groups = envgroupstest.New()
	}
	return f.groups
}

// withGroups completes a test-built Service the way the composition root
// does: the metadata store beside the OpenBao one, and — for a test that set
// no workspace resolver — one default workspace every caller belongs to.
func withGroups(s *Service) *Service {
	if s.Groups == nil {
		s.Groups = groupsOf(s.Store)
	}
	if s.Base != nil && s.Workspace == nil {
		s.Workspace = coretest.Workspaces{core.DefaultTenant}
	}
	return s
}

// testCtx is a request context with a caller identity, which workspace
// resolution needs.
func testCtx() context.Context {
	return core.WithIdentity(context.Background(), core.Identity{Subject: "tester", Method: "session"})
}

// tctx is testCtx bound to the test's lifetime.
func tctx(t *testing.T) context.Context {
	return core.WithIdentity(t.Context(), core.Identity{Subject: "tester", Method: "session"})
}

// groups is the fakeGroups behind a test service.
func (s *Service) fakeGroups() *envgroupstest.Groups { return s.Groups.(*envgroupstest.Groups) }

// idNamed is the id of the acting workspace's group named name ("" if none).
func idNamed(ctx context.Context, s *Service, name string) string {
	g, _, _ := s.findGroupByName(ctx, name)
	return g.ID
}
