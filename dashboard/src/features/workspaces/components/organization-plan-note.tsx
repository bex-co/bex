import { Link } from "@tanstack/react-router";
import { ReservedText } from "@/common/components/reserved-skeleton";
import { useTranslations } from "@/common/hooks/use-translations";
import type { OrganizationPlanOwner } from "@/features/workspaces/types";

const NOTE_CLASS = "text-muted-foreground relative text-sm";
const LINK_CLASS =
  "text-primary font-medium underline-offset-4 hover:underline";

/** The primary's page the note links to, and that link's words. */
const LINK_KEYS = {
  settings: "workspaces.orgPlanPrimarySettingsLink",
  billing: "workspaces.orgPlanPrimaryBillingLink",
} as const;

/**
 * A secondary workspace's plan line (w7/m464, ADR115 D2): the plan is its
 * organization's, managed on the org's primary workspace, as on Render ("The
 * org plan is managed from the primary workspace"). It links to the primary's
 * Workspace Settings or Billing (`target`) through the `/w/$` alias, which
 * selects that workspace first; a caller who is not in the primary gets the
 * sentence alone.
 *
 * The copy is static, so the pending frame draws this same paragraph with its
 * text reserved (`pending`): it wraps alike at every width and in every
 * language, and offers no link before the page is ready.
 */
export function OrganizationPlanNote({
  owner,
  target = "settings",
  pending = false,
}: {
  owner: OrganizationPlanOwner;
  target?: keyof typeof LINK_KEYS;
  pending?: boolean;
}) {
  const { t } = useTranslations();
  const primaryId = owner.primaryWorkspaceId;
  const linkText = t(LINK_KEYS[target]);
  const link = !primaryId ? null : pending ? (
    <span className={LINK_CLASS}>{linkText}</span>
  ) : (
    <Link
      to="/w/$"
      params={{ _splat: `${primaryId}/${target}` }}
      className={LINK_CLASS}
    >
      {linkText}
    </Link>
  );
  const content = (
    <>
      {t("workspaces.orgPlanSecondaryNote")}
      {link ? <> {link}</> : null}
    </>
  );

  return (
    <p
      className={NOTE_CLASS}
      data-skeleton-region={pending ? "organization-plan-note" : undefined}
      data-testid={pending ? undefined : "organization-plan-note"}
    >
      {pending ? <ReservedText>{content}</ReservedText> : content}
    </p>
  );
}
