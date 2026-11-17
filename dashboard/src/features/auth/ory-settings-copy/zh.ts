import type { OrySettingsCopy } from "./keys";

// Ory Elements' own `zh` catalog entries, copied verbatim (see ./keys.ts).
const zh: OrySettingsCopy = {
  "orySettings.profileTitle": "个人资料设置",
  "orySettings.profileDescription": "更新您的个人资料信息",
  "orySettings.passwordTitle": "更改密码",
  "orySettings.passwordDescription": "修改您的密码",
  "orySettings.passwordLabel": "密码",
  "orySettings.oidcTitle": "关联的账户",
  "orySettings.oidcDescription": "将社交登录提供商与您的账户连接起来。",
  "orySettings.oidcInfo": "这些提供者的关联账户可用于登录您的账户",
  "orySettings.lookupSecretTitle": "备用恢复码（第二因素）",
  "orySettings.lookupSecretDescription":
    "恢复码是2FA的安全备份，允许您在丢失2FA设备时重新获得账户访问权限。",
  "orySettings.lookupSecretEnable": "启用",
  "orySettings.webauthnTitle": "管理硬件令牌",
  "orySettings.webauthnDescription": "管理您的硬件令牌设置",
  "orySettings.webauthnNameLabel": "安全密钥名称",
  "orySettings.webauthnAdd": "添加安全密钥",
  "orySettings.webauthnInfo":
    "硬件令牌用于第二因素身份验证或作为带有密钥的第一因素",
  "orySettings.totpTitle": "身份验证器应用",
  "orySettings.totpDescription":
    "向您的账户添加一个 TOTP 身份验证器应用以提高账户安全性。流行的认证应用包括 LastPass 和 Google Authenticator。",
  "orySettings.totpSecretLabel": "身份验证器密钥",
  "orySettings.totpCodeLabel": "验证代码",
  "orySettings.totpInfoNotLinked":
    "要启用，请使用您的身份验证器扫描二维码并输入代码。",
  "orySettings.save": "保存",
};

export default zh;
