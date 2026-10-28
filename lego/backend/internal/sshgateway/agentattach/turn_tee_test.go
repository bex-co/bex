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

package agentattach

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/agenttranscript/uimessagetest"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

func attachOnce(t *testing.T, gw *Server, secret []byte, session string) []string {
	t.Helper()
	srv := httptest.NewServer(gw.Handler())
	defer srv.Close()
	req, _ := http.NewRequest(http.MethodGet, srv.URL, nil)
	req.Header.Set(TicketHeader, attachTicket(t, secret, session))
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	return dataPayloads(string(body))
}

func rawMessages(payloads []string) []json.RawMessage {
	out := make([]json.RawMessage, len(payloads))
	for i, p := range payloads {
		out[i] = json.RawMessage(p)
	}
	return out
}

// The live tee of the recorded production turn forwards every chunk verbatim,
// stores only completed items, and a later reattach replays those items (and
// suppresses the driver's own replay of them) to the same UIMessage parts.
func TestLiveTeeOfTheRecordedTurnStoresItems(t *testing.T) {
	fixture, err := uimessagetest.LoadFixture()
	if err != nil {
		t.Fatal(err)
	}
	history := make([]string, len(fixture))
	for i, chunk := range fixture {
		history[i] = string(chunk)
	}
	secret := []byte("shell-ticket-secret")
	session := "ags-000000000000000000003"
	st := newFakeAttachStore()
	driver, host, port := fakeDriver(history)
	defer driver.Close()
	gw := newAttachGateway(st, fixedPodIP{ip: host}, secret, port)

	first := attachOnce(t, gw, secret, session)
	if fmt.Sprint(first) != fmt.Sprint(history) {
		t.Fatalf("live delivery changed: %d frames, want the driver's %d verbatim", len(first), len(history))
	}
	stored, _ := st.AgentSessionTranscript(context.Background(), session, -1, 1<<30, 0)
	var bytes int
	full := map[string]string{}
	for _, p := range stored {
		bytes += len(p.Part)
		for _, f := range p.FullOutputs {
			full[uimessagetest.FullKey(f.ToolCallID, f.Field)] = f.Text
		}
		if !p.Item || strings.Contains(string(p.Part), `-delta"`) || strings.Contains(string(p.Part), `"data-acp-info"`) {
			t.Fatalf("raw chunk stored: %.100s", p.Part)
		}
	}
	t.Logf("tee: %d chunks → %d rows / %d bytes", len(history), len(stored), bytes)
	if len(stored) > 600 {
		t.Fatalf("tee stored %d rows, want ≤ 600", len(stored))
	}

	want, err := uimessagetest.Merge(fixture)
	if err != nil {
		t.Fatal(err)
	}
	second := attachOnce(t, gw, secret, session)
	got, err := uimessagetest.Merge(rawMessages(second))
	if err != nil {
		t.Fatalf("reattach stream does not merge: %v", err)
	}
	if len(got) != len(want) {
		t.Fatalf("reattach rebuilt %d parts, want %d (no duplicate live tail)", len(got), len(want))
	}
	for i := range want {
		if want[i]["state"] == "streaming" {
			got[i]["state"] = "streaming"
		}
		// A clipped output replays flagged; its tee-stored full copy restores it.
		if _, err := uimessagetest.RestoreFullOutput(got[i], full); err != nil {
			t.Fatalf("part %d: %v", i, err)
		}
		if fmt.Sprint(want[i]) != fmt.Sprint(got[i]) {
			t.Fatalf("part %d differs after reattach", i)
		}
	}
	if again, _ := st.AgentSessionTranscript(context.Background(), session, -1, 1<<30, 0); len(again) != len(stored) {
		t.Fatalf("reattach stored %d more rows", len(again)-len(stored))
	}
}

// The driver stream ending inside an item (a lost pod, a cut connection)
// persists the in-flight item as partial so the Completer can replace it.
func TestLiveTeeFlushesInFlightItemsAsPartial(t *testing.T) {
	secret := []byte("shell-ticket-secret")
	session := "ags-000000000000000000004"
	st := newFakeAttachStore()
	driver, host, port := fakeDriver([]string{
		`{"type":"start"}`,
		`{"type":"text-start","id":"t"}`,
		`{"type":"text-delta","id":"t","delta":"half an ans"}`,
	})
	defer driver.Close()
	attachOnce(t, newAttachGateway(st, fixedPodIP{ip: host}, secret, port), secret, session)
	stored, _ := st.AgentSessionTranscript(context.Background(), session, -1, 1<<30, 0)
	if len(stored) != 2 || stored[0].Partial || !stored[1].Partial || stored[1].PartIndex != 1 ||
		string(stored[1].Part) != `{"type":"text","id":"t","text":"half an ans","state":"streaming"}` {
		t.Fatalf("stored = %+v", stored)
	}
}

// A reattach suppresses exactly the live chunks it already replayed: by owning
// item for an item-model turn (a chunk of an unreplayed item is forwarded even
// below the highest replayed index), by ordinal prefix for a legacy raw-chunk
// turn. Standalone rows (start/finish/error) are valid in both shapes and do
// not make a turn legacy.
func TestReplayedTurnSuppressesWhatTheReplayDelivered(t *testing.T) {
	items := newReplayedTurn()
	items.add(store.AgentSessionTranscriptPart{PartIndex: 0, Part: json.RawMessage(`{"type":"start"}`)})
	items.add(store.AgentSessionTranscriptPart{PartIndex: 3, Item: true, Part: json.RawMessage(`{"type":"text","id":"t","text":"x","state":"done"}`)})
	if items.legacy {
		t.Fatal("a start row made the turn legacy")
	}
	for _, c := range []struct {
		ordinal, owner int64
		want           bool
	}{
		{4, 3, true},   // a chunk of the replayed item
		{2, 1, false},  // an item the replay did not deliver (still open when stored)
		{2, -1, true},  // a never-stored chunk inside the replayed prefix
		{9, -1, false}, // a never-stored chunk past it
	} {
		if got := items.suppress(c.ordinal, c.owner); got != c.want {
			t.Errorf("item turn suppress(%d, %d) = %v, want %v", c.ordinal, c.owner, got, c.want)
		}
	}

	legacy := newReplayedTurn()
	for i, p := range []string{`{"type":"start"}`, `{"type":"text-start","id":"t"}`, `{"type":"text-delta","id":"t","delta":"x"}`} {
		legacy.add(store.AgentSessionTranscriptPart{PartIndex: int64(i), Part: json.RawMessage(p)})
	}
	if !legacy.legacy || !legacy.suppress(2, 1) || legacy.suppress(3, 1) {
		t.Fatalf("legacy turn: legacy=%v suppress(2)=%v suppress(3)=%v, want the ordinal prefix", legacy.legacy, legacy.suppress(2, 1), legacy.suppress(3, 1))
	}
}
