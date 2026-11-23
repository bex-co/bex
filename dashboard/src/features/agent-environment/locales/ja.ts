import type { TranslationEntry } from "@/i18n";

const jaAgentEnvironment: Record<string, TranslationEntry> = {
  "agentEnvironment.title": {
    message: "エージェント",
    description: "Settings → Agents card title and section-nav label",
  },
  "agentEnvironment.description": {
    message:
      "このワークスペースのエージェントセッションがネットワークにアクセスする方法と、使用するモデルを設定します。新しいセッションすべてに適用されます。",
    description: "Agents settings card description",
  },
  "agentEnvironment.loadFailed": {
    message: "エージェント設定を読み込めませんでした。",
    description: "Agents card error when the environment query fails",
  },
  "agentEnvironment.retry": {
    message: "再試行",
    description: "Retry button on the Agents card load error",
  },
  "agentEnvironment.readOnlyNote": {
    message:
      "エージェント設定を変更できるのは、ワークスペースの管理者のみです。",
    description: "Read-only notice shown to non-admin members",
  },
  "agentEnvironment.networkTitle": {
    message: "ネットワークアクセス",
    description: "Heading (and radiogroup label) of the network tier picker",
  },
  "agentEnvironment.networkDescription": {
    message:
      "GitHub とモデルプロバイダーには常にアクセスできます。エージェントのサンドボックスがそれ以外にアクセスできる範囲を選択してください。",
    description: "Network tier picker description",
  },
  "agentEnvironment.tierRegistriesTitle": {
    message: "パッケージレジストリ（推奨）",
    description: "Network tier option: built-in package registries",
  },
  "agentEnvironment.tierRegistriesDescription": {
    message:
      "エージェントは npm、PyPI、Go、Maven、crates、RubyGems、Docker Hub から依存関係をインストールできるため、コードのビルドとテストが可能です。",
    description: "Description of the registries network tier",
  },
  "agentEnvironment.tierCustomTitle": {
    message: "カスタム",
    description: "Network tier option: registries plus listed hosts",
  },
  "agentEnvironment.tierCustomDescription": {
    message:
      "パッケージレジストリに加えて、下に指定したホスト名にアクセスできます。",
    description: "Description of the custom network tier",
  },
  "agentEnvironment.tierFullTitle": {
    message: "インターネット全体",
    description: "Network tier option: any public host over HTTP(S)",
  },
  "agentEnvironment.tierFullDescription": {
    message:
      "bex の下り通信プロキシを経由して、HTTP(S) で任意の公開ウェブサイトにアクセスできます。モデルプロバイダー API への直接呼び出しはブロックされたままです（エージェント自身のモデルは引き続き動作します）。SSH などの HTTP 以外のプロトコルは利用できません。",
    description:
      "Description of the full-internet tier: HTTP(S) via the egress proxy, direct model-provider API calls blocked, no SSH/non-HTTP",
  },
  "agentEnvironment.registryHostsSummary_other": {
    message: "含まれるパッケージレジストリ（{count} ホスト）",
    description:
      "Disclosure summary listing the built-in registry hosts (many)",
  },
  "agentEnvironment.hostsLabel": {
    message: "追加のホスト名",
    description: "Label of the custom-tier hostname textarea",
  },
  "agentEnvironment.hostsHelp": {
    message:
      "1 行に 1 つのホスト名、最大 {max} 件。完全一致のホスト名のみ（スキーム、ポート、パス、ワイルドカードは使用できません）。",
    description: "Help under the custom hostname textarea",
  },
  "agentEnvironment.hostsError.too_many_entries": {
    message: "ホスト名は最大 {max} 件まで指定できます。",
    description: "Custom hosts error: too many entries",
  },
  "agentEnvironment.hostsError.empty": {
    message: "ホスト名を空にすることはできません。",
    description: "Custom hosts error: empty entry",
  },
  "agentEnvironment.hostsError.whitespace": {
    message: "「{entry}」の前後に空白があります。",
    description: "Custom hosts error: surrounding whitespace",
  },
  "agentEnvironment.hostsError.not_a_hostname": {
    message:
      "「{entry}」は完全一致のホスト名ではありません。スキーム、ポート、パス、ワイルドカードを含まない api.example.com のような名前を使用してください。",
    description: "Custom hosts error: not a hostname",
  },
  "agentEnvironment.hostsError.ip_literal": {
    message:
      "「{entry}」は IP アドレスです。代わりにホスト名を使用してください。",
    description: "Custom hosts error: IP literal",
  },
  "agentEnvironment.hostsError.private_name": {
    message:
      "「{entry}」はプライベート名またはクラスターローカル名のため、許可できません。",
    description: "Custom hosts error: private name",
  },
  "agentEnvironment.hostsError.already_baseline": {
    message:
      "「{entry}」はすでにアクセス可能です（またはモデルプロキシ経由でのみアクセス可能です）。リストから削除してください。",
    description: "Custom hosts error: already in the baseline",
  },
  "agentEnvironment.hostsError.duplicate": {
    message: "「{entry}」が複数回指定されています。",
    description: "Custom hosts error: duplicate",
  },
  "agentEnvironment.hostsError.invalid": {
    message: "ホスト名リストが無効です。",
    description: "Custom hosts error: unknown reason",
  },
  "agentEnvironment.modelLabel": {
    message: "デフォルトモデル",
    description: "Label of the default model field",
  },
  "agentEnvironment.modelPlaceholder": {
    message: "プロバイダーのデフォルト",
    description: "Placeholder of the empty default model field",
  },
  "agentEnvironment.modelHelp": {
    message:
      "新しいセッションが使用するモデル ID（claude-sonnet-4-5 など）。空のままにすると、プロバイダーのデフォルトが使われます。",
    description: "Help under the default model field",
  },
  "agentEnvironment.modelPlatformDefault": {
    message: "プラットフォームのデフォルト（{model}）",
    description:
      "Default model picker — no default: the platform catalog's first model (w10/m516)",
  },
  "agentEnvironment.modelCatalogHelp": {
    message:
      "モデルを指定しない新しいセッションと自動化はこのモデルを使用します。メンバーはセッションごとにプラットフォームカタログの別のモデルを選ぶこともできます。",
    description: "Help under the platform model catalog picker (w10/m516)",
  },
  "agentEnvironment.modelInvalid": {
    message: "空白を含まないモデル ID を使用してください（255 文字以内）。",
    description: "Default model validation error",
  },
  "agentEnvironment.modelNotOfferedError": {
    message:
      "プラットフォームモデルでは {model} を利用できません。一覧から選んでください。",
    description:
      "Server refusal: the default model is not in the platform catalog (w10/m516)",
  },
  "agentEnvironment.templateLabel": {
    message: "デフォルトのサンドボックス",
    description: "Label of the default sandbox template picker (w10/m519)",
  },
  "agentEnvironment.templateHelp": {
    message:
      "サンドボックスを指定しない新しいセッションと自動化はこれで実行されます。以降に開始したセッションに適用され、メンバーはセッションごとに別のものを選ぶこともできます。",
    description: "Help under the default sandbox template picker (w10/m519)",
  },
  "agentEnvironment.templateInvalidError": {
    message:
      "{template} はサンドボックスのテンプレートではありません。次のいずれかを選んでください: {templates}",
    description:
      "Server refusal: the default sandbox template is not a registered one; {templates} lists those (w10/m519)",
  },
  "agentEnvironment.tierInvalid": {
    message: "そのネットワークアクセスのオプションはサポートされていません。",
    description: "Server refusal: invalid tier",
  },
  "agentEnvironment.forbidden": {
    message:
      "エージェント設定を変更できるのは、ワークスペースの管理者のみです。",
    description: "Server refusal: caller is not an admin",
  },
  "agentEnvironment.saveFailed": {
    message: "エージェント設定を保存できませんでした。もう一度お試しください。",
    description: "Generic save failure",
  },
  "agentEnvironment.saveFailedReason": {
    message: "エージェント設定を保存できませんでした: {reason}",
    description: "Save failure with the server's reason",
  },
  "agentEnvironment.save": {
    message: "保存",
    description: "Agents card save button",
  },
  "agentEnvironment.saved": {
    message: "エージェント設定を保存しました",
    description: "Success toast after saving",
  },
  "agentEnvironment.modelKeyTitle": {
    message: "モデルプロバイダー",
    description: "Heading of the model source / provider key section",
  },
  "agentEnvironment.modelKeyDescription": {
    message:
      "ワークスペースが独自のプロバイダーキーを追加しない限り、エージェントセッションは bex が提供するモデルを使用します。独自のキーを使うと、セッションはプロバイダーを直接呼び出し、bex が請求するのはサンドボックスのコンピュートのみです。",
    description: "Explains the model source choice (ADR091 D7)",
  },
  "agentEnvironment.modelSourceLabel": {
    message: "モデルの提供元:",
    description: "Label before the current model source",
  },
  "agentEnvironment.modelSourcePlatform": {
    message: "bex 提供（モデルプロバイダーの原価 + 30% で請求）",
    description: "Model source: the platform model",
  },
  "agentEnvironment.modelSourceByo": {
    message: "独自のキー（bex からのトークン課金なし）",
    description: "Model source: the workspace's own key",
  },
  "agentEnvironment.modelSourceNone": {
    message:
      "なし: エージェントセッションを開始するにはプロバイダーキーを追加してください",
    description: "Model source: no key available",
  },
  "agentEnvironment.modelKeyLabel": {
    message: "プロバイダーの API キー",
    description: "Label of the write-only key field when none is set",
  },
  "agentEnvironment.modelKeyReplaceLabel": {
    message: "プロバイダーの API キーを置き換え",
    description: "Label of the write-only key field when a key is set",
  },
  "agentEnvironment.modelKeyPlaceholder": {
    message: "sk-ant-…",
    description: "Placeholder of the key field when none is set",
  },
  "agentEnvironment.modelKeyPlaceholderSet": {
    message: "キーは保存済みです。置き換えるには新しいキーを入力してください",
    description:
      "Placeholder of the key field when a key is set (the key is never shown)",
  },
  "agentEnvironment.modelKeyHelp": {
    message:
      "書き込み専用: キーは安全に保存され、再表示されることはありません。",
    description: "Help under the key field",
  },
  "agentEnvironment.modelKeySave": {
    message: "キーを保存",
    description: "Button that saves the model key",
  },
  "agentEnvironment.modelKeyClear": {
    message: "キーを削除",
    description: "Button that opens the remove-key confirmation",
  },
  "agentEnvironment.modelKeyClearTitle": {
    message: "モデルプロバイダーのキーを削除しますか？",
    description: "Remove-key confirmation title",
  },
  "agentEnvironment.modelKeyClearBody": {
    message:
      "エージェントセッションは、次のモデルリクエストからこのキーを使用しなくなります。キーは再表示できないため、復元するには貼り付け直す必要があります。",
    description: "Remove-key confirmation body",
  },
  "agentEnvironment.modelKeyClearConfirm": {
    message: "キーを削除",
    description: "Remove-key confirmation button",
  },
  "agentEnvironment.modelKeySaved": {
    message: "モデルキーを保存しました",
    description: "Toast after saving the key",
  },
  "agentEnvironment.modelKeyCleared": {
    message: "モデルキーを削除しました",
    description: "Toast after removing the key",
  },
  "agentEnvironment.modelKeyInvalid": {
    message: "空白を含まないキーを入力してください（4,096 文字以内）。",
    description: "Client/server refusal of a malformed key",
  },
  "agentEnvironment.modelKeySaveFailed": {
    message: "モデルキーを保存できませんでした。もう一度お試しください。",
    description: "Generic model key write failure",
  },
  "agentEnvironment.spendTitle": {
    message: "エージェントのモデル支出",
    description: "Heading of the agent-model spend section (ADR091 D6)",
  },
  "agentEnvironment.spendPricing": {
    message:
      "モデルプロバイダーの原価 + 30% で請求されます。上限に達すると、今月の残りの期間は新しいモデルリクエストが停止されます。",
    description: "Pricing statement under the spend heading",
  },
  "agentEnvironment.spendSummary": {
    message: "今月 {cap} のうち {used}",
    description: "Month-to-date spend against the effective monthly cap",
  },
  "agentEnvironment.spendSummaryCeiling": {
    message: "請求クレジット超過分 {cap} のうち {used}、今月の合計 {total}",
    description:
      "Spend summary and meter value when the trust tier's ceiling binds on the spend billing credits leave ({used}), which differs from month-to-date ({total})",
  },
  "agentEnvironment.spendMeterValue": {
    message: "{cap} のうち {used}",
    description: "Screen-reader value of the spend progress bar",
  },
  "agentEnvironment.spendUnavailable": {
    message: "現在、支出を取得できません。",
    description: "Shown when the spend fields could not be read",
  },
  "agentEnvironment.spendTierNextCharge": {
    message:
      "信頼ティア {tier}: エージェントのモデル支出には、まず請求クレジットが充当されます。クレジットで賄いきれない支出が {amount} に達すると、カードに請求されます。",
    description: "Trust tier with its mid-month charge point (ADR091 D10.1)",
  },
  "agentEnvironment.spendTierNextChargeCredit": {
    message:
      "信頼ティア {tier}: この請求期間のエージェントのモデル支出のうち、最初の {credit} は請求クレジットで賄われます。それを超える支出が {amount} に達すると、カードに請求されます。",
    description:
      "Trust tier with its charge point when billing credits cover agent model spend first (ADR091 D10.2a). {credit} is the credit counted in this billing period, {amount} the charge point",
  },
  "agentEnvironment.spendTierMonthEnd": {
    message:
      "信頼ティア {tier}: エージェントのモデル支出は月末に請求されます。",
    description: "Top trust tier, charged only at month end",
  },
  "agentEnvironment.spendTierNew": {
    message:
      "新しいワークスペースでは、請求クレジットを超えたエージェントのモデル支出が {amount} に達した時点でカードに請求されます。請求が成功するとセッションは継続して実行され、上限はプランの上限まで引き上げられます。",
    description:
      "Explains T0: charged at its threshold, working while the charge settles (ADR091 D10.2a)",
  },
  "agentEnvironment.spendTierLocked": {
    message:
      "支払いの失敗または異議申し立てにより、このワークスペースは制限されています。制限を解除するには、請求ページで解決してください。",
    description: "Explains a payment-locked trust tier",
  },
  "agentEnvironment.spendCapLabel": {
    message: "月間支出上限（USD）",
    description: "Label of the admin-editable cap field",
  },
  "agentEnvironment.spendCapSave": {
    message: "上限を保存",
    description: "Button that saves the cap",
  },
  "agentEnvironment.spendCapReset": {
    message: "プランのデフォルトに戻す",
    description: "Button that returns the cap to the plan default",
  },
  "agentEnvironment.spendCapHelp": {
    message:
      "プランのデフォルトは {plan} です。管理者は最大 {max} まで設定できます。",
    description: "Help under the cap field",
  },
  "agentEnvironment.spendCapInvalid": {
    message: "$0 より大きく {max} 以下の金額を入力してください。",
    description: "Client/server refusal of a cap value",
  },
  "agentEnvironment.spendCapSaved": {
    message: "支出上限を保存しました",
    description: "Toast after saving the cap",
  },
  "agentEnvironment.spendCapResetDone": {
    message: "支出上限をプランのデフォルトに戻しました",
    description: "Toast after resetting the cap",
  },
  "agentEnvironment.spendCapSaveFailed": {
    message: "支出上限を保存できませんでした。もう一度お試しください。",
    description: "Generic cap write failure",
  },
  "agentEnvironment.secrets.title": {
    message: "エージェントのシークレット",
    description: "Settings → Agent secrets card title (ADR098)",
  },
  "agentEnvironment.secrets.description": {
    message:
      "セッションに環境変数として渡せる API キーやトークンです。値は書き込み専用で、誰も読み戻すことはできず、トランスクリプトでは伏せ字になります。",
    description: "Agent secrets card description",
  },
  "agentEnvironment.secrets.loadFailed": {
    message: "エージェントのシークレットを読み込めませんでした。",
    description: "Agent secrets card error when the list query fails",
  },
  "agentEnvironment.secrets.empty": {
    message: "エージェントのシークレットはまだありません。",
    description: "Agent secrets card empty state",
  },
  "agentEnvironment.secrets.deleteLabel": {
    message: "{name} を削除",
    description: "Accessible label of a secret's delete button",
  },
  "agentEnvironment.secrets.deleteTitle": {
    message: "{name} を削除しますか？",
    description: "Delete-secret confirmation title",
  },
  "agentEnvironment.secrets.deleteBody": {
    message:
      "このシークレットを使用しているセッションは、再設定するまで次のターンが失敗します。",
    description: "Delete-secret confirmation body",
  },
  "agentEnvironment.secrets.deleteConfirm": {
    message: "シークレットを削除",
    description: "Delete-secret confirmation button",
  },
  "agentEnvironment.secrets.nameLabel": {
    message: "シークレット名",
    description: "Agent secrets card: name field label",
  },
  "agentEnvironment.secrets.namePlaceholder": {
    message: "STRIPE_API_KEY",
    description: "Agent secrets card: name field placeholder (an env var name)",
  },
  "agentEnvironment.secrets.valueLabel": {
    message: "シークレットの値",
    description: "Agent secrets card: value field label",
  },
  "agentEnvironment.secrets.valuePlaceholder": {
    message: "値",
    description: "Agent secrets card: value field placeholder",
  },
  "agentEnvironment.secrets.save": {
    message: "保存",
    description: "Agent secrets card: save (create or replace) button",
  },
  "agentEnvironment.secrets.help": {
    message:
      "既存の名前で保存すると、その値が置き換えられます。変更は、セッションの次のターンから反映されます。",
    description: "Agent secrets card help line for admins",
  },
  "agentEnvironment.secrets.readOnly": {
    message:
      "エージェントのシークレットを追加、変更、削除できるのは、ワークスペースの管理者のみです。",
    description: "Agent secrets card help line for non-admins",
  },
  "agentEnvironment.secrets.saved": {
    message: "{name} を保存しました",
    description: "Toast after a secret is saved",
  },
  "agentEnvironment.secrets.deleted": {
    message: "{name} を削除しました",
    description: "Toast after a secret is deleted",
  },
  "agentEnvironment.secrets.nameInvalid": {
    message:
      "英字、数字、_ を使用し、先頭は英字または _ にしてください（128 文字以内）。",
    description: "Secret name validation error",
  },
  "agentEnvironment.secrets.nameReserved": {
    message:
      "この環境変数は bex 自身が設定するため、シークレットの名前には使用できません。",
    description: "Secret name reserved error",
  },
  "agentEnvironment.secrets.valueRequired": {
    message: "値を入力してください。",
    description: "Secret value required error",
  },
  "agentEnvironment.secrets.valueInvalid": {
    message: "値は 32 KiB 以内にしてください。",
    description: "Secret value too long (or contains a NUL byte)",
  },
  "agentEnvironment.secrets.limitExceeded": {
    message: "エージェントのシークレットの上限に達しました。",
    description: "Secret count or size limit error",
  },
  "agentEnvironment.secrets.notFound": {
    message:
      "このシークレットはすでに存在しません。他の誰かが削除した可能性があります。",
    description: "AGENT_SECRET_NOT_FOUND error (deleted concurrently)",
  },
  "agentEnvironment.secrets.unavailable": {
    message:
      "このデプロイメントではエージェントのシークレットを利用できません。",
    description: "AGENT_SECRETS_UNAVAILABLE error",
  },
  "agentEnvironment.secrets.forbidden": {
    message:
      "エージェントのシークレットを変更できるのは、ワークスペースの管理者のみです。",
    description: "Secret write refused for a non-admin",
  },
  "agentEnvironment.secrets.saveFailed": {
    message: "変更を保存できませんでした。もう一度お試しください。",
    description: "Generic secret write failure",
  },
  "agentEnvironment.secrets.valueKeepPlaceholder": {
    message: "現在の値を維持",
    description:
      "Agent secrets card: value placeholder when the name is an existing secret (empty keeps its value)",
  },
  "agentEnvironment.secrets.editLabel": {
    message: "{name} を編集",
    description:
      "Accessible label of a secret's edit button (loads it into the form to rotate or rescope)",
  },
  "agentEnvironment.secrets.scopeLabel": {
    message: "スコープ",
    description:
      "Agent secrets card: accessible label of the scope picker (ADR098 D9)",
  },
  "agentEnvironment.secrets.scopeWorkspace": {
    message: "すべてのセッション",
    description:
      "Scope picker option: every session in the workspace receives the secret",
  },
  "agentEnvironment.secrets.scopeRepo": {
    message: "1 つのリポジトリ",
    description:
      "Scope picker option: sessions on one repository receive the secret",
  },
  "agentEnvironment.secrets.scopeExplicit": {
    message: "選択した場合のみ",
    description:
      "Scope picker option: only sessions that pick the secret receive it",
  },
  "agentEnvironment.secrets.scopeBadgeRepo": {
    message: "リポジトリ: {repo}",
    description:
      "Secret row badge: repo scope, naming the repository (owner/name)",
  },
  "agentEnvironment.secrets.repoLabel": {
    message: "リポジトリ",
    description:
      "Agent secrets card: accessible label of the repo scope's repository field",
  },
  "agentEnvironment.secrets.repoPlaceholder": {
    message: "owner/name",
    description: "Agent secrets card: repository field placeholder",
  },
  "agentEnvironment.secrets.repoEmpty": {
    message: "一致する接続済みリポジトリはありません。",
    description:
      "Agent secrets card: repository picker with no matching connected repository",
  },
  "agentEnvironment.secrets.repoInvalid": {
    message: "リポジトリを owner/name の形式で入力してください。",
    description:
      "Repo scope validation error (also AGENT_SECRET_SCOPE_INVALID)",
  },
  "agentEnvironment.repoEnvs.title": {
    message: "リポジトリ環境",
    description: "Settings → Repository environments card title (ADR109)",
  },
  "agentEnvironment.repoEnvs.description": {
    message:
      "エージェントセッションが各リポジトリをどのように準備するかを設定します。セットアップステップは、各セッションのエージェントが起動する前に、リポジトリのルートから順に、セッションのシークレットを使って実行されます。チェックは、lint やテストなど、エージェントが成果物を提出する前に実行するコマンドです。",
    description:
      "Repository environments card description: what setup steps and checks are",
  },
  "agentEnvironment.repoEnvs.loadFailed": {
    message: "リポジトリ環境を読み込めませんでした。",
    description: "Repository environments card error when the list query fails",
  },
  "agentEnvironment.repoEnvs.emptyList": {
    message: "リポジトリ環境はまだありません。",
    description: "Repository environments card empty state",
  },
  "agentEnvironment.repoEnvs.setupSummary": {
    message: "セットアップ: {names}",
    description:
      "Repository row summary: the setup step names, comma-separated",
  },
  "agentEnvironment.repoEnvs.checksSummary": {
    message: "チェック: {names}",
    description: "Repository row summary: the check names, comma-separated",
  },
  "agentEnvironment.repoEnvs.noSetup": {
    message: "セットアップステップなし",
    description:
      "Repository row summary (and the read-only dialog) when the environment has no setup steps",
  },
  "agentEnvironment.repoEnvs.noChecks": {
    message: "チェックなし",
    description:
      "Repository row summary (and the read-only dialog) when the environment has no checks",
  },
  "agentEnvironment.repoEnvs.editLabel": {
    message: "{repo} を編集",
    description:
      "Accessible label of a repository environment's edit button (repo is owner/name)",
  },
  "agentEnvironment.repoEnvs.viewLabel": {
    message: "{repo} を表示",
    description:
      "Accessible label of a repository environment's view button, for non-admins",
  },
  "agentEnvironment.repoEnvs.deleteLabel": {
    message: "{repo} の環境を削除",
    description: "Accessible label of a repository environment's delete button",
  },
  "agentEnvironment.repoEnvs.deleteTitle": {
    message: "{repo} の環境を削除しますか？",
    description: "Delete-environment confirmation title",
  },
  "agentEnvironment.repoEnvs.deleteBody": {
    message:
      "次のターンから、このリポジトリのセッションはセットアップステップとチェックなしで開始されます。",
    description: "Delete-environment confirmation body",
  },
  "agentEnvironment.repoEnvs.deleteConfirm": {
    message: "環境を削除",
    description: "Delete-environment confirmation button",
  },
  "agentEnvironment.repoEnvs.add": {
    message: "環境を追加",
    description:
      "Repository environments card: button that opens the add-environment dialog",
  },
  "agentEnvironment.repoEnvs.help": {
    message:
      "保存すると、リポジトリの環境全体が置き換えられます。変更は、セッションの次のターンから反映されます。",
    description: "Repository environments card help line for admins",
  },
  "agentEnvironment.repoEnvs.readOnly": {
    message:
      "リポジトリ環境を追加、変更、削除できるのは、ワークスペースの管理者のみです。",
    description: "Repository environments card help line for non-admins",
  },
  "agentEnvironment.repoEnvs.saved": {
    message: "{repo} の環境を保存しました",
    description: "Toast after an environment is saved",
  },
  "agentEnvironment.repoEnvs.deleted": {
    message: "{repo} の環境を削除しました",
    description: "Toast after an environment is deleted",
  },
  "agentEnvironment.repoEnvs.createTitle": {
    message: "リポジトリ環境を追加",
    description: "Add-environment dialog title",
  },
  "agentEnvironment.repoEnvs.editTitle": {
    message: "{repo} を編集",
    description: "Edit-environment dialog title (repo is owner/name)",
  },
  "agentEnvironment.repoEnvs.dialogDescription": {
    message:
      "セットアップステップは、各セッションのエージェントが起動する前に、リポジトリのルートから順に、セッションのシークレットを使って実行されます。ステップが失敗すると、エージェントが起動する前にターンが終了します。チェックは、エージェントが成果物を提出する前に実行するものです。",
    description: "Environment dialog description",
  },
  "agentEnvironment.repoEnvs.repoLabel": {
    message: "リポジトリ",
    description: "Environment dialog: repository field label",
  },
  "agentEnvironment.repoEnvs.repoPlaceholder": {
    message: "owner/name",
    description: "Environment dialog: repository field placeholder",
  },
  "agentEnvironment.repoEnvs.repoEmpty": {
    message: "一致する接続済みリポジトリはありません。",
    description:
      "Environment dialog: repository picker with no matching connected repository",
  },
  "agentEnvironment.repoEnvs.setupTitle": {
    message: "セットアップステップ",
    description: "Environment dialog: setup steps section heading",
  },
  "agentEnvironment.repoEnvs.setupHint": {
    message:
      "ツールと依存関係をインストールします。各ステップは bash で順に実行されます。最大 10 ステップ、各 10 分までです。",
    description: "Environment dialog: setup steps section hint",
  },
  "agentEnvironment.repoEnvs.checksTitle": {
    message: "チェック",
    description: "Environment dialog: checks section heading",
  },
  "agentEnvironment.repoEnvs.checksHint": {
    message:
      "lint、テスト、ビルドなどのコマンドです。エージェントは成果物を提出する前に、リポジトリのルートから各コマンドを実行して結果を報告します。bex 自体は実行しません。最大 10 件です。",
    description: "Environment dialog: checks section hint",
  },
  "agentEnvironment.repoEnvs.addSetup": {
    message: "セットアップステップを追加",
    description: "Environment dialog: add a setup step row",
  },
  "agentEnvironment.repoEnvs.addCheck": {
    message: "チェックを追加",
    description: "Environment dialog: add a check row",
  },
  "agentEnvironment.repoEnvs.setupNameLabel": {
    message: "セットアップステップ {position} の名前",
    description:
      "Accessible label of a setup step's name field (position is its 1-based place in the list)",
  },
  "agentEnvironment.repoEnvs.setupRunLabel": {
    message: "セットアップステップ {position} のコマンド",
    description: "Accessible label of a setup step's command field",
  },
  "agentEnvironment.repoEnvs.setupMoveUp": {
    message: "セットアップステップ {position} を上へ移動",
    description: "Accessible label: move a setup step one place earlier",
  },
  "agentEnvironment.repoEnvs.setupMoveDown": {
    message: "セットアップステップ {position} を下へ移動",
    description: "Accessible label: move a setup step one place later",
  },
  "agentEnvironment.repoEnvs.setupRemove": {
    message: "セットアップステップ {position} を削除",
    description: "Accessible label: remove a setup step row",
  },
  "agentEnvironment.repoEnvs.checkNameLabel": {
    message: "チェック {position} の名前",
    description:
      "Accessible label of a check's name field (position is its 1-based place in the list)",
  },
  "agentEnvironment.repoEnvs.checkRunLabel": {
    message: "チェック {position} のコマンド",
    description: "Accessible label of a check's command field",
  },
  "agentEnvironment.repoEnvs.checkMoveUp": {
    message: "チェック {position} を上へ移動",
    description: "Accessible label: move a check one place earlier",
  },
  "agentEnvironment.repoEnvs.checkMoveDown": {
    message: "チェック {position} を下へ移動",
    description: "Accessible label: move a check one place later",
  },
  "agentEnvironment.repoEnvs.checkRemove": {
    message: "チェック {position} を削除",
    description: "Accessible label: remove a check row",
  },
  "agentEnvironment.repoEnvs.setupNamePlaceholder": {
    message: "deps",
    description: "Setup step name placeholder (an example step name)",
  },
  "agentEnvironment.repoEnvs.setupRunPlaceholder": {
    message: "npm ci",
    description: "Setup step command placeholder (an example shell command)",
  },
  "agentEnvironment.repoEnvs.checkNamePlaceholder": {
    message: "test",
    description: "Check name placeholder (an example check name)",
  },
  "agentEnvironment.repoEnvs.checkRunPlaceholder": {
    message: "npm test",
    description: "Check command placeholder (an example shell command)",
  },
  "agentEnvironment.repoEnvs.close": {
    message: "閉じる",
    description: "Read-only environment dialog: close button",
  },
  "agentEnvironment.repoEnvs.cancel": {
    message: "キャンセル",
    description: "Environment dialog: cancel button",
  },
  "agentEnvironment.repoEnvs.save": {
    message: "保存",
    description: "Environment dialog: save (create or replace) button",
  },
  "agentEnvironment.repoEnvs.nameRequired": {
    message: "名前を入力してください。",
    description: "Step or check name required error",
  },
  "agentEnvironment.repoEnvs.nameInvalid": {
    message:
      "名前は英字または数字で始め、英字、数字、スペース、「.」、「_」、「-」を使用してください（64 文字以内）。",
    description: "Step or check name format error",
  },
  "agentEnvironment.repoEnvs.nameDuplicate": {
    message: "このリストの別の行がすでにこの名前を使用しています。",
    description: "Step or check name repeated within its list",
  },
  "agentEnvironment.repoEnvs.runRequired": {
    message: "コマンドを入力してください。",
    description: "Step or check command required error",
  },
  "agentEnvironment.repoEnvs.setupRunTooLong": {
    message: "セットアップのコマンドは 8 KiB 以内にしてください。",
    description: "Setup step command too long (or contains a NUL byte)",
  },
  "agentEnvironment.repoEnvs.checkRunTooLong": {
    message: "チェックのコマンドは 2 KiB 以内にしてください。",
    description: "Check command too long (or contains a NUL byte)",
  },
  "agentEnvironment.repoEnvs.setupTooMany": {
    message: "1 つの環境に設定できるセットアップステップは最大 10 個です。",
    description: "Too many setup steps",
  },
  "agentEnvironment.repoEnvs.checksTooMany": {
    message: "1 つの環境に設定できるチェックは最大 10 件です。",
    description: "Too many checks",
  },
  "agentEnvironment.repoEnvs.environmentEmpty": {
    message:
      "セットアップステップまたはチェックを 1 つ以上追加してください。すべて削除する場合は、代わりに環境を削除してください。",
    description: "Environment with neither setup steps nor checks",
  },
  "agentEnvironment.repoEnvs.repoInvalid": {
    message: "リポジトリを owner/name の形式で入力してください。",
    description:
      "Repository validation error (also AGENT_REPO_ENVIRONMENT_INVALID on the repository)",
  },
  "agentEnvironment.repoEnvs.repoExists": {
    message:
      "このリポジトリにはすでに環境があります。代わりにリストから編集してください。",
    description: "Adding an environment for a repository that already has one",
  },
  "agentEnvironment.repoEnvs.setupRefused": {
    message:
      "セットアップステップが拒否されました。各名前が有効で重複がなく、各コマンドが 8 KiB 以内であることを確認してください。",
    description: "AGENT_REPO_ENVIRONMENT_INVALID on the setup steps",
  },
  "agentEnvironment.repoEnvs.checksRefused": {
    message:
      "チェックが拒否されました。各名前が有効で重複がなく、各コマンドが 2 KiB 以内であることを確認してください。",
    description: "AGENT_REPO_ENVIRONMENT_INVALID on the checks",
  },
  "agentEnvironment.repoEnvs.invalid": {
    message:
      "この環境は拒否されました。リポジトリ、セットアップステップ、チェックを確認してください。",
    description: "AGENT_REPO_ENVIRONMENT_INVALID without a recognized field",
  },
  "agentEnvironment.repoEnvs.limitExceeded": {
    message:
      "このワークスペースはリポジトリ環境の上限である 50 件に達しました。追加するには、いずれかを削除してください。",
    description:
      "AGENT_REPO_ENVIRONMENT_LIMIT_EXCEEDED (also checked before saving)",
  },
  "agentEnvironment.repoEnvs.notFound": {
    message:
      "このリポジトリにはすでに環境がありません。他の誰かが削除した可能性があります。",
    description:
      "AGENT_REPO_ENVIRONMENT_NOT_FOUND error (deleted concurrently)",
  },
  "agentEnvironment.repoEnvs.unavailable": {
    message: "このデプロイメントではリポジトリ環境を利用できません。",
    description: "AGENT_REPO_ENVIRONMENTS_UNAVAILABLE error",
  },
  "agentEnvironment.repoEnvs.forbidden": {
    message: "リポジトリ環境を変更できるのは、ワークスペースの管理者のみです。",
    description: "Environment write refused for a non-admin",
  },
  "agentEnvironment.repoEnvs.saveFailed": {
    message: "変更を保存できませんでした。もう一度お試しください。",
    description: "Generic environment write failure",
  },
  "agentEnvironment.repoEnvs.builds.status.queued": {
    message: "待機中",
    description: "Build status badge: waiting to start (ADR111 D2)",
  },
  "agentEnvironment.repoEnvs.builds.status.building": {
    message: "ビルド中",
    description: "Build status badge: the setup steps are running",
  },
  "agentEnvironment.repoEnvs.builds.status.success": {
    message: "準備完了",
    description:
      "Build status badge: success — sessions can start from this snapshot",
  },
  "agentEnvironment.repoEnvs.builds.status.failed": {
    message: "失敗",
    description: "Build status badge: the build failed",
  },
  "agentEnvironment.repoEnvs.builds.status.cancelled": {
    message: "キャンセル済み",
    description:
      "Build status badge: a newer build replaced it, or its environment was deleted",
  },
  "agentEnvironment.repoEnvs.builds.trigger.environmentSaved": {
    message: "セットアップ手順の変更",
    description:
      "What queued a build: the environment was saved with changed setup steps",
  },
  "agentEnvironment.repoEnvs.builds.trigger.secretChanged": {
    message: "リポジトリのシークレットの変更",
    description: "What queued a build: a repo-scoped secret was set or changed",
  },
  "agentEnvironment.repoEnvs.builds.trigger.manual": {
    message: "手動ビルド",
    description:
      "What queued a build: an admin clicked Build now (or used the API)",
  },
  "agentEnvironment.repoEnvs.builds.trigger.refresh": {
    message: "毎日の更新",
    description: "What queued a build: the daily rebuild",
  },
  "agentEnvironment.repoEnvs.builds.failure.setupFailed": {
    message: "セットアップ手順の 1 つが失敗しました。",
    description:
      "Failed build, failureCode setup_failed (the reason below names the step and exit code)",
  },
  "agentEnvironment.repoEnvs.builds.failure.stepTimeout": {
    message: "セットアップ手順の 1 つが 3 時間の制限を超えました。",
    description: "Failed build, failureCode step_timeout",
  },
  "agentEnvironment.repoEnvs.builds.failure.buildTimeout": {
    message: "ビルドが 3 時間の制限を超えました。",
    description: "Failed build, failureCode build_timeout",
  },
  "agentEnvironment.repoEnvs.builds.failure.checkoutFailed": {
    message: "リポジトリのデフォルトブランチをチェックアウトできませんでした。",
    description: "Failed build, failureCode checkout_failed",
  },
  "agentEnvironment.repoEnvs.builds.failure.storageLimitReached": {
    message:
      "スナップショットがこのワークスペースのサンドボックスストレージ上限を超えてしまいます。",
    description: "Failed build, failureCode storage_limit_reached",
  },
  "agentEnvironment.repoEnvs.builds.failure.internalError": {
    message: "bex 側でビルドを完了できませんでした。もう一度お試しください。",
    description: "Failed build, failureCode internal_error",
  },
  "agentEnvironment.repoEnvs.builds.failure.unknown": {
    message: "ビルドに失敗しました。",
    description: "Failed build with no recognized failureCode",
  },
  "agentEnvironment.repoEnvs.builds.title": {
    message: "{repo} のビルド",
    description: "Build log dialog title (repo is owner/name)",
  },
  "agentEnvironment.repoEnvs.builds.openLabel": {
    message: "{repo} のビルド",
    description:
      "Accessible label of a repository row's button that opens its builds and logs",
  },
  "agentEnvironment.repoEnvs.builds.description": {
    message:
      "セッションは、セットアップ手順が現在のものと一致する最新の成功したビルドから開始されます。一致するビルドがない場合は、代わりに各ターンの開始時にセットアップ手順を実行します。",
    description:
      "Build log dialog description: how sessions use snapshots and the fallback (ADR111 D4)",
  },
  "agentEnvironment.repoEnvs.builds.buildNow": {
    message: "今すぐビルド",
    description:
      "Button that queues a build of the repository's setup steps (admins only)",
  },
  "agentEnvironment.repoEnvs.builds.snapshotNote": {
    message:
      "セットアップ手順が書き込んだファイルはスナップショットに保存されるため、認証情報をファイルに書き込まないでください。シークレットは環境変数として手順に渡され、保存されません。",
    description: "Note beside Build now (Devin's warning, ADR111 D6)",
  },
  "agentEnvironment.repoEnvs.builds.readOnly": {
    message: "ビルドを開始できるのはワークスペースの管理者のみです。",
    description: "Shown under a disabled Build now for non-admins",
  },
  "agentEnvironment.repoEnvs.builds.forbidden": {
    message: "ビルドを開始できるのはワークスペースの管理者のみです。",
    description: "Build now refused with FORBIDDEN",
  },
  "agentEnvironment.repoEnvs.builds.nothingToBuild": {
    message:
      "この環境にはセットアップ手順がないため、ビルドするものがありません。チェックにビルドは不要です。",
    description:
      "Build now on a checks-only environment (also AGENT_REPO_ENVIRONMENT_NOTHING_TO_BUILD)",
  },
  "agentEnvironment.repoEnvs.builds.queued": {
    message: "{repo} のビルドをキューに追加しました",
    description: "Toast after Build now succeeds",
  },
  "agentEnvironment.repoEnvs.builds.rateLimited": {
    message:
      "このリポジトリは過去 24 時間にすでに 12 回ビルドを開始しています。しばらくしてからもう一度お試しください。",
    description:
      "AGENT_REPO_ENVIRONMENT_BUILD_RATE_LIMITED without a usable nextBuildAt",
  },
  "agentEnvironment.repoEnvs.builds.rateLimitedUntil": {
    message:
      "このリポジトリは過去 24 時間にすでに 12 回ビルドを開始しています。次のビルドは {time} に開始できます。",
    description:
      "AGENT_REPO_ENVIRONMENT_BUILD_RATE_LIMITED; time is nextBuildAt in the viewer's local date and time",
  },
  "agentEnvironment.repoEnvs.builds.unavailable": {
    message: "このデプロイ環境では環境ビルドを利用できません。",
    description: "AGENT_REPO_ENVIRONMENT_BUILDS_UNAVAILABLE",
  },
  "agentEnvironment.repoEnvs.builds.buildFailed": {
    message: "ビルドを開始できませんでした。もう一度お試しください。",
    description: "Generic Build now failure",
  },
  "agentEnvironment.repoEnvs.builds.loadFailed": {
    message: "ビルドを読み込めませんでした。",
    description: "Build history query failed",
  },
  "agentEnvironment.repoEnvs.builds.none": {
    message: "ビルドはまだありません。",
    description: "Build history list with no builds",
  },
  "agentEnvironment.repoEnvs.builds.noneLog": {
    message:
      "このリポジトリはまだビルドされていません。「今すぐビルド」はセットアップ手順を 1 回実行し、結果をスナップショットとして保存します。",
    description: "Build log panel when the repository has no builds",
  },
  "agentEnvironment.repoEnvs.builds.historyLabel": {
    message: "ビルド履歴",
    description: "Accessible label of the build history list",
  },
  "agentEnvironment.repoEnvs.builds.logTitle": {
    message: "ビルドログ",
    description: "Build log panel header when no build is selected",
  },
  "agentEnvironment.repoEnvs.builds.logFailed": {
    message: "このビルドのログを読み込めませんでした。",
    description: "Single build (log) query failed",
  },
  "agentEnvironment.repoEnvs.builds.took": {
    message: "所要時間 {duration}",
    description:
      "A finished build's wall time, e.g. 'took 3m 12s' (joined with ' · ')",
  },
  "agentEnvironment.repoEnvs.builds.imageRemoved": {
    message: "スナップショットは削除されました",
    description:
      "A successful build whose snapshot image was since deleted (older builds keep no image)",
  },
  "agentEnvironment.repoEnvs.builds.notBuilt": {
    message: "未ビルド",
    description: "Repository row: has setup steps but no build yet",
  },
  "agentEnvironment.repoEnvs.builds.noSteps.queued": {
    message:
      "このビルドは開始を待っています。ビルドはワークスペースのエージェントサンドボックス上限をセッションと共有します。",
    description: "Build log of a queued build",
  },
  "agentEnvironment.repoEnvs.builds.noSteps.building": {
    message:
      "セットアップ手順を実行中です。ビルドが完了すると、そのログがここに表示されます。",
    description: "Build log of a running build",
  },
  "agentEnvironment.repoEnvs.builds.noSteps.cancelled": {
    message:
      "このビルドはキャンセルされました。新しいビルドに置き換えられたか、環境が削除されました。",
    description: "Build log of a cancelled build",
  },
  "agentEnvironment.repoEnvs.builds.noSteps.failed": {
    message: "実行されたセットアップ手順はありません。",
    description:
      "Build log of a build that failed before any step ran (e.g. the checkout failed)",
  },
  "agentEnvironment.repoEnvs.builds.noSteps.other": {
    message: "このビルドには記録された手順がありません。",
    description: "Build log with no steps, any other status",
  },
  "agentEnvironment.repoEnvs.builds.stepExit": {
    message: "終了コード {code}",
    description: "A setup step's exit code badge",
  },
  "agentEnvironment.repoEnvs.builds.stepKilled": {
    message: "停止",
    description: "A setup step with no exit code: killed or never started",
  },
  "agentEnvironment.repoEnvs.builds.noOutput": {
    message: "出力なし。",
    description: "A setup step that printed nothing",
  },
  "agentEnvironment.skills.title": {
    message: "エージェントのスキル",
    description: "Settings → Agent skills card title (ADR099)",
  },
  "agentEnvironment.skills.description": {
    message:
      "このワークスペースのすべてのエージェントセッションが SKILL.md ファイルとして受け取る、再利用可能な指示です。エージェントは、タスクに合う場合、または依頼されたときのみスキルを適用します。",
    description: "Agent skills card description",
  },
  "agentEnvironment.skills.loadFailed": {
    message: "エージェントのスキルを読み込めませんでした。",
    description: "Agent skills card error when the list query fails",
  },
  "agentEnvironment.skills.empty": {
    message: "エージェントのスキルはまだありません。",
    description: "Agent skills card empty state",
  },
  "agentEnvironment.skills.onlyWhenAsked": {
    message: "依頼時のみ",
    description:
      "Badge on a skill the agent applies only when a prompt invokes it",
  },
  "agentEnvironment.skills.enabledLabel": {
    message: "{name} を有効にする",
    description: "Accessible label of a skill's enabled switch",
  },
  "agentEnvironment.skills.editLabel": {
    message: "{name} を編集",
    description: "Accessible label of a skill's edit button",
  },
  "agentEnvironment.skills.viewLabel": {
    message: "{name} を表示",
    description: "Accessible label of a skill's view button (non-admins)",
  },
  "agentEnvironment.skills.exportLabel": {
    message: "{name} を SKILL.md としてダウンロード",
    description: "Accessible label of a skill's export button",
  },
  "agentEnvironment.skills.deleteLabel": {
    message: "{name} を削除",
    description: "Accessible label of a skill's delete button",
  },
  "agentEnvironment.skills.deleteTitle": {
    message: "スキル {name} を削除しますか？",
    description: "Delete skill confirm title",
  },
  "agentEnvironment.skills.deleteBody": {
    message:
      "新しいエージェントのターンでは、このスキルを受け取らなくなります。すでに実行されたセッションのトランスクリプトは保持されます。",
    description: "Delete skill confirm body",
  },
  "agentEnvironment.skills.deleteConfirm": {
    message: "スキルを削除",
    description: "Delete skill confirm button",
  },
  "agentEnvironment.skills.new": {
    message: "新しいスキル",
    description: "Button that opens the create-skill dialog",
  },
  "agentEnvironment.skills.import": {
    message: "SKILL.md をインポート",
    description: "Button that imports a SKILL.md file",
  },
  "agentEnvironment.skills.help": {
    message:
      "メンバーはスキルを使用できます。追加や変更ができるのは管理者のみです。",
    description: "Agent skills help line for admins",
  },
  "agentEnvironment.skills.readOnly": {
    message:
      "エージェントのスキルを追加または変更できるのは、ワークスペースの管理者のみです。",
    description: "Agent skills help line for non-admins",
  },
  "agentEnvironment.skills.createTitle": {
    message: "新しいスキル",
    description: "Create-skill dialog title",
  },
  "agentEnvironment.skills.editTitle": {
    message: "{name} を編集",
    description: "Edit-skill dialog title",
  },
  "agentEnvironment.skills.viewTitle": {
    message: "{name}",
    description: "View-skill dialog title (read-only)",
  },
  "agentEnvironment.skills.dialogDescription": {
    message:
      "エージェントは説明を読んでスキルを適用するタイミングを判断し、適用する際には指示に従います。",
    description: "Skill dialog description",
  },
  "agentEnvironment.skills.nameLabel": {
    message: "名前",
    description: "Skill name field label",
  },
  "agentEnvironment.skills.namePlaceholder": {
    message: "run-integration-tests",
    description: "Skill name field placeholder",
  },
  "agentEnvironment.skills.descriptionLabel": {
    message: "説明",
    description: "Skill description field label",
  },
  "agentEnvironment.skills.descriptionPlaceholder": {
    message: "このスキルを使用するタイミングを 1 行で",
    description: "Skill description field placeholder",
  },
  "agentEnvironment.skills.onlyWhenAskedLabel": {
    message: "私が依頼したときのみ",
    description: "Toggle: the agent applies the skill only when invoked",
  },
  "agentEnvironment.skills.onlyWhenAskedHint": {
    message:
      "コンポーザーから、またはプロンプトで @skills:name を使って呼び出します。",
    description: "Hint under the only-when-asked toggle",
  },
  "agentEnvironment.skills.bodyLabel": {
    message: "指示",
    description: "Skill body field label",
  },
  "agentEnvironment.skills.bodyPlaceholder": {
    message: "エージェントが従う手順を Markdown で記述します。",
    description: "Skill body field placeholder",
  },
  "agentEnvironment.skills.contentLabel": {
    message: "SKILL.md",
    description: "Whole-file editor label",
  },
  "agentEnvironment.skills.editAsFile": {
    message: "SKILL.md として編集",
    description: "Switch the dialog to the whole-file editor",
  },
  "agentEnvironment.skills.editAsFields": {
    message: "フィールドを編集",
    description: "Switch the dialog back to separate fields",
  },
  "agentEnvironment.skills.history.tab": {
    message: "バージョン履歴",
    description: "Skill dialog tab listing the saved versions",
  },
  "agentEnvironment.skills.history.editTab": {
    message: "編集",
    description: "Skill dialog tab with the editing form",
  },
  "agentEnvironment.skills.history.current": {
    message: "現在",
    description: "Badge on the newest saved version",
  },
  "agentEnvironment.skills.history.changes": {
    message: "変更点",
    description: "Version view: line diff against the previous version",
  },
  "agentEnvironment.skills.history.content": {
    message: "内容",
    description: "Version view: the full SKILL.md",
  },
  "agentEnvironment.skills.history.load": {
    message: "エディターに読み込む",
    description: "Button that fills the editor with the selected version",
  },
  "agentEnvironment.skills.history.loadMore": {
    message: "さらに読み込む",
    description: "Button that loads the next page of versions",
  },
  "agentEnvironment.skills.history.noChanges": {
    message: "このバージョンでは何も変更されていません。",
    description: "Changes view of a version identical to the previous one",
  },
  "agentEnvironment.skills.history.loadFailed": {
    message: "バージョン履歴を読み込めませんでした。",
    description: "Version history list query failed",
  },
  "agentEnvironment.skills.history.versionFailed": {
    message: "このバージョンを読み込めませんでした。",
    description: "Single version query failed",
  },
  "agentEnvironment.skills.history.loadedNotice": {
    message:
      "バージョン {version} を読み込みました。新しいバージョンとして保持するには保存してください。",
    description: "Notice in the editor after Load into editor",
  },
  "agentEnvironment.skills.viewTab": {
    message: "プレビュー",
    description: "Read-only skill dialog tab showing the skill",
  },
  "agentEnvironment.skills.save": {
    message: "スキルを保存",
    description: "Skill dialog save button",
  },
  "agentEnvironment.skills.cancel": {
    message: "キャンセル",
    description: "Skill dialog cancel button",
  },
  "agentEnvironment.skills.close": {
    message: "閉じる",
    description: "Skill dialog close button (read-only)",
  },
  "agentEnvironment.skills.saved": {
    message: "{name} を保存しました",
    description: "Toast after a skill is saved",
  },
  "agentEnvironment.skills.deleted": {
    message: "{name} を削除しました",
    description: "Toast after a skill is deleted",
  },
  "agentEnvironment.skills.readFailed": {
    message: "スキルを開けませんでした。もう一度お試しください。",
    description: "Error when a single skill read fails",
  },
  "agentEnvironment.skills.importFailed": {
    message: "そのファイルを読み取れませんでした。",
    description: "Error when an imported file can't be read",
  },
  "agentEnvironment.skills.nameRequired": {
    message: "名前を入力してください。",
    description: "Skill name empty",
  },
  "agentEnvironment.skills.nameInvalid": {
    message:
      "小文字の英字、数字、単一のハイフンのみ、64 文字以内で入力してください。",
    description: "Skill name fails the Agent Skills format",
  },
  "agentEnvironment.skills.descriptionRequired": {
    message: "1 行の説明を入力してください。",
    description: "Skill description empty",
  },
  "agentEnvironment.skills.descriptionInvalid": {
    message: "説明は 1,024 文字以内の 1 行にしてください。",
    description: "Skill description too long or multi-line",
  },
  "agentEnvironment.skills.bodyInvalid": {
    message: "指示は 64 KiB までです。",
    description: "Skill body too large",
  },
  "agentEnvironment.skills.invalid": {
    message: "このスキルは無効です。名前、説明、指示を確認してください。",
    description: "AGENT_SKILL_INVALID error",
  },
  "agentEnvironment.skills.limitExceeded": {
    message:
      "エージェントのスキルの上限に達しました（ワークスペースあたり 100 件、有効なものの合計 1 MiB）。",
    description: "AGENT_SKILL_LIMIT_EXCEEDED error",
  },
  "agentEnvironment.skills.conflict": {
    message:
      "このスキルは開いた後に変更されたか、名前がすでに使われています。閉じてもう一度開き、最新の状態を確認してください。",
    description: "AGENT_SKILL_CONFLICT error",
  },
  "agentEnvironment.skills.notFound": {
    message: "そのスキルはすでに存在しません。",
    description: "AGENT_SKILL_NOT_FOUND error",
  },
  "agentEnvironment.skills.unavailable": {
    message: "このデプロイメントではエージェントのスキルを利用できません。",
    description: "AGENT_SKILLS_UNAVAILABLE error",
  },
  "agentEnvironment.skills.forbidden": {
    message:
      "エージェントのスキルを変更できるのは、ワークスペースの管理者のみです。",
    description: "Skill write refused for a non-admin",
  },
  "agentEnvironment.skills.saveFailed": {
    message: "変更を保存できませんでした。もう一度お試しください。",
    description: "Generic skill write failure",
  },
  "agentEnvironment.agentsTitle": {
    message: "利用可能なエージェント",
    description: "Settings → Agents: availability section heading",
  },
  "agentEnvironment.agentsDescription": {
    message:
      "メンバーがセッションを開始できるエージェントを選択します。エージェントをオフにしても、実行中のセッションは引き続き実行されます。",
    description: "Settings → Agents: availability section description",
  },
  "agentEnvironment.agentsDataUseNote": {
    message:
      "モデルプロバイダーは、セッションが送信する内容を学習に使用する場合があります。",
    description:
      "Settings → Agents: note under an agent whose provider trains on session data (Muse Code)",
  },
  "agentEnvironment.agentsDataUseTitle": {
    message: "{agent} をオンにしますか？",
    description:
      "Confirm title before turning on an agent whose provider trains on session data",
  },
  "agentEnvironment.agentsDataUseBody.muse": {
    message:
      "このエージェントは Meta のコントリビューターティアで動作します。Meta は、セッションが送信するプロンプト、コード、ファイル、コンテキストと、それに対する応答を、モデルの学習に使用する場合があります。オンにすると、管理者であるあなたがワークスペースを代表してこれを承諾したことが記録されます。",
    description: "Confirm body: Muse Code's data-use acknowledgement (ADR108)",
  },
  "agentEnvironment.agentsDataUseConfirm": {
    message: "承諾してオンにする",
    description: "Confirm button: acknowledge data use and enable the agent",
  },
  "agentEnvironment.agentsSaved": {
    message: "利用可能なエージェントを更新しました",
    description: "Toast after an availability change",
  },
  "agentEnvironment.agentsSaveFailed": {
    message:
      "利用可能なエージェントを更新できませんでした。もう一度お試しください。",
    description: "Error after a failed availability change",
  },
  "agentEnvironment.githubCliLabel": {
    message: "GitHub CLI（gh）",
    description:
      "Label of the switch that gives the workspace's agent sessions gh, the GitHub CLI (w10/m524)",
  },
  "agentEnvironment.githubCliHelp": {
    message:
      "オフにすると、このワークスペースのすべてのエージェントセッション（実行中のものを含む）で gh を使えなくなります。セッションブランチへの Git プッシュと bex のプルリクエストツールは引き続き使えます。",
    description:
      "Help under the GitHub CLI switch: what turning it off does (w10/m524)",
  },
  "agentEnvironment.githubCliTurnedOn": {
    message: "エージェントセッションの GitHub CLI をオンにしました",
    description:
      "Toast after an admin turns the sessions' GitHub CLI on (w10/m524)",
  },
  "agentEnvironment.githubCliTurnedOff": {
    message: "エージェントセッションの GitHub CLI をオフにしました",
    description:
      "Toast after an admin turns the sessions' GitHub CLI off (w10/m524)",
  },
  "agentEnvironment.githubCliSaveFailed": {
    message:
      "GitHub CLI の設定を変更できませんでした。もう一度お試しください。",
    description: "Error after a failed GitHub CLI switch (w10/m524)",
  },
  "agentEnvironment.githubCliMandatoryLabel": {
    message: "オフを必須にする",
    description:
      "Label of the switch that makes the workspace's GitHub CLI off a floor no automation or session can turn back on (w10/m561)",
  },
  "agentEnvironment.githubCliMandatoryHelp": {
    message:
      "GitHub CLI がオフの間に適用されます。その間、オートメーションやセッションで gh をオンに戻すことはできません。",
    description:
      "Help under the mandatory switch: it applies only while GitHub CLI is off (w10/m561)",
  },
  "agentEnvironment.githubCliMandatoryTurnedOn": {
    message: "GitHub CLI のオフを必須にしました",
    description:
      "Toast after an admin makes the workspace's GitHub CLI off mandatory (w10/m561)",
  },
  "agentEnvironment.githubCliMandatoryTurnedOff": {
    message: "GitHub CLI のオフの必須設定を解除しました",
    description:
      "Toast after an admin makes the workspace's GitHub CLI off no longer mandatory (w10/m561)",
  },
  "agentEnvironment.githubCliMandatorySaveFailed": {
    message:
      "GitHub CLI のオフを必須にするかどうかを変更できませんでした。もう一度お試しください。",
    description: "Error after a failed mandatory switch (w10/m561)",
  },
  "agentEnvironment.githubCliAutomationsLabel": {
    message: "オートメーションの GitHub CLI",
    description:
      "Label of the select that binds the gh of the sessions automations start: inherit, on or off (w10/m561)",
  },
  "agentEnvironment.githubCliAutomationsHelp": {
    message:
      "オートメーションが開始するセッションの gh です。オートメーション側で個別に設定しない限り、これが使われます。「継承」は上のスイッチに従います。",
    description:
      "Help under the automations GitHub CLI select: an automation's own binding wins, and inherit follows the workspace switch (w10/m561)",
  },
  "agentEnvironment.githubCliAutomationsInherited": {
    message: "オートメーションは GitHub CLI の設定を継承するようになりました",
    description:
      "Toast after an admin returns automations to inheriting the workspace's GitHub CLI switch (w10/m561)",
  },
  "agentEnvironment.githubCliAutomationsTurnedOn": {
    message: "オートメーションの GitHub CLI をオンにしました",
    description:
      "Toast after an admin turns GitHub CLI on for the sessions automations start (w10/m561)",
  },
  "agentEnvironment.githubCliAutomationsTurnedOff": {
    message: "オートメーションの GitHub CLI をオフにしました",
    description:
      "Toast after an admin turns GitHub CLI off for the sessions automations start (w10/m561)",
  },
  "agentEnvironment.githubCliAutomationsSaveFailed": {
    message:
      "オートメーションの GitHub CLI を変更できませんでした。もう一度お試しください。",
    description:
      "Error after a failed automations GitHub CLI choice (w10/m561)",
  },
  "agentEnvironment.githubCliAutomationsLocked": {
    message:
      "GitHub CLI のオフが必須のため、オートメーションで gh をオンにすることはできません。",
    description:
      "Mapped message for AGENT_ENVIRONMENT_GITHUB_CLI_LOCKED: automations cannot be bound on under a mandatory off (w10/m561)",
  },
  "agentEnvironment.modelsTitle": {
    message: "メンバーが選べるモデル",
    description:
      "Settings → Agents: heading of the list of platform models members can choose (w10/m527)",
  },
  "agentEnvironment.modelsDescription": {
    message:
      "メンバーがセッション、メッセージ、自動化で選べるプラットフォームのモデルを選択します。オフにしたモデルをすでに使っているセッションや自動化は、次のターンからデフォルトのモデルで実行されます。モデルを1つでもオフにすると、後からプラットフォームに追加されたモデルは、オンにするまでオフのままです。",
    description:
      "Settings → Agents: description of the model list — what turning a model off does (w10/m527), and that once one is off a model added to the catalog later stays off (w10/m568)",
  },
  "agentEnvironment.modelsDefaultLocked": {
    message:
      "セッションのフォールバック先となるため、プラットフォームのデフォルトモデルは常にオンのままです。",
    description:
      "Note under the platform catalog's default model, whose switch is locked on (w10/m527)",
  },
  "agentEnvironment.modelsNew": {
    message: "カタログに新規追加",
    description:
      "Note under a platform model that is off because it joined the catalog after an admin narrowed the list; an admin turns it on with its switch (w10/m568)",
  },
  "agentEnvironment.modelsTurnedOn": {
    message: "メンバー向けに {model} をオンにしました",
    description:
      "Toast after an admin turns a model on for members; {model} is its id (w10/m527)",
  },
  "agentEnvironment.modelsTurnedOff": {
    message: "メンバー向けに {model} をオフにしました",
    description:
      "Toast after an admin turns a model off for members; {model} is its id (w10/m527)",
  },
  "agentEnvironment.modelsInvalid": {
    message:
      "モデルを更新できませんでした。プラットフォームのデフォルトモデルはオンのままにする必要があり、リストに含められるのはプラットフォームカタログのモデルだけです。ページを再読み込みして現在のカタログを確認してください。",
    description:
      "Server refusal AGENT_ENVIRONMENT_MODELS_INVALID of a model list change (w10/m527)",
  },
  "agentEnvironment.modelsSaveFailed": {
    message:
      "メンバーが選べるモデルを更新できませんでした。もう一度お試しください。",
    description: "Error after a failed model switch (w10/m527)",
  },
  "agentEnvironment.modelOffOption": {
    message: "{model}（オフ）",
    description:
      "Default model picker: the saved default, which an admin turned off (w10/m527)",
  },
  "agentEnvironment.modelDefaultOffNote": {
    message:
      "{model} はオフになっているため、モデルを指定しないセッションと自動化は、管理者がオンのモデルを選ぶか {model} を再びオンにするまで、プラットフォームのデフォルト {fallback} を使用します。",
    description:
      "Note under the default model picker when the saved default is turned off; {fallback} is the platform default that runs instead (w10/m527)",
  },
  "agentEnvironment.modelDefaultRetiredNote": {
    message:
      "{model} はプラットフォームカタログから外れたため、モデルを指定しないセッションと自動化は、管理者が別のモデルを選ぶまで、プラットフォームのデフォルト {fallback} を使用します。",
    description:
      "Note under the default model picker when the saved default left the platform catalog; {fallback} is the platform default that runs instead (w10/m527)",
  },
};

export default jaAgentEnvironment;
