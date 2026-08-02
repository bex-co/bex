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
	"errors"
	"log"

	"github.com/bex-co/bex/lego/backend/internal/notifications/push"
)

// PushEvidenceLogger emits only bounded worker classifications. The durable
// delivery row remains the detailed audit ledger; this line gives operators a
// searchable terminal/prune transition without any tenant, device, token,
// ticket, provider message, or endpoint value.
type PushEvidenceLogger struct{}

func (PushEvidenceLogger) RecordPushEvidence(_ context.Context, evidence PushEvidence) {
	log.Printf("notifications: push evidence operation=%s result=%s error_code=%s", evidence.Operation, evidence.Result, evidence.ErrorCode)
}

// PushTransportSender adapts the durable worker's token-bearing internal row
// to the provider-neutral transport. It is intentionally the only place that
// sees both types; neither policy/store code nor the public service can send.
type PushTransportSender struct {
	Transport push.Transport
}

func (s PushTransportSender) Send(ctx context.Context, request PushSendRequest) (string, error) {
	if s.Transport == nil {
		return "", errors.New("push transport unavailable")
	}
	if request.Provider != push.ProviderExpo {
		return "", &push.PermanentError{Operation: "send", Code: "unsupported_provider"}
	}
	priority := push.PriorityNormal
	if request.Urgency == string(DeliveryUrgencyImportant) || request.Urgency == string(DeliveryUrgencyCritical) {
		priority = push.PriorityHigh
	}
	ticket, err := s.Transport.Send(ctx, push.Message{
		Token: request.Token,
		Title: request.Title,
		Body:  request.Body,
		Data: push.EnvelopeData{
			Schema: request.Data.Schema, NotificationID: request.Data.NotificationID,
			Event: request.Data.Event, Route: request.Data.Route,
		},
		// The logical notification id is stable across a lease crash/retry. Expo
		// and the operating system can therefore coalesce a duplicate attempt.
		CollapseKey: request.Data.NotificationID,
		Tag:         request.Data.NotificationID,
		Priority:    priority,
	})
	if err != nil {
		return "", err
	}
	return ticket.ID, nil
}
