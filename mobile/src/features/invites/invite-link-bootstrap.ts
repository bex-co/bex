/**
 * Remove the bearer from navigation synchronously, before waiting on secure
 * storage. The capture promise keeps the snapshotted value alive without ever
 * publishing it through component state or rendered copy.
 */
export function bootstrapInviteLink(
  value: unknown,
  scrubRoute: () => void,
  capture: (value: unknown) => Promise<boolean>,
): Promise<boolean> {
  scrubRoute();
  return capture(value);
}
