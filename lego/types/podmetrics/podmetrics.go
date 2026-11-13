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

// Package podmetrics decodes the metrics.k8s.io PodMetricsList subset bex
// reads. It is a leaf utility shared by both sides of the App CR boundary: the
// operator autoscaler scales from it and bex-api reports the same pods' current
// CPU/memory on the service's metrics surfaces. The two must interpret the API
// payload identically, or an App would scale on one number while its dashboard
// shows another; a second, hand-copied decoder would drift silently.
package podmetrics

import (
	"encoding/json"
	"fmt"
	"sort"

	"k8s.io/apimachinery/pkg/api/resource"
)

// Usage is one pod's current resource consumption, as reported by
// metrics-server. CPUCores is fractional vCPU, MemoryBytes is bytes.
type Usage struct {
	Pod         string
	Timestamp   string
	CPUCores    float64
	MemoryBytes float64
}

// Parse decodes a metrics-server PodMetricsList and sums each pod's container
// usage into cores + bytes. Unparseable quantities are treated as zero. The
// result is ordered by pod name so both callers see a stable sequence.
func Parse(raw []byte) ([]Usage, error) {
	var list podMetricsList
	if err := json.Unmarshal(raw, &list); err != nil {
		return nil, fmt.Errorf("decode pod metrics: %w", err)
	}
	out := make([]Usage, 0, len(list.Items))
	for _, it := range list.Items {
		u := Usage{Pod: it.Metadata.Name, Timestamp: it.Timestamp}
		for _, ctr := range it.Containers {
			if q, err := resource.ParseQuantity(ctr.Usage.CPU); err == nil {
				u.CPUCores += q.AsApproximateFloat64()
			}
			if q, err := resource.ParseQuantity(ctr.Usage.Memory); err == nil {
				u.MemoryBytes += float64(q.Value())
			}
		}
		out = append(out, u)
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Pod < out[j].Pod })
	return out, nil
}

// podMetricsList is the subset of metrics.k8s.io/v1beta1 PodMetricsList bex reads.
type podMetricsList struct {
	Items []struct {
		Metadata struct {
			Name string `json:"name"`
		} `json:"metadata"`
		Timestamp  string `json:"timestamp"`
		Containers []struct {
			Usage struct {
				CPU    string `json:"cpu"`
				Memory string `json:"memory"`
			} `json:"usage"`
		} `json:"containers"`
	} `json:"items"`
}
