import {
  datastoreLifecycleTransition,
  datastoreSuspensionConverged,
} from "../resources/datastore-lifecycle";
import {
  protectedConfirmationFromError,
  type LifecycleCapability,
  type LifecycleRunResult,
} from "../services/lifecycle";

export type PostgresLifecycleAction = "suspend" | "resume" | "restart";

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

export function postgresLifecycleCapabilities(
  resource: PostgresLifecycleResource,
): LifecycleCapability<PostgresLifecycleAction>[] {
  const transition = datastoreLifecycleTransition(resource);
  if (transition === "resume") {
    return [{ action: "resume", requiresConfirmation: false }];
  }
  return transition === "suspend"
    ? [
        { action: "suspend", requiresConfirmation: true },
        { action: "restart", requiresConfirmation: true },
      ]
    : [];
}

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
  }): Promise<
    LifecycleRunResult<PostgresLifecycleResource, PostgresLifecycleAction>
  > {
    const { action, resource } = input;
    const active = this.pending.get(resource.id);
    if (active) return { status: "busy", action: active };
    const capability = postgresLifecycleCapabilities(resource).find(
      (candidate) => candidate.action === action,
    );
    if (!capability) return { status: "not_allowed", reason: "state" };
    if (capability.requiresConfirmation && !input.confirmed) {
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
