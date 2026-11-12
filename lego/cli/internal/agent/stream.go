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

package agent

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

// The live conversation stream (ADR047 D9, ADR106 D2): bex-api mints a
// single-use, 90-second read ticket; the isolated gateway answers GET
// <streamUrl> carrying it with the AI SDK v1 UI-message stream — the durable
// transcript replayed as chunks, then the live tail, then `[DONE]`. The ticket
// is the stream's only credential, so no bearer is sent.

const (
	// ticketHeader carries the agent-session ticket (agentsessionticket.TicketHeader).
	ticketHeader = "X-Bex-Agent-Ticket"
	// maxStreamEvent bounds one SSE event (a tool's whole output can be large).
	maxStreamEvent = 16 << 20

	codeSessionNotAttachable = "AGENT_SESSION_NOT_ATTACHABLE"
	codeSessionTurnInFlight  = "AGENT_SESSION_TURN_IN_FLIGHT"
)

var (
	// errStreamUnavailable: this deployment serves no stream, or the stream
	// kept failing; the caller polls instead.
	errStreamUnavailable = errors.New("the live stream is unavailable")
	// errNotAttachable: the session has no sandbox yet (provisioning).
	errNotAttachable = errors.New("the session is not attachable yet")
)

type attachTicket struct {
	Ticket    string `json:"ticket"`
	StreamURL string `json:"streamUrl"`
}

// attachTicket mints a read ticket for id's stream.
func (a *api) attachTicket(ctx context.Context, id string) (attachTicket, error) {
	raw, err := a.post(ctx, request{path: []string{"agent-sessions", id, "attach-ticket"}, query: url.Values{"action": {"read"}}}, nil)
	var apiErr *apiError
	switch {
	case errors.As(err, &apiErr) && apiErr.Code == codeSessionNotAttachable:
		return attachTicket{}, errNotAttachable
	case errors.As(err, &apiErr) && apiErr.Status == http.StatusServiceUnavailable && apiErr.Code != "",
		errors.As(err, &apiErr) && apiErr.Status == http.StatusNotFound && apiErr.Code == "":
		// A coded 503 is this deployment saying it has no stream (a bare one
		// is a rolling gateway, retried as transient); a bare 404 is an
		// API without the route (an unknown session is a coded 404).
		return attachTicket{}, errStreamUnavailable
	case err != nil:
		return attachTicket{}, fmt.Errorf("attach to agent session %s: %w", id, err)
	}
	var t attachTicket
	if err := json.Unmarshal(raw, &t); err != nil || t.Ticket == "" || t.StreamURL == "" {
		return attachTicket{}, errStreamUnavailable
	}
	return t, nil
}

// streamOnce reads one connection of t's stream into r, until `[DONE]` or
// the connection ends.
func (a *api) streamOnce(ctx context.Context, t attachTicket, r *chunkRenderer) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, t.StreamURL, nil)
	if err != nil {
		return err
	}
	req.Header.Set(ticketHeader, t.Ticket)
	req.Header.Set("Accept", "text/event-stream")
	resp, err := a.http.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 64<<10))
		return responseError(resp, body)
	}
	_, err = readSSE(resp.Body, r.chunk)
	return err
}

// readSSE calls emit with each event's data until `[DONE]` or the end of the
// body. Comment lines (the gateway's heartbeats) carry nothing.
func readSSE(body io.Reader, emit func([]byte)) (done bool, err error) {
	scanner := bufio.NewScanner(body)
	scanner.Buffer(make([]byte, 64<<10), maxStreamEvent)
	var data []string
	dispatch := func() bool {
		if len(data) == 0 {
			return false
		}
		payload := strings.Join(data, "\n")
		data = data[:0]
		if payload == "[DONE]" {
			return true
		}
		emit([]byte(payload))
		return false
	}
	for scanner.Scan() {
		line := scanner.Text()
		switch {
		case line == "":
			if dispatch() {
				return true, nil
			}
		case strings.HasPrefix(line, ":"):
		case strings.HasPrefix(line, "data:"):
			data = append(data, strings.TrimPrefix(strings.TrimPrefix(line, "data:"), " "))
		}
	}
	if dispatch() {
		return true, nil
	}
	if err := scanner.Err(); err != nil {
		return false, err
	}
	return false, io.ErrUnexpectedEOF
}

// streamUntilSettled shows id's stream through r until the session settles,
// and returns that read. A connection can end before the turn does: dropped,
// or closed by the gateway with `[DONE]` because the turn's new sandbox does
// not serve yet. Either way the session is read, and while it is still
// working the stream is attached again with a fresh ticket (r skips what it
// already printed), backing off while nothing new arrives.
// errStreamUnavailable: no stream here, or one that keeps failing.
func (a *api) streamUntilSettled(ctx context.Context, id string, r *chunkRenderer) (session, []byte, error) {
	var poll poller
	delay, drops := pollFirst, 0
	for {
		t, err := a.attachTicket(ctx, id)
		switch {
		case errors.Is(err, errNotAttachable):
		case errors.Is(err, errStreamUnavailable):
			return session{}, nil, err
		case err != nil:
			if !poll.tolerate(ctx, err) {
				return session{}, nil, err
			}
		default:
			before := r.printed
			r.reconnect()
			err := a.streamOnce(ctx, t, r)
			switch {
			case ctx.Err() != nil:
				return session{}, nil, ctx.Err()
			case r.printed > before:
				delay, drops = pollFirst, 0
			case err != nil:
				if drops++; drops > transientLimit {
					return session{}, nil, errStreamUnavailable
				}
			}
		}
		s, raw, ok, err := a.readSession(ctx, id, &poll)
		if err != nil {
			return session{}, nil, err
		}
		if ok && settled(s) {
			return s, raw, nil
		}
		if err := poll.sleep(ctx, a, delay); err != nil {
			return session{}, nil, err
		}
		delay = nextPoll(delay)
	}
}

