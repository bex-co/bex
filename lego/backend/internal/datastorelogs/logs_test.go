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

package datastorelogs

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

// podStreams fakes core.PodLogSource from a pod -> raw log text map. A pod with
// no entry fails to read, standing in for one that restarted or was reaped.
func podStreams(streams map[string]string) (core.PodLogSource, *[]string) {
	var containers []string
	return func(_ context.Context, _, pod, container string, _ int64) (io.ReadCloser, error) {
		containers = append(containers, container)
		body, ok := streams[pod]
		if !ok {
			return nil, errors.New("pod is gone")
		}
		return io.NopCloser(strings.NewReader(body)), nil
	}, &containers
}

func instance(t *testing.T, streams map[string]string, pods ...string) (Instance, *[]string) {
	t.Helper()
	source, containers := podStreams(streams)
	return Instance{
		Namespace: "bex-apps",
		Name:      "db-1",
		Kind:      KindPostgres,
		Container: core.CNPGPostgresContainer,
		Pods:      pods,
		PodLogs:   source,
	}, containers
}

func messages(entries []Entry) []string {
	out := make([]string, 0, len(entries))
	for _, e := range entries {
		out = append(out, e.Message)
	}
	return out
}

func stamped(ts, msg string) string { return ts + " " + msg + "\n" }

