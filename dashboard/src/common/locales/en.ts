import type { TranslationEntry } from "@/i18n";

const enCommon: Record<string, TranslationEntry> = {
  "common.protectedConfirmationTitle": {
    message: "Protected environment confirmation required",
    description:
      "Title for the retry dialog shown after a protected environment blocks a destructive action",
  },
  "common.protectedConfirmationBody": {
    message:
      "{name} belongs to a protected environment. Type the exact command below to continue.",
    description:
      "Resource-neutral explanation for a protected environment confirmation gate",
  },
  "common.protectedConfirmationPrompt": {
    message: "Type {phrase} below to confirm.",
    description:
      "Prompt naming the server-issued protected environment confirmation phrase",
  },
  "common.protectedConfirmationCancel": {
    message: "Cancel",
    description:
      "Cancel button in the protected environment confirmation dialog",
  },
  "common.navEnvGroups": {
    message: "Environment Groups",
    description: "Workspace sidebar link to environment groups",
  },
  "common.appName": {
    message: "bex",
    description: "Product name shown in the dashboard chrome",
  },
  "common.headDescription": {
    message:
      "Deploy and operate applications on infrastructure you control with bex, the open-source, AI-native Render alternative.",
    description:
      "Generic dashboard description used in description, Open Graph, and Twitter metadata",
  },
  "common.headImageAlt": {
    message: "bex Dashboard logo",
    description: "Alternative text for the dashboard's generic social image",
  },
  "common.loading": {
    message: "Loading…",
    description: "Generic loading state label",
  },
  "common.sudoCommandLabel": {
    message: "Sudo Command",
    description:
      "Input label on Render-style destructive type-to-confirm gates (live capture: docs/render-artifacts/workspace-lifecycle.md)",
  },
  "common.navIntegrationsGroup": {
    message: "Integrations",
    description:
      "Sidebar nav section label (webhooks, notifications) — Render's grouping",
  },
  "common.navWorkspaceGroup": {
    message: "Workspace",
    description:
      "Sidebar nav section label (usage, settings) — Render's grouping",
  },
  "common.navMonitorGroup": {
    message: "Monitor",
    description:
      "Service sidebar section label (logs, metrics) — Render's grouping",
  },
  "common.navWebhooks": {
    message: "Webhooks",
    description: "Sidebar nav link to the outbound event webhooks page",
  },
  "common.navNotifications": {
    message: "Notifications",
    description: "Sidebar nav link to the notification settings page",
  },
  "common.navProjects": {
    message: "Projects",
    description:
      "Sidebar nav link to the unified projects page (services, databases, key value grouped together)",
  },
  "common.navUsage": {
    message: "Usage",
    description: "Sidebar nav link to the workspace usage page",
  },
  "common.navBlueprints": {
    message: "Blueprints",
    description:
      "Sidebar nav link to the blueprints management page (IaC stacks auto-registered on deploy)",
  },
  "common.navSettings": {
    message: "Settings",
    description:
      "Sidebar nav link to the workspace settings page (the sidebar is workspace-scoped; account settings hang off the user menu)",
  },
  "common.navBackToDashboard": {
    message: "Dashboard",
    description:
      "Contextual project sidebar's back link label, shown on a project's Overview page (links up to the workspace Overview)",
  },
  "common.navBackToProject": {
    message: "Project",
    description:
      "Contextual project sidebar's back link label, shown on a project's Settings page (links up to the project's Overview)",
  },
  "common.navProjectOverview": {
    message: "Overview",
    description:
      "Contextual project sidebar nav link to the project's own Overview page",
  },
  "common.navManageGroup": {
    message: "Manage",
    description: "Contextual project sidebar section label above Settings",
  },
  "common.changeLanguage": {
    message: "Change language",
    description: "Accessible label for the language switcher button",
  },
  "common.userMenuSettings": {
    message: "Account Settings",
    description:
      "User menu item that navigates to account settings (/settings)",
  },
  "common.userMenuTheme": {
    message: "Theme",
    description: "User menu submenu label for theme selection",
  },
  "common.userMenuLanguage": {
    message: "Language",
    description: "User menu submenu label for language selection",
  },
  "common.userMenuThemeLight": {
    message: "Light",
    description: "Light theme option label",
  },
  "common.userMenuThemeDark": {
    message: "Dark",
    description: "Dark theme option label",
  },
  "common.userMenuThemeSystem": {
    message: "System",
    description: "System theme option label",
  },
  "common.userMenuLogOut": {
    message: "Log out",
    description: "User menu item that signs the user out",
  },
  "common.goHome": {
    message: "Go home",
    description: "Button label that navigates back to the home page",
  },
  "common.goBack": {
    message: "Go back",
    description: "Button label that navigates to the previous page",
  },
  "common.tryAgain": {
    message: "Try again",
    description: "Button label that retries after an error",
  },
  "common.notFoundTitle": {
    message: "Page not found",
    description: "404 page heading",
  },
  "common.notFoundDescription": {
    message: "The page you're looking for doesn't exist or has been moved.",
    description: "404 page explanatory text",
  },
  "common.errorTitle": {
    message: "Something went wrong",
    description: "Generic error page heading",
  },
  "common.errorDefaultMessage": {
    message: "Something went wrong.",
    description: "Fallback error message when none is provided",
  },
  "common.resourceNotFoundToast": {
    message: "That resource doesn't exist or was deleted.",
    description:
      "Toast shown after a dead resource URL redirects to the home page (w9/m55)",
  },
  "common.resourceErrorBody": {
    message: "The resource couldn't be loaded. Check the API and try again.",
    description:
      "Body text of a detail page's inline error state when the resource query failed",
  },
};

export default enCommon;
