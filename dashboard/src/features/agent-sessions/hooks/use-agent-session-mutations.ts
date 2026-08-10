import { useCallback } from "react";
import { useMutation } from "@apollo/client/react";
import {
  CreateAgentSessionDocument,
  SteerAgentSessionDocument,
  ResumeAgentSessionDocument,
  AttachAgentSessionDocument,
  CancelAgentSessionDocument,
} from "@/graphql/definitions";
import {
  toAgentSessionTicket,
  toAgentSessionView,
} from "@/features/agent-sessions/lib/mapper";
import { toAgentSessionError } from "@/features/agent-sessions/lib/errors";
import type {
  AgentSessionTicket,
  AgentSessionView,
} from "@/features/agent-sessions/types";

/** The composer's collected create fields (ADR047 D9 create form subset). */
export interface CreateAgentSessionInput {
  /** Workspace the session is created in; omitted ⇒ the caller's default. */
  ownerId?: string | null;
  /** GitHub owner/repository. */
  repo: string;
  /** The `bex-agent/*` working branch. */
  branch: string;
  agent: string;
  model?: string;
  modelEndpoint?: string;
  /** The initial fire-and-forget prompt. */
  task: string;
  /** Platform-registered sandbox image; omitted ⇒ the default template. */
  template?: string;
  /**
   * Ask the platform to open a draft PR when the session finishes (w5/m65).
   * The `bex-agent/*` branch is always pushed; the PR is opt-in, so omitting
   * this delivers the branch alone.
   */
  openPr?: boolean;
  /** Extra egress FQDNs beyond the model endpoint + built-in setup registries. */
  egressAllowlist?: string[];
}

export interface UseAgentSessionMutationsResult {
  /** Fires `createAgentSession`; resolves the new session + attach ticket. */
  create: (input: CreateAgentSessionInput) => Promise<AgentSessionTicket>;
  /** Steers an idle session with a follow-up prompt (redispatch path). */
  steer: (
    id: string,
    prompt: string,
    egressAllowlist?: string[],
  ) => Promise<AgentSessionTicket>;
  /** Resumes an idle session (re-mints its attach ticket). */
  resume: (id: string) => Promise<AgentSessionTicket>;
  /** Reconnect-mints a running session's attach ticket (m43 reconnect). */
  attach: (id: string) => Promise<AgentSessionTicket>;
  /** Cancels a session; resolves the updated (canceling/canceled) view. */
  cancel: (id: string) => Promise<AgentSessionView>;
}

/**
 * Write wrappers over bex-api's agent-session mutations (ADR047 D9). Every verb
 * runs `no-cache` (the polled queries own the cache) and re-throws through
 * `toAgentSessionError`, so a caller catches a typed
 * `AgentSessionsUnavailableError` (503) or `AgentSessionError` (coded
 * `AGENT_SESSION_*`, egress errors carrying `{reason, entry}`) — never a raw
 * Apollo error. bex-api authorizes + mints; the browser presents the returned
 * ticket to the m43 stream endpoint (t002).
 */
export function useAgentSessionMutations(): UseAgentSessionMutationsResult {
  const [createMutation] = useMutation(CreateAgentSessionDocument);
  const [steerMutation] = useMutation(SteerAgentSessionDocument);
  const [resumeMutation] = useMutation(ResumeAgentSessionDocument);
  const [attachMutation] = useMutation(AttachAgentSessionDocument);
  const [cancelMutation] = useMutation(CancelAgentSessionDocument);

  const create = useCallback(
    async (input: CreateAgentSessionInput): Promise<AgentSessionTicket> => {
      try {
        const res = await createMutation({
          variables: {
            ownerId: input.ownerId ?? null,
            repo: input.repo,
            branch: input.branch,
            agentConfig: {
              agent: input.agent,
              model: input.model || undefined,
              modelEndpoint: input.modelEndpoint || undefined,
              task: input.task,
              template: input.template || undefined,
              openPr: input.openPr || undefined,
            },
            egressAllowlist: input.egressAllowlist?.length
              ? input.egressAllowlist
              : undefined,
          },
          fetchPolicy: "no-cache",
        });
        const session = res.data?.createAgentSession;
        if (!session) throw new Error("createAgentSession returned no session");
        return toAgentSessionTicket(session);
      } catch (err) {
        throw toAgentSessionError(err);
      }
    },
    [createMutation],
  );

  const steer = useCallback(
    async (
      id: string,
      prompt: string,
      egressAllowlist?: string[],
    ): Promise<AgentSessionTicket> => {
      try {
        const res = await steerMutation({
          variables: {
            id,
            prompt,
            egressAllowlist: egressAllowlist?.length
              ? egressAllowlist
              : undefined,
          },
          fetchPolicy: "no-cache",
        });
        const session = res.data?.steerAgentSession;
        if (!session) throw new Error("steerAgentSession returned no session");
        return toAgentSessionTicket(session);
      } catch (err) {
        throw toAgentSessionError(err);
      }
    },
    [steerMutation],
  );

  const resume = useCallback(
    async (id: string): Promise<AgentSessionTicket> => {
      try {
        const res = await resumeMutation({
          variables: { id },
          fetchPolicy: "no-cache",
        });
        const session = res.data?.resumeAgentSession;
        if (!session) throw new Error("resumeAgentSession returned no session");
        return toAgentSessionTicket(session);
      } catch (err) {
        throw toAgentSessionError(err);
      }
    },
    [resumeMutation],
  );

  const attach = useCallback(
    async (id: string): Promise<AgentSessionTicket> => {
      try {
        const res = await attachMutation({
          variables: { id },
          fetchPolicy: "no-cache",
        });
        const session = res.data?.attachAgentSession;
        if (!session) throw new Error("attachAgentSession returned no session");
        return toAgentSessionTicket(session);
      } catch (err) {
        throw toAgentSessionError(err);
      }
    },
    [attachMutation],
  );

  const cancel = useCallback(
    async (id: string): Promise<AgentSessionView> => {
      try {
        const res = await cancelMutation({
          variables: { id },
          fetchPolicy: "no-cache",
        });
        const session = res.data?.cancelAgentSession;
        if (!session) throw new Error("cancelAgentSession returned no session");
        return toAgentSessionView(session);
      } catch (err) {
        throw toAgentSessionError(err);
      }
    },
    [cancelMutation],
  );

  return { create, steer, resume, attach, cancel };
}
