import type { TranslationEntry } from "@/i18n";

const jaUsage: Record<string, TranslationEntry> = {
  "usage.pageTitle": {
    message: "請求",
    description:
      "Billing page heading and browser title (renamed from Usage in w5/m70)",
  },
  "usage.pageSubtitle": {
    message: "お支払い、請求書、今月これまでのワークスペースの使用状況",
    description: "Billing page subtitle beneath the heading",
  },
  "usage.sectionNavigation": {
    message: "請求のセクション",
    description: "Accessible label for the billing page section navigation",
  },
  "usage.resourceCapsServices": {
    message: "サービス",
    description: "Service count label in the resource-cap card",
  },
  "usage.resourceCapsPostgres": {
    message: "Postgres",
    description: "Postgres count label in the resource-cap card",
  },
  "usage.resourceCapsKeyValues": {
    message: "Key Value",
    description: "Key Value count label in the resource-cap card",
  },
  "usage.resourceCapsFree": {
    message: "無料: {used} / {limit}",
    description: "Free-instance count in the resource-cap card (w4/m241)",
  },
  "usage.freeHoursTitle": {
    message: "月間の無料利用枠",
    description:
      "Title of the Free instance hours card on the usage page (w8/m139)",
  },
  "usage.freeHoursDescription": {
    message:
      "無料の Web Service は、ワークスペースごとに月間のインスタンス時間の枠を共有します。スピンダウン中の時間はカウントされません。",
    description: "Explains the monthly Free instance hours allowance (w8/m139)",
  },
  "usage.freeHoursUsed": {
    message: "今月の無料インスタンス時間 {included} のうち {used} を使用",
    description:
      "Free instance hours used against the monthly allowance (w8/m139)",
  },
  "usage.freeHoursNear": {
    message:
      "利用枠の上限に近づいています。使い切ると、無料の Web Service は来月の初めまで停止されます。",
    description:
      "Warning once Free instance hours reach 80 percent of the allowance (w8/m139)",
  },
  "usage.freeHoursSpent": {
    message:
      "利用枠を使い切りました。無料の Web Service は来月の初めまで停止されます。今すぐ実行するには、サービスを有料プランにアップグレードしてください。",
    description:
      "Notice once the monthly Free instance hours are spent (w8/m139)",
  },
  "usage.buildMinutesTitle": {
    message: "パイプライン時間",
    description:
      "Title of the included build minutes card on the billing page (w6/m308)",
  },
  "usage.buildMinutesDescription": {
    message:
      "ビルドとプレデプロイコマンドはパイプライン時間を消費します。プランには毎月一定の時間が含まれ、それを超えた分は 1 分単位で課金されます。",
    description: "Explains the plan's included build minutes (w6/m308)",
  },
  "usage.buildMinutesUsed": {
    message: "今月の含まれるビルド時間 {included} のうち {used} を使用",
    description:
      "Build minutes used against the plan's included minutes (w6/m308)",
  },
  "usage.buildMinutesNear": {
    message:
      "含まれるビルド時間の上限に近づいています。お支払い方法がない場合、使い切ると来月の初めまで新しいビルドとプレデプロイコマンドが停止します。",
    description:
      "Warning once build minutes reach 80 percent of the included minutes (w6/m308)",
  },
  "usage.buildMinutesSpent": {
    message:
      "含まれるビルド時間を使い切りました。それを超えた分は 1 分単位で課金されます。お支払い方法がない場合、来月の初めまで新しいビルドとプレデプロイコマンドが停止します。",
    description: "Notice once the included build minutes are spent (w6/m308)",
  },
  "usage.bandwidthTitle": {
    message: "アウトバウンド帯域幅",
    description:
      "Title of the included bandwidth card on the billing page (w6/m319)",
  },
  "usage.bandwidthDescription": {
    message:
      "サービスからインターネットへ送信するトラフィックは帯域幅を消費します。プランには毎月一定の帯域幅が含まれ、それを超えた分は GB 単位で課金されます。",
    description: "Explains the plan's included outbound bandwidth (w6/m319)",
  },
  "usage.bandwidthUsed": {
    message: "今月の含まれる帯域幅 {included} GB のうち {used} を使用",
    description:
      "Outbound bandwidth used against the plan's included bandwidth, in GB (w6/m319)",
  },
  "usage.bandwidthNear": {
    message:
      "含まれる帯域幅の上限に近づいています。お支払い方法がない場合、使い切るとワークスペースのサービスは来月の初めまでスピンダウンします。",
    description:
      "Warning once bandwidth reaches 80 percent of the included bandwidth (w6/m319)",
  },
  "usage.bandwidthSpent": {
    message:
      "含まれる帯域幅を使い切りました。それを超えた分は GB 単位で課金されます。お支払い方法がない場合、ワークスペースのサービスは来月の初めまでスピンダウンします。",
    description: "Notice once the included bandwidth is spent (w6/m319)",
  },
  "usage.customDomainsTitle": {
    message: "カスタムドメイン",
    description:
      "Title of the included custom domains card on the billing page (w7/m429)",
  },
  "usage.customDomainsDescription": {
    message:
      "プランには一定数の検証済みカスタムドメインが含まれます。それを超える各ドメインは、検証されている期間に応じて 1 件ごとに月単位で課金されます。件数を理由にドメインの追加が拒否されることはありません。",
    description:
      "Explains the plan's included custom domains and how domains beyond them are billed (w7/m429)",
  },
  "usage.customDomainsUsed": {
    message:
      "プランに含まれる {included} 件中 {used} 件のカスタムドメインを使用中です",
    description:
      "Verified custom domains now against the count the plan includes (w7/m429)",
  },
  "usage.customDomainsNear": {
    message:
      "プランに含まれるカスタムドメインの件数に近づいています。超過したドメインは 1 件ごとに月単位で課金されます。",
    description:
      "Note once the verified custom domains reach 80 percent of the included count (w7/m429)",
  },
  "usage.customDomainsSpent": {
    message:
      "プランに含まれるカスタムドメインはすべて使用中です。追加で検証されたドメインは 1 件ごとに月単位で課金されます。",
    description: "Notice once every included custom domain is in use (w7/m429)",
  },
  "usage.categoryCustomDomains": {
    message: "カスタムドメイン",
    description:
      "Charge-tree category for a workspace's custom domains beyond its plan (w7/m429)",
  },
  "usage.chargeCronMinimum": {
    message: "Cron Job の最低料金",
    description:
      "Charge line under a cron job: what its runs in a month cost short of the cron job minimum, charged once the month ends (w9/m448)",
  },
  "usage.chargeCronMinimumRate": {
    message: "月額最低 ${minimum}",
    description:
      "Rate column value for the cron job minimum line; minimum is the USD amount without the currency symbol (w9/m448)",
  },
  "usage.chargeCustomDomains": {
    message: "プラン超過分のカスタムドメイン",
    description:
      "Charge line for custom domains beyond the plan's included count, billed per domain-month (w7/m429)",
  },
  "usage.chargeIncluded": {
    message: "{included} 含まれる",
    description:
      "Part of a charge line's quantity that the plan's included build minutes or bandwidth covered (w6/m308, w6/m319)",
  },
  "usage.pipelineTitle": {
    message: "ビルドパイプライン",
    description:
      "Title of the Build Pipeline card in workspace settings (w6/m309)",
  },
  "usage.pipelineDescription": {
    message:
      "ビルドとプレデプロイコマンドはパイプライン時間を消費します。プランに含まれる時間を超えた分は 1 分単位で課金され、設定した月間の支出上限までとなります。上限に達すると、来月までビルドが停止します。",
    description: "Explains the pipeline spend limit (w6/m309)",
  },
  "usage.pipelineSpendLimitValue": {
    message: "支出上限: 月額 {limit}",
    description: "The workspace pipeline spend limit (w6/m309)",
  },
  "usage.pipelineSpendLimitNone": {
    message: "支出上限なし",
    description: "The workspace has no pipeline spend limit (w6/m309)",
  },
  "usage.pipelineSpendLimitSet": {
    message: "支出上限を設定",
    description: "Button that opens the pipeline spend limit dialog (w6/m309)",
  },
  "usage.pipelineSpendLimitEdit": {
    message: "編集",
    description: "Button that edits the pipeline spend limit (w6/m309)",
  },
  "usage.pipelineSpendLimitRemove": {
    message: "上限を削除",
    description: "Button that removes the pipeline spend limit (w6/m309)",
  },
  "usage.pipelineSpendLimitDialogTitle": {
    message: "支出上限を設定",
    description: "Title of the pipeline spend limit dialog (w6/m309)",
  },
  "usage.pipelineSpendLimitDialogDescription": {
    message:
      "プランに含まれるビルド時間を超えた分について、このワークスペースが 1 か月に支出する上限額です。上限に達すると、来月の初めまで新しいビルドとプレデプロイコマンドが停止します。",
    description: "Explains what the pipeline spend limit caps (w6/m309)",
  },
  "usage.pipelineSpendLimitLabel": {
    message: "月間の上限（USD）",
    description: "Label of the pipeline spend limit input (w6/m309)",
  },
  "usage.pipelineSpendLimitSave": {
    message: "保存",
    description: "Saves the pipeline spend limit (w6/m309)",
  },
  "usage.pipelineSpendLimitSaved": {
    message: "支出上限を月額 {limit} に設定しました",
    description: "Toast after setting the pipeline spend limit (w6/m309)",
  },
  "usage.pipelineSpendLimitRemoved": {
    message: "支出上限を削除しました",
    description: "Toast after removing the pipeline spend limit (w6/m309)",
  },
  "usage.pipelineSpendLimitError": {
    message: "支出上限を更新できませんでした",
    description: "Toast when updating the pipeline spend limit fails (w6/m309)",
  },
  "usage.pipelineSpendLimitUnreadable": {
    message:
      "支出上限を読み込めませんでした。再読み込みしてもう一度お試しください。",
    description: "The pipeline spend limit could not be read (w6/m309)",
  },
  "usage.pipelineSpendLimitForbidden": {
    message: "支出上限を表示・変更できるのは、請求の管理者のみです。",
    description: "The caller may not read the pipeline spend limit (w6/m309)",
  },
  "usage.resourceCapsNearLimit": {
    message: "上限に近づいています",
    description:
      "Warning shown once a resource count reaches 80 percent of its cap, below the cap itself",
  },
  "usage.resourceCapsLimitReached": {
    message: "上限に達しました",
    description:
      "Shown when a resource count is at or over its cap, so creating another is refused",
  },
  "usage.resourceCapsFinishingDeletion": {
    message: "{count} 件が削除処理中",
    description:
      "Sub-line on a resource-cap tile: how many of the counted resources are mid-deletion and thus hidden from the resource list but still holding quota (w6/m129)",
  },
  "usage.resourceCapsFinishingDeletionHint": {
    message: "削除が完了するまで、上限にカウントされます",
    description:
      "Tooltip explaining why resources finishing deletion are included in the used count",
  },
  "usage.billingDetailsDeniedTitle": {
    message: "あなたのロールでは請求の詳細を表示できません",
    description:
      "Billing page heading for a Contributor or a Viewer, whose role may not view billing details (Render's View billing details row, w9/m620)",
  },
  "usage.errorTitle": {
    message: "使用量を読み込めませんでした",
    description: "Error state heading on the Usage page",
  },
  "usage.monthPickerLabel": {
    message: "月を選択",
    description: "Aria-label for the month-picker select on the Usage page",
  },
  "usage.currentMonth": {
    message: "今月",
    description:
      "Default option in the month picker meaning the current calendar month",
  },
  "usage.creditsTitle": {
    message: "クレジット残高",
    description: "Heading of the remaining billing-credit card (w5/m70)",
  },
  "usage.creditsDescription": {
    message:
      "お支払い方法に請求する前に、請求書へ適用されるプロモーションクレジット",
    description: "Subtitle of the remaining billing-credit card",
  },
  "usage.creditsExpiryNote": {
    message: "うち {amount} は {date} に失効します",
    description:
      "Earliest-expiring grant note beside the credit balance; amount is a $ value, date is YYYY-MM-DD",
  },
  "usage.creditsCardStillRequired": {
    message:
      "クレジットがある場合でも、お支払い方法は必要です。クレジットが先に請求書へ充当され、残りの金額がカードで支払われます。",
    description:
      "ADR046 clarification on the credit card — credit does not replace payment onboarding",
  },
  "usage.colInvoicePeriod": {
    message: "期間",
    description: "Table column header for an invoice's billing period start",
  },
  "usage.colInvoiceStatus": {
    message: "ステータス",
    description: "Table column header for an invoice's status",
  },
  "usage.colInvoiceAmount": {
    message: "金額",
    description: "Table column header for an invoice's amount",
  },
  "usage.billingSetupTitle": {
    message: "請求の設定",
    description: "Customer billing onboarding card title",
  },
  "usage.billingSetupDescription": {
    message: "このワークスペースの支払い回収と税務の準備状況",
    description: "Customer billing onboarding card description",
  },
  "usage.billingTestMode": {
    message: "Stripe テストモード",
    description: "Badge making the non-live Stripe environment explicit",
  },
  "usage.billingMode": {
    message: "Stripe Billing",
    description: "Fallback badge for the Stripe billing environment",
  },
  "usage.billingReady": {
    message: "準備完了",
    description: "Billing onboarding status is ready",
  },
  "usage.billingActionNeeded": {
    message: "対応が必要です",
    description: "Billing onboarding status needs action",
  },
  "usage.billingCustomerStatus": {
    message: "顧客",
    description: "Stripe Customer readiness row",
  },
  "usage.billingSubscriptionStatus": {
    message: "従量課金サブスクリプション",
    description: "Stripe Subscription readiness row",
  },
  "usage.billingPaymentStatus": {
    message: "お支払い方法",
    description: "Default payment method readiness row",
  },
  "usage.billingTaxStatus": {
    message: "自動税計算",
    description: "Stripe Tax activation row",
  },
  "usage.billingLifecycleStatus": {
    message: "回収ライフサイクル",
    description: "Stripe collection and reversible enforcement state row",
  },
  "usage.billingLifecycleGrace": {
    message:
      "お支払いに失敗しました（{reason}）。猶予期間中はワークスペースを引き続き利用できます。{deadline} 以降に、元に戻せる一時停止が予定されています。",
    description: "Visible billing grace state and authoritative deadline",
  },
  "usage.billingLifecycleEnforced": {
    message:
      "請求の執行が有効です（{reason}）。コンピュートは一時停止されていますが、データベース、Key Value のデータ、シークレット、請求の証跡は削除されていません。",
    description: "Visible reversible billing enforcement state",
  },
  "usage.billingLifecycleRecovering": {
    message:
      "お支払いが回復しました。bex は、請求の執行によって変更されたリソースのみを復元しています。",
    description: "Visible precise recovery state",
  },
  "usage.billingLifecycleExcluded": {
    message:
      "このワークスペースは、運用担当者によって Stripe の回収対象から除外されています。",
    description: "Visible structural billing exclusion state",
  },
  "usage.billingLifecycleComped": {
    message:
      "このワークスペースは料金が計算されますが、運用担当者によって全額が無償にされています。",
    description: "Visible rated-but-free comp state",
  },
  "usage.billingLifecycleUnknown": {
    message:
      "請求の状態は {reason} です。請求ポータルを使用するか、サポートにお問い合わせください。",
    description: "Forward-compatible unknown billing lifecycle state",
  },
  "usage.billingNoDeadline": {
    message: "期限は報告されていません",
    description: "Fallback when a malformed grace state has no deadline",
  },
  "usage.billingOff": {
    message: "オフ",
    description:
      "Neutral status for a deliberately disabled billing capability (e.g. tax not activated)",
  },
  "usage.billingAddPayment": {
    message: "お支払い方法を追加",
    description: "Button opening setup-mode Stripe Checkout (live mode)",
  },
  "usage.billingUpdatePayment": {
    message: "お支払い方法を更新",
    description: "Button reopening setup-mode Stripe Checkout (live mode)",
  },
  "usage.billingAddPaymentTest": {
    message: "テスト用のお支払い方法を追加",
    description:
      "Button opening setup-mode Stripe Checkout when Stripe is in test mode",
  },
  "usage.billingUpdatePaymentTest": {
    message: "テスト用のお支払い方法を更新",
    description:
      "Button reopening setup-mode Stripe Checkout when Stripe is in test mode",
  },
  "usage.billingOpenPortal": {
    message: "請求ポータルを開く",
    description: "Button opening Stripe Customer Portal",
  },
  "usage.billingHostedNote": {
    message:
      "お支払いの詳細は、Stripe がホストするページでのみ入力されます。bex が Stripe のサーバーキーをブラウザーに送信することはありません。",
    description: "Hosted billing security note",
  },
  "usage.billingUnavailable": {
    message:
      "現在、請求の詳細を利用できません。しばらくしてからもう一度お試しください。",
    description:
      "Billing readiness could not be read (a Stripe outage, the integration off). Every role that reaches the card may view billing details since w9/m620, so this is never a role refusal",
  },
  "usage.billingCheckoutError": {
    message: "Stripe Checkout を開けませんでした。もう一度お試しください。",
    description: "Toast after Checkout session creation fails",
  },
  "usage.billingPortalError": {
    message:
      "Stripe の請求ポータルを開けませんでした。もう一度お試しください。",
    description: "Toast after Portal session creation fails",
  },
  "usage.paymentRequiredTitle": {
    message: "続行するにはお支払い方法を追加してください",
    description: "Just-in-time paid-intent onboarding dialog title",
  },
  "usage.paymentRequiredDescription": {
    message:
      "この有料プランにはお支払い方法が必要です。新しいタブで Stripe Checkout を完了してください。準備ができると、この操作は自動的に再開されます。",
    description: "Just-in-time paid-intent onboarding explanation",
  },
  "usage.paymentRequiredRetrying": {
    message: "お支払い方法が見つかりました。操作を再試行しています…",
    description: "Status while resuming the interrupted paid mutation",
  },
  "usage.paymentRequiredCancel": {
    message: "キャンセル",
    description: "Cancel just-in-time payment onboarding",
  },
  "usage.planTitle": {
    message: "プラン",
    description: "Plan card title on the billing page",
  },
  "usage.planChange": {
    message: "プランを変更",
    description:
      "Link from the billing page's plan card to the workspace-settings plan dialog",
  },
  "usage.paymentMethodTitle": {
    message: "お支払い方法",
    description: "Payment-method card title on the billing page",
  },
  "usage.paymentMethodCard": {
    message: "{brand}（末尾 {last4}）",
    description: "Names the card on file, e.g. 'Visa ending in 4242'",
  },
  "usage.paymentMethodOnFile": {
    message: "お支払い方法が登録されています",
    description:
      "Shown when a payment method exists but is not a card the provider could name",
  },
  "usage.paymentMethodNone": {
    message: "お支払い方法がありません",
    description: "Shown when the workspace has no payment method",
  },
  "usage.includedUsageTitle": {
    message: "含まれる使用量",
    description:
      "Plan-allowance card title on the billing page (was 'Resource limits')",
  },
  "usage.includedUsageDescription": {
    message: "このワークスペースのプランに含まれる内容と、その使用状況です。",
    description: "Included-usage card description",
  },
  "usage.chargesTitle": {
    message: "料金",
    description: "Charge-tree card title on the billing page",
  },
  "usage.chargesDescriptionEstimate": {
    message:
      "今期これまでに発生した料金を、bex の料金表に基づいて算出したものです。請求書ではなく、見積もりです。",
    description:
      "Charge-tree description when no Stripe subscription prices the period",
  },
  "usage.chargesDescriptionInvoiced": {
    message: "今期これまでに発生した料金を、Stripe が算出したものです。",
    description:
      "Charge-tree description when a real Stripe amount is available. Says the total is Stripe's rating rather than the amount it will invoice: credits and comp discounts can sit between the two, and the amount actually due gets its own line (w6/m98).",
  },
  "usage.amountDueAfterCredits": {
    message: "クレジット適用後の請求額",
    description:
      "Label for the charge-tree line showing what Stripe actually collects once credits and discounts are applied to the charge above it",
  },
  "usage.chargesEmpty": {
    message: "この期間の使用量はありません。",
    description: "Charge-tree empty state",
  },
  "usage.coveragePartial": {
    message: "一部のデータのみ",
    description:
      "Amber caveat label above the charges when the metering behind the estimate is degraded/incomplete (w4/048), mirroring the Metrics page's degraded badge",
  },
  "usage.coveragePartialLead": {
    message:
      "この見積もりは、実際の使用量より少なくなっている可能性があります。",
    description: "Lead sentence of the partial-coverage caveat tooltip",
  },
  "usage.coveragePartialThrough": {
    message: "計測済みの範囲: {through} まで。",
    description:
      "Partial-coverage caveat clause naming the date the meters have recorded through (a degraded hour counts as recorded, w4/496); {through} is a YYYY-MM-DD date",
  },
  "usage.coveragePartialSources": {
    message: "計測が低下しているソース: {sources}。",
    description:
      "Partial-coverage caveat clause listing the degraded metering source names; {sources} is a comma-separated list",
  },
  "usage.expandAll": {
    message: "すべて展開",
    description: "Charge-tree button that opens every category",
  },
  "usage.collapseAll": {
    message: "すべて折りたたむ",
    description: "Charge-tree button that closes every category",
  },
  "usage.totalToDate": {
    message: "今月これまでの合計",
    description: "Charge-tree total row for the month in progress",
  },
  "usage.totalForPeriod": {
    message: "期間の合計",
    description: "Charge-tree total row for a completed past month",
  },
  "usage.totalBillingPeriodWindow": {
    message: "今回の請求期間の合計（{start} – {end}）",
    description:
      "Charge total row when the headline is Stripe's rated amount: it covers the subscription period (anchored on the subscription's day of month), not the calendar month (w4/m139)",
  },
  "usage.totalBillingPeriod": {
    message: "今回の請求期間の合計",
    description:
      "Charge total row for Stripe's rated amount when its period bounds are unknown (w4/m139)",
  },
  "usage.breakdownMonthToDate": {
    message: "上記の内訳: {month} のこれまで",
    description:
      "Row under a Stripe-rated total: the charge tree's own sum and window (the calendar month), which differs from the subscription period (w4/m139)",
  },
  "usage.projectedTotal": {
    message: "{month} の予測合計",
    description:
      "Charge-tree straight-line projection of month-end spend, for the estimate fallback whose total accrues over the calendar month",
  },
  "usage.projectedTotalBillingPeriod": {
    message: "今回の請求期間の予測",
    description:
      "Charge-tree straight-line projection of a Stripe-rated total. The subscription period need not align with the calendar month (it can span two), so no month is named (w6/050).",
  },
  "usage.chargeFree": {
    message: "含まれる",
    description: "Rate column value for a charge line that is priced at zero",
  },
  "usage.resourceDeleted": {
    message: "（削除済み）",
    description:
      "Marker beside a charged resource that no longer exists; the name shown is the one bex retained",
  },
  "usage.categoryServices": {
    message: "サービス",
    description: "Charge-tree category for App services",
  },
  "usage.categoryPostgres": {
    message: "Postgres",
    description: "Charge-tree category for managed Postgres instances",
  },
  "usage.categoryKeyValue": {
    message: "Key Value",
    description: "Charge-tree category for managed Key Value instances",
  },
  "usage.categoryObjectStorage": {
    message: "オブジェクトストレージ",
    description:
      "Charge-tree category for a workspace's object storage (ADR112)",
  },
  "usage.categoryAgentSessions": {
    message: "エージェントセッション",
    description:
      "Charge-tree category for platform model usage of agent sessions",
  },
  "usage.chargeDisk": {
    message: "ディスク",
    description: "Charge line for a service's provisioned persistent disk",
  },
  "usage.chargeAgentModelCost": {
    message: "モデルの使用量",
    description:
      "Charge line for platform model spend, billed at provider cost plus markup (ADR091)",
  },
  "usage.chargeAgentModelRate": {
    message: "プロバイダー原価 + 30%",
    description: "Rate column value for the platform model spend line",
  },
  "usage.chargeAgentModelInputTokens": {
    message: "入力トークン",
    description: "Breakdown line for platform model input tokens",
  },
  "usage.chargeAgentModelOutputTokens": {
    message: "出力トークン",
    description: "Breakdown line for platform model output tokens",
  },
  "usage.chargeAgentModelCacheReadTokens": {
    message: "キャッシュ読み取りトークン",
    description: "Breakdown line for platform model cache-read tokens",
  },
  "usage.chargeAgentModelCacheWriteTokens": {
    message: "キャッシュ書き込みトークン",
    description: "Breakdown line for platform model cache-write tokens",
  },
  "usage.categorySandboxes": {
    message: "サンドボックス",
    description: "Charge-tree category for hosted agent sandboxes",
  },
  "usage.chargeCompute": {
    message: "コンピュート",
    description: "Charge line for metered instance time",
  },
  "usage.chargeBandwidth": {
    message: "帯域幅",
    description: "Charge line for metered outbound bandwidth",
  },
  "usage.chargeBuild": {
    message: "ビルド時間",
    description: "Charge line for metered build time",
  },
  "usage.chargeStorage": {
    message: "ストレージ",
    description: "Charge line for metered datastore storage",
  },
  "usage.chargeSandboxCompute": {
    message: "サンドボックスのコンピュート",
    description: "Charge line for metered sandbox compute",
  },
  "usage.chargeSandboxMemory": {
    message: "サンドボックスのメモリ",
    description:
      "Charge line for the memory a running sandbox was allocated, billed per GiB-hour since billing switched to use",
  },
  "usage.chargeSandboxMemoryUnbilled": {
    message: "サンドボックスのメモリ（コンピュートに含まれる）",
    description:
      "Charge line for the allocated memory a sandbox ran with before billing switched to use; recorded beside sandbox compute, which already includes it, so it is shown and not billed separately",
  },
  "usage.chargeSandboxCpu": {
    message: "サンドボックスの CPU",
    description:
      "Charge line for the CPU a running sandbox used, billed per CPU-hour used",
  },
  "usage.chargeSandboxCpuUnbilled": {
    message: "サンドボックスの CPU（コンピュートに含まれる）",
    description:
      "Charge line for the CPU a sandbox used before billing switched to use; recorded beside sandbox compute, which already charged for it, so it is shown and not billed separately",
  },
  "usage.sandboxUsageBilledFromNote": {
    message:
      "{date} UTC 以降、実行中のサンドボックスは使用した CPU と割り当てられたメモリに対して課金されます。それ以前の利用はサンドボックスのコンピュートとして課金されました。",
    description:
      "Note under the charge tree of a period with sandbox usage; date is the instant billing switched, formatted in UTC",
  },
  "usage.chargeSandboxSnapshotStorage": {
    message: "サンドボックスのスナップショットストレージ",
    description:
      "Charge line for the image a suspended sandbox retains, billed per GB-month",
  },
  "usage.chargeObjectStorage": {
    message: "ストレージ",
    description:
      "Charge line for the data a workspace keeps in object storage, billed per GB-month (shown at no charge until a price is set, ADR112 D8)",
  },
  "usage.chargeObjectStorageIngress": {
    message: "インバウンド帯域幅",
    description:
      "Charge line for the bytes put into a workspace's object storage (shown at no charge, ADR112 D8)",
  },
  "usage.chargeObjectStorageEgress": {
    message: "アウトバウンド帯域幅",
    description:
      "Charge line for the bytes read out of a workspace's object storage, public reads included (shown at no charge, ADR112 D8)",
  },
  "usage.creditsTotalLabel": {
    message: "合計残高",
    description: "Label above the credit balance amount",
  },
  "usage.invoiceHistoryTitle": {
    message: "請求書の履歴",
    description: "Invoice-history card title on the billing page",
  },
  "usage.invoiceHistoryDescription": {
    message: "確定済みの請求書です。",
    description: "Invoice-history card description",
  },
  "usage.invoiceKindAgentModelCharge": {
    message: "エージェントモデルの料金",
    description:
      "Label on an invoice-history row for a charge made when agent model spend reached its threshold",
  },
  "usage.invoiceHistoryChargesNote": {
    message:
      "月中に発生したエージェントモデルの料金は、その月の請求書から差し引かれます。",
    description:
      "Invoice-history note when the list includes agent model charges",
  },
  "usage.creditsAppliedLine": {
    message: "適用されたクレジット −{applied} → 請求額 {due}",
    description:
      "Credit applied to the current period and the remaining amount due",
  },
  "usage.chargesDescriptionPending": {
    message:
      "今期これまでに発生した料金を、bex の料金表に基づいて算出したものです。",
    description:
      "Charges card description while the invoiced total is still loading; deliberately states only what is already true, without claiming the figure is or is not a Stripe invoice.",
  },
  "usage.paymentMethodRejectedPrepaid": {
    message:
      "プリペイドカードは使用できません。クレジットカードまたはデビットカードを追加してください。",
    description:
      "Shown when the last card offered was refused at binding because it is prepaid (ADR091 D10.5)",
  },
  "usage.paymentMethodRejectedUnpaidBalance": {
    message:
      "このカードには未払いの残高があります。別のカードを追加してください。",
    description:
      "Shown when the last card offered was refused because it carries an unpaid written-off balance (ADR091 D10.6)",
  },
  "usage.agentModelTitle": {
    message: "エージェントモデル",
    description:
      "Billing page card and nav entry for agent model spend (ADR091 D6)",
  },
  "usage.agentModelPricing": {
    message: "モデルプロバイダーの原価 + 30% で課金されます。",
    description: "Pricing statement under the Agent model title",
  },
  "usage.agentModelMonthToDate": {
    message: "今月これまで",
    description: "Label of the current month's agent model total",
  },
  "usage.agentModelPeriodTotal": {
    message: "月の合計",
    description: "Label of a closed month's agent model total",
  },
  "usage.agentModelBreakdown_other": {
    message: "モデル別の内訳（{count} モデル）",
    description: "Toggle that expands the per-model breakdown",
  },
  "usage.agentModelEmpty": {
    message: "この期間のエージェントモデルの使用量はありません。",
    description: "Shown when the period has no agent model spend",
  },
  "usage.lessThanAmount": {
    message: "<{amount}",
    description:
      "A charge that rounds below the smallest amount shown, e.g. <$0.01",
  },
};

export default jaUsage;
