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

package postgres

import (
	"context"
	"net/url"
	"testing"
)

// Use a connection-local setting: no ALTER SYSTEM/DATABASE affects other test
// packages. Kubernetes remains fake, so this verifies observation, not CNPG
// reconciliation or whether a declared parameter has been applied.
func TestParameterSpecObservationIntegration(t *testing.T) {
	uri, err := url.Parse(testDBURI(t))
	if err != nil {
		t.Fatal("parse test database URI")
	}
	query := uri.Query()
	query.Set("work_mem", "8MB")
	uri.RawQuery = query.Encode()
	ctx := context.Background()
	svc, _ := newService()
	seedDatabaseAt(t, svc, "observed-live", uri.String())
	for _, declared := range []string{"8MB", "16MB", "not-a-size"} {
		t.Run(declared, func(t *testing.T) {
			if _, err := svc.SetParameterOverrides(ctx, "observed-live", map[string]string{"work_mem": declared, "bex_unknown_parameter": "enabled"}); err != nil {
				t.Fatal(err)
			}
			rows, err := svc.ParameterSpec(ctx, "observed-live")
			if err != nil {
				t.Fatal(err)
			}
			if len(rows) != 2 {
				t.Fatalf("declared set expanded with runtime-only rows: %d", len(rows))
			}
			unknown, observed := rows[0], rows[1]
			if unknown.Name != "bex_unknown_parameter" || unknown.Value != "enabled" || unknown.ObservationStatus != "not_observed" || unknown.ObservedSetting != nil || unknown.ObservedUnit != nil {
				t.Fatalf("unknown declaration=%+v", unknown)
			}
			if observed.Name != "work_mem" || observed.Value != declared || observed.ObservationStatus != "observed" || observed.ObservedSetting == nil || *observed.ObservedSetting != "8192" || observed.ObservedUnit == nil || *observed.ObservedUnit != "kB" {
				t.Fatalf("normalized runtime observation=%+v", observed)
			}
		})
	}
}

func TestParameterSpecObservationConnectionFailureIntegration(t *testing.T) {
	uri, err := url.Parse(testDBURI(t))
	if err != nil {
		t.Fatal("parse test database URI")
	}
	query := uri.Query()
	// PostgreSQL itself rejects this startup setting; the connection failure
	// must not erase declared configuration or claim runtime absence.
	query.Set("work_mem", "not-a-size")
	uri.RawQuery = query.Encode()
	svc, _ := newService()
	seedDatabaseAt(t, svc, "unavailable-live", uri.String())
	ctx := context.Background()
	if _, err := svc.SetParameterOverrides(ctx, "unavailable-live", map[string]string{"work_mem": "8MB"}); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.ParameterOverrides(ctx, "unavailable-live"); err == nil {
		t.Fatal("invalid startup setting unexpectedly connected")
	}
	rows, err := svc.ParameterSpec(ctx, "unavailable-live")
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 {
		t.Fatalf("declared rows=%d", len(rows))
	}
	row := rows[0]
	if row.Name != "work_mem" || row.Value != "8MB" || row.ObservationStatus != "unavailable" || row.ObservedSetting != nil || row.ObservedUnit != nil {
		t.Fatalf("unavailable observation=%+v", row)
	}
}
