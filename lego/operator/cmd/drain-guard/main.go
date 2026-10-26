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

// bex-drain-guard holds a CAPI pre-drain hook on the tenant pool's Machines
// and, when one is deleted or Cluster Autoscaler marks its node for removal,
// hands its single-instance Apps over to another node before the drain
// (w1/m186, w1/m187, internal/drainguard). It is deliberately not part
// of the operator: it runs as its own narrowly scoped ServiceAccount so the
// operator never gains rights in the locked-down CAPI namespace.
package main

import (
	"flag"
	"fmt"
	"os"
	"time"

	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/clientcmd"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/cluster"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"

	"github.com/bex-co/bex/lego/operator/internal/drainguard"
)

var scheme = runtime.NewScheme()

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
}

// options are drain-guard's flags.
type options struct {
	namespace, selector, capiKubeconfig, probeAddr string
	maxHold                                        time.Duration
}

func main() {
	var o options
	flag.StringVar(&o.namespace, "machine-namespace", "bex-capi", "namespace of the CAPI Machines to guard")
	flag.StringVar(&o.selector, "machine-selector", "", "label selector of the Machines to guard (required)")
	flag.StringVar(&o.capiKubeconfig, "capi-kubeconfig", "",
		"kubeconfig of the cluster that holds the Machines; empty means the cluster drain-guard runs in")
	flag.DurationVar(&o.maxHold, "max-hold", drainguard.DefaultMaxHold, "longest a deleting Machine's drain is held")
	flag.StringVar(&o.probeAddr, "health-probe-bind-address", ":8081", "address of the health probe endpoint")
	zapOpts := zap.Options{}
	zapOpts.BindFlags(flag.CommandLine)
	flag.Parse()
	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&zapOpts)))

	if err := run(o); err != nil {
		fmt.Fprintf(os.Stderr, "drain-guard: %v\n", err)
		os.Exit(1)
	}
}

func run(o options) error {
	if o.selector == "" {
		// An empty selector would hook every Machine, the control plane's
		// included; guarding is always an explicit choice of pool.
		return fmt.Errorf("--machine-selector is required")
	}
	sel, err := labels.Parse(o.selector)
	if err != nil {
		return fmt.Errorf("--machine-selector: %w", err)
	}
	workloadCfg, err := ctrl.GetConfig()
	if err != nil {
		return fmt.Errorf("kubeconfig: %w", err)
	}
	capiCfg := workloadCfg
	if o.capiKubeconfig != "" {
		if capiCfg, err = clientcmd.BuildConfigFromFlags("", o.capiKubeconfig); err != nil {
			return fmt.Errorf("--capi-kubeconfig: %w", err)
		}
	}
	mgr, err := ctrl.NewManager(capiCfg, ctrl.Options{
		Scheme:                 scheme,
		Metrics:                metricsserver.Options{BindAddress: "0"},
		HealthProbeBindAddress: o.probeAddr,
		// The Role grants Machines in this namespace only.
		Cache: cache.Options{DefaultNamespaces: map[string]cache.Config{o.namespace: {}}},
		// Machines are unstructured, which the manager's client reads live by
		// default; serve them from the watch's cache like typed objects.
		Client: client.Options{Cache: &client.CacheOptions{Unstructured: true}},
	})
	if err != nil {
		return fmt.Errorf("manager: %w", err)
	}
	if err := mgr.AddHealthzCheck("ping", healthz.Ping); err != nil {
		return err
	}
	// Uncached: drain-guard reads pods and Deployments only while a node is
	// leaving, so a cluster-wide watch would cost more than it saves.
	workload, err := client.New(workloadCfg, client.Options{Scheme: scheme})
	if err != nil {
		return fmt.Errorf("workload client: %w", err)
	}
	// The workload cluster's cache serves the one watch drain-guard needs there:
	// Nodes, for Cluster Autoscaler's removal taint.
	workloadCluster, err := cluster.New(workloadCfg, func(co *cluster.Options) { co.Scheme = scheme })
	if err != nil {
		return fmt.Errorf("workload cluster: %w", err)
	}
	if err := mgr.Add(workloadCluster); err != nil {
		return err
	}
	r := &drainguard.Reconciler{Machines: mgr.GetClient(), Workload: workload, Selector: sel, MaxHold: o.maxHold}
	if err := r.SetupWithManager(mgr, workloadCluster.GetCache()); err != nil {
		return err
	}
	ctrl.Log.WithName("drain-guard").Info("guarding Machines",
		"namespace", o.namespace, "selector", o.selector, "maxHold", o.maxHold.String())
	return mgr.Start(ctrl.SetupSignalHandler())
}
