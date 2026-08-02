import type { LogFilters } from "./types";

const repeatedKeys = [
  "types",
  "level",
  "instance",
  "host",
  "statusCode",
  "method",
  "path",
] as const;

export function buildLogQuery(filters: LogFilters): string {
  const params = new URLSearchParams();
  params.append("resource", filters.resource);
  if (filters.text) params.append("text", filters.text);
  if (filters.startTime) params.append("startTime", filters.startTime);
  if (filters.endTime) params.append("endTime", filters.endTime);
  if (filters.limit) params.append("limit", String(filters.limit));
  if (filters.direction) params.append("direction", filters.direction);

  for (const key of repeatedKeys) {
    const values = filters[key];
    if (!values) continue;
    const wireKey = key === "types" ? "type" : key;
    for (const value of values) params.append(wireKey, value);
  }
  return params.toString();
}

/** Filters the kubelet-backed live stream cannot honor. History remains valid. */
export function hasStoreOnlyTailFilters(filters: LogFilters): boolean {
  return (
    filters.types?.includes("request") === true ||
    (filters.level?.length ?? 0) > 0 ||
    (filters.host?.length ?? 0) > 0 ||
    (filters.statusCode?.length ?? 0) > 0 ||
    (filters.method?.length ?? 0) > 0 ||
    (filters.path?.length ?? 0) > 0
  );
}
