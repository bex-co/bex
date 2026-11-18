import type { WorkspaceLoginRequirement } from "@/common/apollo/auth-error-link";
import {
  LOGIN_REQUIREMENT_SETTINGS,
  type LoginRequirementOutcomeCopy,
} from "@/features/workspaces/lib/login-requirements";

/**
 * Org Settings -> Security (w7/m466, docs/ADR012-auth.md 9c): the org form of
 * the workspace login settings, where Render puts "Require Login Method".
 * While one is on, every workspace of the org requires it beside its own
 * setting, and so do the org's own pages. Owners change them; Members and
 * Guests read them. Each takes the workspace setting's coded refusals (an
 * installation that cannot enforce it, the plan gate), with the lock-out
 * guard (the Owner does not meet it themselves) worded for the org; any
 * other refusal is worded as every org action's (`organizationErrorMessage`).
 */
interface OrganizationLoginRequirementSetting extends LoginRequirementOutcomeCopy {
  labelKey: string;
  descriptionKey: string;
  confirmTitleKey: string;
  confirmBodyKey: string;
}

export const ORGANIZATION_LOGIN_REQUIREMENT_SETTINGS: Readonly<
  Record<WorkspaceLoginRequirement, OrganizationLoginRequirementSetting>
> = {
  twoFactor: {
    labelKey: "organizations.requireTwoFactorLabel",
    descriptionKey: "organizations.requireTwoFactorDescription",
    confirmTitleKey: "organizations.requireTwoFactorConfirmTitle",
    confirmBodyKey: "organizations.requireTwoFactorConfirm",
    enabledToastKey: "organizations.requireTwoFactorEnabledToast",
    disabledToastKey: "organizations.requireTwoFactorDisabledToast",
    errorTitleKey: LOGIN_REQUIREMENT_SETTINGS.twoFactor.errorTitleKey,
    errorKey: LOGIN_REQUIREMENT_SETTINGS.twoFactor.errorKey,
    refusals: {
      ...LOGIN_REQUIREMENT_SETTINGS.twoFactor.refusals,
      WORKSPACE_2FA_ADMIN_NOT_ENROLLED: {
        messageKey: "organizations.requireTwoFactorErrorOwnerNotEnrolled",
        accountFix: true,
      },
    },
  },
  googleLogin: {
    labelKey: "organizations.requireGoogleLoginLabel",
    descriptionKey: "organizations.requireGoogleLoginDescription",
    confirmTitleKey: "organizations.requireGoogleLoginConfirmTitle",
    confirmBodyKey: "organizations.requireGoogleLoginConfirm",
    enabledToastKey: "organizations.requireGoogleLoginEnabledToast",
    disabledToastKey: "organizations.requireGoogleLoginDisabledToast",
    errorTitleKey: LOGIN_REQUIREMENT_SETTINGS.googleLogin.errorTitleKey,
    errorKey: LOGIN_REQUIREMENT_SETTINGS.googleLogin.errorKey,
    refusals: {
      ...LOGIN_REQUIREMENT_SETTINGS.googleLogin.refusals,
      WORKSPACE_GOOGLE_LOGIN_ADMIN_NOT_SIGNED_IN: {
        messageKey: "organizations.requireGoogleLoginErrorOwnerNotSignedIn",
        accountFix: true,
      },
    },
  },
};
