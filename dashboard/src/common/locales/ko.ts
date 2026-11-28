import type { TranslationEntry } from "@/i18n";

const koCommon: Record<string, TranslationEntry> = {
  "common.navRouter": {
    message: "라우터",
    description: "Router navigation item",
  },
  "common.protectedConfirmationTitle": {
    message: "보호된 환경 확인이 필요합니다",
    description:
      "Title for the retry dialog shown after a protected environment blocks a destructive action",
  },
  "common.protectedConfirmationBody": {
    message:
      "{name} 항목은 보호된 환경에 속해 있습니다. 계속하려면 아래에 정확한 명령을 입력하세요.",
    description:
      "Resource-neutral explanation for a protected environment confirmation gate",
  },
  "common.protectedConfirmationPrompt": {
    message: "확인하려면 아래에 {phrase} 를 입력하세요.",
    description:
      "Prompt naming the server-issued protected environment confirmation phrase",
  },
  "common.confirmPhrasePrompt": {
    message: "계속하려면 {phrase} 를 입력하세요",
    description:
      "Label above the typed-confirmation input in a phrase-gated confirm dialog.",
  },
  "common.close": {
    message: "닫기",
    description:
      "Accessible name of the shared dialog's built-in close (X) button.",
  },
  "common.sidebarTitle": {
    message: "사이드바",
    description: "Screen-reader title of the mobile navigation drawer.",
  },
  "common.sidebarMobileDescription": {
    message: "모바일 사이드바를 표시합니다.",
    description: "Screen-reader description of the mobile navigation drawer.",
  },
  "common.skipToMainContent": {
    message: "본문으로 건너뛰기",
    description:
      "Label of the first-tab skip link that moves keyboard focus past the sidebar and header to the page content.",
  },
  "common.sidebarToggle": {
    message: "사이드바 전환",
    description:
      "Accessible name of the button that opens or collapses the navigation sidebar.",
  },
  "common.sidebarResize": {
    message: "사이드바 크기 조절",
    description: "Accessible name of the sidebar's draggable resize separator.",
  },
  "common.sidebarResizeHint": {
    message: "드래그하여 크기 조절 · 더블 클릭하여 전환",
    description: "Hover tooltip on the sidebar's resize separator.",
  },
  "common.commandSuggestions": {
    message: "추천 항목",
    description:
      "Accessible name of a search or picker results list (the command menu's listbox).",
  },
  "common.comboboxPlaceholder": {
    message: "옵션 선택",
    description:
      "Fallback placeholder of a searchable picker when its caller supplies none.",
  },
  "common.comboboxEmpty": {
    message: "검색 결과가 없습니다",
    description:
      "Fallback text when a searchable picker's filter matches no option.",
  },
  "common.comboboxUseCustom": {
    message: 'Enter를 눌러 "{value}" 사용',
    description:
      "Hint under a picker's empty results: Enter keeps the typed value as entered. {value} is the typed text.",
  },
  "common.comboboxClear": {
    message: "입력 지우기",
    description: "Accessible name of a searchable picker's clear (X) button.",
  },
  "common.cancel": {
    message: "취소",
    description:
      "Generic Cancel button — dismisses a dialog or an inline form without saving.",
  },
  "common.pricePerRunMinute": {
    message: "실행 시간 ${price}/분",
    description:
      "A cron job's USD price per minute of run time, on a Change Compute Plan card (w4/546): it is billed per second only while a run executes; price is an API-supplied decimal without the currency symbol",
  },
  "common.pricePerMinute": {
    message: "${price}/분",
    description:
      "A cron job's USD price per minute of run time on a create-form plan card, short to fit beside the plan name; the form's hint says it is billed only while a run executes (w4/546). price is an API-supplied decimal without the currency symbol",
  },
  "common.pricePerMonth": {
    message: "${price}/월",
    description:
      "Monthly USD price shown on a resource plan card; price is an API-supplied decimal without the currency symbol",
  },
  "common.reviewChangesTitle": {
    message: "변경 사항을 적용할까요?",
    description:
      "Title of Render's plan review dialog, shown before a compute or storage change applies (w3/m674)",
  },
  "common.reviewChangesTotal": {
    message: "합계",
    description:
      "Plan review dialog: the label of the monthly total under the priced lines",
  },
  "common.reviewEditComputePlan": {
    message: "컴퓨팅 플랜 편집",
    description:
      "Plan review dialog: accessible name of the Compute plan line's Edit button, which returns to the plan picker",
  },
  "common.planUnavailable": {
    message: "사용 불가",
    description:
      "Shown in place of the price on a disabled plan card whose compute plan is paused (it cannot be selected for now); the server's full reason is the card's tooltip",
  },
  "common.planConnections_other": {
    message: "연결 {formatted}개",
    description:
      "A plan card's connection-limit line, under its RAM: the client connections the plan accepts (Render's Key Value cards read \"50 Connections\"); {formatted} is the locale-formatted count",
  },
  "common.protectedConfirmationRetry": {
    message: "확인 후 저장",
    description:
      "Confirm button in the protected environment retry dialog raised by a settings save",
  },
  "common.protectedConfirmationCancel": {
    message: "취소",
    description:
      "Cancel button in the protected environment confirmation dialog",
  },
  "common.navEnvGroups": {
    message: "환경 그룹",
    description: "Workspace sidebar link to environment groups",
  },
  "common.navBuildSources": {
    message: "빌드 소스",
    description: "Workspace sidebar link to build sources",
  },
  "common.navSandboxGroups": {
    message: "샌드박스 그룹",
    description: "Workspace sidebar link to sandbox groups",
  },
  "common.appName": {
    message: "bex",
    description: "Product name shown in the dashboard chrome",
  },
  "common.headDescription": {
    message:
      "오픈 소스 AI 네이티브 Render 대안인 bex로 직접 제어하는 인프라에서 애플리케이션을 배포하고 운영하세요.",
    description:
      "Generic dashboard description used in description, Open Graph, and Twitter metadata",
  },
  "common.headImageAlt": {
    message: "bex 대시보드 로고",
    description: "Alternative text for the dashboard's generic social image",
  },
  "common.loading": {
    message: "로딩 중…",
    description: "Generic loading state label",
  },
  "common.topbarBreadcrumbs": {
    message: "탐색 경로",
    description: "Accessible label for the dashboard topbar hierarchy",
  },
  "common.topbarSearch": {
    message: "검색",
    description: "Open the workspace-wide topbar search",
  },
  "common.copyCode": {
    message: "코드 복사",
    description: "Accessible name of the copy button on a code block",
  },
  "common.topbarSearchPlaceholder": {
    message: "페이지와 리소스 검색…",
    description: "Placeholder in the workspace-wide topbar command search",
  },
  "common.topbarSearchDescription": {
    message: "대시보드 페이지와 워크스페이스 리소스를 검색합니다.",
    description: "Accessible description for the topbar command search",
  },
  "common.topbarSearchEmpty": {
    message: "일치하는 페이지나 리소스가 없습니다.",
    description: "Empty state in the workspace-wide topbar command search",
  },
  "common.copyField": {
    message: "{field} 복사",
    description:
      "Accessible name for a copy control when the caller only has a field label (m101/t014)",
  },
  "common.topbarNavigation": {
    message: "탐색",
    description: "Page-links group in the workspace-wide topbar search",
  },
  "common.topbarResources": {
    message: "리소스",
    description: "Workspace-resources group in the topbar search",
  },
  "common.topbarNew": {
    message: "새로 만들기",
    description: "Persistent topbar menu for creating a resource",
  },
  "common.topbarCreate": {
    message: "리소스 만들기",
    description: "Heading in the persistent topbar create menu",
  },
  "common.topbarHelp": {
    message: "도움말 및 리소스",
    description: "Accessible label for the topbar help menu",
  },
  "common.topbarDocumentation": {
    message: "문서",
    description: "Topbar help-menu link to bex documentation",
  },
  "common.topbarCliGuide": {
    message: "CLI 가이드",
    description: "Topbar help-menu link to the bex CLI guide",
  },
  "common.topbarRepository": {
    message: "GitHub 저장소",
    description: "Topbar help-menu link to the bex source repository",
  },
  "common.topbarWorkspaceSettings": {
    message: "워크스페이스 설정",
    description: "Workspace-settings label in topbar navigation and search",
  },
  "common.searchShortcutEnvVarsDocs": {
    message: "환경 변수",
    description:
      "Search shortcut to the docs page on environment variables and secrets (Render's 'Environment variables' shortcut)",
  },
  "common.searchShortcutEnvVarsDocsDescription": {
    message: "서비스의 환경 변수와 시크릿 설정",
    description:
      "One-line description beside the environment variables docs search shortcut",
  },
  "common.searchShortcutCustomDomainsDocs": {
    message: "사용자 지정 도메인",
    description: "Search shortcut to the custom domains docs page",
  },
  "common.searchShortcutCustomDomainsDocsDescription": {
    message: "서비스에 사용자 지정 도메인 추가",
    description:
      "One-line description beside the custom domains docs search shortcut",
  },
  "common.searchShortcutGitCredentials": {
    message: "Git 자격 증명",
    description:
      "Search shortcut to Account Settings → Integrations, where GitHub is connected",
  },
  "common.searchShortcutGitCredentialsDescription": {
    message: "GitHub 연결",
    description:
      "One-line description beside the Git credentials search shortcut; bex connects GitHub only",
  },
  "common.searchShortcutAuditLogs": {
    message: "감사 로그",
    description: "Search shortcut to Workspace Settings → Audit Logs",
  },
  "common.searchShortcutAuditLogsDescription": {
    message: "감사 로그 내보내기",
    description: "One-line description beside the Audit Logs search shortcut",
  },
  "common.searchShortcutApiKeys": {
    message: "API 키",
    description: "Search shortcut to Account Settings → API Keys",
  },
  "common.searchShortcutApiKeysDescription": {
    message: "API 키 발급",
    description: "One-line description beside the API Keys search shortcut",
  },
  "common.searchShortcutSshKeys": {
    message: "SSH 키",
    description: "Search shortcut to Account Settings → SSH Public Keys",
  },
  "common.searchShortcutSshKeysDescription": {
    message: "SSH 공개 키 등록",
    description: "One-line description beside the SSH Keys search shortcut",
  },
  "common.searchShortcutCliTokens": {
    message: "CLI 토큰",
    description:
      "Search shortcut to Account Settings → Connected agents, which lists the CLI's tokens",
  },
  "common.searchShortcutCliTokensDescription": {
    message: "CLI 토큰 확인 및 취소",
    description: "One-line description beside the CLI Tokens search shortcut",
  },
  "common.searchShortcutRegistryCredentials": {
    message: "컨테이너 레지스트리 자격 증명",
    description: "Search shortcut to Workspace Settings → Registry Credentials",
  },
  "common.searchShortcutRegistryCredentialsDescription": {
    message: "외부 이미지의 자격 증명 관리",
    description:
      "One-line description beside the container registry credentials search shortcut",
  },
  "common.searchShortcutMcpServer": {
    message: "MCP 서버",
    description: "Search shortcut to the MCP server docs page",
  },
  "common.searchShortcutMcpServerDescription": {
    message: "AI 앱에서 bex 서비스 관리",
    description: "One-line description beside the MCP Server search shortcut",
  },
  "common.topbarSwitchProject": {
    message: "프로젝트 전환",
    description: "Heading in a project breadcrumb dropdown",
  },
  "common.topbarSwitchEnvironment": {
    message: "환경 전환",
    description: "Heading in an environment breadcrumb dropdown",
  },
  "common.topbarSwitchService": {
    message: "서비스 전환",
    description: "Heading in a service breadcrumb dropdown",
  },
  "common.topbarAllResources": {
    message: "모든 리소스",
    description: "Service breadcrumb menu link back to workspace resources",
  },
  "common.topbarProjectResource": {
    message: "프로젝트",
    description: "Project kind label in workspace-wide search results",
  },
  "common.topbarServiceResource": {
    message: "서비스",
    description: "Service kind label in workspace-wide search results",
  },
  "common.sudoCommandLabel": {
    message: "Sudo 명령",
    description:
      "Input label on Render-style destructive type-to-confirm gates (live capture: docs/render-artifacts/workspace-lifecycle.md)",
  },
  "common.navIntegrationsGroup": {
    message: "연동",
    description:
      "Sidebar nav section label (webhooks, notifications) — Render's grouping",
  },
  "common.navWorkspaceGroup": {
    message: "워크스페이스",
    description:
      "Sidebar nav section label (usage, settings) — Render's grouping",
  },
  "common.navMonitorGroup": {
    message: "모니터링",
    description:
      "Service sidebar section label (logs, metrics) — Render's grouping",
  },
  "common.navWebhooks": {
    message: "웹훅",
    description: "Sidebar nav link to the outbound event webhooks page",
  },
  "common.labelInvalid": {
    message: "줄바꿈이나 제어 문자 없이 100자 이내의 이름을 입력하세요.",
    description:
      "Inline validation under a free-form name field (project, environment, group, webhook, credential, Blueprint)",
  },
  "common.navNotifications": {
    message: "알림",
    description: "Sidebar nav link to the notification settings page",
  },
  "common.navProjects": {
    message: "프로젝트",
    description:
      "Sidebar nav link to the unified projects page (services, databases, key value grouped together)",
  },
  "common.navUsage": {
    message: "결제",
    description:
      "Sidebar nav link to the workspace billing page (/billing, renamed from Usage in w5/m70; key kept to avoid churn)",
  },
  "common.navBlueprints": {
    message: "Blueprints",
    description:
      "Sidebar nav link to the blueprints management page (IaC stacks auto-registered on deploy)",
  },
  "common.navAgents": {
    message: "에이전트",
    description:
      "Sidebar nav link to the cloud coding-agent sessions page (/agents)",
  },
  "common.navAutomations": {
    message: "자동화",
    description:
      "Sidebar nav link to the scheduled agent automations page (/automations)",
  },
  "common.navSettings": {
    message: "설정",
    description:
      "Sidebar nav link to the workspace settings page (the sidebar is workspace-scoped; account settings hang off the user menu)",
  },
  "common.navBackToDashboard": {
    message: "대시보드",
    description:
      "Contextual project sidebar's back link label, shown on a project's Overview page (links up to the workspace Overview)",
  },
  "common.navBackToProject": {
    message: "프로젝트",
    description:
      "Contextual project sidebar's back link label, shown on a project's Settings page (links up to the project's Overview)",
  },
  "common.navBackToEnvironment": {
    message: "환경",
    description:
      "Contextual project sidebar's back link label, shown on an environment's settings page (links up to the environment's section on the project page, Render's words, w1/m683)",
  },
  "common.navProjectOverview": {
    message: "개요",
    description:
      "Contextual project sidebar nav link to the project's own Overview page",
  },
  "common.navObservabilityGroup": {
    message: "관측성",
    description:
      "Contextual project sidebar section label above the project's log views (Render's OBSERVABILITY group, w3/m505)",
  },
  "common.navManageGroup": {
    message: "관리",
    description: "Contextual project sidebar section label above Settings",
  },
  "common.changeLanguage": {
    message: "언어 변경",
    description: "Accessible label for the language switcher button",
  },
  "common.userMenuSettings": {
    message: "계정 설정",
    description:
      "User menu item that navigates to account settings (/settings)",
  },
  "common.userMenuTheme": {
    message: "테마",
    description: "User menu submenu label for theme selection",
  },
  "common.userMenuLanguage": {
    message: "언어",
    description: "User menu submenu label for language selection",
  },
  "common.userMenuThemeLight": {
    message: "라이트",
    description: "Light theme option label",
  },
  "common.userMenuThemeDark": {
    message: "다크",
    description: "Dark theme option label",
  },
  "common.userMenuThemeSystem": {
    message: "시스템",
    description: "System theme option label",
  },
  "common.themeNotSaved": {
    message: "테마를 저장하지 못했습니다. 이전 테마로 되돌렸습니다.",
    description:
      "Toast when a theme change could not be saved to the account (w3/m531); the previous theme is put back",
  },
  "common.userMenuLogOut": {
    message: "로그아웃",
    description: "User menu item that signs the user out",
  },
  "common.userMenuOpen": {
    message: "계정 메뉴",
    description:
      "Accessible name of the avatar button that opens the account menu (w4/407)",
  },
  "common.userMenuOpenFor": {
    message: "{name} 계정 메뉴",
    description:
      "Accessible name of the avatar button that opens the account menu, naming the signed-in identity (w4/407)",
  },
  "common.goHome": {
    message: "홈으로 이동",
    description: "Button label that navigates back to the home page",
  },
  "common.goBack": {
    message: "뒤로 가기",
    description: "Button label that navigates to the previous page",
  },
  "common.planCatalogUnavailable": {
    message:
      "요금제를 불러오지 못해 아직 선택할 수 없습니다. 다른 입력 내용은 유지됩니다.",
    description:
      "A plan picker whose catalog read failed or returned no plans; shown with a Try again button that re-reads only the catalog (w4/m218)",
  },
  "common.tryAgain": {
    message: "다시 시도",
    description: "Button label that retries after an error",
  },
  "common.notFoundTitle": {
    message: "페이지를 찾을 수 없습니다",
    description: "404 page heading",
  },
  "common.notFoundDescription": {
    message: "찾으시는 페이지가 존재하지 않거나 이동되었습니다.",
    description: "404 page explanatory text",
  },
  "common.errorTitle": {
    message: "문제가 발생했습니다",
    description: "Generic error page heading",
  },
  "common.errorDefaultMessage": {
    message: "문제가 발생했습니다.",
    description: "Fallback error message when none is provided",
  },
  "common.loadErrorRetry": {
    message:
      "불러오는 중 문제가 발생했습니다. 페이지를 새로 고친 후 다시 시도하세요.",
    description:
      "Body of a panel whose read failed with no server reason to show (a network failure or throttling)",
  },
  "common.serviceNotFoundTitle": {
    message: "서비스를 찾을 수 없습니다",
    description:
      "Title of a service page whose id leads nowhere (Render's ResourceNotFound, w3/m706)",
  },
  "common.serviceDeletedTitle": {
    message: "서비스가 삭제되었습니다",
    description:
      "Title of a service page whose service the dashboard had read and is now gone (w3/m706)",
  },
  "common.cronJobNotFoundTitle": {
    message: "Cron Job을 찾을 수 없습니다",
    description: "Title of a cron job page whose id leads nowhere (w3/m706)",
  },
  "common.cronJobDeletedTitle": {
    message: "Cron Job이 삭제되었습니다",
    description:
      "Title of a cron job page whose cron job the dashboard had read and is now gone (w3/m706)",
  },
  "common.sandboxGroupNotFoundTitle": {
    message: "샌드박스 그룹을 찾을 수 없습니다",
    description:
      "Title of a sandbox group page whose id leads nowhere (w3/m706)",
  },
  "common.sandboxGroupDeletedTitle": {
    message: "샌드박스 그룹이 삭제되었습니다",
    description:
      "Title of a sandbox group page whose group the dashboard had read and is now gone (w3/m706)",
  },
  "common.deployNotFoundTitle": {
    message: "배포를 찾을 수 없습니다",
    description:
      "Title of a deploy page whose deploy id leads nowhere under a live service (w3/m706)",
  },
  "common.buildNotFoundTitle": {
    message: "빌드를 찾을 수 없습니다",
    description:
      "Title of a build page whose build id leads nowhere under a live build source (w3/m706)",
  },
  "common.resourceNotFoundBody": {
    message:
      "{id}을(를) 찾을 수 없습니다. 삭제되었거나 링크가 잘못되었을 수 있습니다.",
    description:
      "Body of a resource page whose id leads nowhere; {id} is the id, drawn as a code chip (w3/m706)",
  },
  "common.resourceDeletedBody": {
    message: "{name}({id})이(가) 삭제되었습니다.",
    description:
      "Body of a resource page whose resource the dashboard had read and is now gone; {name} is its name, {id} its id (w3/m706)",
  },
  "common.resourceNotFoundForBody": {
    message:
      "{parent}의 {id}을(를) 찾을 수 없습니다. 링크가 잘못되었을 수 있습니다.",
    description:
      "Body of a deploy or build page whose id leads nowhere; {id} is the id, {parent} the service or build source it was opened under (w3/m706)",
  },
  "common.goToProject": {
    message: "프로젝트로 이동",
    description:
      "Button on a missing resource's page that opens the project it was in (w3/m706)",
  },
  "common.returnHome": {
    message: "홈으로 돌아가기",
    description:
      "Button on a missing, denied or failed resource's page that opens the home page (w3/m706)",
  },
  "common.viewDeploys": {
    message: "배포 보기",
    description:
      "Button on a missing deploy's page that opens its service's deploys (w3/m706)",
  },
  "common.viewBuilds": {
    message: "빌드 보기",
    description:
      "Button on a missing build's page that opens its build source's builds (w3/m706)",
  },
  "common.goToWebhooks": {
    message: "웹훅으로 이동",
    description:
      "Button on a missing or denied webhook's page that opens the webhooks list (w3/m706)",
  },
  "common.resourceDoesNotExist": {
    message: "{id}이(가) 존재하지 않습니다.",
    description:
      "Body of the Page not found state a resource page draws for a dead id; {id} is the id (w3/m706)",
  },
  "common.accessDeniedTitle": {
    message: "접근이 거부되었습니다",
    description: "Title of a resource page whose read was refused (w3/m706)",
  },
  "common.accessDeniedBody": {
    message: "{id}에 대한 접근 권한이 없습니다.",
    description:
      "Body of a resource page whose read was refused; {id} is the id (w3/m706)",
  },
  "common.accessDeniedBodyNoId": {
    message: "이 리소스에 대한 접근 권한이 없습니다.",
    description: "Body of a refused page that names no id (w3/m706)",
  },
  "common.resourceLoadFailedTitle": {
    message: "오류가 발생했습니다",
    description: "Title of a resource page whose read failed (w3/m706)",
  },
  "common.resourceLoadFailedBody": {
    message: "{id}을(를) 불러올 수 없습니다.",
    description:
      "Body of a resource page whose read failed; {id} is the id (w3/m706)",
  },
  "common.copyId": {
    message: "ID 복사",
    description:
      "Accessible name of the copy button beside an id in a resource state (w3/m706)",
  },
  "common.copyIdError": {
    message: "ID를 복사할 수 없습니다",
    description:
      "Toast when copying an id from a resource state fails (w3/m706)",
  },
  "common.resourceErrorBody": {
    message: "리소스를 불러오지 못했습니다. API를 확인한 후 다시 시도하세요.",
    description:
      "Body text of a detail page's inline error state when the resource query failed",
  },
  "common.sessionExpiredTitle": {
    message: "세션이 만료되었습니다",
    description:
      "Heading of the inline state shown when a resource query fails with a 401 (expired session)",
  },
  "common.sessionExpiredBody": {
    message:
      "계속하려면 다시 로그인하세요. 로그인하면 이 페이지로 바로 돌아옵니다.",
    description:
      "Body text of the expired-session inline state, reassuring the user their place is preserved",
  },
  "common.signIn": {
    message: "로그인",
    description:
      "Button label that sends an expired session back to the login page",
  },
  "common.colActions": {
    message: "작업",
    description:
      "Screen-reader-only header for a table's trailing row-actions column",
  },
  "common.justNow": {
    message: "방금 전",
    description:
      'Elapsed-time text for an instant under a minute old ("Deployed just now")',
  },
  "common.timeLocal": {
    message: "로컬",
    description:
      "Timestamp tooltip: label of the row showing the instant in the viewer's local timezone",
  },
  "common.timeUtc": {
    message: "UTC",
    description:
      "Timestamp tooltip: label of the row showing the instant in UTC",
  },
  "common.timeUnix": {
    message: "타임스탬프",
    description:
      "Timestamp tooltip: label of the row showing the instant as Unix milliseconds",
  },
  "common.timeFromPrevious": {
    message: "이전 줄부터",
    description:
      "Timestamp tooltip (w4/716): label of a log line's row showing the time since the line before it, Render's 'From previous'",
  },
  "common.timeFromStart": {
    message: "시작부터",
    description:
      "Timestamp tooltip (w4/716): label of a deploy log line's row showing the time since the deploy began, Render's 'From start'",
  },
  "common.timeRowLabel": {
    message: "{label}:",
    description:
      "Timestamp tooltip (w4/716): a row's label followed by Render's colon; {label} is the zone ('PST'), UTC, Timestamp, From previous or From start",
  },
  "common.timeCopied": {
    message: "복사됨",
    description:
      "Timestamp tooltip (w4/716): toast after a row's copy button copied its value, Render's 'Copied!'",
  },
  "common.timeCopyFailed": {
    message: "시간을 복사할 수 없습니다",
    description:
      "Timestamp tooltip (w4/716): toast when the clipboard refused a row's value",
  },
  "common.ipAllowListProxiedTitle_other": {
    message:
      "허용 목록이 사용자 지정 도메인 {count}개의 클라이언트를 확인할 수 없습니다",
    description:
      "Title of the warning shown under an inbound IP allowlist when several custom domains it covers are proxied by Cloudflare (w1/m171)",
  },
  "common.ipAllowListProxiedBody_other": {
    message:
      "이 도메인은 Cloudflare를 통해 프록시되므로 허용 목록이 방문자가 아닌 Cloudflare의 주소와 비교됩니다. 목록은 저장되지만, 이 도메인에서는 허용하려던 방문자가 차단됩니다.",
    description:
      "Body of the Cloudflare-proxied allowlist warning, followed by the list of proxied hostnames (w1/m171)",
  },
  "common.ipAllowListProxiedFix": {
    message:
      "실제 클라이언트 주소로 필터링하려면 Cloudflare에서 DNS 레코드를 DNS only(회색 구름)로 설정하세요.",
    description:
      "How to fix the Cloudflare-proxied allowlist warning: switch the record from proxied (orange cloud) to DNS only (w1/m171)",
  },
  "common.inboundIpAllowedRanges_other": {
    message: "개의 IP 범위가 프라이빗 네트워크 외부에서 허용됩니다.",
    description:
      "Status line of a Networking panel after a count badge (Render's, w4/m710): how many IPv4 ranges every level of inbound IP rules (workspace, environment, the resource's own) allows together",
  },
  "common.inboundIpValidateLabel": {
    message: "IP 주소 확인",
    description:
      "Label of the Networking panel's address check field (Render's Validate IP address, w4/m710)",
  },
  "common.inboundIpCheckEmpty": {
    message: "IP 주소를 입력하세요",
    description:
      "Validate IP address field, nothing typed yet (Render's tooltip, w4/m710)",
  },
  "common.inboundIpCheckInvalid": {
    message: "유효하지 않은 IP 주소입니다",
    description:
      "Validate IP address field: the text is not an IPv4 address (Render's, w4/m710)",
  },
  "common.inboundIpCheckAllowed": {
    message: "이 IP 주소의 접근이 허용됩니다",
    description:
      "Validate IP address field: every level of inbound IP rules admits the address (Render's, w4/m710)",
  },
  "common.inboundIpRefusedWorkspace": {
    message:
      "워크스페이스 인바운드 IP 규칙에 따라 이 IP 주소의 접근이 허용되지 않습니다.",
    description:
      "Validate IP address field: the workspace's inbound IP rules refuse the address (Render's, w4/m710)",
  },
  "common.inboundIpRefusedEnvironment": {
    message:
      "환경 인바운드 IP 규칙에 따라 이 IP 주소의 접근이 허용되지 않습니다.",
    description:
      "Validate IP address field: the environment's inbound IP rules refuse the address (Render's, w4/m710)",
  },
  "common.inboundIpRefusedOwn": {
    message:
      "{type} 인바운드 IP 규칙에 따라 이 IP 주소의 접근이 허용되지 않습니다.",
    description:
      "Validate IP address field: the resource's own (unsaved) inbound IP rules refuse the address; {type} is its type, e.g. Web Service, PostgreSQL, Key Value, Environment (Render's, w4/m710)",
  },
  "common.inboundIpAffectedBy": {
    message: "적용되는 규칙:",
    description:
      "Label before the breadcrumb of a higher level of inbound IP rules that also applies to this resource (Render's 'Affected by:', w4/m710)",
  },
  "common.inboundIpLevelWorkspace": {
    message: "워크스페이스",
    description:
      "Breadcrumb item naming the workspace level of inbound IP rules (Render's, w4/m710)",
  },
  "common.inboundIpLevelEnvironment": {
    message: "환경",
    description:
      "Breadcrumb item naming the environment level of inbound IP rules, and an environment's type in the own-rules heading and refusal (Render's, w4/m710)",
  },
  "common.inboundIpViewRules": {
    message: "규칙 보기",
    description:
      "Link, opening in a new tab, to where a higher level of inbound IP rules is edited (Render's View rules, w4/m710)",
  },
  "common.inboundIpSource": {
    message: "소스",
    description:
      "Column header of a read-only inbound IP rules table: the CIDR range (Render's Source, w4/m710)",
  },
  "common.inboundIpDescription": {
    message: "설명",
    description:
      "Column header of a read-only inbound IP rules table: the rule's description (Render's, w4/m710)",
  },
  "common.inboundIpBlockedWorkspace": {
    message: "워크스페이스 인바운드 IP 규칙이 모든 인터넷 트래픽을 차단합니다.",
    description:
      "Warning in place of the table when the workspace has no inbound IP rule (Render's, w4/m710)",
  },
  "common.inboundIpBlockedEnvironment": {
    message: "환경 인바운드 IP 규칙이 모든 인터넷 트래픽을 차단합니다.",
    description:
      "Warning in place of the table when the environment has no inbound IP rule (Render's, w4/m710)",
  },
  "common.inboundIpOwnRules": {
    message: "{type} 인바운드 IP 규칙",
    description:
      "Heading above a resource's own editable inbound IP rules; {type} is its type, e.g. Web Service, PostgreSQL, Key Value, Environment (Render's, w4/m710)",
  },
  "common.inboundIpTypePostgres": {
    message: "PostgreSQL",
    description:
      "A Postgres database's type as the inbound IP rules heading and refusal name it (Render's userFacingType, w4/m710)",
  },
  "common.inboundIpLoadError": {
    message:
      "함께 적용되는 워크스페이스 및 환경 인바운드 IP 규칙을 불러오지 못했습니다. 새로고침한 뒤 다시 시도하세요.",
    description:
      "Shown in place of the workspace and environment inbound IP rules when one of them could not be read (w4/m710)",
  },
  "common.relativeAgeNow": {
    message: "지금",
    description: "Compact relative age under one minute (w4/m188)",
  },
  "common.relativeAgeMinutes": {
    message: "{n}분",
    description: "Compact relative age in whole minutes, e.g. 4m (w4/m188)",
  },
  "common.relativeAgeHours": {
    message: "{n}시간",
    description: "Compact relative age in whole hours, e.g. 3h (w4/m188)",
  },
  "common.relativeAgeDays": {
    message: "{n}일",
    description: "Compact relative age in whole days, e.g. 5d (w4/m188)",
  },
  "common.relativeAgeMonths": {
    message: "{n}개월",
    description: "Compact relative age in 30-day months, e.g. 2mo (w4/m188)",
  },
  "common.relativeAgeYears": {
    message: "{n}년",
    description: "Compact relative age in 365-day years, e.g. 1y (w4/m188)",
  },
  "common.relativeUntilNow": {
    message: "지금",
    description:
      "Countdown to a future instant less than one minute away (w4/m188)",
  },
  "common.relativeUntilMinutes": {
    message: "{n}분 후",
    description: "Compact countdown in whole minutes, e.g. in 5m (w4/m188)",
  },
  "common.relativeUntilHours": {
    message: "{n}시간 후",
    description: "Compact countdown in whole hours, e.g. in 3h (w4/m188)",
  },
  "common.relativeUntilDays": {
    message: "{n}일 후",
    description: "Compact countdown in whole days, e.g. in 83d (w4/m188)",
  },
  "common.openBilling": {
    message: "결제 열기",
    description:
      "Link from a plan notice (a Pro-and-higher feature on Hobby) to the Billing page (w8/m138)",
  },
  "common.billingSettlingTitle": {
    message: "최종 사용량 정산 중…",
    description:
      "Title while a deletion waits for the workspace's final usage to be charged (BILLING_SETTLING); retried automatically",
  },
  "common.billingSettlingDescription": {
    message:
      "bex가 삭제 전에 최종 사용량을 청구하고 있습니다. 1~2분 정도 걸릴 수 있으며 자동으로 재시도되니 이 페이지를 열어 두세요.",
    description:
      "Body while a deletion waits for the final usage to be charged (BILLING_SETTLING)",
  },
  "common.billingSettlingTimeout": {
    message:
      "최종 사용량을 아직 처리하는 중입니다. 몇 분 후에 다시 삭제해 보세요.",
    description:
      "Deletion error after BILLING_SETTLING retries ran out (about three minutes)",
  },
  "common.balanceDueTitle": {
    message: "미결제 잔액",
    description:
      "Title of the alert when a deletion is refused because the workspace has an unpaid balance (BILLING_BALANCE_DUE)",
  },
  "common.balanceDueAmount": {
    message:
      "이 워크스페이스에 미결제 잔액이 있습니다({amount}). 청구서를 결제한 후 다시 삭제하세요.",
    description: "Balance-due deletion refusal with the amount",
  },
  "common.balanceDue": {
    message:
      "이 워크스페이스에 미결제 잔액이 있습니다. 청구서를 결제한 후 다시 삭제하세요.",
    description: "Balance-due deletion refusal without an amount",
  },
  "common.balanceDueNamedAmount": {
    message:
      "{name} 워크스페이스에 미결제 잔액이 있습니다({amount}). 청구서를 결제한 후 계정을 다시 삭제하세요.",
    description:
      "Account-deletion refusal naming the workspace with the unpaid balance and its amount",
  },
  "common.balanceDueNamed": {
    message:
      "{name} 워크스페이스에 미결제 잔액이 있습니다. 청구서를 결제한 후 계정을 다시 삭제하세요.",
    description:
      "Account-deletion refusal naming the workspace with the unpaid balance",
  },
  "common.balanceDuePay": {
    message: "청구서 결제",
    description: "Link to Stripe's hosted invoice page, opens in a new tab",
  },
  "common.regionFrankfurt": {
    message: "프랑크푸르트, 독일",
    description:
      "Display name of Render's `frankfurt` region, shown where a resource's region is read (w4/693)",
  },
  "common.regionOhio": {
    message: "오하이오, 미국",
    description:
      "Display name of Render's `ohio` region, shown where a resource's region is read (w4/693)",
  },
  "common.regionOregon": {
    message: "오리건, 미국",
    description:
      "Display name of Render's `oregon` region, shown where a resource's region is read (w4/693)",
  },
  "common.regionSingapore": {
    message: "싱가포르",
    description:
      "Display name of Render's `singapore` region, shown where a resource's region is read (w4/693)",
  },
  "common.regionVirginia": {
    message: "버지니아, 미국",
    description:
      "Display name of Render's `virginia` region, shown where a resource's region is read (w4/693)",
  },
  "common.createOptional": {
    message: "선택 사항",
    description:
      "Mark beside a create-page row label whose field may be left blank (Render's 'Optional')",
  },
  "common.createAdvanced": {
    message: "고급",
    description: "Heading of the collapsed Advanced card on a create page",
  },
  "common.planListShowAll": {
    message: "모든 플랜 보기",
    description: "Button under a create page's plan list that shows every plan",
  },
  "common.planListShowFewer": {
    message: "플랜 접기",
    description:
      "Button under an expanded plan list that folds it back to the first plans",
  },
  "common.createBlockedName": {
    message: "계속하려면 이름을 입력하세요.",
    description:
      "Create page action bar: why Submit is held while the name is empty or invalid",
  },
  "common.createBlockedFields": {
    message: "계속하려면 위 항목을 입력하거나 수정하세요.",
    description:
      "Create page action bar: why Submit is held while a field is missing or invalid",
  },
  "common.createTypePostgres": {
    message: "Postgres",
    description:
      "Postgres in the New menu and a create page's type switcher (Render's product name)",
  },
  "common.createTypeSwitcher": {
    message: "{type}: 서비스 유형 선택",
    description:
      "Accessible name of the breadcrumb menu that switches a create page to another resource type ({type} = the current page's heading)",
  },
  "common.createTypeProject": {
    message: "프로젝트",
    description: "Project in the New menu (Render's bare 'Project')",
  },
  "common.createTypeBlueprint": {
    message: "Blueprint",
    description: "Blueprint in the New menu (Render's bare 'Blueprint')",
  },
  "common.createTypeBuildSource": {
    message: "빌드 소스",
    description: "Build Source in the New menu",
  },
  "common.createTypeSandboxGroup": {
    message: "샌드박스 그룹",
    description:
      "Sandbox Group in the New menu (Render's bare 'Sandbox Group')",
  },
  "common.createTypeWebhook": {
    message: "웹훅",
    description:
      "Webhook in the New menu (bex's own create, after Render's entries)",
  },
  "common.createBlockedNameTaken": {
    message: "계속하려면 사용 중이지 않은 이름을 선택하세요.",
    description:
      "Create page action bar: why Submit is held while the name is already in use",
  },
  "common.createRequired": {
    message: "필수",
    description:
      "Mark beside a create-page row label whose field must be filled before Submit",
  },
  "common.createBlockedLoadingPlans": {
    message: "플랜을 불러오는 중…",
    description:
      "Create page action bar: Submit waits only for the plan catalog to load (not the user's fault)",
  },
  "common.datastoreViewDocs": {
    message: "문서 보기",
    description:
      "A Postgres's or Key Value's header link to bex's docs for it, opening in a new tab (Render's View docs, w3/m675)",
  },
  "common.datastoreConnect": {
    message: "연결",
    description:
      "A Postgres's or Key Value's header button opening its connection strings (Render's Connect, w3/m675)",
  },
  "common.datastoreConnectInternal": {
    message: "내부",
    description:
      "Connect menu tab: the private-network URL (Render's Internal tab)",
  },
  "common.datastoreConnectExternal": {
    message: "외부",
    description:
      "Connect menu tab: the public URL and the CLI command (Render's External tab)",
  },
  "common.datastoreConnectInternalBody": {
    message:
      "같은 리전의 모든 bex 서비스는 사설 네트워크로 {link}할 수 있습니다.",
    description:
      "Connect menu's Internal line; {link} is common.datastoreConnectInternalLink, linking the docs",
  },
  "common.datastoreConnectInternalLink": {
    message: "내부 통신",
    description: "Linked words inside common.datastoreConnectInternalBody",
  },
  "common.datastoreConnectExternalBody": {
    message: "bex {link}에서 연결합니다.",
    description:
      "Connect menu's External line once an IP rule admits traffic; {link} is common.datastoreConnectExternalLink, linking the docs",
  },
  "common.datastoreConnectExternalLink": {
    message: "외부의 서비스",
    description: "Linked words inside common.datastoreConnectExternalBody",
  },
  "common.datastoreConnectExternalClosed": {
    message:
      "외부 트래픽이 허용되지 않습니다. {link} 섹션에서 IP 주소를 추가하세요.",
    description:
      "Connect menu's External tab while no IP rule admits traffic (Render's note); {link} is the Info card holding the IP rules, named as that card is",
  },
  "common.datastoreConnectCliBody": {
    message: "{link}로 연결합니다",
    description:
      "Connect menu's line above the CLI command; {link} is common.datastoreConnectCliLink, linking the CLI docs",
  },
  "common.datastoreConnectCliLink": {
    message: "bex CLI 도구",
    description:
      "Linked words inside common.datastoreConnectCliBody: the bex command-line tool",
  },
  "common.datastoreConnectRetry": {
    message: "불러오지 못했습니다. 다시 시도할까요?",
    description:
      "Accessible name and tooltip of the retry button on a Connect menu value that failed to load",
  },
};

export default koCommon;
