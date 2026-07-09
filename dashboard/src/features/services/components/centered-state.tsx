/**
 * A centered icon + title + body block for a panel's empty / error / unavailable
 * states (the un-carded inner content; the caller supplies the surrounding Card).
 * Shared by the service tabs (env-vars, custom-domains) so their states stay
 * visually identical.
 */
export function CenteredState({
  icon,
  title,
  body,
}: {
  icon: React.ReactNode;
  title: string;
  body: string;
}) {
  return (
    <div className="flex flex-col items-center justify-center py-12 text-center">
      <div className="text-muted-foreground/50 mb-3 [&_svg]:size-8">{icon}</div>
      <p className="mb-1 font-medium">{title}</p>
      <p className="text-muted-foreground text-sm">{body}</p>
    </div>
  );
}
