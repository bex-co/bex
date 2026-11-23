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

package controller

import (
	"context"
	"errors"
	"fmt"
	"net/netip"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	ctrlmetrics "sigs.k8s.io/controller-runtime/pkg/metrics"

	"github.com/bex-co/bex/lego/operator/internal/listeners"
	"github.com/bex-co/bex/lego/operator/internal/portscan"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// Reading a pod's listeners (w6/m530). Private-port discovery and bound-port
// detection ask the listener reader on the pod's node (cmd/listener-reader,
// config/listener-reader) which TCP ports the pod listens on, read from the
// pod's own socket table. The connect sweep (internal/portscan) put up to
// 65,535 conntrack entries on the operator's node and on the swept pod's node
// per scan, and evicted every other pod's idle connections there (w6/153,
// w6/149). It now runs only for a node that has no reader pod at all, logged
// once per node and interval and counted, so a sweep is never silent. A node
// whose reader is not Ready, or whose read fails, is not swept: the scan
// fails, the last record stands, and the next interval asks again.

// ListenerSource reads the TCP ports a pod accepts connections on at its own
// address, and apart those it listens on only at loopback (w8/388).
// errNoListenerReader means the pod's node has no reader to ask.
type ListenerSource interface {
	PodListeners(ctx context.Context, pod *corev1.Pod) (listeners.Answer, error)
}

var (
	errNoListenerReader       = errors.New("the pod's node runs no listener reader")
	errListenerReaderNotReady = errors.New("the node's listener reader is not Ready")
	errNoListenerSource       = errors.New("no listener reader and no port scanner")
)

const (
	// listenerReaderComponent is the reader pods' LabelComponent value.
	listenerReaderComponent = "listener-reader"
	// listenerReadTimeout bounds one read from a node's reader, the reader's
	// own bound on it; a sweep keeps portDetectTimeout.
	listenerReadTimeout = listeners.ReadTimeout
	// listenerSweepLogEvery spaces the log line a node without a reader
	// writes, so a node that lost its reader is visible but not noisy.
	listenerSweepLogEvery = privatePortScanInterval
)

// listenerReads counts every read of a pod's listeners by where it came from.
// source="sweep" is a connect sweep of a pod whose node has no reader pod.
var listenerReads = prometheus.NewCounterVec(prometheus.CounterOpts{
	Name: "bex_port_listener_reads_total",
	Help: "Reads of a tenant pod's listening ports, by source (reader: the pod's node's listener reader; sweep: a connect to every port, only where the node has no reader pod) and result.",
}, []string{"source", "result"})

func init() {
	ctrlmetrics.Registry.MustRegister(listenerReads)
}

// readsListeners reports whether the reconciler can read listeners at all.
func (r *AppReconciler) readsListeners() bool {
	return r.Listeners != nil || r.PortScanner != nil
}

// podListeners is the TCP ports pod listens on: the answer of the reader on
// its node, complete, or, only when that node runs no reader pod, sweep's
// answer for the pod's address, which may be partial (portscan.ErrPartial)
// and names no loopback port.
func (r *AppReconciler) podListeners(ctx context.Context, pod *corev1.Pod, sweep func(context.Context, string) ([]int, error)) (listeners.Answer, error) {
	if r.Listeners != nil {
		read, readErr := r.readerListeners(ctx, pod)
		if !errors.Is(readErr, errNoListenerReader) {
			return read, readErr
		}
		r.noteListenerSweep(ctx, pod.Spec.NodeName)
	}
	if r.PortScanner == nil {
		return listeners.Answer{}, errNoListenerSource
	}
	scanCtx, cancel := context.WithTimeout(ctx, portDetectTimeout)
	defer cancel()
	open, err := sweep(scanCtx, pod.Status.PodIP)
	listenerReads.WithLabelValues("sweep", sweepResult(err)).Inc()
	return listeners.Answer{Ports: open}, err
}

// readerListeners is the answer of the reader on pod's node alone, never a
// sweep: only a reader sees loopback (w8/388). errNoListenerReader when that
// node runs no reader pod. The caller checks r.Listeners is set.
func (r *AppReconciler) readerListeners(ctx context.Context, pod *corev1.Pod) (listeners.Answer, error) {
	readCtx, cancel := context.WithTimeout(ctx, listenerReadTimeout)
	read, err := r.Listeners.PodListeners(readCtx, pod)
	cancel()
	if errors.Is(err, errNoListenerReader) {
		return listeners.Answer{}, err
	}
	listenerReads.WithLabelValues("reader", readerResult(err)).Inc()
	if err != nil {
		return listeners.Answer{}, fmt.Errorf("reading listeners on node %s: %w", pod.Spec.NodeName, err)
	}
	return read, nil
}

// fullSweep connects to every port of ip, for a node without a reader. It is a
// method rather than the method value r.PortScanner.Scan, which would panic on
// a nil PortScanner before podListeners checks it.
func (r *AppReconciler) fullSweep(ctx context.Context, ip string) ([]int, error) {
	return r.PortScanner.Scan(ctx, ip)
}

// noteListenerSweep logs, once per node and listenerSweepLogEvery, that a
// pod on node is being swept because the node has no reader.
func (r *AppReconciler) noteListenerSweep(ctx context.Context, node string) {
	now := time.Now()
	if last, logged := r.listenerSweepLogged.LoadOrStore(node, now); logged {
		// Two sweeps of one node at once: only the one that moves the stamp logs.
		if now.Sub(last.(time.Time)) < listenerSweepLogEvery || !r.listenerSweepLogged.CompareAndSwap(node, last, now) {
			return
		}
	}
	logf.FromContext(ctx).Info("no listener reader on the pod's node; sweeping the pod's ports instead", "node", node)
}

func readerResult(err error) string {
	switch {
	case err == nil:
		return "ok"
	case errors.Is(err, listeners.ErrPodNotFound):
		return "pod_not_found"
	case errors.Is(err, listeners.ErrAddressMismatch):
		return "address_mismatch"
	case errors.Is(err, errListenerReaderNotReady):
		return "reader_not_ready"
	}
	return "failed"
}

func sweepResult(err error) string {
	switch {
	case err == nil:
		return "ok"
	case errors.Is(err, portscan.ErrPartial):
		return "partial"
	case errors.Is(err, portscan.ErrUnreachable):
		return "unreachable"
	}
	return "failed"
}

// NodeListenerReader asks the listener reader on a pod's node: a Ready pod of
// the config/listener-reader DaemonSet in Namespace on the pod's node.
type NodeListenerReader struct {
	// Pods lists the reader pods; the manager's cache.
	Pods client.Reader
	// Namespace is where the DaemonSet runs (the operator's own namespace).
	Namespace string
	// HTTP talks to the readers.
	HTTP listeners.Client
	// Port overrides listeners.Port, for tests.
	Port uint16
}

// PodListeners reads pod's listeners from its node's reader. A node with no
// reader pod is errNoListenerReader; one whose reader pods are none Ready is
// errListenerReaderNotReady.
//
// Of a node's Ready readers, it asks one the DaemonSet is not deleting. A
// roll surges the new reader Ready before it deletes the old one, which stays
// Running and Ready in the cache for a moment after its port closes, so asking
// it failed one read at each roll (w6/166). A terminating reader is asked only
// when it is the node's only Ready one.
func (n *NodeListenerReader) PodListeners(ctx context.Context, pod *corev1.Pod) (listeners.Answer, error) {
	node := pod.Spec.NodeName
	if node == "" || pod.Status.PodIP == "" {
		return listeners.Answer{}, errors.New("the pod has no node or address yet")
	}
	var readers corev1.PodList
	if err := n.Pods.List(ctx, &readers, client.InNamespace(n.Namespace),
		client.MatchingLabels{appv1alpha1.LabelComponent: listenerReaderComponent}); err != nil {
		return listeners.Answer{}, fmt.Errorf("listing listener readers: %w", err)
	}
	var (
		present bool
		reader  *corev1.Pod
		addr    netip.Addr
	)
	for i := range readers.Items {
		candidate := &readers.Items[i]
		if candidate.Spec.NodeName != node {
			continue
		}
		present = true
		ip, err := netip.ParseAddr(candidate.Status.PodIP)
		if err != nil || candidate.Status.Phase != corev1.PodRunning || !podReady(candidate) {
			continue
		}
		if reader == nil || reader.DeletionTimestamp != nil {
			reader, addr = candidate, ip
		}
	}
	if reader == nil {
		if !present {
			return listeners.Answer{}, errNoListenerReader
		}
		return listeners.Answer{}, errListenerReaderNotReady
	}
	port := n.Port
	if port == 0 {
		port = listeners.Port
	}
	return n.HTTP.Read(ctx, netip.AddrPortFrom(addr, port), string(pod.UID), pod.Status.PodIP)
}
