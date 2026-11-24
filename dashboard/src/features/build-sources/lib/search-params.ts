import type { BuildSourceKind } from "@/features/build-sources/types";

export interface BuildSourcesListSearch {
  q?: string;
  kind?: BuildSourceKind;
}

/**
 * Search validator for `/build-sources`. Returns every owned key (`undefined`
 * when omitted or rejected) so TanStack Router never leaks raw invalid params
 * into readers (`dashboard/AGENTS.md`, w5/078).
 */
export function parseBuildSourcesListSearch(
  raw: Record<string, unknown>,
): BuildSourcesListSearch {
  const rawQ = raw.q;
  const q =
    typeof rawQ === "string" && rawQ.trim() !== "" ? rawQ : undefined;
  const rawKind = raw.kind;
  const kind: BuildSourceKind | undefined =
    rawKind === "git" || rawKind === "image" ? rawKind : undefined;
  return { q, kind };
}

export interface NewBuildSourceSearch {
  projectId?: string;
}

/**
 * Search validator for `/build-sources/new`. Preserves an optional `projectId`
 * pre-selection from Project Overview while rejecting non-string/blank values.
 */
export function parseNewBuildSourceSearch(
  raw: Record<string, unknown>,
): NewBuildSourceSearch {
  const rawProjectId = raw.projectId;
  const projectId =
    typeof rawProjectId === "string" && rawProjectId.trim() !== ""
      ? rawProjectId.trim()
      : undefined;
  return { projectId };
}
