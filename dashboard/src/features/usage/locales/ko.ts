import type { TranslationEntry } from "@/i18n";

const koUsage: Record<string, TranslationEntry> = {
  "usage.pageTitle": {
    message: "결제",
    description:
      "Billing page heading and browser title (renamed from Usage in w5/m70)",
  },
  "usage.pageSubtitle": {
    message: "결제 수단, 청구서, 이번 달 워크스페이스 사용량",
    description: "Billing page subtitle beneath the heading",
  },
  "usage.sectionNavigation": {
    message: "결제 섹션",
    description: "Accessible label for the billing page section navigation",
  },
  "usage.resourceCapsServices": {
    message: "서비스",
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
    message: "무료: {limit}개 중 {used}개",
    description: "Free-instance count in the resource-cap card (w4/m241)",
  },
  "usage.freeHoursTitle": {
    message: "월 포함 사용량",
    description:
      "Title of the Free instance hours card on the usage page (w8/m139)",
  },
  "usage.freeHoursDescription": {
    message:
      "무료 Web Service는 워크스페이스당 매월 인스턴스 시간 허용량을 공유합니다. 절전 상태로 보낸 시간은 포함되지 않습니다.",
    description: "Explains the monthly Free instance hours allowance (w8/m139)",
  },
  "usage.freeHoursUsed": {
    message: "이번 달 무료 인스턴스 시간 {included} 중 {used} 사용",
    description:
      "Free instance hours used against the monthly allowance (w8/m139)",
  },
  "usage.freeHoursNear": {
    message:
      "허용량에 가까워졌습니다. 허용량을 모두 사용하면 다음 달 초까지 무료 Web Service가 일시 중지됩니다.",
    description:
      "Warning once Free instance hours reach 80 percent of the allowance (w8/m139)",
  },
  "usage.freeHoursSpent": {
    message:
      "허용량을 모두 사용했습니다. 다음 달 초까지 무료 Web Service가 일시 중지됩니다. 지금 실행하려면 서비스를 유료 요금제로 업그레이드하세요.",
    description:
      "Notice once the monthly Free instance hours are spent (w8/m139)",
  },
  "usage.buildMinutesTitle": {
    message: "파이프라인 시간",
    description:
      "Title of the included build minutes card on the billing page (w6/m308)",
  },
  "usage.buildMinutesDescription": {
    message:
      "빌드와 사전 배포 명령은 파이프라인 시간을 사용합니다. 요금제에 매월 일정량이 포함되어 있으며, 초과분은 분 단위로 청구됩니다.",
    description: "Explains the plan's included build minutes (w6/m308)",
  },
  "usage.buildMinutesUsed": {
    message: "이번 달 포함된 빌드 시간 {included} 중 {used} 사용",
    description:
      "Build minutes used against the plan's included minutes (w6/m308)",
  },
  "usage.buildMinutesNear": {
    message:
      "포함된 빌드 시간에 가까워졌습니다. 결제 수단이 없으면 소진 시 다음 달 초까지 새 빌드와 사전 배포 명령이 중지됩니다.",
    description:
      "Warning once build minutes reach 80 percent of the included minutes (w6/m308)",
  },
  "usage.buildMinutesSpent": {
    message:
      "포함된 빌드 시간을 모두 사용했습니다. 추가 시간은 분 단위로 청구되며, 결제 수단이 없으면 다음 달 초까지 새 빌드와 사전 배포 명령이 중지됩니다.",
    description: "Notice once the included build minutes are spent (w6/m308)",
  },
  "usage.bandwidthTitle": {
    message: "아웃바운드 대역폭",
    description:
      "Title of the included bandwidth card on the billing page (w6/m319)",
  },
  "usage.bandwidthDescription": {
    message:
      "서비스가 인터넷으로 보내는 트래픽은 대역폭을 사용합니다. 요금제에 매월 일정량이 포함되어 있으며, 초과분은 GB 단위로 청구됩니다.",
    description: "Explains the plan's included outbound bandwidth (w6/m319)",
  },
  "usage.bandwidthUsed": {
    message: "이번 달 포함된 대역폭 {included} GB 중 {used} 사용",
    description:
      "Outbound bandwidth used against the plan's included bandwidth, in GB (w6/m319)",
  },
  "usage.bandwidthNear": {
    message:
      "포함된 대역폭에 가까워졌습니다. 결제 수단이 없으면 소진 시 다음 달 초까지 워크스페이스의 서비스가 절전 상태가 됩니다.",
    description:
      "Warning once bandwidth reaches 80 percent of the included bandwidth (w6/m319)",
  },
  "usage.bandwidthSpent": {
    message:
      "포함된 대역폭을 모두 사용했습니다. 추가 대역폭은 GB 단위로 청구되며, 결제 수단이 없으면 다음 달 초까지 워크스페이스의 서비스가 절전 상태가 됩니다.",
    description: "Notice once the included bandwidth is spent (w6/m319)",
  },
  "usage.customDomainsTitle": {
    message: "사용자 지정 도메인",
    description:
      "Title of the included custom domains card on the billing page (w7/m429)",
  },
  "usage.customDomainsDescription": {
    message:
      "플랜에는 일정 수의 인증된 사용자 지정 도메인이 포함됩니다. 이를 초과하는 도메인은 인증된 기간만큼 도메인당 매월 청구됩니다. 도메인 수 때문에 추가가 거부되지는 않습니다.",
    description:
      "Explains the plan's included custom domains and how domains beyond them are billed (w7/m429)",
  },
  "usage.customDomainsUsed": {
    message:
      "플랜에 포함된 {included}개 중 {used}개의 사용자 지정 도메인 사용 중",
    description:
      "Verified custom domains now against the count the plan includes (w7/m429)",
  },
  "usage.customDomainsNear": {
    message:
      "플랜에 포함된 사용자 지정 도메인 수에 가까워지고 있습니다. 초과하는 도메인은 도메인당 매월 청구됩니다.",
    description:
      "Note once the verified custom domains reach 80 percent of the included count (w7/m429)",
  },
  "usage.customDomainsSpent": {
    message:
      "플랜에 포함된 사용자 지정 도메인을 모두 사용 중입니다. 추가로 인증된 도메인은 도메인당 매월 청구됩니다.",
    description: "Notice once every included custom domain is in use (w7/m429)",
  },
  "usage.categoryCustomDomains": {
    message: "사용자 지정 도메인",
    description:
      "Charge-tree category for a workspace's custom domains beyond its plan (w7/m429)",
  },
  "usage.chargeCronMinimum": {
    message: "Cron Job 최소 요금",
    description:
      "Charge line under a cron job: what its runs in a month cost short of the cron job minimum, charged once the month ends (w9/m448)",
  },
  "usage.chargeCronMinimumRate": {
    message: "월 최소 ${minimum}",
    description:
      "Rate column value for the cron job minimum line; minimum is the USD amount without the currency symbol (w9/m448)",
  },
  "usage.chargeCustomDomains": {
    message: "플랜 초과 사용자 지정 도메인",
    description:
      "Charge line for custom domains beyond the plan's included count, billed per domain-month (w7/m429)",
  },
  "usage.chargeIncluded": {
    message: "{included} 포함",
    description:
      "Part of a charge line's quantity that the plan's included build minutes or bandwidth covered (w6/m308, w6/m319)",
  },
  "usage.pipelineTitle": {
    message: "빌드 파이프라인",
    description:
      "Title of the Build Pipeline card in workspace settings (w6/m309)",
  },
  "usage.pipelineDescription": {
    message:
      "빌드와 사전 배포 명령은 파이프라인 시간을 사용합니다. 요금제에 포함된 시간을 초과한 분량은 설정한 월 지출 한도까지 분 단위로 청구됩니다. 한도에 도달하면 다음 달까지 빌드가 중지됩니다.",
    description: "Explains the pipeline spend limit (w6/m309)",
  },
  "usage.pipelineSpendLimitValue": {
    message: "지출 한도: 월 {limit}",
    description: "The workspace pipeline spend limit (w6/m309)",
  },
  "usage.pipelineSpendLimitNone": {
    message: "지출 한도 없음",
    description: "The workspace has no pipeline spend limit (w6/m309)",
  },
  "usage.pipelineSpendLimitSet": {
    message: "지출 한도 설정",
    description: "Button that opens the pipeline spend limit dialog (w6/m309)",
  },
  "usage.pipelineSpendLimitEdit": {
    message: "편집",
    description: "Button that edits the pipeline spend limit (w6/m309)",
  },
  "usage.pipelineSpendLimitRemove": {
    message: "한도 제거",
    description: "Button that removes the pipeline spend limit (w6/m309)",
  },
  "usage.pipelineSpendLimitDialogTitle": {
    message: "지출 한도 설정",
    description: "Title of the pipeline spend limit dialog (w6/m309)",
  },
  "usage.pipelineSpendLimitDialogDescription": {
    message:
      "요금제에 포함된 시간을 초과한 빌드 시간에 대해 이 워크스페이스가 매월 지출할 수 있는 최대 금액입니다. 한도에 도달하면 다음 달 초까지 새 빌드와 사전 배포 명령이 중지됩니다.",
    description: "Explains what the pipeline spend limit caps (w6/m309)",
  },
  "usage.pipelineSpendLimitLabel": {
    message: "월 한도(USD)",
    description: "Label of the pipeline spend limit input (w6/m309)",
  },
  "usage.pipelineSpendLimitSave": {
    message: "저장",
    description: "Saves the pipeline spend limit (w6/m309)",
  },
  "usage.pipelineSpendLimitSaved": {
    message: "지출 한도를 월 {limit}(으)로 설정했습니다",
    description: "Toast after setting the pipeline spend limit (w6/m309)",
  },
  "usage.pipelineSpendLimitRemoved": {
    message: "지출 한도를 제거했습니다",
    description: "Toast after removing the pipeline spend limit (w6/m309)",
  },
  "usage.pipelineSpendLimitError": {
    message: "지출 한도를 업데이트하지 못했습니다",
    description: "Toast when updating the pipeline spend limit fails (w6/m309)",
  },
  "usage.pipelineSpendLimitUnreadable": {
    message:
      "지출 한도를 불러오지 못했습니다. 페이지를 새로 고쳐 다시 시도하세요.",
    description: "The pipeline spend limit could not be read (w6/m309)",
  },
  "usage.pipelineSpendLimitForbidden": {
    message: "결제 관리자만 지출 한도를 보고 변경할 수 있습니다.",
    description: "The caller may not read the pipeline spend limit (w6/m309)",
  },
  "usage.resourceCapsNearLimit": {
    message: "한도 근접",
    description:
      "Warning shown once a resource count reaches 80 percent of its cap, below the cap itself",
  },
  "usage.resourceCapsLimitReached": {
    message: "한도 도달",
    description:
      "Shown when a resource count is at or over its cap, so creating another is refused",
  },
  "usage.resourceCapsFinishingDeletion": {
    message: "삭제 완료 중 {count}개",
    description:
      "Sub-line on a resource-cap tile: how many of the counted resources are mid-deletion and thus hidden from the resource list but still holding quota (w6/m129)",
  },
  "usage.resourceCapsFinishingDeletionHint": {
    message: "삭제가 완료될 때까지 한도에 계속 포함됩니다",
    description:
      "Tooltip explaining why resources finishing deletion are included in the used count",
  },
  "usage.errorTitle": {
    message: "사용량을 불러오지 못했습니다",
    description: "Error state heading on the Usage page",
  },
  "usage.monthPickerLabel": {
    message: "월 선택",
    description: "Aria-label for the month-picker select on the Usage page",
  },
  "usage.currentMonth": {
    message: "이번 달",
    description:
      "Default option in the month picker meaning the current calendar month",
  },
  "usage.creditsTitle": {
    message: "남은 크레딧",
    description: "Heading of the remaining billing-credit card (w5/m70)",
  },
  "usage.creditsDescription": {
    message: "결제 수단에 청구하기 전에 청구서에 먼저 적용되는 프로모션 크레딧",
    description: "Subtitle of the remaining billing-credit card",
  },
  "usage.creditsExpiryNote": {
    message: "그중 {amount}은 {date}에 만료됩니다",
    description:
      "Earliest-expiring grant note beside the credit balance; amount is a $ value, date is YYYY-MM-DD",
  },
  "usage.creditsCardStillRequired": {
    message:
      "크레딧이 있어도 결제 수단은 여전히 필요합니다. 크레딧이 먼저 청구서에 적용되고 나머지는 카드로 결제됩니다.",
    description:
      "ADR046 clarification on the credit card — credit does not replace payment onboarding",
  },
  "usage.colInvoicePeriod": {
    message: "기간",
    description: "Table column header for an invoice's billing period start",
  },
  "usage.colInvoiceStatus": {
    message: "상태",
    description: "Table column header for an invoice's status",
  },
  "usage.colInvoiceAmount": {
    message: "금액",
    description: "Table column header for an invoice's amount",
  },
  "usage.billingSetupTitle": {
    message: "결제 설정",
    description: "Customer billing onboarding card title",
  },
  "usage.billingSetupDescription": {
    message: "이 워크스페이스의 결제 수금 및 세금 준비 상태",
    description: "Customer billing onboarding card description",
  },
  "usage.billingTestMode": {
    message: "Stripe 테스트 모드",
    description: "Badge making the non-live Stripe environment explicit",
  },
  "usage.billingMode": {
    message: "Stripe Billing",
    description: "Fallback badge for the Stripe billing environment",
  },
  "usage.billingReady": {
    message: "준비됨",
    description: "Billing onboarding status is ready",
  },
  "usage.billingActionNeeded": {
    message: "조치 필요",
    description: "Billing onboarding status needs action",
  },
  "usage.billingCustomerStatus": {
    message: "고객",
    description: "Stripe Customer readiness row",
  },
  "usage.billingSubscriptionStatus": {
    message: "종량제 구독",
    description: "Stripe Subscription readiness row",
  },
  "usage.billingPaymentStatus": {
    message: "결제 수단",
    description: "Default payment method readiness row",
  },
  "usage.billingTaxStatus": {
    message: "자동 세금 계산",
    description: "Stripe Tax activation row",
  },
  "usage.billingLifecycleStatus": {
    message: "수금 상태",
    description: "Stripe collection and reversible enforcement state row",
  },
  "usage.billingLifecycleGrace": {
    message:
      "결제에 실패했습니다({reason}). 유예 기간 동안 워크스페이스를 계속 사용할 수 있으며, {deadline} 이후 되돌릴 수 있는 일시 중지가 예약됩니다.",
    description: "Visible billing grace state and authoritative deadline",
  },
  "usage.billingLifecycleEnforced": {
    message:
      "결제 제한이 적용 중입니다({reason}). 컴퓨팅은 일시 중지되었지만 데이터베이스, Key Value 데이터, 시크릿, 결제 증빙은 삭제되지 않았습니다.",
    description: "Visible reversible billing enforcement state",
  },
  "usage.billingLifecycleRecovering": {
    message:
      "결제가 복구되었습니다. bex가 결제 제한으로 변경된 리소스만 복원하는 중입니다.",
    description: "Visible precise recovery state",
  },
  "usage.billingLifecycleExcluded": {
    message: "운영자가 이 워크스페이스를 Stripe 수금 대상에서 제외했습니다.",
    description: "Visible structural billing exclusion state",
  },
  "usage.billingLifecycleComped": {
    message: "이 워크스페이스는 요금이 산정되지만 운영자가 전액 면제했습니다.",
    description: "Visible rated-but-free comp state",
  },
  "usage.billingLifecycleUnknown": {
    message: "결제 상태: {reason}. 결제 포털을 이용하거나 지원팀에 문의하세요.",
    description: "Forward-compatible unknown billing lifecycle state",
  },
  "usage.billingNoDeadline": {
    message: "보고된 기한 없음",
    description: "Fallback when a malformed grace state has no deadline",
  },
  "usage.billingOff": {
    message: "꺼짐",
    description:
      "Neutral status for a deliberately disabled billing capability (e.g. tax not activated)",
  },
  "usage.billingAddPayment": {
    message: "결제 수단 추가",
    description: "Button opening setup-mode Stripe Checkout (live mode)",
  },
  "usage.billingUpdatePayment": {
    message: "결제 수단 업데이트",
    description: "Button reopening setup-mode Stripe Checkout (live mode)",
  },
  "usage.billingAddPaymentTest": {
    message: "테스트 결제 수단 추가",
    description:
      "Button opening setup-mode Stripe Checkout when Stripe is in test mode",
  },
  "usage.billingUpdatePaymentTest": {
    message: "테스트 결제 수단 업데이트",
    description:
      "Button reopening setup-mode Stripe Checkout when Stripe is in test mode",
  },
  "usage.billingOpenPortal": {
    message: "결제 포털 열기",
    description: "Button opening Stripe Customer Portal",
  },
  "usage.billingHostedNote": {
    message:
      "결제 정보는 Stripe가 호스팅하는 페이지에서만 입력합니다. bex는 Stripe 서버 키를 브라우저로 보내지 않습니다.",
    description: "Hosted billing security note",
  },
  "usage.billingUnavailable": {
    message:
      "결제 온보딩을 사용할 수 없거나 결제 접근 권한(결제 담당자 또는 관리자 역할)이 없습니다.",
    description: "Degraded or unauthorized billing onboarding state",
  },
  "usage.billingCheckoutError": {
    message: "Stripe Checkout을 열지 못했습니다. 다시 시도하세요.",
    description: "Toast after Checkout session creation fails",
  },
  "usage.billingPortalError": {
    message: "Stripe 결제 포털을 열지 못했습니다. 다시 시도하세요.",
    description: "Toast after Portal session creation fails",
  },
  "usage.paymentRequiredTitle": {
    message: "계속하려면 결제 수단을 추가하세요",
    description: "Just-in-time paid-intent onboarding dialog title",
  },
  "usage.paymentRequiredDescription": {
    message:
      "이 유료 요금제에는 결제 수단이 필요합니다. 새 탭에서 Stripe Checkout을 완료하세요. 준비되면 이 작업이 자동으로 이어서 진행됩니다.",
    description: "Just-in-time paid-intent onboarding explanation",
  },
  "usage.paymentRequiredRetrying": {
    message: "결제 수단을 확인했습니다. 작업을 다시 시도하는 중…",
    description: "Status while resuming the interrupted paid mutation",
  },
  "usage.paymentRequiredCancel": {
    message: "취소",
    description: "Cancel just-in-time payment onboarding",
  },
  "usage.planTitle": {
    message: "요금제",
    description: "Plan card title on the billing page",
  },
  "usage.planChange": {
    message: "요금제 변경",
    description:
      "Link from the billing page's plan card to the workspace-settings plan dialog",
  },
  "usage.paymentMethodTitle": {
    message: "결제 수단",
    description: "Payment-method card title on the billing page",
  },
  "usage.paymentMethodCard": {
    message: "{brand} 카드, 끝자리 {last4}",
    description: "Names the card on file, e.g. 'Visa ending in 4242'",
  },
  "usage.paymentMethodOnFile": {
    message: "등록된 결제 수단",
    description:
      "Shown when a payment method exists but is not a card the provider could name",
  },
  "usage.paymentMethodNone": {
    message: "결제 수단 없음",
    description: "Shown when the workspace has no payment method",
  },
  "usage.includedUsageTitle": {
    message: "포함 사용량",
    description:
      "Plan-allowance card title on the billing page (was 'Resource limits')",
  },
  "usage.includedUsageDescription": {
    message: "이 워크스페이스의 요금제에 포함된 항목과 사용 중인 양입니다.",
    description: "Included-usage card description",
  },
  "usage.chargesTitle": {
    message: "요금",
    description: "Charge-tree card title on the billing page",
  },
  "usage.chargesDescriptionEstimate": {
    message:
      "bex 요금표를 기준으로 이번 기간에 지금까지 누적된 금액입니다. 청구서가 아닌 추정치입니다.",
    description:
      "Charge-tree description when no Stripe subscription prices the period",
  },
  "usage.chargesDescriptionInvoiced": {
    message: "Stripe가 산정한, 이번 기간에 지금까지 누적된 금액입니다.",
    description:
      "Charge-tree description when a real Stripe amount is available. Says the total is Stripe's rating rather than the amount it will invoice: credits and comp discounts can sit between the two, and the amount actually due gets its own line (w6/m98).",
  },
  "usage.amountDueAfterCredits": {
    message: "크레딧 적용 후 결제 금액",
    description:
      "Label for the charge-tree line showing what Stripe actually collects once credits and discounts are applied to the charge above it",
  },
  "usage.chargesEmpty": {
    message: "이 기간에는 사용량이 없습니다.",
    description: "Charge-tree empty state",
  },
  "usage.coveragePartial": {
    message: "일부 데이터",
    description:
      "Amber caveat label above the charges when the metering behind the estimate is degraded/incomplete (w4/048), mirroring the Metrics page's degraded badge",
  },
  "usage.coveragePartialLead": {
    message: "이 추정치는 사용량을 실제보다 적게 반영했을 수 있습니다.",
    description: "Lead sentence of the partial-coverage caveat tooltip",
  },
  "usage.coveragePartialThrough": {
    message: "{through}까지 측정되었습니다.",
    description:
      "Partial-coverage caveat clause naming the date the meters have recorded through (a degraded hour counts as recorded, w4/496); {through} is a YYYY-MM-DD date",
  },
  "usage.coveragePartialSources": {
    message: "성능이 저하된 측정 소스: {sources}.",
    description:
      "Partial-coverage caveat clause listing the degraded metering source names; {sources} is a comma-separated list",
  },
  "usage.expandAll": {
    message: "모두 펼치기",
    description: "Charge-tree button that opens every category",
  },
  "usage.collapseAll": {
    message: "모두 접기",
    description: "Charge-tree button that closes every category",
  },
  "usage.totalToDate": {
    message: "이번 달 누적 합계",
    description: "Charge-tree total row for the month in progress",
  },
  "usage.totalForPeriod": {
    message: "기간 합계",
    description: "Charge-tree total row for a completed past month",
  },
  "usage.totalBillingPeriodWindow": {
    message: "이번 결제 기간 합계({start} – {end})",
    description:
      "Charge total row when the headline is Stripe's rated amount: it covers the subscription period (anchored on the subscription's day of month), not the calendar month (w4/m139)",
  },
  "usage.totalBillingPeriod": {
    message: "이번 결제 기간 합계",
    description:
      "Charge total row for Stripe's rated amount when its period bounds are unknown (w4/m139)",
  },
  "usage.breakdownMonthToDate": {
    message: "위 내역: {month} 누적",
    description:
      "Row under a Stripe-rated total: the charge tree's own sum and window (the calendar month), which differs from the subscription period (w4/m139)",
  },
  "usage.projectedTotal": {
    message: "{month} 예상 합계",
    description:
      "Charge-tree straight-line projection of month-end spend, for the estimate fallback whose total accrues over the calendar month",
  },
  "usage.projectedTotalBillingPeriod": {
    message: "이번 결제 기간 예상 금액",
    description:
      "Charge-tree straight-line projection of a Stripe-rated total. The subscription period need not align with the calendar month (it can span two), so no month is named (w6/050).",
  },
  "usage.chargeFree": {
    message: "포함됨",
    description: "Rate column value for a charge line that is priced at zero",
  },
  "usage.resourceDeleted": {
    message: "(삭제됨)",
    description:
      "Marker beside a charged resource that no longer exists; the name shown is the one bex retained",
  },
  "usage.categoryServices": {
    message: "서비스",
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
    message: "오브젝트 스토리지",
    description:
      "Charge-tree category for a workspace's object storage (ADR112)",
  },
  "usage.categoryAgentSessions": {
    message: "에이전트 세션",
    description:
      "Charge-tree category for platform model usage of agent sessions",
  },
  "usage.chargeDisk": {
    message: "디스크",
    description: "Charge line for a service's provisioned persistent disk",
  },
  "usage.chargeAgentModelCost": {
    message: "모델 사용량",
    description:
      "Charge line for platform model spend, billed at provider cost plus markup (ADR091)",
  },
  "usage.chargeAgentModelRate": {
    message: "제공업체 비용 + 30%",
    description: "Rate column value for the platform model spend line",
  },
  "usage.chargeAgentModelInputTokens": {
    message: "입력 토큰",
    description: "Breakdown line for platform model input tokens",
  },
  "usage.chargeAgentModelOutputTokens": {
    message: "출력 토큰",
    description: "Breakdown line for platform model output tokens",
  },
  "usage.chargeAgentModelCacheReadTokens": {
    message: "캐시 읽기 토큰",
    description: "Breakdown line for platform model cache-read tokens",
  },
  "usage.chargeAgentModelCacheWriteTokens": {
    message: "캐시 쓰기 토큰",
    description: "Breakdown line for platform model cache-write tokens",
  },
  "usage.categorySandboxes": {
    message: "샌드박스",
    description: "Charge-tree category for hosted agent sandboxes",
  },
  "usage.chargeCompute": {
    message: "컴퓨팅",
    description: "Charge line for metered instance time",
  },
  "usage.chargeBandwidth": {
    message: "대역폭",
    description: "Charge line for metered outbound bandwidth",
  },
  "usage.chargeBuild": {
    message: "빌드 시간",
    description: "Charge line for metered build time",
  },
  "usage.chargeStorage": {
    message: "스토리지",
    description: "Charge line for metered datastore storage",
  },
  "usage.chargeSandboxCompute": {
    message: "샌드박스 컴퓨팅",
    description: "Charge line for metered sandbox compute",
  },
  "usage.chargeSandboxMemory": {
    message: "샌드박스 메모리",
    description:
      "Charge line for the memory a running sandbox was allocated, billed per GiB-hour since billing switched to use",
  },
  "usage.chargeSandboxMemoryUnbilled": {
    message: "샌드박스 메모리(컴퓨팅에 포함)",
    description:
      "Charge line for the allocated memory a sandbox ran with before billing switched to use; recorded beside sandbox compute, which already includes it, so it is shown and not billed separately",
  },
  "usage.chargeSandboxCpu": {
    message: "샌드박스 CPU",
    description:
      "Charge line for the CPU a running sandbox used, billed per CPU-hour used",
  },
  "usage.chargeSandboxCpuUnbilled": {
    message: "샌드박스 CPU(컴퓨팅에 포함)",
    description:
      "Charge line for the CPU a sandbox used before billing switched to use; recorded beside sandbox compute, which already charged for it, so it is shown and not billed separately",
  },
  "usage.sandboxUsageBilledFromNote": {
    message:
      "{date} UTC부터 실행 중인 샌드박스는 사용한 CPU와 할당된 메모리에 대해 요금이 청구됩니다. 그 이전 사용량은 샌드박스 컴퓨팅으로 청구되었습니다.",
    description:
      "Note under the charge tree of a period with sandbox usage; date is the instant billing switched, formatted in UTC",
  },
  "usage.chargeSandboxSnapshotStorage": {
    message: "샌드박스 스냅샷 스토리지",
    description:
      "Charge line for the image a suspended sandbox retains, billed per GB-month",
  },
  "usage.chargeObjectStorage": {
    message: "스토리지",
    description:
      "Charge line for the data a workspace keeps in object storage, billed per GB-month (shown at no charge until a price is set, ADR112 D8)",
  },
  "usage.chargeObjectStorageIngress": {
    message: "인바운드 대역폭",
    description:
      "Charge line for the bytes put into a workspace's object storage (shown at no charge, ADR112 D8)",
  },
  "usage.chargeObjectStorageEgress": {
    message: "아웃바운드 대역폭",
    description:
      "Charge line for the bytes read out of a workspace's object storage, public reads included (shown at no charge, ADR112 D8)",
  },
  "usage.creditsTotalLabel": {
    message: "총 잔액",
    description: "Label above the credit balance amount",
  },
  "usage.invoiceHistoryTitle": {
    message: "청구서 내역",
    description: "Invoice-history card title on the billing page",
  },
  "usage.invoiceHistoryDescription": {
    message: "확정된 청구서입니다.",
    description: "Invoice-history card description",
  },
  "usage.invoiceKindAgentModelCharge": {
    message: "에이전트 모델 요금",
    description:
      "Label on an invoice-history row for a charge made when agent model spend reached its threshold",
  },
  "usage.invoiceHistoryChargesNote": {
    message:
      "한 달 동안 발생한 에이전트 모델 요금은 해당 월 청구서에서 차감됩니다.",
    description:
      "Invoice-history note when the list includes agent model charges",
  },
  "usage.creditsAppliedLine": {
    message: "적용된 크레딧 −{applied} → 결제 금액 {due}",
    description:
      "Credit applied to the current period and the remaining amount due",
  },
  "usage.chargesDescriptionPending": {
    message: "bex 요금표를 기준으로 이번 기간에 지금까지 누적된 금액입니다.",
    description:
      "Charges card description while the invoiced total is still loading; deliberately states only what is already true, without claiming the figure is or is not a Stripe invoice.",
  },
  "usage.paymentMethodRejectedPrepaid": {
    message:
      "선불 카드는 사용할 수 없습니다. 신용 카드 또는 체크 카드를 추가하세요.",
    description:
      "Shown when the last card offered was refused at binding because it is prepaid (ADR091 D10.5)",
  },
  "usage.paymentMethodRejectedUnpaidBalance": {
    message:
      "이 카드에는 미결제 잔액이 연결되어 있습니다. 다른 카드를 추가하세요.",
    description:
      "Shown when the last card offered was refused because it carries an unpaid written-off balance (ADR091 D10.6)",
  },
  "usage.agentModelTitle": {
    message: "에이전트 모델",
    description:
      "Billing page card and nav entry for agent model spend (ADR091 D6)",
  },
  "usage.agentModelPricing": {
    message: "모델 제공업체 비용 + 30%로 청구됩니다.",
    description: "Pricing statement under the Agent model title",
  },
  "usage.agentModelMonthToDate": {
    message: "이번 달 누적",
    description: "Label of the current month's agent model total",
  },
  "usage.agentModelPeriodTotal": {
    message: "월 합계",
    description: "Label of a closed month's agent model total",
  },
  "usage.agentModelBreakdown_other": {
    message: "모델별 내역(모델 {count}개)",
    description: "Toggle that expands the per-model breakdown",
  },
  "usage.agentModelEmpty": {
    message: "이 기간에는 에이전트 모델 사용량이 없습니다.",
    description: "Shown when the period has no agent model spend",
  },
  "usage.lessThanAmount": {
    message: "<{amount}",
    description:
      "A charge that rounds below the smallest amount shown, e.g. <$0.01",
  },
};

export default koUsage;
