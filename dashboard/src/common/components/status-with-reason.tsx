/**
 * A datastore's status with, beneath it, why it is unavailable (w4/m170,
 * w5/m129). Both strings arrive translated.
 */
export function StatusWithReason({
  status,
  reason,
}: {
  status: string;
  reason?: string | null;
}) {
  if (!reason) return status;
  return (
    <span className="flex flex-col gap-0.5">
      <span>{status}</span>
      <span className="text-muted-foreground text-xs">{reason}</span>
    </span>
  );
}
