import type { TranslationEntry } from "@/i18n";

const zhCommon: Record<string, TranslationEntry> = {
  "common.navRouter": {
    message: "Router",
    description: "Router navigation item",
  },
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
  "common.confirmPhrasePrompt": {
    message: "若要继续，请输入 {phrase}",
    description:
      "Label above the typed-confirmation input in a phrase-gated confirm dialog.",
  },
  "common.close": {
    message: "关闭",
    description:
      "Accessible name of the shared dialog's built-in close (X) button.",
  },
  "common.sidebarTitle": {
    message: "侧边栏",
    description: "Screen-reader title of the mobile navigation drawer.",
  },
  "common.sidebarMobileDescription": {
    message: "显示移动端侧边栏。",
    description: "Screen-reader description of the mobile navigation drawer.",
  },
  "common.skipToMainContent": {
    message: "跳到主要内容",
    description:
      "Label of the first-tab skip link that moves keyboard focus past the sidebar and header to the page content.",
  },
  "common.sidebarToggle": {
    message: "切换侧边栏",
    description:
      "Accessible name of the button that opens or collapses the navigation sidebar.",
  },
  "common.sidebarResize": {
    message: "调整侧边栏宽度",
    description: "Accessible name of the sidebar's draggable resize separator.",
  },
  "common.sidebarResizeHint": {
    message: "拖动调整宽度 · 双击展开或收起",
    description: "Hover tooltip on the sidebar's resize separator.",
  },
  "common.commandSuggestions": {
    message: "建议",
    description:
      "Accessible name of a search or picker results list (the command menu's listbox).",
  },
  "common.comboboxPlaceholder": {
    message: "选择一个选项",
    description:
      "Fallback placeholder of a searchable picker when its caller supplies none.",
  },
  "common.comboboxEmpty": {
    message: "未找到结果",
    description:
      "Fallback text when a searchable picker's filter matches no option.",
  },
  "common.comboboxUseCustom": {
    message: "按 Enter 使用“{value}”",
    description:
      "Hint under a picker's empty results: Enter keeps the typed value as entered. {value} is the typed text.",
  },
  "common.comboboxClear": {
    message: "清除输入",
    description: "Accessible name of a searchable picker's clear (X) button.",
  },
  "common.cancel": {
    message: "取消",
    description:
      "Generic Cancel button — dismisses a dialog or an inline form without saving.",
  },
  "common.pricePerRunMinute": {
    message: "${price}/分钟运行时间",
    description:
      "A cron job's USD price per minute of run time, on a Change Compute Plan card (w4/546): it is billed per second only while a run executes; price is an API-supplied decimal without the currency symbol",
  },
  "common.pricePerMinute": {
    message: "${price}/分钟",
    description:
      "A cron job's USD price per minute of run time on a create-form plan card, short to fit beside the plan name; the form's hint says it is billed only while a run executes (w4/546). price is an API-supplied decimal without the currency symbol",
  },
  "common.pricePerMonth": {
    message: "${price}/月",
    description:
      "Monthly USD price shown on a resource plan card; price is an API-supplied decimal without the currency symbol",
  },
  "common.reviewChangesTitle": {
    message: "应用更改？",
    description:
      "Title of Render's plan review dialog, shown before a compute or storage change applies (w3/m674)",
  },
  "common.reviewChangesTotal": {
    message: "总计",
    description:
      "Plan review dialog: the label of the monthly total under the priced lines",
  },
  "common.reviewEditComputePlan": {
    message: "编辑计算套餐",
    description:
      "Plan review dialog: accessible name of the Compute plan line's Edit button, which returns to the plan picker",
  },
  "common.planUnavailable": {
    message: "暂不可用",
    description:
      "Shown in place of the price on a disabled plan card whose compute plan is paused (it cannot be selected for now); the server's full reason is the card's tooltip",
  },
  "common.planConnections_other": {
    message: "{formatted} 个连接",
    description:
      "A plan card's connection-limit line, under its RAM: the client connections the plan accepts (Render's Key Value cards read \"50 Connections\"); {formatted} is the locale-formatted count",
  },
  "common.protectedConfirmationRetry": {
    message: "确认并保存",
    description:
      "Confirm button in the protected environment retry dialog raised by a settings save",
  },
  "common.protectedConfirmationCancel": {
    message: "取消",
    description:
      "Cancel button in the protected environment confirmation dialog",
  },
  "common.navEnvGroups": {
    message: "环境组",
    description: "Workspace sidebar link to environment groups",
  },
  "common.navBuildSources": {
    message: "构建源",
    description: "Workspace sidebar link to build sources",
  },
  "common.navSandboxGroups": {
    message: "沙箱组",
    description: "Workspace sidebar link to sandbox groups",
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
  "common.copyCode": {
    message: "复制代码",
    description: "Accessible name of the copy button on a code block",
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
  "common.copyField": {
    message: "复制{field}",
    description:
      "Accessible name for a copy control when the caller only has a field label (m101/t014)",
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
  "common.searchShortcutEnvVarsDocs": {
    message: "环境变量",
    description:
      "Search shortcut to the docs page on environment variables and secrets (Render's 'Environment variables' shortcut)",
  },
  "common.searchShortcutEnvVarsDocsDescription": {
    message: "为服务设置环境变量和密钥",
    description:
      "One-line description beside the environment variables docs search shortcut",
  },
  "common.searchShortcutCustomDomainsDocs": {
    message: "自定义域名",
    description: "Search shortcut to the custom domains docs page",
  },
  "common.searchShortcutCustomDomainsDocsDescription": {
    message: "为服务添加自定义域名",
    description:
      "One-line description beside the custom domains docs search shortcut",
  },
  "common.searchShortcutGitCredentials": {
    message: "Git 凭据",
    description:
      "Search shortcut to Account Settings → Integrations, where GitHub is connected",
  },
  "common.searchShortcutGitCredentialsDescription": {
    message: "连接 GitHub",
    description:
      "One-line description beside the Git credentials search shortcut; bex connects GitHub only",
  },
  "common.searchShortcutAuditLogs": {
    message: "审计日志",
    description: "Search shortcut to Workspace Settings → Audit Logs",
  },
  "common.searchShortcutAuditLogsDescription": {
    message: "导出审计日志",
    description: "One-line description beside the Audit Logs search shortcut",
  },
  "common.searchShortcutApiKeys": {
    message: "API 密钥",
    description: "Search shortcut to Account Settings → API Keys",
  },
  "common.searchShortcutApiKeysDescription": {
    message: "创建 API 密钥",
    description: "One-line description beside the API Keys search shortcut",
  },
  "common.searchShortcutSshKeys": {
    message: "SSH 密钥",
    description: "Search shortcut to Account Settings → SSH Public Keys",
  },
  "common.searchShortcutSshKeysDescription": {
    message: "添加 SSH 公钥",
    description: "One-line description beside the SSH Keys search shortcut",
  },
  "common.searchShortcutCliTokens": {
    message: "CLI 令牌",
    description:
      "Search shortcut to Account Settings → Connected agents, which lists the CLI's tokens",
  },
  "common.searchShortcutCliTokensDescription": {
    message: "查看并撤销 CLI 令牌",
    description: "One-line description beside the CLI Tokens search shortcut",
  },
  "common.searchShortcutRegistryCredentials": {
    message: "容器镜像仓库凭据",
    description: "Search shortcut to Workspace Settings → Registry Credentials",
  },
  "common.searchShortcutRegistryCredentialsDescription": {
    message: "管理外部镜像的凭据",
    description:
      "One-line description beside the container registry credentials search shortcut",
  },
  "common.searchShortcutMcpServer": {
    message: "MCP 服务器",
    description: "Search shortcut to the MCP server docs page",
  },
  "common.searchShortcutMcpServerDescription": {
    message: "在 AI 应用中管理你的 bex 服务",
    description: "One-line description beside the MCP Server search shortcut",
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
  "common.labelInvalid": {
    message: "请输入最多 100 个字符的名称，不能包含换行或控制字符。",
    description:
      "Inline validation under a free-form name field (project, environment, group, webhook, credential, Blueprint)",
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
    message: "账单",
    description:
      "Sidebar nav link to the workspace billing page (/billing, renamed from Usage in w5/m70; key kept to avoid churn)",
  },
  "common.navBlueprints": {
    message: "蓝图",
    description:
      "Sidebar nav link to the blueprints management page (IaC stacks auto-registered on deploy)",
  },
  "common.navAgents": {
    message: "智能体",
    description:
      "Sidebar nav link to the cloud coding-agent sessions page (/agents)",
  },
  "common.navAutomations": {
    message: "自动化",
    description:
      "Sidebar nav link to the scheduled agent automations page (/automations)",
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
  "common.navBackToEnvironment": {
    message: "环境",
    description:
      "Contextual project sidebar's back link label, shown on an environment's settings page (links up to the environment's section on the project page, Render's words, w1/m683)",
  },
  "common.navProjectOverview": {
    message: "概览",
    description:
      "Contextual project sidebar nav link to the project's own Overview page",
  },
  "common.navObservabilityGroup": {
    message: "可观测性",
    description:
      "Contextual project sidebar section label above the project's log views (Render's OBSERVABILITY group, w3/m505)",
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
  "common.themeNotSaved": {
    message: "主题未能保存，已恢复为之前的主题。",
    description:
      "Toast when a theme change could not be saved to the account (w3/m531); the previous theme is put back",
  },
  "common.userMenuLogOut": {
    message: "退出登录",
    description: "User menu item that signs the user out",
  },
  "common.userMenuOpen": {
    message: "账户菜单",
    description:
      "Accessible name of the avatar button that opens the account menu (w4/407)",
  },
  "common.userMenuOpenFor": {
    message: "{name} 的账户菜单",
    description:
      "Accessible name of the avatar button that opens the account menu, naming the signed-in identity (w4/407)",
  },
  "common.goHome": {
    message: "返回首页",
    description: "Button label that navigates back to the home page",
  },
  "common.goBack": {
    message: "返回上一页",
    description: "Button label that navigates to the previous page",
  },
  "common.planCatalogUnavailable": {
    message: "套餐列表加载失败，暂时无法选择套餐。你填写的其他内容已保留。",
    description:
      "A plan picker whose catalog read failed or returned no plans; shown with a Try again button that re-reads only the catalog (w4/m218)",
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
    message: "你要查找的页面不存在或已被移动。",
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
  "common.loadErrorRetry": {
    message: "加载时出错，请刷新页面重试。",
    description:
      "Body of a panel whose read failed with no server reason to show (a network failure or throttling)",
  },
  "common.serviceNotFoundTitle": {
    message: "未找到服务",
    description:
      "Title of a service page whose id leads nowhere (Render's ResourceNotFound, w3/m706)",
  },
  "common.serviceDeletedTitle": {
    message: "服务已删除",
    description:
      "Title of a service page whose service the dashboard had read and is now gone (w3/m706)",
  },
  "common.cronJobNotFoundTitle": {
    message: "未找到定时任务",
    description: "Title of a cron job page whose id leads nowhere (w3/m706)",
  },
  "common.cronJobDeletedTitle": {
    message: "定时任务已删除",
    description:
      "Title of a cron job page whose cron job the dashboard had read and is now gone (w3/m706)",
  },
  "common.sandboxGroupNotFoundTitle": {
    message: "未找到沙箱组",
    description:
      "Title of a sandbox group page whose id leads nowhere (w3/m706)",
  },
  "common.sandboxGroupDeletedTitle": {
    message: "沙箱组已删除",
    description:
      "Title of a sandbox group page whose group the dashboard had read and is now gone (w3/m706)",
  },
  "common.deployNotFoundTitle": {
    message: "未找到部署",
    description:
      "Title of a deploy page whose deploy id leads nowhere under a live service (w3/m706)",
  },
  "common.buildNotFoundTitle": {
    message: "未找到构建",
    description:
      "Title of a build page whose build id leads nowhere under a live build source (w3/m706)",
  },
  "common.resourceNotFoundBody": {
    message: "找不到 {id}。它可能已被删除，或链接不正确。",
    description:
      "Body of a resource page whose id leads nowhere; {id} is the id, drawn as a code chip (w3/m706)",
  },
  "common.resourceDeletedBody": {
    message: "{name}（{id}）已被删除。",
    description:
      "Body of a resource page whose resource the dashboard had read and is now gone; {name} is its name, {id} its id (w3/m706)",
  },
  "common.resourceNotFoundForBody": {
    message: "找不到 {parent} 的 {id}。链接可能不正确。",
    description:
      "Body of a deploy or build page whose id leads nowhere; {id} is the id, {parent} the service or build source it was opened under (w3/m706)",
  },
  "common.goToProject": {
    message: "前往项目",
    description:
      "Button on a missing resource's page that opens the project it was in (w3/m706)",
  },
  "common.returnHome": {
    message: "返回首页",
    description:
      "Button on a missing, denied or failed resource's page that opens the home page (w3/m706)",
  },
  "common.viewDeploys": {
    message: "查看部署",
    description:
      "Button on a missing deploy's page that opens its service's deploys (w3/m706)",
  },
  "common.viewBuilds": {
    message: "查看构建",
    description:
      "Button on a missing build's page that opens its build source's builds (w3/m706)",
  },
  "common.goToWebhooks": {
    message: "前往 Webhooks",
    description:
      "Button on a missing or denied webhook's page that opens the webhooks list (w3/m706)",
  },
  "common.resourceDoesNotExist": {
    message: "{id} 不存在。",
    description:
      "Body of the Page not found state a resource page draws for a dead id; {id} is the id (w3/m706)",
  },
  "common.accessDeniedTitle": {
    message: "拒绝访问",
    description: "Title of a resource page whose read was refused (w3/m706)",
  },
  "common.accessDeniedBody": {
    message: "你无权访问 {id}。",
    description:
      "Body of a resource page whose read was refused; {id} is the id (w3/m706)",
  },
  "common.accessDeniedBodyNoId": {
    message: "你无权访问此资源。",
    description: "Body of a refused page that names no id (w3/m706)",
  },
  "common.resourceLoadFailedTitle": {
    message: "发生错误",
    description: "Title of a resource page whose read failed (w3/m706)",
  },
  "common.resourceLoadFailedBody": {
    message: "无法加载 {id}。",
    description:
      "Body of a resource page whose read failed; {id} is the id (w3/m706)",
  },
  "common.copyId": {
    message: "复制 ID",
    description:
      "Accessible name of the copy button beside an id in a resource state (w3/m706)",
  },
  "common.copyIdError": {
    message: "无法复制 ID",
    description:
      "Toast when copying an id from a resource state fails (w3/m706)",
  },
  "common.resourceErrorBody": {
    message: "无法加载该资源。请检查 API 后重试。",
    description:
      "Body text of a detail page's inline error state when the resource query failed",
  },
  "common.sessionExpiredTitle": {
    message: "登录已过期",
    description:
      "Heading of the inline state shown when a resource query fails with a 401 (expired session)",
  },
  "common.sessionExpiredBody": {
    message: "重新登录即可继续 —— 登录后会自动返回此页面。",
    description:
      "Body text of the expired-session inline state, reassuring the user their place is preserved",
  },
  "common.signIn": {
    message: "登录",
    description:
      "Button label that sends an expired session back to the login page",
  },
  "common.colActions": {
    message: "操作",
    description:
      "Screen-reader-only header for a table's trailing row-actions column",
  },
  "common.justNow": {
    message: "刚刚",
    description:
      'Elapsed-time text for an instant under a minute old ("Deployed just now")',
  },
  "common.timeLocal": {
    message: "本地时间",
    description:
      "Timestamp tooltip: label of the row showing the instant in the viewer's local timezone",
  },
  "common.timeUtc": {
    message: "UTC",
    description:
      "Timestamp tooltip: label of the row showing the instant in UTC",
  },
  "common.timeUnix": {
    message: "时间戳",
    description:
      "Timestamp tooltip: label of the row showing the instant as Unix milliseconds",
  },
  "common.timeFromPrevious": {
    message: "距上一行",
    description:
      "Timestamp tooltip (w4/716): label of a log line's row showing the time since the line before it, Render's 'From previous'",
  },
  "common.timeFromStart": {
    message: "距开始",
    description:
      "Timestamp tooltip (w4/716): label of a deploy log line's row showing the time since the deploy began, Render's 'From start'",
  },
  "common.timeRowLabel": {
    message: "{label}：",
    description:
      "Timestamp tooltip (w4/716): a row's label followed by Render's colon; {label} is the zone ('PST'), UTC, Timestamp, From previous or From start",
  },
  "common.timeCopied": {
    message: "已复制！",
    description:
      "Timestamp tooltip (w4/716): toast after a row's copy button copied its value, Render's 'Copied!'",
  },
  "common.timeCopyFailed": {
    message: "无法复制时间",
    description:
      "Timestamp tooltip (w4/716): toast when the clipboard refused a row's value",
  },
  "common.ipAllowListProxiedTitle_other": {
    message: "白名单在 {count} 个自定义域名上无法识别客户端",
    description:
      "Title of the warning shown under an inbound IP allowlist when custom domains it covers are proxied by Cloudflare (w1/m171)",
  },
  "common.ipAllowListProxiedBody_other": {
    message:
      "以下域名经由 Cloudflare 代理，白名单在这些域名上匹配的是 Cloudflare 的地址，而不是访问者的地址。白名单已保存，但在这些域名上会拦截你本想放行的访问者：",
    description:
      "Body of the Cloudflare-proxied allowlist warning, followed by the list of proxied hostnames (w1/m171)",
  },
  "common.ipAllowListProxiedFix": {
    message:
      "如需按真实客户端地址过滤，请在 Cloudflare 中将该 DNS 记录设为仅 DNS（灰色云朵）。",
    description:
      "How to fix the Cloudflare-proxied allowlist warning: switch the record from proxied (orange cloud) to DNS only (w1/m171)",
  },
  "common.inboundIpAllowedRanges_other": {
    message: "个 IP 范围允许从你的私有网络外部访问。",
    description:
      "Status line of a Networking panel after a count badge (Render's, w4/m710): how many IPv4 ranges every level of inbound IP rules (workspace, environment, the resource's own) allows together",
  },
  "common.inboundIpValidateLabel": {
    message: "验证 IP 地址",
    description:
      "Label of the Networking panel's address check field (Render's Validate IP address, w4/m710)",
  },
  "common.inboundIpCheckEmpty": {
    message: "输入一个 IP 地址",
    description:
      "Validate IP address field, nothing typed yet (Render's tooltip, w4/m710)",
  },
  "common.inboundIpCheckInvalid": {
    message: "此 IP 地址无效",
    description:
      "Validate IP address field: the text is not an IPv4 address (Render's, w4/m710)",
  },
  "common.inboundIpCheckAllowed": {
    message: "允许此 IP 地址访问",
    description:
      "Validate IP address field: every level of inbound IP rules admits the address (Render's, w4/m710)",
  },
  "common.inboundIpRefusedWorkspace": {
    message: "根据你的工作区入站 IP 规则，不允许此 IP 地址访问。",
    description:
      "Validate IP address field: the workspace's inbound IP rules refuse the address (Render's, w4/m710)",
  },
  "common.inboundIpRefusedEnvironment": {
    message: "根据你的环境入站 IP 规则，不允许此 IP 地址访问。",
    description:
      "Validate IP address field: the environment's inbound IP rules refuse the address (Render's, w4/m710)",
  },
  "common.inboundIpRefusedOwn": {
    message: "根据你的 {type} 入站 IP 规则，不允许此 IP 地址访问。",
    description:
      "Validate IP address field: the resource's own (unsaved) inbound IP rules refuse the address; {type} is its type, e.g. Web Service, PostgreSQL, Key Value, Environment (Render's, w4/m710)",
  },
  "common.inboundIpAffectedBy": {
    message: "受以下规则影响：",
    description:
      "Label before the breadcrumb of a higher level of inbound IP rules that also applies to this resource (Render's 'Affected by:', w4/m710)",
  },
  "common.inboundIpLevelWorkspace": {
    message: "工作区",
    description:
      "Breadcrumb item naming the workspace level of inbound IP rules (Render's, w4/m710)",
  },
  "common.inboundIpLevelEnvironment": {
    message: "环境",
    description:
      "Breadcrumb item naming the environment level of inbound IP rules, and an environment's type in the own-rules heading and refusal (Render's, w4/m710)",
  },
  "common.inboundIpViewRules": {
    message: "查看规则",
    description:
      "Link, opening in a new tab, to where a higher level of inbound IP rules is edited (Render's View rules, w4/m710)",
  },
  "common.inboundIpSource": {
    message: "来源",
    description:
      "Column header of a read-only inbound IP rules table: the CIDR range (Render's Source, w4/m710)",
  },
  "common.inboundIpDescription": {
    message: "描述",
    description:
      "Column header of a read-only inbound IP rules table: the rule's description (Render's, w4/m710)",
  },
  "common.inboundIpBlockedWorkspace": {
    message: "工作区入站 IP 规则拦截了所有互联网流量。",
    description:
      "Warning in place of the table when the workspace has no inbound IP rule (Render's, w4/m710)",
  },
  "common.inboundIpBlockedEnvironment": {
    message: "环境入站 IP 规则拦截了所有互联网流量。",
    description:
      "Warning in place of the table when the environment has no inbound IP rule (Render's, w4/m710)",
  },
  "common.inboundIpOwnRules": {
    message: "{type} 入站 IP 规则",
    description:
      "Heading above a resource's own editable inbound IP rules; {type} is its type, e.g. Web Service, PostgreSQL, Key Value, Environment (Render's, w4/m710)",
  },
  "common.inboundIpTypePostgres": {
    message: "PostgreSQL",
    description:
      "A Postgres database's type as the inbound IP rules heading and refusal name it (Render's userFacingType, w4/m710)",
  },
  "common.inboundIpLoadError": {
    message: "无法加载同样适用的工作区和环境入站 IP 规则。请刷新后重试。",
    description:
      "Shown in place of the workspace and environment inbound IP rules when one of them could not be read (w4/m710)",
  },
  "common.relativeAgeNow": {
    message: "刚刚",
    description: "Compact relative age under one minute (w4/m188)",
  },
  "common.relativeAgeMinutes": {
    message: "{n}分钟",
    description: "Compact relative age in whole minutes, e.g. 4m (w4/m188)",
  },
  "common.relativeAgeHours": {
    message: "{n}小时",
    description: "Compact relative age in whole hours, e.g. 3h (w4/m188)",
  },
  "common.relativeAgeDays": {
    message: "{n}天",
    description: "Compact relative age in whole days, e.g. 5d (w4/m188)",
  },
  "common.relativeAgeMonths": {
    message: "{n}个月",
    description: "Compact relative age in 30-day months, e.g. 2mo (w4/m188)",
  },
  "common.relativeAgeYears": {
    message: "{n}年",
    description: "Compact relative age in 365-day years, e.g. 1y (w4/m188)",
  },
  "common.relativeUntilNow": {
    message: "现在",
    description:
      "Countdown to a future instant less than one minute away (w4/m188)",
  },
  "common.relativeUntilMinutes": {
    message: "{n}分钟后",
    description: "Compact countdown in whole minutes, e.g. in 5m (w4/m188)",
  },
  "common.relativeUntilHours": {
    message: "{n}小时后",
    description: "Compact countdown in whole hours, e.g. in 3h (w4/m188)",
  },
  "common.relativeUntilDays": {
    message: "{n}天后",
    description: "Compact countdown in whole days, e.g. in 83d (w4/m188)",
  },
  "common.openBilling": {
    message: "打开账单",
    description:
      "Link from a plan notice (a Pro-and-higher feature on Hobby) to the Billing page (w8/m138)",
  },
  "common.billingSettlingTitle": {
    message: "正在结算最终用量…",
    description:
      "Title while a deletion waits for the workspace's final usage to be charged (BILLING_SETTLING); retried automatically",
  },
  "common.billingSettlingDescription": {
    message:
      "bex 正在删除前结算最终用量，这可能需要一两分钟。系统会自动重试，请保持此页面打开。",
    description:
      "Body while a deletion waits for the final usage to be charged (BILLING_SETTLING)",
  },
  "common.billingSettlingTimeout": {
    message: "最终用量仍在处理中。请过几分钟后再次删除。",
    description:
      "Deletion error after BILLING_SETTLING retries ran out (about three minutes)",
  },
  "common.balanceDueTitle": {
    message: "存在未付余额",
    description:
      "Title of the alert when a deletion is refused because the workspace has an unpaid balance (BILLING_BALANCE_DUE)",
  },
  "common.balanceDueAmount": {
    message: "此工作区有 {amount} 的未付余额。请先支付账单，然后重新删除。",
    description: "Balance-due deletion refusal with the amount",
  },
  "common.balanceDue": {
    message: "此工作区有未付余额。请先支付账单，然后重新删除。",
    description: "Balance-due deletion refusal without an amount",
  },
  "common.balanceDueNamedAmount": {
    message:
      "工作区 {name} 有 {amount} 的未付余额。请先支付账单，然后重新删除账户。",
    description:
      "Account-deletion refusal naming the workspace with the unpaid balance and its amount",
  },
  "common.balanceDueNamed": {
    message: "工作区 {name} 有未付余额。请先支付账单，然后重新删除账户。",
    description:
      "Account-deletion refusal naming the workspace with the unpaid balance",
  },
  "common.balanceDuePay": {
    message: "支付账单",
    description: "Link to Stripe's hosted invoice page, opens in a new tab",
  },
  "common.regionFrankfurt": {
    message: "法兰克福（德国）",
    description:
      "Display name of Render's `frankfurt` region, shown where a resource's region is read (w4/693)",
  },
  "common.regionOhio": {
    message: "俄亥俄州（美国）",
    description:
      "Display name of Render's `ohio` region, shown where a resource's region is read (w4/693)",
  },
  "common.regionOregon": {
    message: "俄勒冈州（美国）",
    description:
      "Display name of Render's `oregon` region, shown where a resource's region is read (w4/693)",
  },
  "common.regionSingapore": {
    message: "新加坡",
    description:
      "Display name of Render's `singapore` region, shown where a resource's region is read (w4/693)",
  },
  "common.regionVirginia": {
    message: "弗吉尼亚州（美国）",
    description:
      "Display name of Render's `virginia` region, shown where a resource's region is read (w4/693)",
  },
  "common.createOptional": {
    message: "可选",
    description:
      "Mark beside a create-page row label whose field may be left blank (Render's 'Optional')",
  },
  "common.createAdvanced": {
    message: "高级",
    description: "Heading of the collapsed Advanced card on a create page",
  },
  "common.planListShowAll": {
    message: "显示全部套餐",
    description: "Button under a create page's plan list that shows every plan",
  },
  "common.planListShowFewer": {
    message: "收起套餐",
    description:
      "Button under an expanded plan list that folds it back to the first plans",
  },
  "common.createBlockedName": {
    message: "请输入名称后继续。",
    description:
      "Create page action bar: why Submit is held while the name is empty or invalid",
  },
  "common.createBlockedFields": {
    message: "请填写或修正上面的字段后继续。",
    description:
      "Create page action bar: why Submit is held while a field is missing or invalid",
  },
  "common.createTypePostgres": {
    message: "Postgres",
    description:
      "Postgres in the New menu and a create page's type switcher (Render's product name)",
  },
  "common.createTypeSwitcher": {
    message: "{type}：选择服务类型",
    description:
      "Accessible name of the breadcrumb menu that switches a create page to another resource type ({type} = the current page's heading)",
  },
  "common.createTypeProject": {
    message: "项目",
    description: "Project in the New menu (Render's bare 'Project')",
  },
  "common.createTypeBlueprint": {
    message: "蓝图",
    description: "Blueprint in the New menu (Render's bare 'Blueprint')",
  },
  "common.createTypeBuildSource": {
    message: "构建源",
    description: "Build Source in the New menu",
  },
  "common.createTypeSandboxGroup": {
    message: "沙箱组",
    description:
      "Sandbox Group in the New menu (Render's bare 'Sandbox Group')",
  },
  "common.createTypeWebhook": {
    message: "Webhook",
    description:
      "Webhook in the New menu (bex's own create, after Render's entries)",
  },
  "common.createBlockedNameTaken": {
    message: "请换一个未被使用的名称后继续。",
    description:
      "Create page action bar: why Submit is held while the name is already in use",
  },
  "common.createRequired": {
    message: "必填",
    description:
      "Mark beside a create-page row label whose field must be filled before Submit",
  },
  "common.createBlockedLoadingPlans": {
    message: "正在加载套餐…",
    description:
      "Create page action bar: Submit waits only for the plan catalog to load (not the user's fault)",
  },
  "common.datastoreViewDocs": {
    message: "查看文档",
    description:
      "A Postgres's or Key Value's header link to bex's docs for it, opening in a new tab (Render's View docs, w3/m675)",
  },
  "common.datastoreConnect": {
    message: "连接",
    description:
      "A Postgres's or Key Value's header button opening its connection strings (Render's Connect, w3/m675)",
  },
  "common.datastoreConnectInternal": {
    message: "内部",
    description:
      "Connect menu tab: the private-network URL (Render's Internal tab)",
  },
  "common.datastoreConnectExternal": {
    message: "外部",
    description:
      "Connect menu tab: the public URL and the CLI command (Render's External tab)",
  },
  "common.datastoreConnectInternalBody": {
    message: "同一区域内你所有的 bex 服务都可以通过私有网络{link}。",
    description:
      "Connect menu's Internal line; {link} is common.datastoreConnectInternalLink, linking the docs",
  },
  "common.datastoreConnectInternalLink": {
    message: "在内部通信",
    description: "Linked words inside common.datastoreConnectInternalBody",
  },
  "common.datastoreConnectExternalBody": {
    message: "从 bex {link}连接。",
    description:
      "Connect menu's External line once an IP rule admits traffic; {link} is common.datastoreConnectExternalLink, linking the docs",
  },
  "common.datastoreConnectExternalLink": {
    message: "之外的服务",
    description: "Linked words inside common.datastoreConnectExternalBody",
  },
  "common.datastoreConnectExternalClosed": {
    message: "不允许外部流量。请在「{link}」部分添加 IP 地址。",
    description:
      "Connect menu's External tab while no IP rule admits traffic (Render's note); {link} is the Info card holding the IP rules, named as that card is",
  },
  "common.datastoreConnectCliBody": {
    message: "使用 {link} 连接",
    description:
      "Connect menu's line above the CLI command; {link} is common.datastoreConnectCliLink, linking the CLI docs",
  },
  "common.datastoreConnectCliLink": {
    message: "bex 命令行工具",
    description:
      "Linked words inside common.datastoreConnectCliBody: the bex command-line tool",
  },
  "common.datastoreConnectRetry": {
    message: "加载失败。要重试吗？",
    description:
      "Accessible name and tooltip of the retry button on a Connect menu value that failed to load",
  },
  "common.unsavedChangesTitle": {
    message: "放弃未保存的更改？",
    description:
      "Title of the dialog that holds a navigation away from an edited, unsaved settings row (Render's Discard unsaved changes?, w3/118)",
  },
  "common.unsavedChangesBody": {
    message:
      "你编辑了{fields}，但尚未保存更改。如果现在离开，你的更改将会丢失。",
    description:
      "That dialog's sentence; {fields} is common.unsavedChangesFields, in bold, naming the edited rows",
  },
  "common.unsavedChangesFields_other": {
    message: "{list}字段",
    description:
      'The edited rows that dialog names, in bold; {list} is their labels joined as the language joins a list (Render: "the A field", "the A and B fields", "the A, B, and C fields")',
  },
  "common.unsavedChangesKeepEditing": {
    message: "继续编辑",
    description:
      "That dialog's button that stays on the page and focuses the first edited field",
  },
  "common.unsavedChangesDiscard": {
    message: "放弃",
    description:
      "That dialog's destructive button that leaves the page and drops the edits",
  },
};

export default zhCommon;
