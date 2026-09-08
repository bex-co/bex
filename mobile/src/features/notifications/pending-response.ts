// One pending response is enough for the OS initial response. Live and initial
// deliveries share this queue, so the same tap cannot navigate twice. The
// bounded dedupe set contains response IDs only, never notification content.
export class PendingNotificationResponse<T> {
  private pending: { id: string; value: T } | null = null;
  private consumed = new Set<string>();
  private running = false;

  capture(id: string, value: T): void {
    if (!this.consumed.has(id) && this.pending?.id !== id) {
      this.pending = { id, value };
    }
  }

  async drain(
    decide: (value: T) => "wait" | "reject" | "open",
    open: (value: T) => Promise<boolean>,
    clear: (id: string) => void,
  ): Promise<void> {
    if (this.running) return;
    this.running = true;
    try {
      while (this.pending) {
        const response = this.pending;
        const decision = decide(response.value);
        if (decision === "wait") return;
        if (decision === "open" && !(await open(response.value))) return;
        this.consumed.add(response.id);
        if (this.consumed.size > 128) {
          const oldest = this.consumed.values().next().value;
          if (oldest !== undefined) this.consumed.delete(oldest);
        }
        if (this.pending === response) this.pending = null;
        clear(response.id);
      }
    } finally {
      this.running = false;
    }
  }
}
