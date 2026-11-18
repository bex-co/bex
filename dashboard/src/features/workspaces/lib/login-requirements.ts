import type {
  LoginRefusalOrganization,
  WorkspaceLoginRequirement,
} from "@/common/apollo/auth-error-link";
import type { Translate } from "@/common/hooks/use-translations";
import type { WorkspaceOrganizationRef } from "@/features/workspaces/types";

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
 * What a switch that writes a login requirement says once its mutation
 * settles: a workspace's (Workspace Settings) or an org's (Org Settings ->
 * Security, w7/m466). Both take the same coded refusals, each in its own
 * words.
 */
export interface LoginRequirementOutcomeCopy {
  enabledToastKey: string;
  disabledToastKey: string;
  errorTitleKey: string;
  /** A failure with no recognised code. */
  errorKey: string;
  refusals: Readonly<Record<string, LoginRequirementRefusal>>;
}

/** The field that holds a requirement on a workspace, on an org, and on a
 *  workspace's org. */
type LoginRequirementField = "requireTwoFactor" | "requireGoogleLogin";

/**
 * Workspace Settings' two login requirements, Render's "Enforce 2FA & Google
 * login" (https://render.com/docs/login-settings): what the card, its
 * skeleton, the mutation hook and the blocked pages need to know about each.
 * Both are admin-only, Pro plan or higher (w7/070), readable by every member,
 * and enforced server-side; the card only writes the setting. An org can
 * require either in every one of its workspaces (w7/m466), which locks the
 * workspace's own switch on.
 */
interface LoginRequirementSetting extends LoginRequirementOutcomeCopy {
  /** The settings section's anchor, and the stem of the control's ids. */
  section: string;
  field: LoginRequirementField;
  titleKey: string;
  descriptionKey: string;
  switchLabelKey: string;
  /** The notice below Pro, and the text of the server's PLAN_FEATURE_REQUIRED. */
  proPlanOnlyKey: string;
  /** The blocked page's heading and why, when the workspace's own setting
   *  refused; and when its org's did (w7/m466), naming the org. */
  blockedTitleKey: string;
  blockedBodyKey: string;
  blockedOrganizationTitleKey: string;
  blockedOrganizationBodyKey: string;
}

export const LOGIN_REQUIREMENT_SETTINGS: Readonly<
  Record<WorkspaceLoginRequirement, LoginRequirementSetting>
> = {
  twoFactor: {
    section: "two-factor",
    field: "requireTwoFactor",
    titleKey: "workspaces.twoFactorTitle",
    descriptionKey: "workspaces.twoFactorDescription",
    switchLabelKey: "workspaces.twoFactorSwitchLabel",
    proPlanOnlyKey: "workspaces.twoFactorProPlanOnly",
    enabledToastKey: "workspaces.twoFactorEnabledToast",
    disabledToastKey: "workspaces.twoFactorDisabledToast",
    errorTitleKey: "workspaces.twoFactorErrorTitle",
    errorKey: "workspaces.twoFactorError",
    blockedTitleKey: "workspaces.twoFactorBlockedTitle",
    blockedBodyKey: "workspaces.twoFactorBlockedBody",
    blockedOrganizationTitleKey: "workspaces.twoFactorBlockedOrganizationTitle",
    blockedOrganizationBodyKey: "workspaces.twoFactorBlockedOrganizationBody",
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
    field: "requireGoogleLogin",
    titleKey: "workspaces.googleLoginTitle",
    descriptionKey: "workspaces.googleLoginDescription",
    switchLabelKey: "workspaces.googleLoginSwitchLabel",
    proPlanOnlyKey: "workspaces.googleLoginProPlanOnly",
    enabledToastKey: "workspaces.googleLoginEnabledToast",
    disabledToastKey: "workspaces.googleLoginDisabledToast",
    errorTitleKey: "workspaces.googleLoginErrorTitle",
    errorKey: "workspaces.googleLoginError",
    blockedTitleKey: "workspaces.googleLoginBlockedTitle",
    blockedBodyKey: "workspaces.googleLoginBlockedBody",
    blockedOrganizationTitleKey:
      "workspaces.googleLoginBlockedOrganizationTitle",
    blockedOrganizationBodyKey: "workspaces.googleLoginBlockedOrganizationBody",
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

/** The two requirements in the order every settings surface lists them. */
export const LOGIN_REQUIREMENTS: readonly WorkspaceLoginRequirement[] = [
  "twoFactor",
  "googleLogin",
];

/**
 * `organization`, a workspace's org, when its own setting requires
 * `requirement` in all of its workspaces (w7/m466, docs/ADR012-auth.md 9c);
 * null when it requires nothing of the kind, or there is no org.
 */
export function requiringOrganization(
  requirement: WorkspaceLoginRequirement,
  organization: WorkspaceOrganizationRef | null,
): WorkspaceOrganizationRef | null {
  return organization?.[LOGIN_REQUIREMENT_SETTINGS[requirement].field]
    ? organization
    : null;
}

/**
 * A blocked page's heading and explanation for `requirement`: naming the
 * workspace when its own setting refused, or the org when the org's did
 * (w7/m466), which also holds the org's own pages, where there is no
 * workspace to name.
 */
export function loginBlockedCopy(
  t: Translate,
  requirement: WorkspaceLoginRequirement,
  organization: Pick<LoginRefusalOrganization, "name"> | null,
  workspace = "",
): { title: string; description: string } {
  const setting = LOGIN_REQUIREMENT_SETTINGS[requirement];
  return organization
    ? {
        title: t(setting.blockedOrganizationTitleKey, {
          organization: organization.name,
        }),
        description: t(setting.blockedOrganizationBodyKey),
      }
    : {
        title: t(setting.blockedTitleKey, { workspace }),
        description: t(setting.blockedBodyKey),
      };
}
