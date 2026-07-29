// m35-netprobe is a tiny, dependency-free network probe copied into disposable
// OpenSandbox workloads by verify-sandbox-isolation.sh. The production base
// image intentionally has no Python, curl, or OpenSSL, while the verifier must
// distinguish DNS, TCP, and TLS-SNI enforcement rather than infer from a
// generic timeout.
package main

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"os"
	"strconv"
	"time"
)

const timeout = 5 * time.Second

func main() {
	if len(os.Args) < 3 {
		usage()
	}

	var err error
	switch os.Args[1] {
	case "resolve":
		if len(os.Args) != 3 {
			usage()
		}
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()
		var addresses []string
		addresses, err = net.DefaultResolver.LookupHost(ctx, os.Args[2])
		if err == nil && len(addresses) == 0 {
			err = fmt.Errorf("resolver returned no addresses")
		}
		if err == nil {
			fmt.Println(addresses[0])
		}
	case "tcp":
		if len(os.Args) != 4 {
			usage()
		}
		err = dialTCP(os.Args[2], os.Args[3])
	case "tls":
		if len(os.Args) != 5 {
			usage()
		}
		err = dialTLS(os.Args[2], os.Args[3], os.Args[4])
	default:
		usage()
	}

	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func dialTCP(host, rawPort string) error {
	port, err := strconv.Atoi(rawPort)
	if err != nil || port < 1 || port > 65535 {
		return fmt.Errorf("invalid port %q", rawPort)
	}
	conn, err := net.DialTimeout("tcp", net.JoinHostPort(host, strconv.Itoa(port)), timeout)
	if err != nil {
		return err
	}
	return conn.Close()
}

func dialTLS(host, rawPort, serverName string) error {
	port, err := strconv.Atoi(rawPort)
	if err != nil || port < 1 || port > 65535 {
		return fmt.Errorf("invalid port %q", rawPort)
	}
	dialer := &net.Dialer{Timeout: timeout}
	conn, err := tls.DialWithDialer(dialer, "tcp", net.JoinHostPort(host, strconv.Itoa(port)), &tls.Config{
		// The verifier tests network identity, not the public CA bundle in the
		// sandbox image. ServerName still emits the exact SNI Cilium evaluates.
		InsecureSkipVerify: true, //nolint:gosec -- intentional network-policy probe
		ServerName:         serverName,
		MinVersion:         tls.VersionTLS12,
	})
	if err != nil {
		return err
	}
	return conn.Close()
}

func usage() {
	fmt.Fprintln(os.Stderr, "usage: m35-netprobe resolve HOST | tcp HOST PORT | tls HOST PORT SNI")
	os.Exit(2)
}
