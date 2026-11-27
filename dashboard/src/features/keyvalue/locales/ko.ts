import type { TranslationEntry } from "@/i18n";

const koKeyValue: Record<string, TranslationEntry> = {
  "keyvalue.resourceType": {
    message: "Key Value",
    description: "Key Value resource type used in document titles",
  },
  "keyvalue.statTotal": {
    message: "전체 Key Value 스토어",
    description: "Key Value page stat card label",
  },
  "keyvalue.statAvailable": {
    message: "사용 가능",
    description: "Key Value page stat card label (healthy stores)",
  },
  "keyvalue.statCreating": {
    message: "생성 중",
    description: "Key Value page stat card label (provisioning stores)",
  },
  "keyvalue.cardTitle": {
    message: "Key Value",
    description: "Key Value table card title",
  },
  "keyvalue.colName": {
    message: "이름",
    description: "Key Value table column header",
  },
  "keyvalue.colStatus": {
    message: "상태",
    description: "Key Value table column header",
  },
  "keyvalue.colPlan": {
    message: "컴퓨팅 플랜",
    description: "Key Value table column header (plan / tier)",
  },
  "keyvalue.colVersion": {
    message: "버전",
    description: "Key Value table column header (Valkey version)",
  },
  "keyvalue.colCreated": {
    message: "생성",
    description: "Key Value table column header (relative age from createdAt)",
  },
  "keyvalue.colActions": {
    message: "작업",
    description: "Key Value table actions column header (screen-reader only)",
  },
  "keyvalue.statusAvailable": {
    message: "사용 가능",
    description: "Key Value status badge (Valkey instance healthy)",
  },
  "keyvalue.statusCreating": {
    message: "생성 중",
    description: "Key Value status badge (provisioning)",
  },
  "keyvalue.statusRestarting": {
    message: "다시 시작 중",
    description:
      "Key Value status badge (Render's config_restart: an established store restarting after a config change, restart, or resume — not a first provision)",
  },
  "keyvalue.statusUnavailable": {
    message: "사용 불가",
    description: "Key Value status badge (provisioning failed)",
  },
  "keyvalue.statusSuspended": {
    message: "일시 중지됨",
    description:
      "Key Value status badge (hibernated; suspension wins over the status enum)",
  },
  "keyvalue.statusUnknown": {
    message: "알 수 없음",
    description: "Key Value status badge for an unrecognized status",
  },
  "keyvalue.statusReasonStatefulSetReadFailed": {
    message: "Key Value의 워크로드 상태를 확인하지 못했습니다.",
    description:
      "Why an unavailable Key Value is unavailable: the operator's StatefulSetReadFailed Ready reason (bex-api statusReasonCode)",
  },
  "keyvalue.statusReasonStatefulSetFailed": {
    message:
      "Key Value의 워크로드를 업데이트하지 못해 최신 변경 사항이 적용되지 않았습니다.",
    description:
      "Why an unavailable Key Value is unavailable: the operator's StatefulSetFailed Ready reason (bex-api statusReasonCode)",
  },
  "keyvalue.statusReasonPVCReadFailed": {
    message: "스토리지 볼륨의 상태를 확인하지 못했습니다.",
    description:
      "Why an unavailable Key Value is unavailable: the operator's PVCReadFailed Ready reason (bex-api statusReasonCode)",
  },
  "keyvalue.statusReasonPVCResizeFailed": {
    message: "Key Value의 스토리지 볼륨 크기를 조정하지 못했습니다.",
    description:
      "Why an unavailable Key Value is unavailable: the operator's PVCResizeFailed Ready reason (bex-api statusReasonCode)",
  },
  "keyvalue.statusReasonSecretFailed": {
    message: "Key Value의 연결 정보를 저장하지 못했습니다.",
    description:
      "Why an unavailable Key Value is unavailable: the operator's SecretFailed Ready reason (bex-api statusReasonCode)",
  },
  "keyvalue.statusReasonCredentialSecretFailed": {
    message: "Key Value의 자격 증명을 저장하지 못했습니다.",
    description:
      "Why an unavailable Key Value is unavailable: the operator's CredentialSecretFailed Ready reason (bex-api statusReasonCode)",
  },
  "keyvalue.statusReasonSecretRecreateFailed": {
    message: "Key Value의 연결 정보를 다시 만들지 못했습니다.",
    description:
      "Why an unavailable Key Value is unavailable: the operator's SecretRecreateFailed Ready reason (bex-api statusReasonCode)",
  },
  "keyvalue.statusReasonServiceFailed": {
    message: "Key Value의 내부 주소를 설정하지 못했습니다.",
    description:
      "Why an unavailable Key Value is unavailable: the operator's ServiceFailed Ready reason (bex-api statusReasonCode)",
  },
  "keyvalue.statusReasonNetworkPolicyFailed": {
    message: "Key Value의 네트워크 정책을 적용하지 못했습니다.",
    description:
      "Why an unavailable Key Value is unavailable: the operator's NetworkPolicyFailed Ready reason (bex-api statusReasonCode)",
  },
  "keyvalue.statusReasonBackupNetworkPolicyFailed": {
    message: "백업 작업의 네트워크 정책을 적용하지 못했습니다.",
    description:
      "Why an unavailable Key Value is unavailable: the operator's BackupNetworkPolicyFailed Ready reason (bex-api statusReasonCode)",
  },
  "keyvalue.statusReasonBackupCronJobFailed": {
    message: "일일 백업 일정을 업데이트하지 못했습니다.",
    description:
      "Why an unavailable Key Value is unavailable: the operator's BackupCronJobFailed Ready reason (bex-api statusReasonCode)",
  },
  "keyvalue.statusReasonTLSIssuerMissing": {
    message:
      "공개 액세스를 활성화할 수 없습니다. 인증서 발급자가 구성되어 있지 않습니다.",
    description:
      "Why an unavailable Key Value is unavailable: the operator's TLSIssuerMissing Ready reason (bex-api statusReasonCode)",
  },
  "keyvalue.statusReasonCertificateFailed": {
    message: "공개 액세스용 인증서를 발급하지 못했습니다.",
    description:
      "Why an unavailable Key Value is unavailable: the operator's CertificateFailed Ready reason (bex-api statusReasonCode)",
  },
  "keyvalue.statusReasonCertificateCleanupFailed": {
    message: "공개 액세스용 인증서를 제거하지 못했습니다.",
    description:
      "Why an unavailable Key Value is unavailable: the operator's CertificateCleanupFailed Ready reason (bex-api statusReasonCode)",
  },
  "keyvalue.statusReasonStorageClassMissing": {
    message: "스토리지 볼륨에 스토리지 클래스가 없어 확장할 수 없습니다.",
    description:
      "Why an unavailable Key Value is unavailable: the operator's StorageClassMissing Ready reason (bex-api statusReasonCode)",
  },
  "keyvalue.statusReasonStorageClassNotFound": {
    message:
      "스토리지 볼륨의 스토리지 클래스를 찾을 수 없어 확장할 수 없습니다.",
    description:
      "Why an unavailable Key Value is unavailable: the operator's StorageClassNotFound Ready reason (bex-api statusReasonCode)",
  },
  "keyvalue.statusReasonStorageClassNotExpandable": {
    message: "스토리지 볼륨의 스토리지 클래스가 확장을 허용하지 않습니다.",
    description:
      "Why an unavailable Key Value is unavailable: the operator's StorageClassNotExpandable Ready reason (bex-api statusReasonCode)",
  },
  "keyvalue.statusReasonStorageBlockedByQuota": {
    message:
      "스토리지 볼륨을 확장할 수 없습니다. 워크스페이스의 스토리지 할당량에 여유가 없습니다.",
    description:
      "Why an unavailable Key Value is unavailable: the operator's StorageBlockedByQuota Ready reason (bex-api statusReasonCode)",
  },
  "keyvalue.statusReasonPersistenceTransitionFailed": {
    message:
      "저장된 데이터를 시작에 맞게 준비하지 못했습니다. 데이터는 보존되며 다시 시도합니다.",
    description:
      "Why an unavailable Key Value is unavailable: the operator's PersistenceTransitionFailed Ready reason (bex-api statusReasonCode)",
  },
  "keyvalue.statusReasonPersistenceSourceUnknown": {
    message:
      "저장된 데이터의 영속성 모드를 알 수 없어, 플랫폼 운영자가 확인하기 전까지 Key Value를 시작할 수 없습니다. 데이터는 변경되지 않았습니다.",
    description:
      "Why an unavailable Key Value is unavailable: the operator's PersistenceSourceUnknown Ready reason (bex-api statusReasonCode)",
  },
  "keyvalue.statusReasonStorageShrinkRejected": {
    message:
      "Key Value 스토리지는 늘리기만 가능합니다. 볼륨을 현재 크기보다 작게 만들 수 없습니다.",
    description:
      "Why an unavailable Key Value is unavailable: the operator's StorageShrinkRejected Ready reason (bex-api statusReasonCode)",
  },
  "keyvalue.errorTitle": {
    message: "Key Value 스토어를 불러오지 못했습니다",
    description: "Key Value list error card title",
  },
  "keyvalue.errorBody": {
    message: "bex-api 요청에 실패했습니다. 연결을 확인한 후 다시 시도하세요.",
    description: "Key Value list error card body",
  },
  "keyvalue.emptyTitle": {
    message: "아직 Key Value 스토어가 없습니다",
    description: "Key Value list empty state title",
  },
  "keyvalue.emptyBody": {
    message: "첫 관리형 Key Value 스토어를 만들면 여기에 표시됩니다.",
    description: "Key Value list empty state body",
  },
  "keyvalue.actionsMenu": {
    message: "{name} 작업",
    description: "Accessible label for the per-row actions trigger",
  },
  "keyvalue.actionDelete": {
    message: "삭제",
    description: "Row action: permanently delete the Key Value store",
  },
  "keyvalue.deleteConfirmTitle": {
    message: "Key Value 인스턴스 삭제",
    description: "Delete-confirmation dialog title",
  },
  "keyvalue.deleteConfirmBody": {
    message:
      "이 작업은 되돌릴 수 없습니다. 이 Key Value 인스턴스를 삭제하시겠습니까?",
    description:
      "Delete-confirmation dialog body (verbatim Render copy, docs/render-artifacts/key-value.md)",
  },
  "keyvalue.deleteConfirmPrompt": {
    message: "확인하려면 아래에 {phrase}을(를) 입력하세요.",
    description:
      "Body prompt naming Render's exact sudo delete phrase (rendered bold by SudoCommandField)",
  },
  "keyvalue.deleteCancel": {
    message: "취소",
    description: "Delete-confirmation dialog cancel button",
  },
  "keyvalue.deleteConfirm": {
    message: "Key Value 인스턴스 삭제",
    description: "Delete-confirmation dialog confirm button",
  },
  "keyvalue.deleteSuccess": {
    message: "{name} 삭제 중…",
    description: "Toast after a delete request is accepted",
  },
  "keyvalue.deleteError": {
    message: "{name}을(를) 삭제하지 못했습니다. 다시 시도하세요.",
    description: "Toast when a delete request fails",
  },
  "keyvalue.createButton": {
    message: "새 Key Value",
    description: "Button that navigates to the create-Key-Value page",
  },
  "keyvalue.createTitle": {
    message: "새 Key Value",
    description: "Create-Key-Value page title",
  },
  "keyvalue.createDescription": {
    message: "관리형 Valkey(Redis 호환) 인스턴스를 프로비저닝합니다.",
    description: "Create-Key-Value page description",
  },
  "keyvalue.fieldName": {
    message: "이름",
    description: "Create-Key-Value form field label (store name)",
  },
  "keyvalue.fieldNamePlaceholder": {
    message: "example-key-value-name",
    description: "Create-Key-Value name input placeholder",
  },
  "keyvalue.fieldNameError": {
    message: "줄 바꿈이나 제어 문자 없이 100자 이하의 이름을 입력하세요.",
    description: "Create-Key-Value name validation message",
  },
  "keyvalue.fieldPlan": {
    message: "컴퓨팅 플랜",
    description: "Create-Key-Value form field label (plan / tier)",
  },
  "keyvalue.fieldVersion": {
    message: "Valkey 버전",
    description: "Create-Key-Value form field label (major version)",
  },
  "keyvalue.fieldVersionDefault": {
    message: "기본값 (최신)",
    description: "Create-Key-Value version select default option",
  },
  "keyvalue.fieldMaxmemoryPolicy": {
    message: "Maxmemory 정책",
    description: "Create-Key-Value form field label (eviction policy)",
  },
  "keyvalue.fieldMaxmemoryRecommended": {
    message: "(캐시에 권장)",
    description:
      "Suffix on the recommended maxmemory policy option (allkeys-lru)",
  },
  "keyvalue.fieldMaxmemoryPolicyHint": {
    message: "스토어가 메모리 한도에 도달했을 때 키를 제거하는 방식입니다.",
    description: "Create-Key-Value maxmemory policy helper text",
  },
  "keyvalue.fieldPersistenceMode": {
    message: "영속성 모드",
    description: "Create-Key-Value form field label (persistence)",
  },
  "keyvalue.fieldPersistenceModeHint": {
    message: "다시 시작해도 유지되도록 데이터를 디스크에 저장하는 방식입니다.",
    description: "Create-Key-Value persistence mode helper text",
  },
  "keyvalue.persistenceJournalSnapshot": {
    message: "저널 + 스냅샷",
    description: "Persistence mode option: AOF journal plus RDB snapshots",
  },
  "keyvalue.persistenceSnapshot": {
    message: "스냅샷만",
    description: "Persistence mode option: RDB snapshots only",
  },
  "keyvalue.persistenceOff": {
    message: "끔",
    description: "Persistence mode option: no persistence",
  },
  "keyvalue.fieldPublic": {
    message: "공개 액세스",
    description: "Create-Key-Value form field label (external endpoint toggle)",
  },
  "keyvalue.fieldPublicHint": {
    message: "TLS를 통해 사설 네트워크 외부에서의 연결을 허용합니다.",
    description: "Create-Key-Value public toggle helper text",
  },
  "keyvalue.createCancel": {
    message: "취소",
    description: "Create-Key-Value page cancel button",
  },
  "keyvalue.createSubmit": {
    message: "Key Value 인스턴스 생성",
    description: "Create-Key-Value page submit button",
  },
  "keyvalue.createSuccess": {
    message: "{name} 생성 중…",
    description:
      "Toast after a create request is accepted (provisioning is async)",
  },
  "keyvalue.createError": {
    message: "{name}을(를) 생성하지 못했습니다. 다시 시도하세요.",
    description: "Toast when a create request fails",
  },
  "keyvalue.capLimitTitle": {
    message: "Key Value 한도에 도달했습니다",
    description:
      "Alert title when the workspace's KV creation cap is hit (w7/m9)",
  },
  "keyvalue.capLimitUpgrade": {
    message: "요금제 업그레이드",
    description: "Upgrade CTA button inside the KV cap-limit Alert (w7/m9)",
  },
  "keyvalue.metaTitle": {
    message: "세부 정보",
    description: "Key Value detail metadata card title",
  },
  "keyvalue.metaId": {
    message: "ID",
    description: "Key Value detail metadata row label (immutable red- id)",
  },
  "keyvalue.metaStatus": {
    message: "상태",
    description: "Key Value detail metadata row label",
  },
  "keyvalue.metaPlan": {
    message: "컴퓨팅 플랜",
    description: "Key Value detail metadata row label",
  },
  "keyvalue.metaConnectionLimit": {
    message: "연결 한도",
    description:
      "Key Value detail Details card row label: the plan's client connection limit, as Render's Key Value Instance card shows it",
  },
  "keyvalue.metaVersion": {
    message: "버전",
    description: "Key Value detail metadata row label",
  },
  "keyvalue.metaPublic": {
    message: "공개 액세스",
    description: "Key Value detail metadata row label (external endpoint)",
  },
  "keyvalue.metaExternalHost": {
    message: "외부 호스트",
    description: "Key Value detail metadata row label (SNI hostname)",
  },
  "keyvalue.metaRegion": {
    message: "리전",
    description: "Key Value detail metadata row label (platform placement)",
  },
  "keyvalue.metaCreated": {
    message: "생성",
    description: "Key Value detail metadata row label (relative age)",
  },
  "keyvalue.yes": {
    message: "예",
    description: "Metadata value for a true boolean field",
  },
  "keyvalue.publicPending": {
    message: "예(외부 엔드포인트 여는 중: 인증서 대기 중)",
    description:
      "Details value for Public access while the store's certificate is pending, so it does not read as reachable (w4/m357)",
  },
  "keyvalue.publicFailed": {
    message: "예(아직 접근할 수 없음: 인증서 발급 실패)",
    description:
      "Details value for Public access when the store's certificate failed to issue (w4/m357)",
  },
  "keyvalue.no": {
    message: "아니요",
    description: "Metadata value for a false boolean field",
  },
  "keyvalue.connTitle": {
    message: "연결",
    description: "Connection-info panel card title",
  },
  "keyvalue.connDescription": {
    message:
      "이 Key Value 스토어의 연결 문자열입니다. 요청할 때만 표시되며 자동으로 표시되지 않습니다.",
    description: "Connection-info panel card description",
  },
  "keyvalue.internalAuthOff": {
    message: "내부 연결에는 비밀번호가 필요하지 않습니다.",
    description: "Connections panel: internal auth off",
  },
  "keyvalue.internalAuthOn": {
    message: "내부 연결에는 비밀번호가 필요합니다.",
    description: "Connections panel: internal auth on",
  },
  "keyvalue.internalAuthEnable": {
    message: "내부 인증 활성화",
    description: "Button and confirm label (Render's wording)",
  },
  "keyvalue.internalAuthConfirmTitle": {
    message: "내부 인증을 활성화할까요?",
    description: "Confirm dialog title",
  },
  "keyvalue.internalAuthConfirmBody": {
    message:
      "모든 내부 연결에 비밀번호가 필요해집니다. 인증 없는 URL을 사용하는 기존 연결은 끊어지며, 연결된 서비스는 다음 배포 때 새 URL을 적용합니다. 연결 정보에 표시된 URL로 업데이트하세요. 이 작업은 되돌릴 수 없습니다.",
    description: "Confirm dialog body",
  },
  "keyvalue.internalAuthEnabled": {
    message: "{name}의 내부 인증을 활성화했습니다.",
    description: "Toast after enabling",
  },
  "keyvalue.internalAuthError": {
    message: "내부 인증을 활성화하지 못했습니다. 다시 시도하세요.",
    description: "Toast when enabling fails",
  },
  "keyvalue.connReveal": {
    message: "연결 정보 표시",
    description: "Button that fetches the connection info on demand",
  },
  "keyvalue.connHide": {
    message: "연결 정보 숨기기",
    description: "Button that clears the revealed connection info",
  },
  "keyvalue.connInternal": {
    message: "내부 Key Value URL",
    description: "Connection-info field label (in-cluster redis:// URL)",
  },
  "keyvalue.connExternal": {
    message: "외부 Key Value URL",
    description: "Connection-info field label (public rediss:// URL)",
  },
  "keyvalue.connExternalPending": {
    message:
      "외부 접근을 활성화하는 중입니다. 공개적으로 신뢰되는 인증서를 발급하고 있습니다(보통 몇 분 소요). 스토어가 TLS 연결을 받기 시작하면 외부 URL과 CLI 명령으로 연결할 수 있으며, 그때까지는 내부 URL로 서비스에서만 접근할 수 있습니다. 숨겼다가 다시 표시하면 새로 고쳐집니다.",
    description:
      "Shown under the external URL while a public store's TLS listener is still opening (w9/m521)",
  },
  "keyvalue.connExternalFailed": {
    message:
      "외부 URL과 CLI 명령으로 아직 연결할 수 없습니다. 이 스토어의 공개적으로 신뢰되는 인증서 발급에 실패했으며, bex가 자동으로 다시 시도합니다. 성공할 때까지는 내부 URL로 서비스에서만 스토어에 접근할 수 있습니다.",
    description:
      "Shown under the external URL when a public store's certificate failed to issue (w4/m357, w9/m521)",
  },
  "keyvalue.connExternalUnavailable": {
    message:
      "공개되어 있지 않습니다. 외부 액세스를 활성화하려면 네트워킹에서 인바운드 IP 규칙을 추가하세요.",
    description:
      "Shown instead of the external URL and the CLI command when the store isn't public (w9/134)",
  },
  "keyvalue.connCli": {
    message: "Valkey CLI 명령",
    description:
      "Connection-info field label (ready-to-run valkey-cli command)",
  },
  "keyvalue.connErrorTitle": {
    message: "연결 정보를 불러오지 못했습니다",
    description: "Connection-info panel error title",
  },
  "keyvalue.connErrorBody": {
    message:
      "스토어가 아직 프로비저닝 중이거나 자격 증명을 볼 권한이 없을 수 있습니다.",
    description: "Connection-info panel error body",
  },
  "keyvalue.copied": {
    message: "클립보드에 복사했습니다",
    description: "Toast after copying a connection field",
  },
  "keyvalue.copyError": {
    message: "클립보드에 복사하지 못했습니다",
    description: "Toast when clipboard copy fails",
  },
  "keyvalue.dangerDelete": {
    message: "Key Value 인스턴스 삭제",
    description: "Discoverable delete action at the bottom of Key Value Info",
  },
  "keyvalue.actionSuspend": {
    message: "Key Value 인스턴스 일시 중지",
    description: "Suspend action button label",
  },
  "keyvalue.actionResume": {
    message: "Key Value 인스턴스 재개",
    description: "Resume action button label",
  },
  "keyvalue.confirmSuspendTitle": {
    message: "Key Value 인스턴스 일시 중지",
    description: "Suspend-confirmation dialog title",
  },
  "keyvalue.confirmSuspendBody": {
    message: "이 Key Value 인스턴스를 일시 중지하시겠습니까?",
    description: "Suspend-confirmation dialog description",
  },
  "keyvalue.confirmSuspendDetail": {
    message:
      "{name}을(를) 일시 중지할지 확인하세요. 활성 연결이 끊어지며 언제든 재개할 수 있습니다. 데이터 보존은 영속성 모드에 따라 달라집니다. 끔은 모든 데이터가 사라지고, 스냅샷만은 마지막 스냅샷까지의 데이터를 보존하며, 저널 + 스냅샷은 약 1초 전까지의 쓰기를 보존합니다.",
    description: "Suspend-confirmation consequences and named instance",
  },
  "keyvalue.confirmSuspendPrompt": {
    message: "확인하려면 아래에 {phrase}을(를) 입력하세요.",
    description:
      "Body prompt naming Render's exact Key Value suspend phrase (rendered bold by SudoCommandField)",
  },
  "keyvalue.confirmCancel": {
    message: "취소",
    description: "Suspend-confirmation dialog cancel button",
  },
  "keyvalue.toastSuspendSuccess": {
    message: "{name} 일시 중지 중…",
    description: "Toast after a suspend request is accepted",
  },
  "keyvalue.toastSuspendError": {
    message: "{name}을(를) 일시 중지하지 못했습니다. 다시 시도하세요.",
    description: "Toast when a suspend request fails",
  },
  "keyvalue.toastResumeSuccess": {
    message: "{name} 재개 중…",
    description: "Toast after a resume request is accepted",
  },
  "keyvalue.toastResumeError": {
    message: "{name}을(를) 재개하지 못했습니다. 다시 시도하세요.",
    description: "Toast when a resume request fails",
  },
  "keyvalue.planTitle": {
    message: "컴퓨팅 플랜",
    description: "Key Value detail plan-picker card title",
  },
  "keyvalue.planDescription": {
    message:
      "더 큰 컴퓨팅 플랜으로 변경할 수 있으며, Key Value 인스턴스는 더 작은 플랜으로 변경할 수 없습니다. 변경하면 스토어가 다시 시작되어 1~2분간 사용할 수 없습니다. 데이터는 영속성 모드에 따라 보존되며, 영속성이 끔인 스토어는 모든 키가 사라집니다.",
    description: "Key Value detail plan-picker card description",
  },
  "keyvalue.planDowngradeUnavailable": {
    message:
      "Key Value 인스턴스는 더 작은 컴퓨팅 플랜으로 변경할 수 없습니다. 원하는 플랜으로 새 인스턴스를 만들고 데이터를 옮기세요.",
    description:
      "Tooltip on a disabled plan card smaller than the store's current plan: Render does not downgrade a Key Value instance (w8/249)",
  },
  "keyvalue.planPausedReason": {
    message:
      "Key Value 요금제 “{name}”은(는) 일시적으로 사용할 수 없습니다. {largest} 또는 더 작은 요금제를 선택하세요.",
    description:
      "A paused Key Value plan's tooltip and screen-reader reason, composed in the dashboard's language; matches bex-api's English unavailableReason (w4/552)",
  },
  "keyvalue.planPickerSave": {
    message: "저장",
    description: "Key Value plan-picker save button",
  },
  "keyvalue.planPickerCancel": {
    message: "취소",
    description: "Key Value plan-picker cancel / reset button",
  },
  "keyvalue.reviewComputeLead": {
    message: "컴퓨팅 선택 사항을 검토하세요.",
    description:
      "Key Value plan review: the lead of a compute plan change's review (Render's copy)",
  },
  "keyvalue.reviewUnavailable": {
    message: "업데이트하는 동안 Key Value 인스턴스를 사용할 수 없습니다.",
    description:
      "Key Value plan review warning: the store restarts to apply the plan (Render's copy); the data-loss sentence follows it when persistence is Off",
  },
  "keyvalue.reviewUpgradeCompute": {
    message: "컴퓨팅 업그레이드",
    description:
      "Key Value plan review: the button that applies a plan change (Render's copy; Key Value plans only go up)",
  },
  "keyvalue.restartConfirmDataLoss": {
    message:
      "영속성이 끔이므로 다시 시작하면 {name}의 모든 키가 삭제됩니다. 이 작업은 되돌릴 수 없습니다.",
    description:
      "Restart confirm (plan change): a Persistence Off store loses all data on the restart (w4/m263)",
  },
  "keyvalue.restartConfirmDataLossUnknown": {
    message:
      "이 스토어의 영속성 모드를 알 수 없습니다. 끔이라면 다시 시작할 때 {name}의 모든 키가 삭제됩니다. 이 작업은 되돌릴 수 없습니다.",
    description:
      "Restart confirm when the persistence mode could not be read: the typed phrase is still required",
  },
  "keyvalue.planPickerSuccess": {
    message: "{name}(으)로 요금제를 업데이트하는 중…",
    description: "Toast after a plan update is accepted",
  },
  "keyvalue.planPickerError": {
    message: "요금제를 업데이트하지 못했습니다. 다시 시도하세요.",
    description: "Toast when a plan update fails",
  },
  "keyvalue.maxmemoryTitle": {
    message: "Maxmemory 정책",
    description: "Key Value detail eviction-policy card title",
  },
  "keyvalue.maxmemoryDescription": {
    message:
      "스토어가 메모리 한도에 도달했을 때 키를 제거하는 방식을 변경합니다. 새 정책은 다시 시작하지 않고 실행 중인 스토어에 적용됩니다.",
    description: "Key Value detail eviction-policy card description",
  },
  "keyvalue.maxmemoryLabel": {
    message: "Maxmemory 정책",
    description: "Accessible label for the eviction-policy select",
  },
  "keyvalue.maxmemoryEdit": {
    message: "Maxmemory 정책 편집",
    description:
      "Accessible label for the maxmemory-policy edit-in-place pencil",
  },
  "keyvalue.maxmemorySuccess": {
    message: "Maxmemory 정책을 {policy}(으)로 업데이트하는 중…",
    description: "Toast after an eviction-policy update is accepted",
  },
  "keyvalue.maxmemoryError": {
    message: "Maxmemory 정책을 업데이트하지 못했습니다. 다시 시도하세요.",
    description: "Toast when an eviction-policy update fails",
  },
  "keyvalue.persistenceTitle": {
    message: "영속성 모드",
    description: "Key Value detail persistence-mode card title",
  },
  "keyvalue.persistenceDescription": {
    message:
      "모드를 변경하면 스토어가 다시 시작되며, 데이터를 준비하는 동안 연결이 중단됩니다. 저널 + 스냅샷과 스냅샷만 사이를 전환할 때는 데이터가 보존됩니다. 끔으로 또는 끔에서 전환하면 모든 데이터가 삭제됩니다.",
    description: "Key Value detail persistence-mode card description",
  },
  "keyvalue.persistenceLabel": {
    message: "영속성 모드",
    description: "Accessible label for the persistence-mode select",
  },
  "keyvalue.persistenceEdit": {
    message: "영속성 모드 편집",
    description:
      "Accessible label for the persistence-mode edit-in-place pencil",
  },
  "keyvalue.persistenceSuccess": {
    message: "영속성 모드를 {mode}(으)로 업데이트하는 중…",
    description: "Toast after a persistence-mode update is accepted",
  },
  "keyvalue.persistenceConfirmTitle": {
    message: "모든 데이터를 삭제하고 영속성을 변경할까요?",
    description: "Destructive persistence-mode confirmation title",
  },
  "keyvalue.persistenceConfirmDescription": {
    message:
      "끔으로 또는 끔에서 전환하면 {name}의 모든 데이터가 삭제되고 스토어가 다시 시작됩니다. 이 작업은 되돌릴 수 없습니다.",
    description: "Destructive persistence-mode confirmation description",
  },
  "keyvalue.persistenceConfirmAction": {
    message: "데이터 삭제 후 모드 변경",
    description: "Destructive persistence-mode confirmation action",
  },
  "keyvalue.persistenceError": {
    message: "영속성 모드를 업데이트하지 못했습니다. 다시 시도하세요.",
    description: "Toast when a persistence-mode update fails",
  },
  "keyvalue.networkingTitle": {
    message: "네트워킹",
    description: "Detail-page Networking card title (IP allowlist)",
  },
  "keyvalue.networkingDescription": {
    message: "외부 엔드포인트에 접근할 수 있는 소스 IP를 제한합니다.",
    description: "Networking card subtitle",
  },
  "keyvalue.networkingHint": {
    message:
      "CIDR 규칙을 저장하면 일치하는 소스 IP의 외부 액세스가 활성화됩니다. 빈 목록을 저장하면 외부 연결이 차단됩니다. 내부 액세스는 변경되지 않습니다.",
    description: "Networking IP allowlist helper text",
  },
  "keyvalue.networkingHintRestart": {
    message:
      "CIDR 규칙을 저장하면 일치하는 소스 IP의 외부 액세스가 활성화됩니다. 이 스토어는 외부 액세스를 사용한 적이 없으므로 첫 저장 시 다시 시작되며 최대 1분간 연결이 끊어집니다. 빈 목록을 저장하면 외부 연결이 차단됩니다.",
    description:
      "Networking IP allowlist helper text when enabling external access restarts the store (its first enable)",
  },
  "keyvalue.networkingRestartConfirmTitle": {
    message: "모든 데이터를 삭제하고 외부 액세스를 활성화할까요?",
    description:
      "Confirmation title when a first external-access enable restarts a persistence-Off store",
  },
  "keyvalue.networkingRestartConfirmDescription": {
    message:
      "외부 액세스를 활성화하면 {name}이 처음으로 다시 시작됩니다. 영속성 모드가 끔이므로 다시 시작하면 모든 데이터가 삭제됩니다. 이 작업은 되돌릴 수 없습니다.",
    description:
      "Confirmation description when a first external-access enable restarts a persistence-Off store",
  },
  "keyvalue.networkingRestartConfirmAction": {
    message: "데이터 삭제 후 규칙 저장",
    description:
      "Confirmation action when a first external-access enable restarts a persistence-Off store",
  },
  "keyvalue.networkingInternalOnly": {
    message:
      "외부 연결이 비활성화되어 있습니다. 액세스를 활성화하려면 IP 규칙을 하나 이상 저장하세요.",
    description: "External access is disabled; saving rules enables it",
  },
  "keyvalue.networkingEmpty": {
    message: "IP 규칙이 없습니다. 외부 연결을 차단하려면 빈 목록을 저장하세요.",
    description: "Empty datastore allowlist draft and the effect of saving it",
  },
  "keyvalue.networkingAdd": {
    message: "추가",
    description: "Networking button to add a CIDR to the draft allowlist",
  },
  "keyvalue.networkingEntryDescription": {
    message: "설명 (선택 사항)",
    description:
      "Placeholder for the optional per-entry allowlist description input",
  },
  "keyvalue.networkingCIDR": {
    message: "새 CIDR 블록",
    description: "Accessible label for the add-row CIDR input",
  },
  "keyvalue.networkingCIDRRule": {
    message: "규칙 {number}의 CIDR 블록",
    description: "Accessible label for an existing allowlist CIDR row",
  },
  "keyvalue.networkingDescriptionLabel": {
    message: "새 규칙 설명",
    description: "Accessible label for the add-row description input",
  },
  "keyvalue.networkingDescriptionRule": {
    message: "규칙 {number}의 설명",
    description: "Accessible label for an existing allowlist description row",
  },
  "keyvalue.networkingRemove": {
    message: "{cidr} 제거",
    description: "Accessible label to remove a CIDR chip",
  },
  "keyvalue.networkingMoveUp": {
    message: "{cidr} 위로 이동",
    description: "Accessible label to move an allowlist entry earlier",
  },
  "keyvalue.networkingMoveDown": {
    message: "{cidr} 아래로 이동",
    description: "Accessible label to move an allowlist entry later",
  },
  "keyvalue.networkingInvalid": {
    message: "중복되지 않는 올바른 IPv4 또는 IPv6 CIDR 블록을 입력하세요.",
    description: "Validation error for Key Value allowlist entries",
  },
  "keyvalue.networkingSave": {
    message: "허용 목록 저장",
    description: "Networking button to persist the allowlist",
  },
  "keyvalue.networkingSaved": {
    message: "허용 목록을 업데이트했습니다.",
    description: "Toast after saving the IP allowlist",
  },
  "keyvalue.networkingError": {
    message: "허용 목록을 저장하지 못했습니다. 다시 시도하세요.",
    description: "Toast when saving the allowlist fails",
  },
  "keyvalue.nameEdit": {
    message: "Key Value 이름 편집",
    description: "Accessible label for the Key Value name edit-in-place pencil",
  },
  "keyvalue.nameDescription": {
    message:
      "표시 이름을 변경합니다. Key Value ID와 모든 연결 정보는 그대로 유지됩니다.",
    description: "Key Value detail rename row description",
  },
  "keyvalue.nameSuccess": {
    message: "Key Value 스토어 이름을 {name}(으)로 변경했습니다.",
    description: "Toast after a Key Value rename succeeds",
  },
  "keyvalue.nameError": {
    message: "Key Value 스토어 이름을 변경하지 못했습니다. 다시 시도하세요.",
    description: "Toast when a Key Value rename fails unexpectedly",
  },
  "keyvalue.nameConflict": {
    message: "이 워크스페이스에 같은 이름의 Key Value 스토어가 이미 있습니다.",
    description: "Toast when a Key Value rename collides with another name",
  },
  "keyvalue.nameInvalid": {
    message: "줄 바꿈이나 제어 문자 없이 100자 이하의 이름을 입력하세요.",
    description: "Key Value rename validation message",
  },
  // --- Sidebar and page headings (w3/m649) ---
  "keyvalue.navInfo": {
    message: "정보",
    description:
      "Key Value sidebar entry and page heading: the instance's Info page (general details, plan, connections, networking)",
  },
  "keyvalue.navLogs": {
    message: "로그",
    description:
      "Key Value sidebar entry and page heading: the instance's Logs page",
  },
  "keyvalue.navMetrics": {
    message: "메트릭",
    description:
      "Key Value sidebar entry and page heading: the instance's Metrics page",
  },
  "keyvalue.logsRangeLabel": {
    message: "시간 범위",
    description: "Accessible label for the log time-range select",
  },
  "keyvalue.logsAllInstances": {
    message: "모든 인스턴스",
    description: "Default option in the instance filter (no filter applied)",
  },
  "keyvalue.logsSearchPlaceholder": {
    message: "로그 검색…",
    description: "Placeholder for the log text-search input",
  },
  "keyvalue.logsLoading": {
    message: "로그를 불러오는 중…",
    description: "Loading state message while fetching logs",
  },
  "keyvalue.logsEmptyTitle": {
    message: "로그 줄이 없습니다",
    description: "Empty state title when no log lines are returned",
  },
  "keyvalue.logsEmptyBody": {
    message:
      "이 시간 범위에는 로그 줄이 없습니다. Valkey는 키 공간 이벤트, 느린 쿼리, 시작 메시지를 로그로 남깁니다.",
    description: "Empty state body when no log lines and no filters active",
  },
  "keyvalue.logsUnavailableTitle": {
    message: "로그를 사용할 수 없습니다",
    description: "Empty state title when the logs source is not configured",
  },
  "keyvalue.logsUnavailableBody": {
    message:
      "플랫폼의 로그 소스가 구성되어 있지 않습니다. 플랫폼 관리자에게 BEX_LOKI_URL을 활성화해 달라고 요청하세요.",
    description: "Empty state body when BEX_LOKI_URL is not set",
  },
  "keyvalue.logsUnauthorizedTitle": {
    message: "액세스가 거부되었습니다",
    description: "Empty state title when the caller lacks can_view_logs",
  },
  "keyvalue.logsUnauthorizedBody": {
    message: "이 Key Value 스토어의 로그를 볼 권한이 없습니다.",
    description: "Empty state body for a 403 on the logs query",
  },
  "keyvalue.fieldNameDescription": {
    message: "Key Value 인스턴스의 고유한 이름입니다.",
    description: "Create-Key-Value Name row description",
  },
  "keyvalue.fieldVersionDescription": {
    message: "인스턴스가 실행할 Valkey 메이저 버전입니다.",
    description: "Create-Key-Value Version row description",
  },
  "keyvalue.fieldPlanDescription": {
    message: "인스턴스의 사용 가능한 RAM과 연결 수 한도를 정합니다.",
    description:
      "Create-Key-Value plan row description (Render: the plan determines RAM and connection limit)",
  },
  "keyvalue.connectInternalTitle": {
    message: "내부 Redis URL",
    description:
      "Header Connect menu, Internal tab: the private-network URL's title (Render's copy, which names Key Value by its type name Redis, w3/m675)",
  },
  "keyvalue.connectExternalTitle": {
    message: "외부 Redis URL",
    description:
      "Header Connect menu, External tab: the public URL's title (Render's copy, w3/m675)",
  },
  "keyvalue.connectCliTitle": {
    message: "Redis CLI 명령",
    description:
      "Header Connect menu, External tab: the `bex kv-cli <id>` command's title (Render's copy, w3/m675)",
  },
};

export default koKeyValue;
