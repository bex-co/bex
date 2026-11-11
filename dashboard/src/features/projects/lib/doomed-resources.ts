/** A resource a delete confirmation names: its id and display name. */
interface NamedResource {
  id: string;
  name: string;
}

/** The resources an environment or a project delete removes, by id. */
export interface DoomedResourceIds {
  serviceIds: readonly string[];
  databaseIds: readonly string[];
  keyValueIds: readonly string[];
}

/**
 * The names an environment or project delete confirmation lists: every
 * service, database and key value store it removes, as Render's dashboard
 * names what a delete takes (w4/m264, w6/m290, w4/542). A resource whose list
 * has not answered yet reads as its id, never as a count. Shared by both
 * dialogs so they cannot drift apart.
 */
export function doomedResourceNames(
  ids: DoomedResourceIds,
  lists: readonly (readonly NamedResource[])[],
): string[] {
  const names = new Map<string, string>(
    lists.flatMap((list) => list.map((r): [string, string] => [r.id, r.name])),
  );
  return [...ids.serviceIds, ...ids.databaseIds, ...ids.keyValueIds].map(
    (id) => names.get(id) ?? id,
  );
}
