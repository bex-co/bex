import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/common/components/ui/card";
import type { WorkspaceLoginRequirement } from "@/common/apollo/auth-error-link";
import { useTranslations } from "@/common/hooks/use-translations";
import {
  LoginRequirementRow,
  OrganizationRequirementNote,
} from "@/features/workspaces/components/login-requirement-row";
import {
  LOGIN_REQUIREMENT_SETTINGS,
  requiringOrganization,
} from "@/features/workspaces/lib/login-requirements";
import type { WorkspaceOrganizationRef } from "@/features/workspaces/types";

/**
 * WorkspaceLoginRequirementCard's pending shape, for either requirement: its
 * title, description and switch label are static copy, so they render as
 * themselves; only the control, which depends on the caller's role and the
 * workspace's setting, is a placeholder: the box of the control the card
 * will draw, in the card's own min-h-6 row, so the row wraps the same way in
 * both. That is the switch's box (h-5 w-9) unless the caller is known not to
 * be an admin; then `status` is the state the card will show ("Required" or
 * "Not required") and the placeholder is that badge's box, which is wider
 * and at phone width can wrap under the label where a switch does not
 * (w7/089). No control is enabled before its deciding data exists.
 *
 * A workspace already known to be below Pro (`belowPro`) settles to the card
 * with its plan notice, so the preview carries the notice too: static copy,
 * without the admin's "Change plan" link, which waits for the role. A
 * workspace already known to be in an org that requires it (`organization`,
 * the workspace's org; w7/m466) settles to the card with the org's note, so
 * the preview reserves the note's footprint, the Owner's link included,
 * without its text.
 */
export function WorkspaceLoginRequirementCardSkeleton({
  requirement,
  status = null,
  belowPro = false,
  organization = null,
}: {
  requirement: WorkspaceLoginRequirement;
  /** Whether it is required, when the card will show the state (a
   *  non-admin's); null while the card may show the switch. */
  status?: boolean | null;
  belowPro?: boolean;
  organization?: WorkspaceOrganizationRef | null;
}) {
  const { t } = useTranslations();
  const setting = LOGIN_REQUIREMENT_SETTINGS[requirement];
  const lockedBy = requiringOrganization(requirement, organization);
  return (
    <Card data-testid={`workspace-${setting.section}-skeleton`} aria-hidden>
      <CardHeader>
        <CardTitle>{t(setting.titleKey)}</CardTitle>
        <CardDescription>
          {`${t(setting.descriptionKey)} ${t("workspaces.loginRequirementAdminOnly")}`}
        </CardDescription>
      </CardHeader>
      <CardContent className="space-y-4">
        <LoginRequirementRow
          id={`workspace-${setting.section}`}
          region={setting.section}
          label={t(setting.switchLabelKey)}
          control={status === null ? "pending" : "pendingStatus"}
          required={status ?? false}
        />
        {lockedBy ? (
          <OrganizationRequirementNote organization={lockedBy} pending />
        ) : null}
        {belowPro ? (
          <p className="text-muted-foreground text-xs">
            {t(setting.proPlanOnlyKey)}
          </p>
        ) : null}
      </CardContent>
    </Card>
  );
}
