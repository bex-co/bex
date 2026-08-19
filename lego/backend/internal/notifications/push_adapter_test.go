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
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/notifications/push"
)

type capturePushTransport struct {
	message push.Message
}

func (c *capturePushTransport) Send(_ context.Context, message push.Message) (push.Ticket, error) {
	c.message = message
	return push.Ticket{ID: "ticket-1"}, nil
}

func (*capturePushTransport) CheckReceipts(context.Context, []string) (map[string]push.Receipt, error) {
	return nil, nil
}

func TestPushTransportSenderMapsClosedEnvelopeAndUrgency(t *testing.T) {
	transport := &capturePushTransport{}
	sender := PushTransportSender{Transport: transport}
	ticket, err := sender.Send(context.Background(), PushSendRequest{
		Provider: "expo", Platform: "ios", Token: "ExpoPushToken[abcdefghijklmnop]",
		Title: "Deploy failed", Body: "api deploy failed.", Urgency: "important",
		Data: PushEnvelopeData{
			Schema: "bex.notification.v1", NotificationID: "evt-abcdefghijklmnopqrst",
			Event: "deploy_failed", Route: "/services/srv-abcdefghijklmnopqrst",
			Subject: "identity-1", WorkspaceID: "tea-1", SessionID: "session-1",
		},
	})
	if err != nil || ticket != "ticket-1" {
		t.Fatalf("Send() = (%q, %v)", ticket, err)
	}
	if transport.message.Priority != push.PriorityHigh || transport.message.CollapseKey != "evt-abcdefghijklmnopqrst" || transport.message.Tag != transport.message.CollapseKey {
		t.Fatalf("provider message policy = %+v", transport.message)
	}
	if transport.message.Data.Event != "deploy_failed" || transport.message.Data.Route != "/services/srv-abcdefghijklmnopqrst" {
		t.Fatalf("provider envelope = %+v", transport.message.Data)
	}
}

func TestPushTransportSenderRejectsUnknownProviderBeforeTransport(t *testing.T) {
	transport := &capturePushTransport{}
	_, err := (PushTransportSender{Transport: transport}).Send(context.Background(), PushSendRequest{Provider: "apns"})
	if err == nil || transport.message.Token != "" {
		t.Fatalf("unknown provider error/message = (%v, %+v)", err, transport.message)
	}
}

func TestPushTransportSenderSupportsExactlyConfiguredProviders(t *testing.T) {
	cases := []struct {
		name    string
		sender  PushTransportSender
		expo    bool
		webpush bool
	}{
		{name: "neither", sender: PushTransportSender{}},
		{name: "expo only", sender: PushTransportSender{Transport: &capturePushTransport{}}, expo: true},
		{name: "webpush only", sender: PushTransportSender{WebPush: &push.WebPush{}}, webpush: true},
		{name: "both", sender: PushTransportSender{Transport: &capturePushTransport{}, WebPush: &push.WebPush{}}, expo: true, webpush: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.sender.Supports(push.ProviderExpo); got != tc.expo {
				t.Fatalf("Supports(expo) = %v, want %v", got, tc.expo)
			}
			if got := tc.sender.Supports(push.ProviderWebPush); got != tc.webpush {
				t.Fatalf("Supports(webpush) = %v, want %v", got, tc.webpush)
			}
			if tc.sender.Supports("apns") {
				t.Fatal("unknown provider must be unsupported")
			}
		})
	}
}
