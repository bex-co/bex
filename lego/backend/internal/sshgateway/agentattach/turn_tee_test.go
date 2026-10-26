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
	for _, p := range stored {
		bytes += len(p.Part)
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
