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

package modelproxy

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log"
	"mime"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/agentsession"
)

// Platform-route metering (w10/m14 t003) and data policy (t006). Only an
// exchange whose mint carried PinnedModel — the platform-owned key — is
// metered; workspace BYO keys are the tenant's own spend and are never read.

// platformDataPolicy is the OpenRouter provider-routing object the gateway
// merges into every platform-route Messages request: route only to providers
// that do not store prompts non-transiently or train on them. Verified live
// on 2026-10-27: https://openrouter.ai/api/v1/messages validates
// provider.data_collection (a bad value is a 400 naming the field) and
// serves the pinned model with "deny".
const platformDataPolicy = `"provider":{"data_collection":"deny"}`

// withPlatformDataPolicy appends platformDataPolicy as the last member of
// body, a JSON object pinnedModelRequest has already accepted (so it is one
// object, carries a "model" member, and carries no client "provider" to
// conflict with). The client's own bytes are kept verbatim ahead of it.
func withPlatformDataPolicy(body []byte) []byte {
	trimmed := bytes.TrimRight(body, " \t\r\n")
	out := make([]byte, 0, len(trimmed)+len(platformDataPolicy)+1)
	out = append(out, trimmed[:len(trimmed)-1]...)
	out = append(out, ',')
	out = append(out, platformDataPolicy...)
	return append(out, '}')
}

const (
	// maxMeteredJSONBytes bounds the copy of a non-streaming response kept for
	// its usage object; a Messages response is far smaller. Past it the
	// exchange is relayed unchanged and its usage is dropped loudly.
	maxMeteredJSONBytes = 8 << 20
	// maxMeteredSSELine bounds one buffered SSE line; a longer line is
	// skipped. Anthropic usage rides on small message_start / message_delta
	// events, but a Responses stream reports it on response.completed, which
	// carries the whole response, so the bound matches a JSON body's.
	maxMeteredSSELine = maxMeteredJSONBytes
)

// anthropicUsage is the Messages API usage object. Pointers, because
// OpenRouter sends null for absent cache counts and a message_delta carries
// only the counts that changed. Cost is OpenRouter's own USD cost of the
// exchange (ADR091 D2.1): returned by default on /api/v1/messages, top-level
// usage.cost on a JSON response and on the message_delta event of a stream.
type anthropicUsage struct {
	InputTokens              *int64   `json:"input_tokens"`
	OutputTokens             *int64   `json:"output_tokens"`
	CacheCreationInputTokens *int64   `json:"cache_creation_input_tokens"`
	CacheReadInputTokens     *int64   `json:"cache_read_input_tokens"`
	Cost                     *float64 `json:"cost"`
}

// responsesUsage is the Responses API usage object (Meta's API, ADR108):
// input_tokens includes the cached prompt tokens it details.
type responsesUsage struct {
	InputTokens        *int64 `json:"input_tokens"`
	OutputTokens       *int64 `json:"output_tokens"`
	InputTokensDetails *struct {
		CachedTokens *int64 `json:"cached_tokens"`
	} `json:"input_tokens_details"`
}

// anthropic maps a Responses usage onto the meter's Messages shape: uncached
// input, output and cache reads.
func (u *responsesUsage) anthropic() *anthropicUsage {
	if u == nil {
		return nil
	}
	out := &anthropicUsage{InputTokens: u.InputTokens, OutputTokens: u.OutputTokens}
	if d := u.InputTokensDetails; d != nil && d.CachedTokens != nil && u.InputTokens != nil {
		cached := min(*d.CachedTokens, *u.InputTokens)
		uncached := *u.InputTokens - cached
		out.InputTokens, out.CacheReadInputTokens = &uncached, &cached
	}
	return out
}

// usageMeter is the tee side of a relayed platform-route response: it reads
// the bytes the client is being sent, never delays or alters them, and
// extracts the Anthropic or Responses usage. Write never fails, so a parse problem can
// never break the relay.
type usageMeter struct {
	sse      bool
	jsonBuf  bytes.Buffer
	overflow bool
	line     []byte
	skipping bool
	seen     bool
	tokens   [4]int64 // input, output, cache read, cache write
	cost     *float64 // last reported non-negative usage.cost, USD
}

func newUsageMeter(contentType string) *usageMeter {
	mediaType, _, _ := mime.ParseMediaType(contentType)
	return &usageMeter{sse: mediaType == "text/event-stream"}
}

func (m *usageMeter) Write(p []byte) (int, error) {
	n := len(p)
	if !m.sse {
		if m.jsonBuf.Len()+len(p) > maxMeteredJSONBytes {
			m.overflow = true
		} else {
			m.jsonBuf.Write(p)
		}
		return n, nil
	}
	for len(p) > 0 {
		i := bytes.IndexByte(p, '\n')
		chunk := p
		if i >= 0 {
			chunk = p[:i]
		}
		if !m.skipping {
			if len(m.line)+len(chunk) > maxMeteredSSELine {
				m.skipping, m.line = true, m.line[:0]
			} else {
				m.line = append(m.line, chunk...)
			}
		}
		if i < 0 {
			break
		}
		if !m.skipping {
			m.sseLine(bytes.TrimRight(m.line, "\r"))
		}
		m.line, m.skipping = m.line[:0], false
		p = p[i+1:]
	}
	return n, nil
}

