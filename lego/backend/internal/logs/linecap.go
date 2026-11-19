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

import "time"

// Render's per-instance limit (w3/m506): "Render processes a maximum of 6,000
// application-generated log lines per minute for each running instance. Excess
// lines are dropped and do not appear in the explorer or log streams"
// (render.com/docs/logging). The shipper keeps a token bucket per pod, refilled
// at appLineRate up to appLineBurst, and drops the rest before Loki stores it
// (deploy/gitops/base/log-shipper.yaml, app_logs). A live tail reads the pods
// directly, so it keeps the same bucket per instance, or it would show lines
// history never stored. scripts/gitops-validate.sh pins the shipper's values.
const (
	appLineRate  = 100  // lines a second: 6,000 a minute
	appLineBurst = 6000 // lines
)

// lineCap is one tail's buckets, by instance. A bucket starts full and is
// advanced by each line's own timestamp, not by the clock: the shipper counts
// a line moments after it is written, while a tail may replay its start
// bound's backlog or drain late behind a slow viewer.
type lineCap map[string]*lineBucket

type lineBucket struct {
	tokens float64
	last   time.Time
}

// admit reports whether e survives its instance's bucket, spending a token if
// it does. Only an App container's own lines count: request lines, datastore
// logs and bex's platform lines are not application-generated, as on Render,
// and always pass.
func (c lineCap) admit(e LogEntry) bool {
	if e.Labels[LabelType] != LogTypeApp || e.Labels["container"] == progressContainer {
		return true
	}
	b := c[e.Labels[LabelInstance]]
	if b == nil {
		b = &lineBucket{tokens: appLineBurst}
		c[e.Labels[LabelInstance]] = b
	}
	// An unstamped line (the rest of a split line) is counted at the last
	// line's time.
	if at, err := time.Parse(time.RFC3339Nano, e.Timestamp); err == nil && at.After(b.last) {
		if !b.last.IsZero() {
			b.tokens = min(appLineBurst, b.tokens+at.Sub(b.last).Seconds()*appLineRate)
		}
		b.last = at
	}
	if b.tokens < 1 {
		return false
	}
	b.tokens--
	return true
}
