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
	"os/exec"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/bex-co/bex/lego/types/tiers"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// w6/m458: every plan's PgBouncer runs Render's settings
// (render.com/docs/postgresql-connection-pooling, read 2026-11-17), not
// PgBouncer's defaults of 100 clients and a pool of 20.
func TestEveryPlansPoolerRunsRendersPgBouncerSettings(t *testing.T) {
	for _, id := range tiers.Postgres.IDs() {
		plan, _ := tiers.Postgres.ByID(id)
		pool := strconv.Itoa(int(plan.MaxConnections) - 10)
		want := map[string]any{
			"max_client_conn":     "30000",
			"default_pool_size":   pool,
			"max_db_connections":  pool,
			"client_idle_timeout": "86400",
			"server_login_retry":  "2",
		}
		// The primary's pooler and each replica's own (w6/m467).
		for _, cluster := range []string{"db", appv1alpha1.ReadReplicaClusterName("db", "a")} {
			got, _, _ := unstructured.NestedMap(poolerSpec(cluster, plan), "pgbouncer", "parameters")
			if !reflect.DeepEqual(got, want) {
				t.Errorf("plan %q pooler of %s parameters = %v, want %v", id, cluster, got, want)
			}
		}
	}
	// The pool follows the plan's own figure: a Render 2c-8g database
	// (max_connections 200, w6/m332) pools 190.
	got, _, _ := unstructured.NestedString(poolerSpec("db", tiers.PostgresTier{MaxConnections: 200}), "pgbouncer", "parameters", "default_pool_size")
	if got != "190" {
		t.Errorf("a 200-connection plan pools %q, want 190", got)
	}
}

// w6/m458: PgBouncer starts with its open-file soft limit at the container's
// hard limit. containerd 2 starts containers at a soft limit of 1,024, and
// PgBouncer, which needs a descriptor per client and per server connection,
// never raises it: on production 1,000 clients left it unable to accept or
// to open more than 10 of its 90 server connections.
func TestPgBouncerStartsWithTheHardFileLimit(t *testing.T) {
	want := []any{"/bin/sh", "-c", pgbouncerRunCommand}
	for _, cluster := range []string{"db", appv1alpha1.ReadReplicaClusterName("db", "a")} {
		containers, _, _ := unstructured.NestedSlice(poolerSpec(cluster, tiers.PostgresTier{MaxConnections: 100}), "template", "spec", "containers")
		if len(containers) != 1 {
			t.Fatalf("pooler of %s template containers = %v, want only pgbouncer", cluster, containers)
		}
		c, _ := containers[0].(map[string]any)
		if c["name"] != "pgbouncer" || !reflect.DeepEqual(c["command"], want) {
			t.Errorf("pooler of %s pgbouncer container = %v, want command %v", cluster, c, want)
		}
	}

	// Run the command as the container would, with a stand-in for CNPG's
	// manager that reports the limit it inherits and the arguments it gets,
	// under a soft limit of 256 and a hard limit of 4,096.
	const manager = "/controller/manager pgbouncer run"
	if !strings.HasSuffix(pgbouncerRunCommand, "exec "+manager) {
		t.Fatalf("the command %q does not end by exec'ing CNPG's %q", pgbouncerRunCommand, manager)
	}
	stub := filepath.Join(t.TempDir(), "manager")
	if err := os.WriteFile(stub, []byte("#!/bin/sh\necho \"$(ulimit -n) $*\"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	command := strings.Replace(pgbouncerRunCommand, "/controller/manager", stub, 1)
	out, err := exec.Command("/bin/sh", "-c", `ulimit -S -n 256 && ulimit -H -n 4096 && exec /bin/sh -c "$0"`, command).CombinedOutput()
	if err != nil {
		t.Fatalf("the command failed: %v: %s", err, out)
	}
	if got := strings.TrimSpace(string(out)); got != "4096 pgbouncer run" {
		t.Errorf("CNPG's manager started with %q, want the hard limit 4096 and its pgbouncer run arguments", got)
	}
}
