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

package push

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

const (
	defaultExpoEndpoint = "https://exp.host/--/api/v2/push"
	defaultTimeout      = 10 * time.Second
	maxResponseBytes    = 64 << 10
)

type httpDoer interface {
	Do(*http.Request) (*http.Response, error)
}

// Option customizes construction without exposing provider details through the
// Transport interface.
type Option func(*expoOptions)

type expoOptions struct {
	doer httpDoer
}

// WithHTTPClient supplies a fake or instrumented client for tests.
func WithHTTPClient(client httpDoer) Option {
	return func(o *expoOptions) { o.doer = client }
}

type expoTransport struct {
	accessToken string
	sendURL     string
	receiptsURL string
	http        httpDoer
}

// New constructs the configured transport. Empty Provider is the only disabled
// state and returns (nil, nil) before allocating an HTTP client or parsing any
// provider settings.
func New(config Config, options ...Option) (Transport, error) {
	provider := strings.TrimSpace(config.Provider)
	if provider == "" {
		return nil, nil
	}
	if provider != ProviderExpo {
		return nil, fmt.Errorf("push: unsupported BEX_PUSH_PROVIDER %q", provider)
	}
	if err := validateCredential(config.AccessToken); err != nil {
		return nil, err
	}
	endpoint := strings.TrimRight(strings.TrimSpace(config.Endpoint), "/")
	if endpoint == "" {
		endpoint = defaultExpoEndpoint
	}
	if err := validateEndpoint(endpoint); err != nil {
		return nil, err
	}
	timeout := config.Timeout
	if timeout == 0 {
		timeout = defaultTimeout
	}
	if timeout < 0 || timeout > time.Minute {
		return nil, errors.New("push: timeout must be between zero and one minute")
	}
	opts := expoOptions{}
	for _, option := range options {
		option(&opts)
	}
	if opts.doer == nil {
		opts.doer = &http.Client{Timeout: timeout}
	}
	return &expoTransport{
		accessToken: config.AccessToken,
		sendURL:     endpoint + "/send",
		receiptsURL: endpoint + "/getReceipts",
		http:        opts.doer,
	}, nil
}

func validateCredential(token string) error {
	if token == "" {
		return errors.New("push: BEX_EXPO_PUSH_ACCESS_TOKEN is required when BEX_PUSH_PROVIDER=expo")
	}
	if len(token) > 4096 || strings.TrimSpace(token) != token || strings.ContainsAny(token, "\r\n\x00") {
		return errors.New("push: BEX_EXPO_PUSH_ACCESS_TOKEN is malformed")
	}
	return nil
}

func validateEndpoint(raw string) error {
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return errors.New("push: BEX_EXPO_PUSH_URL must be an absolute HTTPS API base URL")
	}
	if u.Scheme == "https" {
		return nil
	}
	host := u.Hostname()
	parsedIP := net.ParseIP(host)
	if u.Scheme == "http" && (host == "localhost" || parsedIP != nil && parsedIP.IsLoopback()) {
		return nil
	}
	return errors.New("push: BEX_EXPO_PUSH_URL must use HTTPS (HTTP is allowed only for loopback stubs)")
}

type expoMessage struct {
	To         string       `json:"to"`
	Title      string       `json:"title,omitempty"`
	Body       string       `json:"body,omitempty"`
	Data       EnvelopeData `json:"data"`
	CollapseID string       `json:"collapseId,omitempty"`
	Tag        string       `json:"tag,omitempty"`
	Priority   Priority     `json:"priority,omitempty"`
}

type expoTicket struct {
	Status  string `json:"status"`
	ID      string `json:"id"`
	Details struct {
		Error string `json:"error"`
	} `json:"details"`
}

type expoEnvelope struct {
	Data json.RawMessage `json:"data"`
}

