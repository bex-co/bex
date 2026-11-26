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
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"
	"time"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// w3/m656: the almost-full warning reads a database's current used bytes as
// its fullest instance volume, each volume at its newest sample.

func volumeSeries(pvc string, now time.Time, values ...float64) MetricSeries {
	s := MetricSeries{Labels: map[string]string{"instance": pvc}}
	for i, v := range values {
		at := now.Add(time.Duration(i-len(values)+1) * time.Minute)
		s.Points = append(s.Points, MetricPoint{Timestamp: at.Format(time.RFC3339), Value: v})
	}
	return s
}

func TestCurrentDiskUsedIsTheFullestVolumeAtItsNewestSample(t *testing.T) {
	now := time.Date(2026, 11, 25, 12, 0, 0, 0, time.UTC)
	for _, tc := range []struct {
		name   string
		series []MetricSeries
		want   float64
		ok     bool
	}{
		{"single instance", []MetricSeries{volumeSeries("db-1", now, 10, 30, 20)}, 20, true},
		// The HA standby holds a full copy; its own newest sample counts, not
		// the primary's older peak.
		{"HA pair", []MetricSeries{
			volumeSeries("db-1", now, 900, 100),
			volumeSeries("db-2", now, 500),
		}, 500, true},
		{"read replicas", []MetricSeries{
			volumeSeries("db-1", now, 100),
			volumeSeries("db-r-ca978112ca-1", now, 700),
			volumeSeries("db-r-3e23e81600-1", now, 300),
		}, 700, true},
		{"no sample", nil, 0, false},
		{"only malformed points", []MetricSeries{{Points: []MetricPoint{{Timestamp: "soon", Value: 9}}}}, 0, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var asked DiskUsageRequest
			src := func(_ context.Context, req DiskUsageRequest) ([]MetricSeries, error) {
				asked = req
				return tc.series, nil
			}
			got, ok, err := CurrentDiskUsed(context.Background(), src, DiskUsageRequest{Namespace: "ns", Resource: "db", PVCPattern: "^db-1$"}, now)
			if err != nil || got != tc.want || ok != tc.ok {
				t.Fatalf("CurrentDiskUsed = %v, %v, %v; want %v, %v", got, ok, err, tc.want, tc.ok)
			}
			if !asked.Start.Equal(now.Add(-10*time.Minute)) || !asked.End.Equal(now) || asked.Resolution != time.Minute || asked.Namespace != "ns" || asked.PVCPattern != "^db-1$" {
				t.Errorf("asked %+v, want the ten minutes before now at one-minute steps", asked)
			}
		})
	}
}

func TestCurrentDiskUsedPassesTheSourceErrorOn(t *testing.T) {
	boom := errors.New("prometheus: status 503")
	src := func(context.Context, DiskUsageRequest) ([]MetricSeries, error) { return nil, boom }
	if _, ok, err := CurrentDiskUsed(context.Background(), src, DiskUsageRequest{}, time.Now()); !errors.Is(err, boom) || ok {
		t.Fatalf("CurrentDiskUsed = %v, %v; want the source's error", ok, err)
	}
}

// The database read asks Prometheus for every instance volume of the
// database, its read replicas' included, and none of a database whose name
// merely starts the same.
func TestDatabaseStorageUsedReadsEveryInstanceVolume(t *testing.T) {
	replica := appv1alpha1.ReadReplicaClusterName("dpg-a", "east") + "-1"
	var query string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		query = r.URL.Query().Get("query")
		end := r.URL.Query().Get("end")
		_, _ = fmt.Fprintf(w, `{"status":"success","data":{"resultType":"matrix","result":[
			{"metric":{"persistentvolumeclaim":"dpg-a-1"},"values":[[%[1]s,"100"]]},
			{"metric":{"persistentvolumeclaim":"dpg-a-2"},"values":[[%[1]s,"250"]]},
			{"metric":{"persistentvolumeclaim":%[2]q},"values":[[%[1]s,"400"]]}]}}`, end, replica)
	}))
	defer ts.Close()

	used, ok, err := DatabaseStorageUsed(NewPrometheusDiskUsageSource(ts.URL, ts.Client()))(context.Background(), "tea-a", "dpg-a")
	if err != nil || !ok || used != 400 {
		t.Fatalf("DatabaseStorageUsed = %v, %v, %v; want the replica's 400", used, ok, err)
	}
	if !strings.Contains(query, `namespace="tea-a"`) {
		t.Errorf("query %q is not scoped to the database's namespace", query)
	}
	pattern := regexp.MustCompile(pvcPattern(DatastoreDatabase, "dpg-a"))
	for pvc, want := range map[string]bool{"dpg-a-1": true, "dpg-a-2": true, replica: true, "dpg-ab-1": false, "dpg-a-extra-1": false} {
		if pattern.MatchString(pvc) != want {
			t.Errorf("PVC %q matched = %v, want %v", pvc, !want, want)
		}
	}
}
