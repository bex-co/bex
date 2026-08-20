/**
 * Structured server-side log lines for the dashboard pod (w4/m88).
 *
 * One JSON object per line on stdout so k9s/`kubectl logs` and the Alloy
 * shipper (JSON `level` parse) can see SSR/route failures. Never log cookies,
 * Authorization, GraphQL variables, or other request secrets — only bounded
 * identity: level, msg, optional path, optional status.
 */

export type ServerLogLevel = "error" | "warn" | "info";

export type ServerLogFields = {
  level: ServerLogLevel;
  msg: string;
  path?: string;
  status?: number;
};

const MSG_MAX = 512;
const PATH_MAX = 256;

export type ServerLogWriter = {
  write(chunk: string): void;
};

function truncate(value: string, max: number): string {
  if (value.length <= max) return value;
  return `${value.slice(0, max - 1)}…`;
}

/**
 * Write one JSON log line. `out` defaults to `process.stdout` so production
 * pods surface lines in the container stream; tests inject a buffer.
 */
export function writeServerLog(
  fields: ServerLogFields,
  out: ServerLogWriter = process.stdout,
): void {
  const line: Record<string, string | number> = {
    level: fields.level,
    msg: truncate(String(fields.msg ?? ""), MSG_MAX),
  };
  if (fields.path != null && fields.path !== "") {
    line.path = truncate(String(fields.path), PATH_MAX);
  }
  if (fields.status != null && Number.isFinite(fields.status)) {
    line.status = fields.status;
  }
  out.write(`${JSON.stringify(line)}\n`);
}

/** Convenience: error-level route/SSR failure. */
export function logServerError(
  fields: Omit<ServerLogFields, "level">,
  out?: ServerLogWriter,
): void {
  writeServerLog({ level: "error", ...fields }, out);
}
