import { LogBuffer } from "./log-buffer";
import { hasStoreOnlyTailFilters } from "./query";
import {
  LogApiError,
  type LogFilters,
  type LogTailConnection,
  type LogTransport,
} from "./types";

export type LogSessionPhase =
  | "idle"
  | "catching_up"
  | "connecting"
  | "live"
  | "reconnecting"
  | "history_only"
  | "error";

export type LogSessionSnapshot = {
  phase: LogSessionPhase;
  lines: ReturnType<LogBuffer["snapshot"]>["lines"];
  paused: boolean;
  unseen: number;
  reconnectAttempt: number;
  tailBlockedByStoreOnlyFilters: boolean;
  error?: LogApiError;
};

type Scheduler = (callback: () => void, delayMs: number) => () => void;
type Listener = (snapshot: LogSessionSnapshot) => void;

const defaultScheduler: Scheduler = (callback, delayMs) => {
  const id = setTimeout(callback, delayMs);
  return () => clearTimeout(id);
};

export class LogSession {
  private readonly buffer: LogBuffer;
  private listeners = new Set<Listener>();
  private phase: LogSessionPhase = "idle";
  private reconnectAttempt = 0;
  private tailBlockedByStoreOnlyFilters = false;
  private error?: LogApiError;
  private abort?: AbortController;
  private connection?: LogTailConnection;
  private cancelReconnect?: () => void;
  private generation = 0;
  private activeFilters?: LogFilters;

  constructor(
    private readonly transport: LogTransport,
    capacity = 500,
    private readonly now: () => Date = () => new Date(),
    private readonly schedule: Scheduler = defaultScheduler,
  ) {
    this.buffer = new LogBuffer(capacity);
  }

  subscribe(listener: Listener): () => void {
    this.listeners.add(listener);
    listener(this.snapshot());
    return () => this.listeners.delete(listener);
  }

  snapshot(): LogSessionSnapshot {
    const buffer = this.buffer.snapshot();
    return {
      phase: this.phase,
      lines: buffer.lines,
      paused: buffer.paused,
      unseen: buffer.unseen,
      reconnectAttempt: this.reconnectAttempt,
      tailBlockedByStoreOnlyFilters: this.tailBlockedByStoreOnlyFilters,
      error: this.error,
    };
  }

  async start(filters: LogFilters): Promise<void> {
    this.stopActive();
    const generation = ++this.generation;
    this.activeFilters = filters;
    this.error = undefined;
    this.reconnectAttempt = 0;
    this.tailBlockedByStoreOnlyFilters = false;
    this.phase = "catching_up";
    this.buffer.replace([]);
    this.emit();
    const abort = new AbortController();
    this.abort = abort;
    // Freeze the boundary before history begins. History ends here and the
    // live stream starts here; inclusive timestamps are harmless because ids
    // are deduplicated by LogBuffer.
    const handoff = this.now().toISOString();
    try {
      const page = await this.transport.history(
        { ...filters, endTime: handoff, limit: filters.limit ?? 100 },
        abort.signal,
      );
      if (generation !== this.generation || abort.signal.aborted) return;
      this.buffer.replace(page.logs);
      if (hasStoreOnlyTailFilters(filters)) {
        this.tailBlockedByStoreOnlyFilters = true;
        this.phase = "history_only";
        this.emit();
        return;
      }
      this.phase = "connecting";
      this.emit();
      await this.connect({ ...filters, startTime: handoff }, generation);
    } catch (error) {
      if (generation !== this.generation || abort.signal.aborted) return;
      this.fail(error);
    }
  }

  setPaused(paused: boolean): void {
    this.buffer.setPaused(paused);
    this.emit();
  }

  stop(): void {
    ++this.generation;
    this.stopActive();
    this.phase = "idle";
    this.emit();
  }

  private async connect(
    filters: LogFilters,
    generation: number,
  ): Promise<void> {
    const abort = this.abort;
    if (!abort || abort.signal.aborted) return;
    try {
      this.connection = await this.transport.subscribe(
        filters,
        {
          onLine: (line) => {
            if (generation !== this.generation) return;
            this.reconnectAttempt = 0;
            this.error = undefined;
            this.phase = "live";
            this.buffer.append([line]);
            this.emit();
          },
          onError: (error) => {
            if (generation !== this.generation) return;
            if (error.code === "invalid_filter" || error.code === "forbidden") {
              this.fail(error);
              return;
            }
            this.queueReconnect(filters, generation, error);
          },
          onClose: () => {
            if (generation !== this.generation) return;
            this.queueReconnect(
              filters,
              generation,
              new LogApiError("network", "Log stream closed."),
            );
          },
        },
        abort.signal,
      );
      if (generation !== this.generation) this.connection.close();
      else {
        this.error = undefined;
        this.phase = "live";
        this.emit();
      }
    } catch (error) {
      if (generation === this.generation && !abort.signal.aborted) {
        this.queueReconnect(filters, generation, error);
      }
    }
  }

  private queueReconnect(
    filters: LogFilters,
    generation: number,
    error: unknown,
  ): void {
    if (this.cancelReconnect || generation !== this.generation) return;
    this.connection?.close();
    this.connection = undefined;
    this.reconnectAttempt += 1;
    this.error = asLogError(error);
    this.phase = "reconnecting";
    this.emit();
    const delay = Math.min(1_000 * 2 ** (this.reconnectAttempt - 1), 30_000);
    this.cancelReconnect = this.schedule(() => {
      this.cancelReconnect = undefined;
      if (generation !== this.generation || !this.activeFilters) return;
      const lines = this.buffer.snapshot().lines;
      const newest =
        lines.length > 0 ? lines[lines.length - 1].timestamp : undefined;
      void this.connect(
        { ...this.activeFilters, startTime: newest ?? filters.startTime },
        generation,
      );
    }, delay);
  }

  private fail(error: unknown): void {
    this.error = asLogError(error);
    this.phase = "error";
    this.connection?.close();
    this.connection = undefined;
    this.emit();
  }

  private stopActive(): void {
    this.cancelReconnect?.();
    this.cancelReconnect = undefined;
    this.connection?.close();
    this.connection = undefined;
    this.abort?.abort();
    this.abort = undefined;
  }

  private emit(): void {
    const snapshot = this.snapshot();
    for (const listener of this.listeners) listener(snapshot);
  }
}

function asLogError(error: unknown): LogApiError {
  return error instanceof LogApiError
    ? error
    : new LogApiError(
        "unknown",
        error instanceof Error ? error.message : "Log request failed.",
      );
}