func (t *expoTransport) Send(ctx context.Context, message Message) (Ticket, error) {
	payload, err := encodeMessage(message)
	if err != nil {
		return Ticket{}, err
	}
	var envelope expoEnvelope
	if err := t.post(ctx, "send", t.sendURL, payload, &envelope); err != nil {
		return Ticket{}, err
	}
	var ticket expoTicket
	if err := json.Unmarshal(envelope.Data, &ticket); err != nil || ticket.Status == "" {
		var tickets []expoTicket
		if listErr := json.Unmarshal(envelope.Data, &tickets); listErr != nil || len(tickets) != 1 {
			return Ticket{}, &TransientError{Operation: "send", Detail: "malformed_response"}
		}
		ticket = tickets[0]
	}
	if ticket.Status == "error" {
		return Ticket{}, providerError("send", ticket.Details.Error, 0)
	}
	if ticket.Status != "ok" || ticket.ID == "" || len(ticket.ID) > 256 {
		return Ticket{}, &TransientError{Operation: "send", Detail: "malformed_ticket"}
	}
	return Ticket{ID: ticket.ID}, nil
}

func encodeMessage(message Message) ([]byte, error) {
	if !validExpoToken(message.Token) {
		return nil, &InvalidTokenError{Code: "InvalidExpoPushToken"}
	}
	checks := []struct {
		name  string
		value string
		max   int
	}{
		{"title", message.Title, MaxTitleBytes},
		{"body", message.Body, MaxBodyBytes},
		{"schema", message.Data.Schema, 64},
		{"notificationId", message.Data.NotificationID, MaxNotificationIDBytes},
		{"event", message.Data.Event, MaxEventBytes},
		{"route", message.Data.Route, MaxRouteBytes},
		{"collapseKey", message.CollapseKey, MaxCollapseBytes},
		{"tag", message.Tag, MaxTagBytes},
	}
	for _, check := range checks {
		if !utf8.ValidString(check.value) {
			return nil, &PayloadError{Field: check.name, Reason: "invalid_utf8"}
		}
		if len(check.value) > check.max {
			return nil, &PayloadError{Field: check.name, Reason: "too_large"}
		}
	}
	if message.Data.Schema != "bex.notification.v1" {
		return nil, &PayloadError{Field: "schema", Reason: "unsupported"}
	}
	if message.Data.NotificationID == "" || message.Data.Event == "" || message.Data.Route == "" {
		return nil, &PayloadError{Field: "data", Reason: "required"}
	}
	if !strings.HasPrefix(message.Data.Route, "/") {
		return nil, &PayloadError{Field: "route", Reason: "must_be_app_relative"}
	}
	if message.Priority != PriorityDefault && message.Priority != PriorityNormal && message.Priority != PriorityHigh {
		return nil, &PayloadError{Field: "priority", Reason: "unsupported"}
	}
	payload, err := json.Marshal(expoMessage{
		To:         message.Token,
		Title:      message.Title,
		Body:       message.Body,
		Data:       message.Data,
		CollapseID: message.CollapseKey,
		Tag:        message.Tag,
		Priority:   message.Priority,
	})
	if err != nil {
		return nil, &PayloadError{Field: "message", Reason: "not_encodable"}
	}
	if len(payload) > MaxPayloadBytes {
		return nil, &PayloadError{Field: "message", Reason: "too_large"}
	}
	return payload, nil
}

func validExpoToken(token string) bool {
	var body string
	switch {
	case strings.HasPrefix(token, "ExpoPushToken[") && strings.HasSuffix(token, "]"):
		body = strings.TrimSuffix(strings.TrimPrefix(token, "ExpoPushToken["), "]")
	case strings.HasPrefix(token, "ExponentPushToken[") && strings.HasSuffix(token, "]"):
		body = strings.TrimSuffix(strings.TrimPrefix(token, "ExponentPushToken["), "]")
	default:
		return false
	}
	if len(body) < 10 || len(body) > 200 {
		return false
	}
	for _, r := range body {
		if !(r >= 'a' && r <= 'z') && !(r >= 'A' && r <= 'Z') && !(r >= '0' && r <= '9') && r != '-' && r != '_' {
			return false
		}
	}
	return true
}

