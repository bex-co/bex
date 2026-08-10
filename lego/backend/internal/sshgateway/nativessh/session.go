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

package nativessh

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	"golang.org/x/crypto/ssh"
	"k8s.io/client-go/tools/remotecommand"

	"github.com/bex-co/bex/lego/backend/internal/apps"
	"github.com/bex-co/bex/lego/backend/internal/sshgateway"
)

type ptyRequest struct {
	Term         string
	Columns      uint32
	Rows         uint32
	WidthPixels  uint32
	HeightPixels uint32
	Modes        string
}

type windowChangeRequest struct {
	Columns      uint32
	Rows         uint32
	WidthPixels  uint32
	HeightPixels uint32
}

type execRequest struct {
	Command string
}

type subsystemRequest struct {
	Name string
}

// sftpServerPath is the OpenSSH SFTP server binary in the agent-session sandbox
// image (Debian `openssh-sftp-server`, ADR054 D4). It is a FIXED argv — never
// caller input — exec'd only for a `sftp` subsystem on a sandbox target, so Zed
// can upload its remote-server binary when the sandbox's closed egress blocks a
// direct download.
const sftpServerPath = "/usr/lib/openssh/sftp-server"

func serveSession(ctx context.Context, channel ssh.Channel, requests <-chan *ssh.Request, executor sshgateway.Executor, target apps.SSHInstanceTarget) error {
	defer channel.Close()
	var tty bool
	var initial *remotecommand.TerminalSize
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case request, ok := <-requests:
			if !ok {
				return nil
			}
			switch request.Type {
			case "pty-req":
				if tty {
					_ = request.Reply(false, nil)
					continue
				}
				var payload ptyRequest
				if err := ssh.Unmarshal(request.Payload, &payload); err != nil || !sshgateway.ValidTerminalSize(payload.Columns, payload.Rows) {
					_ = request.Reply(false, nil)
					continue
				}
				tty = true
				initial = &remotecommand.TerminalSize{Width: uint16(payload.Columns), Height: uint16(payload.Rows)}
				_ = request.Reply(true, nil)
			case "shell":
				if len(request.Payload) != 0 {
					_ = request.Reply(false, nil)
					continue
				}
				_ = request.Reply(true, nil)
				return runExec(ctx, channel, requests, executor, target, []string{"/bin/sh"}, tty, initial)
			case "exec":
				var payload execRequest
				if err := ssh.Unmarshal(request.Payload, &payload); err != nil || strings.TrimSpace(payload.Command) == "" {
					_ = request.Reply(false, nil)
					continue
				}
				if isSCPProtocolCommand(payload.Command) {
					_ = request.Reply(false, nil)
					continue
				}
				_ = request.Reply(true, nil)
				return runExec(ctx, channel, requests, executor, target, []string{"/bin/sh", "-lc", payload.Command}, tty, initial)
			case "subsystem":
				// The ONLY honored subsystem, and only for an agent-session sandbox
				// target (ADR054 D4): the fixed sftp-server binary, so Zed can upload
				// its remote-server. App (srv-…) targets reject every subsystem exactly
				// as before, and SCP protocol stays rejected on the exec path above.
				var payload subsystemRequest
				if !target.Sandbox || ssh.Unmarshal(request.Payload, &payload) != nil || payload.Name != "sftp" {
					_ = request.Reply(false, nil)
					continue
				}
				_ = request.Reply(true, nil)
				return runExec(ctx, channel, requests, executor, target, []string{sftpServerPath}, false, nil)
			default:
				// env, agent/X11 forwarding, and every other extension are unsupported.
				// None reaches Kubernetes.
				_ = request.Reply(false, nil)
			}
		}
	}
}

func runExec(ctx context.Context, channel ssh.Channel, requests <-chan *ssh.Request, executor sshgateway.Executor, target apps.SSHInstanceTarget, command []string, tty bool, initial *remotecommand.TerminalSize) error {
	execCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	sizes := sshgateway.NewSizeQueue(execCtx, initial)
	go func() {
		for request := range requests {
			if request.Type != "window-change" || !tty {
				_ = request.Reply(false, nil)
				continue
			}
			var payload windowChangeRequest
			if err := ssh.Unmarshal(request.Payload, &payload); err != nil || !sshgateway.ValidTerminalSize(payload.Columns, payload.Rows) {
				_ = request.Reply(false, nil)
				continue
			}
			sizes.Push(remotecommand.TerminalSize{Width: uint16(payload.Columns), Height: uint16(payload.Rows)})
			_ = request.Reply(true, nil)
		}
		cancel()
	}()

	stderr := io.Writer(channel.Stderr())
	if tty {
		stderr = nil
	}
	code, err := executor.Execute(execCtx, target, command, tty, sizes, channel, channel, stderr)
	execFailed := err != nil
	if err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return err
		}
		_, _ = io.WriteString(channel.Stderr(), "unable to start /bin/sh in this image\n")
		if code == 0 {
			code = 126
		}
	}
	code = sshgateway.ClampExit(code)
	_, sendErr := channel.SendRequest("exit-status", false, ssh.Marshal(struct{ Status uint32 }{Status: uint32(code)}))
	if sendErr != nil {
		return fmt.Errorf("send exit status: %w", sendErr)
	}
	if execFailed {
		return errors.New("app shell unavailable")
	}
	return nil
}

// isSCPProtocolCommand rejects the command shape emitted by the legacy SCP
// protocol (`scp -t` sink / `scp -f` source). Users still have an ordinary
// shell and can run a program named scp themselves; the gateway simply does
// not claim or accidentally provide file-transfer protocol support.
func isSCPProtocolCommand(command string) bool {
	fields := strings.Fields(command)
	if len(fields) < 2 {
		return false
	}
	program := fields[0]
	if slash := strings.LastIndex(program, "/"); slash >= 0 {
		program = program[slash+1:]
	}
	if program != "scp" {
		return false
	}
	for _, field := range fields[1:] {
		if field == "--" || !strings.HasPrefix(field, "-") {
			break
		}
		if strings.Contains(strings.TrimPrefix(field, "-"), "t") || strings.Contains(strings.TrimPrefix(field, "-"), "f") {
			return true
		}
	}
	return false
}