// TestCollectParsesAndLabels pins the wire shape: kubelet's RFC3339Nano prefix
// is split off, and every entry carries the instance identity Render clients
// filter on.
func TestCollectParsesAndLabels(t *testing.T) {
	in, containers := instance(t, map[string]string{
		"db-1-0": stamped("2026-08-07T10:00:00.5Z", "database system is ready"),
	}, "db-1-0")

	got, err := Collect(context.Background(), in, Query{})
	if err != nil {
		t.Fatalf("Collect: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("got %d entries; want 1", len(got))
	}
	if got[0].Message != "database system is ready" {
		t.Errorf("message = %q", got[0].Message)
	}
	if got[0].Timestamp != "2026-08-07T10:00:00.5Z" {
		t.Errorf("timestamp = %q; want the normalized UTC stamp", got[0].Timestamp)
	}
	want := map[string]string{"service": "db-1", "instance": "db-1-0", "type": KindPostgres}
	for k, v := range want {
		if got[0].Labels[k] != v {
			t.Errorf("label %q = %q; want %q", k, got[0].Labels[k], v)
		}
	}
	if len(*containers) != 1 || (*containers)[0] != core.CNPGPostgresContainer {
		t.Errorf("read containers = %v; want only the datastore container", *containers)
	}
}

// TestCollectKeepsUnstampedLine covers a line the datastore emitted without a
// parseable leading stamp: it must survive with its full text as the message,
// never be silently dropped.
func TestCollectKeepsUnstampedLine(t *testing.T) {
	in, _ := instance(t, map[string]string{
		"db-1-0": "no leading timestamp here\n",
	}, "db-1-0")

	got, err := Collect(context.Background(), in, Query{})
	if err != nil {
		t.Fatalf("Collect: %v", err)
	}
	if len(got) != 1 || got[0].Message != "no leading timestamp here" {
		t.Fatalf("got %+v; want the raw line preserved", got)
	}
	if got[0].Timestamp != "" {
		t.Errorf("timestamp = %q; want empty for an unstamped line", got[0].Timestamp)
	}
}

// TestCollectMergesPodsOldestFirst proves entries from several pods interleave
// by timestamp rather than staying grouped by pod.
func TestCollectMergesPodsOldestFirst(t *testing.T) {
	in, _ := instance(t, map[string]string{
		"db-1-0": stamped("2026-08-07T10:00:03Z", "third"),
		"db-1-1": stamped("2026-08-07T10:00:01Z", "first") + stamped("2026-08-07T10:00:02Z", "second"),
	}, "db-1-0", "db-1-1")

	got, err := Collect(context.Background(), in, Query{})
	if err != nil {
		t.Fatalf("Collect: %v", err)
	}
	if want := []string{"first", "second", "third"}; !equal(messages(got), want) {
		t.Errorf("messages = %v; want %v", messages(got), want)
	}
}

// TestCollectSkipsUnreadablePod pins the reaped-pod tolerance: one pod failing
// to stream must not fail the whole read.
func TestCollectSkipsUnreadablePod(t *testing.T) {
	in, _ := instance(t, map[string]string{
		"db-1-1": stamped("2026-08-07T10:00:01Z", "survivor"),
	}, "db-1-0", "db-1-1") // db-1-0 has no stream => read error

	got, err := Collect(context.Background(), in, Query{})
	if err != nil {
		t.Fatalf("a reaped pod must not fail the read: %v", err)
	}
	if want := []string{"survivor"}; !equal(messages(got), want) {
		t.Errorf("messages = %v; want %v", messages(got), want)
	}
}

// TestCollectInstanceFilter restricts the read to named pods — and must not
// even open a stream for the others.
func TestCollectInstanceFilter(t *testing.T) {
	in, _ := instance(t, map[string]string{
		"db-1-0": stamped("2026-08-07T10:00:01Z", "primary"),
		"db-1-1": stamped("2026-08-07T10:00:02Z", "replica"),
	}, "db-1-0", "db-1-1")

	got, err := Collect(context.Background(), in, Query{Instance: []string{"db-1-1"}})
	if err != nil {
		t.Fatalf("Collect: %v", err)
	}
	if want := []string{"replica"}; !equal(messages(got), want) {
		t.Errorf("messages = %v; want %v", messages(got), want)
	}
}

// TestCollectSearchIsCaseInsensitive covers the text filter.
func TestCollectSearchIsCaseInsensitive(t *testing.T) {
	in, _ := instance(t, map[string]string{
		"db-1-0": stamped("2026-08-07T10:00:01Z", "FATAL: out of memory") +
			stamped("2026-08-07T10:00:02Z", "checkpoint complete"),
	}, "db-1-0")

	got, err := Collect(context.Background(), in, Query{Search: "fatal"})
	if err != nil {
		t.Fatalf("Collect: %v", err)
	}
	if want := []string{"FATAL: out of memory"}; !equal(messages(got), want) {
		t.Errorf("messages = %v; want %v", messages(got), want)
	}
}

// TestCollectTimeWindow covers the Since/End filter, which neither feature's
// own suite exercised.
func TestCollectTimeWindow(t *testing.T) {
	in, _ := instance(t, map[string]string{
		"db-1-0": stamped("2026-08-07T10:00:00Z", "before") +
			stamped("2026-08-07T10:00:05Z", "inside") +
			stamped("2026-08-07T10:00:10Z", "after"),
	}, "db-1-0")
	since := time.Date(2026, 8, 7, 10, 0, 1, 0, time.UTC)
	end := time.Date(2026, 8, 7, 10, 0, 9, 0, time.UTC)

	for _, tc := range []struct {
		name  string
		query Query
		want  []string
	}{
		{"both bounds", Query{Since: since, End: end}, []string{"inside"}},
		{"since only", Query{Since: since}, []string{"inside", "after"}},
		{"end only", Query{End: end}, []string{"before", "inside"}},
		{"unbounded", Query{}, []string{"before", "inside", "after"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := Collect(context.Background(), in, tc.query)
			if err != nil {
				t.Fatalf("Collect: %v", err)
			}
			if !equal(messages(got), tc.want) {
				t.Errorf("messages = %v; want %v", messages(got), tc.want)
			}
		})
	}
}

// TestCollectBoundsAreInclusive pins that an entry exactly on Since or End is
// kept — an off-by-one here silently hides a boundary line.
func TestCollectBoundsAreInclusive(t *testing.T) {
	in, _ := instance(t, map[string]string{
		"db-1-0": stamped("2026-08-07T10:00:00Z", "at-since") + stamped("2026-08-07T10:00:10Z", "at-end"),
	}, "db-1-0")

	got, err := Collect(context.Background(), in, Query{
		Since: time.Date(2026, 8, 7, 10, 0, 0, 0, time.UTC),
		End:   time.Date(2026, 8, 7, 10, 0, 10, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("Collect: %v", err)
	}
	if want := []string{"at-since", "at-end"}; !equal(messages(got), want) {
		t.Errorf("messages = %v; want both boundary entries kept", messages(got))
	}
}

// TestCollectLimitDirection pins which end of the merged stream survives the
// cap: forward keeps the oldest, the default keeps the most recent.
func TestCollectLimitDirection(t *testing.T) {
	var raw strings.Builder
	for i := range 5 {
		raw.WriteString(stamped(fmt.Sprintf("2026-08-07T10:00:0%dZ", i), fmt.Sprintf("line-%d", i)))
	}
	in, _ := instance(t, map[string]string{"db-1-0": raw.String()}, "db-1-0")

	forward, err := Collect(context.Background(), in, Query{Limit: 2, Direction: core.DirectionForward})
	if err != nil {
		t.Fatalf("Collect forward: %v", err)
	}
	if want := []string{"line-0", "line-1"}; !equal(messages(forward), want) {
		t.Errorf("forward = %v; want the oldest %v", messages(forward), want)
	}

	backward, err := Collect(context.Background(), in, Query{Limit: 2})
	if err != nil {
		t.Fatalf("Collect backward: %v", err)
	}
	if want := []string{"line-3", "line-4"}; !equal(messages(backward), want) {
		t.Errorf("backward = %v; want the newest %v", messages(backward), want)
	}
}

// TestCollectLimitClamping pins the default and ceiling, and that the clamped
// value is what bounds the per-pod read.
func TestCollectLimitClamping(t *testing.T) {
	var tails []int64
	in := Instance{
		Namespace: "bex-apps",
		Name:      "kv-1",
		Kind:      KindKeyValue,
		Container: core.ValkeyContainer,
		Pods:      []string{"kv-1-0"},
		PodLogs: func(_ context.Context, _, _, _ string, tail int64) (io.ReadCloser, error) {
			tails = append(tails, tail)
			return io.NopCloser(strings.NewReader("")), nil
		},
	}

	for _, tc := range []struct {
		name     string
		limit    int64
		wantTail int64
	}{
		{"unset defaults", 0, DefaultLimit},
		{"negative defaults", -5, DefaultLimit},
		{"over ceiling clamps", MaxLimit + 500, MaxLimit},
		{"in range passes through", 7, 7},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tails = nil
			if _, err := Collect(context.Background(), in, Query{Limit: tc.limit}); err != nil {
				t.Fatalf("Collect: %v", err)
			}
			if len(tails) != 1 || tails[0] != tc.wantTail {
				t.Errorf("pod read tail = %v; want %d", tails, tc.wantTail)
			}
		})
	}
}

// TestCollectNoPods returns nothing rather than erroring — an instance that is
// scaled to zero or still provisioning simply has no lines yet.
func TestCollectNoPods(t *testing.T) {
	in, _ := instance(t, map[string]string{})
	got, err := Collect(context.Background(), in, Query{})
	if err != nil {
		t.Fatalf("Collect: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("got %d entries; want none", len(got))
	}
}

// TestKindsMatchTheGenericLogsVocabulary pins the two type labels, which the
// generic logs feature stamps onto the same resources through its own path.
func TestKindsMatchTheGenericLogsVocabulary(t *testing.T) {
	if KindPostgres != "postgres" || KindKeyValue != "keyvalue" {
		t.Errorf("kinds = %q/%q; want postgres/keyvalue", KindPostgres, KindKeyValue)
	}
}

func equal(got, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}
