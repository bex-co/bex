import { canonicalLinkId } from "@/common/lib/link-id";

/** Keep saved dashboard links compatible with the API's canonical evm- IDs.
 * Only the historical env- ID shape is rewritten. Discovery still determines
 * whether the environment is selectable. */
export function canonicalEnvironmentLinkId(value: string): string {
  return canonicalLinkId(value, "env", "evm");
}

/** Whether a resource's `environmentId` names this environment, whichever of
 * the two id spellings either side carries. */
export function inEnvironment(
  environmentId: string | null | undefined,
  environment: string,
): boolean {
  return (
    environmentId != null &&
    canonicalEnvironmentLinkId(environmentId) ===
      canonicalEnvironmentLinkId(environment)
  );
}
