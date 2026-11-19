import type { OryCopy } from "./keys";

// Ory Elements' own `ru` catalog entries, copied verbatim (see ./keys.ts).
const ru: OryCopy = {
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
  "oryCard.loginTitle": "Войти",
  "oryCard.loginSubtitle": "Войти с помощью {parts}",
  "oryCard.registrationTitle": "Зарегистрировать аккаунт",
  "oryCard.registrationSubtitle": "Зарегистрироваться с помощью {parts}",
  "oryCard.passwordPart": "ваш {identifierLabel} и пароль",
  "oryCard.oidcPart": "социальный провайдер",
  "oryCard.identifierPart": "ваш {identifierLabel}",
  "oryCard.or": "или",
  "oryCard.recoveryTitle": "Восстановите свой аккаунт",
  "oryCard.recoverySubtitle":
    "Введите идентификатор, связанный с вашим аккаунтом, чтобы получить одноразовый код доступа",
  "oryCard.verificationTitle": "Подтвердите свой аккаунт",
  "oryCard.verificationSubtitle":
    "Введите адрес электронной почты, связанный с вашим аккаунтом, чтобы подтвердить его",
  "oryCard.loginFooterLabel": "Нет аккаунта?",
  "oryCard.loginFooterLink": "Зарегистрироваться",
  "oryCard.registrationFooterLabel": "Уже есть аккаунт?",
  "oryCard.registrationFooterLink": "Войти",
};

export default ru;
