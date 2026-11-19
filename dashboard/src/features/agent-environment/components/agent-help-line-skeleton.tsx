/**
 * An agent card's help line while the card loads, as real text kept
 * transparent, so it wraps as the ready line does at this width and in this
 * language. An admin reads `help` and a member reads `readOnly`. While the
 * role is unknown both share one cell, so the reserved height is the taller
 * one's; once it is known, only that caller's line is held (w7/094).
 */
export function AgentHelpLineSkeleton({
  help,
  readOnly,
  canManage,
  testId,
}: {
  help: string;
  readOnly: string;
  canManage?: boolean | null;
  testId: string;
}) {
  const lines =
    canManage == null ? [help, readOnly] : [canManage ? help : readOnly];
  return (
    <div
      className="text-muted-foreground grid text-xs"
      data-testid={testId}
      aria-hidden
    >
      {lines.map((line) => (
        <p key={line} className="invisible col-start-1 row-start-1">
          {line}
        </p>
      ))}
    </div>
  );
}
