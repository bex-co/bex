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

package agentcred

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// pkt frames one pkt-line.
func pkt(payload string) string { return fmt.Sprintf("%04x%s", len(payload)+4, payload) }

// band frames payload as one side-band packet.
func band(n byte, payload string) string { return pkt(string([]byte{n}) + payload) }

// githubWorkflowRefusal is the report GitHub sent on 2026-11-18 when a
// contents-only App token created bex-agent/w10-014-probe at an older commit of
// bex-co/bex (w10/014), as GIT_TRACE_PACKET recorded it.
func githubWorkflowRefusal(ref string) string {
	return band(2, "\x00") +
		band(1, pkt("unpack ok\n")) +
		band(1, pkt("ng "+ref+" refusing to allow a GitHub App to create or update workflow `.github/workflows/backend-test.yml` without `workflows` permission\n")) +
		band(1, "0000") +
		"0000"
}

func relay(t *testing.T, upstream string) string {
	t.Helper()
	var out bytes.Buffer
	read, err := relayReceivePackReport(&out, strings.NewReader(upstream))
	if err != nil || read != int64(len(upstream)) {
		t.Fatalf("relay read %d of %d bytes, err %v", read, len(upstream), err)
	}
	return out.String()
}

// w10/014: GitHub's refusal of a workflow change reaches the agent with a plain
// explanation in front of it; every byte GitHub sent still arrives, in order.
func TestRelayReceivePackReportExplainsAWorkflowRefusal(t *testing.T) {
	upstream := githubWorkflowRefusal("refs/heads/bex-agent/task-1")
	got := relay(t, upstream)
	// The ng line's side-band packet starts 9 bytes before it: the packet's
	// length, its band byte, then the inner pkt-line's length.
	refusal := strings.Index(upstream, "ng refs/") - 9
	want := upstream[:refusal] + string(workflowRefusalExplanation) + upstream[refusal:]
	if got != want {
		t.Fatalf("relayed\n%q\nwant\n%q", got, want)
	}

	// The refusal split across two band-1 packets is still explained, once.
	line := pkt("ng refs/heads/bex-agent/task-1 refusing to allow a GitHub App to create or update workflow `ci.yml` without `workflows` permission\n")
	cut := strings.Index(line, "`workflows`") + 4
	split := band(1, pkt("unpack ok\n")) + band(1, line[:cut]) + band(1, line[cut:]) + band(1, line) + "0000"
	got = relay(t, split)
	if strings.Count(got, string(workflowRefusalExplanation)) != 1 || strings.Replace(got, string(workflowRefusalExplanation), "", 1) != split {
		t.Fatalf("split refusal relayed as %q", got)
	}
}

// Every other report passes through byte for byte: an accepted push, another
// refusal, a report without side-band framing (which has no channel for a
// message), and a stream cut mid-packet.
func TestRelayReceivePackReportLeavesOtherReportsAlone(t *testing.T) {
	refusal := "ng refs/heads/bex-agent/task-1 refusing to allow a GitHub App to create or update workflow `ci.yml` without `workflows` permission\n"
	for name, upstream := range map[string]string{
		"accepted":       band(2, "Resolving deltas: 100% (3/3)\n") + band(1, pkt("unpack ok\n")+pkt("ok refs/heads/bex-agent/task-1\n")+"0000") + "0000",
		"other refusal":  band(1, pkt("unpack ok\n")+pkt("ng refs/heads/bex-agent/task-1 protected branch hook declined\n")+"0000") + "0000",
		"no side-band":   pkt("unpack ok\n") + pkt(refusal) + "0000",
		"probe answer":   "0000",
		"empty":          "",
		"cut mid-packet": band(1, pkt("unpack ok\n"))[:9],
		"not pkt-lines":  "HTTP/1.1 garbage " + refusal,
	} {
		if got := relay(t, upstream); got != upstream {
			t.Errorf("%s: relayed %q, want it unchanged", name, got)
		}
	}
}

// The agent's own git prints the explanation as "remote: bex: …" lines ahead
// of the "[remote rejected]" line, through the real proxy and a real push.
func TestGitPushPrintsTheWorkflowRefusalExplanation(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not installed")
	}
	broker, repoURL, _ := gitProxyHarness(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Query().Get("service") == "git-receive-pack":
			w.Header().Set("Content-Type", "application/x-git-receive-pack-advertisement")
			_, _ = io.WriteString(w, pkt("# service=git-receive-pack\n")+"0000"+
				pkt(strings.Repeat("0", 40)+" capabilities^{}\x00report-status side-band-64k ofs-delta\n")+"0000")
		case r.Method == http.MethodPost:
			_, _ = io.Copy(io.Discard, r.Body)
			w.Header().Set("Content-Type", "application/x-git-receive-pack-result")
			_, _ = io.WriteString(w, githubWorkflowRefusal("refs/heads/bex-agent/task-1"))
		default:
			http.NotFound(w, r)
		}
	})
	front := httptest.NewServer(broker.Handler())
	t.Cleanup(front.Close)

	dir := t.TempDir()
	git := func(args ...string) (string, error) {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_NOSYSTEM=1", "GIT_TERMINAL_PROMPT=0",
			"GIT_AUTHOR_NAME=agent", "GIT_AUTHOR_EMAIL=agent@example.com", "GIT_COMMITTER_NAME=agent", "GIT_COMMITTER_EMAIL=agent@example.com")
		out, err := cmd.CombinedOutput()
		return string(out), err
	}
	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("hello\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"init", "-q"}, {"add", "README.md"}, {"commit", "-qm", "work"}} {
		if out, err := git(args...); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	out, err := git("push", front.URL+strings.TrimPrefix(repoURL, "http://gateway"), "HEAD:bex-agent/task-1")
	if err == nil {
		t.Fatalf("the refused push succeeded:\n%s", out)
	}
	explained := strings.Index(out, "remote: bex: GitHub refused this push because it changes a file under .github/workflows/")
	rejected := strings.Index(out, "[remote rejected]")
	if explained < 0 || rejected < explained || !strings.Contains(out, "remote: bex: If you did not edit a workflow, run `git fetch origin HEAD && git merge FETCH_HEAD`, then push again.") {
		t.Fatalf("git push printed:\n%s", out)
	}
}
