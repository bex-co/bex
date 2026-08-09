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
	"errors"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

// predeploy_test.go covers the `predeploy` log type (w1/m33): a live read of the
// pre-deploy Job pod's logs, requested on its own (mixing it with app/request is
// refused), never the durable store.

func preDeployPod(app, name string) *corev1.Pod {
	return &corev1.Pod{ObjectMeta: metav1.ObjectMeta{
		Name: name, Namespace: "default", Labels: map[string]string{core.PodLabelPreDeploy: app},
	}}
}

func TestPreDeployLogsReadFromJobPod(t *testing.T) {
	// An app pod AND a pre-deploy pod exist; a type=predeploy query must read only
	// the latter (the migration's output), never the app container.
	svc := newService(
		map[string][]string{
			"web-1":               {"2026-07-14T00:00:00Z app line"},
			"predeploy-web-gen-3": {"2026-07-14T00:00:01Z running migrations", "2026-07-14T00:00:02Z done"},
		},
		sampleApp("web"),
		podFor("web", "web-1"),
		preDeployPod("web", "predeploy-web-gen-3"),
	)

	got, err := svc.QueryLogs(context.Background(), LogQuery{App: "web", Types: []string{LogTypePreDeploy}})
	if err != nil {
		t.Fatalf("QueryLogs(type=predeploy): %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("want the 2 pre-deploy lines, got %d: %+v", len(got), got)
	}
	for _, e := range got {
		if e.Message == "app line" {
			t.Errorf("type=predeploy leaked an app-container line: %+v", got)
		}
	}
}

func TestPreDeployTypeMustBeRequestedAlone(t *testing.T) {
	svc := newService(nil, sampleApp("web"))
	_, err := svc.QueryLogs(context.Background(), LogQuery{App: "web", Types: []string{LogTypePreDeploy, LogTypeApp}})
	if !errors.Is(err, core.ErrBadRequest) {
		t.Errorf("mixing predeploy with app should be core.ErrBadRequest, got %v", err)
	}
}

func TestNormalizeTypesAcceptsPreDeploy(t *testing.T) {
	got, err := NormalizeTypes([]string{"predeploy"})
	if err != nil {
		t.Fatalf("NormalizeTypes: %v", err)
	}
	if len(got) != 1 || got[0] != LogTypePreDeploy {
		t.Errorf("NormalizeTypes([predeploy]) = %v, want [predeploy]", got)
	}
	if _, err := NormalizeTypes([]string{"bogus"}); !errors.Is(err, core.ErrBadRequest) {
		t.Errorf("unknown type should be core.ErrBadRequest, got %v", err)
	}
}

// TestLiveSubscribableRejectsProducerlessTypes pins codex #3: the live tail has
// exactly two producers (app pod stdout, build Job stdout). A subscription that
// wants neither — predeploy (a Job-pod read, not a tail), request logs, or a
// store-only filter — must be refused with a bounded bad request rather than
// parking on ctx.Done() and holding one of the process-global SSE slots.
func TestLiveSubscribableRejectsProducerlessTypes(t *testing.T) {
	for _, q := range []LogQuery{
		{Types: []string{LogTypePreDeploy}},
		{Types: []string{LogTypeRequest}},
		{Level: []string{"error"}},
	} {
		if err := q.liveSubscribable(); !errors.Is(err, core.ErrBadRequest) {
			t.Errorf("liveSubscribable(%+v) = %v, want ErrBadRequest", q, err)
		}
	}
	// app, build, and the empty "all types" query DO have a live producer.
	for _, q := range []LogQuery{
		{Types: []string{LogTypeApp}},
		{Types: []string{LogTypeBuild}},
		{},
	} {
		if err := q.liveSubscribable(); err != nil {
			t.Errorf("liveSubscribable(%+v) = %v, want nil", q, err)
		}
	}
}
