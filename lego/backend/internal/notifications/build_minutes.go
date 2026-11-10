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

	"github.com/bex-co/bex/lego/backend/internal/buildminutes"
	"github.com/bex-co/bex/lego/backend/internal/email"
)

// BuildMinutesNotifier mails a workspace's billing owners about its plan's
// included pipeline minutes (w6/m308): a warning near them and a notice when
// they are spent. The sweeper claims each stage before calling it, so it sends
// once per month and never retries.
type BuildMinutesNotifier struct{ Service *Service }

// NotifyBuildMinutes satisfies buildminutes.Notifier.
func (n BuildMinutesNotifier) NotifyBuildMinutes(ctx context.Context, workspaceID, stage string, usedMinutes, includedMinutes float64, canPay bool) error {
	return n.Service.mailBillingOwners(ctx, workspaceID, fmt.Sprintf("build minutes %s for %s", stage, workspaceID), func() (string, email.Message, error) {
		return buildMinutesEmail(stage, usedMinutes, includedMinutes, canPay, n.Service.DashboardBaseURL)
	})
}

func buildMinutesEmail(stage string, usedMinutes, includedMinutes float64, canPay bool, dashboardBaseURL string) (string, email.Message, error) {
	var title, paragraph string
	switch stage {
	case buildminutes.StageWarned:
		title = "Your workspace is nearly out of included build minutes"
		paragraph = fmt.Sprintf("Your workspace has used %.0f of the %.0f build minutes its plan includes this month.", usedMinutes, includedMinutes)
		if canPay {
			paragraph += " Build minutes beyond them are billed per minute."
		} else {
			paragraph += " When they run out, new builds and pre-deploy commands stop until the start of next month. Add a payment method to keep building."
		}
	case buildminutes.StageExhausted:
		title = "Your workspace has used its included build minutes"
		if canPay {
			paragraph = fmt.Sprintf("Your workspace has used the %.0f build minutes its plan includes this month. Further build minutes are billed per minute.", includedMinutes)
		} else {
			paragraph = fmt.Sprintf("Your workspace has used the %.0f build minutes its plan includes this month, so new builds and pre-deploy commands stop until the start of next month. Add a payment method to keep building.", includedMinutes)
		}
	default:
		return "", email.Message{}, fmt.Errorf("unknown build minutes stage %q", stage)
	}
	msg := email.Message{Title: title, Paragraphs: []string{paragraph}}
	if dashboardBaseURL != "" {
		msg.CTA = &email.CTA{Lead: "See your monthly included usage", Label: "Open billing", URL: dashboardBaseURL + "/billing"}
	}
	return "[bex] " + title, msg, nil
}
