import { createWorkspaceLocalStore } from "../lib/workspace-local-store";

export type OverviewShape = "empty" | "populated";

const store = createWorkspaceLocalStore<OverviewShape>(
  "bex.overviewShape",
  "populated",
  (raw) => (raw === "empty" ? "empty" : "populated"),
);

/**
 * Which Overview layout a workspace last rendered. The pending skeleton cannot
 * know which one is coming before the data arrives, so it previews the last
 * one this browser saw (AGENTS.md skeleton rule; w3/m176). Unknown workspaces
 * preview the resource list.
 */
export const useOverviewShape = (workspaceId: string | null) =>
  store.useValue(workspaceId);

export const rememberOverviewShape = store.write;
