import type { WorkspaceLoginRequirement } from "@/common/apollo/auth-error-link";
import type { WorkspaceView } from "@/features/workspaces/types";

/**
 * One coded refusal of a login requirement's mutation, with the locale string
 * the card shows for it: the server's wording is not what the dashboard
 * decides by or shows. `accountFix` marks the lock-out guard, which the acting
 * admin fixes on their own account (enroll a second factor, sign in with
 * Google), so the card offers that next to the message.
 */
interface LoginRequirementRefusal {
  messageKey: string;
  accountFix: boolean;
}

/**
 * Workspace Settings' two login requirements, Render's "Enforce 2FA & Google
 * login" (https://render.com/docs/login-settings): what the card, its
 * skeleton and the mutation hook need to know about each. Both are admin-only,
 * Pro plan or higher (w7/070), readable by every member, and enforced
 * server-side; the card only writes the setting.
 */
interface LoginRequirementSetting {
  /** The settings section's anchor, and the stem of the control's ids. */
  section: string;
  required: (
    workspace: Pick<WorkspaceView, "requireTwoFactor" | "requireGoogleLogin">,
  ) => boolean;
  titleKey: string;
  descriptionKey: string;
  switchLabelKey: string;
  /** The notice below Pro, and the text of the server's PLAN_FEATURE_REQUIRED. */
  proPlanOnlyKey: string;
  enabledToastKey: string;
  disabledToastKey: string;
  errorTitleKey: string;
  /** A failure with no recognised code. */
  errorKey: string;
  refusals: Readonly<Record<string, LoginRequirementRefusal>>;
}

export const LOGIN_REQUIREMENT_SETTINGS: Readonly<
  Record<WorkspaceLoginRequirement, LoginRequirementSetting>
> = {
  twoFactor: {
    section: "two-factor",
    required: (workspace) => workspace.requireTwoFactor,
    titleKey: "workspaces.twoFactorTitle",
    descriptionKey: "workspaces.twoFactorDescription",
    switchLabelKey: "workspaces.twoFactorSwitchLabel",
    proPlanOnlyKey: "workspaces.twoFactorProPlanOnly",
    enabledToastKey: "workspaces.twoFactorEnabledToast",
    disabledToastKey: "workspaces.twoFactorDisabledToast",
    errorTitleKey: "workspaces.twoFactorErrorTitle",
    errorKey: "workspaces.twoFactorError",
    refusals: {
      WORKSPACE_2FA_ADMIN_NOT_ENROLLED: {
        messageKey: "workspaces.twoFactorErrorAdminNotEnrolled",
        accountFix: true,
      },
      WORKSPACE_2FA_UNAVAILABLE: {
        messageKey: "workspaces.twoFactorErrorUnavailable",
        accountFix: false,
      },
      PLAN_FEATURE_REQUIRED: {
        messageKey: "workspaces.twoFactorProPlanOnly",
        accountFix: false,
      },
    },
  },
  googleLogin: {
    section: "google-login",
    required: (workspace) => workspace.requireGoogleLogin,
    titleKey: "workspaces.googleLoginTitle",
    descriptionKey: "workspaces.googleLoginDescription",
    switchLabelKey: "workspaces.googleLoginSwitchLabel",
    proPlanOnlyKey: "workspaces.googleLoginProPlanOnly",
    enabledToastKey: "workspaces.googleLoginEnabledToast",
    disabledToastKey: "workspaces.googleLoginDisabledToast",
    errorTitleKey: "workspaces.googleLoginErrorTitle",
    errorKey: "workspaces.googleLoginError",
    refusals: {
      WORKSPACE_GOOGLE_LOGIN_ADMIN_NOT_SIGNED_IN: {
        messageKey: "workspaces.googleLoginErrorAdminNotSignedIn",
        accountFix: true,
      },
      WORKSPACE_GOOGLE_LOGIN_UNAVAILABLE: {
        messageKey: "workspaces.googleLoginErrorUnavailable",
        accountFix: false,
      },
      PLAN_FEATURE_REQUIRED: {
        messageKey: "workspaces.googleLoginProPlanOnly",
        accountFix: false,
      },
    },
  },
};
