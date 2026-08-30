import type { TranslationEntry } from "@/i18n";

const zhAuth: Record<string, TranslationEntry> = {
  "auth.loginTitle": {
    message: "欢迎回来",
    description: "Login page hero title",
  },
  "auth.loginSubtitle": {
    message: "登录您的账户",
    description: "Login page hero subtitle",
  },
  "auth.registerTitle": {
    message: "创建您的账户",
    description: "Registration page hero title",
  },
  "auth.registerSubtitle": {
    message: "输入您的信息以开始使用",
    description: "Registration page hero subtitle",
  },
  "auth.forgotPasswordTitle": {
    message: "重置您的密码",
    description: "Forgot-password page hero title",
  },
  "auth.forgotPasswordSubtitle": {
    message: "输入账户邮箱以获取一次性访问代码",
    description: "Forgot-password page hero subtitle",
  },
  "auth.resetPasswordTitle": {
    message: "设置新密码",
    description:
      "Document title for the post-recovery reset-password landing page (renders the settings flow's password field)",
  },
  "auth.verificationTitle": {
    message: "验证您的邮箱",
    description: "Verification page hero title",
  },
  "auth.verificationSubtitle": {
    message: "输入与您账户关联的邮箱地址以继续",
    description:
      "Verification page hero subtitle (email-first Ory step; code arrives after)",
  },
  "auth.settingsTitle": {
    message: "设置",
    description: "Account settings page heading",
  },
  "auth.settingsSubtitle": {
    message: "管理您的身份、集成、访问权限和安全设置。",
    description: "Account settings page subheading",
  },
  "auth.settingsNavigation": {
    message: "设置分区",
    description: "Accessible label for the settings page section navigation",
  },
  "auth.accountSection": {
    message: "账户",
    description: "Settings page account section heading and navigation label",
  },
  "auth.accountSectionSubtitle": {
    message: "更新您的资料、密码和双重身份验证设置。",
    description: "Settings page account section description",
  },
  "auth.integrationsSection": {
    message: "集成",
    description:
      "Settings page integrations section heading and navigation label",
  },
  "auth.integrationsSectionSubtitle": {
    message: "连接源代码管理和私有镜像仓库。",
    description: "Settings page integrations section description",
  },
  "auth.accessSection": {
    message: "访问凭据",
    description: "Settings page access section heading and navigation label",
  },
  "auth.accessSectionSubtitle": {
    message: "管理编程访问和 SSH 身份。",
    description: "Settings page access section description",
  },
  "auth.securityComplianceSection": {
    message: "安全与合规",
    description:
      "Settings page section heading grouping security and audit cards",
  },
  "auth.securityComplianceSectionSubtitle": {
    message: "账户安全控制以及您工作区的审计记录。",
    description: "Settings page Security & Compliance section description",
  },
  "auth.dangerZoneSection": {
    message: "危险操作",
    description: "Account settings destructive-actions section",
  },
  "auth.dangerZoneSectionSubtitle": {
    message: "永久删除您的账户并撤销所有访问权限。",
    description: "Account deletion section description",
  },
  "auth.deleteAccountTitle": {
    message: "删除账户",
    description: "Account deletion card title",
  },
  "auth.deleteAccountDescription": {
    message:
      "此操作会永久删除您的身份、凭据和仅有您一名成员的工作区。共享工作区仍可由其他管理员使用。",
    description: "Account deletion consequences",
  },
  "auth.deleteAccountWillDelete": {
    message: "以下工作区将被删除",
    description: "Account deletion preview heading for sole-member workspaces",
  },
  "auth.deleteAccountWillLeave": {
    message: "您将离开以下工作区",
    description: "Account deletion preview heading for shared workspaces",
  },
  "auth.deleteAccountBlockedTitle": {
    message: "请先解决工作区管理权",
    description: "Account deletion blocker heading",
  },
  "auth.deleteAccountBlockedDescription": {
    message:
      "以下工作区有其他成员，但没有其他管理员。请先将其他成员提升为管理员、移除其他成员，或删除该工作区。",
    description: "Actionable account deletion blocker guidance",
  },
  "auth.deleteAccountBlockedAction": {
    message: "打开设置",
    description: "Link to resolve an account deletion workspace blocker",
  },
  "auth.deleteAccountConfirmLabel": {
    message: "请在下方输入 {phrase} 以确认。",
    description: "Exact account deletion confirmation prompt",
  },
  "auth.deleteAccountSubmit": {
    message: "删除我的账户",
    description: "Account deletion submit button",
  },
  "auth.deleteAccountErrorTitle": {
    message: "无法开始删除账户",
    description: "Account deletion mutation error heading",
  },
  "auth.deleteAccountError": {
    message: "无法开始删除账户，请重试。",
    description: "Generic account deletion mutation error",
  },
  "auth.deleteAccountPreviewErrorTitle": {
    message: "无法加载删除预览",
    description: "Account deletion preview error heading",
  },
  "auth.deleteAccountPreviewError": {
    message: "无法安全确定工作区的处理方式。尚未删除任何内容。",
    description: "Account deletion preview failure explanation",
  },
  "auth.deleteAccountRetry": {
    message: "重试",
    description: "Account deletion preview retry button",
  },
  "auth.accountDeletedTitle": {
    message: "账户删除已开始",
    description: "Accepted account deletion terminal page title",
  },
  "auth.accountDeletedSubtitle": {
    message: "您已安全退出登录。",
    description: "Accepted account deletion terminal page subtitle",
  },
  "auth.accountDeletedStatus": {
    message: "正在删除您的工作区和凭据。您可以关闭此页面，无需执行其他操作。",
    description: "Accepted account deletion background completion status",
  },
  "auth.accountDeletedHome": {
    message: "返回 bex",
    description: "Accepted account deletion terminal page home link",
  },
  "auth.loggingOutTitle": {
    message: "正在退出登录…",
    description: "Logout page heading while the logout request is in flight",
  },
  "auth.loggingOutSubtitle": {
    message: "正在结束您的会话，请稍候。",
    description: "Logout page subtext while the logout request is in flight",
  },
  "auth.loggedOutTitle": {
    message: "已退出登录",
    description: "Logout page heading once logout has completed",
  },
  "auth.loggedOutSubtitle": {
    message: "正在跳转到登录页…",
    description: "Logout page subtext once logout has completed",
  },
  "auth.logoutFailedTitle": {
    message: "退出登录失败",
    description: "Logout page heading when the provider logout request failed",
  },
  "auth.logoutFailedSubtitle": {
    message: "无法结束您的会话，您可能仍处于登录状态，请重试。",
    description: "Logout page subtext when the provider logout request failed",
  },
  "auth.logoutRetry": {
    message: "重试",
    description: "Logout page retry button after a failed sign-out",
  },
  "auth.logoutConfirmTitle": {
    message: "退出登录？",
    description:
      "Logout page heading on the confirmation screen (codex #12 CSRF logout fix)",
  },
  "auth.logoutConfirmSubtitle": {
    message: "确定要结束当前会话吗？",
    description: "Logout page subtext on the confirmation screen",
  },
  "auth.logoutConfirm": {
    message: "退出登录",
    description: "Logout page confirm button that triggers the actual logout",
  },
  "auth.logoutCancel": {
    message: "取消",
    description: "Logout page cancel button that returns to the dashboard",
  },
  "auth.logoutTitle": {
    message: "退出登录",
    description: "Logout page document title",
  },
  "auth.featureSecureTitle": {
    message: "默认安全",
    description: "Auth hero feature bullet title",
  },
  "auth.featureSecureDescription": {
    message:
      "会话由 Ory Kratos 管理——这是经过实战检验的身份基础设施，而非自研的认证系统。",
    description: "Auth hero feature bullet description",
  },
  "auth.featureDashboardTitle": {
    message: "一个仪表盘管理所有服务",
    description: "Auth hero feature bullet title",
  },
  "auth.featureDashboardDescription": {
    message: "在一个地方部署、监控和管理运行在 bex 上的所有内容。",
    description: "Auth hero feature bullet description",
  },
  "auth.featureOpenSourceTitle": {
    message: "开放构建",
    description: "Auth hero feature bullet title",
  },
  "auth.featureOpenSourceDescription": {
    message: "bex 是开源的 Render 替代方案。",
    description: "Auth hero feature bullet description",
  },
  "auth.consentTitle": {
    message: "授权访问",
    description: "OAuth2 consent page hero title",
  },
  "auth.consentSubtitle": {
    message: "{client} 请求代表你在 bex 中执行操作。",
    description: "OAuth2 consent page hero subtitle, names the OAuth client",
  },
  "auth.consentUnverifiedClient": {
    message: "未经验证的第三方应用",
    description:
      "Warning label for an OAuth client whose self-provided branding is not trusted",
  },
  "auth.consentClientId": {
    message: "客户端 ID",
    description: "Label for the immutable OAuth client identifier",
  },
  "auth.consentRedirectOrigin": {
    message: "授权码接收地址",
    description: "Label for the origin that will receive the OAuth code",
  },
  "auth.consentNoRedirectOrigin": {
    message: "无重定向（设备流程）",
    description: "Shown when an OAuth device flow has no redirect origin",
  },
  "auth.consentScopesTitle": {
    message: "它请求的权限",
    description: "Heading above the list of requested OAuth scopes",
  },
  "auth.consentScopeOpenid": {
    message: "确认你的身份",
    description: "Human description of the openid scope",
  },
  "auth.consentScopeOfflineAccess": {
    message: "保持连接而无需再次询问（刷新令牌）",
    description: "Human description of the offline_access scope",
  },
  "auth.consentScopeProfile": {
    message: "读取你的基本资料",
    description: "Human description of the profile scope",
  },
  "auth.consentScopeEmail": {
    message: "读取你的邮箱地址",
    description: "Human description of the email scope",
  },
  "auth.consentScopeRead": {
    message: "读取普通工作区资源（服务、日志、指标）",
    description: "Human description of the bex.read OAuth capability",
  },
  "auth.consentScopeWrite": {
    message: "更改工作区资源（部署、重启、创建、删除、账单）",
    description: "Human description of the bex.write OAuth capability",
  },
  "auth.consentScopeSensitive": {
    message: "读取密钥与连接字符串（环境变量、数据库 URL、文件）",
    description: "Human description of the bex.sensitive OAuth capability",
  },
  "auth.consentScopeApiCompat": {
    message: "完整控制面访问（平台客户端兼容别名）",
    description: "Human description of the bex.api compatibility scope",
  },
  "auth.consentAudienceTitle": {
    message: "令牌的适用范围",
    description: "Heading above the requested access-token audience",
  },
  "auth.consentRememberHint": {
    message: "同意后将记住一小时——此后该 agent 会再次询问。",
    description: "Consent page footnote about the remember window",
  },
  "auth.consentApprove": {
    message: "同意",
    description: "OAuth2 consent page approve button",
  },
  "auth.consentDeny": {
    message: "拒绝",
    description: "OAuth2 consent page deny button",
  },
  "auth.consentFailed": {
    message: "操作未能完成，请重试。",
    description: "Consent page error banner after a failed accept/reject",
  },
  "auth.consentExpiredTitle": {
    message: "授权已失效",
    description: "Consent page heading when there is no live consent request",
  },
  "auth.consentExpiredSubtitle": {
    message: "请从引导你到这里的应用重新发起连接。",
    description:
      "Consent page subtext when there is no live consent request to decide",
  },
  "auth.consentInvalidRequestTitle": {
    message: "授权请求无效",
    description:
      "Consent page heading when the requesting client's authorization request fails a protocol check (PKCE or scope)",
  },
  "auth.consentPkceRequiredSubtitle": {
    message: "此应用的请求缺少必要的安全参数。请联系该应用的开发者。",
    description:
      "Consent page subtext when the authorization request lacks PKCE with S256",
  },
  "auth.consentScopeRequiredSubtitle": {
    message:
      "此应用请求了 API 访问权限，但未指定所需的具体权限。它必须请求以下至少一项：{scopes}。",
    description:
      "Consent page subtext when an audience request lacks a granular capability scope (round-14 #1); {scopes} is the comma-separated list of accepted capability scopes",
  },
  "auth.consentUnavailableTitle": {
    message: "授权暂不可用",
    description:
      "Consent page heading when the authorization provider is unreachable, misconfigured, or a headless accept failed",
  },
  "auth.consentUnavailableSubtitle": {
    message: "这通常很快就能恢复——请稍后重试。",
    description:
      "Consent page subtext when the authorization provider is unreachable, misconfigured, or a headless accept failed",
  },
  "auth.consentWrongUserTitle": {
    message: "登录账户不匹配",
    description:
      "Consent page heading when the browser's session belongs to a different account than the one that started this authorization",
  },
  "auth.consentWrongUserSubtitle": {
    message: "此浏览器的会话所属账户与发起此授权的账户不同。请退出登录后重试。",
    description:
      "Consent page subtext when the browser's session belongs to a different account than the one that started this authorization",
  },
  "auth.deviceSuccessTitle": {
    message: "bex CLI 已连接",
    description: "Device authorization success page title",
  },
  "auth.deviceTitle": {
    message: "连接 bex CLI",
    description: "bex CLI device verification route document title",
  },
  "auth.deviceConfirmSubtitle": {
    message: "一台通过 bex CLI 登录的设备正在请求访问你的账户。",
    description: "Device confirm page hero subtitle",
  },
  "auth.deviceConfirmHeading": {
    message: "授权此设备",
    description: "Device confirm page card heading",
  },
  "auth.deviceConfirmDescription": {
    message: "请确认下方代码与你的 CLI 显示的一致：",
    description: "Device confirm page card description above the code",
  },
  "auth.deviceConfirmHint": {
    message: "仅当你本人在终端发起此请求时才应授权。",
    description:
      "Device confirm page footnote warning against unsolicited requests",
  },
  "auth.deviceConfirmCancel": {
    message: "取消",
    description: "Device confirm page cancel button",
  },
  "auth.deviceConfirmAuthorize": {
    message: "授权设备",
    description: "Device confirm page authorize/submit button",
  },
  "auth.deviceExpiredTitle": {
    message: "设备代码已过期",
    description:
      "Device confirm page heading when there is no live device request",
  },
  "auth.deviceExpiredSubtitle": {
    message:
      "此页面需要来自 `bex login` 的设备代码，或该代码已过期。请在终端中重新运行该命令。",
    description:
      "Device confirm page subtext when there is no live device request to confirm",
  },
  "auth.deviceUnavailableTitle": {
    message: "设备授权暂不可用",
    description:
      "Device confirm page heading when the authorization provider is unreachable or misconfigured",
  },
  "auth.deviceUnavailableSubtitle": {
    message: "这通常很快就能恢复——请稍后重试。",
    description:
      "Device confirm page subtext when the authorization provider is unreachable or misconfigured",
  },
  "auth.deviceRefusedTitle": {
    message: "授权被拒绝",
    description:
      "Device confirm page heading when the device request names a client bex did not expect",
  },
  "auth.deviceRefusedSubtitle": {
    message: "此设备请求无法验证。请在终端中重新运行 `bex login`。",
    description:
      "Device confirm page subtext when the device request names a client bex did not expect",
  },
  "auth.deviceSuccessSubtitle": {
    message: "浏览器授权已完成。",
    description: "Device authorization success page subtitle",
  },
  "auth.deviceSuccessHint": {
    message: "请返回终端继续。",
    description: "Device authorization success page terminal hint",
  },
  "auth.deviceSuccessWaiting": {
    message: "正在等待浏览器授权…",
    description:
      "Device success page terminal replica: the CLI line shown while it polls for tokens",
  },
  "auth.deviceSuccessDone": {
    message: "授权完成，你已登录。",
    description:
      "Device success page terminal replica: the CLI success line (rendered after a ✓ glyph)",
  },
  "auth.deviceSuccessClose": {
    message: "你可以关闭此标签页。",
    description:
      "Device success page: follow-up sentence after the return-to-terminal hint",
  },
  "auth.deviceSuccessDashboard": {
    message: "打开仪表盘",
    description:
      "Device success page: quiet link to the dashboard home for users staying in the browser",
  },
};

export default zhAuth;
