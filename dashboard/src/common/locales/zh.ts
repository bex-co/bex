import type { TranslationEntry } from "@/i18n";

const zhCommon: Record<string, TranslationEntry> = {
  "common.protectedConfirmationTitle": {
    message: "需要受保护环境确认",
    description:
      "Title for the retry dialog shown after a protected environment blocks a destructive action",
  },
  "common.protectedConfirmationBody": {
    message: "{name} 属于受保护环境。请输入下方的完整命令以继续。",
    description:
      "Resource-neutral explanation for a protected environment confirmation gate",
  },
  "common.protectedConfirmationPrompt": {
    message: "在下方输入 {phrase} 以确认。",
    description:
      "Prompt naming the server-issued protected environment confirmation phrase",
  },
  "common.protectedConfirmationCancel": {
    message: "取消",
    description:
      "Cancel button in the protected environment confirmation dialog",
  },
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
  "common.topbarBreadcrumbs": {
    message: "面包屑导航",
    description: "Accessible label for the dashboard topbar hierarchy",
  },
  "common.topbarSearch": {
    message: "搜索",
    description: "Open the workspace-wide topbar search",
  },
  "common.topbarSearchPlaceholder": {
    message: "搜索页面和资源…",
    description: "Placeholder in the workspace-wide topbar command search",
  },
  "common.topbarSearchDescription": {
    message: "搜索控制台页面和工作区资源。",
    description: "Accessible description for the topbar command search",
  },
  "common.topbarSearchEmpty": {
    message: "没有匹配的页面或资源。",
    description: "Empty state in the workspace-wide topbar command search",
  },
  "common.topbarNavigation": {
    message: "导航",
    description: "Page-links group in the workspace-wide topbar search",
  },
  "common.topbarResources": {
    message: "资源",
    description: "Workspace-resources group in the topbar search",
  },
  "common.topbarNew": {
    message: "新建",
    description: "Persistent topbar menu for creating a resource",
  },
  "common.topbarCreate": {
    message: "创建资源",
    description: "Heading in the persistent topbar create menu",
  },
  "common.topbarHelp": {
    message: "帮助与资源",
    description: "Accessible label for the topbar help menu",
  },
  "common.topbarDocumentation": {
    message: "文档",
    description: "Topbar help-menu link to bex documentation",
  },
  "common.topbarCliGuide": {
    message: "CLI 指南",
    description: "Topbar help-menu link to the bex CLI guide",
  },
  "common.topbarRepository": {
    message: "GitHub 仓库",
    description: "Topbar help-menu link to the bex source repository",
  },
  "common.topbarWorkspaceSettings": {
    message: "工作区设置",
    description: "Workspace-settings label in topbar navigation and search",
  },
  "common.topbarSwitchProject": {
    message: "切换项目",
    description: "Heading in a project breadcrumb dropdown",
  },
  "common.topbarSwitchEnvironment": {
    message: "切换环境",
    description: "Heading in an environment breadcrumb dropdown",
  },
  "common.topbarSwitchService": {
    message: "切换服务",
    description: "Heading in a service breadcrumb dropdown",
  },
  "common.topbarAllResources": {
    message: "所有资源",
    description: "Service breadcrumb menu link back to workspace resources",
  },
  "common.topbarProjectResource": {
    message: "项目",
    description: "Project kind label in workspace-wide search results",
  },
  "common.topbarServiceResource": {
    message: "服务",
    description: "Service kind label in workspace-wide search results",
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
