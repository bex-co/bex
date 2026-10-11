import { useEffect, useMemo, useState } from "react";
import { useApolloClient } from "@apollo/client/react";
import { DeployPreDeployEndDocument } from "@/graphql/definitions";
import type { DeployView } from "./use-deploy";

// bex-api's serviceEvents page cap (Render's paging range).
const EVENTS_PAGE = 100;

// The step-end fact's `at` comes from the cluster's Job completion clock while
// updatedAt is Postgres's settle time; this margin keeps a little skew between
// the two from putting the fact just past the search ceiling.
const CEILING_SKEW_MS = 60_000;

// Event timestamps are RFC3339 at whole-second precision (events/graphql.go)
// while log timestamps carry nanoseconds, so the step's final lines can sit
// later in the very second its end fact names. The log bound is inclusive of
// that whole second.
const EVENT_PRECISION_MS = 1000;

/**
 * How the canceled deploy's continued pre-deploy step bounds its log window:
 * - `none`: not a canceled deploy with a finished step — use the normal bounds;
 * - `checking`: resolving the step's end fact (don't show the truncated window);
 * - `found`: `end` is the diagnostic log end, never before deploy finishedAt;
 * - `missing` / `error`: no fact (legacy) or the read failed — the
 *   cancellation-bound window applies and may omit later step output.
 */
export type CanceledStepEnd =
  | { status: "none" }
  | { status: "checking" }
  | { status: "found"; end: string }
  | { status: "missing" }
  | { status: "error"; retry: () => void };

interface StepEndRequest {
  key: string;
  serviceId: string;
  deployId: string;
  startTime: string;
  ceiling: string;
  finishedAt: string;
}

/**
 * A canceled deploy whose pre-deploy step kept running reaches its succeeded/
 * failed verdict after the cancellation closed the deploy (ADR004, w5/085), so
 * that step's last output lies after deploy.finishedAt. Only such a deploy
 * needs the step-end fact; a step that was actually stopped stays "canceled".
 */
function stepEndRequest(
  serviceId: string,
  deployId: string | undefined,
  status: string | undefined,
  preDeployStatus: string | undefined,
  createdAt: string | null | undefined,
  updatedAt: string | null | undefined,
  finishedAt: string | null | undefined,
  attempt: number,
): StepEndRequest | null {
  if (
    !deployId ||
    status !== "canceled" ||
    (preDeployStatus !== "succeeded" && preDeployStatus !== "failed") ||
    !createdAt ||
    !finishedAt
  )
    return null;
  // updatedAt moves when the step's verdict settles, at or after its fact.
  const settled = Date.parse(updatedAt ?? finishedAt);
  if (!Number.isFinite(settled)) return null;
  const ceiling = new Date(settled + CEILING_SKEW_MS).toISOString();
  return {
    key: [serviceId, deployId, createdAt, ceiling, attempt].join("\0"),
    serviceId,
    deployId,
    startTime: createdAt,
    ceiling,
    finishedAt,
  };
}

/** The later of the cancellation and the step's (whole-second) end fact. */
export function stepLogEnd(finishedAt: string, factTimestamp: string): string {
  const fact = Date.parse(factTimestamp) + EVENT_PRECISION_MS;
  return Number.isFinite(fact) && fact > Date.parse(finishedAt)
    ? new Date(fact).toISOString()
    : finishedAt;
}

type Outcome =
  | { status: "found"; end: string }
  | { status: "missing" }
  | { status: "error" };

/**
 * Resolves the deploy's own pre_deploy_ended fact over a frozen
 * [createdAt, settled+skew] interval, following cursor pages until the fact
 * with this deploy's details.deployId turns up or the interval is exhausted —
 * another deploy's step end in the same interval never counts.
 */
export function useCanceledStepEnd(
  serviceId: string,
  deploy: DeployView | null,
): CanceledStepEnd {
  const client = useApolloClient();
  const [attempt, setAttempt] = useState(0);
  const [outcome, setOutcome] = useState<{ key: string; value: Outcome }>();
  const request = useMemo(
    () =>
      stepEndRequest(
        serviceId,
        deploy?.id,
        deploy?.status,
        deploy?.preDeployStatus,
        deploy?.createdAt,
        deploy?.updatedAt,
        deploy?.finishedAt,
        attempt,
      ),
    [
      serviceId,
      deploy?.id,
      deploy?.status,
      deploy?.preDeployStatus,
      deploy?.createdAt,
      deploy?.updatedAt,
      deploy?.finishedAt,
      attempt,
    ],
  );

  useEffect(() => {
    if (!request) return;
    let live = true;
    const settle = (value: Outcome) => {
      if (live) setOutcome({ key: request.key, value });
    };
    void (async () => {
      let cursor: string | undefined;
      for (;;) {
        const { data } = await client.query({
          query: DeployPreDeployEndDocument,
          variables: {
            serviceId: request.serviceId,
            startTime: request.startTime,
            endTime: request.ceiling,
            cursor,
            limit: EVENTS_PAGE,
          },
          fetchPolicy: "network-only",
        });
        const page = (data?.serviceEvents ?? []).filter((e) => !!e);
        const match = page.find(
          (e) => e.details?.deployId === request.deployId && e.timestamp,
        );
        if (match?.timestamp)
          return settle({
            status: "found",
            end: stepLogEnd(request.finishedAt, match.timestamp),
          });
        const next = page.at(-1)?.cursor ?? undefined;
        if (page.length < EVENTS_PAGE || !next || next === cursor)
          return settle({ status: "missing" });
        cursor = next;
      }
    })().catch(() => settle({ status: "error" }));
    return () => {
      live = false;
    };
  }, [client, request]);

  if (!request) return { status: "none" };
  if (outcome?.key !== request.key) return { status: "checking" };
  if (outcome.value.status === "error")
    return { status: "error", retry: () => setAttempt((n) => n + 1) };
  return outcome.value;
}
