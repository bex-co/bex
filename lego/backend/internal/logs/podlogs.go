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
	"io"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// NewPodLogSource returns the production PodLogSource, backed by a client-go
// clientset. It lives here (not service.go) so the domain layer keeps no hard
// dependency on the typed clientset — the pod-log subresource is the one read
// controller-runtime's client can't serve, so it's the one place bex-api reaches
// past the generic client.
func NewPodLogSource(cs kubernetes.Interface) PodLogSource {
	return func(ctx context.Context, namespace, pod, container string, tail int64) (io.ReadCloser, error) {
		opts := &corev1.PodLogOptions{Container: container, Timestamps: true}
		if tail > 0 {
			opts.TailLines = &tail
		}
		return cs.CoreV1().Pods(namespace).GetLogs(pod, opts).Stream(ctx)
	}
}

// NewPodLogStream returns the production PodLogStream — the live-tail sibling of
// NewPodLogSource (Follow:true). Same rationale for living here: keep the typed
// clientset out of the domain layer.
//
// since bounds where the follow starts. Without it kubelet replays the pod's
// ENTIRE log from offset 0 on every subscribe — and a browser EventSource
// reconnects on its own, invisibly to the client code, so a long-lived tail on a
// chatty pod re-read its whole history repeatedly. The lines were already
// filtered by LogQuery.keep before reaching the viewer, so this changes no
// output; it stops doing the work of shipping them from kubelet only to drop
// them here (w6/m93).
func NewPodLogStream(cs kubernetes.Interface) PodLogStream {
	return func(ctx context.Context, namespace, pod, container string, since time.Time) (io.ReadCloser, error) {
		opts := &corev1.PodLogOptions{Container: container, Timestamps: true, Follow: true}
		if !since.IsZero() {
			// Exclusive on the kubelet side is not guaranteed — SinceTime has
			// second granularity — so the boundary second can still replay. The
			// viewer's key-based dedupe absorbs that; this only has to stop the
			// unbounded replay, not be exact.
			opts.SinceTime = &metav1.Time{Time: since}
		}
		return cs.CoreV1().Pods(namespace).GetLogs(pod, opts).Stream(ctx)
	}
}
