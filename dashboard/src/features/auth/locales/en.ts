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
};

export default enAuth;