func (t *expoTransport) CheckReceipts(ctx context.Context, ids []string) (map[string]Receipt, error) {
	if len(ids) == 0 || len(ids) > maxReceiptIDs {
		return nil, &PayloadError{Field: "receiptIds", Reason: "count_out_of_range"}
	}
	seen := make(map[string]struct{}, len(ids))
	for _, id := range ids {
		if id == "" || len(id) > 256 {
			return nil, &PayloadError{Field: "receiptIds", Reason: "malformed"}
		}
		if _, duplicate := seen[id]; duplicate {
			return nil, &PayloadError{Field: "receiptIds", Reason: "duplicate"}
		}
		seen[id] = struct{}{}
	}
	payload, _ := json.Marshal(struct {
		IDs []string `json:"ids"`
	}{IDs: ids})
	var envelope struct {
		Data map[string]expoTicket `json:"data"`
	}
	if err := t.post(ctx, "receipt", t.receiptsURL, payload, &envelope); err != nil {
		return nil, err
	}
	if envelope.Data == nil {
		return nil, &TransientError{Operation: "receipt", Detail: "malformed_response"}
	}
	result := make(map[string]Receipt, len(envelope.Data))
	for id, providerReceipt := range envelope.Data {
		if _, requested := seen[id]; !requested {
			continue
		}
		receipt := Receipt{ID: id}
		switch providerReceipt.Status {
		case "ok":
		case "error":
			receipt.Err = providerError("receipt", providerReceipt.Details.Error, 0)
		default:
			receipt.Err = &TransientError{Operation: "receipt", Detail: "malformed_result"}
		}
		result[id] = receipt
	}
	return result, nil
}

func (t *expoTransport) post(ctx context.Context, operation, endpoint string, payload []byte, output any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(payload))
	if err != nil {
		return &TransientError{Operation: operation, Detail: "request"}
	}
	req.Header.Set("Authorization", "Bearer "+t.accessToken)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	response, err := t.http.Do(req)
	if err != nil {
		return &TransientError{Operation: operation, Detail: "network"}
	}
	defer response.Body.Close()
	if response.StatusCode == http.StatusTooManyRequests {
		return &RateLimitedError{Operation: operation, RetryAfter: parseRetryAfter(response.Header.Get("Retry-After"))}
	}
	if response.StatusCode == http.StatusRequestTimeout || response.StatusCode >= 500 {
		return &TransientError{Operation: operation, Detail: "provider_outage"}
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return &PermanentError{Operation: operation, Code: "http_" + strconv.Itoa(response.StatusCode)}
	}
	decoder := json.NewDecoder(io.LimitReader(response.Body, maxResponseBytes))
	if err := decoder.Decode(output); err != nil {
		return &TransientError{Operation: operation, Detail: "malformed_response"}
	}
	return nil
}

func providerError(operation, code string, retryAfter time.Duration) error {
	switch code {
	case "DeviceNotRegistered":
		return &InvalidTokenError{Code: code}
	case "InvalidCredentials", "MismatchSenderId":
		return &PermanentError{Operation: operation, Code: code}
	case "MessageRateExceeded", "MessageRateLimited":
		return &RateLimitedError{Operation: operation, RetryAfter: retryAfter}
	case "MessageTooBig":
		return &PayloadError{Field: "message", Reason: "provider_too_large"}
	case "":
		return &PermanentError{Operation: operation, Code: "provider_rejected"}
	default:
		// Expo's documented details.error values are handled explicitly above.
		// Never retain an unknown provider-controlled string in an error because
		// callers may log the typed error.
		return &PermanentError{Operation: operation, Code: "provider_rejected"}
	}
}

func parseRetryAfter(raw string) time.Duration {
	if seconds, err := strconv.Atoi(strings.TrimSpace(raw)); err == nil && seconds >= 0 {
		return time.Duration(seconds) * time.Second
	}
	if when, err := http.ParseTime(raw); err == nil {
		delay := time.Until(when)
		if delay > 0 {
			return delay
		}
	}
	return 0
}
