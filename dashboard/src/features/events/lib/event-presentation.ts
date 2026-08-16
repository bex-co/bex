// Style mapping for the service activity feed: a service event's type/status to
// its icon-chip colour and its badge variant. Pure functions, no JSX — kept out
// of event-icon.tsx so the component file exports only a component.

export function eventIconClass(
  type: string,
  status: string,
  factStatus: string,
): string {
  if (
    status === "update_failed" ||
    factStatus === "failed" ||
    type === "image_pull_failed" ||
    type === "server_failed"
  ) {
    return "bg-destructive/10 text-destructive";
  }
  if (
    factStatus === "succeeded" ||
    (type === "deploy_ended" && status === "live")
  ) {
    return "bg-emerald-500/10 text-emerald-600 dark:text-emerald-400";
  }
  if (
    type === "deploy_started" ||
    type === "build_started" ||
    type === "pre_deploy_started"
  ) {
    return "bg-primary/10 text-primary";
  }
  return "bg-muted text-muted-foreground";
}

// A lifecycle-step fact's status (w7/m66) → a Badge variant. succeeded reads as
// the default (accent), failed as destructive, canceled as a muted outline.
export function lifecycleStatusVariant(
  status: string,
): "default" | "destructive" | "outline" {
  if (status === "failed") return "destructive";
  if (status === "canceled") return "outline";
  return "default";
}
