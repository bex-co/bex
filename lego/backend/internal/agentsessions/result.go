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

package agentsessions

import (
	"context"
	"log"
	"strings"

	"github.com/bex-co/bex/lego/backend/internal/agentsession"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// withOutputEnv adds a structured-output turn's contract to the driver env
// (ADR094 D5): the Pod-bound result URL, the schema the driver checks a result
// against before it posts, and whether the turn must end with a final one. A
// free-form turn adds nothing, so its sandbox has no submit_result tool.
func (s *Service) withOutputEnv(env map[string]string, record store.AgentSession, output store.AgentTurnOutput) map[string]string {
	if len(output.Schema) == 0 {
		return env
	}
	url, err := agentsession.ResultURL(s.gitProxyURL(), store.SandboxNamespace(record.WorkspaceID), record.ID)
	if err != nil {
		// Without the URL the agent cannot submit; a required result then fails
		// the turn STRUCTURED_OUTPUT_MISSING instead of hanging it.
		log.Printf("agent-session dispatch: result URL invalid (session=%s): %v", record.ID, err)
		return env
	}
	env["BEX_AGENT_RESULT_URL"] = url
	env["BEX_AGENT_OUTPUT_SCHEMA"] = string(output.Schema)
	env["BEX_AGENT_OUTPUT_REQUIRED"] = "0"
	if output.Required {
		env["BEX_AGENT_OUTPUT_REQUIRED"] = "1"
	}
	return env
}

// SubmitResult stores a result the agent submitted for its turn's outputSchema
// (ADR094 D5). The gateway has already bound the request to a Pod carrying the
// session's labels and signed the envelope, so there is no caller identity to
// authorize. The session row is re-read and the Pod held to the same
// current-sandbox rule as every other gateway verb, then the result is checked
// against the schema stored on the turn: the driver's own check is a courtesy
// to the agent, this one is the authority. The last valid submission wins.
func (s *Service) SubmitResult(ctx context.Context, envelope agentsession.ResultEnvelope) (agentsession.ResultResponse, error) {
	record, err := s.Store.GetAgentSession(ctx, envelope.SessionID)
	if err != nil || !agentsession.CurrentSandboxCaller(record, envelope.Workspace, envelope.PodName, envelope.PodUID) {
		return agentsession.ResultResponse{}, agentsession.NewResultError(agentsession.ResultForbidden, "this session is not running a sandbox")
	}
	turns, err := s.Store.AgentSessionTurns(ctx, record.ID)
	if err != nil {
		return agentsession.ResultResponse{}, agentsession.NewResultError(agentsession.ResultUnavailable, "the turn could not be read")
	}
	var schema []byte
	for _, turn := range turns {
		if turn.Turn == record.Turns {
			schema = turn.OutputSchema
		}
	}
	if len(schema) == 0 {
		return agentsession.ResultResponse{}, agentsession.NewResultError(agentsession.ResultNotExpected, "this turn has no outputSchema")
	}
	if problems := checkResult(schema, envelope.Result); len(problems) > 0 {
		return agentsession.ResultResponse{}, agentsession.NewResultError(agentsession.ResultSchemaMismatch, strings.Join(problems, "; "))
	}
	stored, err := s.Store.RecordAgentSessionResult(ctx, record.ID, record.Turns, envelope.Result, envelope.Final)
	if err != nil {
		log.Printf("agent-session result: store failed (session=%s): %v", record.ID, err)
		return agentsession.ResultResponse{}, agentsession.NewResultError(agentsession.ResultUnavailable, "the result could not be stored")
	}
	if !stored { // the turn ended between the check above and the write
		return agentsession.ResultResponse{}, agentsession.NewResultError(agentsession.ResultForbidden, "this session is no longer running this turn")
	}
	return agentsession.ResultResponse{Accepted: true, Final: envelope.Final}, nil
}
