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

// Package listeners reads the TCP ports a pod listens on from its node, without
// connecting to it (w6/m530). Render "scans the running container for an HTTP
// listener on 0.0.0.0" (w8/177); the operator used to answer that with a TCP
// connect to every port of the pod (internal/portscan), and each sweep put up
// to 65,535 entries into the Cilium conntrack table of the operator's node and
// of the swept pod's node, evicting the idle connections of every other pod
// there (w6/153, w6/149).
//
// A pod's sockets are in /proc/<pid>/net/tcp and tcp6 of any of its processes,
// read from the node's PID namespace: every container of a pod, its pause
// process included, shares the pod's network namespace. The table names each
// listener's local address, so "accepts a connection on the pod address" is
// answered as the connect answered it: 0.0.0.0, ::, the pod address or another
// non-loopback address count, 127.0.0.0/8, ::1 and ::ffff:127.0.0.0/104 do
// not. Those are answered apart: Render names a scan that finds a listener
// only on localhost (w8/388). The reader needs the node's PID namespace and
// nothing else: /proc/<pid>/cgroup and net/* are world-readable, while
// environ and ns/* stay refused.
//
// This package is the node agent's half (Read, Handler, cmd/listener-reader)
// and the operator's client of it (Client).
package listeners

import (
	"bufio"
	"bytes"
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/netip"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/bex-co/bex/lego/operator/internal/procnet"
)

var (
	// ErrPodNotFound means no process on this node belongs to the pod: it
	// runs on another node, or it is gone.
	ErrPodNotFound = errors.New("listeners: no process of the pod runs on this node")
	// ErrAddressMismatch means the pod's processes are here but their network
	// namespace does not hold the pod's address, so their sockets are not the
	// ones the address reaches.
	ErrAddressMismatch = errors.New("listeners: the pod's network namespace does not hold its address")
	// ErrUnreadable means the pod's processes are here but none of their
	// socket tables could be read (each exited mid-read, or the kernel
	// refused).
	ErrUnreadable = errors.New("listeners: the pod's socket tables could not be read")
	// ErrBadRequest means the pod UID or address is malformed.
	ErrBadRequest = errors.New("listeners: malformed pod UID or address")
)

// podUIDPattern is a Kubernetes pod UID: an RFC 4122 UUID, lowercase.
var podUIDPattern = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)

// errOtherProcess means a /proc entry is not one of the pod's processes.
var errOtherProcess = errors.New("listeners: not the pod's process")

// Read returns the TCP ports the pod with podUID listens on at an address other
// than loopback and, apart, those it listens on only at loopback, each
// ascending, read under procRoot, the node's /proc. podIP is the pod's address:
// the read refuses a namespace that does not hold it.
func Read(ctx context.Context, procRoot, podUID string, podIP netip.Addr) (Answer, error) {
	podIP = podIP.Unmap()
	if !podUIDPattern.MatchString(podUID) || !podIP.IsValid() || podIP.IsLoopback() || podIP.IsUnspecified() {
		return Answer{}, ErrBadRequest
	}
	entries, err := os.ReadDir(procRoot)
	if err != nil {
		return Answer{}, fmt.Errorf("%w: %w", ErrUnreadable, err)
	}
	match := cgroupMatcher(podUID)
	found, mismatched := false, false
	for _, entry := range entries {
		if err := ctx.Err(); err != nil {
			return Answer{}, err
		}
		if !isPID(entry.Name()) {
			continue
		}
		answer, err := readProcess(filepath.Join(procRoot, entry.Name()), match, podIP)
		switch {
		case err == nil:
			return answer, nil
		case errors.Is(err, errOtherProcess):
			continue
		case errors.Is(err, ErrAddressMismatch):
			found, mismatched = true, true
		default: // it exited between the listing and the read: try the next one
			found = true
		}
	}
	switch {
	case mismatched:
		return Answer{}, ErrAddressMismatch
	case found:
		return Answer{}, ErrUnreadable
	}
	return Answer{}, ErrPodNotFound
}

