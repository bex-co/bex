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
    message: "Enter your email to receive a recovery code",
    description: "Forgot-password page hero subtitle",
  },
  "auth.verificationTitle": {
    message: "Verify your email",
    description: "Verification page hero title",
  },
  "auth.verificationSubtitle": {
    message: "Enter the code we sent to confirm your address",
    description: "Verification page hero subtitle",
  },
  "auth.settingsTitle": {
    message: "Settings",
    description: "Account settings page heading",
  },
  "auth.settingsSubtitle": {
    message: "Manage your account profile, password, and two-factor security.",
    description: "Account settings page subheading",
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
  "auth.deviceSuccessTitle": {
    message: "Render CLI connected",
    description: "Device authorization success page title",
  },
  "auth.deviceTitle": {
    message: "Connect Render CLI",
    description: "Render CLI device verification route document title",
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