var (
	usageMarker     = []byte(`"usage"`)
	nullUsageMarker = []byte(`"usage":null`)
)

// sseLine handles one complete SSE line. Only data lines that mention usage
// are decoded, so the many content deltas of a long turn cost a byte scan.
func (m *usageMeter) sseLine(line []byte) {
	data, ok := bytes.CutPrefix(line, []byte("data:"))
	// response.created / in_progress carry "usage":null on the whole
	// response object: nothing to read, so it is not decoded.
	if !ok || !bytes.Contains(data, usageMarker) || bytes.Contains(data, nullUsageMarker) {
		return
	}
	var event struct {
		Type    string `json:"type"`
		Message struct {
			Usage *anthropicUsage `json:"usage"`
		} `json:"message"`
		Usage    *anthropicUsage `json:"usage"`
		Response struct {
			Usage *responsesUsage `json:"usage"`
		} `json:"response"`
	}
	if json.Unmarshal(bytes.TrimSpace(data), &event) != nil {
		return
	}
	switch event.Type {
	case "message_start":
		m.merge(event.Message.Usage)
	case "message_delta":
		// Anthropic's message_delta usage is cumulative for the message, so a
		// present count replaces the message_start one.
		m.merge(event.Usage)
	case "response.completed", "response.incomplete", "response.failed":
		// A Responses stream reports the whole exchange's usage once, on its
		// terminal event.
		m.merge(event.Response.Usage.anthropic())
	}
}

func (m *usageMeter) merge(u *anthropicUsage) {
	if u == nil {
		return
	}
	for i, v := range []*int64{u.InputTokens, u.OutputTokens, u.CacheReadInputTokens, u.CacheCreationInputTokens} {
		if v != nil && *v >= 0 {
			m.tokens[i] = *v
			m.seen = true
		}
	}
	if u.Cost != nil && *u.Cost >= 0 {
		cost := *u.Cost
		m.cost = &cost
	}
}

// result is the exchange's usage once the relay is done; ok is false when
// none was found (unparsed is true when that is a loss, not a usage-free
// response). cost is the reported USD cost, nil when the provider sent none.
func (m *usageMeter) result() (tokens [4]int64, cost *float64, ok, unparsed bool) {
	if !m.sse {
		if m.overflow {
			return tokens, nil, false, true
		}
		var body struct {
			Object string          `json:"object"`
			Usage  json.RawMessage `json:"usage"`
		}
		if json.Unmarshal(m.jsonBuf.Bytes(), &body) != nil {
			return tokens, nil, false, true
		}
		if body.Object == "response" {
			var usage *responsesUsage
			if json.Unmarshal(body.Usage, &usage) == nil {
				m.merge(usage.anthropic())
			}
		} else {
			var usage *anthropicUsage
			if json.Unmarshal(body.Usage, &usage) == nil {
				m.merge(usage)
			}
		}
	}
	if !m.seen {
		return tokens, nil, false, true
	}
	return m.tokens, m.cost, true, false
}

// usageAPI is the bex-api usage verb; *agentsession.ModelClient satisfies it.
type usageAPI interface {
	ReportUsage(ctx context.Context, rep agentsession.ModelUsageReport) error
}

const (
	usageQueueSize      = 4096
	usageReportAttempts = 6
	usageReportTimeout  = 10 * time.Second
	usageRetryBaseDelay = time.Second
)

// usageReporter delivers usage reports off the request path: the response
// has already been relayed when a report is queued, so metering can never
// block or fail a model exchange. One worker drains a bounded queue, retrying
// each report with backoff under its fixed request id (bex-api records a
// request id once). A full queue or an exhausted retry drops the report with
// a log line and a metric — the OpenRouter account balance (ADR091 D5) is the
// spend backstop for such a loss.
type usageReporter struct {
	api   usageAPI
	note  func(result string)
	delay time.Duration
	queue chan agentsession.ModelUsageReport
}

func newUsageReporter(api usageAPI, note func(string), delay time.Duration) *usageReporter {
	r := &usageReporter{api: api, note: note, delay: delay, queue: make(chan agentsession.ModelUsageReport, usageQueueSize)}
	go r.run()
	return r
}

func (r *usageReporter) enqueue(rep agentsession.ModelUsageReport) {
	select {
	case r.queue <- rep:
	default:
		r.note("dropped_queue_full")
		log.Printf("agent model proxy: usage report dropped, queue full (session=%s request=%s)", rep.SessionID, rep.RequestID)
	}
}

func (r *usageReporter) run() {
	for rep := range r.queue {
		r.deliver(rep)
	}
}

func (r *usageReporter) deliver(rep agentsession.ModelUsageReport) {
	delay := r.delay
	var err error
	for attempt := 1; attempt <= usageReportAttempts; attempt++ {
		ctx, cancel := context.WithTimeout(context.Background(), usageReportTimeout)
		err = r.api.ReportUsage(ctx, rep)
		cancel()
		if err == nil {
			r.note("reported")
			return
		}
		// A refusal (unknown/cross session, malformed report) will not change
		// on retry.
		if errors.Is(err, agentsession.ErrForbidden) {
			break
		}
		if attempt < usageReportAttempts {
			r.note("retried")
			time.Sleep(delay)
			delay *= 2
		}
	}
	r.note("dropped_failed")
	log.Printf("agent model proxy: usage report dropped (session=%s request=%s): %v", rep.SessionID, rep.RequestID, err)
}
