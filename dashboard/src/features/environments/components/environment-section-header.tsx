import type { ReactNode } from "react";

/**
 * An environment section's header, shared by the ready section and its
 * loading preview so both lay out the same rows (w4/715, w1/m682). Render
 * draws the environment's name as the section's h2 (a protected one adds its
 * icon, and "Protected environment" under it), then one row holding the
 * counted switcher on the left and the section's actions on the right. The
 * name is one 28px line at every width: it truncates rather than wrapping.
 * Where the row is too narrow for both, the actions wrap under the switcher.
 */
export function EnvironmentSectionHeader({
  titleId,
  title,
  description,
  switcher,
  actions,
  titleAs = "h2",
}: {
  /** The heading's id, which names the section. */
  titleId?: string;
  /** The name and protected icon (or their skeleton bar). */
  title: ReactNode;
  /** "Protected environment", for a protected one. */
  description?: ReactNode;
  /** The All / Services / Env Groups switcher (or its preview). */
  switcher: ReactNode;
  /** The section's actions (or their reserved boxes). */
  actions: ReactNode;
  /** `div` for the loading preview, which names nothing yet. */
  titleAs?: "h2" | "div";
}) {
  const Title = titleAs;
  return (
    <div className="space-y-3">
      <div className="space-y-1">
        <Title
          id={titleId}
          className="flex h-7 min-w-0 items-center gap-2 text-lg font-semibold"
        >
          {title}
        </Title>
        {description}
      </div>
      <div className="flex flex-wrap items-center justify-between gap-3">
        {switcher}
        <div className="flex flex-wrap items-center gap-2">{actions}</div>
      </div>
    </div>
  );
}
