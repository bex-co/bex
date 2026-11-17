import type { OrySettingsCopy } from "./keys";

// Ory Elements' own `ja` catalog entries, copied verbatim (see ./keys.ts).
const ja: OrySettingsCopy = {
  "orySettings.profileTitle": "プロフィール設定",
  "orySettings.profileDescription": "プロフィール情報を更新",
  "orySettings.passwordTitle": "パスワードを変更",
  "orySettings.passwordDescription": "パスワードを変更",
  "orySettings.passwordLabel": "パスワード",
  "orySettings.oidcTitle": "接続されたアカウント",
  "orySettings.oidcDescription":
    "ソーシャルログインプロバイダーをアカウントに接続",
  "orySettings.oidcInfo":
    "これらのプロバイダーからの接続されたアカウントは、アカウントへのログインに使用できます。",
  "orySettings.lookupSecretTitle": "バックアップ復旧コード（2段階認証）",
  "orySettings.lookupSecretDescription":
    "復旧コードは2段階認証の安全なバックアップであり、2段階認証デバイスを紛失した場合でもアカウントへのアクセスを回復できるようにします。",
  "orySettings.lookupSecretEnable": "有効化",
  "orySettings.webauthnTitle": "ハードウェアトークンを管理",
  "orySettings.webauthnDescription": "ハードウェアトークンの設定を管理",
  "orySettings.webauthnNameLabel": "セキュリティキーの名前",
  "orySettings.webauthnAdd": "セキュリティキーを追加",
  "orySettings.webauthnInfo":
    "ハードウェアトークンは2段階認証に使用されるか、パスキーと併用して1段階認証に使用されます。",
  "orySettings.totpTitle": "認証アプリ",
  "orySettings.totpDescription":
    "アカウントにTOTP認証アプリを追加して、アカウントのセキュリティを強化できます。人気のある認証アプリにはLastPassやGoogle Authenticatorがあります。",
  "orySettings.totpSecretLabel": "認証用シークレット",
  "orySettings.totpCodeLabel": "コードを確認",
  "orySettings.totpInfoNotLinked":
    "有効にするには、認証アプリでQRコードをスキャンし、コードを入力してください。",
  "orySettings.save": "保存",
};

export default ja;
