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

package notifications

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/email"
	"github.com/bex-co/bex/lego/backend/internal/resourcemeta"
)

// DatabaseExpiryNotifier mails a workspace's owners about a free Postgres
// database's expiry (ADR009 §Free-plan expiry, w8/m126). Like BillingNotifier
// it is an internal worker sink, not a caller verb. The sweeper claims each
// stage before calling it, so it sends once and never retries.
type DatabaseExpiryNotifier struct{ Service *Service }

// NotifyFreeDatabaseExpiry satisfies postgres.FreeExpiryNotifier.
func (n DatabaseExpiryNotifier) NotifyFreeDatabaseExpiry(ctx context.Context, workspaceID, databaseID, name, stage string, expiresAt, deleteAt time.Time) error {
	s := n.Service
	if s == nil {
		return fmt.Errorf("database expiry notifier unavailable")
	}
	owners, ok := s.Store.(billingOwnerStore)
	if !ok || s.Mailer == nil || s.Identities == nil {
		log.Printf("notifications: free database %s %s not emailed (store, SMTP or identity lookup unavailable)", databaseID, stage)
		return nil
	}
	subjects, err := owners.ListBillingOwnerSubjects(ctx, workspaceID)
	if err != nil {
		return err
	}
	if len(subjects) == 0 {
		return nil
	}
	subject, msg, err := databaseExpiryEmail(databaseID, name, stage, expiresAt, deleteAt, s.DashboardBaseURL)
	if err != nil {
		return err
	}
	return s.mailOwners(ctx, fmt.Sprintf("free database %s %s", databaseID, stage), subjects, subject, msg, nil)
}

// databaseExpiryEmail is the message for one stage: warned, expired,
// final-warned or deleted.
func databaseExpiryEmail(databaseID, name, stage string, expiresAt, deleteAt time.Time, dashboardBaseURL string) (string, email.Message, error) {
	expires, deletes := expiresAt.UTC().Format(time.RFC3339), deleteAt.UTC().Format(time.RFC3339)
	var title, paragraph string
	switch stage {
	case "warned":
		title = fmt.Sprintf("Free database %s expires soon", name)
		paragraph = fmt.Sprintf("Your free Postgres database %s (%s) expires on %s. Upgrade it to a paid plan to keep it running.", name, databaseID, expires)
	case "expired":
		title = fmt.Sprintf("Free database %s has expired", name)
		paragraph = fmt.Sprintf("Your free Postgres database %s (%s) expired on %s and is no longer reachable. Upgrade it to a paid plan before %s, or it and its data will be deleted.", name, databaseID, expires, deletes)
	case "final-warned":
		title = fmt.Sprintf("Free database %s will be deleted soon", name)
		paragraph = fmt.Sprintf("Your expired free Postgres database %s (%s) and its data will be deleted on %s. Upgrade it to a paid plan to keep it.", name, databaseID, deletes)
	case "deleted":
		title = fmt.Sprintf("Free database %s was deleted", name)
		paragraph = fmt.Sprintf("Your free Postgres database %s (%s) expired on %s and was not upgraded within %d days, so it and its data were deleted.",
			name, databaseID, expires, int(deleteAt.Sub(expiresAt).Hours()/24))
	default:
		return "", email.Message{}, fmt.Errorf("unknown free database expiry stage %q", stage)
	}
	msg := email.Message{Title: title, Paragraphs: []string{paragraph}}
	if u := (resourcemeta.Config{DashboardBaseURL: dashboardBaseURL}).DashboardURL(resourcemeta.PostgresDashboardRoute, databaseID); u != "" && stage != "deleted" {
		msg.CTA = &email.CTA{Lead: "Upgrade the database to a paid plan", Label: "Open database", URL: u}
	}
	return "[bex] " + title, msg, nil
}
