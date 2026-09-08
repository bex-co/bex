import {
  presentAction,
  type ResourceActionDecision,
} from "../capabilities/resource-actions";

export type ServiceLifecycleAction = "suspend" | "resume" | "restart";

// The server's per-resource decision for one lifecycle verb (ADR087
// serverActions projection, normalized by toResourceSnapshot): the tri-state
// permission plus the blocking precondition, or null when the projection has
// no row for this exact workspace+resource+action. Presentation eligibility
// comes from here — never from the service's phase, type, or suspension,
// which the projection's execute paths already account for.
export type ServiceLifecycleDecision = Pick<
  ResourceActionDecision,
  "outcome" | "precondition"
>;

export type ServiceLifecycleResource = {
  id: string;
  name: string;
  type: string;
  phase: string;
  suspended: boolean | string | null;
  updatedAt?: string | null;
  revision?: string | null;
  latestDeployId?: string | null;
};

export type LifecycleCapability<Action extends string> = {
  action: Action;
  requiresConfirmation: boolean;
};

export type LifecycleRunResult<Resource, Action extends string> =
  | { status: "success"; resource: Resource }
  | { status: "accepted_unverified"; resource: Resource | null }
  | { status: "busy"; action: Action }
  | { status: "not_allowed"; reason: "state" | "type" }
  | {
      status: "confirmation_required";
      source: "device" | "server";
      confirmation?: string;
    }
  | { status: "timeout"; resource: Resource | null }
  | { status: "error"; error: unknown };

export type ServiceLifecycleMutationPort = {
  suspend: (id: string, confirmation?: string) => Promise<void>;
  resume: (id: string) => Promise<void>;
  restart: (id: string) => Promise<{ operationId?: string | null } | void>;
};

export type ServiceLifecycleControllerOptions = {
  mutate: ServiceLifecycleMutationPort;
  refresh: (id: string) => Promise<ServiceLifecycleResource | null>;
  wait?: () => Promise<void>;
  maxPolls?: number;
};

export function isLifecycleSuspended(value: unknown): boolean {
  return value === true || value === "suspended";
}

// Device-confirmation policy per verb (UX, not authorization): suspend and
// restart confirm on-device; resume does not.
const REQUIRES_DEVICE_CONFIRMATION: Record<ServiceLifecycleAction, boolean> = {
  suspend: true,
  restart: true,
  resume: false,
};

export function protectedConfirmationFromError(error: unknown): string | null {
  const message =
    error instanceof Error
      ? error.message
      : typeof error === "string"
        ? error
        : null;
  if (!message?.toLowerCase().includes("protected environment")) return null;
  const match = message.match(/retry with confirm=(?:"([^"]+)"|'([^']+)')/i);
  return match?.[1] ?? match?.[2] ?? null;
}

export class ServiceLifecycleController {
  private readonly pending = new Map<string, ServiceLifecycleAction>();
  private readonly wait: () => Promise<void>;
  private readonly maxPolls: number;

  constructor(private readonly options: ServiceLifecycleControllerOptions) {
    this.wait =
      options.wait ??
      (() => new Promise((resolve) => setTimeout(resolve, 2_500)));
    this.maxPolls = Math.max(1, options.maxPolls ?? 12);
  }

  pendingAction(id: string): ServiceLifecycleAction | null {
    return this.pending.get(id) ?? null;
  }

  async run(input: {
    action: ServiceLifecycleAction;
    resource: ServiceLifecycleResource;
    confirmed: boolean;
    serverConfirmation?: string;
    /**
     * The server's decision for this exact action, read from the current
     * projection snapshot at confirmation time. Null (no row for this
     * workspace+resource+action) means the verb does not exist for this
     * resource — never an implicit allow.
     */
    decision: ServiceLifecycleDecision | null;
  }): Promise<
    LifecycleRunResult<ServiceLifecycleResource, ServiceLifecycleAction>
  > {
    const { action, resource, decision } = input;
    const active = this.pending.get(resource.id);
    if (active) return { status: "busy", action: active };
    if (!decision) {
      return { status: "not_allowed", reason: "type" };
    }
    // Shared presentation semantics: denied/unavailable decisions are absent,
    // blocked ones do not send. Protected-environment suspend presents as
    // ready — the mutation's phrase round trip below completes it.
    if (presentAction(decision).kind !== "ready") {
      return { status: "not_allowed", reason: "state" };
    }
    if (REQUIRES_DEVICE_CONFIRMATION[action] && !input.confirmed) {
      return { status: "confirmation_required", source: "device" };
    }

    this.pending.set(resource.id, action);
    try {
      let operationId: string | null = null;
      try {
        if (action === "suspend") {
          await this.options.mutate.suspend(
            resource.id,
            input.serverConfirmation,
          );
        } else if (action === "resume") {
          await this.options.mutate.resume(resource.id);
        } else {
          const accepted = await this.options.mutate.restart(resource.id);
          operationId = accepted?.operationId ?? null;
        }
      } catch (error) {
        const confirmation = protectedConfirmationFromError(error);
        if (confirmation && input.serverConfirmation !== confirmation) {
          return {
            status: "confirmation_required",
            source: "server",
            confirmation,
          };
        }
        return { status: "error", error };
      }

      let latest: ServiceLifecycleResource | null = null;
      for (let poll = 0; poll < this.maxPolls; poll += 1) {
        latest = await this.options.refresh(resource.id);
        if (latest && serviceConverged(action, resource, latest, operationId)) {
          return { status: "success", resource: latest };
        }
        if (poll < this.maxPolls - 1) await this.wait();
      }
      return { status: "timeout", resource: latest };
    } catch (error) {
      return { status: "error", error };
    } finally {
      this.pending.delete(resource.id);
    }
  }
}

function serviceConverged(
  action: ServiceLifecycleAction,
  before: ServiceLifecycleResource,
  after: ServiceLifecycleResource,
  operationId: string | null,
): boolean {
  if (action === "suspend") return isLifecycleSuspended(after.suspended);
  if (action === "resume") return !isLifecycleSuspended(after.suspended);
  if (operationId && after.latestDeployId === operationId) return true;
  return (
    after.phase.toLowerCase() === "running" &&
    ((after.revision != null && after.revision !== before.revision) ||
      (after.updatedAt != null && after.updatedAt !== before.updatedAt))
  );
}
