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

package api

import (
	"context"
	"io"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
)

// NewPodLogSource returns the production PodLogSource for Core.Logs, backed by a
// client-go clientset. It lives here (not core.go) so the domain layer keeps no
// hard dependency on the typed clientset — the pod-log subresource is the one
// read controller-runtime's client can't serve, so it's the one place bex-api
// reaches past the generic client.
func NewPodLogSource(cs kubernetes.Interface) PodLogSource {
	return func(ctx context.Context, namespace, pod, container string, tail int64) (io.ReadCloser, error) {
		opts := &corev1.PodLogOptions{Container: container, Timestamps: true}
		if tail > 0 {
			opts.TailLines = &tail
		}
		return cs.CoreV1().Pods(namespace).GetLogs(pod, opts).Stream(ctx)
	}
}
