import type { TranslationEntry } from "@/i18n";

const zhCommon: Record<string, TranslationEntry> = {
  "common.appName": {
    message: "bex",
    description: "Product name shown in the dashboard chrome",
  },
  "common.loading": {
    message: "加载中…",
    description: "Generic loading state label",
  },
  "common.navDashboardGroup": {
    message: "仪表盘",
    description: "Sidebar nav section label",
  },
  "common.navServices": {
    message: "服务",
    description: "Sidebar nav link to the services list page",
  },
  "common.navDatabases": {
    message: "数据库",
    description: "Sidebar nav link to the databases list page",
  },
  "common.navKeyValue": {
    message: "键值存储",
    description: "Sidebar nav link to the Key Value list page",
  },
  "common.navSettings": {
    message: "设置",
    description: "Sidebar nav link to the account settings page",
  },
  "common.changeLanguage": {
    message: "切换语言",
    description: "Accessible label for the language switcher button",
  },
  "common.userMenuSettings": {
    message: "设置",
    description: "User menu item that navigates to account settings",
  },
  "common.userMenuTheme": {
    message: "主题",
    description: "User menu submenu label for theme selection",
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
};

export default zhCommon;