func isPID(name string) bool {
	if name == "" || name[0] == '0' {
		return false
	}
	for _, c := range name {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}

// readProcess reads one process's listeners when it is one of the pod's. Its
// /proc/<pid> directory is opened once and every file read through that
// handle: a handle on an exited process reads nothing, so a PID the kernel
// hands to another process mid-read can never answer for the pod.
func readProcess(dir string, inPod func([]byte) bool, podIP netip.Addr) (Answer, error) {
	root, err := os.OpenRoot(dir)
	if err != nil {
		return Answer{}, errOtherProcess
	}
	defer func() { _ = root.Close() }()
	cgroup, err := root.ReadFile("cgroup")
	if err != nil || !inPod(cgroup) {
		return Answer{}, errOtherProcess
	}
	local, err := holdsAddress(root, podIP)
	if err != nil {
		return Answer{}, err
	}
	if !local {
		return Answer{}, ErrAddressMismatch
	}
	return listening(root)
}

// cgroupMatcher reports whether a /proc/<pid>/cgroup file names the pod's
// cgroup. The kubelet names it pod<uid> under the cgroupfs driver and
// …-pod<uid with _ for ->.slice under the systemd driver, with a QoS segment
// (kubepods-burstable-…) or none (Guaranteed) and the node's own prefix
// (kind's kubelet-kubepods-…); a private cgroup namespace prefixes the path
// with /.. segments. Matching a whole path segment covers every form.
func cgroupMatcher(podUID string) func([]byte) bool {
	cgroupfs := "pod" + podUID
	systemd := "-pod" + strings.ReplaceAll(podUID, "-", "_") + ".slice"
	return func(cgroup []byte) bool {
		for line := range bytes.Lines(cgroup) {
			// hierarchy-ID:controller-list:path
			fields := strings.SplitN(strings.TrimRight(string(line), "\n"), ":", 3)
			if len(fields) != 3 {
				continue
			}
			for segment := range strings.SplitSeq(fields[2], "/") {
				if segment == cgroupfs || strings.HasSuffix(segment, systemd) {
					return true
				}
			}
		}
		return false
	}
}

// holdsAddress reports whether the process's network namespace has podIP as a
// local address: an IPv4 address is a /32 host LOCAL entry of its fib_trie, an
// IPv6 one a row of its if_inet6. ns/net, which would name the namespace, is
// refused without ptrace rights; the address proves the same thing.
func holdsAddress(root *os.Root, podIP netip.Addr) (bool, error) {
	name, parse := "net/fib_trie", fibTrieLocal
	if podIP.Is6() {
		name, parse = "net/if_inet6", ifInet6Local
	}
	f, err := root.Open(name)
	if err != nil {
		return false, fmt.Errorf("%w: %w", ErrUnreadable, err)
	}
	defer func() { _ = f.Close() }()
	return slices.Contains(parse(f), podIP), nil
}

// fibTrieLocal is the local IPv4 addresses of a /proc/net/fib_trie: each leaf
// line "|-- a.b.c.d" followed by a "/32 host LOCAL" entry.
func fibTrieLocal(r io.Reader) []netip.Addr {
	var local []netip.Addr
	var leaf netip.Addr
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "+-- ") {
			leaf = netip.Addr{} // a prefix node: its entries are on the leaves below
			continue
		}
		if rest, ok := strings.CutPrefix(line, "|-- "); ok {
			leaf, _ = netip.ParseAddr(rest)
			continue
		}
		if leaf.IsValid() && strings.HasPrefix(line, "/32 host LOCAL") {
			local = append(local, leaf)
		}
	}
	return local
}

// ifInet6Local is the addresses of a /proc/net/if_inet6: the first column of
// each row, 32 hex digits in network order.
func ifInet6Local(r io.Reader) []netip.Addr {
	var local []netip.Addr
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) == 0 || len(fields[0]) != 32 {
			continue
		}
		var b [16]byte
		if _, err := hex.Decode(b[:], []byte(fields[0])); err == nil {
			local = append(local, netip.AddrFrom16(b))
		}
	}
	return local
}

// listening reads the namespace's two TCP tables: Ports are those a
// connection to the pod's address could reach, anything bound to an address
// other than loopback, and Loopback those bound only to loopback. A missing
// tcp6 (IPv6 disabled) contributes nothing; a missing tcp means the process
// went away.
func listening(root *os.Root) (Answer, error) {
	var ports, loopback []int
	for _, name := range []string{"net/tcp", "net/tcp6"} {
		f, err := root.Open(name)
		if err != nil {
			if name == "net/tcp6" && errors.Is(err, os.ErrNotExist) {
				continue
			}
			return Answer{}, fmt.Errorf("%w: %w", ErrUnreadable, err)
		}
		for _, socket := range procnet.Listening(f) {
			if socket.Addr.IsLoopback() { // netip unmaps a 4-in-6 address itself
				loopback = append(loopback, socket.Port)
			} else {
				ports = append(ports, socket.Port)
			}
		}
		_ = f.Close()
	}
	slices.Sort(ports)
	ports = slices.Compact(ports)
	loopback = slices.DeleteFunc(loopback, func(p int) bool { _, reachable := slices.BinarySearch(ports, p); return reachable })
	slices.Sort(loopback)
	return Answer{Ports: ports, Loopback: slices.Compact(loopback)}, nil
}
