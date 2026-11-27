import { inEnvironment } from "@/features/environments/lib/link-id";

/**
 * The section id a project page's `/environment/<id>` URL names for bex's
 * project-only resources, which have no environment (an old `?env=unassigned`
 * link lands there too, w1/m682).
 */
export const UNASSIGNED_SECTION = "unassigned";

/** Whether `id` names one of a project page's sections, in either id spelling. */
export function namesSection(
  environments: ReadonlyArray<{ id: string }>,
  id: string,
): boolean {
  return (
    id === UNASSIGNED_SECTION ||
    environments.some((environment) => inEnvironment(id, environment.id))
  );
}
