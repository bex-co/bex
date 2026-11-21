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
	"fmt"
	"io"
	"slices"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/core"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// A disk service's Recreate deploy runs a disk holder's pod beside its
// instance (w6/m559). It carries the App's label and runs no app container,
// so the kubelet refuses a read of that container on it, and the store-less
// read failed whole while the holder ran.
func TestStoreLessLogReadsLeaveTheDiskHolderOut(t *testing.T) {
	holder := podFor("web", "dskhld-web-x2k9p")
	holder.Labels[core.PodLabelComponent] = appv1alpha1.ComponentDiskHolder
	svc := newService(map[string][]string{
		"web-1": {"2026-11-21T12:00:00Z serving"},
	}, sampleApp("web"), podFor("web", "web-1"), holder)
	source := svc.PodLogs
	svc.PodLogs = func(ctx context.Context, ns, pod, container string, tail int64) (io.ReadCloser, error) {
		if pod == holder.Name {
			return nil, fmt.Errorf("container %s is not valid for pod %s", container, pod)
		}
		return source(ctx, ns, pod, container, tail)
	}

	got, err := svc.QueryLogs(context.Background(), LogQuery{App: "web"})
	if err != nil {
		t.Fatalf("QueryLogs with a disk holder running: %v", err)
	}
	if len(got) != 1 || got[0].Message != "serving" {
		t.Errorf("entries = %+v, want the instance's one line", got)
	}
	names, err := svc.appPodNames(context.Background(), "default", LogQuery{App: "web"})
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(names, []string{"web-1"}) {
		t.Errorf("pods the tail and history read = %v, want the instance alone", names)
	}
}
