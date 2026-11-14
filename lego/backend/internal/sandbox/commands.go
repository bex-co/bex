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

package sandbox

import (
	"context"
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/sandboxexec"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// The command registry (w32/m381, ADR014 D4). Render keeps a sandbox awake
// while a command runs, connected or not: "Disconnecting your client does not
// stop an executing command inside the sandbox" (render.com/docs/
// sandboxes-executing), and a sandbox "is considered idle if no command
// execution or file transfer is in progress" (sandboxes-creating, both read
// 2026-11-13). A command started through pods/exec without a TTY survives its
// stream, so bex-api cannot see it end once the stream is gone. The exec
// wrapper (homeExecScript) therefore records each command it starts in an
// ordinary sandbox:
//
//   - <registry>/<execution id>.run holds the PIDs and /proc start times of the
//     command and of the wrapper waiting on it. It is written before the
//     command starts and removed after its end is logged. The start times tell
//     a live process from a reused PID: after a resume, a stale entry's PID may
//     name another process. The wrapper counts too, so a command that just
//     ended but is not logged yet still reads as running.
//   - <registry>/exits gets one line per ended command, `<execution id>
//     <exit code> <unix seconds>`, and moves to exits.1 at 1 MiB.
//
// The idle sweeper and history reads ask the sandbox about the records whose
// stream ended without an exit (commandProbe). A process a finished command
// left in the background is not in the registry, so it keeps nothing awake,
// as on Render. The tenant can write the registry: what it says is bounded
// to the record's own span and to what a running command could do anyway.
var commandRegistry = "/run/bex-exec"

// f20 sets f to the 20th word of the /proc/<pid>/stat fields after the
// command's name: the process's start time in clock ticks since boot.
const f20 = `f20() { f=; [ $# -ge 20 ] && shift 19 && f=$1; }
`

// commandStart is the wrapper's child, passed to it as its own argument: it
// records itself and its parent, then becomes the command, keeping its PID.
const commandStart = `r=$1
shift
` + f20 + `line=
read -r line 2>/dev/null </proc/$$/stat
f20 ${line##*") "}
self=$f
line=
read -r line 2>/dev/null </proc/$PPID/stat
f20 ${line##*") "}
{ printf "%s %s %s %s\n" "$$" "$self" "$PPID" "$f" > "$r.run"; } 2>/dev/null
exec "$@"`

// commandEnd runs in the wrapper once the command ended with $rc. stat reads
// the log's size without reading the log; wc is the fallback.
const commandEnd = `{ d=${reg%/*}
  printf "%s %s %s\n" "${reg##*/}" "$rc" "$(date +%s)" >> "$d/exits"
  size=$(stat -c %s "$d/exits" 2>/dev/null || wc -c <"$d/exits")
  [ "$size" -lt 1048576 ] || mv -f "$d/exits" "$d/exits.1"
  rm -f "$reg.run"; } 2>/dev/null`

// commandProbe answers `running <id>` for each execution id after the
// registry ($1) whose command or wrapper is alive, and `exited <id> <exit
// code> <unix seconds>` for each one whose end is logged, in one pass over the
// log.
const commandProbe = `cd "$1" 2>/dev/null || exit 0
shift
` + f20 + `alive() {
  line=
  read -r line 2>/dev/null </proc/$1/stat || return 1
  f20 ${line##*") "}
  [ -n "$2" ] && [ "$f" = "$2" ]
}
for id; do
  pid= start= ppid= pstart=
  read -r pid start ppid pstart 2>/dev/null <"$id.run" || continue
  if alive "$pid" "$start" || alive "$ppid" "$pstart"; then echo "running $id"; fi
done
logs=
for log in exits.1 exits; do [ -f "$log" ] && logs="$logs $log"; done
[ -z "$logs" ] || awk -v ids=" $* " 'index(ids, " " $1 " ") { last[$1] = $0 } END { for (id in last) print "exited " last[id] }' $logs
exit 0`

// maxProbedCommands bounds one probe's argv. The newest records are asked
// about first.
const maxProbedCommands = 100

// probeTimeout bounds one probe. The sweeper runs it under its suspend
// claim, which an exec arriving meanwhile waits on for suspendClaimGrace.
const probeTimeout = 5 * time.Second

// probeFailureGrace is how long after a record was last known to be running
// a registry that cannot be read still keeps its sandbox up. Past it the
// sandbox suspends as before, rather than bill for a pod bex cannot reach.
const probeFailureGrace = time.Hour

type commandExit struct {
	id   string
	code int
	at   time.Time
}

type commandReport struct {
	running []string
	exited  []commandExit
}

// probeCommands asks the sandbox's registry about ids. It runs as bex-api
// itself, under the workspace's probe subject (whose authority is the
// ticket's HMAC), never as the caller whose read triggered it, and holds
// nothing active.
func (s *Service) probeCommands(ctx context.Context, ws string, raw osSandbox, ids []string) (commandReport, error) {
	ctx = core.WithIdentity(ctx, core.Identity{Subject: sandboxexec.ProbeSubjectPrefix + ws, Method: "sandbox-command-probe"})
	ctx, cancel := context.WithTimeout(ctx, probeTimeout)
	defer cancel()
	res, err := s.systemExec(ctx, ws, raw.ID, "", append([]string{"/bin/sh", "-c", commandProbe, "bex-probe", commandRegistry}, ids...), maxExecOutputBytes)
	if err != nil {
		return commandReport{}, err
	}
	if res.ExitCode != 0 || res.Truncated {
		return commandReport{}, unreachable("the sandbox's command registry could not be read")
	}
	known := make(map[string]bool, len(ids))
	for _, id := range ids {
		known[id] = true
	}
	ended := map[string]bool{}
	var report commandReport
	var running []string
	for line := range strings.Lines(res.Stdout) {
		switch f := strings.Fields(line); {
		case len(f) == 2 && f[0] == "running" && known[f[1]]:
			running = append(running, f[1])
		case len(f) == 4 && f[0] == "exited" && known[f[1]]:
			code, errCode := strconv.Atoi(f[2])
			at, errAt := strconv.ParseInt(f[3], 10, 64)
			if errCode == nil && errAt == nil {
				report.exited = append(report.exited, commandExit{id: f[1], code: code, at: time.Unix(at, 0)})
				ended[f[1]] = true
			}
		}
	}
	// A command logged as ended has ended, whatever its wrapper's last moment.
	for _, id := range running {
		if !ended[id] {
			report.running = append(report.running, id)
		}
	}
	return report, nil
}

// reconcileCommands brings a running sandbox's open records up to date with
// its registry. A command still running keeps its record open and the
// sandbox active. One that ended closes its record at its own exit and counts
// as activity then, so the idle window runs from the command's end; the time
// is the tenant's to write, so it is held to the present (the store holds it
// to the record's start). A stale record the registry does not know ends at
// its last refresh, as before. It reports whether a command still runs and
// when the last one ended.
func (s *Service) reconcileCommands(ctx context.Context, ws string, raw osSandbox, open []store.OpenSandboxExecution) (running bool, lastEnd time.Time, err error) {
	ids := make([]string, len(open))
	for i, o := range open {
		ids[i] = o.ID
	}
	report, err := s.probeCommands(ctx, ws, raw, ids)
	if err != nil {
		return false, time.Time{}, err
	}
	now := s.Now()
	for _, id := range report.running {
		s.touchExecution(ctx, ws, id)
	}
	for _, e := range report.exited {
		at, code := e.at, e.code
		if at.After(now) {
			at = now
		}
		s.stopExecutionAt(ctx, ws, e.id, at, &code)
		if at.After(lastEnd) {
			lastEnd = at
		}
	}
	switch {
	case len(report.running) > 0:
		s.touchActivityAt(ctx, ws, raw.ID, now)
	case !lastEnd.IsZero():
		s.touchActivityAt(ctx, ws, raw.ID, lastEnd)
	}
	if err := s.Executions.StopStaleSandboxExecutions(ctx, ws, canonicalID(raw), now.Add(-executionStaleAfter)); err != nil {
		log.Printf("sandbox: close stale executions of %s: %v", canonicalID(raw), err)
	}
	return len(report.running) > 0, lastEnd, nil
}

// commandsKeepAwake is the idle sweeper's last check before it pauses a
// sandbox it claimed: a command whose stream ended without its exit may still
// be running, or have ended within the idle window. Only a sandbox with such
// a record pays for a probe. A registry that cannot be read keeps the sandbox
// up for another idle window, up to probeFailureGrace.
func (s *Service) commandsKeepAwake(ctx context.Context, ws string, raw osSandbox, idle time.Duration) bool {
	if s.Executions == nil || !s.execEnabled() {
		return false
	}
	now := s.Now()
	open, err := s.Executions.OpenSandboxExecutions(ctx, ws, canonicalID(raw), now, maxProbedCommands)
	if err != nil {
		log.Printf("sandbox idle: open executions workspace=%s sandbox=%s: %v", ws, raw.ID, err)
		return true
	}
	if len(open) == 0 {
		return false
	}
	running, lastEnd, err := s.reconcileCommands(ctx, ws, raw, open)
	if err != nil {
		var known time.Time
		for _, o := range open {
			if o.SeenAt.After(known) {
				known = o.SeenAt
			}
		}
		if now.Sub(known) >= probeFailureGrace {
			log.Printf("sandbox idle: command registry workspace=%s sandbox=%s unreadable for %s; suspending: %v", ws, raw.ID, probeFailureGrace, err)
			return false
		}
		log.Printf("sandbox idle: command registry workspace=%s sandbox=%s: %v", ws, raw.ID, err)
		s.touchActivityAt(ctx, ws, raw.ID, now)
		return true
	}
	return running || now.Sub(lastEnd) < idle
}
