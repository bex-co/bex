import type { TranslationEntry } from "@/i18n";

const jaGit: Record<string, TranslationEntry> = {
  "git.title": {
    message: "GitHub を接続",
    description: "Settings Connect GitHub card title",
  },
  "git.description": {
    message:
      "GitHub アカウントを接続すると、プライベートリポジトリをデプロイでき、インストール済みのすべてのリポジトリでゼロ設定のプッシュ時デプロイを利用できます。",
    description: "Settings Connect GitHub card description",
  },
  "git.connectedBadge": {
    message: "接続済み",
    description: "Badge shown when GitHub is connected",
  },
  "git.disconnectedBody": {
    message:
      "このワークスペースがデプロイ元とする GitHub アカウントと Organization を接続します。GitHub にリダイレクトされます。",
    description:
      "Body text in the disconnected state: what the single connect action does (ADR078 §3b)",
  },
  "git.connectButton": {
    message: "GitHub アカウントを接続",
    description:
      "The single connect action (ADR078 §3b, w1/m526) when the workspace has no GitHub connection: starts GitHub OAuth and lands on the account chooser",
  },
  "git.connectAnotherButton": {
    message: "別のアカウントを接続",
    description:
      "The single connect action (ADR078 §3b) once the workspace holds a GitHub connection: starts GitHub OAuth and lands on the account chooser",
  },
  "git.installAnotherButton": {
    message: "別の GitHub アカウントにインストール",
    description:
      "Account chooser action that installs the bex GitHub App on an account that does not have it yet (connectGit's stateful install URL, ADR078 §3b)",
  },
  "git.connectedAs": {
    message: "接続中のアカウント",
    description: "Label preceding the connected GitHub account login",
  },
  "git.manageAccess": {
    message: "GitHub でリポジトリのアクセスを管理",
    description: "Link to GitHub's install-settings page to change repo grants",
  },
  "git.disconnectButton": {
    message: "接続を解除",
    description: "Button that removes the GitHub connection",
  },
  "git.disconnectConfirmTitle": {
    message: "GitHub の接続を解除しますか？",
    description: "Confirm-dialog title for disconnecting GitHub",
  },
  "git.disconnectConfirmBody": {
    message:
      "再接続するまで、プライベートリポジトリのデプロイとハンズフリーのプッシュ時デプロイは停止します。アプリのインストールは、GitHub 側で削除するまで残ります。",
    description: "Confirm-dialog body for disconnecting GitHub",
  },
  "git.cancel": {
    message: "キャンセル",
    description: "Cancel button in the disconnect confirm dialog",
  },
  "git.unavailableTitle": {
    message: "GitHub 連携が設定されていません",
    description: "State when the backend has no GitHub App configured (503)",
  },
  "git.unavailableBody": {
    message:
      "この bex のデプロイメントには、GitHub App が設定されていません。プラットフォームの運用担当者に、BEX_GITHUB_APP_ID、BEX_GITHUB_APP_PRIVATE_KEY、BEX_GITHUB_APP_SLUG の設定を依頼してください。",
    description: "Body for the unavailable state",
  },
  "git.errorTitle": {
    message: "GitHub の接続を読み込めませんでした",
    description: "Generic error state title",
  },
  "git.errorBody": {
    message: "問題が発生しました。しばらくしてからもう一度お試しください。",
    description: "Generic error state body",
  },
  "git.connectError": {
    message: "GitHub の接続を開始できませんでした。",
    description: "Toast when starting the connect flow fails",
  },
  "git.callbackErrorTitle": {
    message: "GitHub の接続が完了しませんでした",
    description: "Title shown after the GitHub install callback fails",
  },
  "git.callbackErrorExpired": {
    message:
      "この接続リクエストは期限切れです。「GitHub アカウントを接続」を選択して、もう一度お試しください。",
    description: "Callback error shown when the signed state has expired",
  },
  "git.callbackErrorInvalid": {
    message:
      "この接続リクエストを確認できませんでした。「GitHub アカウントを接続」を選択して、もう一度お試しください。",
    description: "Callback error shown when signed state is invalid",
  },
  "git.callbackErrorMissing": {
    message:
      "この GitHub のインストールは、まだワークスペースに接続されていません。すでにアプリがあるアカウントでは、GitHub はインストールフローを完了できません。「GitHub アカウントを接続」を選択し、そのアカウントを選んでください。",
    description:
      "Callback error shown when state is missing (e.g. a direct github.com install): GitHub drops the state for an account that already has the App, so the recovery is the single connect action, whose account chooser lists it (ADR078 §3a/§3b)",
  },
  "git.callbackErrorNoClaimable": {
    message:
      "管理している GitHub アカウントが見つかりませんでした。正しい GitHub ユーザーで認可したか確認するか、先にそのアカウントに bex GitHub App をインストールしてください。",
    description:
      "Claim-callback failure: zero candidates. Rewritten in w2/m162 — under ADR078 N:N an account connected to another workspace is claimable, so this no longer covers that case and must not imply it",
  },
  "git.callbackErrorAmbiguous": {
    message:
      "管理している GitHub アカウントが複数見つかりましたが、選択を記録できませんでした。「GitHub アカウントを接続」を選択して、最初からやり直してください。",
    description:
      "Claim-callback failure: ambiguity the chooser could not be offered for (the selection failed to persist). The ordinary case renders the account chooser instead",
  },
  "git.claimSelectionTitle": {
    message: "接続する GitHub アカウントを選択",
    description:
      "Heading of the account chooser every connect lands on (ADR078 §3b)",
  },
  "git.claimSelectionBody": {
    message:
      "これらの GitHub アカウントには bex GitHub App がインストールされており、あなたが管理しています。このワークスペースに接続するアカウントを選ぶか、別のアカウントに App をインストールしてください。",
    description:
      "Body of the account chooser when it lists at least one installed account",
  },
  "git.claimSelectionEmpty": {
    message:
      "管理している GitHub アカウントには、まだ bex GitHub App がインストールされていません。接続するには、いずれかのアカウントにインストールしてください。",
    description:
      "Body of the account chooser when no account the user administers has the App yet; only the install action follows",
  },
  "git.claimSelectionGoneTitle": {
    message: "この選択は期限切れです",
    description: "Claim selection unknown, expired, or already used",
  },
  "git.claimSelectionGoneBody": {
    message:
      "「GitHub アカウントを接続」を選択して、最初からやり直してください。",
    description:
      "Recovery for an expired or spent account choice; the alert carries the Connect GitHub account button",
  },
  "git.selectClaimError": {
    message: "その GitHub アカウントを接続できませんでした。",
    description: "Toast when completing a claim selection fails",
  },
  "git.claimConnectedToast": {
    message: "{account} をこのワークスペースに接続しました。",
    description:
      "Toast after a choice in the account chooser binds; {account} is the GitHub account or org login",
  },
  "git.callbackErrorGeneric": {
    message:
      "GitHub は接続を完了できませんでした。「GitHub アカウントを接続」を選択して、もう一度お試しください。",
    description: "Generic GitHub callback failure message",
  },
  "git.callbackErrorInstallRequested": {
    message:
      "bex GitHub App のインストールリクエストを Organization のオーナーに送信しました。オーナーのいずれかが承認したら、もう一度接続してください。",
    description:
      "Callback outcome for an org member's install request (setup_action=request): the org's owners must approve it first (w1/m526)",
  },
  "git.disconnectSuccess": {
    message: "GitHub の接続を解除しました。",
    description: "Toast after a successful disconnect",
  },
  "git.disconnectError": {
    message: "GitHub の接続を解除できませんでした。",
    description: "Toast when disconnect fails",
  },
  "git.credentialsTrigger": {
    message: "認証情報 ({count})",
    description:
      "In-place credentials menu trigger on the source picker (w8/m31); {count} is the number of connected GitHub accounts",
  },
  "git.credentialsMenuLabel": {
    message: "認証情報",
    description:
      "Accessible name of the credentials menu panel on the source picker; the trigger's label without its count",
  },
  "git.credentialsAccountsHeading": {
    message: "アカウントと組織",
    description:
      "Heading above the connected-account list in the credentials menu",
  },
  "git.repoCount_other": {
    message: "{count} 件のリポジトリ",
    description: "Repo count shown next to a connected GitHub account (plural)",
  },
  "git.openInGitHub": {
    message: "GitHub で開く",
    description: "Link/title opening a connected account's GitHub page",
  },
  "git.configureInGitHub": {
    message: "GitHub で設定",
    description:
      "Link to GitHub's install-settings page to change repo grants (Render parity label)",
  },
  "git.disconnectAccount": {
    message: "{account} の接続を解除",
    description:
      "Accessible label for a specific account's disconnect button in the credentials menu",
  },
};

export default jaGit;
