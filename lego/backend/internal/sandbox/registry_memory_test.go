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

package sandbox

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"slices"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"
)

// memoryRegistry is a Zot-shaped distribution API held in memory: the catalog
// with its n/last paging, tag lists and tag deletes. A repository with no tag
// left answers 404 on its tag list, as Zot v2.1.22 does, yet stays in the
// catalog, which is the less convenient of its two possible behaviours.
type memoryRegistry struct {
	srv *httptest.Server

	mu    sync.Mutex
	repos map[string]map[string]bool // repository -> its tags
	// deleted records each "repository:tag" delete that succeeded.
	deleted []string
	// catalogStatus, when not 0, answers every catalog call with it;
	// catalogCalls counts them (each page is one).
	catalogStatus int
	catalogCalls  int
	// denyDelete lists the repositories whose tag deletes the registry refuses
	// (a credential without the delete action there); failDeletes refuses all.
	denyDelete  map[string]bool
	failDeletes bool
	// attempts counts every tag delete the registry was asked for.
	attempts int
}

func newMemoryRegistry(t *testing.T) *memoryRegistry {
	t.Helper()
	r := &memoryRegistry{repos: map[string]map[string]bool{}, denyDelete: map[string]bool{}}
	r.srv = httptest.NewServer(http.HandlerFunc(r.serve))
	t.Cleanup(r.srv.Close)
	return r
}

func (r *memoryRegistry) config() RegistryConfig {
	return RegistryConfig{BaseURL: r.srv.URL, Prefix: DefaultSnapshotRepoPrefix}
}

func (r *memoryRegistry) reaper() *RegistryReaper { return &RegistryReaper{RegistryConfig: r.config()} }

func (r *memoryRegistry) add(repo string, tags ...string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.repos[repo] == nil {
		r.repos[repo] = map[string]bool{}
	}
	for _, tag := range tags {
		r.repos[repo][tag] = true
	}
}

// tags is the repository's remaining tags, sorted.
func (r *memoryRegistry) tags(repo string) []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []string
	for tag := range r.repos[repo] {
		out = append(out, tag)
	}
	sort.Strings(out)
	return out
}

func (r *memoryRegistry) deletes() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return slices.Clone(r.deleted)
}

func (r *memoryRegistry) deleteAttempts() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.attempts
}

func (r *memoryRegistry) catalogRequests() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.catalogCalls
}

// everyTag is every repository's remaining tags.
func (r *memoryRegistry) everyTag() map[string][]string {
	r.mu.Lock()
	repos := make([]string, 0, len(r.repos))
	for repo := range r.repos {
		repos = append(repos, repo)
	}
	r.mu.Unlock()
	out := map[string][]string{}
	for _, repo := range repos {
		out[repo] = r.tags(repo)
	}
	return out
}

func (r *memoryRegistry) deny(repo string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.denyDelete[repo] = true
}

func (r *memoryRegistry) setFailDeletes(fail bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.failDeletes = fail
}

func (r *memoryRegistry) setCatalogStatus(status int) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.catalogStatus = status
}

func (r *memoryRegistry) serve(w http.ResponseWriter, req *http.Request) {
	r.mu.Lock()
	defer r.mu.Unlock()
	path := strings.TrimPrefix(req.URL.Path, "/v2/")
	switch {
	case req.Method == http.MethodGet && path == "_catalog":
		r.catalogCalls++
		if r.catalogStatus != 0 {
			w.WriteHeader(r.catalogStatus)
			return
		}
		var names []string
		for name := range r.repos {
			names = append(names, name)
		}
		sort.Strings(names)
		if last := req.URL.Query().Get("last"); last != "" {
			i := sort.SearchStrings(names, last)
			if i < len(names) && names[i] == last {
				i++
			}
			names = names[i:]
		}
		if n, err := strconv.Atoi(req.URL.Query().Get("n")); err == nil && n < len(names) {
			names = names[:n]
		}
		_, _ = fmt.Fprintf(w, `{"repositories":[%s]}`, quoteAll(names))
	case req.Method == http.MethodGet && strings.HasSuffix(path, "/tags/list"):
		repo := strings.TrimSuffix(path, "/tags/list")
		var tags []string
		for tag := range r.repos[repo] {
			tags = append(tags, tag)
		}
		if len(tags) == 0 {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		sort.Strings(tags)
		_, _ = fmt.Fprintf(w, `{"name":%q,"tags":[%s]}`, repo, quoteAll(tags))
	case req.Method == http.MethodDelete && strings.Contains(path, "/manifests/"):
		i := strings.LastIndex(path, "/manifests/")
		repo, tag := path[:i], path[i+len("/manifests/"):]
		r.attempts++
		switch {
		case r.failDeletes:
			w.WriteHeader(http.StatusInternalServerError)
		case r.denyDelete[repo]:
			w.WriteHeader(http.StatusForbidden)
		case !r.repos[repo][tag]:
			w.WriteHeader(http.StatusNotFound)
		default:
			delete(r.repos[repo], tag)
			r.deleted = append(r.deleted, repo+":"+tag)
			w.WriteHeader(http.StatusAccepted)
		}
	default:
		w.WriteHeader(http.StatusNotFound)
	}
}

func quoteAll(values []string) string {
	quoted := make([]string, len(values))
	for i, v := range values {
		quoted[i] = strconv.Quote(v)
	}
	return strings.Join(quoted, ",")
}
