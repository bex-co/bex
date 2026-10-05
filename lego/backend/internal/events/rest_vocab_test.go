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

package events

import (
	"encoding/json"
	"maps"
	"os"
	"slices"
	"testing"
)

// restEmittedTypes is every type the REST event shape can carry: both deploy
// phases, every audit verb's type (plus the auto-deploy sub-types view derives
// from apps.SetAutoDeploy), every service fact, and the datastore types
// GET /events/{id} resolves.
func restEmittedTypes() []string {
	types := []string{TypeDeployStarted, TypeDeployEnded, TypeAutoDeployEnabled, TypeAutoDeployDisabled}
	types = append(types, slices.Collect(maps.Values(eventTypes))...)
	types = append(types, slices.Collect(maps.Values(indexedAuditEventTypes))...)
	types = append(types, allFactTypes...)
	types = append(types,
		TypePostgresUnavailable, TypePostgresAvailable, TypeKeyValueUnhealthy, TypeKeyValueAvailable,
		TypePostgresBackupCompleted, TypePostgresBackupFailed, TypePostgresRestoreSucceeded,
		TypePostgresRestoreFailed, TypePostgresUpgradeStarted, TypePostgresUpgradeSucceeded,
		TypePostgresUpgradeFailed,
	)
	slices.Sort(types)
	return slices.Compact(types)
}

// TestRESTEventTypesAreRenderOrDeclaredExtensions is w8/060's gate. Render's
// list-events `type` is a closed enum; bex emits Render's spelling wherever a
// Render type names the fact, and keeps its own snake_case name where none
// does (ADR018 Service events row: deliberate REST drift for bex-named types).
// That drift must stay a decision: a new type outside the pinned enum fails
// here until it is listed below with the reason Render cannot name it.
func TestRESTEventTypesAreRenderOrDeclaredExtensions(t *testing.T) {
	const config = "bex-named configuration write; Render records settings edits without a typed event"
	const datastore = "datastore lifecycle in Render's webhook vocabulary; the service-event enum has no datastore member (reachable only by evt- id)"
	extensions := map[string]string{
		TypeEnvVarsChanged:                       "env-var write; Render rolls it as a deploy and has no env event",
		TypeServiceEnvironmentChanged:            "environment (env vars + secret files) save; same as env_vars_changed",
		TypeServiceMoved:                         "project/environment reassignment (w6/m134); Render has no membership-move type",
		TypeEnvGroupLinked:                       "env-group link; Render has no env-group event",
		TypeEnvGroupUnlinked:                     "env-group unlink; Render has no env-group event",
		TypeAutoDeployChanged:                    "legacy auto-deploy audit rows with no recorded value (new rows map to auto_deploy_enabled/disabled)",
		TypeIdleTimeoutChanged:                   config,
		TypeRootDirectoryChanged:                 config,
		TypeDockerfilePathChanged:                config,
		TypePortChanged:                          config,
		TypeBuildFilterChanged:                   config,
		TypeCommandsChanged:                      config,
		TypeSourceChanged:                        config,
		TypeDisplayNameChanged:                   config,
		TypePreDeployChanged:                     config,
		TypeMaxShutdownDelayChanged:              config,
		TypePublishPathChanged:                   config,
		TypeRoutesChanged:                        config,
		TypeHeadersChanged:                       config,
		TypeNotifyOnFailChanged:                  config,
		TypeSubdomainPolicyChanged:               config,
		TypeIPAllowListChanged:                   config,
		TypeDiskRestored:                         "restore from snapshot; Render has no restore event (labeled extension, w8/m34)",
		TypeCustomDomainAdded:                    "custom-domain write; Render has no custom-domain service event",
		TypeCustomDomainRemoved:                  "custom-domain write; Render has no custom-domain service event",
		TypeCustomDomainVerified:                 "custom-domain verification; Render has no custom-domain service event",
		TypeDeployHookRegenerated:                "deploy-hook rotation; Render has no hook event",
		TypeJobStarted:                           "one-off job created; Render's enum has only job_run_ended",
		TypeJobCanceled:                          "one-off job canceled; Render's enum has only job_run_ended",
		TypeBranchChanged:                        "observed branch switch (w3/m19 bex extension); Render has only branch_deleted",
		TypeServiceHibernated:                    "free-tier idle sleep (w6/m47); Render's spin-down records no event, and service_suspended would claim a person suspended it",
		TypeServiceWoken:                         "free-tier wake on traffic (w6/m47); service_resumed would claim a person resumed it",
		TypePostgresCreated:                      datastore,
		TypePostgresRestarted:                    datastore,
		TypePostgresCredentialsCreated:           datastore,
		TypePostgresCredentialsDeleted:           datastore,
		TypePostgresBackupStarted:                datastore,
		TypePostgresBackupCompleted:              datastore,
		TypePostgresBackupFailed:                 datastore,
		TypePostgresRestoreSucceeded:             datastore,
		TypePostgresRestoreFailed:                datastore,
		TypePostgresUpgradeStarted:               datastore,
		TypePostgresUpgradeSucceeded:             datastore,
		TypePostgresUpgradeFailed:                datastore,
		TypePostgresUnavailable:                  datastore,
		TypePostgresAvailable:                    datastore,
		TypeKeyValueUnhealthy:                    datastore,
		TypeKeyValueAvailable:                    datastore,
		TypePostgresHAStatusChanged:              datastore,
		TypePostgresConnectionPoolEnabledChanged: datastore,
		TypePostgresDiskSizeChanged:              datastore,
		TypeKeyValueConfigRestart:                datastore,
	}

	raw, err := os.ReadFile("../api/openapi/render-public-api-1.json")
	if err != nil {
		t.Fatal(err)
	}
	var spec struct {
		Components struct {
			Parameters struct {
				EventType struct {
					Schema struct {
						AnyOf []struct {
							Enum []string `json:"enum"`
						} `json:"anyOf"`
					} `json:"schema"`
				} `json:"eventTypeParam"`
			} `json:"parameters"`
		} `json:"components"`
	}
	if err := json.Unmarshal(raw, &spec); err != nil {
		t.Fatal(err)
	}
	if len(spec.Components.Parameters.EventType.Schema.AnyOf) == 0 || len(spec.Components.Parameters.EventType.Schema.AnyOf[0].Enum) == 0 {
		t.Fatal("pinned spec has no eventTypeParam enum")
	}
	render := spec.Components.Parameters.EventType.Schema.AnyOf[0].Enum

	emitted := restEmittedTypes()
	for _, eventType := range emitted {
		inRender := slices.Contains(render, eventType)
		_, declared := extensions[eventType]
		switch {
		case !inRender && !declared:
			t.Errorf("%q is emitted on REST but is neither in Render's event-type enum nor a declared extension", eventType)
		case inRender && declared:
			t.Errorf("%q is in Render's enum; drop it from the extension list", eventType)
		}
	}
	for eventType := range extensions {
		if !slices.Contains(emitted, eventType) {
			t.Errorf("declared extension %q is no longer emitted; drop it", eventType)
		}
	}
}
