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

// Package push defines the provider-neutral mobile push boundary and the Expo
// Push Service adapter. Provider configuration is explicit: an empty provider
// returns a nil Transport and constructs no HTTP client.
package push

import (
	"context"
	"fmt"
	"time"
)

const (
	ProviderExpo    = "expo"
	ProviderWebPush = "webpush"

	// MaxPayloadBytes is Expo's maximum encoded notification size.
	MaxPayloadBytes        = 4096
	MaxTitleBytes          = 120
	MaxBodyBytes           = 1024
	MaxRouteBytes          = 512
	MaxNotificationIDBytes = 128
	MaxEventBytes          = 64
	MaxBindingIDBytes      = 200
	MaxCollapseBytes       = 64
	MaxTagBytes            = 64

	maxReceiptIDs = 1000
)

// Config is the complete provider configuration. Endpoint is the Expo push API
// base (the adapter appends /send and /getReceipts). It is primarily an
// integration-test/self-hosted stub seam; production should leave it empty.
type Config struct {
	Provider    string
	AccessToken string
	Endpoint    string
	Timeout     time.Duration
}

// Priority is intentionally provider-neutral. Later notification policy may
// choose it without importing Expo vocabulary.
type Priority string

const (
	PriorityDefault Priority = ""
	PriorityNormal  Priority = "normal"
	PriorityHigh    Priority = "high"
)

// Message is deliberately narrow. Arbitrary caller data is excluded so
// credentials, environment values, and logs cannot accidentally become push
// payloads. Route is an app-internal deep link, not an arbitrary URL.
type Message struct {
	Token       string
	Title       string
	Body        string
	Data        EnvelopeData
	CollapseKey string
	Tag         string
	Priority    Priority
}

// EnvelopeData is the complete closed custom-data contract understood by the
// native client. Keeping it typed prevents arbitrary facts or credentials from
// drifting into provider-visible payloads.
type EnvelopeData struct {
	Schema         string `json:"schema"`
	NotificationID string `json:"notificationId"`
	Event          string `json:"event"`
	Route          string `json:"route"`
	Subject        string `json:"subject"`
	WorkspaceID    string `json:"workspaceId"`
	SessionID      string `json:"sessionId"`
}

// Ticket identifies a message accepted by the provider. Acceptance is not
// delivery: callers must persist ID and call CheckReceipts later.
type Ticket struct {
	ID string
}

// Receipt is the provider's terminal delivery result. Err is nil on delivery;
// typed errors tell the caller whether to retry or prune the device token.
type Receipt struct {
	ID  string
	Err error
}

// Transport is the provider-neutral push seam used by the notifications
// service. CheckReceipts may omit IDs for which the provider has no result yet.
type Transport interface {
	Send(context.Context, Message) (Ticket, error)
	CheckReceipts(context.Context, []string) (map[string]Receipt, error)
}

// TransientError represents a provider/network outage that may be retried.
// Detail is an internal stable label and never provider text or payload data.
type TransientError struct {
	Operation string
	Detail    string
}

func (e *TransientError) Error() string {
	return fmt.Sprintf("push %s temporarily unavailable (%s)", e.Operation, e.Detail)
}

// RateLimitedError represents provider throttling. RetryAfter is zero when the
// provider did not supply a usable delay.
type RateLimitedError struct {
	Operation  string
	RetryAfter time.Duration
}

func (e *RateLimitedError) Error() string { return fmt.Sprintf("push %s rate limited", e.Operation) }

// InvalidTokenError is a permanent device-token error. It intentionally does
// not retain the token.
type InvalidTokenError struct {
	Code string
}

func (e *InvalidTokenError) Error() string { return "push device token is permanently invalid" }

// PayloadError is a permanent caller error. Field is a fixed schema name; the
// rejected value is never retained.
type PayloadError struct {
	Field  string
	Reason string
}

func (e *PayloadError) Error() string {
	return fmt.Sprintf("invalid push payload field %s (%s)", e.Field, e.Reason)
}

// PermanentError represents other non-retryable provider rejection. Code is a
// provider-defined classification, never the provider's free-form message.
type PermanentError struct {
	Operation string
	Code      string
}

func (e *PermanentError) Error() string {
	return fmt.Sprintf("push %s permanently rejected (%s)", e.Operation, e.Code)
}
