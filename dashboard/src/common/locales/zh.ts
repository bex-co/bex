import type { TranslationEntry } from "@/i18n";

const zhCommon: Record<string, TranslationEntry> = {
  "common.navEnvGroups": {
    message: "环境变量组",
    description: "Workspace sidebar link to environment groups",
  },
  "common.appName": {
    message: "bex",
    description: "Product name shown in the dashboard chrome",
  },
  "common.headDescription": {
    message:
      "使用开源、AI 原生的 Render 替代方案 bex，在你掌控的基础设施上部署和运维应用。",
    description:
      "Generic dashboard description used in description, Open Graph, and Twitter metadata",
  },
  "common.headImageAlt": {
    message: "bex 仪表盘标志",
    description: "Alternative text for the dashboard's generic social image",
  },
  "common.loading": {
    message: "加载中…",
    description: "Generic loading state label",
  },
  "common.sudoCommandLabel": {
    message: "Sudo 命令",
    description:
      "Input label on Render-style destructive type-to-confirm gates (live capture: docs/render-artifacts/workspace-lifecycle.md)",
  },
  "common.navIntegrationsGroup": {
    message: "集成",
    description:
      "Sidebar nav section label (webhooks, notifications) — Render's grouping",
  },
  "common.navWorkspaceGroup": {
    message: "工作区",
    description:
      "Sidebar nav section label (usage, settings) — Render's grouping",
  },
  "common.navMonitorGroup": {
    message: "监控",
    description:
      "Service sidebar section label (logs, metrics) — Render's grouping",
  },
  "common.navWebhooks": {
    message: "Webhooks",
    description: "Sidebar nav link to the outbound event webhooks page",
  },
  "common.navNotifications": {
    message: "通知",
    description: "Sidebar nav link to the notification settings page",
  },
  "common.navProjects": {
    message: "项目",
    description:
      "Sidebar nav link to the unified projects page (services, databases, key value grouped together)",
  },
  "common.navUsage": {
    message: "用量",
    description: "Sidebar nav link to the workspace usage page",
  },
  "common.navBlueprints": {
    message: "蓝图",
    description:
      "Sidebar nav link to the blueprints management page (IaC stacks auto-registered on deploy)",
  },
  "common.navSettings": {
    message: "设置",
    description:
      "Sidebar nav link to the workspace settings page (the sidebar is workspace-scoped; account settings hang off the user menu)",
  },
  "common.navBackToDashboard": {
    message: "仪表盘",
    description:
      "Contextual project sidebar's back link label, shown on a project's Overview page (links up to the workspace Overview)",
  },
  "common.navBackToProject": {
    message: "项目",
    description:
      "Contextual project sidebar's back link label, shown on a project's Settings page (links up to the project's Overview)",
  },
  "common.navProjectOverview": {
    message: "概览",
    description:
      "Contextual project sidebar nav link to the project's own Overview page",
  },
  "common.navManageGroup": {
    message: "管理",
    description: "Contextual project sidebar section label above Settings",
  },
  "common.changeLanguage": {
    message: "切换语言",
    description: "Accessible label for the language switcher button",
  },
  "common.userMenuSettings": {
    message: "账户设置",
    description:
      "User menu item that navigates to account settings (/settings)",
  },
  "common.userMenuTheme": {
    message: "主题",
    description: "User menu submenu label for theme selection",
  },
  "common.userMenuLanguage": {
    message: "语言",
    description: "User menu submenu label for language selection",
  },
  "common.userMenuThemeLight": {
    message: "浅色",
    description: "Light theme option label",
  },
  "common.userMenuThemeDark": {
    message: "深色",
    description: "Dark theme option label",
  },
  "common.userMenuThemeSystem": {
    message: "跟随系统",
    description: "System theme option label",
  },
  "common.userMenuLogOut": {
    message: "退出登录",
    description: "User menu item that signs the user out",
  },
  "common.goHome": {
    message: "返回首页",
    description: "Button label that navigates back to the home page",
  },
  "common.goBack": {
    message: "返回上一页",
    description: "Button label that navigates to the previous page",
  },
  "common.tryAgain": {
    message: "重试",
    description: "Button label that retries after an error",
  },
  "common.notFoundTitle": {
    message: "页面未找到",
    description: "404 page heading",
  },
  "common.notFoundDescription": {
    message: "您要查找的页面不存在或已被移动。",
    description: "404 page explanatory text",
  },
  "common.errorTitle": {
    message: "出错了",
    description: "Generic error page heading",
  },
  "common.errorDefaultMessage": {
    message: "出错了。",
    description: "Fallback error message when none is provided",
  },
  "common.resourceNotFoundToast": {
    message: "该资源不存在或已被删除。",
    description:
      "Toast shown after a dead resource URL redirects to the home page (w9/m55)",
  },
  "common.resourceErrorBody": {
    message: "无法加载该资源。请检查 API 后重试。",
    description:
      "Body text of a detail page's inline error state when the resource query failed",
  },
};

export default zhCommon;
