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
	"bufio"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/store"
)

func TestTranscriptBatcherFlushesBoundedBatches(t *testing.T) {
	ctx := context.Background()
	st := newFakeAttachStore()
	batcher := newTranscriptBatcher(ctx, st, "ags-test")
	for i := range 100 {
		batcher.enqueue(store.AgentSessionTranscriptPart{
			Turn: 1, PartIndex: int64(i), Part: []byte(fmt.Sprintf(`{"i":%d}`, i)),
		})
	}
	batcher.close()
	batches := st.appendBatches.Load()
	if batches > 4 || batches < 3 {
		t.Fatalf("100 parts produced %d append batches, want ~3-4 bounded batches", batches)
	}
	if got := len(st.parts["ags-test"]); got != 100 {
		t.Fatalf("stored parts = %d, want 100", got)
	}
}

func TestForwardAgentTurnPersistsInBoundedBatches(t *testing.T) {
	secret := []byte("shell-ticket-secret")
	session := "ags-00000000000000000000f"
	st := newFakeAttachStore()
	parts := make([]string, 64)
	for i := range parts {
		parts[i] = fmt.Sprintf(`{"type":"text-delta","delta":"%d"}`, i)
	}
	driver, host, port, _ := fakeTurnDriver(parts)
	defer driver.Close()

	gw := newAttachGateway(st, fixedPodIP{ip: host}, secret, port)
	srv := httptest.NewServer(gw.Handler())
	defer srv.Close()
	req, _ := http.NewRequest(http.MethodPost, srv.URL, strings.NewReader(`{"prompt":"go"}`))
	req.Header.Set(TicketHeader, turnTicket(t, secret, session))
	resp, err := (&http.Client{Timeout: 2 * time.Second}).Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	reader := bufio.NewReader(resp.Body)
	for {
		_, readErr := reader.ReadString('\n')
		if readErr == io.EOF {
			break
		}
	}
	time.Sleep(50 * time.Millisecond)
	batches := st.appendBatches.Load()
	if batches == 0 || batches >= int32(len(parts)) {
		t.Fatalf("64-part turn produced %d append batches, want bounded << 64", batches)
	}
	if got := len(st.parts[session]); got != len(parts) {
		t.Fatalf("stored parts = %d, want %d", got, len(parts))
	}
}
