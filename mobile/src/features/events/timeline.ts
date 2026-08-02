export type TimelineDeploy = {
  id: string | null;
  status: string | null;
  createdAt: string | null;
  updatedAt: string | null;
  commitId: string | null;
  commitMessage: string | null;
  image: string | null;
  trigger: string | null;
};

export type TimelineEvent = {
  id: string | null;
  cursor: string | null;
  type: string | null;
  timestamp: string | null;
  details: {
    deployId?: string | null;
    deployStatus?: string | null;
    status?: string | null;
    commitId?: string | null;
    commitMessage?: string | null;
    image?: string | null;
    branchFrom?: string | null;
    branchTo?: string | null;
    fromCount?: number | null;
    toCount?: number | null;
    reasonCode?: string | null;
  } | null;
};

export type TimelineItem =
  | { kind: "deploy"; key: string; timestamp: string; deploy: TimelineDeploy }
  | { kind: "event"; key: string; timestamp: string; event: TimelineEvent };

const validTimestamp = (value: string | null): string | null => {
  if (!value || Number.isNaN(Date.parse(value))) return null;
  return value;
};

export function mergeTimeline(
  deploys: readonly (TimelineDeploy | null)[],
  events: readonly (TimelineEvent | null)[],
): TimelineItem[] {
  const deployItems: TimelineItem[] = deploys.flatMap((deploy) => {
    if (!deploy?.id) return [];
    const timestamp = validTimestamp(deploy.updatedAt ?? deploy.createdAt);
    return timestamp
      ? [{ kind: "deploy", key: `deploy:${deploy.id}`, timestamp, deploy }]
      : [];
  });
  const eventItems: TimelineItem[] = events.flatMap((event) => {
    if (!event?.id) return [];
    const timestamp = validTimestamp(event.timestamp);
    return timestamp
      ? [{ kind: "event", key: `event:${event.id}`, timestamp, event }]
      : [];
  });
  return [...deployItems, ...eventItems].sort(
    (left, right) => Date.parse(right.timestamp) - Date.parse(left.timestamp),
  );
}

export function appendUnique<T>(
  current: readonly T[],
  incoming: readonly T[],
  identity: (item: T) => string | null,
): T[] {
  const seen = new Set(current.map(identity).filter(Boolean));
  return [
    ...current,
    ...incoming.filter((item) => {
      const id = identity(item);
      if (!id || seen.has(id)) return false;
      seen.add(id);
      return true;
    }),
  ];
}

const KNOWN_EVENT_TYPES = new Set([
  "deploy_started",
  "deploy_ended",
  "build_started",
  "build_ended",
  "pre_deploy_started",
  "pre_deploy_ended",
  "server_available",
  "server_unavailable",
  "plan_changed",
  "branch_changed",
  "instance_count_changed",
  "autoscaling_config_changed",
  "job_run_started",
  "job_run_ended",
]);

export function knownEventType(type: string | null): boolean {
  return Boolean(type && KNOWN_EVENT_TYPES.has(type.toLowerCase()));
}
