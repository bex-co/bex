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
  "auth.settingsTitle": {
    message: "设置",
    description: "Account settings page heading",
  },
  "auth.settingsSubtitle": {
    message: "管理您的账户资料和密码。",
    description: "Account settings page subheading",
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
};

export default zhAuth;
