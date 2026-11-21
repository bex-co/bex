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
	"net/http"
	"net/url"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/bex-co/bex/lego/backend/internal/core"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// deleting stamps obj as a CR whose delete Kubernetes accepted while its
// finalizer still holds it.
func deleting[T client.Object](obj T) T {
	now := metav1.NewTime(time.Unix(1_700_000_000, 0))
	obj.SetDeletionTimestamp(&now)
	obj.SetFinalizers([]string{"app.bex.co/finalizer"})
	return obj
}

// w8/366: for about 10 s after `bex services delete`, GET /v1/logs and
// /v1/logs/values still answered 200 with the service's lines and instance ids
// while every other read of it was 404. A service, Postgres or Key Value whose
// delete was accepted now has no logs to read, as one that never existed has
// none; live resources keep theirs.
func TestDeletingResourcesHaveNoLogs(t *testing.T) {
	lines := []string{"2026-07-05T00:00:01Z boot ok"}
	svc := newService(map[string][]string{"gone-1": lines, "web-1": lines, postgresID + "-1": lines, keyValueID + "-0": lines},
		deleting(sampleApp("gone")), podFor("gone", "gone-1"),
		sampleApp("web"), podFor("web", "web-1"),
		deleting(sampleDatabase(postgresID)), databasePod(postgresID, postgresID+"-1"),
		deleting(&appv1alpha1.KeyValue{ObjectMeta: metav1.ObjectMeta{Name: keyValueID, Namespace: "default"}}),
		keyValuePod(keyValueID, keyValueID+"-0"),
	)
	window := url.Values{"startTime": {"2026-07-05T00:00:00Z"}, "endTime": {"2026-07-05T00:01:00Z"}}
	get := func(path, resource string, extra url.Values) int {
		q := url.Values{"resource": {resource}}
		for k, v := range window {
			q[k] = v
		}
		for k, v := range extra {
			q[k] = v
		}
		return serveREST(svc, http.MethodGet, path+"?"+q.Encode()).Code
	}
	for _, resource := range []string{"gone", postgresID, keyValueID} {
		if code := get("/v1/logs", resource, nil); code != http.StatusNotFound {
			t.Errorf("GET /v1/logs for deleting %s = %d, want 404", resource, code)
		}
		if code := get("/v1/logs/values", resource, url.Values{"label": {"instance"}}); code != http.StatusNotFound {
			t.Errorf("GET /v1/logs/values for deleting %s = %d, want 404", resource, code)
		}
		if _, err := svc.LogRetention(context.Background(), resource); !errors.Is(err, core.ErrNotFound) {
			t.Errorf("LogRetention for deleting %s = %v, want ErrNotFound", resource, err)
		}
	}
	if code := get("/v1/logs", "web", nil); code != http.StatusOK {
		t.Errorf("GET /v1/logs for a live service = %d, want 200", code)
	}
	// Without a log store the values read is 503 for any resource; what matters
	// is that a live service is not refused as absent.
	if code := get("/v1/logs/values", "web", url.Values{"label": {"instance"}}); code == http.StatusNotFound {
		t.Errorf("GET /v1/logs/values for a live service = %d, want it found", code)
	}
}
