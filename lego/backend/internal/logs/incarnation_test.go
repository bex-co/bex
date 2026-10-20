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

package logs

import (
	"context"
	"slices"
	"strconv"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

// The w8/m124 shape: "web" was deleted and recreated. The new App has its own
// id and creation instant; the old one's pod is still terminating.
const (
	oldWebID = "srv-dbbibd9q56vc73bn8dgg"
	newWebID = "srv-dbbibk9q56vc73bn8dig"
)

func TestLokiRangeStartsAtTheAppsCreation(t *testing.T) {
	now := time.Date(2026, 10, 20, 9, 0, 0, 0, time.UTC)
	created := now.Add(-time.Hour)
	for _, tc := range []struct {
		name      string
		since     time.Time
		notBefore time.Time
		want      time.Time
	}{
		{"an earlier caller start is raised to the App's creation", now.Add(-3 * time.Hour), created, created},
		{"no caller start: the default lookback, raised to the creation", time.Time{}, created, created},
		{"a later caller start is kept", now.Add(-time.Minute), created, now.Add(-time.Minute)},
		// An App older than the lookback never widens the window.
		{"an old App keeps the default lookback", time.Time{}, now.Add(-30 * 24 * time.Hour), now.Add(-lokiLookback)},
		{"unscoped reads are unchanged", time.Time{}, time.Time{}, now.Add(-lokiLookback)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			start, _ := lokiRange(LogQuery{Since: tc.since, notBefore: tc.notBefore}, now)
			if !start.Equal(tc.want) {
				t.Errorf("start = %s, want %s", start, tc.want)
			}
		})
	}
}

func TestOwnPodsDropsAnEarlierIncarnationsPod(t *testing.T) {
	pod := func(name, id string) *corev1.Pod {
		p := podFor("web", name)
		if id != "" {
			p.Labels[core.LabelAppID] = id
		}
		return p
	}
	app := sampleApp("web")
	app.Labels = map[string]string{core.LabelAppID: newWebID}
	svc := newService(nil, app, pod("web-old-x", oldWebID), pod("web-new-y", newWebID), pod("web-predeploy-z", ""))
	var q LogQuery
	q.scopeToApp(app)
	pods, err := svc.ownPods(context.Background(), "default", q)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, p := range pods {
		names = append(names, p.Name)
	}
	slices.Sort(names)
	if !slices.Equal(names, []string{"web-new-y", "web-predeploy-z"}) {
		t.Fatalf("own pods = %v, want the new incarnation's and the unlabeled Job pod, never the old one's", names)
	}
}

// TestQueryLogsReadsOnlyTheCurrentIncarnation: the Loki read for a recreated
// service selects its own id (or unlabeled pre-rollout streams) and starts at
// its creation, so the deleted predecessor's lines cannot match.
func TestQueryLogsReadsOnlyTheCurrentIncarnation(t *testing.T) {
	f := newFakeLoki(lokiResp(map[string]any{
		"stream": `{"app":"web","pod":"web-new-y","container":"app"}`,
		"values": `[["1760950000000000000","mine"]]`,
	}))
	defer f.srv.Close()
	created := time.Now().Add(-10 * time.Minute).Truncate(time.Second)
	app := sampleApp("web")
	app.Labels = map[string]string{core.LabelAppID: newWebID}
	app.CreationTimestamp = metav1.NewTime(created)
	svc := newService(nil, app)
	svc.History = NewLokiSource(f.srv.URL, f.srv.Client())

	if _, err := svc.QueryLogs(context.Background(), LogQuery{App: newWebID, Since: created.Add(-time.Hour)}); err != nil {
		t.Fatal(err)
	}
	if q := f.lastValues.Get("query"); q != `{namespace="default", app="web", app_id=~"`+newWebID+`|", container=~"app|"}` {
		t.Errorf("Loki query = %s, want it scoped to %s", q, newWebID)
	}
	// The start is the later of the caller's and the App's floor
	// (core.AppNotBefore: the earlier of the CR's creation and the id's mint).
	start, err := strconv.ParseInt(f.lastValues.Get("start"), 10, 64)
	want := core.AppNotBefore(app)
	if since := created.Add(-time.Hour); since.After(want) {
		want = since
	}
	if err != nil || !time.Unix(0, start).Equal(want) {
		t.Errorf("Loki start = %v (err %v), want the App's floor %s", time.Unix(0, start), err, want)
	}
}
