import { isLifecycleSuspended } from "../services/lifecycle";

// w6/m141/t003: the status/suspended presentation predicates that lived here
// (ACTIONABLE_STATUSES / datastoreLifecycleTransition) are superseded by the
// generated per-resource decisions (databaseActions / keyValueActions,
// normalized by toResourceSnapshot and read through resourceDecision +
// presentAction). Eligibility comes from the server projection — never from
// the datastore's status or suspension, which the projection's execute paths
// already account for. What remains is convergence: proving a suspend/resume
// mutation took effect on refresh.
export function datastoreSuspensionConverged(
  action: "suspend" | "resume",
  suspended: boolean | string | null,
): boolean {
  return isLifecycleSuspended(suspended) === (action === "suspend");
}
