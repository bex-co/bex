import { CurrentUserClient, CurrentUserError } from "./current-user-client";
import type { CurrentUser } from "./current-user";

/**
 * Framework-free state machine for the drawer's personal status read. It owns
 * the identity boundary: each {@link load} bumps a generation and aborts the
 * prior request, and {@link reset} (called when the signed-in identity changes)
 * discards any in-flight response so a late reply from a signed-out or replaced
 * session can never render. Mirrors the auth SessionManager's subscribe model
 * so the React hook stays thin and the guard logic stays unit-testable.
 */
export type CurrentUserState =
  | { status: "idle" }
  | { status: "loading" }
  | { status: "ready"; user: CurrentUser }
  | { status: "unavailable" }
  | { status: "offline" };

type Listener = (state: CurrentUserState) => void;

export class CurrentUserController {
  private state: CurrentUserState = { status: "idle" };
  private listeners = new Set<Listener>();
  private generation = 0;
  private controller?: AbortController;

  constructor(private readonly client: CurrentUserClient) {}

  subscribe(listener: Listener): () => void {
    this.listeners.add(listener);
    listener(this.state);
    return () => this.listeners.delete(listener);
  }

  getState(): CurrentUserState {
    return this.state;
  }

  private setState(state: CurrentUserState): void {
    this.state = state;
    for (const listener of this.listeners) listener(state);
  }

  /** Abort any in-flight read and forget the identity (identity boundary). */
  reset(): void {
    this.generation += 1;
    this.controller?.abort();
    this.controller = undefined;
    this.setState({ status: "idle" });
  }

  /** Fetch the current user once. Safe to call repeatedly for retry. */
  async load(): Promise<void> {
    const generation = ++this.generation;
    this.controller?.abort();
    const controller = new AbortController();
    this.controller = controller;
    this.setState({ status: "loading" });
    try {
      const user = await this.client.fetch(controller.signal);
      if (this.generation !== generation) return;
      this.setState({ status: "ready", user });
    } catch (error) {
      // A superseded or aborted request must never overwrite newer identity
      // state — this is the late-response suppression the boundary depends on.
      if (this.generation !== generation || controller.signal.aborted) return;
      const code =
        error instanceof CurrentUserError ? error.code : "unavailable";
      this.setState({ status: code === "network" ? "offline" : "unavailable" });
    }
  }
}
