import type { OrySettingsCopy } from "./keys";

// Ory Elements' own `en` catalog entries, copied verbatim (see ./keys.ts).
const en: OrySettingsCopy = {
  "orySettings.profileTitle": "Profile Settings",
  "orySettings.profileDescription": "Update your profile information",
  "orySettings.passwordTitle": "Change Password",
  "orySettings.passwordDescription": "Modify your password",
  "orySettings.passwordLabel": "Password",
  "orySettings.oidcTitle": "Connected accounts",
  "orySettings.oidcDescription":
    "Connect a social login provider with your account.",
  "orySettings.oidcInfo":
    "Connected accounts from these providers can be used to login to your account",
  "orySettings.lookupSecretTitle": "Backup Recovery Codes (second factor)",
  "orySettings.lookupSecretDescription":
    "Recovery codes are a secure backup for 2FA, allowing you to regain access to your account if you lose your 2FA device.",
  "orySettings.lookupSecretEnable": "Enable",
  "orySettings.webauthnTitle": "Manage Hardware Tokens",
  "orySettings.webauthnDescription": "Manage your hardware token settings",
  "orySettings.webauthnNameLabel": "Name of the security key",
  "orySettings.webauthnAdd": "Add security key",
  "orySettings.webauthnInfo":
    "Hardware Tokens are used for second-factor authentication or as first-factor with Passkeys",
  "orySettings.totpTitle": "Authenticator App",
  "orySettings.totpDescription":
    "Add a TOTP Authenticator App to your account to improve your account security. Popular Authenticator Apps are LastPass and Google Authenticator",
  "orySettings.totpSecretLabel": "Authenticator Secret",
  "orySettings.totpCodeLabel": "Verify code",
  "orySettings.totpInfoNotLinked":
    "To enable scan the QR code with your authenticator and enter the code.",
  "orySettings.save": "Save",
};

export default en;
