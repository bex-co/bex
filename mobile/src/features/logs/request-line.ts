import type { LogLine } from "./types";

// requestLead is what a request line shows before its message, as Render's Log
// Explorer does (`[GET] example.com/api/orders clientIP=…`): its method, status
// code, host and path are labels, not part of the message (w4/m325). "" for
// any other line.
export function requestLead(line: LogLine): string {
  const label = (name: string) =>
    line.labels.find((l) => l.name === name)?.value ?? "";
  if (label("type") !== "request") return "";
  const parts = [
    label("method") && `[${label("method")}]`,
    label("statusCode"),
    label("host") + label("path"),
  ].filter(Boolean);
  return parts.length ? `${parts.join(" ")} ` : "";
}
