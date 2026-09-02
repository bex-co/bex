import { useCallback } from "react";
import { usePaymentRequiredGate } from "@/features/usage/context/payment-required-context";
import { useApolloClient, useMutation } from "@apollo/client/react";
import type { TypedDocumentNode } from "@graphql-typed-document-node/core";
import type { AgentSessionFieldsFragment } from "@/graphql/definitions";
import {
  CreateAgentSessionDocument,
  SteerAgentSessionDocument,
  ResumeAgentSessionDocument,
  AttachAgentSessionDocument,
  CancelAgentSessionDocument,
  PinAgentSessionDocument,
  UnpinAgentSessionDocument,
  ArchiveAgentSessionDocument,
  UnarchiveAgentSessionDocument,
  DeleteAgentSessionDocument,
  AgentSessionDocument,
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
  /** The initial fire-and-forget prompt. */
  task: string;
  /** Platform-registered sandbox image; omitted ⇒ the default template. */
  template?: string;
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
  /** Pins a session so its hibernated workspace never expires (ADR059 D5). */
  pin: (id: string) => Promise<AgentSessionView>;
  /** Removes the never-expire pin, back onto the retention clock. */
  unpin: (id: string) => Promise<AgentSessionView>;
  /** Archives a session out of the working set (ADR065 D1); still viewable. */
  archive: (id: string) => Promise<AgentSessionView>;
  /** Returns an archived session to the working set. */
  unarchive: (id: string) => Promise<AgentSessionView>;
  /** Permanently deletes a finished session — row, transcript, snapshot (D4). */
  deleteSession: (id: string) => Promise<void>;
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

/**
 * Collapses the five `id → AgentSessionView` verbs (cancel/pin/unpin/archive/
 * unarchive) onto one body: fire the mutation `no-cache`, unwrap the single
 * result field, project it, and re-throw through `toAgentSessionError` — the
 * exact shape previously retyped per verb (the field-mutation-factory move,
 * c75a6811). The ticket verbs and delete keep their own bodies: their result
 * shapes differ.
 */
function useByIdViewMutation<TData>(
  document: TypedDocumentNode<TData, { id: string }>,
  pick: (data: TData) => AgentSessionFieldsFragment | null | undefined,
  name: string,
  onSuccess: (session: AgentSessionFieldsFragment) => void,
): (id: string) => Promise<AgentSessionView> {
  const [mutate] = useMutation(document);
  return useCallback(
    async (id: string) => {
      try {
        const res = await mutate({
          variables: { id },
          fetchPolicy: "no-cache",
        });
        const session = res.data == null ? null : pick(res.data);
        if (!session) throw new Error(`${name} returned no session`);
        const view = toAgentSessionView(session);
        onSuccess(session);
        return view;
      } catch (err) {
        throw toAgentSessionError(err);
      }
    },
    // pick and name are call-site literals, stable per hook instance.
    // eslint-disable-next-line react-hooks/exhaustive-deps
    [mutate, onSuccess],
  );
}

export function useAgentSessionMutations(): UseAgentSessionMutationsResult {
  const client = useApolloClient();
  // Lifecycle mutations deliberately use `no-cache`. Prime the detail query
  // with the returned server view, then evict every list variant (working set,
  // archived/all, phase/repo filters and pagination): the current detail never
  // flashes blank, mounted lists refetch immediately, and dormant variants
  // cannot resurrect stale membership later.
  const reconcileSession = useCallback(
    (session: AgentSessionFieldsFragment) => {
      client.writeQuery({
        query: AgentSessionDocument,
        variables: { id: session.id },
        data: { agentSession: session },
      });
      client.cache.evict({ id: "ROOT_QUERY", fieldName: "agentSessions" });
      client.cache.gc();
    },
    [client],
  );
  const invalidateDeletedSession = useCallback(
    (id: string) => {
      client.cache.evict({
        id: "ROOT_QUERY",
        fieldName: "agentSession",
        args: { id },
      });
      client.cache.evict({ id: "ROOT_QUERY", fieldName: "agentSessions" });
      client.cache.gc();
    },
    [client],
  );
  // ADR075 D7 (w6/m42 t008): create/steer/resume dispatch metered sandbox
  // compute, so under BEX_REQUIRE_PAYMENT_METHOD=all they can 402 — route them
  // through the same interception dialog as every other billable create
  // ("interception, not a dead end"); the other verbs never provision compute.
  const paymentGate = usePaymentRequiredGate();
  const [createMutation] = useMutation(CreateAgentSessionDocument);
  const [steerMutation] = useMutation(SteerAgentSessionDocument);
  const [resumeMutation] = useMutation(ResumeAgentSessionDocument);
  const [attachMutation] = useMutation(AttachAgentSessionDocument);
  const [deleteMutation] = useMutation(DeleteAgentSessionDocument);
  const cancel = useByIdViewMutation(
    CancelAgentSessionDocument,
    (d) => d.cancelAgentSession,
    "cancelAgentSession",
    reconcileSession,
  );
  const pin = useByIdViewMutation(
    PinAgentSessionDocument,
    (d) => d.pinAgentSession,
    "pinAgentSession",
    reconcileSession,
  );
  const unpin = useByIdViewMutation(
    UnpinAgentSessionDocument,
    (d) => d.unpinAgentSession,
    "unpinAgentSession",
    reconcileSession,
  );
  const archive = useByIdViewMutation(
    ArchiveAgentSessionDocument,
    (d) => d.archiveAgentSession,
    "archiveAgentSession",
    reconcileSession,
  );
  const unarchive = useByIdViewMutation(
    UnarchiveAgentSessionDocument,
    (d) => d.unarchiveAgentSession,
    "unarchiveAgentSession",
    reconcileSession,
  );

  const create = useCallback(
    async (input: CreateAgentSessionInput): Promise<AgentSessionTicket> => {
      try {
        const res = await paymentGate.run(() =>
          createMutation({
            variables: {
              ownerId: input.ownerId ?? null,
              repo: input.repo,
              branch: input.branch,
              agentConfig: {
                agent: input.agent,
                model: input.model || undefined,
                task: input.task,
                template: input.template || undefined,
              },
              egressAllowlist: input.egressAllowlist?.length
                ? input.egressAllowlist
                : undefined,
            },
            fetchPolicy: "no-cache",
          }),
        );
        const session = res.data?.createAgentSession;
        if (!session) throw new Error("createAgentSession returned no session");
        const ticket = toAgentSessionTicket(session);
        reconcileSession(session);
        return ticket;
      } catch (err) {
        throw toAgentSessionError(err);
      }
    },
    [createMutation, paymentGate, reconcileSession],
  );

  const steer = useCallback(
    async (
      id: string,
      prompt: string,
      egressAllowlist?: string[],
    ): Promise<AgentSessionTicket> => {
      try {
        const res = await paymentGate.run(() =>
          steerMutation({
            variables: {
              id,
              prompt,
              egressAllowlist: egressAllowlist?.length
                ? egressAllowlist
                : undefined,
            },
            fetchPolicy: "no-cache",
          }),
        );
        const session = res.data?.steerAgentSession;
        if (!session) throw new Error("steerAgentSession returned no session");
        const ticket = toAgentSessionTicket(session);
        reconcileSession(session);
        return ticket;
      } catch (err) {
        throw toAgentSessionError(err);
      }
    },
    [paymentGate, steerMutation, reconcileSession],
  );

  const resume = useCallback(
    async (id: string): Promise<AgentSessionTicket> => {
      try {
        const res = await paymentGate.run(() =>
          resumeMutation({
            variables: { id },
            fetchPolicy: "no-cache",
          }),
        );
        const session = res.data?.resumeAgentSession;
        if (!session) throw new Error("resumeAgentSession returned no session");
        const ticket = toAgentSessionTicket(session);
        reconcileSession(session);
        return ticket;
      } catch (err) {
        throw toAgentSessionError(err);
      }
    },
    [paymentGate, resumeMutation, reconcileSession],
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
  const deleteSession = useCallback(
    async (id: string): Promise<void> => {
      try {
        const res = await deleteMutation({
          variables: { id },
          fetchPolicy: "no-cache",
        });
        if (res.data?.deleteAgentSession !== true) {
          throw new Error("deleteAgentSession did not confirm");
        }
        invalidateDeletedSession(id);
      } catch (err) {
        throw toAgentSessionError(err);
      }
    },
    [deleteMutation, invalidateDeletedSession],
  );

  return {
    create,
    steer,
    resume,
    attach,
    cancel,
    pin,
    unpin,
    archive,
    unarchive,
    deleteSession,
  };
}
