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

package metrics

import (
	"errors"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/core"
	ids "github.com/bex-co/bex/lego/backend/internal/id"
)

func TestAggregateReplicasAtTimestampsMinMaxAvg(t *testing.T) {
	app := "srv-a"
	a := ids.ServiceInstanceID(app, "pod-a")
	b := ids.ServiceInstanceID(app, "pod-b")
	ts := "2026-09-08T12:00:00Z"
	series := []MetricSeries{
		{Labels: map[string]string{"resource": app, "instance": a}, Unit: "cores", Points: []MetricPoint{{Timestamp: ts, Value: 10}}},
		{Labels: map[string]string{"resource": app, "instance": b}, Unit: "cores", Points: []MetricPoint{{Timestamp: ts, Value: 30}}},
	}
	for method, want := range map[string]float64{
		replicaAggregateMin: 10,
		replicaAggregateMax: 30,
		replicaAggregateAvg: 20,
	} {
		out := aggregateReplicasAtTimestamps(app, method, series)
		if len(out) != 1 || len(out[0].Points) != 1 {
			t.Fatalf("%s: %+v", method, out)
		}
		if out[0].Points[0].Value != want {
			t.Fatalf("%s = %v, want %v", method, out[0].Points[0].Value, want)
		}
		if out[0].Labels["aggregate"] != method {
			t.Fatalf("%s label = %q", method, out[0].Labels["aggregate"])
		}
	}
}

func TestAggregateReplicasDoesNotFillGaps(t *testing.T) {
	app := "srv-a"
	a := ids.ServiceInstanceID(app, "pod-a")
	b := ids.ServiceInstanceID(app, "pod-b")
	series := []MetricSeries{
		{Labels: map[string]string{"instance": a}, Unit: "cores", Points: []MetricPoint{
			{Timestamp: "t1", Value: 10},
			{Timestamp: "t2", Value: 12},
		}},
		{Labels: map[string]string{"instance": b}, Unit: "cores", Points: []MetricPoint{
			{Timestamp: "t1", Value: 30},
			// t2 absent — must not become 0 or borrow from t1
		}},
	}
	out := aggregateReplicasAtTimestamps(app, replicaAggregateAvg, series)
	if len(out) != 1 || len(out[0].Points) != 2 {
		t.Fatalf("points = %+v", out)
	}
	byTS := map[string]float64{}
	for _, p := range out[0].Points {
		byTS[p.Timestamp] = p.Value
	}
	if byTS["t1"] != 20 {
		t.Fatalf("t1 avg = %v, want 20", byTS["t1"])
	}
	if byTS["t2"] != 12 {
		t.Fatalf("t2 avg = %v, want 12 (only pod-a present)", byTS["t2"])
	}
}

func TestFilterSeriesByInstancesSelectsOne(t *testing.T) {
	app := "srv-a"
	a := ids.ServiceInstanceID(app, "pod-a")
	b := ids.ServiceInstanceID(app, "pod-b")
	series := []MetricSeries{
		{Labels: map[string]string{"instance": a}, Points: []MetricPoint{{Value: 10}}},
		{Labels: map[string]string{"instance": b}, Points: []MetricPoint{{Value: 30}}},
	}
	got := filterSeriesByInstances(app, []string{b}, series, nil)
	if len(got) != 1 || got[0].Labels["instance"] != b || got[0].Points[0].Value != 30 {
		t.Fatalf("got %+v", got)
	}
}

func TestFilterSeriesUnknownDoesNotBroaden(t *testing.T) {
	app := "srv-a"
	a := ids.ServiceInstanceID(app, "pod-a")
	series := []MetricSeries{{Labels: map[string]string{"instance": a}, Points: []MetricPoint{{Value: 10}}}}
	got := filterSeriesByInstances(app, []string{"srv-a-zzzzzzzzzzzzzzzzzzzz"}, series, nil)
	if len(got) != 0 {
		t.Fatalf("unknown selector broadened to %+v", got)
	}
}

func TestValidateInstanceSelection(t *testing.T) {
	if err := validateInstanceSelection(nil); err != nil {
		t.Fatal(err)
	}
	if err := validateInstanceSelection([]string{""}); !errors.Is(err, core.ErrBadRequest) {
		t.Fatalf("blank = %v", err)
	}
	tooMany := make([]string, maxSelectedInstances+1)
	for i := range tooMany {
		tooMany[i] = "x"
	}
	if err := validateInstanceSelection(tooMany); !errors.Is(err, core.ErrBadRequest) {
		t.Fatalf("oversized = %v", err)
	}
}

func TestParseReplicaAggregate(t *testing.T) {
	r, max, err := parseReplicaAggregate("MAX")
	if err != nil || r != replicaAggregateMax || !max {
		t.Fatalf("MAX = %q %v %v", r, max, err)
	}
	r, max, err = parseReplicaAggregate("MIN")
	if err != nil || r != replicaAggregateMin || max {
		t.Fatalf("MIN = %q %v %v", r, max, err)
	}
	if _, _, err := parseReplicaAggregate("MEDIAN"); !errors.Is(err, core.ErrBadRequest) {
		t.Fatalf("unknown = %v", err)
	}
}

func TestApplyInstanceSelectionEndToEnd(t *testing.T) {
	app := "srv-a"
	a := ids.ServiceInstanceID(app, "pod-a")
	b := ids.ServiceInstanceID(app, "pod-b")
	ts := "2026-09-08T12:00:00Z"
	series := []MetricSeries{
		{Labels: map[string]string{"resource": app, "instance": a}, Unit: "cores", Points: []MetricPoint{{Timestamp: ts, Value: 10}}},
		{Labels: map[string]string{"resource": app, "instance": b}, Unit: "cores", Points: []MetricPoint{{Timestamp: ts, Value: 30}}},
	}
	out, err := applyInstanceSelection(MetricQuery{
		App: app, Instances: []string{b}, ReplicaAggregate: replicaAggregateMax,
	}, series, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != 1 || out[0].Points[0].Value != 30 {
		t.Fatalf("select b + MAX = %+v", out)
	}
	out, err = applyInstanceSelection(MetricQuery{
		App: app, Instances: []string{a, b}, ReplicaAggregate: replicaAggregateAvg,
	}, series, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != 1 || out[0].Points[0].Value != 20 {
		t.Fatalf("select both + AVG = %+v", out)
	}
}
