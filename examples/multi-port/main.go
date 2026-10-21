// multi-port is the w6/m148 fixture for Render-style private services that
// listen on every port their image declares. The default role is a private
// service that ignores $PORT and serves HTTP on 8123 and a TCP banner on 9000,
// the two ports its Dockerfile EXPOSEs. ROLE=consumer is a web service that
// reaches both through Blueprint fromService references (API_HOST,
// API_HOSTPORT, API_PORT) and reports what it saw.
package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"time"
	"unsafe"
)

// ioUringSetup creates and closes a 4-entry io_uring (syscall 425 on both
// amd64 and arm64), reporting "ok" or the errno.
func ioUringSetup() string {
	var params [120]byte
	fd, _, errno := syscall.Syscall(425, 4, uintptr(unsafe.Pointer(&params[0])), 0)
	if errno != 0 {
		return errno.Error()
	}
	syscall.Close(int(fd))
	return "ok"
}

func main() {
	if os.Getenv("ROLE") == "consumer" {
		consumer()
		return
	}
	server()
}

func marker() string {
	if m := os.Getenv("MARKER"); m != "" {
		return m
	}
	return "multi-port"
}

func server() {
	tcp, err := net.Listen("tcp", "0.0.0.0:9000")
	if err != nil {
		log.Fatal(err)
	}
	go func() {
		for {
			conn, err := tcp.Accept()
			if err != nil {
				log.Print(err)
				continue
			}
			fmt.Fprintf(conn, "%s tcp 9000\n", marker())
			conn.Close()
		}
	}()
	http.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprintf(w, "%s http 8123\n", marker())
	})
	// The process's own privilege profile, read from inside the running pod,
	// so acceptance checks what the container actually got (w6/m148).
	http.HandleFunc("/proc", func(w http.ResponseWriter, _ *http.Request) {
		status, _ := os.ReadFile("/proc/self/status")
		uidMap, _ := os.ReadFile("/proc/self/uid_map")
		for _, line := range strings.Split(string(status), "\n") {
			for _, key := range []string{"CapBnd:", "CapEff:", "NoNewPrivs:", "Seccomp:"} {
				if strings.HasPrefix(line, key) {
					fmt.Fprintln(w, line)
				}
			}
		}
		fmt.Fprintf(w, "uid_map: %s", uidMap)
		// The two things Render's unfiltered runtime allows and bex's
		// render-v1 seccomp profile must too (w6/m150): an io_uring instance
		// and a user namespace created by a non-root process.
		fmt.Fprintf(w, "io_uring_setup: %s\n", ioUringSetup())
		if out, err := exec.Command("unshare", "-Ur", "true").CombinedOutput(); err != nil {
			fmt.Fprintf(w, "unshare -Ur: %v %s\n", err, strings.TrimSpace(string(out)))
		} else {
			fmt.Fprintln(w, "unshare -Ur: ok")
		}
	})
	log.Print("listening on 8123 (http) and 9000 (tcp); $PORT is ignored")
	log.Fatal(http.ListenAndServe("0.0.0.0:8123", nil))
}

func consumer() {
	client := &http.Client{Timeout: 5 * time.Second}
	http.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) { io.WriteString(w, "ok\n") })
	http.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
		report := map[string]string{
			"API_HOST":     os.Getenv("API_HOST"),
			"API_HOSTPORT": os.Getenv("API_HOSTPORT"),
			"API_PORT":     os.Getenv("API_PORT"),
		}
		if resp, err := client.Get("http://" + report["API_HOSTPORT"] + "/"); err != nil {
			report["http"] = "error: " + err.Error()
		} else {
			body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
			resp.Body.Close()
			report["http"] = string(body)
		}
		if resp, err := client.Get("http://" + report["API_HOSTPORT"] + "/proc"); err == nil {
			body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
			resp.Body.Close()
			report["server_proc"] = string(body)
		}
		if conn, err := net.DialTimeout("tcp", net.JoinHostPort(report["API_HOST"], "9000"), 5*time.Second); err != nil {
			report["tcp"] = "error: " + err.Error()
		} else {
			conn.SetReadDeadline(time.Now().Add(5 * time.Second))
			line, _ := bufio.NewReader(conn).ReadString('\n')
			conn.Close()
			report["tcp"] = line
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(report)
	})
	port := os.Getenv("PORT")
	if port == "" {
		port = "3000"
	}
	log.Fatal(http.ListenAndServe("0.0.0.0:"+port, nil))
}
