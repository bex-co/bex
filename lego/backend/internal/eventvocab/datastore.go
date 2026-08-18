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

// Package eventvocab owns Render event names shared by independent event
// projections. Keeping datastore audit mappings here prevents a webhook from
// advertising an event that retrieve-by-ID cannot translate.
package eventvocab

import "github.com/bex-co/bex/lego/backend/internal/core"

const (
	TypePlanChanged                = "plan_changed"
	TypePostgresCreated            = "postgres_created"
	TypePostgresRestarted          = "postgres_restarted"
	TypePostgresCredentialsCreated = "postgres_credentials_created"
	TypePostgresCredentialsDeleted = "postgres_credentials_deleted"
	TypePostgresBackupStarted      = "postgres_backup_started"
)

// DatastoreAuditTypes returns the closed mapping from source audit verbs to
// webhook/retrieval event names. Callers receive their own map so package-local
// additions cannot mutate the shared vocabulary.
func DatastoreAuditTypes() map[string]string {
	return map[string]string{
		core.AuditVerbPostgresCreated:            TypePostgresCreated,
		core.AuditVerbPostgresRestarted:          TypePostgresRestarted,
		core.AuditVerbPostgresCredentialsCreated: TypePostgresCredentialsCreated,
		core.AuditVerbPostgresCredentialsDeleted: TypePostgresCredentialsDeleted,
		core.AuditVerbPostgresBackupStarted:      TypePostgresBackupStarted,
		core.AuditVerbPostgresPlanChanged:        TypePlanChanged,
		core.AuditVerbKeyValuePlanChanged:        TypePlanChanged,
	}
}
