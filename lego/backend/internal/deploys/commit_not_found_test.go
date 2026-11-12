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
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// w8/280: a deploy of a commit SHA the repository does not have opened a
// deploy, queued a build and failed 40-55 s later with git's raw "upload-pack:
// not our ref". Render's deploy hook answers 404, "The commit SHA specified in
// the ref query parameter was not found in the service's linked repository".

func TestTriggerRefusesACommitTheRepositoryDoesNotHave(t *testing.T) {
	ds := newFakeStore()
	svc, cl := newService(ds, repoApp("web", "srv-1", "main"))
	svc.Commits = &fakeResolver{err: store.ErrCommitNotFound}

	_, err := svc.Trigger(context.Background(), "web", TriggerParams{CommitID: "0123456789abcdef0123456789abcdef01234567"})
	var coded *core.CodedError
	if !errors.Is(err, core.ErrNotFound) || !errors.As(err, &coded) || coded.Code != commitNotFoundCode {
		t.Fatalf("Trigger = %v, want a coded 404 %s", err, commitNotFoundCode)
	}
	if want := `commit "0123456789abcdef0123456789abcdef01234567" was not found in the service's linked repository`; err.Error() != want {
		t.Errorf("refusal = %q, want %q", err, want)
	}
	if list, _ := ds.ListDeploys(context.Background(), "srv-1", store.DeployFilter{}); len(list) != 0 {
		t.Errorf("a refused deploy opened %d deploy rows", len(list))
	}
	if app := getApp(t, cl, "web"); app.Spec.BuildCommit != "" {
		t.Errorf("a refused deploy wrote spec.buildCommit %q", app.Spec.BuildCommit)
	}

	// The deploy hook answers the same 404 (Render's documented answer).
	hook, err := svc.GetDeployHook(context.Background(), "web")
	if err != nil {
		t.Fatal(err)
	}
	rec := httptest.NewRecorder()
	svc.DeployHookHandler().ServeHTTP(rec, httptest.NewRequest(http.MethodPost, hook.URL+"&ref="+url.QueryEscape("0123456"), nil))
	if rec.Code != http.StatusNotFound || !strings.Contains(rec.Body.String(), commitNotFoundCode) {
		t.Errorf("hook with an unknown ref = %d %s, want 404 %s", rec.Code, rec.Body.String(), commitNotFoundCode)
	}
	// It names the parameter the hook was called with.
	if !strings.Contains(rec.Body.String(), `"params":{"ref":"0123456"}`) {
		t.Errorf("hook refusal %s should name the ref parameter", rec.Body.String())
	}
}

// Only the repository's own "no such commit" refuses. With no commitId the
// branch head deploys whatever the lookup says, a restart redeploys what runs,
// and a lookup that could not be made (GitHub down) opens the deploy as before.
func TestOnlyAMissingNamedCommitRefusesADeploy(t *testing.T) {
	for name, tc := range map[string]struct {
		err    error
		params TriggerParams
	}{
		"no commitId":        {store.ErrCommitNotFound, TriggerParams{}},
		"a restart":          {store.ErrCommitNotFound, TriggerParams{CommitID: "abc1234", restart: true}},
		"GitHub unreachable": {errors.New("github is down"), TriggerParams{CommitID: "abc1234"}},
	} {
		t.Run(name, func(t *testing.T) {
			ds := newFakeStore()
			svc, _ := newService(ds, repoApp("web", "srv-1", "main"))
			svc.Commits = &fakeResolver{err: tc.err}
			if _, err := svc.Trigger(context.Background(), "web", tc.params); err != nil {
				t.Fatalf("Trigger = %v, want the deploy to open", err)
			}
		})
	}
}
