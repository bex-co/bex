import { ApolloLink, Observable, type ErrorLike } from "@apollo/client";
import { ServerError } from "@apollo/client/errors";
import {
  telemetryLabel,
  trackEvent,
  type TelemetryEvent,
  type TelemetryProps,
} from "@/common/lib/telemetry";
import { rememberCheckout } from "@/common/lib/telemetry-router";

type Variables = Record<string, unknown>;

interface MutationEvent {
  event: TelemetryEvent;
  /** Bounded labels read from the variables; each passes `telemetryLabel`. */
  params?: (variables: Variables) => Record<string, unknown>;
  /** Runs once the event is reported. */
  after?: () => void;
}

const plan = (v: Variables) => ({ plan: v.plan });

/**
 * The product actions worth counting, by GraphQL operation name. One table
 * instead of a `trackEvent` at each of the ~110 mutation call sites: a new
 * surface that runs the same mutation is counted without knowing this exists.
 * A mutation absent here still reports its failures (`action_failed`).
 */
export const MUTATION_EVENTS: Readonly<Record<string, MutationEvent>> = {
  CreateService: {
    event: "service_created",
    params: (v) => ({
      service_type: v.type,
      runtime: v.runtime,
      plan: v.plan,
      source: v.image ? "image" : v.repo ? "git" : undefined,
    }),
  },
  UpdateServicePlan: { event: "service_plan_changed", params: plan },
  SuspendService: { event: "service_suspended" },
  ResumeService: { event: "service_resumed" },
  DeleteService: { event: "service_deleted" },
  TriggerDeploy: {
    event: "deploy_triggered",
    params: (v) => ({ deploy_mode: v.deployMode }),
  },
  RollbackService: { event: "deploy_rolled_back" },
  CreateShellSession: { event: "shell_opened" },
  AddCustomDomain: { event: "custom_domain_added" },
  CreateDatabase: { event: "database_created", params: plan },
  UpdateDatabasePlan: { event: "database_plan_changed", params: plan },
  DeleteDatabase: { event: "database_deleted" },
  CreateKeyValue: { event: "keyvalue_created", params: plan },
  UpdateKeyValuePlan: { event: "keyvalue_plan_changed", params: plan },
  DeleteKeyValue: { event: "keyvalue_deleted" },
  CreateBlueprint: { event: "blueprint_created" },
  SyncBlueprint: { event: "blueprint_synced" },
  CreateEnvGroup: { event: "env_group_created" },
  CreateProject: { event: "project_created" },

  CreateAgentSession: {
    event: "agent_session_created",
    params: (v) => {
      const config = (v.agentConfig ?? {}) as Variables;
      return {
        agent: config.agent,
        // A model on the visitor's own endpoint is theirs to name.
        model: config.modelEndpoint ? "custom" : config.model,
      };
    },
  },
  SteerAgentSession: { event: "agent_message_sent" },
  CreateAgentAutomation: { event: "automation_created" },
  RunAgentAutomation: { event: "automation_run" },

  CreateBillingCheckoutSession: {
    event: "begin_checkout",
    // Stripe Checkout's return counts once per checkout begun.
    after: rememberCheckout,
  },
  CreateBillingPortalSession: { event: "billing_portal_opened" },
  CreateWorkspace: { event: "workspace_created", params: plan },
  FinalizeWorkspaceCreation: { event: "workspace_created" },
  ChangeWorkspacePlan: { event: "workspace_plan_changed", params: plan },
  DeleteWorkspace: { event: "workspace_deleted" },
  DeleteAccount: { event: "account_deleted" },
  ConnectGit: { event: "git_connect_started" },
  ClaimGit: { event: "git_connect_started" },
  InviteWorkspaceMember: {
    event: "member_invited",
    params: (v) => ({ role: v.role }),
  },
  AcceptWorkspaceInvite: { event: "invite_accepted" },
  CreateApiKey: { event: "api_key_created" },
  CreateSSHKey: { event: "ssh_key_added" },
  CreateWebhookEndpoint: { event: "webhook_created" },
};

function labels(params: Record<string, unknown>): TelemetryProps {
  return Object.fromEntries(
    Object.entries(params).map(([key, value]) => [key, telemetryLabel(value)]),
  );
}

function errorCode(error: ErrorLike): string {
  if (ServerError.is(error)) return `HTTP_${error.statusCode}`;
  // fetch rejects with a TypeError when the request got no answer at all.
  return error.name === "TypeError" ? "NETWORK" : error.name;
}

/**
 * Counts product actions: a mutation that succeeded reports its
 * `MUTATION_EVENTS` entry, and any mutation that was refused or failed reports
 * `action_failed` with the operation name and the stable error code. First in
 * the chain, so it sees the final outcome after the retry link.
 */
export function createTelemetryLink(): ApolloLink {
  return new ApolloLink((operation, forward) => {
    if (operation.operationType !== "mutation") return forward(operation);
    const action = operation.operationName ?? "";
    const failed = (code: unknown) =>
      trackEvent("action_failed", {
        action: telemetryLabel(action),
        error_code: telemetryLabel(code) ?? "UNKNOWN",
      });
    return new Observable((observer) => {
      const sub = forward(operation).subscribe({
        next: (result) => {
          const refusal = result.errors?.[0];
          const mapped = MUTATION_EVENTS[action];
          if (refusal) failed(refusal.extensions?.["code"]);
          else if (mapped) {
            trackEvent(
              mapped.event,
              labels(mapped.params?.(operation.variables ?? {}) ?? {}),
            );
            mapped.after?.();
          }
          observer.next(result);
        },
        error: (error: ErrorLike) => {
          // A cancelled request (unmount, superseded submit) is not a failure.
          if (error.name !== "AbortError") failed(errorCode(error));
          observer.error(error);
        },
        complete: () => observer.complete(),
      });
      return () => sub.unsubscribe();
    });
  });
}
