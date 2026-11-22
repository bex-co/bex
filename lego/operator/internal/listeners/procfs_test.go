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

package listeners

import (
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"net/netip"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

const (
	podUID   = "3f6c1d2e-8a4b-4c5d-9e0f-1a2b3c4d5e6f"
	otherUID = "aa6c1d2e-8a4b-4c5d-9e0f-1a2b3c4d5e70"
	podIP    = "10.244.1.5"
)

// process is one /proc/<pid> entry of a fake procfs. A nil table is a file
// that does not exist.
type process struct {
	pid     string
	cgroup  string
	fibTrie *string
	ifInet6 *string
	tcp     *string
	tcp6    *string
}

// fakeProc writes procs as a procfs tree and returns its root.
func fakeProc(t *testing.T, procs ...process) string {
	t.Helper()
	root := t.TempDir()
	for _, p := range procs {
		dir := filepath.Join(root, p.pid)
		if err := os.MkdirAll(filepath.Join(dir, "net"), 0o755); err != nil {
			t.Fatal(err)
		}
		write := func(name string, content *string) {
			if content == nil {
				return
			}
			if err := os.WriteFile(filepath.Join(dir, name), []byte(*content), 0o644); err != nil {
				t.Fatal(err)
			}
		}
		write("cgroup", &p.cgroup)
		write("net/fib_trie", p.fibTrie)
		write("net/if_inet6", p.ifInet6)
		write("net/tcp", p.tcp)
		write("net/tcp6", p.tcp6)
	}
	// Entries that are not processes.
	for _, name := range []string{"self", "net", "sys"} {
		if err := os.MkdirAll(filepath.Join(root, name), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

// fibTrie is a pod namespace's /proc/net/fib_trie holding loopback and the
// given addresses, as the kernel prints it (Main and Local tables, the subnet
// and broadcast leaves that are not local addresses included).
func fibTrie(local ...string) *string {
	var leaves strings.Builder
	for _, ip := range local {
		fmt.Fprintf(&leaves, "        |-- %s\n           /32 host LOCAL\n", ip)
	}
	table := `  +-- 0.0.0.0/0 3 0 5
     |-- 0.0.0.0
        /0 universe UNICAST
     +-- 10.244.1.0/24 2 0 2
        |-- 10.244.1.0
           /24 link UNICAST
` + leaves.String() + `     +-- 127.0.0.0/8 2 0 2
        +-- 127.0.0.0/31 1 0 0
           |-- 127.0.0.0
              /8 host LOCAL
           |-- 127.0.0.1
              /32 host LOCAL
        |-- 127.255.255.255
           /32 link BROADCAST
`
	return new("Main:\n" + table + "Local:\n" + table)
}

// hexAddr is addr as a /proc/net/tcp local address on this node.
func hexAddr(addr string) string {
	b := netip.MustParseAddr(addr).AsSlice()
	var out strings.Builder
	for i := 0; i < len(b); i += 4 {
		fmt.Fprintf(&out, "%08X", binary.NativeEndian.Uint32(b[i:i+4]))
	}
	return out.String()
}

// tcpTable is a /proc/net/tcp table with one LISTEN row per "addr:port" and an
// established connection, which never counts.
func tcpTable(listen ...string) *string {
	rows := make([]string, 0, len(listen)+2)
	rows = append(rows, "  sl  local_address rem_address   st tx_queue rx_queue tr tm->when retrnsmt   uid  timeout inode")
	for i, l := range listen {
		at := strings.LastIndex(l, ":")
		var port int
		_, _ = fmt.Sscanf(l[at+1:], "%d", &port)
		addr := hexAddr(l[:at])
		zero := strings.Repeat("0", len(addr))
		rows = append(rows, fmt.Sprintf("   %d: %s:%04X %s:0000 0A 00000000:00000000 00:00000000 00000000  1000        0 %d 1 0000000000000000 100 0 0 10 0", i, addr, port, zero, 100+i))
	}
	rows = append(rows, fmt.Sprintf("   %d: %s:2710 %s:D431 01 00000000:00000000 00:00000000 00000000  1000        0 9 1 0000000000000000 20 4 30 10 -1",
		len(listen), hexAddr(podIP), hexAddr("10.244.1.9")))
	return new(strings.Join(rows, "\n") + "\n")
}

// systemdCgroup is a cgroup v2 file under the systemd driver for qos
// ("besteffort", "burstable", or "" for Guaranteed).
func systemdCgroup(uid, qos string) string {
	under := strings.ReplaceAll(uid, "-", "_")
	if qos == "" {
		return "0::/kubepods.slice/kubepods-pod" + under + ".slice/cri-containerd-0123abcd.scope\n"
	}
	return "0::/kubepods.slice/kubepods-" + qos + ".slice/kubepods-" + qos + "-pod" + under + ".slice/cri-containerd-0123abcd.scope\n"
}

func podProcess(pid, cgroup string) process {
	return process{pid: pid, cgroup: cgroup, fibTrie: fibTrie(podIP),
		tcp:  tcpTable("0.0.0.0:10000", podIP+":6379", "127.0.0.1:8124"),
		tcp6: tcpTable("::1:9000", "::ffff:127.0.0.1:9001", ":::50051")}
}

func read(t *testing.T, root string) (Answer, error) {
	t.Helper()
	return Read(t.Context(), root, podUID, netip.MustParseAddr(podIP))
}

var wantPorts = []int{6379, 10000, 50051}

// Every cgroup path the kubelet writes for a pod names it: both drivers, each
// QoS class, a private cgroup namespace's /.. prefix, kind's kubelet- prefix
// and the cgroup v1 multi-hierarchy file.
func TestReadFindsThePodUnderEveryCgroupLayout(t *testing.T) {
	under := strings.ReplaceAll(podUID, "-", "_")
	for name, cgroup := range map[string]string{
		"systemd besteffort":   systemdCgroup(podUID, "besteffort"),
		"systemd burstable":    systemdCgroup(podUID, "burstable"),
		"systemd guaranteed":   systemdCgroup(podUID, ""),
		"cgroupfs besteffort":  "0::/kubepods/besteffort/pod" + podUID + "/0123abcd\n",
		"cgroupfs burstable":   "0::/kubepods/burstable/pod" + podUID + "/0123abcd\n",
		"cgroupfs guaranteed":  "0::/kubepods/pod" + podUID + "/0123abcd\n",
		"private namespace":    "0::/../../kubepods-burstable-pod" + under + ".slice/cri-containerd-0123abcd.scope\n",
		"kind kubelet prefix":  "0::/kubelet.slice/kubelet-kubepods.slice/kubelet-kubepods-besteffort.slice/kubelet-kubepods-besteffort-pod" + under + ".slice/cri-containerd-0123abcd.scope\n",
		"cgroup v1 systemd":    "12:pids:/kubepods.slice/kubepods-burstable.slice/kubepods-burstable-pod" + under + ".slice/cri-containerd-0123abcd.scope\n1:name=systemd:/kubepods.slice\n",
		"cgroup v1 cgroupfs":   "11:memory:/kubepods/burstable/pod" + podUID + "/0123abcd\n10:cpu,cpuacct:/kubepods/burstable/pod" + podUID + "/0123abcd\n",
		"no trailing new line": "0::/kubepods/pod" + podUID + "/0123abcd",
	} {
		t.Run(name, func(t *testing.T) {
			got, err := read(t, fakeProc(t, podProcess("4242", cgroup)))
			if err != nil || !slices.Equal(got.Ports, wantPorts) {
				t.Fatalf("Read = %v, %v; want %v", got, err, wantPorts)
			}
		})
	}
}

// Loopback listeners, IPv4, IPv6 and v4-mapped, are refused by a connection to
// the pod address, so they are not listeners; a listener bound to the pod
// address alone is. The established connection's port is never one. The
// loopback ones are answered apart (w8/388), less any port also bound off
// loopback.
func TestReadKeepsOnlyListenersOffLoopback(t *testing.T) {
	readAll := func(p process) Answer {
		t.Helper()
		answer, err := read(t, fakeProc(t, p))
		if err != nil {
			t.Fatalf("Read: %v", err)
		}
		return answer
	}
	got := readAll(process{pid: "7", cgroup: systemdCgroup(podUID, "burstable"), fibTrie: fibTrie(podIP),
		tcp:  tcpTable("127.0.0.1:8124", "127.0.0.53:53", podIP+":6379", "127.0.0.1:6379"),
		tcp6: tcpTable("::1:9000", "::ffff:127.0.0.1:9001", "::ffff:"+podIP+":9002", "fd00::5:9003")})
	if want := []int{6379, 9002, 9003}; !slices.Equal(got.Ports, want) {
		t.Fatalf("Read's ports = %v; want %v", got.Ports, want)
	}
	if want := []int{53, 8124, 9000, 9001}; !slices.Equal(got.Loopback, want) {
		t.Fatalf("Read's loopback ports = %v; want %v", got.Loopback, want)
	}
	got = readAll(process{pid: "7", cgroup: systemdCgroup(podUID, "burstable"), fibTrie: fibTrie(podIP),
		tcp: tcpTable("127.0.0.1:8124"), tcp6: tcpTable("::1:9000", "::1:8124")})
	if len(got.Ports) != 0 || !slices.Equal(got.Loopback, []int{8124, 9000}) {
		t.Fatalf("Read of a loopback-only pod = %+v; want no port, and 8124 and 9000 on loopback", got)
	}
}

// A kernel with IPv6 disabled has no tcp6.
func TestReadWithoutTCP6(t *testing.T) {
	p := podProcess("7", systemdCgroup(podUID, "besteffort"))
	p.tcp6 = nil
	got, err := read(t, fakeProc(t, p))
	if want := []int{6379, 10000}; err != nil || !slices.Equal(got.Ports, want) {
		t.Fatalf("Read = %v, %v; want %v", got, err, want)
	}
}

// Another pod's process with the same port open is never read, whichever
// comes first in /proc; a pod with no process here is ErrPodNotFound.
func TestReadIgnoresOtherPods(t *testing.T) {
	other := process{pid: "100", cgroup: systemdCgroup(otherUID, "besteffort"), fibTrie: fibTrie("10.244.1.6"),
		tcp: tcpTable("0.0.0.0:6379", "0.0.0.0:7777")}
	host := process{pid: "1", cgroup: "0::/init.scope\n", fibTrie: fibTrie("172.22.0.6"), tcp: tcpTable("0.0.0.0:22")}
	got, err := read(t, fakeProc(t, host, other, podProcess("200", systemdCgroup(podUID, "burstable"))))
	if err != nil || !slices.Equal(got.Ports, wantPorts) {
		t.Fatalf("Read = %v, %v; want %v", got, err, wantPorts)
	}
	if got, err := read(t, fakeProc(t, host, other)); !errors.Is(err, ErrPodNotFound) {
		t.Fatalf("Read with no process of the pod = %v, %v; want ErrPodNotFound", got, err)
	}
}

// Every container of a pod shares its network namespace, so any one of its
// processes answers: the pause process, the app, a sidecar.
func TestReadAnswersFromAnyOfThePodsProcesses(t *testing.T) {
	cgroup := func(container string) string {
		return "0::/kubepods.slice/kubepods-burstable.slice/kubepods-burstable-pod" + strings.ReplaceAll(podUID, "-", "_") + ".slice/cri-containerd-" + container + ".scope\n"
	}
	root := fakeProc(t, podProcess("300", cgroup("pause")), podProcess("301", cgroup("app")), podProcess("302", cgroup("sidecar")))
	got, err := read(t, root)
	if err != nil || !slices.Equal(got.Ports, wantPorts) {
		t.Fatalf("Read = %v, %v; want %v", got, err, wantPorts)
	}
}

// A process that exits between the listing and its read leaves its /proc
// entry without tables: the next process of the pod answers, and a pod whose
// every process went away is ErrUnreadable rather than a pod with no listener.
func TestReadSkipsAProcessThatVanishedMidRead(t *testing.T) {
	gone := process{pid: "400", cgroup: systemdCgroup(podUID, "burstable"), fibTrie: fibTrie(podIP)} // no net/tcp left
	got, err := read(t, fakeProc(t, gone, podProcess("401", systemdCgroup(podUID, "burstable"))))
	if err != nil || !slices.Equal(got.Ports, wantPorts) {
		t.Fatalf("Read = %v, %v; want %v", got, err, wantPorts)
	}
	noTrie := process{pid: "402", cgroup: systemdCgroup(podUID, "burstable"), tcp: tcpTable("0.0.0.0:80")}
	if got, err := read(t, fakeProc(t, gone, noTrie)); !errors.Is(err, ErrUnreadable) {
		t.Fatalf("Read with every process gone = %v, %v; want ErrUnreadable", got, err)
	}
}

// The pod address proves the namespace is the pod's: a namespace that does
// not hold it is refused, its listeners never returned.
func TestReadRefusesANamespaceWithoutThePodAddress(t *testing.T) {
	p := podProcess("500", systemdCgroup(podUID, "besteffort"))
	p.fibTrie = fibTrie("10.244.1.77")
	if got, err := read(t, fakeProc(t, p)); !errors.Is(err, ErrAddressMismatch) {
		t.Fatalf("Read with another address = %v, %v; want ErrAddressMismatch", got, err)
	}
	// The subnet's own leaf and loopback are in fib_trie too, but not as the
	// pod's /32 host LOCAL entry.
	for _, ip := range []string{"10.244.1.0", "127.0.0.0"} {
		if got, err := Read(t.Context(), fakeProc(t, podProcess("501", systemdCgroup(podUID, "besteffort"))), podUID, netip.MustParseAddr(ip)); err == nil {
			t.Fatalf("Read as %s = %v, want a refusal", ip, got)
		}
	}
}

// An IPv6 pod address is checked against if_inet6.
func TestReadChecksAnIPv6AddressInIfInet6(t *testing.T) {
	inet6 := func(addrs ...string) *string {
		var rows strings.Builder
		for i, a := range addrs {
			fmt.Fprintf(&rows, "%s %02x 40 00 80     eth0\n", hex.EncodeToString(netip.MustParseAddr(a).AsSlice()), i+1)
		}
		return new(rows.String())
	}
	p := podProcess("600", systemdCgroup(podUID, "besteffort"))
	p.ifInet6 = inet6("::1", "fd00:10:244:1::5", "fe80::1")
	got, err := Read(t.Context(), fakeProc(t, p), podUID, netip.MustParseAddr("fd00:10:244:1::5"))
	if err != nil || !slices.Equal(got.Ports, wantPorts) {
		t.Fatalf("Read = %v, %v; want %v", got, err, wantPorts)
	}
	if _, err := Read(t.Context(), fakeProc(t, p), podUID, netip.MustParseAddr("fd00:10:244:1::6")); !errors.Is(err, ErrAddressMismatch) {
		t.Fatalf("Read of an absent IPv6 address: %v, want ErrAddressMismatch", err)
	}
}

func TestReadRefusesMalformedRequests(t *testing.T) {
	root := fakeProc(t, podProcess("7", systemdCgroup(podUID, "besteffort")))
	for _, uid := range []string{"", "pod", strings.ToUpper(podUID), "../" + podUID[3:], podUID + "0"} {
		if _, err := Read(t.Context(), root, uid, netip.MustParseAddr(podIP)); !errors.Is(err, ErrBadRequest) {
			t.Fatalf("Read(uid %q) = %v, want ErrBadRequest", uid, err)
		}
	}
	for _, ip := range []netip.Addr{{}, netip.MustParseAddr("127.0.0.1"), netip.MustParseAddr("0.0.0.0"), netip.MustParseAddr("::")} {
		if _, err := Read(t.Context(), root, podUID, ip); !errors.Is(err, ErrBadRequest) {
			t.Fatalf("Read(ip %v) = %v, want ErrBadRequest", ip, err)
		}
	}
}

// Real kernel output (w6/m530 t001, 2026-11-20): the files the reader saw,
// from a host-PID container on Linux 7.0.12, for a hostUsers: false busybox
// pod on the local CAPD cluster binding 0.0.0.0:8123, 127.0.0.1:8124,
// [::1]:8125 and [::]:8126. The cgroup path is the CAPD node container's
// nested one, under a private cgroup namespace's /.. prefix.
const (
	realPodUID = "9d4212f2-102d-4aac-bca2-640eaa0efa69"
	realPodIP  = "192.168.88.196"
	realCgroup = "0::/../7eeba1e7eadd216c7cb23926ab0d6720259f252315b60d650ff6f05be89f8f18/kubelet.slice/kubelet-kubepods.slice/kubelet-kubepods-besteffort.slice/kubelet-kubepods-besteffort-pod9d4212f2_102d_4aac_bca2_640eaa0efa69.slice/cri-containerd-4bc9c61d36f0cded0c8fc77983bf090c2f2673835778586353e1b5f870a41196.scope\n"
	realTCP    = `  sl  local_address rem_address   st tx_queue rx_queue tr tm->when retrnsmt   uid  timeout inode
   0: 00000000:1FBB 00000000:0000 0A 00000000:00000000 00:00000000 00000000     0        0 116690725 1 000000005d3d12ef 100 0 0 10 0
   1: 0100007F:1FBC 00000000:0000 0A 00000000:00000000 00:00000000 00000000     0        0 116689613 1 00000000b5a10da6 100 0 0 10 0
`
	realTCP6 = `  sl  local_address                         remote_address                        st tx_queue rx_queue tr tm->when retrnsmt   uid  timeout inode
   0: 00000000000000000000000000000000:1FBE 00000000000000000000000000000000:0000 0A 00000000:00000000 00:00000000 00000000     0        0 116693519 1 00000000310d3b7d 100 0 0 10 0
   1: 00000000000000000000000001000000:1FBD 00000000000000000000000000000000:0000 0A 00000000:00000000 00:00000000 00000000     0        0 116691541 1 000000008b0af72b 100 0 0 10 0
`
	realFibTrie = `Main:
  +-- 0.0.0.0/0 3 0 4
     |-- 0.0.0.0
        /0 universe UNICAST
     +-- 127.0.0.0/8 2 0 2
        +-- 127.0.0.0/31 1 0 0
           |-- 127.0.0.0
              /8 host LOCAL
           |-- 127.0.0.1
              /32 host LOCAL
        |-- 127.255.255.255
           /32 link BROADCAST
     |-- 169.254.1.1
        /32 link UNICAST
     |-- 192.168.88.196
        /32 host LOCAL
Local:
  +-- 0.0.0.0/0 3 0 4
     |-- 0.0.0.0
        /0 universe UNICAST
     +-- 127.0.0.0/8 2 0 2
        +-- 127.0.0.0/31 1 0 0
           |-- 127.0.0.0
              /8 host LOCAL
           |-- 127.0.0.1
              /32 host LOCAL
        |-- 127.255.255.255
           /32 link BROADCAST
     |-- 169.254.1.1
        /32 link UNICAST
     |-- 192.168.88.196
        /32 host LOCAL
`
)

// The loopback rule on the kernel's own output: 0.0.0.0 and :: count,
// 127.0.0.1 and ::1 do not, and the namespace's fib_trie proves the pod IP
// (169.254.1.1, Calico's link route, is not a local address).
func TestReadOnRealKernelOutput(t *testing.T) {
	requireLittleEndianFixtures(t)
	root := fakeProc(t, process{pid: "70275", cgroup: realCgroup, fibTrie: new(realFibTrie), tcp: new(realTCP), tcp6: new(realTCP6)})
	got, err := Read(t.Context(), root, realPodUID, netip.MustParseAddr(realPodIP))
	if want := []int{8123, 8126}; err != nil || !slices.Equal(got.Ports, want) {
		t.Fatalf("Read = %v, %v; want %v", got, err, want)
	}
	if want := []int{8124, 8125}; !slices.Equal(got.Loopback, want) {
		t.Fatalf("Read's loopback ports = %v; want %v", got.Loopback, want)
	}
	if _, err := Read(t.Context(), root, realPodUID, netip.MustParseAddr("169.254.1.1")); !errors.Is(err, ErrAddressMismatch) {
		t.Fatalf("Read as the link route's address: %v, want ErrAddressMismatch", err)
	}
}

func requireLittleEndianFixtures(t *testing.T) {
	t.Helper()
	var b [2]byte
	binary.NativeEndian.PutUint16(b[:], 1)
	if b[0] != 1 {
		t.Skip("the fixture is a little-endian kernel's output")
	}
}
