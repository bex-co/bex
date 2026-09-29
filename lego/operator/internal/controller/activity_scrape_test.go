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

package controller

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// The activity query is only as good as what Prometheus keeps. w1/m161 taught
// the plugin to export bex_websocket_ingress_bytes_total and the query to read
// it, but the traefik-websocket-meter scrape job's metric_relabel keep-list
// still named only the egress series, so Prometheus dropped the new counter
// and a client-only WebSocket service hibernated on schedule in production
// (2026-09-29). Every bex_websocket_* series the query reads must survive that
// job's keep regex, anchored the way Prometheus anchors it.
func TestActivityQueryWebSocketSeriesSurviveTheScrapeKeepList(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("..", "..", "..", "..", "deploy", "gitops", "base", "prometheus.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	cfg := string(raw)
	job := strings.Index(cfg, "job_name: traefik-websocket-meter")
	if job < 0 {
		t.Fatal("prometheus.yaml has no traefik-websocket-meter scrape job")
	}
	keep := regexp.MustCompile(`(?s)metric_relabel_configs:.*?regex: (\S+)\s+action: keep`).FindStringSubmatch(cfg[job:])
	if keep == nil {
		t.Fatal("traefik-websocket-meter has no metric_relabel keep regex")
	}
	kept := regexp.MustCompile("^(?:" + keep[1] + ")$")

	app := &appv1alpha1.App{ObjectMeta: metav1.ObjectMeta{Name: "web", Namespace: "tea-x"}}
	series := regexp.MustCompile(`bex_websocket_[a-z_]+`).FindAllString(activityQuery(app, 15*time.Minute), -1)
	if len(series) < 2 {
		t.Fatalf("expected the query to read both WebSocket directions, found %v", series)
	}
	for _, name := range series {
		if !kept.MatchString(name) {
			t.Errorf("the activity query reads %s, but the scrape job's keep regex %q drops it", name, keep[1])
		}
	}
}