// followRendered is follow on the live stream: it prints the turn through r
// as the agent works and returns the settled session, so the result and exit
// codes are follow's. Without a stream it polls; a stream that fails after
// printing waits instead, so nothing prints twice.
func (a *api) followRendered(ctx context.Context, id string, r *chunkRenderer) (waitResult, error) {
	s, raw, err := a.streamUntilSettled(ctx, id, r)
	switch {
	case errors.Is(err, errStreamUnavailable) && r.printed == 0:
		return a.follow(ctx, id, r.fromTurn, r.p)
	case errors.Is(err, errStreamUnavailable):
		res, werr := a.wait(ctx, []string{id}, false)
		if werr == nil {
			res.tailErr = errors.New("the live stream stopped before the turn ended")
		}
		return res, werr
	case err != nil && ctx.Err() != nil:
		if stopped(ctx) {
			return waitResult{timedOut: true}, nil
		}
		return waitResult{}, errInterrupted
	case err != nil:
		return waitResult{}, err
	}
	return waitResult{sessions: []session{s}, raws: [][]byte{raw}}, nil
}

// followLive is followRendered for one command; JSON lines poll as before,
// since they print transcript rows, not stream chunks.
func (a *api) followLive(ctx context.Context, id string, fromTurn int, p *partPrinter) (waitResult, error) {
	if p.jsonLines {
		return a.follow(ctx, id, fromTurn, p)
	}
	return a.followRendered(ctx, id, newChunkRenderer(p, fromTurn))
}

// chunkRenderer turns UI-message chunks into the transcript printer's lines,
// one per complete item: text and reasoning at their end chunk, a tool call
// when its input is available. A reconnect replays the transcript from the
// start, so items are counted per turn and the ones already printed are
// skipped.
type chunkRenderer struct {
	p        *partPrinter
	fromTurn int
	seen     map[int]int // items printed per turn, across connections
	printed  int         // items printed in all

	turn    int
	ordinal int
	open    map[openKey]*strings.Builder
	calls   map[string]toolCall
}

type openKey struct{ kind, id string }

type toolCall struct {
	name     string
	replayed bool // already printed by an earlier connection
}

func newChunkRenderer(p *partPrinter, fromTurn int) *chunkRenderer {
	return &chunkRenderer{p: p, fromTurn: fromTurn, seen: map[int]int{},
		open: map[openKey]*strings.Builder{}, calls: map[string]toolCall{}}
}

// reconnect starts a new connection, whose replay begins again at turn 1.
func (r *chunkRenderer) reconnect() {
	r.turn, r.ordinal = 0, 0
	clear(r.open)
	clear(r.calls)
}

// shown reports whether the current turn is one to print.
func (r *chunkRenderer) shown() bool { return r.turn >= r.fromTurn }

type streamChunk struct {
	Type       string          `json:"type"`
	ID         string          `json:"id"`
	Delta      string          `json:"delta"`
	ToolCallID string          `json:"toolCallId"`
	ToolName   string          `json:"toolName"`
	Input      json.RawMessage `json:"input"`
	ErrorText  string          `json:"errorText"`
	Data       struct {
		Turn int    `json:"turn"`
		Text string `json:"text"`
	} `json:"data"`
}

func (r *chunkRenderer) chunk(raw []byte) {
	var c streamChunk
	if json.Unmarshal(raw, &c) != nil {
		return
	}
	if kind, phase, ok := strings.Cut(c.Type, "-"); ok && (kind == "text" || kind == "reasoning") {
		key := openKey{kind, c.ID}
		switch b := r.open[key]; phase {
		case "start":
			if r.shown() {
				r.open[key] = &strings.Builder{}
			}
		case "delta":
			if b != nil {
				b.WriteString(c.Delta)
			}
		case "end":
			if b != nil {
				delete(r.open, key)
				r.item(uiPart{Type: kind, Text: b.String()}, "")
			}
		}
		return
	}
	switch c.Type {
	case "data-user-prompt":
		r.turn, r.ordinal = c.Data.Turn, 0
		r.p.setPrompt(c.Data.Turn, c.Data.Text)
	case "tool-input-available":
		r.calls[c.ToolCallID] = toolCall{name: c.ToolName}
		r.item(uiPart{Type: "dynamic-tool", ToolName: c.ToolName, Input: c.Input}, c.ToolCallID)
	case "tool-input-error", "tool-output-error":
		if call := r.calls[c.ToolCallID]; !call.replayed && r.shown() {
			r.p.toolError(r.turn, firstNonEmpty(call.name, c.ToolName, "tool"), c.ErrorText)
		}
	case "error", "data-bex-turn-error":
		if r.shown() && c.ErrorText != "" {
			r.p.turnError(r.turn, c.ErrorText)
		}
	}
}

// item prints the next item of the current turn unless an earlier connection
// already did (turns before fromTurn are not printed at all).
func (r *chunkRenderer) item(ui uiPart, toolCallID string) {
	k := r.ordinal
	r.ordinal++
	if !r.shown() || k < r.seen[r.turn] {
		if toolCallID != "" {
			r.calls[toolCallID] = toolCall{name: r.calls[toolCallID].name, replayed: true}
		}
		return
	}
	r.seen[r.turn] = k + 1
	r.printed++
	r.p.printItem(r.turn, ui)
}
