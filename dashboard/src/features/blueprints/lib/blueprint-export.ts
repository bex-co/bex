import type { HistoryState } from "@tanstack/history";
import type { Translate } from "@/common/hooks/use-translations";
import type { ResourceRow } from "@/features/projects/types";

/** The kinds `generateBlueprint` exports, in the order Render names them. */
export const BLUEPRINT_EXPORT_KINDS = [
  "service",
  "database",
  "keyvalue",
  "envgroup",
] as const;

export type BlueprintExportKind = (typeof BLUEPRINT_EXPORT_KINDS)[number];

export interface BlueprintExportResource {
  kind: BlueprintExportKind;
  id: string;
  name: string;
}

/**
 * Where Generate Blueprint was opened from, Render's `source` values
 * (`ExportBlueprint-*.js`): the selection bar, a row's menu, or a project
 * environment's menu, which also names the project and the environment.
 */
const SOURCES = [
  "action bar",
  "table row menu",
  "project environment menu",
] as const;

export type BlueprintExportSource = (typeof SOURCES)[number];

export interface BlueprintExport {
  resources: BlueprintExportResource[];
  source: BlueprintExportSource;
  projectName?: string;
  environmentName?: string;
}

declare module "@tanstack/history" {
  interface HistoryState {
    /**
     * The resources Generate Blueprint opens `/blueprint/generate` with,
     * handed over in the navigation's state as Render's are (w3/m681).
     */
    blueprintExport?: BlueprintExport;
  }
}

function isKind(value: unknown): value is BlueprintExportKind {
  return (BLUEPRINT_EXPORT_KINDS as readonly unknown[]).includes(value);
}

function isResource(value: unknown): value is BlueprintExportResource {
  if (typeof value !== "object" || value === null) return false;
  const { kind, id, name } = value as Record<string, unknown>;
  return isKind(kind) && typeof id === "string" && typeof name === "string";
}

/**
 * The export a navigation handed over, or null when there is none: a pasted
 * link, a server render (the state lives in the browser's history), or a
 * history entry another build wrote. Resources of a kind bex cannot export
 * are dropped, so an export with none left is no export, and the page goes to
 * `/` as Render's does with nothing selected.
 */
export function readBlueprintExport(
  state: HistoryState | undefined,
): BlueprintExport | null {
  const handed = state?.blueprintExport as unknown;
  if (typeof handed !== "object" || handed === null) return null;
  const { resources, source, projectName, environmentName } = handed as Record<
    string,
    unknown
  >;
  if (!Array.isArray(resources)) return null;
  const kept = resources.filter(isResource);
  if (kept.length === 0) return null;
  return {
    resources: kept,
    source: (SOURCES as readonly unknown[]).includes(source)
      ? (source as BlueprintExportSource)
      : "action bar",
    ...(typeof projectName === "string" && { projectName }),
    ...(typeof environmentName === "string" && { environmentName }),
  };
}

/**
 * The rows of a resource list Generate Blueprint can export: services,
 * Postgres, Key Value and environment groups. A sandbox group has no
 * render.yaml form, and Render's selection bar leaves it out too.
 */
export function exportableResources(
  rows: readonly ResourceRow[],
): BlueprintExportResource[] {
  return rows.flatMap((row) =>
    isKind(row.kind) ? [{ kind: row.kind, id: row.id, name: row.name }] : [],
  );
}

/** The resources in the order Render names them: services, then Postgres,
 *  then Key Value, then environment groups, each in the order given. */
export function orderedExportResources(
  resources: readonly BlueprintExportResource[],
): BlueprintExportResource[] {
  return BLUEPRINT_EXPORT_KINDS.flatMap((kind) =>
    resources.filter((resource) => resource.kind === kind),
  );
}

/** `generateBlueprint`'s id lists for an export. */
export function blueprintExportVariables(
  resources: readonly BlueprintExportResource[],
) {
  const ids = (kind: BlueprintExportKind) =>
    resources.filter((r) => r.kind === kind).map((r) => r.id);
  return {
    serviceIds: ids("service"),
    postgresIds: ids("database"),
    keyValueIds: ids("keyvalue"),
    envGroupIds: ids("envgroup"),
  };
}

/**
 * Whether the page is titled for the environment it came from: Render's
 * "Create a new Blueprint from <project> / <environment>" needs the
 * environment menu's names, and every other entry reads "from your services".
 */
export function environmentTitleNames(
  handed: BlueprintExport,
): { project: string; environment: string } | null {
  if (handed.source !== "project environment menu") return null;
  if (!handed.projectName || !handed.environmentName) return null;
  return { project: handed.projectName, environment: handed.environmentName };
}

/**
 * The page's title, also its document title: "Create a new Blueprint from
 * <project> / <environment>" when it came from an environment's menu, else
 * "from your services", as Render's.
 */
export function generateBlueprintTitle(
  t: Translate,
  handed: BlueprintExport | null | undefined,
): string {
  const names = handed ? environmentTitleNames(handed) : null;
  return names
    ? t("blueprints.generatePageTitleEnvironment", names)
    : t("blueprints.generatePageTitle");
}
