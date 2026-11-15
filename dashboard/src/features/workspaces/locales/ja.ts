import type { TranslationEntry } from "@/i18n";

const jaWorkspaces: Record<string, TranslationEntry> = {
  "workspaces.switcherEmpty": {
    message: "ワークスペースを選択",
    description: "Switcher trigger label before any workspace is selected",
  },
  "workspaces.switcherLabel": {
    message: "ワークスペース",
    description: "Switcher dropdown label above the workspace list",
  },
  "workspaces.switcherBilling": {
    message: "請求",
    description: "Switcher menu item linking to /billing",
  },
  "workspaces.switcherSettings": {
    message: "ワークスペース設定",
    description: "Switcher menu item linking to /workspace/settings",
  },
  "workspaces.switcherNew": {
    message: "+ 新規ワークスペース",
    description: "Switcher menu item linking to /new/workspace",
  },
  "workspaces.newTitle": {
    message: "新規ワークスペース",
    description: "New-workspace page document title",
  },
  "workspaces.planPickerLabel": {
    message: "プラン",
    description: "Accessible label for the plan-card radiogroup",
  },
  "workspaces.planHobbyName": {
    message: "Hobby",
    description: "Workspace plan card name",
  },
  "workspaces.planHobbyBilling": {
    message: "$0/月",
    description:
      "Workspace plan card billing label (pricing.yaml hobby usdPerMonth)",
  },
  "workspaces.planHobbyDescription": {
    message: "はじめて使う個人向けです。",
    description: "Workspace plan card one-line pitch",
  },
  "workspaces.planHobbyBulletMembers": {
    message: "メンバー 1 人",
    description: "Hobby plan LimitsFor.MaxMembers bullet",
  },
  "workspaces.planHobbyBulletServices": {
    message: "サービス最大 25 件",
    description: "Hobby plan LimitsFor.MaxServices bullet",
  },
  "workspaces.planHobbyBulletWorkspaces": {
    message: "ユーザーあたり Hobby ワークスペース 5 つ",
    description: "Hobby plan LimitsFor.MaxWorkspacesPerUser bullet",
  },
  "workspaces.planProName": {
    message: "Pro",
    description: "Workspace plan card name",
  },
  "workspaces.planProBilling": {
    message: "$17.50/月",
    description:
      "Workspace plan card billing label (pricing.yaml pro usdPerMonth)",
  },
  "workspaces.planProDescription": {
    message: "一緒にリリースする小規模チーム向けです。",
    description: "Workspace plan card one-line pitch",
  },
  "workspaces.planProBulletMembers": {
    message: "メンバー数無制限",
    description: "Pro plan unlimited MaxMembers bullet",
  },
  "workspaces.planProBulletServices": {
    message: "サービス数無制限",
    description: "Pro plan unlimited MaxServices bullet",
  },
  "workspaces.planScaleName": {
    message: "Scale",
    description: "Workspace plan card name",
  },
  "workspaces.planScaleBilling": {
    message: "$349.30/月",
    description:
      "Workspace plan card billing label (pricing.yaml scale usdPerMonth)",
  },
  "workspaces.planScaleDescription": {
    message: "追加のロールが必要な成長中のチーム向けです。",
    description: "Workspace plan card one-line pitch",
  },
  "workspaces.planScaleBulletMembers": {
    message: "メンバー数無制限",
    description: "Scale plan unlimited MaxMembers bullet",
  },
  "workspaces.planScaleBulletServices": {
    message: "サービス数無制限",
    description: "Scale plan unlimited MaxServices bullet",
  },
  "workspaces.planScaleBulletRoles": {
    message: "追加のロール（コントリビューター、閲覧者、請求担当）",
    description: "Scale plan AllowedRoles beyond Pro bullet",
  },
  "workspaces.planEnterpriseName": {
    message: "Enterprise",
    description: "Workspace plan card name",
  },
  "workspaces.planEnterpriseBilling": {
    message: "個別契約",
    description: "Workspace plan card billing label",
  },
  "workspaces.planEnterpriseDescription": {
    message: "カスタムの上限とサポートを提供します。",
    description: "Workspace plan card one-line pitch",
  },
  "workspaces.planEnterpriseBulletLimits": {
    message: "カスタムの上限",
    description: "Enterprise plan custom-limits bullet",
  },
  "workspaces.planEnterpriseBulletSupport": {
    message: "カスタムのサポート",
    description: "Enterprise plan custom-support bullet",
  },
  "workspaces.planSelect": {
    message: "プランを選択",
    description: "Unselected plan card action label",
  },
  "workspaces.planSelected": {
    message: "選択中のプラン",
    description: "Selected plan card action label",
  },
  "workspaces.planUsageBillingNote": {
    message:
      "サービスとデータストアの使用料は、リソースのティアごとに別途請求されます。",
    description: "Billing note below the workspace plan picker",
  },
  "workspaces.createTitle": {
    message: "ワークスペースを作成",
    description: "/new/workspace page heading",
  },
  "workspaces.createDescription": {
    message: "詳細、請求先、プラン、お支払い方法を選択してください。",
    description: "/new/workspace page subtitle",
  },
  "workspaces.detailsTitle": {
    message: "ワークスペースの詳細",
    description: "/new/workspace details section heading",
  },
  "workspaces.billingEmail": {
    message: "請求先メールアドレス",
    description: "/new/workspace billing email label",
  },
  "workspaces.billingEmailHelp": {
    message: "このワークスペースの領収書と請求に関するお知らせが送信されます。",
    description: "Editable paid-plan billing email help",
  },
  "workspaces.billingEmailHobbyHelp": {
    message:
      "Hobby ワークスペースの請求先メールアドレスは、アカウントのメールアドレスです。",
    description: "Read-only Hobby billing email help",
  },
  "workspaces.billingEmailError": {
    message: "有効な請求先メールアドレスを入力してください。",
    description: "Billing email validation error",
  },
  "workspaces.fieldSlug": {
    message: "ワークスペースのスラッグ",
    description: "/new/workspace slug field label",
  },
  "workspaces.fieldSlugHelp": {
    message:
      "CLI と API で使用する一意のキーです。小文字の英字、数字、ハイフンを 1～30 文字で指定します。",
    description: "/new/workspace slug helper text",
  },
  "workspaces.fieldSlugError": {
    message:
      "小文字の英字、数字、ハイフンのみを 1～30 文字で指定してください。先頭と末尾にハイフンは使用できません。",
    description: "Workspace slug validation error",
  },
  "workspaces.paymentTitle": {
    message: "お支払い方法",
    description: "/new/workspace payment panel heading",
  },
  "workspaces.paymentDescription": {
    message:
      "請求はワークスペースごとに独立しています。このお支払い方法は、新しいワークスペースにのみ適用されます。",
    description: "/new/workspace workspace-specific payment copy",
  },
  "workspaces.paymentRequired": {
    message: "このワークスペースにはお支払い方法が必要です。",
    description: "Required payment policy copy",
  },
  "workspaces.paymentOptional": {
    message: "今すぐお支払い方法を追加するか、追加せずに続行してください。",
    description: "Optional payment policy copy",
  },
  "workspaces.paymentSelfHosted": {
    message: "このセルフホスト環境では、支払いの回収は無効になっています。",
    description: "Billing-off create-flow copy",
  },
  "workspaces.paymentAdd": {
    message: "お支払い方法を追加",
    description: "Open Payment Element action",
  },
  "workspaces.paymentSave": {
    message: "お支払い方法を保存",
    description: "Confirm SetupIntent action",
  },
  "workspaces.paymentAdded": {
    message: "このワークスペースのお支払い方法を確認しました。",
    description: "Successful payment setup state",
  },
  "workspaces.paymentError": {
    message: "そのお支払い方法を確認できませんでした。もう一度お試しください。",
    description: "Payment Element fallback error",
  },
  "workspaces.fieldName": {
    message: "名前",
    description: "Workspace name field label (shared by create + settings)",
  },
  "workspaces.fieldDisplayNamePlaceholder": {
    message: "例: Acme Staging",
    description: "/new/workspace freeform name placeholder",
  },
  "workspaces.fieldNamePlaceholder": {
    message: "例: acme-staging",
    description: "Workspace name field placeholder",
  },
  "workspaces.fieldNameError": {
    message: "改行を含まない、1～100 文字の名前を入力してください。",
    description: "Workspace name validation error",
  },
  "workspaces.fieldPlan": {
    message: "プラン",
    description: "Workspace plan field label (create picker + settings badge)",
  },
  "workspaces.createErrorTitle": {
    message: "ワークスペースを作成できませんでした",
    description: "/new/workspace inline error alert title",
  },
  "workspaces.createCancel": {
    message: "キャンセル",
    description: "/new/workspace cancel button",
  },
  "workspaces.createCancelError": {
    message:
      "ワークスペースの作成をキャンセルできませんでした。もう一度お試しください。",
    description:
      "/new/workspace toast when cancelling the pending creation fails",
  },
  "workspaces.createSubmit": {
    message: "ワークスペースを作成",
    description: "/new/workspace submit button",
  },
  "workspaces.createSuccess": {
    message: "{name} を作成しました",
    description: "Toast on a successful workspace create",
  },
  "workspaces.createError": {
    message: "ワークスペースを作成できませんでした",
    description: "Fallback toast/inline message on a failed create",
  },
  "workspaces.creationLoadError": {
    message:
      "ワークスペースのセットアップを読み込めませんでした。ページを更新してもう一度お試しください。",
    description:
      "Inline message when the workspace-creation plan policy or a resumed setup attempt fails to load",
  },
  "workspaces.settingsTitle": {
    message: "ワークスペース設定",
    description: "Workspace settings page and card title",
  },
  "workspaces.settingsDescription": {
    message:
      "このワークスペースの名前を変更したり、プランとメタデータを確認したりできます。",
    description: "Workspace settings card description",
  },
  "workspaces.settingsEmpty": {
    message: "ワークスペースが選択されていません。",
    description: "Workspace settings page empty state",
  },
  "workspaces.settingsNavigation": {
    message: "設定のセクション",
    description:
      "Accessible label for the workspace settings section navigation",
  },
  "workspaces.renameSubmit": {
    message: "保存",
    description: "Workspace rename form submit button",
  },
  "workspaces.renameErrorTitle": {
    message: "ワークスペースの名前を変更できませんでした",
    description: "Workspace rename inline error alert title",
  },
  "workspaces.renameSuccess": {
    message: "名前を {name} に変更しました",
    description: "Toast on a successful rename",
  },
  "workspaces.renameError": {
    message: "ワークスペースの名前を変更できませんでした",
    description: "Fallback toast/inline message on a failed rename",
  },
  "workspaces.fieldId": {
    message: "ワークスペース ID",
    description: "Workspace settings metadata field label",
  },
  "workspaces.fieldCreatedAt": {
    message: "作成日",
    description: "Workspace settings metadata field label",
  },
  "workspaces.dangerZoneTitle": {
    message: "危険な操作",
    description: "Workspace settings delete section title",
  },
  "workspaces.dangerZoneDescription": {
    message:
      "ワークスペースのすべてのリソースとデータが削除されます。すべてのサービス、データストア、環境変数が失われます。この操作は元に戻せません。",
    description: "Workspace settings delete section description",
  },
  "workspaces.deleteConfirmLabel": {
    message: "確認のため、下に {phrase} と入力してください。",
    description:
      "Body prompt naming the exact 'sudo delete workspace <name>' phrase (rendered bold by SudoCommandField)",
  },
  "workspaces.deleteErrorTitle": {
    message: "ワークスペースを削除できませんでした",
    description: "Delete danger-zone inline error alert title",
  },
  "workspaces.deleteSubmit": {
    message: "ワークスペースを削除",
    description: "Delete danger-zone submit button",
  },
  "workspaces.deleteSuccess": {
    message: "{name} を削除しました",
    description: "Toast on a successful delete",
  },
  "workspaces.deleteError": {
    message: "ワークスペースを削除できませんでした",
    description: "Fallback toast/inline message on a failed delete",
  },
  "workspaces.changePlanTrigger": {
    message: "プランを変更",
    description:
      "Workspace settings plan-badge link opening the change-plan dialog",
  },
  "workspaces.changePlanTitle": {
    message: "プランを変更",
    description: "Change-plan dialog title",
  },
  "workspaces.changePlanDescription": {
    message:
      "このワークスペースの新しいプランを選択してください。お支払い手続きはなく、プランはただちに変更されます。",
    description: "Change-plan dialog description",
  },
  "workspaces.changePlanCancel": {
    message: "キャンセル",
    description: "Change-plan dialog cancel button",
  },
  "workspaces.changePlanSubmit": {
    message: "プランを変更",
    description: "Change-plan dialog submit button",
  },
  "workspaces.changePlanErrorTitle": {
    message: "プランを変更できませんでした",
    description: "Change-plan dialog inline error alert title",
  },
  "workspaces.changePlanSuccess": {
    message: "プランを {plan} に変更しました",
    description: "Toast on a successful plan change",
  },
  "workspaces.changePlanError": {
    message: "ワークスペースのプランを変更できませんでした",
    description: "Fallback toast/inline message on a failed plan change",
  },
  "workspaces.generalTitle": {
    message: "一般",
    description: "Title of the general card on the workspace settings page",
  },
  "workspaces.ipRulesTitle": {
    message: "インバウンド IP ルール",
    description:
      "Workspace Settings → Inbound IP Rules card title and section link (w9/m388)",
  },
  "workspaces.ipRulesDescription": {
    message:
      "このワークスペース内のすべての Web Service、Static Site、Postgres データベース、Key Value インスタンスにアクセスできる IP アドレスを選択します。これらのルールは環境やサービスのルールに加えて適用されるため、送信元は各レベルで許可されている必要があります。Private Service、Background Worker、Cron Job は対象外です。",
    description:
      "Inbound IP Rules card description: scope and layering of workspace rules (Render's workspace-level inbound IP rules, w9/m388)",
  },
  "workspaces.ipRulesHint": {
    message:
      "CIDR ブロック（例: 203.0.113.0/24）を入力して「送信元を追加」を押してください。ルールがない場合、すべてのインバウンドトラフィックがブロックされます。0.0.0.0/0 と ::/0 は、すべての IPv4 と IPv6 の送信元を許可します。",
    description: "Hint above the editable workspace inbound IP rules (w9/m388)",
  },
  "workspaces.ipRulesBlocked": {
    message:
      "ルールなし: このワークスペースのサービスとデータストアへのすべてのインバウンドトラフィックがブロックされています。すべての送信元を許可するには、0.0.0.0/0 と ::/0 を追加してください。",
    description:
      "Warning shown when the workspace rule list is empty (deny-all), saved or about to be saved (w9/m388)",
  },
  "workspaces.ipRulesOpen": {
    message: "すべての送信元 IP に開放",
    description:
      "Shown in the read-only view when the workspace rules restrict nothing (Render's allow-all default, w9/m388)",
  },
  "workspaces.ipRulesScalePlanOnly": {
    message:
      "ワークスペースのインバウンド IP ルールには、Scale プラン以上が必要です。すでに設定されているルールは引き続き適用され、すべて許可にリセットできます。",
    description:
      "Notice replacing the editable workspace rules below the Scale plan (w9/m388)",
  },
  "workspaces.ipRulesAdminOnly": {
    message: "これらのルールを変更できるのは、ワークスペースの管理者のみです。",
    description:
      "Notice for a member who cannot edit the workspace inbound IP rules, and the forbidden-save explanation (w9/m388)",
  },
  "workspaces.ipRulesPlanRefused": {
    message:
      "このワークスペースのプランには、インバウンド IP ルールが含まれていません。トラフィックを制限するには、Scale 以上にアップグレードしてください。ルールをすべて許可にリセットすることは、いつでもできます。",
    description:
      "Inline explanation when bex-api refuses a workspace rule change with PLAN_FEATURE_REQUIRED (w9/m388)",
  },
  "workspaces.ipRulesUnreadable": {
    message:
      "このワークスペースのインバウンド IP ルールを読み込めませんでした。",
    description:
      "Shown in place of the rules when the workspace inbound IP rules read fails (w9/m388)",
  },
  "workspaces.ipRulesResetAllowAll": {
    message: "すべて許可にリセット",
    description:
      "Button replacing the workspace inbound IP rules with the allow-all pair below the Scale plan (w9/m388)",
  },
  "workspaces.ipRulesCIDR": {
    message: "新しい CIDR ブロック",
    description:
      "Accessible label for the add-row CIDR input of the workspace inbound IP rules",
  },
  "workspaces.ipRulesCIDRRule": {
    message: "ルール {number} の CIDR ブロック",
    description:
      "Accessible label for an existing workspace inbound IP rule CIDR",
  },
  "workspaces.ipRulesDescriptionLabel": {
    message: "新しいルールの説明",
    description:
      "Accessible label for the add-row description input of the workspace inbound IP rules",
  },
  "workspaces.ipRulesDescriptionRule": {
    message: "ルール {number} の説明",
    description:
      "Accessible label for an existing workspace inbound IP rule description",
  },
  "workspaces.ipRulesDescriptionPlaceholder": {
    message: "説明（任意）",
    description: "Placeholder for a workspace inbound IP rule description",
  },
  "workspaces.ipRulesAdd": {
    message: "送信元を追加",
    description: "Button adding a CIDR to the draft workspace inbound IP rules",
  },
  "workspaces.ipRulesCancel": {
    message: "キャンセル",
    description:
      "Button discarding unsaved changes to the workspace inbound IP rules",
  },
  "workspaces.ipRulesSave": {
    message: "ルールを保存",
    description:
      "Button saving the workspace inbound IP rules (setWorkspaceIPAllowList)",
  },
  "workspaces.ipRulesInvalid": {
    message:
      "重複しない、有効な IPv4 または IPv6 の CIDR ブロックを入力してください。",
    description:
      "Inline validation when a workspace rule block is not a unique, valid CIDR",
  },
  "workspaces.ipRulesRemove": {
    message: "{cidr} を削除",
    description:
      "Accessible label for a workspace inbound IP rule remove button",
  },
  "workspaces.ipRulesMoveUp": {
    message: "{cidr} を上に移動",
    description: "Accessible label to move a workspace inbound IP rule earlier",
  },
  "workspaces.ipRulesMoveDown": {
    message: "{cidr} を下に移動",
    description: "Accessible label to move a workspace inbound IP rule later",
  },
  "workspaces.ipRulesSaved": {
    message: "インバウンド IP ルールを保存しました",
    description: "Toast after setWorkspaceIPAllowList succeeds (w9/m388)",
  },
  "workspaces.ipRulesSaveErrorTitle": {
    message: "インバウンド IP ルールを保存できませんでした",
    description:
      "Title of the inline alert when a workspace rule change is refused (w9/m388)",
  },
  "workspaces.ipRulesSaveError": {
    message:
      "インバウンド IP ルールを保存できませんでした。もう一度お試しください。",
    description:
      "Fallback inline message when a workspace rule change fails without a server reason (w9/m388)",
  },
  "workspaces.twoFactorTitle": {
    message: "二要素認証",
    description:
      "Title of the two-factor card on the workspace settings page, and its quick-nav label (w22/m402)",
  },
  "workspaces.twoFactorDescription": {
    message:
      "二要素認証（2FA）が有効でないメンバーは、有効にするまでこのワークスペースのリソースや設定にアクセスできません。その間は、API キーと SSH キーもブロックされます。",
    description:
      "Description under the two-factor card title; follows Render's login-settings wording",
  },
  "workspaces.twoFactorSwitchLabel": {
    message: "二要素認証（2FA）を必須にする",
    description:
      "Label of the switch that requires 2FA of every member (Render's control name)",
  },
  "workspaces.twoFactorStatusRequired": {
    message: "必須",
    description:
      "Read-only status shown to a non-admin when the workspace requires 2FA",
  },
  "workspaces.twoFactorStatusNotRequired": {
    message: "必須ではない",
    description:
      "Read-only status shown to a non-admin when the workspace does not require 2FA",
  },
  "workspaces.twoFactorAdminOnly": {
    message: "この設定を変更できるのはワークスペースの管理者のみです。",
    description: "Hint shown to a non-admin in place of the switch",
  },
  "workspaces.twoFactorEnabledToast": {
    message: "{name} で二要素認証が必須になりました",
    description: "Toast after an admin turns the 2FA requirement on",
  },
  "workspaces.twoFactorDisabledToast": {
    message: "{name} で二要素認証は必須ではなくなりました",
    description: "Toast after an admin turns the 2FA requirement off",
  },
  "workspaces.twoFactorErrorTitle": {
    message: "二要素認証の設定を変更できませんでした",
    description: "Title of the inline error when the 2FA switch is refused",
  },
  "workspaces.twoFactorError": {
    message: "二要素認証の設定を変更できませんでした。もう一度お試しください。",
    description:
      "Generic message when the 2FA switch fails without a recognised code",
  },
  "workspaces.twoFactorErrorAdminNotEnrolled": {
    message:
      "このワークスペースで必須にする前に、ご自身のアカウントで二要素認証を有効にしてください。そうしないと、ご自身がアクセスできなくなります。",
    description:
      "Refusal for code WORKSPACE_2FA_ADMIN_NOT_ENROLLED: the acting admin has no second factor",
  },
  "workspaces.twoFactorErrorUnavailable": {
    message:
      "この環境では二要素認証の強制を利用できないため、有効にできません。",
    description:
      "Refusal for code WORKSPACE_2FA_UNAVAILABLE: the deployment has no identity lookup",
  },
  "workspaces.twoFactorEnrollLink": {
    message: "二要素認証を設定",
    description:
      "Link to account security, in the admin-not-enrolled refusal and on the blocked page",
  },
  "workspaces.twoFactorBlockedTitle": {
    message: "{workspace} では二要素認証が必須です",
    description: "Heading of the page a blocked member sees (w22/m402)",
  },
  "workspaces.twoFactorBlockedBody": {
    message:
      "このワークスペースの管理者が、すべてのメンバーに二要素認証（2FA）の使用を求めています。有効にするまで、このワークスペースのリソースや設定にはアクセスできず、API キーと SSH キーも使用できません。",
    description: "Why the blocked page is shown",
  },
  "workspaces.twoFactorBlockedRecheck": {
    message: "有効にしました。もう一度確認",
    description:
      "Button on the blocked page that retries the workspace after enrolling",
  },
  "workspaces.twoFactorBlockedRecheckHint": {
    message:
      "2FA を有効にしてから、アクセスが回復するまで最大 1 分かかることがあります。",
    description: "Hint under the blocked page's check-again button",
  },
  "workspaces.twoFactorBlockedSwitchLabel": {
    message: "別のワークスペースに切り替え",
    description: "Label of the workspace picker on the blocked page",
  },
  "workspaces.twoFactorBlockedSwitchPlaceholder": {
    message: "ワークスペースを選択",
    description: "Placeholder of the workspace picker on the blocked page",
  },
  "workspaces.twoFactorBlockedNoOtherWorkspaces": {
    message: "他のワークスペースはありません",
    description:
      "Placeholder of the blocked page's workspace picker when the member belongs to no other workspace",
  },
  "workspaces.overlappingDeployPolicyTitle": {
    message: "重複デプロイポリシー",
    description:
      "Workspace Settings section title, Render's name for the setting (w4/653)",
  },
  "workspaces.overlappingDeployPolicyDescription": {
    message:
      "サービスのデプロイが進行中に、同じサービスの別のデプロイがトリガーされたときの動作を選択します。",
    description:
      "Workspace Settings → Overlapping Deploy Policy description (w4/653)",
  },
  "workspaces.overlappingDeployPolicyWait": {
    message: "待機",
    description: "The Wait policy's label, Render's wording (w4/653)",
  },
  "workspaces.overlappingDeployPolicyWaitDescription": {
    message:
      "進行中のデプロイの完了を待ち、その後、最後にトリガーされたデプロイにそのまま進みます。",
    description:
      "The Wait policy's sentence, Render's wording (render.com/docs/deploys, w4/653)",
  },
  "workspaces.overlappingDeployPolicyOverride": {
    message: "上書き",
    description: "The Override policy's label, Render's wording (w4/653)",
  },
  "workspaces.overlappingDeployPolicyOverrideDescription": {
    message:
      "進行中のデプロイをただちにキャンセルし、新しいデプロイを開始します。",
    description:
      "The Override policy's sentence, Render's wording (render.com/docs/deploys, w4/653)",
  },
  "workspaces.overlappingDeployPolicySave": {
    message: "保存",
    description: "Saves the selected Overlapping Deploy Policy (w4/653)",
  },
  "workspaces.overlappingDeployPolicySaving": {
    message: "保存中…",
    description: "The Save button while the policy saves (w4/653)",
  },
  "workspaces.overlappingDeployPolicySaved": {
    message: "重複デプロイポリシーを保存しました",
    description:
      "Toast after setWorkspaceOverlappingDeployPolicy succeeds (w4/653)",
  },
  "workspaces.overlappingDeployPolicyError": {
    message:
      "重複デプロイポリシーを保存できませんでした。もう一度お試しください。",
    description:
      "Fallback toast when a policy save fails without a server reason (w4/653)",
  },
  "workspaces.overlappingDeployPolicyAdminOnly": {
    message: "このポリシーを変更できるのはワークスペースの管理者のみです。",
    description:
      "Shown in place of Save to a member who cannot manage the workspace (w4/653)",
  },
};

export default jaWorkspaces;
