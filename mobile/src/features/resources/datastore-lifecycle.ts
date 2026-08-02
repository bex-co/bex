import { isLifecycleSuspended } from "../services/lifecycle";

export type DatastoreLifecycleTransition = "suspend" | "resume";

const ACTIONABLE_STATUSES = new Set(["available", "unavailable"]);

export function datastoreLifecycleTransition(resource: {
  status: string;
  suspended: boolean | string | null;
}): DatastoreLifecycleTransition | null {
  if (isLifecycleSuspended(resource.suspended)) {
    return resource.status.toLowerCase() === "deleting" ? null : "resume";
  }
  return ACTIONABLE_STATUSES.has(resource.status.toLowerCase())
    ? "suspend"
    : null;
}

export function datastoreSuspensionConverged(
  action: DatastoreLifecycleTransition,
  suspended: boolean | string | null,
): boolean {
  return isLifecycleSuspended(suspended) === (action === "suspend");
}
