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
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/synctest"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/store"
)

type batchTestStore struct {
	*fakeAttachStore
	append func(context.Context, []store.AgentSessionTranscriptPart) error
}

func (s *batchTestStore) AppendAgentSessionTranscript(ctx context.Context, id string, parts []store.AgentSessionTranscriptPart) error {
	if s.append != nil {
		if err := s.append(ctx, parts); err != nil {
			return err
		}
	}
	return s.fakeAttachStore.AppendAgentSessionTranscript(ctx, id, parts)
}
func transcriptPart(i int) store.AgentSessionTranscriptPart {
	return store.AgentSessionTranscriptPart{Turn: 1, PartIndex: int64(i), Part: []byte(fmt.Sprintf(`{"type":"text-delta","delta":"%d"}`, i))}
}
func assertTranscriptOrder(t *testing.T, st *fakeAttachStore, n int) {
	t.Helper()
	parts, err := st.AgentSessionTranscript(context.Background(), "ags-test", -1, 1<<30, 0)
	if err != nil || len(parts) != n {
		t.Fatalf("parts=%d error=%v, want %d", len(parts), err, n)
	}
	for i, p := range parts {
		if p.PartIndex != int64(i) || string(p.Part) != string(transcriptPart(i).Part) {
			t.Fatalf("part %d: %+v", i, p)
		}
	}
}

func TestTranscriptBatcherDenseOperationCounts(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		st := newFakeAttachStore()
		b := newTranscriptBatcher(context.Background(), st, "ags-test")
		for i := range 100 {
			b.enqueue(transcriptPart(i))
		}
		b.close()
		if got := st.appendBatches.Load(); got != 4 {
			t.Fatalf("100 parts: %d transactions, want 4", got)
		}
		assertTranscriptOrder(t, st, 100)
		t.Log("100 parts: original per-part tee=100 append transactions; ordered batch tee=4; bytes and ordinals unchanged")
	})
}

func TestTranscriptBatcherSparseFlushAndCanceledClose(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		st := &batchTestStore{fakeAttachStore: newFakeAttachStore(), append: func(ctx context.Context, _ []store.AgentSessionTranscriptPart) error { return ctx.Err() }}
		b := newTranscriptBatcher(ctx, st, "ags-test")
		b.enqueue(transcriptPart(0))
		synctest.Wait()
		if st.appendBatches.Load() != 0 {
			t.Fatal("sparse part flushed before interval")
		}
		time.Sleep(defaultTranscriptFlushInterval)
		synctest.Wait()
		if st.appendBatches.Load() != 1 {
			t.Fatal("sparse part not durable at interval")
		}
		b.enqueue(transcriptPart(1))
		cancel()
		b.close()
		b.close()
		assertTranscriptOrder(t, st.fakeAttachStore, 2)
	})
}

func TestTranscriptBatcherBoundsOutstandingBytes(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		release := make(chan struct{})
		started := make(chan struct{}, 1)
		st := &batchTestStore{fakeAttachStore: newFakeAttachStore(), append: func(ctx context.Context, _ []store.AgentSessionTranscriptPart) error {
			select {
			case started <- struct{}{}:
			default:
			}
			select {
			case <-release:
				return nil
			case <-ctx.Done():
				return ctx.Err()
			}
		}}
		b := newTranscriptBatcher(context.Background(), st, "ags-test")
		part := transcriptPart(0)
		part.Part = []byte(strings.Repeat("x", maxSSEPartBytes))
		b.enqueue(part)
		<-started
		part.PartIndex = 1
		b.enqueue(part)
		queued := make(chan struct{})
		go func() { part.PartIndex = 2; b.enqueue(part); close(queued) }()
		synctest.Wait()
		select {
		case <-queued:
			t.Fatal("accepted more than 2MiB while append blocked")
		default:
		}
		close(release)
		<-queued
		b.close()
		if got := st.appendBatches.Load(); got != 3 {
			t.Fatalf("appends=%d", got)
		}
	})
}

func TestTranscriptBatcherFailureLeavesOrderedSuffixForHarvest(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		attempts := 0
		st := &batchTestStore{fakeAttachStore: newFakeAttachStore(), append: func(context.Context, []store.AgentSessionTranscriptPart) error {
			attempts++
			if attempts == 2 {
				return errors.New("database unavailable")
			}
			return nil
		}}
		b := newTranscriptBatcher(context.Background(), st, "ags-test")
		all := make([]store.AgentSessionTranscriptPart, 100)
		for i := range all {
			all[i] = transcriptPart(i)
			b.enqueue(all[i])
		}
		b.close()
		if attempts != 2 {
			t.Fatalf("append attempts=%d, want stop after failed batch", attempts)
		}
		assertTranscriptOrder(t, st.fakeAttachStore, 32)
		// Completer harvest appends the full turn, idempotently filling its suffix.
		if err := st.fakeAttachStore.AppendAgentSessionTranscript(context.Background(), "ags-test", all); err != nil {
			t.Fatal(err)
		}
		assertTranscriptOrder(t, st.fakeAttachStore, 100)
	})
}

