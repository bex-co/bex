import type { TranslationEntry } from "@/i18n";

const jaWorkspaces: Record<string, TranslationEntry> = {
  "workspaces.idCopied": {
    message: "ワークスペース ID をコピーしました",
    description: "Toast after copying the workspace ID",
  },
  "workspaces.idCopyFailed": {
    message: "ワークスペース ID をコピーできませんでした",
    description: "Toast when copying the workspace ID fails",
  },
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
    message: "組織ごとに $349.30/月",
    description:
      "Workspace plan card billing label (pricing.yaml scale usdPerMonth, per: organization; w7/m464)",
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
    message: "組織ごとの個別契約",
    description:
      "Workspace plan card billing label: Enterprise is charged once per organization like Scale (w7/m464)",
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
  "workspaces.loginRequirementStatusRequired": {
    message: "必須",
    description:
      "Read-only status shown to a non-admin when the workspace requires 2FA",
  },
  "workspaces.loginRequirementStatusNotRequired": {
    message: "必須ではない",
    description:
      "Read-only status shown to a non-admin when the workspace does not require 2FA",
  },
  "workspaces.loginRequirementAdminOnly": {
    message: "この設定を変更できるのはワークスペースの管理者のみです。",
    description: "Hint shown to a non-admin in place of the switch",
  },
  "workspaces.twoFactorProPlanOnly": {
    message:
      "二要素認証の強制には、Pro プラン以上が必要です。すでに強制しているワークスペースは、管理者がオフにするまで引き続き適用されます。",
    description:
      "Notice on the two-factor card below the Pro plan, and the refusal for code PLAN_FEATURE_REQUIRED (Render's plan table, w7/070)",
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
  "workspaces.loginBlockedSwitchLabel": {
    message: "別のワークスペースに切り替え",
    description: "Label of the workspace picker on the blocked page",
  },
  "workspaces.loginBlockedSwitchPlaceholder": {
    message: "ワークスペースを選択",
    description: "Placeholder of the workspace picker on the blocked page",
  },
  "workspaces.loginBlockedNoOtherWorkspaces": {
    message: "他のワークスペースはありません",
    description:
      "Placeholder of the blocked page's workspace picker when the member belongs to no other workspace",
  },
  "workspaces.googleLoginTitle": {
    message: "Google ログイン",
    description:
      "Title of the Google login card on the workspace settings page, and its quick-nav label (w7/m465)",
  },
  "workspaces.googleLoginDescription": {
    message:
      "bex のメールアドレスの Google アカウントでログインしていないメンバーは、このワークスペースのリソースや設定にアクセスできません。ほかの方法でログインしているときに作成された API キーと SSH キーもブロックされます。",
    description:
      "Description under the Google login card title; follows Render's login-settings wording",
  },
  "workspaces.googleLoginSwitchLabel": {
    message: "Google ログインを必須にする",
    description:
      "Label of the switch that requires every member to sign in with Google (Render's control name)",
  },
  "workspaces.googleLoginProPlanOnly": {
    message:
      "Google ログインの強制には、Pro プラン以上が必要です。すでに強制しているワークスペースは、管理者がオフにするまで引き続き適用されます。",
    description:
      "Notice on the Google login card below the Pro plan, and the refusal for code PLAN_FEATURE_REQUIRED (Render's plan table)",
  },
  "workspaces.googleLoginEnabledToast": {
    message: "{name} で Google ログインが必須になりました",
    description: "Toast after an admin turns the Google login requirement on",
  },
  "workspaces.googleLoginDisabledToast": {
    message: "{name} で Google ログインは必須ではなくなりました",
    description: "Toast after an admin turns the Google login requirement off",
  },
  "workspaces.googleLoginErrorTitle": {
    message: "Google ログインの設定を変更できませんでした",
    description:
      "Title of the inline error when the Google login switch is refused",
  },
  "workspaces.googleLoginError": {
    message:
      "Google ログインの設定を変更できませんでした。もう一度お試しください。",
    description:
      "Generic message when the Google login switch fails without a recognised code",
  },
  "workspaces.googleLoginErrorAdminNotSignedIn": {
    message:
      "このワークスペースで Google ログインを必須にする前に、bex のメールアドレスの Google アカウントでログインしてください。そうしないと、ご自身がアクセスできなくなります。",
    description:
      "Refusal for code WORKSPACE_GOOGLE_LOGIN_ADMIN_NOT_SIGNED_IN: the acting admin is not signed in with Google",
  },
  "workspaces.googleLoginErrorUnavailable": {
    message:
      "この環境では Google ログインの強制を利用できないため、有効にできません。",
    description:
      "Refusal for code WORKSPACE_GOOGLE_LOGIN_UNAVAILABLE: the deployment cannot look up Google accounts",
  },
  "workspaces.googleLoginSignIn": {
    message: "Google でログイン",
    description:
      "Button that signs the session in again with Google and returns to the page: on the blocked page and in the admin's lock-out refusal",
  },
  "workspaces.googleLoginSignInError": {
    message:
      "Google でのログインを開始できませんでした。もう一度お試しください。",
    description: "Shown when Kratos did not start the Google sign-in",
  },
  "workspaces.googleLoginBlockedTitle": {
    message: "{workspace} では Google ログインが必須です",
    description:
      "Heading of the page a member who did not sign in with Google sees (w7/m465)",
  },
  "workspaces.googleLoginBlockedBody": {
    message:
      "このワークスペースの管理者が、すべてのメンバーに Google でのログインを求めています。bex のメールアドレスの Google アカウントでログインするまで、このワークスペースのリソースや設定にはアクセスできず、ほかの方法でログインしているときに作成した API キーと SSH キーも使用できません。",
    description: "Why the Google login blocked page is shown",
  },
  "workspaces.googleLoginBlockedHint": {
    message: "ログインすると、このページに戻ります。",
    description: "Hint under the blocked page's Sign in with Google button",
  },
  "workspaces.requireTwoFactorConfirmTitle": {
    message: "{workspace} で二要素認証を必須にしますか？",
    description:
      "Title of the confirmation before an admin requires 2FA in the workspace (w7/m471)",
  },
  "workspaces.requireTwoFactorConfirm": {
    message:
      "二要素認証を使用していないメンバーは、有効にするまでこのワークスペースにアクセスできなくなり、API キーと SSH キーもこのワークスペースでは使用できなくなります。",
    description:
      "Body of the confirmation before an admin requires 2FA in the workspace (w7/m471)",
  },
  "workspaces.requireGoogleLoginConfirmTitle": {
    message: "{workspace} で Google ログインを必須にしますか？",
    description:
      "Title of the confirmation before an admin requires Google login in the workspace (w7/m471)",
  },
  "workspaces.requireGoogleLoginConfirm": {
    message:
      "bex のメールアドレスの Google アカウントでログインしていないメンバーは、その方法でログインするまでこのワークスペースにアクセスできなくなり、ほかの方法でログインしているときに作成された API キーと SSH キーもこのワークスペースでは使用できなくなります。",
    description:
      "Body of the confirmation before an admin requires Google login in the workspace (w7/m471)",
  },
  "workspaces.requireConfirmAction": {
    message: "必須にする",
    description:
      "Confirm button of the workspace's require-2FA and require-Google-login confirmations (w7/m471)",
  },
  "workspaces.loginImpactRegionLabel": {
    message: "アクセスできなくなるユーザー",
    description:
      "Accessible name of the confirmation's list of members and keys a login requirement would block (w7/m471)",
  },
  "workspaces.loginImpactChecking": {
    message: "アクセスできなくなるユーザーを確認しています…",
    description:
      "Screen-reader text while the confirmation checks who a login requirement would block",
  },
  "workspaces.loginImpactError": {
    message:
      "アクセスできなくなるユーザーを確認できませんでした。このまま有効にすることもできます。",
    description:
      "Shown in the confirmation when bex could not check who a login requirement would block; confirming stays possible",
  },
  "workspaces.loginImpactNobody": {
    message:
      "アクセスできなくなる人はいません。すべてのメンバーがこの要件を満たしており、使用できなくなる API キーや SSH キーもありません。",
    description:
      "Shown in the confirmation when a login requirement would block nobody",
  },
  "workspaces.loginImpactMembersTwoFactor": {
    message:
      "一覧の各メンバーは、二要素認証を有効にするまでアクセスできなくなります。一覧の各 API キーと SSH キーは、所有者が有効にするまで使用できなくなります。",
    description:
      "Sentence above the members and keys requiring 2FA would block, after Render's SSO enforcement list",
  },
  "workspaces.loginImpactMembersGoogleLogin": {
    message:
      "一覧の各メンバーは、bex のメールアドレスの Google アカウントでログインするまでアクセスできなくなります。一覧の各 API キーと SSH キーは使用できなくなります。所有者は Google でログインしてから新しいキーを作成できます。",
    description:
      "Sentence above the members and keys requiring Google login would block, after Render's SSO enforcement list",
  },
  "workspaces.loginImpactLosesAccess": {
    message: "アクセス不可",
    description: "Badge on a listed member who would lose access",
  },
  "workspaces.loginImpactKeepsAccess": {
    message: "アクセス可",
    description:
      "Badge on a listed member who keeps access while their listed keys stop working",
  },
  "workspaces.loginImpactApiKey": {
    message: "API キー {name}",
    description:
      "A listed API key that would stop working; {name} is the key's name",
  },
  "workspaces.loginImpactApiKeyInWorkspace": {
    message: "{workspace} の API キー {name}",
    description:
      "A listed API key on an org's confirmation; {workspace} is the workspace it belongs to",
  },
  "workspaces.loginImpactSshKey": {
    message: "SSH キー {name}",
    description:
      "A listed SSH key that would stop working; {name} is the key's name",
  },
  "workspaces.loginImpactUnattributed": {
    message:
      "作成者の記録がない API キー、または作成者がメンバーでなくなった API キーは使用できなくなります：",
    description:
      "Heading of the API keys no current member created, which would stop working",
  },
  "workspaces.loginImpactGoogleSessions": {
    message:
      "要件を満たしていても、現在ほかの方法でログインしているメンバーは、Google でログインするよう求められます。",
    description:
      "One line under the Google login confirmation's list: who the list cannot show",
  },
  "workspaces.loginNoticeMembers_other": {
    message: "{count} 人のメンバー",
    description:
      "The bold count in the notice under an enforced login requirement: how many members do not meet it (w1/m647, Render's wording)",
  },
  "workspaces.loginNoticeTwoFactor_other": {
    message: "チームの{members}がまだ二要素認証を有効にしていません。",
    description:
      "Notice under enforced 2FA when members have not enabled it; {members} is the bold count (Render's wording)",
  },
  "workspaces.loginNoticeGoogleLogin_other": {
    message: "チームの{members}がまだ Google でログインしていません。",
    description:
      "Notice under enforced Google login when members have not signed in with Google; {members} is the bold count (Render's wording)",
  },
  "workspaces.loginNoticeViewMembers_other": {
    message: "メンバーを表示",
    description:
      "Button on the notice that opens the list of members who do not meet the requirement (Render's wording)",
  },
  "workspaces.loginNoticeAllMembers": {
    message: "すべてのメンバー",
    description:
      "The bold words of the all-compliant notice (Render's wording)",
  },
  "workspaces.loginNoticeAllTwoFactor": {
    message: "チームの{members}が二要素認証を有効にしています！",
    description:
      "Notice under enforced 2FA when every member has enabled it; {members} is the bold 'All members' (Render's wording)",
  },
  "workspaces.loginNoticeAllGoogleLogin": {
    message: "チームの{members}が Google でログインしています！",
    description:
      "Notice under enforced Google login when every member has signed in with Google; {members} is the bold 'All members' (Render's wording)",
  },
  "workspaces.loginMembersTitleTwoFactor": {
    message: "二要素認証を使用していないメンバー",
    description:
      "Title of the dialog listing members without 2FA (Render's wording)",
  },
  "workspaces.loginMembersTitleGoogleLogin": {
    message: "Google でログインしていないメンバー",
    description:
      "Title of the dialog listing members who have not signed in with Google (Render's wording)",
  },
  "workspaces.loginMembersDescription_other": {
    message:
      "一覧のメンバーは現在、このチームワークスペースとそのリソースへのアクセスが禁止されています。",
    description:
      "Description of the dialog listing members who do not meet the enforced requirement (Render's wording)",
  },
  "workspaces.twoFactorRemindTitle_other": {
    message: "2FA を有効にするようリマインドしますか？",
    description:
      "Title of the reminder callout in the members-without-2FA dialog (Render's wording)",
  },
  "workspaces.twoFactorRemindBody_other": {
    message: "メールでリマインダーを送信します。",
    description:
      "Body of the reminder callout in the members-without-2FA dialog (Render's wording)",
  },
  "workspaces.twoFactorRemindAction_other": {
    message: "リマインダーを送信",
    description: "Button that emails the 2FA reminders (Render's wording)",
  },
  "workspaces.twoFactorRemindError": {
    message:
      "申し訳ありません。これらのチームメンバーにリマインダーを送信できませんでした。もう一度お試しください。",
    description:
      "Shown in the members dialog when the reminders could not be sent (Render's wording)",
  },
  "workspaces.twoFactorRemindSentToast": {
    message:
      "二要素認証を使用していないメンバーにリマインダーメールを送信しました！",
    description: "Toast after the 2FA reminders are sent (Render's wording)",
  },
  "workspaces.twoFactorRemindLimitTitle": {
    message: "リマインダーの上限に達しました！",
    description:
      "Title of the dialog shown when reminders were already sent today (Render's wording)",
  },
  "workspaces.twoFactorRemindLimitBody": {
    message:
      "本日はすでにリマインダーを送信しています。明日またリマインダーを送信できます。",
    description:
      "Body of the dialog shown when reminders were already sent today; the day is UTC (Render's wording)",
  },
  "workspaces.twoFactorConfirmReminder_other": {
    message: "一覧のメンバーに 2FA を有効にするリマインダーが送信されます。",
    description:
      "Line in the require-2FA confirmation: turning it on emails a reminder to each member listed (Render's wording)",
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
  "workspaces.orgDowngradeWarningTitle": {
    message: "Scale 未満にダウングレードすると組織が削除されます",
    description:
      "Change-plan dialog on an org's primary: title of the warning shown when a plan below Scale is selected (w7/m464)",
  },
  "workspaces.orgDowngradeWarning": {
    message:
      "これは組織のプライマリワークスペースで、そのプランは組織内のすべてのワークスペースに適用されます。ダウングレードすると組織と組織レベルの機能が削除され、このワークスペースは単独のワークスペースとして引き続き使用できます。先に組織のほかのワークスペースを削除し、請求担当・閲覧者・コントリビューターのロールを持つメンバーと保留中の招待を開発者または管理者に変更し、サービス、環境、このワークスペースのインバウンド IP ルールを削除してください。",
    description:
      "Change-plan dialog on an org's primary: what a downgrade below Scale does and Render's preconditions (render.com/docs/organizations, w7/m464)",
  },
  "workspaces.orgDowngradeWarningHobby": {
    message:
      "Hobby のメンバーは 1 人のため、このワークスペースのほかのメンバーも削除し、保留中の招待を取り消してください。",
    description:
      "Change-plan dialog on an org's primary with Hobby selected: the extra step a one-member plan needs, below the downgrade warning (w7/087)",
  },
  "workspaces.orgDowngradeBlocked": {
    message:
      "組織はまだ削除できません。Scale 未満にダウングレードする前に、次の対応が必要です。",
    description:
      "Change-plan dialog: lead-in above the unmet preconditions of ORGANIZATION_DOWNGRADE_BLOCKED (w7/m464)",
  },
  "workspaces.orgDowngradeSecondaryWorkspaces": {
    message: "組織のほかのワークスペースを削除する",
    description:
      "Unmet downgrade precondition secondary_workspaces; the workspace names follow (w7/m464)",
  },
  "workspaces.orgDowngradeMemberRoles": {
    message: "これらのメンバーのロールを開発者または管理者に変更する",
    description:
      "Unmet downgrade precondition member_roles; the members and roles follow (w7/m464)",
  },
  "workspaces.orgDowngradeInviteRoles": {
    message: "これらの保留中の招待のロールを開発者または管理者に変更する",
    description:
      "Unmet downgrade precondition invite_roles; the invited emails and roles follow (w7/m464)",
  },
  "workspaces.orgDowngradeMemberLimit": {
    message:
      "これらのメンバーと保留中の招待を削除する（Hobby のメンバーは 1 人です）",
    description:
      "Unmet downgrade precondition member_limit on a Hobby target: the members and pending invites beyond its one seat follow (w7/087)",
  },
  "workspaces.orgDowngradeInboundIpRules": {
    message: "インバウンド IP ルールを削除する",
    description:
      "Unmet downgrade precondition inbound_ip_rules; the workspace, environments and services that hold them follow (w7/m464)",
  },
  "workspaces.orgDowngradeItems": {
    message: "{label}：{items}",
    description:
      "One unmet downgrade precondition: {label} is what to do, {items} the list of what it applies to (w7/m464)",
  },
  "workspaces.orgDowngradePersonRole": {
    message: "{person}（{role}）",
    description:
      "A member or invitee in a downgrade precondition: {person} is their id or email, {role} the role to change (w7/m464)",
  },
  "workspaces.orgDowngradeIpRuleWorkspace": {
    message: "このワークスペース",
    description:
      "The workspace's own inbound IP rules, in the inbound_ip_rules precondition (w7/m464)",
  },
  "workspaces.orgDowngradeIpRuleEnvironment": {
    message: "環境 {name}",
    description:
      "An environment with inbound IP rules, in the inbound_ip_rules precondition (w7/m464)",
  },
  "workspaces.orgDowngradeIpRuleService": {
    message: "サービス {name}",
    description:
      "A service with inbound IP rules, in the inbound_ip_rules precondition (w7/m464)",
  },
  "workspaces.orgPlanOnPrimaryError": {
    message:
      "このワークスペースのプランは組織のプランです。組織のプライマリワークスペースで変更してください。",
    description:
      "Refusal for code ORGANIZATION_PLAN_ON_PRIMARY: a secondary workspace's plan changes only on the org's primary (w7/m464)",
  },
  "workspaces.orgPlanSecondaryNote": {
    message:
      "組織のプランに含まれています。プランと請求は組織のプライマリワークスペースで管理されます。",
    description:
      "Workspace Settings and Billing on an org's secondary workspace: its plan is the org's, managed on the primary (ADR115, w7/m464)",
  },
  "workspaces.orgPlanPrimarySettingsLink": {
    message: "プライマリワークスペースへ移動",
    description:
      "Link from a secondary workspace's settings to the org's primary workspace settings (w7/m464)",
  },
  "workspaces.orgPlanPrimaryBillingLink": {
    message: "プライマリワークスペースの請求へ移動",
    description:
      "Button on a secondary workspace's Billing plan card linking to the org's primary workspace's Billing (w7/m464)",
  },
  "workspaces.privacyTitle": {
    message: "ワークスペースの公開設定",
    description:
      "Workspace Settings → Workspace privacy card title, radio group name and section link (w7/m464)",
  },
  "workspaces.privacyDescription": {
    message: "組織内の誰がこのワークスペースに参加できるかを選択します。",
    description: "Description under the Workspace privacy card title (w7/m464)",
  },
  "workspaces.privacyPublic": {
    message: "公開",
    description: "The Public privacy setting's label, Render's name (w7/m464)",
  },
  "workspaces.privacyPublicDescription": {
    message:
      "組織のメンバーなら誰でも、自分でこのワークスペースに参加できます。",
    description:
      "What the Public privacy setting allows, Render's wording (render.com/docs/organizations, w7/m464)",
  },
  "workspaces.privacyInviteOnly": {
    message: "招待制",
    description:
      "The Invite-only privacy setting's label, Render's name (w7/m464)",
  },
  "workspaces.privacyInviteOnlyDescription": {
    message:
      "このワークスペースに招待できるのはワークスペースの管理者のみです。",
    description:
      "What the Invite-only privacy setting allows, Render's wording (render.com/docs/organizations, w7/m464)",
  },
  "workspaces.privacySave": {
    message: "保存",
    description: "Saves the selected workspace privacy (w7/m464)",
  },
  "workspaces.privacySaving": {
    message: "保存中…",
    description: "The Save button while the workspace privacy saves (w7/m464)",
  },
  "workspaces.privacySaved": {
    message: "ワークスペースの公開設定を保存しました",
    description: "Toast after setWorkspacePrivacy succeeds (w7/m464)",
  },
  "workspaces.privacyError": {
    message:
      "ワークスペースの公開設定を保存できませんでした。もう一度お試しください。",
    description:
      "Fallback toast when a privacy save fails without a server reason (w7/m464)",
  },
  "workspaces.loginRequirementRequiredByOrganization": {
    message: "{organization} のすべてのワークスペースで必須です。",
    description:
      "Workspace Settings, under the 2FA or Google login switch: why it is on and locked, the workspace's organization requires it in all of its workspaces (w7/m466)",
  },
  "workspaces.loginRequirementOrganizationSettingsLink": {
    message: "組織の設定で変更する",
    description:
      "Link for an org Owner after the required-by-organization note, to the organization's Settings page (w7/m466)",
  },
  "workspaces.twoFactorBlockedOrganizationTitle": {
    message: "{organization} ではすべてのワークスペースで二要素認証が必須です",
    description:
      "Heading of the 2FA blocked page when the organization's setting refuses the member, on a workspace or on the org's own pages (w7/m466)",
  },
  "workspaces.twoFactorBlockedOrganizationBody": {
    message:
      "この組織のオーナーが、組織とそのすべてのワークスペースで、すべてのメンバーに二要素認証（2FA）の使用を求めています。有効にするまで、これらにはアクセスできず、API キーと SSH キーも使用できません。",
    description:
      "Why the 2FA blocked page is shown when the organization's setting refuses the member (w7/m466)",
  },
  "workspaces.googleLoginBlockedOrganizationTitle": {
    message:
      "{organization} ではすべてのワークスペースで Google ログインが必須です",
    description:
      "Heading of the Google login blocked page when the organization's setting refuses the member (w7/m466)",
  },
  "workspaces.googleLoginBlockedOrganizationBody": {
    message:
      "この組織のオーナーが、組織とそのすべてのワークスペースで、すべてのメンバーに Google でのログインを求めています。bex のメールアドレスの Google アカウントでログインするまで、これらにはアクセスできず、ほかの方法でログインしているときに作成した API キーと SSH キーも使用できません。",
    description:
      "Why the Google login blocked page is shown when the organization's setting refuses the member (w7/m466)",
  },
};

export default jaWorkspaces;
