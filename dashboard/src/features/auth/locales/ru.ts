import type { TranslationEntry } from "@/i18n";

const ruAuth: Record<string, TranslationEntry> = {
  "auth.loginTitle": {
    message: "С возвращением",
    description: "Login page hero title",
  },
  "auth.loginSubtitle": {
    message: "Войдите в свой аккаунт",
    description: "Login page hero subtitle",
  },
  "auth.registerTitle": {
    message: "Создайте аккаунт",
    description: "Registration page hero title",
  },
  "auth.registerSubtitle": {
    message: "Введите свои данные, чтобы начать",
    description: "Registration page hero subtitle",
  },
  "auth.forgotPasswordTitle": {
    message: "Сброс пароля",
    description: "Forgot-password page hero title",
  },
  "auth.forgotPasswordSubtitle": {
    message:
      "Введите адрес эл. почты вашего аккаунта, чтобы получить одноразовый код доступа",
    description: "Forgot-password page hero subtitle",
  },
  "auth.resetPasswordTitle": {
    message: "Задайте новый пароль",
    description:
      "Document title for the post-recovery reset-password landing page (renders the settings flow's password field)",
  },
  "auth.verificationTitle": {
    message: "Подтвердите адрес эл. почты",
    description: "Verification page hero title",
  },
  "auth.verificationSubtitle": {
    message:
      "Введите адрес эл. почты, связанный с вашим аккаунтом, чтобы продолжить",
    description:
      "Verification page hero subtitle (email-first Ory step; code arrives after)",
  },
  "auth.verificationRequiredSubtitle": {
    message: "Подтвердите адрес эл. почты, чтобы продолжить пользоваться bex",
    description:
      "Verification page hero subtitle for a signed-in session whose email is not verified yet (ADR075 D8 verification wall)",
  },
  "auth.verificationWrongAccount": {
    message: "Не ваш аккаунт?",
    description:
      "Verification wall: lead-in before the sign-out link, shown only to a signed-in session",
  },
  "auth.verificationSignOut": {
    message: "Выйти",
    description:
      "Verification wall: sign-out link, shown only to a signed-in session",
  },
  "auth.settingsTitle": {
    message: "Настройки",
    description: "Account settings page heading",
  },
  "auth.settingsSubtitle": {
    message: "Управляйте профилем, интеграциями, доступом и безопасностью.",
    description: "Account settings page subheading",
  },
  "auth.settingsNavigation": {
    message: "Разделы настроек",
    description: "Accessible label for the settings page section navigation",
  },
  "auth.accountSection": {
    message: "Аккаунт",
    description: "Settings page account section heading and navigation label",
  },
  "auth.accountSectionSubtitle": {
    message: "Обновите профиль, пароль и двухфакторную аутентификацию.",
    description: "Settings page account section description",
  },
  "auth.integrationsSection": {
    message: "Интеграции",
    description:
      "Settings page integrations section heading and navigation label",
  },
  "auth.integrationsSectionSubtitle": {
    message: "Подключите систему управления исходным кодом.",
    description: "Settings page integrations section description",
  },
  "auth.accessSection": {
    message: "Учётные данные доступа",
    description: "Settings page access section heading and navigation label",
  },
  "auth.accessSectionSubtitle": {
    message: "Управляйте программным доступом и SSH-идентификаторами.",
    description: "Settings page access section description",
  },
  "auth.securityComplianceSection": {
    message: "Безопасность",
    description:
      "Settings page section heading grouping the account's security cards",
  },
  "auth.securityComplianceSectionSubtitle": {
    message:
      "Средства безопасности аккаунта: подключённые агенты и активные сессии.",
    description: "Settings page Security & Compliance section description",
  },
  "auth.dangerZoneSection": {
    message: "Опасная зона",
    description: "Account settings destructive-actions section",
  },
  "auth.dangerZoneSectionSubtitle": {
    message: "Безвозвратно удалите аккаунт и отзовите весь доступ.",
    description: "Account deletion section description",
  },
  "auth.deleteAccountTitle": {
    message: "Удалить аккаунт",
    description: "Account deletion card title",
  },
  "auth.deleteAccountDescription": {
    message:
      "Это безвозвратно удалит вашу учётную запись, учётные данные и рабочие пространства, в которых вы единственный участник. Общие рабочие пространства останутся доступными другим их администраторам.",
    description: "Account deletion consequences",
  },
  "auth.deleteAccountWillDelete": {
    message: "Эти рабочие пространства будут удалены",
    description: "Account deletion preview heading for sole-member workspaces",
  },
  "auth.deleteAccountWillLeave": {
    message: "Вы покинете эти рабочие пространства",
    description: "Account deletion preview heading for shared workspaces",
  },
  "auth.deleteAccountBlockedTitle": {
    message: "Сначала разберитесь с владением рабочими пространствами",
    description: "Account deletion blocker heading",
  },
  "auth.deleteAccountBlockedDescription": {
    message:
      "В каждом из рабочих пространств ниже есть другие участники, но нет другого администратора. Назначьте администратором другого участника, удалите остальных участников или удалите рабочее пространство, прежде чем удалять аккаунт.",
    description: "Actionable account deletion blocker guidance",
  },
  "auth.deleteAccountBlockedAction": {
    message: "Открыть настройки",
    description: "Link to resolve an account deletion workspace blocker",
  },
  "auth.deleteAccountConfirmLabel": {
    message: "Введите {phrase} ниже для подтверждения.",
    description: "Exact account deletion confirmation prompt",
  },
  "auth.deleteAccountSubmit": {
    message: "Удалить мой аккаунт",
    description: "Account deletion submit button",
  },
  "auth.deleteAccountErrorTitle": {
    message: "Не удалось начать удаление аккаунта",
    description: "Account deletion mutation error heading",
  },
  "auth.deleteAccountError": {
    message: "Нам не удалось начать удаление аккаунта. Повторите попытку.",
    description: "Generic account deletion mutation error",
  },
  "auth.deleteAccountPreviewErrorTitle": {
    message: "Предварительный просмотр удаления недоступен",
    description: "Account deletion preview error heading",
  },
  "auth.deleteAccountPreviewError": {
    message:
      "Мы не смогли безопасно определить, что произойдёт с вашими рабочими пространствами. Ничего не удалено.",
    description: "Account deletion preview failure explanation",
  },
  "auth.deleteAccountRetry": {
    message: "Повторить",
    description: "Account deletion preview retry button",
  },
  "auth.errorTitle": {
    message: "Что-то пошло не так",
    description:
      "Title of /auth/error, where Kratos sends a self-service error (w5/161)",
  },
  "auth.errorSubtitle": {
    message: "Нам не удалось завершить этот шаг.",
    description: "Subtitle of /auth/error",
  },
  "auth.errorGeneric": {
    message:
      "Шаг входа истёк или не удалось его завершить. Начните заново со страницы входа.",
    description:
      "/auth/error body when Kratos's error has no readable reason or the id is unknown",
  },
  "auth.errorBackToSignIn": {
    message: "Назад ко входу",
    description: "/auth/error button to the sign-in page",
  },
  "auth.errorFlowExpired": {
    message:
      "Этот шаг входа занял слишком много времени и истёк. Начните заново со страницы входа.",
    description: "/auth/error message for Kratos self_service_flow_expired",
  },
  "auth.errorCsrf": {
    message:
      "Проверка безопасности браузера не совпала — обычно из-за второй вкладки или очищенных cookie. Начните заново со страницы входа.",
    description: "/auth/error message for Kratos security_csrf_violation",
  },
  "auth.errorIdentityMismatch": {
    message:
      "Этот шаг относится к другому аккаунту, не к тому, в который выполнен вход. Выйдите и начните заново.",
    description: "/auth/error message for Kratos security_identity_mismatch",
  },
  "auth.errorReturnToForbidden": {
    message: "Адрес для возврата после входа не разрешён.",
    description:
      "/auth/error message for Kratos self_service_flow_return_to_forbidden",
  },
  "auth.accountDeletedTitle": {
    message: "Удаление аккаунта начато",
    description: "Accepted account deletion terminal page title",
  },
  "auth.accountDeletedSubtitle": {
    message: "Вы безопасно вышли из аккаунта.",
    description: "Accepted account deletion terminal page subtitle",
  },
  "auth.accountDeletedStatus": {
    message:
      "Ваши рабочие пространства и учётные данные удаляются. Эту страницу можно закрыть; дополнительных действий не требуется.",
    description: "Accepted account deletion background completion status",
  },
  "auth.accountDeletedHome": {
    message: "Вернуться в bex",
    description: "Accepted account deletion terminal page home link",
  },
  "auth.loggingOutTitle": {
    message: "Выход...",
    description: "Logout page heading while the logout request is in flight",
  },
  "auth.loggingOutSubtitle": {
    message: "Завершаем вашу сессию, одну секунду.",
    description: "Logout page subtext while the logout request is in flight",
  },
  "auth.loggedOutTitle": {
    message: "Вы вышли",
    description: "Logout page heading once logout has completed",
  },
  "auth.loggedOutSubtitle": {
    message: "Переход на страницу входа…",
    description: "Logout page subtext once logout has completed",
  },
  "auth.logoutFailedTitle": {
    message: "Не удалось выйти",
    description: "Logout page heading when the provider logout request failed",
  },
  "auth.logoutFailedSubtitle": {
    message:
      "Нам не удалось завершить вашу сессию. Возможно, вы всё ещё в аккаунте — повторите попытку.",
    description: "Logout page subtext when the provider logout request failed",
  },
  "auth.logoutRetry": {
    message: "Повторить",
    description: "Logout page retry button after a failed sign-out",
  },
  "auth.logoutConfirmTitle": {
    message: "Выйти?",
    description:
      "Logout page heading on the confirmation screen (codex #12 CSRF logout fix)",
  },
  "auth.logoutConfirmSubtitle": {
    message: "Вы уверены, что хотите завершить сессию?",
    description: "Logout page subtext on the confirmation screen",
  },
  "auth.logoutConfirm": {
    message: "Выйти",
    description: "Logout page confirm button that triggers the actual logout",
  },
  "auth.logoutCancel": {
    message: "Отменить",
    description: "Logout page cancel button that returns to the dashboard",
  },
  "auth.logoutTitle": {
    message: "Выйти",
    description: "Logout page document title",
  },
  "auth.featureSecureTitle": {
    message: "Безопасность по умолчанию",
    description: "Auth hero feature bullet title",
  },
  "auth.featureSecureDescription": {
    message:
      "Сессиями управляет Ory Kratos — проверенная временем инфраструктура идентификации, а не самодельная система аутентификации.",
    description: "Auth hero feature bullet description",
  },
  "auth.featureDashboardTitle": {
    message: "Одна панель для всех сервисов",
    description: "Auth hero feature bullet title",
  },
  "auth.featureDashboardDescription": {
    message:
      "Развёртывайте, отслеживайте и контролируйте всё, что работает на bex, из одного места.",
    description: "Auth hero feature bullet description",
  },
  "auth.featureOpenSourceTitle": {
    message: "Создано в открытую",
    description: "Auth hero feature bullet title",
  },
  "auth.featureOpenSourceDescription": {
    message: "bex — открытая альтернатива Render.",
    description: "Auth hero feature bullet description",
  },
  "auth.consentTitle": {
    message: "Разрешить доступ",
    description: "OAuth2 consent page hero title",
  },
  "auth.consentSubtitle": {
    message: "{client} запрашивает право действовать от вашего имени в bex.",
    description: "OAuth2 consent page hero subtitle, names the OAuth client",
  },
  "auth.consentUnverifiedClient": {
    message: "Непроверенное сторонне приложение",
    description:
      "Warning label for an OAuth client whose self-provided branding is not trusted",
  },
  "auth.consentClientId": {
    message: "ID клиента",
    description: "Label for the immutable OAuth client identifier",
  },
  "auth.consentRedirectOrigin": {
    message: "Адрес получения кода авторизации",
    description: "Label for the origin that will receive the OAuth code",
  },
  "auth.consentNoRedirectOrigin": {
    message: "Без перенаправления (device flow)",
    description: "Shown when an OAuth device flow has no redirect origin",
  },
  "auth.consentScopesTitle": {
    message: "Запрашиваемые права",
    description: "Heading above the list of requested OAuth scopes",
  },
  "auth.consentScopeOpenid": {
    message: "Подтвердить, кто вы",
    description: "Human description of the openid scope",
  },
  "auth.consentScopeOfflineAccess": {
    message: "Оставаться подключённым без повторных запросов (refresh-токен)",
    description: "Human description of the offline_access scope",
  },
  "auth.consentScopeProfile": {
    message: "Читать ваш базовый профиль",
    description: "Human description of the profile scope",
  },
  "auth.consentScopeEmail": {
    message: "Читать ваш адрес эл. почты",
    description: "Human description of the email scope",
  },
  "auth.consentScopeRead": {
    message:
      "Читать обычные ресурсы рабочего пространства (сервисы, логи, метрики)",
    description: "Human description of the bex.read OAuth capability",
  },
  "auth.consentScopeWrite": {
    message:
      "Изменять ресурсы рабочего пространства (деплой, перезапуск, создание, удаление, оплата)",
    description: "Human description of the bex.write OAuth capability",
  },
  "auth.consentScopeSensitive": {
    message:
      "Читать секреты и строки подключения (переменные окружения, URL баз данных, файлы)",
    description: "Human description of the bex.sensitive OAuth capability",
  },
  "auth.consentScopeCloudAgents": {
    message:
      "Запускать облачных агентов для программирования от вашего имени (запускает задачи, которые расходуют вычислительные ресурсы и кредиты модели, и может отправить ветку)",
    description: "Human description of the bex.cloud_agents OAuth capability",
  },
  "auth.consentScopeApiCompat": {
    message:
      "Полный доступ к плоскости управления (совместимость с устаревшими клиентами платформы)",
    description: "Human description of the bex.api compatibility scope",
  },
  "auth.consentAudienceTitle": {
    message: "Где работает токен",
    description: "Heading above the requested access-token audience",
  },
  "auth.consentRememberHint": {
    message:
      "Разрешение запоминается на час — после этого агент спросит снова.",
    description: "Consent page footnote about the remember window",
  },
  "auth.consentApprove": {
    message: "Разрешить",
    description: "OAuth2 consent page approve button",
  },
  "auth.consentDeny": {
    message: "Отклонить",
    description: "OAuth2 consent page deny button",
  },
  "auth.consentFailed": {
    message: "При выполнении что-то пошло не так. Повторите попытку.",
    description: "Consent page error banner after a failed accept/reject",
  },
  "auth.consentExpiredTitle": {
    message: "Авторизация истекла",
    description: "Consent page heading when there is no live consent request",
  },
  "auth.consentExpiredSubtitle": {
    message:
      "Начните подключение заново из приложения, которое отправило вас сюда.",
    description:
      "Consent page subtext when there is no live consent request to decide",
  },
  "auth.consentInvalidRequestTitle": {
    message: "Недопустимый запрос авторизации",
    description:
      "Consent page heading when the requesting client's authorization request fails a protocol check (PKCE or scope)",
  },
  "auth.consentPkceRequiredSubtitle": {
    message:
      "В запросе этого приложения отсутствуют необходимые параметры безопасности. Свяжитесь с разработчиком приложения.",
    description:
      "Consent page subtext when the authorization request lacks PKCE with S256",
  },
  "auth.consentScopeRequiredSubtitle": {
    message:
      "Это приложение запросило доступ к API, не указав, что ему нужно. Оно должно запросить хотя бы одно из: {scopes}.",
    description:
      "Consent page subtext when an audience request lacks a granular capability scope (round-14 #1); {scopes} is the comma-separated list of accepted capability scopes",
  },
  "auth.consentUnavailableTitle": {
    message: "Авторизация недоступна",
    description:
      "Consent page heading when the authorization provider is unreachable, misconfigured, or a headless accept failed",
  },
  "auth.consentUnavailableSubtitle": {
    message: "Скоро это должно решиться — повторите попытку чуть позже.",
    description:
      "Consent page subtext when the authorization provider is unreachable, misconfigured, or a headless accept failed",
  },
  "auth.consentWrongUserTitle": {
    message: "Выполнен вход не в тот аккаунт",
    description:
      "Consent page heading when the browser's session belongs to a different account than the one that started this authorization",
  },
  "auth.consentWrongUserSubtitle": {
    message:
      "Сессия этого браузера относится к другому аккаунту, не к тому, который начал эту авторизацию. Выйдите и повторите попытку.",
    description:
      "Consent page subtext when the browser's session belongs to a different account than the one that started this authorization",
  },
  "auth.deviceSuccessTitle": {
    message: "Устройство подключено",
    description: "Device authorization success page title",
  },
  "auth.deviceTitle": {
    message: "Подключить устройство",
    description: "Device verification route document title",
  },
  "auth.deviceConfirmSubtitle": {
    message:
      "Устройство, на котором вы входите, запрашивает доступ к вашему аккаунту.",
    description: "Device confirm page hero subtitle",
  },
  "auth.deviceConfirmHeading": {
    message: "Авторизовать это устройство",
    description: "Device confirm page card heading",
  },
  "auth.deviceConfirmDescription": {
    message: "Убедитесь, что код ниже совпадает с кодом на вашем устройстве:",
    description: "Device confirm page card description above the code",
  },
  "auth.deviceConfirmHint": {
    message: "Разрешайте это, только если вы сами начали этот вход.",
    description:
      "Device confirm page footnote warning against unsolicited requests",
  },
  "auth.deviceConfirmCancel": {
    message: "Отменить",
    description: "Device confirm page cancel button",
  },
  "auth.deviceConfirmAuthorize": {
    message: "Авторизовать устройство",
    description: "Device confirm page authorize/submit button",
  },
  "auth.deviceExpiredTitle": {
    message: "Код устройства истёк",
    description:
      "Device confirm page heading when there is no live device request",
  },
  "auth.deviceExpiredSubtitle": {
    message:
      "Для этой страницы нужен код устройства, либо срок его действия истёк. Начните вход заново на своём устройстве.",
    description:
      "Device confirm page subtext when there is no live device request to confirm",
  },
  "auth.deviceUnavailableTitle": {
    message: "Авторизация устройства недоступна",
    description:
      "Device confirm page heading when the authorization provider is unreachable or misconfigured",
  },
  "auth.deviceUnavailableSubtitle": {
    message: "Скоро это должно решиться — повторите попытку чуть позже.",
    description:
      "Device confirm page subtext when the authorization provider is unreachable or misconfigured",
  },
  "auth.deviceRefusedTitle": {
    message: "В авторизации отказано",
    description:
      "Device confirm page heading when the device request names a client bex did not expect",
  },
  "auth.deviceRefusedSubtitle": {
    message:
      "Не удалось проверить этот запрос устройства. Начните вход заново на своём устройстве.",
    description:
      "Device confirm page subtext when the device request names a client bex did not expect",
  },
  "auth.deviceSuccessSubtitle": {
    message: "Авторизация в браузере завершена.",
    description: "Device authorization success page subtitle",
  },
  "auth.deviceSuccessHint": {
    message: "Вернитесь в приложение, в которое входите, чтобы продолжить.",
    description: "Device authorization success page hint",
  },
  "auth.deviceSuccessWaiting": {
    message: "Ожидание авторизации в браузере…",
    description:
      "Device success page terminal replica: the CLI line shown while it polls for tokens",
  },
  "auth.deviceSuccessDone": {
    message: "Авторизация завершена. Вы вошли.",
    description:
      "Device success page terminal replica: the CLI success line (rendered after a ✓ glyph)",
  },
  "auth.deviceSuccessClose": {
    message: "Эту вкладку можно закрыть.",
    description:
      "Device success page: follow-up sentence after the return-to-terminal hint",
  },
  "auth.deviceSuccessDashboard": {
    message: "Открыть панель",
    description:
      "Device success page: quiet link to the dashboard home for users staying in the browser",
  },
  "auth.profileEmailLabel": {
    message: "Эл. почта",
    description:
      "Account settings profile form: the email field label (the identity schema title stays as Kratos uses it)",
  },
  "auth.profileEmailPlaceholder": {
    message: "Введите адрес эл. почты",
    description: "Account settings profile form: the email field placeholder",
  },
  "auth.profileNameLabel": {
    message: "Имя",
    description:
      "Account settings profile form: the name field label (the identity schema title stays as Kratos uses it)",
  },
  "auth.profileNamePlaceholder": {
    message: "Введите своё имя",
    description: "Account settings profile form: the name field placeholder",
  },
  "auth.inputPlaceholder": {
    message: "Введите: {placeholder}",
    description:
      "Auth forms: the placeholder of a field with no wording of its own, e.g. the password; {placeholder} is the field's label",
  },
  "auth.showPassword": {
    message: "Показать пароль",
    description:
      "Accessible name of the password field toggle while the password is hidden",
  },
  "auth.hidePassword": {
    message: "Скрыть пароль",
    description:
      "Accessible name of the password field toggle while the password is shown",
  },
  "auth.ssoMoreOptions": {
    message: "Другие варианты",
    description:
      "Login and sign-up: reveals the GitLab and Bitbucket sign-in buttons, as Render's More options does",
  },
  "auth.settingsGuardEmailChangeRefused": {
    message:
      "Вы не можете изменить адрес электронной почты, пока состоите в рабочем пространстве, где требуется вход через Google. Сначала покиньте эти рабочие пространства: {workspaces}.",
    description:
      "Account settings refusal of an email change (Kratos message 4100001) while the member belongs to a workspace that requires Google login; {workspaces} is the list of those workspaces (w7/m465)",
  },
  "auth.settingsGuardUnlinkRefused": {
    message:
      "Вы не можете отключить аккаунт Google, пока состоите в рабочем пространстве, где требуется вход через Google. Сначала покиньте эти рабочие пространства: {workspaces}.",
    description:
      "Account settings refusal of disconnecting the Google account, or an unlink that does not name its provider (Kratos message 4100002), while the member belongs to a workspace that requires Google login; {workspaces} is the list of those workspaces (w7/m465, w7/085)",
  },
};

export default ruAuth;
