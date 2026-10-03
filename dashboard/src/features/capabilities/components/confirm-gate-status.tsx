/**
 * Why an open bound confirmation cannot dispatch right now (w4/m159): access
 * is being rechecked or could not be confirmed. The line is always rendered so
 * the dialog keeps its geometry while a routine refresh comes and goes.
 */
export function ConfirmGateStatus({ reason }: { reason?: string }) {
  return (
    <p
      role="status"
      aria-live="polite"
      className="min-h-5 text-sm text-muted-foreground"
    >
      {reason}
    </p>
  );
}
