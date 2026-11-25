import type { TranslationEntry } from "@/i18n";

const jaAuth: Record<string, TranslationEntry> = {
  "auth.loginTitle": {
    message: "おかえりなさい",
    description: "Login page hero title",
  },
  "auth.loginSubtitle": {
    message: "アカウントにログインしてください",
    description: "Login page hero subtitle",
  },
  "auth.registerTitle": {
    message: "アカウントを作成",
    description: "Registration page hero title",
  },
  "auth.registerSubtitle": {
    message: "詳細を入力して始めましょう",
    description: "Registration page hero subtitle",
  },
  "auth.forgotPasswordTitle": {
    message: "パスワードをリセット",
    description: "Forgot-password page hero title",
  },
  "auth.forgotPasswordSubtitle": {
    message:
      "アカウントのメールアドレスを入力すると、ワンタイムのアクセスコードが届きます",
    description: "Forgot-password page hero subtitle",
  },
  "auth.resetPasswordTitle": {
    message: "新しいパスワードを設定",
    description:
      "Document title for the post-recovery reset-password landing page (renders the settings flow's password field)",
  },
  "auth.verificationTitle": {
    message: "メールアドレスを確認",
    description: "Verification page hero title",
  },
  "auth.verificationSubtitle": {
    message:
      "続行するには、アカウントに関連付けられたメールアドレスを入力してください",
    description:
      "Verification page hero subtitle (email-first Ory step; code arrives after)",
  },
  "auth.verificationRequiredSubtitle": {
    message: "bex を引き続き使用するには、メールアドレスを確認してください",
    description:
      "Verification page hero subtitle for a signed-in session whose email is not verified yet (ADR075 D8 verification wall)",
  },
  "auth.verificationWrongAccount": {
    message: "ご自分のアカウントではありませんか？",
    description:
      "Verification wall: lead-in before the sign-out link, shown only to a signed-in session",
  },
  "auth.verificationSignOut": {
    message: "ログアウト",
    description:
      "Verification wall: sign-out link, shown only to a signed-in session",
  },
  "auth.settingsTitle": {
    message: "設定",
    description: "Account settings page heading",
  },
  "auth.settingsSubtitle": {
    message: "ID、連携、アクセス、セキュリティを管理します。",
    description: "Account settings page subheading",
  },
  "auth.settingsNavigation": {
    message: "設定のセクション",
    description: "Accessible label for the settings page section navigation",
  },
  "auth.accountSection": {
    message: "アカウント",
    description: "Settings page account section heading and navigation label",
  },
  "auth.accountSectionSubtitle": {
    message: "プロフィール、パスワード、2 要素認証を更新します。",
    description: "Settings page account section description",
  },
  "auth.appearanceSection": {
    message: "外観",
    description:
      "Account settings (w3/m528): Render's Appearance section heading and its rail link; a log explorer's Options > Theme Settings opens it",
  },
  "auth.appearanceSectionSubtitle": {
    message:
      "ダッシュボードの見た目を選びます。選択はアカウントに保存されます。",
    description: "Account settings (w3/m528): Appearance section description",
  },
  "auth.dashboardTheme": {
    message: "ダッシュボードのテーマ",
    description:
      "Account settings > Appearance (w3/m528, w3/m531): Render's Dashboard Theme setting, saved on the account; its row label and the select's accessible name",
  },
  "auth.dashboardThemeSystem": {
    message: "システム（デフォルト）",
    description:
      "Account settings > Appearance (w3/m528): the System theme option, marked as the default as Render marks it",
  },
  "auth.logExplorerTheme": {
    message: "ログエクスプローラーのテーマ",
    description:
      "Account settings > Appearance (w3/m531): Render's Log Explorer Theme setting, its row label and the select's accessible name",
  },
  "auth.logExplorerThemeMatch": {
    message: "ダッシュボードに合わせる（デフォルト）",
    description:
      "Account settings > Appearance (w3/m531): the Log Explorer Theme option that follows the dashboard's theme, marked as the default as Render marks it",
  },
  "auth.highContrastMode": {
    message: "ハイコントラストモード",
    description:
      "Account settings > Appearance (w3/m531): Render's High Contrast Mode setting, its row label and the switch's accessible name",
  },
  "auth.highContrastModeDescription": {
    message: "操作できる要素を見やすくします。",
    description:
      "Account settings > Appearance (w3/m531): High Contrast Mode's description, Render's words",
  },
  "auth.highContrastEnabled": {
    message: "有効",
    description:
      "Account settings > Appearance (w3/m531): the text beside High Contrast Mode's switch while it is on",
  },
  "auth.highContrastDisabled": {
    message: "無効",
    description:
      "Account settings > Appearance (w3/m531): the text beside High Contrast Mode's switch while it is off",
  },
  "auth.appearanceEdit": {
    message: "編集",
    description:
      "Account settings > Appearance (w3/097): the ghost button under Dashboard Theme's and Log Explorer Theme's selects that opens the row for editing, Render's words",
  },
  "auth.appearanceCancel": {
    message: "キャンセル",
    description:
      "Account settings > Appearance (w3/097): the button that ends a theme row's edit unsaved and puts the saved theme back, Render's words",
  },
  "auth.appearanceSaveChanges": {
    message: "変更を保存",
    description:
      "Account settings > Appearance (w3/097): the button that writes a theme row's choice to the account, Render's words",
  },
  "auth.appearanceSaving": {
    message: "保存中",
    description:
      "Account settings > Appearance (w3/097): the save button's text while the account is answering, Render's words",
  },
  "auth.integrationsSection": {
    message: "連携",
    description:
      "Settings page integrations section heading and navigation label",
  },
  "auth.integrationsSectionSubtitle": {
    message: "ソース管理を接続します。",
    description: "Settings page integrations section description",
  },
  "auth.accessSection": {
    message: "アクセス認証情報",
    description: "Settings page access section heading and navigation label",
  },
  "auth.accessSectionSubtitle": {
    message: "プログラムからのアクセスと SSH ID を管理します。",
    description: "Settings page access section description",
  },
  "auth.securityComplianceSection": {
    message: "セキュリティ",
    description:
      "Settings page section heading grouping the account's security cards",
  },
  "auth.securityComplianceSectionSubtitle": {
    message:
      "アカウントのセキュリティ管理: 接続済みのエージェントとアクティブなセッション。",
    description: "Settings page Security & Compliance section description",
  },
  "auth.dangerZoneSection": {
    message: "危険な操作",
    description: "Account settings destructive-actions section",
  },
  "auth.dangerZoneSectionSubtitle": {
    message: "アカウントを完全に削除し、すべてのアクセスを失効します。",
    description: "Account deletion section description",
  },
  "auth.deleteAccountTitle": {
    message: "アカウントを削除",
    description: "Account deletion card title",
  },
  "auth.deleteAccountDescription": {
    message:
      "あなたの ID、認証情報、あなた以外にメンバーがいないワークスペースが完全に削除されます。共有ワークスペースは、他の管理者が引き続き利用できます。",
    description: "Account deletion consequences",
  },
  "auth.deleteAccountWillDelete": {
    message: "これらのワークスペースが削除されます",
    description: "Account deletion preview heading for sole-member workspaces",
  },
  "auth.deleteAccountWillLeave": {
    message: "これらのワークスペースから退出します",
    description: "Account deletion preview heading for shared workspaces",
  },
  "auth.deleteAccountBlockedTitle": {
    message: "先にワークスペースのオーナーシップを解決してください",
    description: "Account deletion blocker heading",
  },
  "auth.deleteAccountBlockedDescription": {
    message:
      "以下の各ワークスペースには他のメンバーがいますが、他の管理者がいません。アカウントを削除する前に、別のメンバーを昇格するか、他のメンバーを削除するか、ワークスペースを削除してください。",
    description: "Actionable account deletion blocker guidance",
  },
  "auth.deleteAccountBlockedAction": {
    message: "設定を開く",
    description: "Link to resolve an account deletion workspace blocker",
  },
  "auth.deleteAccountConfirmLabel": {
    message: "確認するには、下に {phrase} と入力してください。",
    description: "Exact account deletion confirmation prompt",
  },
  "auth.deleteAccountSubmit": {
    message: "アカウントを削除する",
    description: "Account deletion submit button",
  },
  "auth.deleteAccountErrorTitle": {
    message: "アカウントの削除を開始できませんでした",
    description: "Account deletion mutation error heading",
  },
  "auth.deleteAccountError": {
    message: "アカウントの削除を開始できませんでした。もう一度お試しください。",
    description: "Generic account deletion mutation error",
  },
  "auth.deleteAccountPreviewErrorTitle": {
    message: "削除のプレビューを利用できません",
    description: "Account deletion preview error heading",
  },
  "auth.deleteAccountPreviewError": {
    message:
      "ワークスペースがどうなるかを安全に判定できませんでした。何も削除されていません。",
    description: "Account deletion preview failure explanation",
  },
  "auth.deleteAccountRetry": {
    message: "再試行",
    description: "Account deletion preview retry button",
  },
  "auth.errorTitle": {
    message: "問題が発生しました",
    description:
      "Title of /auth/error, where Kratos sends a self-service error (w5/161)",
  },
  "auth.errorSubtitle": {
    message: "その手順を完了できませんでした。",
    description: "Subtitle of /auth/error",
  },
  "auth.errorGeneric": {
    message:
      "ログインの手順の有効期限が切れたか、完了できませんでした。ログインページからやり直してください。",
    description:
      "/auth/error body when Kratos's error has no readable reason or the id is unknown",
  },
  "auth.errorBackToSignIn": {
    message: "ログインに戻る",
    description: "/auth/error button to the sign-in page",
  },
  "auth.errorFlowExpired": {
    message:
      "そのログインの手順は時間がかかりすぎて期限切れになりました。ログインページからやり直してください。",
    description: "/auth/error message for Kratos self_service_flow_expired",
  },
  "auth.errorCsrf": {
    message:
      "ブラウザーのセキュリティチェックが一致しませんでした。通常は、別のタブを開いたか、Cookie を削除したことが原因です。ログインページからやり直してください。",
    description: "/auth/error message for Kratos security_csrf_violation",
  },
  "auth.errorIdentityMismatch": {
    message:
      "その手順は、現在ログインしているアカウントとは別のアカウントのものです。ログアウトしてから、やり直してください。",
    description: "/auth/error message for Kratos security_identity_mismatch",
  },
  "auth.errorReturnToForbidden": {
    message: "ログイン後の戻り先のアドレスは許可されていません。",
    description:
      "/auth/error message for Kratos self_service_flow_return_to_forbidden",
  },
  "auth.accountDeletedTitle": {
    message: "アカウントの削除を開始しました",
    description: "Accepted account deletion terminal page title",
  },
  "auth.accountDeletedSubtitle": {
    message: "安全にログアウトしました。",
    description: "Accepted account deletion terminal page subtitle",
  },
  "auth.accountDeletedStatus": {
    message:
      "ワークスペースと認証情報を削除しています。このページは閉じて問題ありません。これ以上の操作は不要です。",
    description: "Accepted account deletion background completion status",
  },
  "auth.accountDeletedHome": {
    message: "bex に戻る",
    description: "Accepted account deletion terminal page home link",
  },
  "auth.loggingOutTitle": {
    message: "ログアウトしています...",
    description: "Logout page heading while the logout request is in flight",
  },
  "auth.loggingOutSubtitle": {
    message: "セッションを終了しています。少々お待ちください。",
    description: "Logout page subtext while the logout request is in flight",
  },
  "auth.loggedOutTitle": {
    message: "ログアウトしました",
    description: "Logout page heading once logout has completed",
  },
  "auth.loggedOutSubtitle": {
    message: "ログインページへ移動しています…",
    description: "Logout page subtext once logout has completed",
  },
  "auth.logoutFailedTitle": {
    message: "ログアウトに失敗しました",
    description: "Logout page heading when the provider logout request failed",
  },
  "auth.logoutFailedSubtitle": {
    message:
      "セッションを終了できませんでした。まだログインしている可能性があります。もう一度お試しください。",
    description: "Logout page subtext when the provider logout request failed",
  },
  "auth.logoutRetry": {
    message: "再試行",
    description: "Logout page retry button after a failed sign-out",
  },
  "auth.logoutConfirmTitle": {
    message: "ログアウトしますか？",
    description:
      "Logout page heading on the confirmation screen (codex #12 CSRF logout fix)",
  },
  "auth.logoutConfirmSubtitle": {
    message: "セッションを終了してもよろしいですか？",
    description: "Logout page subtext on the confirmation screen",
  },
  "auth.logoutConfirm": {
    message: "ログアウト",
    description: "Logout page confirm button that triggers the actual logout",
  },
  "auth.logoutCancel": {
    message: "キャンセル",
    description: "Logout page cancel button that returns to the dashboard",
  },
  "auth.logoutTitle": {
    message: "ログアウト",
    description: "Logout page document title",
  },
  "auth.featureSecureTitle": {
    message: "デフォルトで安全",
    description: "Auth hero feature bullet title",
  },
  "auth.featureSecureDescription": {
    message:
      "セッションは Ory Kratos が管理します。自前で作った認証システムではなく、実績のある ID 基盤です。",
    description: "Auth hero feature bullet description",
  },
  "auth.featureDashboardTitle": {
    message: "すべてのサービスを 1 つのダッシュボードで",
    description: "Auth hero feature bullet title",
  },
  "auth.featureDashboardDescription": {
    message:
      "bex 上で動くあらゆるものを、1 か所からデプロイ、監視、管理できます。",
    description: "Auth hero feature bullet description",
  },
  "auth.featureOpenSourceTitle": {
    message: "オープンに開発",
    description: "Auth hero feature bullet title",
  },
  "auth.featureOpenSourceDescription": {
    message: "bex は、オープンソースの Render の代替です。",
    description: "Auth hero feature bullet description",
  },
  "auth.consentTitle": {
    message: "アクセスを承認",
    description: "OAuth2 consent page hero title",
  },
  "auth.consentSubtitle": {
    message: "{client} が bex でユーザーに代わって操作することを求めています。",
    description: "OAuth2 consent page hero subtitle, names the OAuth client",
  },
  "auth.consentUnverifiedClient": {
    message: "未確認のサードパーティアプリ",
    description:
      "Warning label for an OAuth client whose self-provided branding is not trusted",
  },
  "auth.consentClientId": {
    message: "クライアント ID",
    description: "Label for the immutable OAuth client identifier",
  },
  "auth.consentRedirectOrigin": {
    message: "認可コードの送信先",
    description: "Label for the origin that will receive the OAuth code",
  },
  "auth.consentNoRedirectOrigin": {
    message: "リダイレクトなし（デバイスフロー）",
    description: "Shown when an OAuth device flow has no redirect origin",
  },
  "auth.consentScopesTitle": {
    message: "要求している内容",
    description: "Heading above the list of requested OAuth scopes",
  },
  "auth.consentScopeOpenid": {
    message: "あなたが誰であるかを確認する",
    description: "Human description of the openid scope",
  },
  "auth.consentScopeOfflineAccess": {
    message: "再度確認せずに接続を維持する（リフレッシュトークン）",
    description: "Human description of the offline_access scope",
  },
  "auth.consentScopeProfile": {
    message: "基本プロフィールを読み取る",
    description: "Human description of the profile scope",
  },
  "auth.consentScopeEmail": {
    message: "メールアドレスを読み取る",
    description: "Human description of the email scope",
  },
  "auth.consentScopeRead": {
    message:
      "通常のワークスペースのリソース（サービス、ログ、メトリクス）を読み取る",
    description: "Human description of the bex.read OAuth capability",
  },
  "auth.consentScopeWrite": {
    message:
      "ワークスペースのリソースを変更する（デプロイ、再起動、作成、削除、請求）",
    description: "Human description of the bex.write OAuth capability",
  },
  "auth.consentScopeSensitive": {
    message:
      "シークレットと接続文字列（環境変数、データベース URL、ファイル）を読み取る",
    description: "Human description of the bex.sensitive OAuth capability",
  },
  "auth.consentScopeCloudAgents": {
    message:
      "あなたに代わってクラウドのコーディングエージェントを実行する（コンピュートとモデルのクレジットを使うタスクを開始し、ブランチをプッシュすることがあります）",
    description: "Human description of the bex.cloud_agents OAuth capability",
  },
  "auth.consentScopeApiCompat": {
    message:
      "コントロールプレーンへの完全なアクセス（従来のプラットフォームクライアントとの互換性）",
    description: "Human description of the bex.api compatibility scope",
  },
  "auth.consentAudienceTitle": {
    message: "トークンが有効な範囲",
    description: "Heading above the requested access-token audience",
  },
  "auth.consentRememberHint": {
    message:
      "承認は 1 時間記憶されます。その後は、このエージェントが再度確認します。",
    description: "Consent page footnote about the remember window",
  },
  "auth.consentApprove": {
    message: "承認",
    description: "OAuth2 consent page approve button",
  },
  "auth.consentDeny": {
    message: "拒否",
    description: "OAuth2 consent page deny button",
  },
  "auth.consentFailed": {
    message: "処理を完了できませんでした。もう一度お試しください。",
    description: "Consent page error banner after a failed accept/reject",
  },
  "auth.consentExpiredTitle": {
    message: "認可の期限が切れました",
    description: "Consent page heading when there is no live consent request",
  },
  "auth.consentExpiredSubtitle": {
    message: "ここに移動する前のアプリから、もう一度接続を開始してください。",
    description:
      "Consent page subtext when there is no live consent request to decide",
  },
  "auth.consentInvalidRequestTitle": {
    message: "無効な認可リクエスト",
    description:
      "Consent page heading when the requesting client's authorization request fails a protocol check (PKCE or scope)",
  },
  "auth.consentPkceRequiredSubtitle": {
    message:
      "このアプリのリクエストには、必要なセキュリティパラメーターが含まれていません。アプリの開発者にお問い合わせください。",
    description:
      "Consent page subtext when the authorization request lacks PKCE with S256",
  },
  "auth.consentScopeRequiredSubtitle": {
    message:
      "このアプリは、必要な内容を指定せずに API アクセスを要求しました。次のうち少なくとも 1 つを要求する必要があります: {scopes}。",
    description:
      "Consent page subtext when an audience request lacks a granular capability scope (round-14 #1); {scopes} is the comma-separated list of accepted capability scopes",
  },
  "auth.consentUnavailableTitle": {
    message: "認可を利用できません",
    description:
      "Consent page heading when the authorization provider is unreachable, misconfigured, or a headless accept failed",
  },
  "auth.consentUnavailableSubtitle": {
    message:
      "まもなく解消されるはずです。しばらくしてから、もう一度お試しください。",
    description:
      "Consent page subtext when the authorization provider is unreachable, misconfigured, or a headless accept failed",
  },
  "auth.consentWrongUserTitle": {
    message: "別のアカウントでログインしています",
    description:
      "Consent page heading when the browser's session belongs to a different account than the one that started this authorization",
  },
  "auth.consentWrongUserSubtitle": {
    message:
      "このブラウザーのセッションは、この認可を開始したアカウントとは別のアカウントのものです。ログアウトして、もう一度お試しください。",
    description:
      "Consent page subtext when the browser's session belongs to a different account than the one that started this authorization",
  },
  "auth.deviceSuccessTitle": {
    message: "デバイスを接続しました",
    description: "Device authorization success page title",
  },
  "auth.deviceTitle": {
    message: "デバイスを接続",
    description: "Device verification route document title",
  },
  "auth.deviceConfirmSubtitle": {
    message:
      "ログイン中のデバイスが、あなたのアカウントへのアクセスを求めています。",
    description: "Device confirm page hero subtitle",
  },
  "auth.deviceConfirmHeading": {
    message: "このデバイスを承認",
    description: "Device confirm page card heading",
  },
  "auth.deviceConfirmDescription": {
    message:
      "下のコードが、デバイスに表示されているコードと一致していることを確認してください:",
    description: "Device confirm page card description above the code",
  },
  "auth.deviceConfirmHint": {
    message: "このログインを自分で開始した場合のみ承認してください。",
    description:
      "Device confirm page footnote warning against unsolicited requests",
  },
  "auth.deviceConfirmCancel": {
    message: "キャンセル",
    description: "Device confirm page cancel button",
  },
  "auth.deviceConfirmAuthorize": {
    message: "デバイスを承認",
    description: "Device confirm page authorize/submit button",
  },
  "auth.deviceExpiredTitle": {
    message: "デバイスコードの期限が切れました",
    description:
      "Device confirm page heading when there is no live device request",
  },
  "auth.deviceExpiredSubtitle": {
    message:
      "このページにはデバイスコードが必要ですが、コードの期限が切れています。デバイスからもう一度ログインを開始してください。",
    description:
      "Device confirm page subtext when there is no live device request to confirm",
  },
  "auth.deviceUnavailableTitle": {
    message: "デバイスの認可を利用できません",
    description:
      "Device confirm page heading when the authorization provider is unreachable or misconfigured",
  },
  "auth.deviceUnavailableSubtitle": {
    message:
      "まもなく解消されるはずです。しばらくしてから、もう一度お試しください。",
    description:
      "Device confirm page subtext when the authorization provider is unreachable or misconfigured",
  },
  "auth.deviceRefusedTitle": {
    message: "認可が拒否されました",
    description:
      "Device confirm page heading when the device request names a client bex did not expect",
  },
  "auth.deviceRefusedSubtitle": {
    message:
      "このデバイスのリクエストを確認できませんでした。デバイスからもう一度ログインを開始してください。",
    description:
      "Device confirm page subtext when the device request names a client bex did not expect",
  },
  "auth.deviceSuccessSubtitle": {
    message: "ブラウザーでの認可が完了しました。",
    description: "Device authorization success page subtitle",
  },
  "auth.deviceSuccessHint": {
    message: "ログイン先のアプリに戻って続行してください。",
    description: "Device authorization success page hint",
  },
  "auth.deviceSuccessWaiting": {
    message: "ブラウザーでの認可を待っています…",
    description:
      "Device success page terminal replica: the CLI line shown while it polls for tokens",
  },
  "auth.deviceSuccessDone": {
    message: "認可が完了しました。ログインしました。",
    description:
      "Device success page terminal replica: the CLI success line (rendered after a ✓ glyph)",
  },
  "auth.deviceSuccessClose": {
    message: "このタブは閉じて問題ありません。",
    description:
      "Device success page: follow-up sentence after the return-to-terminal hint",
  },
  "auth.deviceSuccessDashboard": {
    message: "ダッシュボードを開く",
    description:
      "Device success page: quiet link to the dashboard home for users staying in the browser",
  },
  "auth.profileEmailLabel": {
    message: "メールアドレス",
    description:
      "Account settings profile form: the email field label (the identity schema title stays as Kratos uses it)",
  },
  "auth.profileEmailPlaceholder": {
    message: "メールアドレスを入力",
    description: "Account settings profile form: the email field placeholder",
  },
  "auth.profileNameLabel": {
    message: "名前",
    description:
      "Account settings profile form: the name field label (the identity schema title stays as Kratos uses it)",
  },
  "auth.profileNamePlaceholder": {
    message: "名前を入力",
    description: "Account settings profile form: the name field placeholder",
  },
  "auth.inputPlaceholder": {
    message: "{placeholder}を入力",
    description:
      "Auth forms: the placeholder of a field with no wording of its own, e.g. the password; {placeholder} is the field's label",
  },
  "auth.showPassword": {
    message: "パスワードを表示",
    description:
      "Accessible name of the password field toggle while the password is hidden",
  },
  "auth.hidePassword": {
    message: "パスワードを非表示",
    description:
      "Accessible name of the password field toggle while the password is shown",
  },
  "auth.ssoMoreOptions": {
    message: "その他のオプション",
    description:
      "Login and sign-up: reveals the GitLab and Bitbucket sign-in buttons, as Render's More options does",
  },
  "auth.ssoContinueWith": {
    message: "または次の方法で続行",
    description:
      "Login and sign-up: the divider between the email form and the social sign-in buttons below it, as Render's Or continue with",
  },
  "auth.settingsGuardEmailChangeRefused": {
    message:
      "Google ログインが必須のワークスペースに所属している間は、メールアドレスを変更できません。先に次のワークスペースから退出してください: {workspaces}。",
    description:
      "Account settings refusal of an email change (Kratos message 4100001) while the member belongs to a workspace that requires Google login; {workspaces} is the list of those workspaces (w7/m465)",
  },
  "auth.settingsGuardUnlinkRefused": {
    message:
      "Google ログインが必須のワークスペースに所属している間は、Google アカウントの連携を解除できません。先に次のワークスペースから退出してください: {workspaces}。",
    description:
      "Account settings refusal of disconnecting the Google account, or an unlink that does not name its provider (Kratos message 4100002), while the member belongs to a workspace that requires Google login; {workspaces} is the list of those workspaces (w7/m465, w7/085)",
  },
};

export default jaAuth;
