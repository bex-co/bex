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

// A read that starts mid-log first replays an instance's earlier lines through
// its bucket without showing them, so the bucket holds what the shipper's does
// (w8/341). Before, a tail opened during a flood started full and showed up to
// 6,000 lines history had dropped. The warm-up is two refills long: a pod that
// wrote 150 lines a second or more through it (capWarmUpLines in capWarmUp)
// has emptied its bucket, as the shipper's is, and a quiet one starts full. A
// tail warms by time; the store-less read, which asks the kubelet for lines by
// count, warms by count.
const (
	capWarmUpSeconds = 2 * appLineBurst / appLineRate
	capWarmUp        = capWarmUpSeconds * time.Second
	capWarmUpLines   = appLineBurst + capWarmUpSeconds*appLineRate
)

// lineCap is one read's buckets, by instance, each the shipper's bucket for
// the same pod (its app_logs stage.limit sets refill_by_entry_timestamp,
// deploy/log-shipper/patches/limit-entry-timestamp.patch). A bucket counts a
// line at the line's own timestamp, not the clock: the shipper counts a line
// moments after it is written, while a tail may replay its start bound's
// backlog or drain late behind a slow viewer. Tokens are whole, one per
// 1/rate step of the timestamp on a grid fixed to the Unix epoch, capped at
// the burst, so two buckets fed the same lines keep the same ones once a
// flood has drained both or a quiet spell has filled both (w8/m525): their
// difference is a whole number of tokens that never grows and shrinks each
// time only one of them admits; fractional tokens would keep them apart for
// good. testdata/linecap-shared.json holds the shipper's test and this one to
// the same lines.
type lineCap struct {
	step    int64 // nanoseconds of a line's timestamp per token
	burst   int64
	buckets map[string]*lineBucket
}

type lineBucket struct {
	tokens  int64
	last    int64 // the step of the latest stamp counted
	stamped bool
}

func newLineCap() lineCap { return lineCapOf(appLineRate, appLineBurst) }

func lineCapOf(rate, burst int64) lineCap {
	return lineCap{step: int64(time.Second) / rate, burst: burst, buckets: map[string]*lineBucket{}}
}

// admit reports whether e survives its instance's bucket, spending a token if
// it does. Only an App container's own lines count: request lines, datastore
// logs and bex's platform lines are not application-generated, as on Render,
// and always pass.
func (c lineCap) admit(e LogEntry) bool {
	if e.Labels[LabelType] != LogTypeApp || e.Labels["container"] == progressContainer {
		return true
	}
	b := c.buckets[e.Labels[LabelInstance]]
	if b == nil {
		b = &lineBucket{tokens: c.burst}
		c.buckets[e.Labels[LabelInstance]] = b
	}
	// A bucket starts full at its first stamp's step, and a line stamped at
	// or before the last step counted (stderr behind stdout) refills nothing.
	// An unstamped line (the rest of a split line) is counted at the last
	// line's step.
	if at, err := time.Parse(time.RFC3339Nano, e.Timestamp); err == nil {
		switch step := at.UnixNano() / c.step; {
		case !b.stamped:
			b.last, b.stamped = step, true
		case step > b.last:
			b.tokens, b.last = min(c.burst, b.tokens+step-b.last), step
		}
	}
	if b.tokens < 1 {
		return false
	}
	b.tokens--
	return true
}
