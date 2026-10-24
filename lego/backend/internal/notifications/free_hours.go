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

	"github.com/bex-co/bex/lego/backend/internal/email"
)

// FreeHoursNotifier mails a workspace's owners about its monthly Free instance
// hours (w8/m139): a warning near the budget and a notice when it is spent and
// the Free web services are suspended. The sweeper claims each stage before
// calling it, so it sends once per month and never retries.
type FreeHoursNotifier struct{ Service *Service }

// NotifyFreeHours satisfies freehours.Notifier.
func (n FreeHoursNotifier) NotifyFreeHours(ctx context.Context, workspaceID, stage string, usedHours, budgetHours float64) error {
	s := n.Service
	if s == nil {
		return fmt.Errorf("free hours notifier unavailable")
	}
	owners, ok := s.Store.(billingOwnerStore)
	if !ok || s.Mailer == nil || s.Identities == nil {
		log.Printf("notifications: free instance hours %s for %s not emailed (store, SMTP or identity lookup unavailable)", stage, workspaceID)
		return nil
	}
	subjects, err := owners.ListBillingOwnerSubjects(ctx, workspaceID)
	if err != nil {
		return err
	}
	if len(subjects) == 0 {
		return nil
	}
	subject, msg, err := freeHoursEmail(stage, usedHours, budgetHours, s.DashboardBaseURL)
	if err != nil {
		return err
	}
	return s.mailOwners(ctx, fmt.Sprintf("free instance hours %s for %s", stage, workspaceID), subjects, subject, msg, nil)
}

func freeHoursEmail(stage string, usedHours, budgetHours float64, dashboardBaseURL string) (string, email.Message, error) {
	var title, paragraph string
	switch stage {
	case "warned":
		title = "Your workspace is nearly out of Free instance hours"
		paragraph = fmt.Sprintf("Your workspace has used %.0f of its %.0f Free instance hours this month. When they run out, your Free web services are suspended until the start of next month. Upgrade a service to a paid plan to keep it running.", usedHours, budgetHours)
	case "exhausted":
		title = "Your workspace has used all its Free instance hours"
		paragraph = fmt.Sprintf("Your workspace has used its %.0f Free instance hours for this month, so its Free web services are suspended until the start of next month. Upgrade a service to a paid plan to run it now.", budgetHours)
	default:
		return "", email.Message{}, fmt.Errorf("unknown free instance hours stage %q", stage)
	}
	msg := email.Message{Title: title, Paragraphs: []string{paragraph}}
	if dashboardBaseURL != "" {
		msg.CTA = &email.CTA{Lead: "See your monthly included usage", Label: "Open usage", URL: dashboardBaseURL + "/usage"}
	}
	return "[bex] " + title, msg, nil
}
