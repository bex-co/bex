import { useState } from "react";
import { Card, CardContent } from "@/common/components/ui/card";
import { ConfirmDialog } from "@/common/components/confirm-dialog";
import type { WorkspaceLoginRequirement } from "@/common/apollo/auth-error-link";
import { useTranslations } from "@/common/hooks/use-translations";
import { LoginRequirementRefusalAlert } from "@/features/workspaces/components/login-requirement-refusal-alert";
import {
  LOGIN_REQUIREMENTS,
  LOGIN_REQUIREMENT_SETTINGS,
} from "@/features/workspaces/lib/login-requirements";
import { useSetOrganizationLoginRequirement } from "@/features/organizations/hooks/use-set-organization-login-requirement";
import { ORGANIZATION_LOGIN_REQUIREMENT_SETTINGS } from "@/features/organizations/lib/organization-login-requirements";
import {
  OrganizationLoginRequirementSection,
  OrganizationSecurityCardHeader,
  OrganizationSecurityOwnerOnly,
} from "@/features/organizations/components/organization-settings-frame";
import type { Organization } from "@/features/organizations/types";

/**
 * Org Settings -> Security (w7/m466, docs/ADR012-auth.md 9c): "Require
 * two-factor authentication" and "Require Google login", the org form of the
 * workspace login settings, in the section where Render puts "Require Login
 * Method". While one is on, every workspace of the org requires it beside its
 * own setting, and so do the org's own pages.
 *
 * Owners get the switches, everyone else the state and a line saying only
 * Owners change it. Turning one on asks first, since it can lock members and
 * their keys out at once; turning one off does not, as on the workspace
 * cards. The server's coded refusals show beside the switch: the lock-out
 * guard (the Owner does not meet it themselves) with the way to fix it on
 * their own account, and an installation that cannot enforce it.
 * `OrganizationSettingsSkeleton` previews this anatomy.
 */
export function OrganizationSecurityCard({
  organization,
  canChange,
}: {
  organization: Organization;
  canChange: boolean;
}) {
  return (
    <Card>
      <OrganizationSecurityCardHeader />
      <CardContent className="space-y-6">
        {LOGIN_REQUIREMENTS.map((requirement) => (
          <OrganizationLoginRequirementSetting
            key={requirement}
            requirement={requirement}
            organization={organization}
            canChange={canChange}
          />
        ))}
        {canChange ? null : <OrganizationSecurityOwnerOnly />}
      </CardContent>
    </Card>
  );
}

function OrganizationLoginRequirementSetting({
  requirement,
  organization,
  canChange,
}: {
  requirement: WorkspaceLoginRequirement;
  organization: Organization;
  canChange: boolean;
}) {
  const { t } = useTranslations();
  const setting = LOGIN_REQUIREMENT_SETTINGS[requirement];
  const copy = ORGANIZATION_LOGIN_REQUIREMENT_SETTINGS[requirement];
  const { setRequired, busy, error } = useSetOrganizationLoginRequirement(
    requirement,
    organization,
  );
  const [confirming, setConfirming] = useState(false);

  return (
    <OrganizationLoginRequirementSection
      requirement={requirement}
      control={canChange ? "switch" : "status"}
      required={organization[setting.field]}
      disabled={busy}
      onCheckedChange={(next) => {
        if (next) setConfirming(true);
        else void setRequired(false);
      }}
    >
      {error ? (
        <LoginRequirementRefusalAlert
          requirement={requirement}
          title={t(copy.errorTitleKey)}
          error={error}
        />
      ) : null}
      {canChange ? (
        <ConfirmDialog
          open={confirming}
          onOpenChange={setConfirming}
          title={t(copy.confirmTitleKey, { organization: organization.name })}
          description={t(copy.confirmBodyKey)}
          confirmLabel={t("organizations.requireConfirmAction")}
          cancelLabel={t("organizations.cancel")}
          destructive={false}
          onConfirm={() => void setRequired(true)}
        />
      ) : null}
    </OrganizationLoginRequirementSection>
  );
}
