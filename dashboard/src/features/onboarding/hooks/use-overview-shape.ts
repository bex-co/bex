import { useMemo } from "react";
import { createWorkspaceLocalStore } from "../lib/workspace-local-store";

/**
 * The Overview layout a workspace last rendered: the getting-started layout,
 * or the resource list with how many project cards and ungrouped rows it drew
 * (capped — a preview, not a census). The pending skeleton cannot know which
 * one is coming before the data arrives, so it previews the last one this
 * browser saw (AGENTS.md skeleton rule; w3/m176, w3/047). Unknown workspaces
 * preview two projects and two rows.
 */
export type OverviewShape =
  { kind: "empty" } | { kind: "populated"; projects: number; rows: number };

const MAX_PROJECTS = 6;
const MAX_ROWS = 8;
const UNKNOWN = "populated:2:2";
const SHAPE_RE = /^(empty|populated:\d+:\d+)$/;

const store = createWorkspaceLocalStore<string>(
  "bex.overviewShape",
  UNKNOWN,
  (raw) => (raw && SHAPE_RE.test(raw) ? raw : UNKNOWN),
);

function parse(raw: string): OverviewShape {
  if (raw === "empty") return { kind: "empty" };
  const [, projects, rows] = raw.split(":");
  return {
    kind: "populated",
    projects: Math.min(Number(projects), MAX_PROJECTS),
    rows: Math.min(Number(rows), MAX_ROWS),
  };
}

// The caps live in parse, so a stored value from anywhere renders a bounded
// preview; serialize stores the raw counts.
function serialize(shape: OverviewShape): string {
  return shape.kind === "empty"
    ? "empty"
    : `populated:${shape.projects}:${shape.rows}`;
}

export function useOverviewShape(workspaceId: string | null): OverviewShape {
  const raw = store.useValue(workspaceId);
  return useMemo(() => parse(raw), [raw]);
}

export function rememberOverviewShape(
  workspaceId: string | null,
  shape: OverviewShape,
) {
  store.write(workspaceId, serialize(shape));
}
