const LEGACY_ENVIRONMENT_ID = /^env-[0-9a-v]{20}$/;

/** Keep saved dashboard links compatible with the API's canonical evm- IDs.
 * Only the historical ID shape is rewritten; names and unknown formats remain
 * untouched. Discovery still determines whether the environment is selectable. */
export function canonicalEnvironmentLinkId(value: string): string {
  return value.length === 24 && LEGACY_ENVIRONMENT_ID.test(value)
    ? `evm-${value.slice(4)}`
    : value;
}
