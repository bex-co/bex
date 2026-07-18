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
    message: "输入您的邮箱以接收恢复代码",
    description: "Forgot-password page hero subtitle",
  },
  "auth.verificationTitle": {
    message: "验证您的邮箱",
    description: "Verification page hero title",
  },
  "auth.verificationSubtitle": {
    message: "输入我们发送的验证码以确认您的地址",
    description: "Verification page hero subtitle",
  },
  "auth.settingsTitle": {
    message: "设置",
    description: "Account settings page heading",
  },
  "auth.settingsSubtitle": {
    message: "管理您的账户资料、密码和双重身份验证安全设置。",
    description: "Account settings page subheading",
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
  "auth.deviceSuccessTitle": {
    message: "Render CLI 已连接",
    description: "Device authorization success page title",
  },
  "auth.deviceTitle": {
    message: "连接 Render CLI",
    description: "Render CLI device verification route document title",
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
