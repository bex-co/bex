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

	"github.com/bex-co/bex/lego/backend/internal/bandwidth"
	"github.com/bex-co/bex/lego/backend/internal/email"
)

// BandwidthNotifier mails a workspace's billing owners about its plan's
// included outbound bandwidth (w6/m319): a warning near it and a notice when it
// is spent. The sweeper claims each stage before calling it, so it sends once
// per month and never retries.
type BandwidthNotifier struct{ Service *Service }

// NotifyBandwidth satisfies bandwidth.Notifier.
func (n BandwidthNotifier) NotifyBandwidth(ctx context.Context, workspaceID, stage string, usedGB, includedGB float64, canPay bool) error {
	return n.Service.mailBillingOwners(ctx, workspaceID, fmt.Sprintf("included bandwidth %s for %s", stage, workspaceID), func() (string, email.Message, error) {
		return bandwidthEmail(stage, usedGB, includedGB, canPay, n.Service.DashboardBaseURL)
	})
}

func bandwidthEmail(stage string, usedGB, includedGB float64, canPay bool, dashboardBaseURL string) (string, email.Message, error) {
	var title, paragraph string
	switch stage {
	case bandwidth.StageWarned:
		title = "Your workspace is nearly out of included bandwidth"
		paragraph = fmt.Sprintf("Your workspace has used %.1f GB of the %.0f GB of outbound bandwidth its plan includes this month.", usedGB, includedGB)
		if canPay {
			paragraph += " Bandwidth beyond it is billed per GB."
		} else {
			paragraph += " When it runs out, your workspace's services spin down until the start of next month. Add a payment method to keep them running."
		}
	case bandwidth.StageExhausted:
		title = "Your workspace has used its included bandwidth"
		if canPay {
			paragraph = fmt.Sprintf("Your workspace has used the %.0f GB of outbound bandwidth its plan includes this month. Further bandwidth is billed per GB.", includedGB)
		} else {
			paragraph = fmt.Sprintf("Your workspace has used the %.0f GB of outbound bandwidth its plan includes this month, so its services are spun down until the start of next month. Add a payment method to run them now.", includedGB)
		}
	default:
		return "", email.Message{}, fmt.Errorf("unknown bandwidth stage %q", stage)
	}
	msg := email.Message{Title: title, Paragraphs: []string{paragraph}}
	if dashboardBaseURL != "" {
		msg.CTA = &email.CTA{Lead: "See your monthly included usage", Label: "Open billing", URL: dashboardBaseURL + "/billing"}
	}
	return "[bex] " + title, msg, nil
}
