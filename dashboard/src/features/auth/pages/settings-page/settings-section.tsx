import { useId, type ReactNode } from "react";
import { SECTION_NAVIGATION_TARGET_CLASS } from "@/common/components/section-navigation";
import { cn } from "@/common/lib/utils/utils";

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
      className={cn(SECTION_NAVIGATION_TARGET_CLASS, "space-y-4")}
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
