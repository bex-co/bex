package agentsessions

import (
	"sync"
	"testing"
	"time"
)

// Held waits are cheap (ADR094 Consequences): however many waiters a pod holds,
// they share ONE listener, and one notification releases every waiter on the
// changed session. This pins both, at a waiter count well beyond what the
// per-caller rate limit lets a single client hold.
func TestManyHeldWaitsShareOneListenerAndAllWake(t *testing.T) {
	const waiters = 1000
	svc, st, ids := waitFixture(t, PhaseRunning)
	var mu sync.Mutex

	var wg sync.WaitGroup
	results := make(chan WaitResult, waiters)
	for i := 0; i < waiters; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			res, err := svc.Wait(caller("alice"), WaitRequest{IDs: ids, TimeoutSeconds: 20}, nil)
			if err != nil {
				t.Error(err)
				return
			}
			results <- res
		}()
	}
	// Every waiter subscribes to the bus before it blocks.
	deadline := time.Now().Add(10 * time.Second)
	for {
		svc.bus().mu.Lock()
		held := len(svc.bus().subs)
		svc.bus().mu.Unlock()
		if held == waiters {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("only %d of %d waiters are held", held, waiters)
		}
		time.Sleep(10 * time.Millisecond)
	}
	st.listenMu.Lock()
	listeners := len(st.stateListeners)
	st.listenMu.Unlock()
	if listeners != 1 {
		t.Fatalf("%d waiters opened %d listeners, want exactly 1 per pod", waiters, listeners)
	}

	setPhase(st, &mu, ids[0], PhaseCompleted)
	st.notifyState(ids[0])
	done := make(chan struct{})
	go func() { wg.Wait(); close(done) }()
	select {
	case <-done:
	case <-time.After(15 * time.Second):
		t.Fatalf("not every waiter woke on one notification (%d returned)", len(results))
	}
	close(results)
	for res := range results {
		if res.TimedOut || res.Sessions[0].State != StateSucceeded {
			t.Fatalf("a waiter returned %+v", res)
		}
	}
	svc.bus().mu.Lock()
	left := len(svc.bus().subs)
	svc.bus().mu.Unlock()
	if left != 0 {
		t.Fatalf("%d subscriptions leaked after the waits returned", left)
	}
}
