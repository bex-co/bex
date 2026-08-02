/**
 * Orchestrates the drawer's confirmed logout without duplicating m2's
 * local-first teardown. It gates on a native confirmation, runs exactly one
 * {@link signOut} sequence, and refuses re-entry while a request is in flight
 * so a double-tap cannot fire two sign-outs or stack two dialogs. Framework-free
 * so the cancel/confirm/double-tap contract is unit-testable.
 */
export type LogoutDeps = {
  /** Show the native confirmation; resolves true only when the user confirms. */
  confirm: () => Promise<boolean>;
  /** The reviewed local-first OAuth teardown (`authManager.signOut`). */
  signOut: () => Promise<void>;
  /** Notified when the destructive phase starts/ends, for spinner + disable. */
  onPending?: (pending: boolean) => void;
};

export type LogoutOutcome = "skipped" | "canceled" | "done" | "failed";

export class LogoutController {
  private busy = false;

  constructor(private readonly deps: LogoutDeps) {}

  isBusy(): boolean {
    return this.busy;
  }

  async request(): Promise<LogoutOutcome> {
    if (this.busy) return "skipped";
    this.busy = true;
    try {
      const confirmed = await this.deps.confirm();
      if (!confirmed) return "canceled";
      this.deps.onPending?.(true);
      try {
        await this.deps.signOut();
        return "done";
      } catch {
        // signOut is itself local-first and best-effort; a thrown error means
        // teardown could not complete, distinct from a remote-revoke failure it
        // already swallows.
        return "failed";
      } finally {
        this.deps.onPending?.(false);
      }
    } finally {
      this.busy = false;
    }
  }
}
