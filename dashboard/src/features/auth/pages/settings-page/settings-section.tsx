import { useId, type ReactNode } from "react";

/**
 * A labelled grouping of related cards on the account Settings page — a
 * section heading + optional description above its cards. Introduced (w4/m15)
 * to give the Audit Log card a "Security & Compliance" home instead of sitting
 * as a bare sibling of Team/API Keys; the same shell is meant to host the
 * planned session-management (w4/006) and MFA (m11) cards.
 *
 * Renders a `<section>` labelled by its heading (an accessible `region`), so a
 * card can be asserted as living *under* a given section, not merely present.
 */
export function SettingsSection({
  id,
  title,
  description,
  children,
}: {
  id?: string;
  title: string;
  description?: string;
  children: ReactNode;
}) {
  const headingId = useId();
  return (
    <section
      id={id}
      aria-labelledby={headingId}
      className="scroll-mt-16 space-y-4 lg:scroll-mt-6"
    >
      <div className="space-y-1">
        <h2 id={headingId} className="text-lg font-semibold text-foreground">
          {title}
        </h2>
        {description ? (
          <p className="text-sm text-muted-foreground">{description}</p>
        ) : null}
      </div>
      {children}
    </section>
  );
}
