import type { TranslationEntry } from "@/i18n";

const jaCommon: Record<string, TranslationEntry> = {
  "common.navRouter": {
    message: "ルーター",
    description: "Router navigation item",
  },
  "common.protectedConfirmationTitle": {
    message: "保護された環境の確認が必要です",
    description:
      "Title for the retry dialog shown after a protected environment blocks a destructive action",
  },
  "common.protectedConfirmationBody": {
    message:
      "{name} は保護された環境に属しています。続行するには、以下のコマンドをそのまま入力してください。",
    description:
      "Resource-neutral explanation for a protected environment confirmation gate",
  },
  "common.protectedConfirmationPrompt": {
    message: "確認するには、下に {phrase} と入力してください。",
    description:
      "Prompt naming the server-issued protected environment confirmation phrase",
  },
  "common.confirmPhrasePrompt": {
    message: "続行するには {phrase} と入力してください",
    description:
      "Label above the typed-confirmation input in a phrase-gated confirm dialog.",
  },
  "common.close": {
    message: "閉じる",
    description:
      "Accessible name of the shared dialog's built-in close (X) button.",
  },
  "common.sidebarTitle": {
    message: "サイドバー",
    description: "Screen-reader title of the mobile navigation drawer.",
  },
  "common.sidebarMobileDescription": {
    message: "モバイル用サイドバーを表示します。",
    description: "Screen-reader description of the mobile navigation drawer.",
  },
  "common.skipToMainContent": {
    message: "メインコンテンツへスキップ",
    description:
      "Label of the first-tab skip link that moves keyboard focus past the sidebar and header to the page content.",
  },
  "common.sidebarToggle": {
    message: "サイドバーを切り替え",
    description:
      "Accessible name of the button that opens or collapses the navigation sidebar.",
  },
  "common.sidebarResize": {
    message: "サイドバーのサイズを変更",
    description: "Accessible name of the sidebar's draggable resize separator.",
  },
  "common.sidebarResizeHint": {
    message: "ドラッグでサイズ変更 · ダブルクリックで切り替え",
    description: "Hover tooltip on the sidebar's resize separator.",
  },
  "common.commandSuggestions": {
    message: "候補",
    description:
      "Accessible name of a search or picker results list (the command menu's listbox).",
  },
  "common.comboboxPlaceholder": {
    message: "オプションを選択",
    description:
      "Fallback placeholder of a searchable picker when its caller supplies none.",
  },
  "common.comboboxEmpty": {
    message: "結果が見つかりません",
    description:
      "Fallback text when a searchable picker's filter matches no option.",
  },
  "common.comboboxUseCustom": {
    message: "Enter キーで「{value}」を使用",
    description:
      "Hint under a picker's empty results: Enter keeps the typed value as entered. {value} is the typed text.",
  },
  "common.comboboxClear": {
    message: "入力をクリア",
    description: "Accessible name of a searchable picker's clear (X) button.",
  },
  "common.cancel": {
    message: "キャンセル",
    description:
      "Generic Cancel button — dismisses a dialog or an inline form without saving.",
  },
  "common.pricePerRunMinute": {
    message: "${price}/分（実行時間）",
    description:
      "A cron job's USD price per minute of run time, on a Change Instance Type card (w4/546): it is billed per second only while a run executes; price is an API-supplied decimal without the currency symbol",
  },
  "common.pricePerMinute": {
    message: "${price}/分",
    description:
      "A cron job's USD price per minute of run time on a create-form plan card, short to fit beside the plan name; the form's hint says it is billed only while a run executes (w4/546). price is an API-supplied decimal without the currency symbol",
  },
  "common.pricePerMonth": {
    message: "${price}/月",
    description:
      "Monthly USD price shown on a resource plan card; price is an API-supplied decimal without the currency symbol",
  },
  "common.planUnavailable": {
    message: "利用不可",
    description:
      "Shown in place of the price on a disabled plan card whose instance type is paused (it cannot be selected for now); the server's full reason is the card's tooltip",
  },
  "common.protectedConfirmationRetry": {
    message: "確認して保存",
    description:
      "Confirm button in the protected environment retry dialog raised by a settings save",
  },
  "common.protectedConfirmationCancel": {
    message: "キャンセル",
    description:
      "Cancel button in the protected environment confirmation dialog",
  },
  "common.navEnvGroups": {
    message: "環境グループ",
    description: "Workspace sidebar link to environment groups",
  },
  "common.appName": {
    message: "bex",
    description: "Product name shown in the dashboard chrome",
  },
  "common.headDescription": {
    message:
      "オープンソースで AI ネイティブな Render の代替、bex で、管理下のインフラにアプリケーションをデプロイして運用できます。",
    description:
      "Generic dashboard description used in description, Open Graph, and Twitter metadata",
  },
  "common.headImageAlt": {
    message: "bex ダッシュボードのロゴ",
    description: "Alternative text for the dashboard's generic social image",
  },
  "common.loading": {
    message: "読み込み中…",
    description: "Generic loading state label",
  },
  "common.topbarBreadcrumbs": {
    message: "パンくずリスト",
    description: "Accessible label for the dashboard topbar hierarchy",
  },
  "common.topbarSearch": {
    message: "検索",
    description: "Open the workspace-wide topbar search",
  },
  "common.copyCode": {
    message: "コードをコピー",
    description: "Accessible name of the copy button on a code block",
  },
  "common.topbarSearchPlaceholder": {
    message: "ページとリソースを検索…",
    description: "Placeholder in the workspace-wide topbar command search",
  },
  "common.topbarSearchDescription": {
    message: "ダッシュボードのページとワークスペースのリソースを検索します。",
    description: "Accessible description for the topbar command search",
  },
  "common.topbarSearchEmpty": {
    message: "一致するページやリソースはありません。",
    description: "Empty state in the workspace-wide topbar command search",
  },
  "common.copyField": {
    message: "{field} をコピー",
    description:
      "Accessible name for a copy control when the caller only has a field label (m101/t014)",
  },
  "common.topbarNavigation": {
    message: "ナビゲーション",
    description: "Page-links group in the workspace-wide topbar search",
  },
  "common.topbarResources": {
    message: "リソース",
    description: "Workspace-resources group in the topbar search",
  },
  "common.topbarNew": {
    message: "新規",
    description: "Persistent topbar menu for creating a resource",
  },
  "common.topbarCreate": {
    message: "リソースを作成",
    description: "Heading in the persistent topbar create menu",
  },
  "common.topbarHelp": {
    message: "ヘルプとリソース",
    description: "Accessible label for the topbar help menu",
  },
  "common.topbarDocumentation": {
    message: "ドキュメント",
    description: "Topbar help-menu link to bex documentation",
  },
  "common.topbarCliGuide": {
    message: "CLI ガイド",
    description: "Topbar help-menu link to the bex CLI guide",
  },
  "common.topbarRepository": {
    message: "GitHub リポジトリ",
    description: "Topbar help-menu link to the bex source repository",
  },
  "common.topbarWorkspaceSettings": {
    message: "ワークスペース設定",
    description: "Workspace-settings label in topbar navigation and search",
  },
  "common.topbarSwitchProject": {
    message: "プロジェクトを切り替え",
    description: "Heading in a project breadcrumb dropdown",
  },
  "common.topbarSwitchEnvironment": {
    message: "環境を切り替え",
    description: "Heading in an environment breadcrumb dropdown",
  },
  "common.topbarSwitchService": {
    message: "サービスを切り替え",
    description: "Heading in a service breadcrumb dropdown",
  },
  "common.topbarAllResources": {
    message: "すべてのリソース",
    description: "Service breadcrumb menu link back to workspace resources",
  },
  "common.topbarProjectResource": {
    message: "プロジェクト",
    description: "Project kind label in workspace-wide search results",
  },
  "common.topbarServiceResource": {
    message: "サービス",
    description: "Service kind label in workspace-wide search results",
  },
  "common.sudoCommandLabel": {
    message: "Sudo コマンド",
    description:
      "Input label on Render-style destructive type-to-confirm gates (live capture: docs/render-artifacts/workspace-lifecycle.md)",
  },
  "common.navIntegrationsGroup": {
    message: "連携",
    description:
      "Sidebar nav section label (webhooks, notifications) — Render's grouping",
  },
  "common.navWorkspaceGroup": {
    message: "ワークスペース",
    description:
      "Sidebar nav section label (usage, settings) — Render's grouping",
  },
  "common.navMonitorGroup": {
    message: "モニタリング",
    description:
      "Service sidebar section label (logs, metrics) — Render's grouping",
  },
  "common.navWebhooks": {
    message: "Webhook",
    description: "Sidebar nav link to the outbound event webhooks page",
  },
  "common.labelInvalid": {
    message: "100 文字以内で、改行や制御文字を含まない名前を入力してください。",
    description:
      "Inline validation under a free-form name field (project, environment, group, webhook, credential, Blueprint)",
  },
  "common.navNotifications": {
    message: "通知",
    description: "Sidebar nav link to the notification settings page",
  },
  "common.navProjects": {
    message: "プロジェクト",
    description:
      "Sidebar nav link to the unified projects page (services, databases, key value grouped together)",
  },
  "common.navUsage": {
    message: "請求",
    description:
      "Sidebar nav link to the workspace billing page (/billing, renamed from Usage in w5/m70; key kept to avoid churn)",
  },
  "common.navBlueprints": {
    message: "Blueprints",
    description:
      "Sidebar nav link to the blueprints management page (IaC stacks auto-registered on deploy)",
  },
  "common.navAgents": {
    message: "エージェント",
    description:
      "Sidebar nav link to the cloud coding-agent sessions page (/agents)",
  },
  "common.navAutomations": {
    message: "オートメーション",
    description:
      "Sidebar nav link to the scheduled agent automations page (/automations)",
  },
  "common.navSettings": {
    message: "設定",
    description:
      "Sidebar nav link to the workspace settings page (the sidebar is workspace-scoped; account settings hang off the user menu)",
  },
  "common.navBackToDashboard": {
    message: "ダッシュボード",
    description:
      "Contextual project sidebar's back link label, shown on a project's Overview page (links up to the workspace Overview)",
  },
  "common.navBackToProject": {
    message: "プロジェクト",
    description:
      "Contextual project sidebar's back link label, shown on a project's Settings page (links up to the project's Overview)",
  },
  "common.navProjectOverview": {
    message: "概要",
    description:
      "Contextual project sidebar nav link to the project's own Overview page",
  },
  "common.navManageGroup": {
    message: "管理",
    description: "Contextual project sidebar section label above Settings",
  },
  "common.changeLanguage": {
    message: "言語を変更",
    description: "Accessible label for the language switcher button",
  },
  "common.userMenuSettings": {
    message: "アカウント設定",
    description:
      "User menu item that navigates to account settings (/settings)",
  },
  "common.userMenuTheme": {
    message: "テーマ",
    description: "User menu submenu label for theme selection",
  },
  "common.userMenuLanguage": {
    message: "言語",
    description: "User menu submenu label for language selection",
  },
  "common.userMenuThemeLight": {
    message: "ライト",
    description: "Light theme option label",
  },
  "common.userMenuThemeDark": {
    message: "ダーク",
    description: "Dark theme option label",
  },
  "common.userMenuThemeSystem": {
    message: "システム",
    description: "System theme option label",
  },
  "common.userMenuLogOut": {
    message: "ログアウト",
    description: "User menu item that signs the user out",
  },
  "common.userMenuOpen": {
    message: "アカウントメニュー",
    description:
      "Accessible name of the avatar button that opens the account menu (w4/407)",
  },
  "common.userMenuOpenFor": {
    message: "{name} のアカウントメニュー",
    description:
      "Accessible name of the avatar button that opens the account menu, naming the signed-in identity (w4/407)",
  },
  "common.goHome": {
    message: "ホームへ戻る",
    description: "Button label that navigates back to the home page",
  },
  "common.goBack": {
    message: "戻る",
    description: "Button label that navigates to the previous page",
  },
  "common.planCatalogUnavailable": {
    message:
      "プランを読み込めなかったため、まだ選択できません。入力済みの内容は保持されています。",
    description:
      "A plan picker whose catalog read failed or returned no plans; shown with a Try again button that re-reads only the catalog (w4/m218)",
  },
  "common.tryAgain": {
    message: "再試行",
    description: "Button label that retries after an error",
  },
  "common.notFoundTitle": {
    message: "ページが見つかりません",
    description: "404 page heading",
  },
  "common.notFoundDescription": {
    message: "お探しのページは存在しないか、移動されました。",
    description: "404 page explanatory text",
  },
  "common.errorTitle": {
    message: "問題が発生しました",
    description: "Generic error page heading",
  },
  "common.errorDefaultMessage": {
    message: "問題が発生しました。",
    description: "Fallback error message when none is provided",
  },
  "common.loadErrorRetry": {
    message:
      "読み込み中に問題が発生しました。ページを更新して、もう一度お試しください。",
    description:
      "Body of a panel whose read failed with no server reason to show (a network failure or throttling)",
  },
  "common.resourceNotFoundToast": {
    message: "そのリソースは存在しないか、削除されました。",
    description:
      "Toast shown after a dead resource URL redirects to the home page (w9/m55)",
  },
  "common.resourceErrorBody": {
    message:
      "リソースを読み込めませんでした。API を確認して、もう一度お試しください。",
    description:
      "Body text of a detail page's inline error state when the resource query failed",
  },
  "common.sessionExpiredTitle": {
    message: "セッションの有効期限が切れました",
    description:
      "Heading of the inline state shown when a resource query fails with a 401 (expired session)",
  },
  "common.sessionExpiredBody": {
    message: "もう一度ログインすると、このページにそのまま戻ります。",
    description:
      "Body text of the expired-session inline state, reassuring the user their place is preserved",
  },
  "common.signIn": {
    message: "ログイン",
    description:
      "Button label that sends an expired session back to the login page",
  },
  "common.colActions": {
    message: "操作",
    description:
      "Screen-reader-only header for a table's trailing row-actions column",
  },
  "common.justNow": {
    message: "たった今",
    description:
      'Elapsed-time text for an instant under a minute old ("Deployed just now")',
  },
  "common.timeLocal": {
    message: "ローカル",
    description:
      "Timestamp tooltip: label of the row showing the instant in the viewer's local timezone",
  },
  "common.timeUtc": {
    message: "UTC",
    description:
      "Timestamp tooltip: label of the row showing the instant in UTC",
  },
  "common.timeUnix": {
    message: "タイムスタンプ",
    description:
      "Timestamp tooltip: label of the row showing the instant as Unix seconds",
  },
  "common.ipAllowListProxiedTitle_other": {
    message:
      "許可リストでは {count} 件のカスタムドメインのクライアントを判別できません",
    description:
      "Title of the warning shown under an inbound IP allowlist when several custom domains it covers are proxied by Cloudflare (w1/m171)",
  },
  "common.ipAllowListProxiedBody_other": {
    message:
      "これらのドメインは Cloudflare を経由しているため、許可リストは訪問者ではなく Cloudflare のアドレスと照合されます。リストは保存されますが、これらのドメインでは許可するつもりの訪問者もブロックされます。",
    description:
      "Body of the Cloudflare-proxied allowlist warning, followed by the list of proxied hostnames (w1/m171)",
  },
  "common.ipAllowListProxiedFix": {
    message:
      "実際のクライアントアドレスでフィルタリングするには、Cloudflare で DNS レコードを DNS のみ（グレーの雲）に設定してください。",
    description:
      "How to fix the Cloudflare-proxied allowlist warning: switch the record from proxied (orange cloud) to DNS only (w1/m171)",
  },
  "common.relativeAgeNow": {
    message: "今",
    description: "Compact relative age under one minute (w4/m188)",
  },
  "common.relativeAgeMinutes": {
    message: "{n} 分",
    description: "Compact relative age in whole minutes, e.g. 4m (w4/m188)",
  },
  "common.relativeAgeHours": {
    message: "{n} 時間",
    description: "Compact relative age in whole hours, e.g. 3h (w4/m188)",
  },
  "common.relativeAgeDays": {
    message: "{n} 日",
    description: "Compact relative age in whole days, e.g. 5d (w4/m188)",
  },
  "common.relativeAgeMonths": {
    message: "{n} か月",
    description: "Compact relative age in 30-day months, e.g. 2mo (w4/m188)",
  },
  "common.relativeAgeYears": {
    message: "{n} 年",
    description: "Compact relative age in 365-day years, e.g. 1y (w4/m188)",
  },
  "common.relativeUntilNow": {
    message: "今",
    description:
      "Countdown to a future instant less than one minute away (w4/m188)",
  },
  "common.relativeUntilMinutes": {
    message: "{n} 分後",
    description: "Compact countdown in whole minutes, e.g. in 5m (w4/m188)",
  },
  "common.relativeUntilHours": {
    message: "{n} 時間後",
    description: "Compact countdown in whole hours, e.g. in 3h (w4/m188)",
  },
  "common.relativeUntilDays": {
    message: "{n} 日後",
    description: "Compact countdown in whole days, e.g. in 83d (w4/m188)",
  },
  "common.openBilling": {
    message: "請求を開く",
    description:
      "Link from a plan notice (a Pro-and-higher feature on Hobby) to the Billing page (w8/m138)",
  },
  "common.billingSettlingTitle": {
    message: "最終使用量を確定しています…",
    description:
      "Title while a deletion waits for the workspace's final usage to be charged (BILLING_SETTLING); retried automatically",
  },
  "common.billingSettlingDescription": {
    message:
      "bex は削除の前に最終使用量を請求しています。1～2 分かかることがあります。自動的に再試行されるので、このページを開いたままにしてください。",
    description:
      "Body while a deletion waits for the final usage to be charged (BILLING_SETTLING)",
  },
  "common.billingSettlingTimeout": {
    message:
      "最終使用量はまだ処理中です。数分後にもう一度削除をお試しください。",
    description:
      "Deletion error after BILLING_SETTLING retries ran out (about three minutes)",
  },
  "common.balanceDueTitle": {
    message: "未払い残高",
    description:
      "Title of the alert when a deletion is refused because the workspace has an unpaid balance (BILLING_BALANCE_DUE)",
  },
  "common.balanceDueAmount": {
    message:
      "このワークスペースには {amount} の未払い残高があります。請求書をお支払いのうえ、もう一度削除してください。",
    description: "Balance-due deletion refusal with the amount",
  },
  "common.balanceDue": {
    message:
      "このワークスペースには未払い残高があります。請求書をお支払いのうえ、もう一度削除してください。",
    description: "Balance-due deletion refusal without an amount",
  },
  "common.balanceDueNamedAmount": {
    message:
      "ワークスペース {name} には {amount} の未払い残高があります。請求書をお支払いのうえ、もう一度アカウントを削除してください。",
    description:
      "Account-deletion refusal naming the workspace with the unpaid balance and its amount",
  },
  "common.balanceDueNamed": {
    message:
      "ワークスペース {name} には未払い残高があります。請求書をお支払いのうえ、もう一度アカウントを削除してください。",
    description:
      "Account-deletion refusal naming the workspace with the unpaid balance",
  },
  "common.balanceDuePay": {
    message: "請求書を支払う",
    description: "Link to Stripe's hosted invoice page, opens in a new tab",
  },
  "common.regionFrankfurt": {
    message: "フランクフルト（ドイツ）",
    description:
      "Display name of Render's `frankfurt` region, shown where a resource's region is read (w4/693)",
  },
  "common.regionOhio": {
    message: "オハイオ（米国）",
    description:
      "Display name of Render's `ohio` region, shown where a resource's region is read (w4/693)",
  },
  "common.regionOregon": {
    message: "オレゴン（米国）",
    description:
      "Display name of Render's `oregon` region, shown where a resource's region is read (w4/693)",
  },
  "common.regionSingapore": {
    message: "シンガポール",
    description:
      "Display name of Render's `singapore` region, shown where a resource's region is read (w4/693)",
  },
  "common.regionVirginia": {
    message: "バージニア（米国）",
    description:
      "Display name of Render's `virginia` region, shown where a resource's region is read (w4/693)",
  },
};

export default jaCommon;
