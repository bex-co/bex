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

package main

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
)

// w5/m370 t003: a rollout's next manager takes over within the election's
// retry period, not after the old leader's 15 s lease runs out. Production
// measured 14 to 21 s with no App reconciled at every operator rollout.
func TestTheNextManagerLeadsSoonAfterTheLeaderStops(t *testing.T) {
	assets := os.Getenv("KUBEBUILDER_ASSETS")
	if matches, _ := filepath.Glob(filepath.Join("..", "..", "bin", "k8s", "*")); assets == "" && len(matches) > 0 {
		assets = matches[0]
	}
	if assets == "" {
		t.Skip("no envtest binaries: run through `make test`")
	}
	env := &envtest.Environment{BinaryAssetsDirectory: assets}
	cfg, err := env.Start()
	if err != nil {
		t.Fatalf("envtest: %v", err)
	}
	t.Cleanup(func() { _ = env.Stop() })

	start := func(name string) (ctrl.Manager, context.CancelFunc, <-chan error) {
		t.Helper()
		opts := withLeaderElection(ctrl.Options{
			Metrics:                 metricsserver.Options{BindAddress: "0"},
			HealthProbeBindAddress:  "0",
			LeaderElectionNamespace: "default",
		}, true)
		mgr, err := ctrl.NewManager(rest.CopyConfig(cfg), opts)
		if err != nil {
			t.Fatalf("%s: new manager: %v", name, err)
		}
		ctx, cancel := context.WithCancel(context.Background())
		done := make(chan error, 1)
		go func() { done <- mgr.Start(ctx) }()
		t.Cleanup(cancel)
		return mgr, cancel, done
	}

	leader, stopLeader, leaderDone := start("leader")
	select {
	case <-leader.Elected():
	case <-time.After(30 * time.Second):
		t.Fatal("the first manager never became leader")
	}
	next, _, _ := start("next")
	select {
	case <-next.Elected():
		t.Fatal("the next manager led while the leader still held the lease")
	case <-time.After(3 * time.Second):
	}

	stopLeader()
	select {
	case err := <-leaderDone:
		if err != nil {
			t.Fatalf("the leader stopped with %v", err)
		}
	case <-time.After(30 * time.Second):
		t.Fatal("the leader did not stop")
	}
	stopped := time.Now()
	select {
	case <-next.Elected():
		// The election retries every 2 s; the lease runs 15 s.
		if took := time.Since(stopped); took > 6*time.Second {
			t.Fatalf("the next manager led %s after the leader stopped, want within the 2 s retry period", took)
		}
	case <-time.After(30 * time.Second):
		t.Fatal("the next manager never became leader")
	}
}
