import type { OrySettingsCopy } from "./keys";

// Ory Elements' own `ru` catalog entries, copied verbatim (see ./keys.ts).
const ru: OrySettingsCopy = {
  "orySettings.profileTitle": "Настройки профиля",
  "orySettings.profileDescription": "Обновите информацию о вашем профиле",
  "orySettings.passwordTitle": "Сменить пароль",
  "orySettings.passwordDescription": "Измените свой пароль",
  "orySettings.passwordLabel": "Пароль",
  "orySettings.oidcTitle": "Подключенные аккаунты",
  "orySettings.oidcDescription":
    "Подключите провайдера социального входа к своему аккаунту.",
  "orySettings.oidcInfo":
    "Подключенные аккаунты от этих провайдеров могут использоваться для входа в ваш аккаунт",
  "orySettings.lookupSecretTitle":
    "Резервные коды восстановления (вторая ступень)",
  "orySettings.lookupSecretDescription":
    "Резервные коды — это безопасная резервная копия для 2FA, позволяющая восстановить доступ к аккаунту, если вы потеряете устройство 2FA.",
  "orySettings.lookupSecretEnable": "Включить",
  "orySettings.webauthnTitle": "Управление аппаратными токенами",
  "orySettings.webauthnDescription":
    "Управляйте настройками аппаратного токена",
  "orySettings.webauthnNameLabel": "Название ключа безопасности",
  "orySettings.webauthnAdd": "Добавить ключ безопасности",
  "orySettings.webauthnInfo":
    "Аппаратные токены используются для второй ступени аутентификации или как первая ступень с Passkeys",
  "orySettings.totpTitle": "Приложение-аутентификатор",
  "orySettings.totpDescription":
    "Добавьте приложение-аутентификатор TOTP к своему аккаунту, чтобы повысить безопасность. Популярные приложения-аутентификаторы — LastPass и Google Authenticator",
  "orySettings.totpSecretLabel": "Секрет аутентификатора",
  "orySettings.totpCodeLabel": "Подтвердить код",
  "orySettings.totpInfoNotLinked":
    "Чтобы включить, отсканируйте QR-код с помощью вашего аутентификатора и введите код.",
  "orySettings.save": "Сохранить",
};

export default ru;
