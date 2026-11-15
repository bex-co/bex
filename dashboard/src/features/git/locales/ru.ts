import type { TranslationEntry } from "@/i18n";

const ruGit: Record<string, TranslationEntry> = {
  "git.title": {
    message: "Подключить GitHub",
    description: "Settings Connect GitHub card title",
  },
  "git.description": {
    message:
      "Подключите аккаунт GitHub, чтобы развёртывать приватные репозитории и получать деплой при пуше без настройки для каждого установленного репозитория.",
    description: "Settings Connect GitHub card description",
  },
  "git.connectedBadge": {
    message: "Подключено",
    description: "Badge shown when GitHub is connected",
  },
  "git.disconnectedBody": {
    message:
      "Установите GitHub App bex на свой аккаунт и выберите, каким репозиториям предоставить доступ. Вы будете перенаправлены на GitHub.",
    description: "Body text in the disconnected state",
  },
  "git.connectButton": {
    message: "Подключить GitHub",
    description: "Button that starts the GitHub install flow",
  },
  "git.connectAnotherButton": {
    message: "Подключить другой аккаунт",
    description:
      "Button that connects an additional GitHub account/org to the workspace",
  },
  "git.claimButton": {
    message: "Привязать установленный аккаунт",
    description:
      "Button starting the ADR075 §3a claim flow: bind a GitHub account where the app is ALREADY installed (GitHub strips the install URL's state for those)",
  },
  "git.claimHint": {
    message:
      "Уже установили GitHub App bex на этот аккаунт? Используйте «Привязать». Для аккаунта, где приложение установлено, GitHub показывает «Configure» вместо «Install», и этот путь никогда не возвращает в bex.",
    description:
      "Hint under the connect/claim buttons explaining when to use claim",
  },
  "git.claimError": {
    message: "Не удалось начать привязку GitHub.",
    description: "Toast when starting the claim flow fails",
  },
  "git.connectedAs": {
    message: "Подключено как",
    description: "Label preceding the connected GitHub account login",
  },
  "git.manageAccess": {
    message: "Управлять доступом к репозиториям на GitHub",
    description: "Link to GitHub's install-settings page to change repo grants",
  },
  "git.disconnectButton": {
    message: "Отключить",
    description: "Button that removes the GitHub connection",
  },
  "git.disconnectConfirmTitle": {
    message: "Отключить GitHub?",
    description: "Confirm-dialog title for disconnecting GitHub",
  },
  "git.disconnectConfirmBody": {
    message:
      "Деплой приватных репозиториев и автоматический деплой при пуше прекратятся, пока вы не подключитесь снова. Установка приложения останется на GitHub, пока вы не удалите её там.",
    description: "Confirm-dialog body for disconnecting GitHub",
  },
  "git.cancel": {
    message: "Отмена",
    description: "Cancel button in the disconnect confirm dialog",
  },
  "git.unavailableTitle": {
    message: "Интеграция с GitHub не настроена",
    description: "State when the backend has no GitHub App configured (503)",
  },
  "git.unavailableBody": {
    message:
      "В этой установке bex не настроено GitHub App. Попросите оператора платформы настроить BEX_GITHUB_APP_ID, BEX_GITHUB_APP_PRIVATE_KEY и BEX_GITHUB_APP_SLUG.",
    description: "Body for the unavailable state",
  },
  "git.errorTitle": {
    message: "Не удалось загрузить подключение GitHub",
    description: "Generic error state title",
  },
  "git.errorBody": {
    message: "Что-то пошло не так. Повторите попытку через минуту.",
    description: "Generic error state body",
  },
  "git.connectError": {
    message: "Не удалось начать подключение GitHub.",
    description: "Toast when starting the connect flow fails",
  },
  "git.callbackErrorTitle": {
    message: "Подключение GitHub не завершено",
    description: "Title shown after the GitHub install callback fails",
  },
  "git.callbackErrorExpired": {
    message:
      "Срок действия этого запроса на подключение истёк. Нажмите «Подключить GitHub», чтобы повторить попытку.",
    description: "Callback error shown when the signed state has expired",
  },
  "git.callbackErrorInvalid": {
    message:
      "Не удалось проверить этот запрос на подключение. Нажмите «Подключить GitHub», чтобы повторить попытку.",
    description: "Callback error shown when signed state is invalid",
  },
  "git.callbackErrorMissing": {
    message:
      "Эта установка GitHub ещё не подключена к рабочему пространству. GitHub не может завершить процесс установки для аккаунта, где приложение уже есть — используйте «Привязать установленный аккаунт» ниже.",
    description:
      "Callback error shown when state is missing (e.g. a direct github.com install) — points at the claim flow, which is the path that works for already-installed accounts (ADR075 §3a)",
  },
  "git.callbackErrorNoClaimable": {
    message:
      "Не найден ни один аккаунт GitHub, которым вы управляете. Убедитесь, что вы авторизовали нужного пользователя GitHub, или сначала установите GitHub App bex на аккаунт.",
    description:
      "Claim-callback failure: zero candidates. Rewritten in w2/m162 — under ADR078 N:N an account connected to another workspace is claimable, so this no longer covers that case and must not imply it",
  },
  "git.callbackErrorAmbiguous": {
    message:
      "Найдено несколько аккаунтов GitHub, которыми вы управляете, и выбор не удалось записать. Нажмите «Привязать установленный аккаунт», чтобы начать заново.",
    description:
      "Claim-callback failure: ambiguity the picker could not be offered for (the selection failed to persist). The ordinary ambiguous case now renders the account picker instead",
  },
  "git.claimSelectionTitle": {
    message: "Выберите аккаунт GitHub для подключения",
    description: "Heading of the deferred claim account picker (ADR078 §3a)",
  },
  "git.claimSelectionBody": {
    message:
      "Вы управляете несколькими аккаунтами GitHub. Выберите тот, который нужно подключить к этому рабочему пространству.",
    description: "Body of the deferred claim account picker",
  },
  "git.claimSelectionGoneTitle": {
    message: "Срок действия этого выбора истёк",
    description: "Claim selection unknown, expired, or already used",
  },
  "git.claimSelectionGoneBody": {
    message:
      "Нажмите «Привязать установленный аккаунт» ниже, чтобы начать заново.",
    description: "Recovery for an expired or spent claim selection",
  },
  "git.selectClaimError": {
    message: "Не удалось подключить этот аккаунт GitHub.",
    description: "Toast when completing a claim selection fails",
  },
  "git.callbackErrorGeneric": {
    message:
      "GitHub не смог завершить подключение. Нажмите «Подключить GitHub», чтобы повторить попытку.",
    description: "Generic GitHub callback failure message",
  },
  "git.disconnectSuccess": {
    message: "GitHub отключён.",
    description: "Toast after a successful disconnect",
  },
  "git.disconnectError": {
    message: "Не удалось отключить GitHub.",
    description: "Toast when disconnect fails",
  },
  "git.credentialsTrigger": {
    message: "Учётные данные ({count})",
    description:
      "In-place credentials menu trigger on the source picker (w8/m31); {count} is the number of connected GitHub accounts",
  },
  "git.credentialsMenuLabel": {
    message: "Учётные данные",
    description:
      "Accessible name of the credentials menu panel on the source picker; the trigger's label without its count",
  },
  "git.credentialsAccountsHeading": {
    message: "Аккаунты и организации",
    description:
      "Heading above the connected-account list in the credentials menu",
  },
  "git.repoCount_one": {
    message: "{count} репозиторий",
    description:
      "Repo count shown next to a connected GitHub account (singular)",
  },
  "git.repoCount_few": {
    message: "{count} репозитория",
    description: "Repo count shown next to a connected GitHub account (plural)",
  },
  "git.repoCount_many": {
    message: "{count} репозиториев",
    description: "Repo count shown next to a connected GitHub account (plural)",
  },
  "git.repoCount_other": {
    message: "{count} репозитория",
    description: "Repo count shown next to a connected GitHub account (plural)",
  },
  "git.openInGitHub": {
    message: "Открыть на GitHub",
    description: "Link/title opening a connected account's GitHub page",
  },
  "git.configureInGitHub": {
    message: "Настроить на GitHub",
    description:
      "Link to GitHub's install-settings page to change repo grants (Render parity label)",
  },
  "git.disconnectAccount": {
    message: "Отключить {account}",
    description:
      "Accessible label for a specific account's disconnect button in the credentials menu",
  },
};

export default ruGit;
