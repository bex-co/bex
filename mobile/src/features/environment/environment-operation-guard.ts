export type EnvironmentOperationKind = "reveal" | "mutation" | "refresh";
export type EnvironmentOperationStatus = "active" | "timed-out" | "invalidated";

export type EnvironmentOperationLease = {
  signal: AbortSignal;
  status: () => EnvironmentOperationStatus;
  isCurrent: () => boolean;
  finish: () => void;
};

type ActiveOperation = {
  kind: EnvironmentOperationKind;
  epoch: number;
  controller: AbortController;
  status: EnvironmentOperationStatus;
  timer: ReturnType<typeof setTimeout>;
};

/**
 * Bounds secret-bearing network work and invalidates every response across a
 * lifecycle, service, or identity boundary. A timeout aborts transport but
 * remains distinguishable from a boundary invalidation because writes that may
 * have crossed the wire need an honest unknown outcome.
 */
export class EnvironmentOperationGuard {
  private epoch = 0;
  private active = new Set<ActiveOperation>();

  begin(
    kind: EnvironmentOperationKind,
    timeoutMs: number,
  ): EnvironmentOperationLease {
    const operation: ActiveOperation = {
      kind,
      epoch: this.epoch,
      controller: new AbortController(),
      status: "active",
      timer: undefined as never,
    };
    operation.timer = setTimeout(() => {
      if (operation.status !== "active") return;
      operation.status = "timed-out";
      operation.controller.abort();
    }, timeoutMs);
    this.active.add(operation);
    return {
      signal: operation.controller.signal,
      status: () => operation.status,
      isCurrent: () =>
        operation.epoch === this.epoch && operation.status !== "invalidated",
      finish: () => this.finish(operation),
    };
  }

  hasActive(kind: EnvironmentOperationKind): boolean {
    for (const operation of this.active) {
      if (operation.kind === kind && operation.status === "active") return true;
    }
    return false;
  }

  invalidate(): void {
    this.epoch += 1;
    for (const operation of this.active) {
      clearTimeout(operation.timer);
      operation.status = "invalidated";
      operation.controller.abort();
    }
    this.active.clear();
  }

  private finish(operation: ActiveOperation): void {
    clearTimeout(operation.timer);
    this.active.delete(operation);
  }
}

export function environmentTimeoutError(): Error {
  return Object.assign(new Error("environment request outcome is unknown"), {
    name: "TimeoutError",
  });
}
