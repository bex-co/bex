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
      "Подключите аккаунты и организации GitHub, из которых деплоит это рабочее пространство. Вы будете перенаправлены на GitHub.",
    description:
      "Body text in the disconnected state: what the single connect action does (ADR078 §3b)",
  },
  "git.connectButton": {
    message: "Подключить аккаунт GitHub",
    description:
      "The single connect action (ADR078 §3b, w1/m526) when the workspace has no GitHub connection: starts GitHub OAuth and lands on the account chooser",
  },
  "git.connectAnotherButton": {
    message: "Подключить другой аккаунт",
    description:
      "The single connect action (ADR078 §3b) once the workspace holds a GitHub connection: starts GitHub OAuth and lands on the account chooser",
  },
  "git.installAnotherButton": {
    message: "Установить в другой аккаунт GitHub",
    description:
      "Account chooser action that installs the bex GitHub App on an account that does not have it yet (connectGit's stateful install URL, ADR078 §3b)",
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
      "Срок действия этого запроса на подключение истёк. Нажмите «Подключить аккаунт GitHub», чтобы повторить попытку.",
    description: "Callback error shown when the signed state has expired",
  },
  "git.callbackErrorInvalid": {
    message:
      "Не удалось проверить этот запрос на подключение. Нажмите «Подключить аккаунт GitHub», чтобы повторить попытку.",
    description: "Callback error shown when signed state is invalid",
  },
  "git.callbackErrorMissing": {
    message:
      "Эта установка GitHub ещё не подключена к рабочему пространству. GitHub не может завершить установку для аккаунта, в котором приложение уже есть. Нажмите «Подключить аккаунт GitHub» и выберите этот аккаунт.",
    description:
      "Callback error shown when state is missing (e.g. a direct github.com install): GitHub drops the state for an account that already has the App, so the recovery is the single connect action, whose account chooser lists it (ADR078 §3a/§3b)",
  },
  "git.callbackErrorNoClaimable": {
    message:
      "Не найден ни один аккаунт GitHub, которым вы управляете. Убедитесь, что вы авторизовали нужного пользователя GitHub, или сначала установите GitHub App bex на аккаунт.",
    description:
      "Claim-callback failure: zero candidates. Rewritten in w2/m162 — under ADR078 N:N an account connected to another workspace is claimable, so this no longer covers that case and must not imply it",
  },
  "git.callbackErrorAmbiguous": {
    message:
      "Найдено несколько аккаунтов GitHub, которыми вы управляете, и выбор не удалось записать. Нажмите «Подключить аккаунт GitHub», чтобы начать заново.",
    description:
      "Claim-callback failure: ambiguity the chooser could not be offered for (the selection failed to persist). The ordinary case renders the account chooser instead",
  },
  "git.claimSelectionTitle": {
    message: "Выберите аккаунт GitHub для подключения",
    description:
      "Heading of the account chooser every connect lands on (ADR078 §3b)",
  },
  "git.claimSelectionBody": {
    message:
      "В этих аккаунтах GitHub установлено GitHub App bex, и вы ими управляете. Выберите аккаунт для подключения к этому рабочему пространству или установите App в другой аккаунт.",
    description:
      "Body of the account chooser when it lists at least one installed account",
  },
  "git.claimSelectionEmpty": {
    message:
      "Ни в одном из аккаунтов GitHub, которыми вы управляете, ещё нет GitHub App bex. Установите его в аккаунт, чтобы подключить его.",
    description:
      "Body of the account chooser when no account the user administers has the App yet; only the install action follows",
  },
  "git.claimSelectionGoneTitle": {
    message: "Срок действия этого выбора истёк",
    description: "Claim selection unknown, expired, or already used",
  },
  "git.claimSelectionGoneBody": {
    message: "Нажмите «Подключить аккаунт GitHub», чтобы начать заново.",
    description:
      "Recovery for an expired or spent account choice; the alert carries the Connect GitHub account button",
  },
  "git.selectClaimError": {
    message: "Не удалось подключить этот аккаунт GitHub.",
    description: "Toast when completing a claim selection fails",
  },
  "git.claimConnectedToast": {
    message: "{account} подключён к этому рабочему пространству.",
    description:
      "Toast after a choice in the account chooser binds; {account} is the GitHub account or org login",
  },
  "git.callbackErrorGeneric": {
    message:
      "GitHub не удалось завершить подключение. Нажмите «Подключить аккаунт GitHub», чтобы повторить попытку.",
    description: "Generic GitHub callback failure message",
  },
  "git.callbackErrorInstallRequested": {
    message:
      "Ваш запрос на установку GitHub App bex отправлен владельцам организации. Когда один из них одобрит его, подключитесь снова.",
    description:
      "Callback outcome for an org member's install request (setup_action=request): the org's owners must approve it first (w1/m526)",
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
