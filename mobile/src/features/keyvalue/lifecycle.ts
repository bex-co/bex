import {
  isLifecycleSuspended,
  protectedConfirmationFromError,
  type LifecycleCapability,
  type LifecycleRunResult,
} from "../services/lifecycle";

export type KeyValueLifecycleAction = "suspend" | "resume";

export type KeyValueLifecycleResource = {
  id: string;
  name: string;
  status: string;
  suspended: boolean | string | null;
  updatedAt?: string | null;
};

export type KeyValueLifecycleControllerOptions = {
  mutate: {
    suspend: (id: string, confirmation?: string) => Promise<void>;
    resume: (id: string) => Promise<void>;
  };
  refresh: (id: string) => Promise<KeyValueLifecycleResource | null>;
  wait?: () => Promise<void>;
  maxPolls?: number;
};

const ACTIONABLE_STATUSES = new Set(["available", "unavailable"]);

export function keyValueLifecycleCapabilities(
  resource: KeyValueLifecycleResource,
): LifecycleCapability<KeyValueLifecycleAction>[] {
  if (isLifecycleSuspended(resource.suspended)) {
    return resource.status.toLowerCase() === "deleting"
      ? []
      : [{ action: "resume", requiresConfirmation: false }];
  }
  if (!ACTIONABLE_STATUSES.has(resource.status.toLowerCase())) return [];
  return [{ action: "suspend", requiresConfirmation: true }];
}

export class KeyValueLifecycleController {
  private readonly pending = new Map<string, KeyValueLifecycleAction>();
  private readonly wait: () => Promise<void>;
  private readonly maxPolls: number;

  constructor(private readonly options: KeyValueLifecycleControllerOptions) {
    this.wait =
      options.wait ??
      (() => new Promise((resolve) => setTimeout(resolve, 2_500)));
    this.maxPolls = Math.max(1, options.maxPolls ?? 12);
  }

  pendingAction(id: string): KeyValueLifecycleAction | null {
    return this.pending.get(id) ?? null;
  }

  async run(input: {
    action: KeyValueLifecycleAction;
    resource: KeyValueLifecycleResource;
    confirmed: boolean;
    serverConfirmation?: string;
  }): Promise<
    LifecycleRunResult<KeyValueLifecycleResource, KeyValueLifecycleAction>
  > {
    const { action, resource } = input;
    const active = this.pending.get(resource.id);
    if (active) return { status: "busy", action: active };
    const capability = keyValueLifecycleCapabilities(resource).find(
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
        } else {
          await this.options.mutate.resume(resource.id);
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

      let latest: KeyValueLifecycleResource | null = null;
      for (let poll = 0; poll < this.maxPolls; poll += 1) {
        latest = await this.options.refresh(resource.id);
        const converged =
          latest &&
          (action === "suspend"
            ? isLifecycleSuspended(latest.suspended)
            : !isLifecycleSuspended(latest.suspended));
        if (latest && converged) return { status: "success", resource: latest };
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
