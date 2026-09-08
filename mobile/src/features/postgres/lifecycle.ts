import { datastoreSuspensionConverged } from "../resources/datastore-lifecycle";
import {
  presentAction,
  type ResourceActionDecision,
} from "../capabilities/resource-actions";
import {
  protectedConfirmationFromError,
  type LifecycleRunResult,
} from "../services/lifecycle";

export type PostgresLifecycleAction = "suspend" | "resume" | "restart";

// The server's per-resource decision for one lifecycle verb (ADR087
// databaseActions projection, normalized by toResourceSnapshot): the tri-state
// permission plus the blocking precondition, or null when the projection has
// no row for this exact workspace+resource+action. Presentation eligibility
// comes from here — never from the database's status or suspension, which the
// projection's execute paths already account for.
export type PostgresLifecycleDecision = Pick<
  ResourceActionDecision,
  "outcome" | "precondition"
>;

export type PostgresLifecycleResource = {
  id: string;
  name: string;
  status: string;
  suspended: boolean | string | null;
  updatedAt?: string | null;
};

export type PostgresLifecycleControllerOptions = {
  mutate: {
    suspend: (id: string, confirmation?: string) => Promise<void>;
    resume: (id: string) => Promise<void>;
    restart: (id: string) => Promise<void>;
  };
  refresh: (id: string) => Promise<PostgresLifecycleResource | null>;
  wait?: () => Promise<void>;
  maxPolls?: number;
};

// Device-confirmation policy per verb (UX, not authorization): suspend and
// restart confirm on-device; resume does not.
const REQUIRES_DEVICE_CONFIRMATION: Record<PostgresLifecycleAction, boolean> = {
  suspend: true,
  restart: true,
  resume: false,
};

export class PostgresLifecycleController {
  private readonly pending = new Map<string, PostgresLifecycleAction>();
  private readonly wait: () => Promise<void>;
  private readonly maxPolls: number;

  constructor(private readonly options: PostgresLifecycleControllerOptions) {
    this.wait =
      options.wait ??
      (() => new Promise((resolve) => setTimeout(resolve, 2_500)));
    this.maxPolls = Math.max(1, options.maxPolls ?? 12);
  }

  pendingAction(id: string): PostgresLifecycleAction | null {
    return this.pending.get(id) ?? null;
  }

  async run(input: {
    action: PostgresLifecycleAction;
    resource: PostgresLifecycleResource;
    confirmed: boolean;
    serverConfirmation?: string;
    /**
     * The server's decision for this exact action, read from the current
     * projection snapshot at confirmation time. Null (no row for this
     * workspace+resource+action) means the verb does not exist for this
     * resource — never an implicit allow.
     */
    decision: PostgresLifecycleDecision | null;
  }): Promise<
    LifecycleRunResult<PostgresLifecycleResource, PostgresLifecycleAction>
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
      try {
        if (action === "suspend") {
          await this.options.mutate.suspend(
            resource.id,
            input.serverConfirmation,
          );
        } else if (action === "resume") {
          await this.options.mutate.resume(resource.id);
        } else {
          await this.options.mutate.restart(resource.id);
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

      if (action === "restart") {
        // The mutation proves acceptance, but the current view exposes no
        // restart generation or CNPG condition that can prove the bounce.
        let latest: PostgresLifecycleResource | null = null;
        try {
          latest = await this.options.refresh(resource.id);
        } catch {
          // Acceptance remains known when this best-effort refresh is absent.
        }
        return { status: "accepted_unverified", resource: latest };
      }

      let latest: PostgresLifecycleResource | null = null;
      for (let poll = 0; poll < this.maxPolls; poll += 1) {
        latest = await this.options.refresh(resource.id);
        if (latest && datastoreSuspensionConverged(action, latest.suspended)) {
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
