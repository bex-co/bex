export type LogType = "app" | "request" | "build" | "predeploy";

export type LogLabel = {
  name: string;
  value: string;
};

export type LogLine = {
  id: string;
  message: string;
  timestamp: string;
  labels: LogLabel[];
};

export type LogFilters = {
  resource: string;
  types?: LogType[];
  text?: string;
  startTime?: string;
  endTime?: string;
  limit?: number;
  direction?: "backward" | "forward";
  level?: string[];
  instance?: string[];
  host?: string[];
  statusCode?: string[];
  method?: string[];
  path?: string[];
};

export type LogHistoryPage = {
  hasMore: boolean;
  nextStartTime: string;
  nextEndTime: string;
  logs: LogLine[];
};

export type LogFailureCode =
  | "unauthorized"
  | "forbidden"
  | "store_unavailable"
  | "unavailable"
  | "invalid_filter"
  | "network"
  | "unknown";

export class LogApiError extends Error {
  constructor(
    readonly code: LogFailureCode,
    message: string,
    readonly status?: number,
  ) {
    super(message);
    this.name = "LogApiError";
  }
}

export type LogTailCallbacks = {
  onLine: (line: LogLine) => void;
  onError: (error: LogApiError) => void;
  onClose: () => void;
};

export type LogTailConnection = {
  close: () => void;
};

export interface LogTransport {
  history(filters: LogFilters, signal: AbortSignal): Promise<LogHistoryPage>;
  subscribe(
    filters: LogFilters,
    callbacks: LogTailCallbacks,
    signal: AbortSignal,
  ): Promise<LogTailConnection>;
}