func TestTranscriptBatcherAppendTimeoutBoundsClose(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		st := &batchTestStore{fakeAttachStore: newFakeAttachStore(), append: func(ctx context.Context, _ []store.AgentSessionTranscriptPart) error { <-ctx.Done(); return ctx.Err() }}
		b := newTranscriptBatcher(context.Background(), st, "ags-test")
		b.enqueue(transcriptPart(0))
		start := time.Now()
		b.close()
		if elapsed := time.Since(start); elapsed != transcriptAppendTimeout {
			t.Fatalf("close elapsed=%v", elapsed)
		}
	})
}

type transcriptTransport func(*http.Request) (*http.Response, error)

func (f transcriptTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestForwardAgentTurnForwardsBeforeAppendAndFlushesBeforeDone(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		release := make(chan struct{})
		appending := make(chan struct{})
		st := &batchTestStore{fakeAttachStore: newFakeAttachStore(), append: func(ctx context.Context, _ []store.AgentSessionTranscriptPart) error {
			close(appending)
			select {
			case <-release:
				return nil
			case <-ctx.Done():
				return ctx.Err()
			}
		}}
		payload := string(transcriptPart(0).Part)
		server := &Server{Store: st, Secret: []byte("test-secret"), HTTPClient: &http.Client{Transport: transcriptTransport(func(*http.Request) (*http.Response, error) {
			return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader("data: " + payload + "\n\ndata: [DONE]\n\n"))}, nil
		})}}
		w := httptest.NewRecorder()
		sse := server.startAgentSSE(w, w)
		finished := make(chan struct{})
		go func() {
			server.forwardAgentTurn(context.Background(), sse, "127.0.0.1", "ags-test", 1, strings.NewReader(`{"prompt":"go"}`))
			close(finished)
		}()
		<-appending
		synctest.Wait()
		if w.Body.String() != "data: "+payload+"\n\n" {
			t.Fatalf("first frame blocked or terminal before persistence: %q", w.Body.String())
		}
		close(release)
		<-finished
		if w.Body.String() != "data: "+payload+"\n\ndata: [DONE]\n\n" {
			t.Fatalf("wire changed: %q", w.Body.String())
		}
		assertTranscriptOrder(t, st.fakeAttachStore, 1)
		t.Log("first frame observable while append blocked; [DONE] observable only after terminal flush")
	})
}

type transcriptReplayStore struct {
	*fakeAttachStore
	afterReplay func()
}

func (s *transcriptReplayStore) AgentSessionTranscript(ctx context.Context, id string, afterSeq, maxBytes int64, limit int) ([]store.AgentSessionTranscriptPart, error) {
	parts, err := s.fakeAttachStore.AgentSessionTranscript(ctx, id, afterSeq, maxBytes, limit)
	if len(parts) == 0 && s.afterReplay != nil {
		s.afterReplay()
		s.afterReplay = nil
	}
	return parts, err
}

func TestLiveSpliceChargesReplayOnceAndKeepsConcurrentSuffix(t *testing.T) {
	for _, concurrent := range []bool{false, true} {
		t.Run(fmt.Sprintf("concurrent=%t", concurrent), func(t *testing.T) {
			st := &transcriptReplayStore{fakeAttachStore: newFakeAttachStore()}
			if err := st.AppendAgentSessionTranscript(context.Background(), "ags-test", []store.AgentSessionTranscriptPart{transcriptPart(0)}); err != nil {
				t.Fatal(err)
			}
			if concurrent {
				st.afterReplay = func() {
					if err := st.AppendAgentSessionTranscript(context.Background(), "ags-test", []store.AgentSessionTranscriptPart{transcriptPart(1)}); err != nil {
						t.Fatal(err)
					}
				}
			}
			var wire strings.Builder
			var quota int64
			for i := range 3 {
				payload := string(transcriptPart(i).Part)
				fmt.Fprintf(&wire, "data: %s\n\n", payload)
				quota += int64(len(payload))
			}
			wire.WriteString("data: [DONE]\n\n")
			server := &Server{Store: st, MaxTranscriptBytes: quota, HTTPClient: &http.Client{Transport: transcriptTransport(func(*http.Request) (*http.Response, error) {
				return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(wire.String()))}, nil
			})}}
			w := httptest.NewRecorder()
			server.streamAgentAttach(context.Background(), server.startAgentSSE(w, w), "127.0.0.1", nil, "ags-test", 1)
			if w.Body.String() != wire.String() {
				t.Fatalf("replay/splice lost or duplicated bytes: got %q want %q", w.Body.String(), wire.String())
			}
			assertTranscriptOrder(t, st.fakeAttachStore, 3)
		})
	}
}
