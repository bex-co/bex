type ResetHandler = () => Promise<void> | void;

export type BoundaryLease = {
  signal: AbortSignal;
  isCurrent: () => boolean;
  finish: () => void;
};

export class DataBoundary {
  private epoch = 0;
  private controllers = new Set<AbortController>();
  private handlers = new Set<ResetHandler>();

  workspaceId: string | null = null;

  begin(): BoundaryLease {
    const epoch = this.epoch;
    const controller = new AbortController();
    this.controllers.add(controller);
    return {
      signal: controller.signal,
      isCurrent: () => epoch === this.epoch && !controller.signal.aborted,
      finish: () => this.controllers.delete(controller),
    };
  }

  registerResetHandler(handler: ResetHandler): () => void {
    this.handlers.add(handler);
    return () => this.handlers.delete(handler);
  }

  initialize(workspaceId: string): void {
    this.workspaceId = workspaceId;
  }

  async reset(workspaceId: string | null): Promise<void> {
    this.epoch += 1;
    this.workspaceId = workspaceId;
    for (const controller of this.controllers) controller.abort();
    this.controllers.clear();
    await Promise.all([...this.handlers].map((handler) => handler()));
  }
}

export const dataBoundary = new DataBoundary();

export function resetIdentityBoundary(): Promise<void> {
  return dataBoundary.reset(null);
}

export function resetWorkspaceBoundary(workspaceId: string): Promise<void> {
  return dataBoundary.reset(workspaceId);
}

export function initializeWorkspaceBoundary(workspaceId: string): void {
  dataBoundary.initialize(workspaceId);
}
