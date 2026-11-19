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

package podmetrics

import "testing"

// An edge-cached service's pods carry the platform's edge-cache sidecar beside
// the service's own containers. Render's cache runs on its CDN, outside the
// instance, so the sidecar's usage is never the service's: the autoscaler and
// bex-api's current usage both read this decoder, and neither may count it
// (w3/m487). Every other container still adds up.
func TestParseLeavesTheEdgeCacheSidecarOut(t *testing.T) {
	raw := []byte(`{"items":[
	  {"metadata":{"name":"web-b"},"timestamp":"2026-11-18T00:00:00Z","containers":[
	    {"name":"app","usage":{"cpu":"200m","memory":"300Mi"}},
	    {"name":"edge-cache","usage":{"cpu":"80m","memory":"150Mi"}}]},
	  {"metadata":{"name":"web-a"},"timestamp":"2026-11-18T00:00:00Z","containers":[
	    {"name":"app","usage":{"cpu":"250m","memory":"256Mi"}},
	    {"name":"helper","usage":{"cpu":"50m","memory":"64Mi"}}]}
	]}`)
	got, err := Parse(raw)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(got) != 2 || got[0].Pod != "web-a" || got[1].Pod != "web-b" {
		t.Fatalf("want web-a, web-b in name order, got %+v", got)
	}
	const mi = 1 << 20
	if got[0].MemoryBytes != 320*mi || !near(got[0].CPUCores, 0.3) {
		t.Errorf("web-a sums its containers: got %v cores, %v bytes; want 0.3 cores, %v bytes", got[0].CPUCores, got[0].MemoryBytes, 320*mi)
	}
	if got[1].MemoryBytes != 300*mi || !near(got[1].CPUCores, 0.2) {
		t.Errorf("web-b counts only its own container: got %v cores, %v bytes; want 0.2 cores, %v bytes", got[1].CPUCores, got[1].MemoryBytes, 300*mi)
	}
}

func near(got, want float64) bool { return got > want-1e-9 && got < want+1e-9 }
