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
 * workspace's setting, is a placeholder. The placeholder is the switch's own
 * box (h-5 w-9), and the row keeps the card's min-h-6, so the swap shifts
 * nothing at any width (the row wraps the same way in both). No control is
 * enabled before its deciding data exists.
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
  belowPro = false,
  organization = null,
}: {
  requirement: WorkspaceLoginRequirement;
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
          control="pending"
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
