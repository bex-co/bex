import type { TranslationEntry } from "@/i18n";

const enCommon: Record<string, TranslationEntry> = {
  "common.appName": {
    message: "bex",
    description: "Product name shown in the dashboard chrome",
  },
  "common.loading": {
    message: "Loading…",
    description: "Generic loading state label",
  },
  "common.navDashboardGroup": {
    message: "Dashboard",
    description: "Sidebar nav section label",
  },
  "common.navServices": {
    message: "Services",
    description: "Sidebar nav link to the services list page",
  },
  "common.navDatabases": {
    message: "Databases",
    description: "Sidebar nav link to the databases list page",
  },
  "common.navKeyValue": {
    message: "Key Value",
    description: "Sidebar nav link to the Key Value list page",
  },
  "common.navSettings": {
    message: "Settings",
    description: "Sidebar nav link to the account settings page",
  },
  "common.changeLanguage": {
    message: "Change language",
    description: "Accessible label for the language switcher button",
  },
  "common.userMenuSettings": {
    message: "Settings",
    description: "User menu item that navigates to account settings",
  },
  "common.userMenuTheme": {
    message: "Theme",
    description: "User menu submenu label for theme selection",
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
};

export default enCommon;
