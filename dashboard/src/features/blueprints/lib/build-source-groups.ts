import type { BuildSourceView } from "@/features/build-sources/types";

/** One group of the table: a project's build sources, or the ungrouped ones. */
export interface BuildSourceGroup {
  /** The project, or null for the build sources in none. */
  project: { id: string; name: string } | null;
  sources: BuildSourceView[];
}

/**
 * Render groups a build sources table by project: the ungrouped build
 * sources first, then each project's, the projects by name
 * (`artifact-sources-table-utils`).
 */
export function groupBuildSources(
  sources: readonly BuildSourceView[],
  projectName: (projectId: string) => string | undefined,
): BuildSourceGroup[] {
  const ungrouped: BuildSourceView[] = [];
  const byProject = new Map<string, BuildSourceGroup>();
  for (const source of sources) {
    if (!source.projectId) {
      ungrouped.push(source);
      continue;
    }
    let group = byProject.get(source.projectId);
    if (!group) {
      group = {
        project: {
          id: source.projectId,
          name: projectName(source.projectId) ?? source.projectId,
        },
        sources: [],
      };
      byProject.set(source.projectId, group);
    }
    group.sources.push(source);
  }
  const projects = [...byProject.values()].sort((a, b) =>
    a.project!.name.localeCompare(b.project!.name),
  );
  return ungrouped.length > 0
    ? [{ project: null, sources: ungrouped }, ...projects]
    : projects;
}
