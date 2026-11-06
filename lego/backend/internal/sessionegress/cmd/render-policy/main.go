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

// Command render-policy prints, as JSON, the agent-session egress
// CiliumNetworkPolicy bex-api would apply, so scripts/verify-sandbox-isolation.sh
// tests the real policy instead of a hand-written copy that can drift (it did,
// from 2026-08-16 until d2b00c61e; w6/m162).
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"math"
	"os"
	"strings"

	"github.com/bex-co/bex/lego/backend/internal/sessionegress"
)

func main() {
	namespace := flag.String("namespace", "", "tenant namespace")
	session := flag.String("session", "", "agent session id (the bex.co/agent-session label)")
	phase := flag.String("phase", string(sessionegress.PhaseSetup), "setup or agent")
	model := flag.String("model", "https://api.anthropic.com", "the session's model endpoint")
	allow := flag.String("allow", "", "comma-separated tenant destinations")
	// Production wires BEX_AGENT_MODEL_PROXY_URL on :8084, which bex-api-session-egress
	// requires; 0 renders the legacy direct-model policy.
	modelProxyPort := flag.Uint("model-proxy-port", 8084, "gateway model proxy port")
	flag.Parse()
	if *namespace == "" || *session == "" {
		fail(fmt.Errorf("-namespace and -session are required"))
	}
	if p := sessionegress.Phase(*phase); p != sessionegress.PhaseSetup && p != sessionegress.PhaseAgent {
		fail(fmt.Errorf("-phase must be %q or %q, got %q", sessionegress.PhaseSetup, sessionegress.PhaseAgent, *phase))
	}
	if *modelProxyPort > math.MaxUint16 {
		fail(fmt.Errorf("-model-proxy-port %d is not a port", *modelProxyPort))
	}
	var entries []string
	if *allow != "" {
		entries = strings.Split(*allow, ",")
	}
	// The same validation bex-api applies to a session's tenant destinations.
	hosts, err := sessionegress.ExtraDestinations(entries)
	if err != nil {
		fail(err)
	}
	policy, err := sessionegress.RenderPolicy(
		sessionegress.Config{ModelProxyPort: uint16(*modelProxyPort)},
		*namespace, *session, sessionegress.Phase(*phase), *model,
		sessionegress.Egress{Hosts: hosts},
	)
	if err != nil {
		fail(err)
	}
	out, err := json.MarshalIndent(policy.Object, "", "  ")
	if err != nil {
		fail(err)
	}
	fmt.Println(string(out))
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, "render-policy:", err)
	os.Exit(2)
}
