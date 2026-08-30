import type { TranslationEntry } from "@/i18n";

const enAuth: Record<string, TranslationEntry> = {
  "auth.loginTitle": {
    message: "Welcome back",
    description: "Login page hero title",
  },
  "auth.loginSubtitle": {
    message: "Sign in to your account",
    description: "Login page hero subtitle",
  },
  "auth.registerTitle": {
    message: "Create your account",
    description: "Registration page hero title",
  },
  "auth.registerSubtitle": {
    message: "Enter your details to get started",
    description: "Registration page hero subtitle",
  },
  "auth.forgotPasswordTitle": {
    message: "Reset your password",
    description: "Forgot-password page hero title",
  },
  "auth.forgotPasswordSubtitle": {
    message: "Enter the email for your account to get a one-time access code",
    description: "Forgot-password page hero subtitle",
  },
  "auth.resetPasswordTitle": {
    message: "Set a new password",
    description:
      "Document title for the post-recovery reset-password landing page (renders the settings flow's password field)",
  },
  "auth.verificationTitle": {
    message: "Verify your email",
    description: "Verification page hero title",
  },
  "auth.verificationSubtitle": {
    message: "Enter the email address associated with your account to continue",
    description:
      "Verification page hero subtitle (email-first Ory step; code arrives after)",
  },
  "auth.settingsTitle": {
    message: "Settings",
    description: "Account settings page heading",
  },
  "auth.settingsSubtitle": {
    message: "Manage your identity, integrations, access, and security.",
    description: "Account settings page subheading",
  },
  "auth.settingsNavigation": {
    message: "Settings sections",
    description: "Accessible label for the settings page section navigation",
  },
  "auth.accountSection": {
    message: "Account",
    description: "Settings page account section heading and navigation label",
  },
  "auth.accountSectionSubtitle": {
    message: "Update your profile, password, and two-factor authentication.",
    description: "Settings page account section description",
  },
  "auth.integrationsSection": {
    message: "Integrations",
    description:
      "Settings page integrations section heading and navigation label",
  },
  "auth.integrationsSectionSubtitle": {
    message: "Connect source control and private image registries.",
    description: "Settings page integrations section description",
  },
  "auth.accessSection": {
    message: "Access credentials",
    description: "Settings page access section heading and navigation label",
  },
  "auth.accessSectionSubtitle": {
    message: "Manage programmatic access and SSH identities.",
    description: "Settings page access section description",
  },
  "auth.securityComplianceSection": {
    message: "Security & Compliance",
    description:
      "Settings page section heading grouping security and audit cards",
  },
  "auth.securityComplianceSectionSubtitle": {
    message: "Account security controls and your workspace's audit trail.",
    description: "Settings page Security & Compliance section description",
  },
  "auth.dangerZoneSection": {
    message: "Danger zone",
    description: "Account settings destructive-actions section",
  },
  "auth.dangerZoneSectionSubtitle": {
    message: "Permanently remove your account and revoke all access.",
    description: "Account deletion section description",
  },
  "auth.deleteAccountTitle": {
    message: "Delete account",
    description: "Account deletion card title",
  },
  "auth.deleteAccountDescription": {
    message:
      "This permanently deletes your identity, credentials, and sole-member workspaces. Shared workspaces remain available to their other administrators.",
    description: "Account deletion consequences",
  },
  "auth.deleteAccountWillDelete": {
    message: "These workspaces will be deleted",
    description: "Account deletion preview heading for sole-member workspaces",
  },
  "auth.deleteAccountWillLeave": {
    message: "You will leave these workspaces",
    description: "Account deletion preview heading for shared workspaces",
  },
  "auth.deleteAccountBlockedTitle": {
    message: "Resolve workspace ownership first",
    description: "Account deletion blocker heading",
  },
  "auth.deleteAccountBlockedDescription": {
    message:
      "Each workspace below has other members but no other administrator. Promote another member, remove the other members, or delete the workspace before deleting your account.",
    description: "Actionable account deletion blocker guidance",
  },
  "auth.deleteAccountBlockedAction": {
    message: "Open settings",
    description: "Link to resolve an account deletion workspace blocker",
  },
  "auth.deleteAccountConfirmLabel": {
    message: "Type {phrase} below to confirm.",
    description: "Exact account deletion confirmation prompt",
  },
  "auth.deleteAccountSubmit": {
    message: "Delete my account",
    description: "Account deletion submit button",
  },
  "auth.deleteAccountErrorTitle": {
    message: "Account deletion could not start",
    description: "Account deletion mutation error heading",
  },
  "auth.deleteAccountError": {
    message: "We couldn't start account deletion. Please try again.",
    description: "Generic account deletion mutation error",
  },
  "auth.deleteAccountPreviewErrorTitle": {
    message: "Deletion preview unavailable",
    description: "Account deletion preview error heading",
  },
  "auth.deleteAccountPreviewError": {
    message:
      "We couldn't safely determine what will happen to your workspaces. Nothing has been deleted.",
    description: "Account deletion preview failure explanation",
  },
  "auth.deleteAccountRetry": {
    message: "Try again",
    description: "Account deletion preview retry button",
  },
  "auth.accountDeletedTitle": {
    message: "Account deletion started",
    description: "Accepted account deletion terminal page title",
  },
  "auth.accountDeletedSubtitle": {
    message: "You have been signed out safely.",
    description: "Accepted account deletion terminal page subtitle",
  },
  "auth.accountDeletedStatus": {
    message:
      "Your workspaces and credentials are being removed. You can close this page; no further action is required.",
    description: "Accepted account deletion background completion status",
  },
  "auth.accountDeletedHome": {
    message: "Return to bex",
    description: "Accepted account deletion terminal page home link",
  },
  "auth.loggingOutTitle": {
    message: "Signing out...",
    description: "Logout page heading while the logout request is in flight",
  },
  "auth.loggingOutSubtitle": {
    message: "Ending your session, one moment.",
    description: "Logout page subtext while the logout request is in flight",
  },
  "auth.loggedOutTitle": {
    message: "Signed out",
    description: "Logout page heading once logout has completed",
  },
  "auth.loggedOutSubtitle": {
    message: "Redirecting to login…",
    description: "Logout page subtext once logout has completed",
  },
  "auth.logoutFailedTitle": {
    message: "Sign-out failed",
    description: "Logout page heading when the provider logout request failed",
  },
  "auth.logoutFailedSubtitle": {
    message:
      "We couldn't end your session. You may still be signed in — please try again.",
    description: "Logout page subtext when the provider logout request failed",
  },
  "auth.logoutRetry": {
    message: "Try again",
    description: "Logout page retry button after a failed sign-out",
  },
  "auth.logoutConfirmTitle": {
    message: "Sign out?",
    description:
      "Logout page heading on the confirmation screen (codex #12 CSRF logout fix)",
  },
  "auth.logoutConfirmSubtitle": {
    message: "Are you sure you want to end your session?",
    description: "Logout page subtext on the confirmation screen",
  },
  "auth.logoutConfirm": {
    message: "Sign out",
    description: "Logout page confirm button that triggers the actual logout",
  },
  "auth.logoutCancel": {
    message: "Cancel",
    description: "Logout page cancel button that returns to the dashboard",
  },
  "auth.logoutTitle": {
    message: "Sign out",
    description: "Logout page document title",
  },
  "auth.featureSecureTitle": {
    message: "Secure by default",
    description: "Auth hero feature bullet title",
  },
  "auth.featureSecureDescription": {
    message:
      "Sessions are managed by Ory Kratos — battle-tested identity infrastructure, not a hand-rolled auth system.",
    description: "Auth hero feature bullet description",
  },
  "auth.featureDashboardTitle": {
    message: "One dashboard for every service",
    description: "Auth hero feature bullet title",
  },
  "auth.featureDashboardDescription": {
    message:
      "Deploy, monitor, and manage everything running on bex from a single place.",
    description: "Auth hero feature bullet description",
  },
  "auth.featureOpenSourceTitle": {
    message: "Built in the open",
    description: "Auth hero feature bullet title",
  },
  "auth.featureOpenSourceDescription": {
    message: "bex is the open-source Render alternative.",
    description: "Auth hero feature bullet description",
  },
  "auth.consentTitle": {
    message: "Authorize access",
    description: "OAuth2 consent page hero title",
  },
  "auth.consentSubtitle": {
    message: "{client} is asking to act on your behalf in bex.",
    description: "OAuth2 consent page hero subtitle, names the OAuth client",
  },
  "auth.consentUnverifiedClient": {
    message: "Unverified third-party app",
    description:
      "Warning label for an OAuth client whose self-provided branding is not trusted",
  },
  "auth.consentClientId": {
    message: "Client ID",
    description: "Label for the immutable OAuth client identifier",
  },
  "auth.consentRedirectOrigin": {
    message: "Authorization code destination",
    description: "Label for the origin that will receive the OAuth code",
  },
  "auth.consentNoRedirectOrigin": {
    message: "No redirect (device flow)",
    description: "Shown when an OAuth device flow has no redirect origin",
  },
  "auth.consentScopesTitle": {
    message: "It is requesting",
    description: "Heading above the list of requested OAuth scopes",
  },
  "auth.consentScopeOpenid": {
    message: "Confirm who you are",
    description: "Human description of the openid scope",
  },
  "auth.consentScopeOfflineAccess": {
    message: "Stay connected without asking you again (refresh token)",
    description: "Human description of the offline_access scope",
  },
  "auth.consentScopeProfile": {
    message: "Read your basic profile",
    description: "Human description of the profile scope",
  },
  "auth.consentScopeEmail": {
    message: "Read your email address",
    description: "Human description of the email scope",
  },
  "auth.consentScopeRead": {
    message: "Read ordinary workspace resources (services, logs, metrics)",
    description: "Human description of the bex.read OAuth capability",
  },
  "auth.consentScopeWrite": {
    message:
      "Change workspace resources (deploy, restart, create, delete, billing)",
    description: "Human description of the bex.write OAuth capability",
  },
  "auth.consentScopeSensitive": {
    message:
      "Read secrets and connection strings (env vars, database URLs, files)",
    description: "Human description of the bex.sensitive OAuth capability",
  },
  "auth.consentScopeApiCompat": {
    message: "Full control-plane access (legacy platform-client compatibility)",
    description: "Human description of the bex.api compatibility scope",
  },
  "auth.consentAudienceTitle": {
    message: "Where the token works",
    description: "Heading above the requested access-token audience",
  },
  "auth.consentRememberHint": {
    message:
      "Approving is remembered for an hour — after that this agent asks again.",
    description: "Consent page footnote about the remember window",
  },
  "auth.consentApprove": {
    message: "Approve",
    description: "OAuth2 consent page approve button",
  },
  "auth.consentDeny": {
    message: "Deny",
    description: "OAuth2 consent page deny button",
  },
  "auth.consentFailed": {
    message: "Something went wrong completing that. Try again.",
    description: "Consent page error banner after a failed accept/reject",
  },
  "auth.consentExpiredTitle": {
    message: "Authorization expired",
    description: "Consent page heading when there is no live consent request",
  },
  "auth.consentExpiredSubtitle": {
    message: "Start the connection again from the app that sent you here.",
    description:
      "Consent page subtext when there is no live consent request to decide",
  },
  "auth.consentInvalidRequestTitle": {
    message: "Invalid authorization request",
    description:
      "Consent page heading when the requesting client's authorization request fails a protocol check (PKCE or scope)",
  },
  "auth.consentPkceRequiredSubtitle": {
    message:
      "This app's request is missing required security parameters. Contact the app's developer.",
    description:
      "Consent page subtext when the authorization request lacks PKCE with S256",
  },
  "auth.consentScopeRequiredSubtitle": {
    message:
      "This app requested API access without specifying what it needs. It must request at least one of: {scopes}.",
    description:
      "Consent page subtext when an audience request lacks a granular capability scope (round-14 #1); {scopes} is the comma-separated list of accepted capability scopes",
  },
  "auth.consentUnavailableTitle": {
    message: "Authorization unavailable",
    description:
      "Consent page heading when the authorization provider is unreachable, misconfigured, or a headless accept failed",
  },
  "auth.consentUnavailableSubtitle": {
    message: "This should resolve shortly — try again in a moment.",
    description:
      "Consent page subtext when the authorization provider is unreachable, misconfigured, or a headless accept failed",
  },
  "auth.consentWrongUserTitle": {
    message: "Signed in as the wrong account",
    description:
      "Consent page heading when the browser's session belongs to a different account than the one that started this authorization",
  },
  "auth.consentWrongUserSubtitle": {
    message:
      "This browser's session belongs to a different account than the one that started this authorization. Sign out and try again.",
    description:
      "Consent page subtext when the browser's session belongs to a different account than the one that started this authorization",
  },
  "auth.deviceSuccessTitle": {
    message: "bex CLI connected",
    description: "Device authorization success page title",
  },
  "auth.deviceTitle": {
    message: "Connect bex CLI",
    description: "bex CLI device verification route document title",
  },
  "auth.deviceConfirmSubtitle": {
    message:
      "A device signed in with the bex CLI wants access to your account.",
    description: "Device confirm page hero subtitle",
  },
  "auth.deviceConfirmHeading": {
    message: "Authorize this device",
    description: "Device confirm page card heading",
  },
  "auth.deviceConfirmDescription": {
    message: "Confirm the code below matches what your CLI displayed:",
    description: "Device confirm page card description above the code",
  },
  "auth.deviceConfirmHint": {
    message:
      "Only authorize this if you started the request from your terminal.",
    description:
      "Device confirm page footnote warning against unsolicited requests",
  },
  "auth.deviceConfirmCancel": {
    message: "Cancel",
    description: "Device confirm page cancel button",
  },
  "auth.deviceConfirmAuthorize": {
    message: "Authorize device",
    description: "Device confirm page authorize/submit button",
  },
  "auth.deviceExpiredTitle": {
    message: "Device code expired",
    description:
      "Device confirm page heading when there is no live device request",
  },
  "auth.deviceExpiredSubtitle": {
    message:
      "This page needs a device code from `bex login`, or the code has expired. Start the command again from your terminal.",
    description:
      "Device confirm page subtext when there is no live device request to confirm",
  },
  "auth.deviceUnavailableTitle": {
    message: "Device authorization unavailable",
    description:
      "Device confirm page heading when the authorization provider is unreachable or misconfigured",
  },
  "auth.deviceUnavailableSubtitle": {
    message: "This should resolve shortly — try again in a moment.",
    description:
      "Device confirm page subtext when the authorization provider is unreachable or misconfigured",
  },
  "auth.deviceRefusedTitle": {
    message: "Authorization refused",
    description:
      "Device confirm page heading when the device request names a client bex did not expect",
  },
  "auth.deviceRefusedSubtitle": {
    message:
      "This device request could not be verified. Start `bex login` again from your terminal.",
    description:
      "Device confirm page subtext when the device request names a client bex did not expect",
  },
  "auth.deviceSuccessSubtitle": {
    message: "Your browser authorization is complete.",
    description: "Device authorization success page subtitle",
  },
  "auth.deviceSuccessHint": {
    message: "Return to your terminal to continue.",
    description: "Device authorization success page terminal hint",
  },
  "auth.deviceSuccessWaiting": {
    message: "Waiting for browser authorization…",
    description:
      "Device success page terminal replica: the CLI line shown while it polls for tokens",
  },
  "auth.deviceSuccessDone": {
    message: "Authorization complete. You are signed in.",
    description:
      "Device success page terminal replica: the CLI success line (rendered after a ✓ glyph)",
  },
  "auth.deviceSuccessClose": {
    message: "You can close this tab.",
    description:
      "Device success page: follow-up sentence after the return-to-terminal hint",
  },
  "auth.deviceSuccessDashboard": {
    message: "Open the dashboard",
    description:
      "Device success page: quiet link to the dashboard home for users staying in the browser",
  },
};

export default enAuth;
