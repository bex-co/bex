import type { TranslationEntry } from "@/i18n";

const koDatabases: Record<string, TranslationEntry> = {
  "databases.resourceType": {
    message: "데이터베이스",
    description: "Managed Postgres resource type used in document titles",
  },
  "databases.statTotal": {
    message: "전체 데이터베이스",
    description: "Databases page stat card label",
  },
  "databases.statAvailable": {
    message: "사용 가능",
    description: "Databases page stat card label (healthy databases)",
  },
  "databases.statCreating": {
    message: "생성 중",
    description: "Databases page stat card label (provisioning databases)",
  },
  "databases.cardTitle": {
    message: "데이터베이스",
    description: "Databases table card title",
  },
  "databases.colName": {
    message: "이름",
    description: "Databases table column header",
  },
  "databases.colStatus": {
    message: "상태",
    description: "Databases table column header",
  },
  "databases.colPlan": {
    message: "요금제",
    description: "Databases table column header (compute plan / tier)",
  },
  "databases.colVersion": {
    message: "버전",
    description: "Databases table column header (PostgreSQL major version)",
  },
  "databases.colStorage": {
    message: "스토리지",
    description: "Databases table column header (disk size)",
  },
  "databases.colCreated": {
    message: "생성",
    description: "Databases table column header (relative age from createdAt)",
  },
  "databases.colActions": {
    message: "작업",
    description: "Databases table actions column header (screen-reader only)",
  },
  "databases.statusAvailable": {
    message: "사용 가능",
    description: "Database status badge (CNPG cluster healthy)",
  },
  "databases.statusCreating": {
    message: "생성 중",
    description: "Database status badge (provisioning)",
  },
  "databases.statusRestarting": {
    message: "다시 시작 중",
    description:
      "Database status badge (Render's config_restart: an established store restarting after a config change, restart, or resume — not a first provision)",
  },
  "databases.statusUpgrading": {
    message: "업그레이드 중",
    description: "Database status badge (offline PostgreSQL major upgrade)",
  },
  "databases.statusUnavailable": {
    message: "사용 불가",
    description: "Database status badge (provisioning failed)",
  },
  "databases.statusSuspended": {
    message: "일시 중지됨",
    description:
      "Database status badge (hibernated; suspension wins over the status enum)",
  },
  "databases.statusUnknown": {
    message: "알 수 없음",
    description: "Database status badge for an unrecognized status",
  },
  "databases.errorTitle": {
    message: "데이터베이스를 불러오지 못했습니다",
    description: "Databases list error card title",
  },
  "databases.errorBody": {
    message: "bex-api 요청에 실패했습니다. 연결을 확인한 후 다시 시도하세요.",
    description: "Databases list error card body",
  },
  "databases.emptyTitle": {
    message: "아직 데이터베이스가 없습니다",
    description: "Databases list empty state title",
  },
  "databases.emptyBody": {
    message: "첫 관리형 Postgres를 만들면 여기에 표시됩니다.",
    description: "Databases list empty state body",
  },
  "databases.actionsMenu": {
    message: "{name} 작업",
    description: "Accessible label for the per-row actions trigger",
  },
  "databases.actionDelete": {
    message: "삭제",
    description: "Row action: permanently delete the database",
  },
  "databases.deleteConfirmTitle": {
    message: "{name} 데이터베이스를 삭제할까요?",
    description: "Delete-confirmation dialog title",
  },
  "databases.deleteConfirmBody": {
    message:
      "이 작업은 되돌릴 수 없습니다. 이 Postgres 인스턴스를 삭제하시겠습니까?",
    description: "Delete-confirmation dialog body (Render-style copy)",
  },
  "databases.deleteConfirmPrompt": {
    message: "확인하려면 아래에 {phrase}을(를) 입력하세요.",
    description:
      "Body prompt naming the exact 'sudo delete postgres <name>' phrase (rendered bold by SudoCommandField)",
  },
  "databases.deleteCancel": {
    message: "취소",
    description: "Delete-confirmation dialog cancel button",
  },
  "databases.deleteConfirm": {
    message: "데이터베이스 삭제",
    description: "Delete-confirmation dialog confirm button",
  },
  "databases.deleteSuccess": {
    message: "{name} 삭제 중…",
    description: "Toast after a delete request is accepted",
  },
  "databases.deleteError": {
    message: "{name} 데이터베이스를 삭제하지 못했습니다. 다시 시도하세요.",
    description: "Toast when a delete request fails",
  },
  "databases.createButton": {
    message: "새 데이터베이스",
    description: "Button that opens the create-database dialog",
  },
  "databases.createTitle": {
    message: "새 Postgres",
    description: "Create-database dialog title",
  },
  "databases.createDescription": {
    message: "관리형 PostgreSQL 인스턴스를 프로비저닝합니다.",
    description: "Create-database dialog description",
  },
  "databases.fieldName": {
    message: "이름",
    description: "Create-database form field label (database name)",
  },
  "databases.fieldNamePlaceholder": {
    message: "my-database",
    description: "Create-database name input placeholder",
  },
  "databases.fieldNameError": {
    message: "줄 바꿈이나 제어 문자 없이 100자 이하의 이름을 입력하세요.",
    description: "Create-database name validation message",
  },
  "databases.physicalNamesTitle": {
    message: "PostgreSQL 이름 (선택 사항)",
    description: "Create-database advanced physical-name section title",
  },
  "databases.physicalNamesHint": {
    message:
      "생성할 때만 설정할 수 있습니다. 비워 두면 고정된 이름이 자동으로 생성됩니다.",
    description: "Create-database physical-name helper text",
  },
  "databases.fieldDatabaseName": {
    message: "데이터베이스 이름",
    description: "Create-database physical PostgreSQL database label",
  },
  "databases.fieldDatabaseNamePlaceholder": {
    message: "orders_data",
    description: "Create-database physical PostgreSQL database placeholder",
  },
  "databases.fieldDatabaseUser": {
    message: "데이터베이스 사용자",
    description: "Create-database physical PostgreSQL owner-role label",
  },
  "databases.fieldDatabaseUserPlaceholder": {
    message: "orders_owner",
    description: "Create-database physical PostgreSQL owner-role placeholder",
  },
  "databases.fieldPhysicalNameReserved": {
    message: "“{name}”은(는) PostgreSQL 예약어입니다. 다른 이름을 선택하세요.",
    description:
      "Create-database / add-user validation for a name the managed PostgreSQL service reserves (postgres, template0/1, streaming_replica, public, none, pg_*, cnpg_*)",
  },
  "databases.fieldPhysicalNameError": {
    message:
      "소문자, 숫자, 밑줄만 사용해 63자 이하로 입력하고, 문자 또는 밑줄로 시작하세요.",
    description: "Create-database physical PostgreSQL identifier validation",
  },
  "databases.fieldPlan": {
    message: "컴퓨팅 플랜",
    description: "Create-database form field label (plan / tier)",
  },
  "databases.fieldPlanPlaceholder": {
    message: "컴퓨팅 플랜 선택",
    description: "Create-database plan select placeholder",
  },
  "databases.fieldVersion": {
    message: "PostgreSQL 버전",
    description: "Create-database form field label (major version)",
  },
  "databases.fieldVersionDefault": {
    message: "기본값 (최신)",
    description: "Create-database version select default option",
  },
  "databases.fieldDisk": {
    message: "디스크 크기 (GB)",
    description: "Create-database form field label (storage size)",
  },
  "databases.fieldDiskFixed": {
    message: "이 요금제는 디스크 크기가 {size} GB로 고정되어 있습니다.",
    description:
      "Create-database hint under a plan's fixed disk size, e.g. Free (w4/376)",
  },
  "databases.fieldDiskError": {
    message: "{min}~{max} GB 사이의 정수를 입력하세요.",
    description:
      "Create-database validation message when the disk size is below the plan floor or above the volume cap",
  },
  "databases.fieldPublic": {
    message: "공개 액세스",
    description: "Create-database form field label (external endpoint toggle)",
  },
  "databases.fieldPublicHint": {
    message:
      "액세스 제어에서 제한하기 전까지는 유효한 자격 증명이 있으면 모든 IP 주소의 연결을 허용합니다.",
    description:
      "Create-database public toggle helper text; the toggle starts on, as a Render database is reachable by default (w4/553)",
  },
  "databases.fieldPublicHintOff": {
    message:
      "외부 연결이 차단됩니다. 프라이빗 네트워크의 서비스는 계속 연결할 수 있습니다.",
    description:
      "Create-database public toggle helper text once switched off: the database is created private (w4/553)",
  },
  "databases.fieldHighAvailability": {
    message: "고가용성",
    description: "Create-database high availability toggle label (w2/m171)",
  },
  "databases.fieldHighAvailabilityHint": {
    message:
      "다른 노드에서 대기 인스턴스를 실행하며, 프라이머리에 장애가 발생하면 대신 역할을 이어받습니다. 대기 인스턴스는 프라이머리와 동일하게 과금됩니다.",
    description:
      "Create-database HA toggle helper text on a plan that offers HA",
  },
  "databases.fieldHighAvailabilityPlanUnsupported": {
    message:
      "{name} 요금제는 고가용성을 지원하지 않습니다. CPU가 1개 이상인 요금제를 선택하세요.",
    description:
      "Create-database HA toggle helper text when the selected plan is below 1 CPU (Render's rule)",
  },
  "databases.createCancel": {
    message: "취소",
    description: "Create-database dialog cancel button",
  },
  "databases.createSubmit": {
    message: "데이터베이스 생성",
    description: "Create-database dialog submit button",
  },
  "databases.createSuccess": {
    message: "{name} 생성 중…",
    description:
      "Toast after a create request is accepted (provisioning is async)",
  },
  "databases.createError": {
    message: "{name} 데이터베이스를 생성하지 못했습니다. 다시 시도하세요.",
    description: "Toast when a create request fails",
  },
  "databases.capLimitTitle": {
    message: "Postgres 한도에 도달했습니다",
    description:
      "Alert title when the workspace's Postgres creation cap is hit (w7/m9)",
  },
  "databases.capLimitUpgrade": {
    message: "요금제 업그레이드",
    description:
      "Upgrade CTA button inside the Postgres cap-limit Alert (w7/m9)",
  },
  "databases.metaTitle": {
    message: "세부 정보",
    description: "Database detail metadata card title",
  },
  "databases.statusReasonClusterFailed": {
    message:
      "데이터베이스 클러스터가 구성을 거부하여 최신 변경 사항을 적용하지 못했습니다.",
    description:
      "Why an unavailable database is unavailable: the operator's ClusterFailed Ready reason (bex-api statusReasonCode)",
  },
  "databases.statusReasonClusterReadFailed": {
    message: "데이터베이스 클러스터의 상태를 읽을 수 없습니다.",
    description:
      "Why an unavailable database is unavailable: the operator's ClusterReadFailed Ready reason (bex-api statusReasonCode)",
  },
  "databases.statusReasonNetworkPolicyFailed": {
    message: "데이터베이스의 네트워크 정책을 적용하지 못했습니다.",
    description:
      "Why an unavailable database is unavailable: the operator's NetworkPolicyFailed Ready reason (bex-api statusReasonCode)",
  },
  "databases.statusReasonDiskAutoscalingFailed": {
    message: "디스크 자동 확장을 적용하지 못했습니다.",
    description:
      "Why an unavailable database is unavailable: the operator's DiskAutoscalingFailed Ready reason (bex-api statusReasonCode)",
  },
  "databases.statusReasonExportFailed": {
    message: "데이터베이스 내보내기를 시작하지 못했습니다.",
    description:
      "Why an unavailable database is unavailable: the operator's ExportFailed Ready reason (bex-api statusReasonCode)",
  },
  "databases.statusReasonPoolerFailed": {
    message: "커넥션 풀러를 프로비저닝하지 못했습니다.",
    description:
      "Why an unavailable database is unavailable: the operator's PoolerFailed Ready reason (bex-api statusReasonCode)",
  },
  "databases.statusReasonExtensionsFailed": {
    message:
      "데이터베이스의 관리형 확장을 설치하지 못해 아직 사용할 수 없습니다.",
    description:
      "Why an unavailable database is unavailable: the operator's ExtensionsFailed Ready reason (bex-api statusReasonCode)",
  },
  "databases.statusReasonPostUpgradeBackupFailed": {
    message: "버전 업그레이드 후 백업을 만들지 못했습니다.",
    description:
      "Why an unavailable database is unavailable: the operator's PostUpgradeBackupFailed Ready reason (bex-api statusReasonCode)",
  },
  "databases.statusReasonMajorVersionUpgradeFailed": {
    message:
      "메이저 버전 업그레이드에 실패했습니다. 데이터베이스는 현재 버전을 유지합니다.",
    description:
      "Why an unavailable database is unavailable: the operator's MajorVersionUpgradeFailed Ready reason (bex-api statusReasonCode)",
  },
  "databases.statusReasonStorageShrinkRejected": {
    message:
      "Postgres 스토리지는 늘리기만 가능합니다. 디스크는 할당된 {size} GB 미만으로 줄일 수 없습니다.",
    description:
      "Why an unavailable database is unavailable: the operator's StorageShrinkRejected Ready reason (bex-api statusReasonCode)",
  },
  "databases.statusReasonRecoveryUnavailable": {
    message: "백업 저장소가 구성되지 않아 데이터베이스를 복원할 수 없습니다.",
    description:
      "Why an unavailable database is unavailable: the operator's RecoveryUnavailable Ready reason (bex-api statusReasonCode)",
  },
  "databases.statusReasonBackupStoreUnavailable": {
    message: "데이터베이스의 백업 스토리지를 설정하지 못했습니다.",
    description:
      "Why an unavailable database is unavailable: the operator's BackupStoreUnavailable Ready reason (bex-api statusReasonCode)",
  },
  "databases.statusReasonScheduledBackupFailed": {
    message: "일일 백업을 예약하지 못했습니다.",
    description:
      "Why an unavailable database is unavailable: the operator's ScheduledBackupFailed Ready reason (bex-api statusReasonCode)",
  },
  "databases.statusReasonScheduledBackupCleanupFailed": {
    message: "일일 백업 일정을 제거하지 못했습니다.",
    description:
      "Why an unavailable database is unavailable: the operator's ScheduledBackupCleanupFailed Ready reason (bex-api statusReasonCode)",
  },
  "databases.metaStatus": {
    message: "상태",
    description: "Database detail metadata row label",
  },
  "databases.metaPlan": {
    message: "컴퓨팅 플랜",
    description: "Database detail metadata row label",
  },
  "databases.metaVersion": {
    message: "버전",
    description: "Database detail metadata row label",
  },
  "databases.versionLabel": {
    message: "PostgreSQL {version}",
    description: "Formatted PostgreSQL major version label",
  },
  "databases.versionUpgradeAction": {
    message: "업그레이드",
    description: "Button opening the PostgreSQL major-version upgrade dialog",
  },
  "databases.versionUpgrading": {
    message: "업그레이드 진행 중",
    description:
      "Version-row status while the database is offline for pg_upgrade",
  },
  "databases.versionUpgradeTitle": {
    message: "PostgreSQL 업그레이드",
    description: "PostgreSQL major-version upgrade dialog title",
  },
  "databases.versionUpgradeDescription": {
    message:
      "이 데이터베이스를 PostgreSQL {version}에서 지원되는 더 최신 메이저 버전으로 업그레이드합니다.",
    description: "PostgreSQL major-version upgrade dialog description",
  },
  "databases.versionUpgradeTarget": {
    message: "대상 버전",
    description: "PostgreSQL major-version upgrade target selector label",
  },
  "databases.versionUpgradeDowntime": {
    message:
      "오프라인 업그레이드가 실행되는 동안 데이터베이스를 사용할 수 없습니다. 먼저 호환성을 테스트하고 다운타임을 계획하세요. 대용량 데이터베이스는 최대 1시간까지 걸릴 수 있습니다.",
    description:
      "Downtime warning in the PostgreSQL major-version upgrade dialog",
  },
  "databases.versionUpgradeErrorTitle": {
    message: "업그레이드가 차단되었습니다",
    description:
      "Inline backend-guard error title in the version upgrade dialog",
  },
  "databases.versionUpgradeCancel": {
    message: "취소",
    description: "PostgreSQL major-version upgrade dialog cancel button",
  },
  "databases.versionUpgradeConfirm": {
    message: "PostgreSQL {version}(으)로 업그레이드",
    description: "PostgreSQL major-version upgrade dialog confirmation button",
  },
  "databases.versionUpgradeAccepted": {
    message: "PostgreSQL {version} 업그레이드를 시작했습니다.",
    description: "Toast after a PostgreSQL major-version upgrade is accepted",
  },
  "databases.versionUpgradeError": {
    message: "PostgreSQL 업그레이드를 시작하지 못했습니다. 다시 시도하세요.",
    description: "Fallback error when the PostgreSQL upgrade mutation fails",
  },
  "databases.metaDatabaseName": {
    message: "데이터베이스",
    description: "Database detail metadata row label (the normalized db name)",
  },
  "databases.metaDatabaseUser": {
    message: "사용자",
    description: "Database detail metadata row label (owner role)",
  },
  "databases.metaStorage": {
    message: "스토리지",
    description: "Database detail metadata row label (disk size)",
  },
  "databases.diskAutoscalingLabel": {
    message: "디스크 자동 확장",
    description: "Label beside the database disk-usage chart toggle",
  },
  "databases.diskAutoscalingSize": {
    message: "현재 {current} GB · 최대 {cap} GB",
    description: "Current and maximum Postgres disk size beside autoscaling",
  },
  "databases.diskAutoscalingHint": {
    message:
      "사용량이 90%에 도달하면 스토리지가 50% 늘어나며 5 GB 단위로 올림됩니다. 증설은 되돌릴 수 없으며 12시간에 한 번만 가능합니다.",
    description: "Accessible explanation of Postgres disk autoscaling",
  },
  "databases.diskAutoscalingCurrentSize": {
    message: "현재 {current} GB",
    description: "Current disk size while plan limits are unavailable",
  },
  "databases.diskAutoscalingPlanUnsupported": {
    message:
      "이 요금제는 디스크 자동 확장을 지원하지 않습니다. 기존 설정은 끌 수 있습니다.",
    description: "Plan restriction beside the disk autoscaling switch",
  },
  "databases.diskAutoscalingEnabled": {
    message: "디스크 자동 확장을 켰습니다.",
    description: "Toast after enabling Postgres disk autoscaling",
  },
  "databases.diskAutoscalingDisabled": {
    message: "디스크 자동 확장을 껐습니다.",
    description: "Toast after disabling Postgres disk autoscaling",
  },
  "databases.diskAutoscalingError": {
    message: "디스크 자동 확장을 업데이트하지 못했습니다. 다시 시도하세요.",
    description: "Toast when the disk-autoscaling mutation fails",
  },
  "databases.metaHighAvailability": {
    message: "고가용성",
    description:
      "Database detail metadata row label (HA — single-instance MVP is No)",
  },
  "databases.metaPublic": {
    message: "공개 액세스",
    description: "Database detail metadata row label (external endpoint)",
  },
  "databases.metaExternalHost": {
    message: "외부 호스트",
    description: "Database detail metadata row label (SNI hostname)",
  },
  "databases.metaRegion": {
    message: "리전",
    description: "Database detail metadata row label (platform placement)",
  },
  "databases.metaCreated": {
    message: "생성",
    description: "Database detail metadata row label (relative age)",
  },
  "databases.metaExpires": {
    message: "만료",
    description:
      "Details row: when a free database expires (Render's expiresAt, w8/m126)",
  },
  "databases.expirySoon": {
    message:
      "이 무료 데이터베이스는 {expiresAt}에 만료되며 이후 서비스가 중단됩니다.",
    description: "Warning within 7 days of a free database's expiry",
  },
  "databases.expiryExpired": {
    message:
      "이 무료 데이터베이스는 만료되어 일시 중지되었습니다. 데이터베이스와 데이터는 {deleteAt}에 삭제됩니다.",
    description:
      "Warning on an expired free database during its 14-day grace period",
  },
  "databases.expiryUpgrade": {
    message: "유료 요금제로 업그레이드",
    description: "Link from the expiry warning to the Plan section",
  },
  "databases.expiryHintSoon": {
    message: "곧 만료됨",
    description: "List-row hint on a free database within 7 days of expiry",
  },
  "databases.expiryHintExpired": {
    message: "만료됨",
    description: "List-row hint on an expired free database",
  },
  "databases.statusReasonFreeExpired": {
    message:
      "이 무료 데이터베이스가 만료되었습니다. 유료 요금제로 업그레이드하면 복원됩니다.",
    description:
      "Why a suspended database is suspended: its free plan expired (bex-api statusReasonCode POSTGRES_FREE_EXPIRED, w8/m126)",
  },
  "databases.yes": {
    message: "예",
    description: "Metadata value for a true boolean field",
  },
  "databases.no": {
    message: "아니요",
    description: "Metadata value for a false boolean field",
  },
  "databases.connTitle": {
    message: "연결",
    description: "Connection-info panel card title",
  },
  "databases.connDescription": {
    message:
      "연결 문자열과 데이터베이스 비밀번호입니다. 요청할 때만 표시되며 자동으로 표시되지 않습니다.",
    description: "Connection-info panel card description",
  },
  "databases.connReveal": {
    message: "연결 정보 표시",
    description: "Button that fetches the connection info on demand",
  },
  "databases.connHide": {
    message: "연결 정보 숨기기",
    description: "Button that clears the revealed connection info",
  },
  "databases.connLogicalReplicationTitle": {
    message: "논리 복제",
    description:
      "Heading of the logical-replication note on the Connections card",
  },
  "databases.connLogicalReplicationBody": {
    message:
      "기본적으로 꺼져 있습니다. Pro 이상 워크스페이스에서는 스토리지가 10GB 이상인 데이터베이스가 bex 안팎의 구독자에게 변경 사항을 게시할 수 있습니다. 사용하려면 이 데이터베이스의 ID, 구독자가 연결할 자격 증명, 읽을 스키마, 모든 테이블을 게시할지 여부를 지원팀에 알려 주세요. 그런 다음 이 데이터베이스의 사용자로 CREATE PUBLICATION … FOR TABLE로 게시를 만들고, 구독자는 풀링 URL이 아닌 외부 URL에 연결하세요. bex 안팎의 다른 데이터베이스를 구독할 수도 있습니다. 같은 방법으로 구독을 만들 자격 증명을 알려 요청하고, 테이블을 만든 뒤 CREATE SUBSCRIPTION을 실행하세요(PostgreSQL 15 이하에서는 지원팀이 구독을 만들어 드립니다).",
    description:
      "Logical-replication note on the Connections card: off by default, enabled on request (Render's rules)",
  },
  "databases.connPassword": {
    message: "비밀번호",
    description: "Connection-info field label (database password)",
  },
  "databases.connShowPassword": {
    message: "비밀번호 표시",
    description: "Accessible label to unmask the password",
  },
  "databases.connHidePassword": {
    message: "비밀번호 숨기기",
    description: "Accessible label to re-mask the password",
  },
  "databases.connInternal": {
    message: "내부 연결 문자열",
    description: "Connection-info field label (in-cluster URL)",
  },
  "databases.connExternal": {
    message: "외부 연결 문자열",
    description: "Connection-info field label (public URL, TLS)",
  },
  "databases.connPsql": {
    message: "psql 명령",
    description: "Connection-info field label (ready-to-run psql command)",
  },
  "databases.connCaLabel": {
    message: "서버 CA 인증서",
    description: "Connection-info field label (private server CA bundle)",
  },
  "databases.connCaDownload": {
    message: "CA 인증서 다운로드",
    description: "Button downloading the server CA PEM file",
  },
  "databases.connCaHelp": {
    message:
      "외부 연결은 이 데이터베이스의 인증서를 프라이빗 서버 CA로 검증합니다(sslmode=verify-full은 계속 필요합니다). {file}을 다운로드한 뒤 클라이언트가 저장된 파일을 가리키도록 하세요. psql 명령이나 외부 연결 문자열 앞에 다음을 붙이면 됩니다.",
    description:
      "Explanation above the PGSSLROOTCERT example; {file} is the downloaded CA filename",
  },
  "databases.connCaSnippet": {
    message: "다운로드한 CA를 가리키는 환경 변수 접두사 예시",
    description:
      "Accessible label for the copyable PGSSLROOTCERT example snippet",
  },
  "databases.connCertPendingTitle": {
    message: "공개적으로 신뢰되는 인증서를 발급하는 중입니다",
    description:
      "Connections card note title while a public database's Let's Encrypt certificate is still pending (w4/m357)",
  },
  "databases.connCertPendingBody": {
    message:
      "공개적으로 신뢰되는 인증서를 발급하는 중입니다(보통 몇 분 소요). 그동안에는 아래 CA 또는 sslmode=require로 연결하세요. 발급이 완료되면 Render와 같이 외부 URL에 CA가 필요하지 않습니다. 숨겼다가 다시 표시하면 새로 고쳐집니다.",
    description:
      "Connections card note body while the public database certificate is pending",
  },
  "databases.connCertFailedTitle": {
    message: "공개적으로 신뢰되는 인증서가 아직 발급되지 않았습니다",
    description:
      "Connections card note title when issuing a public database's certificate failed (w4/m357)",
  },
  "databases.connCertFailedBody": {
    message:
      "이 데이터베이스의 공개적으로 신뢰되는 인증서 발급에 실패했으며, bex가 자동으로 다시 시도합니다. 그동안에도 CA 방식은 사용할 수 있습니다. 아래 CA 또는 sslmode=require로 연결하세요.",
    description:
      "Connections card note body when the public database certificate failed to issue",
  },
  "databases.connErrorTitle": {
    message: "연결 정보를 불러오지 못했습니다",
    description: "Connection-info panel error title",
  },
  "databases.connErrorBody": {
    message:
      "데이터베이스가 아직 프로비저닝 중이거나 자격 증명을 볼 권한이 없을 수 있습니다.",
    description: "Connection-info panel error body",
  },
  "databases.copied": {
    message: "클립보드에 복사했습니다",
    description: "Toast after copying a connection field",
  },
  "databases.copyError": {
    message: "클립보드에 복사하지 못했습니다",
    description: "Toast when clipboard copy fails",
  },
  "databases.loading": {
    message: "불러오는 중…",
    description: "Generic loading placeholder in a detail panel",
  },
  "databases.dangerDelete": {
    message: "데이터베이스 삭제",
    description: "Detail-page bottom action: permanently delete the database",
  },
  "databases.dangerRestart": {
    message: "데이터베이스 다시 시작",
    description: "Detail-page bottom action: rolling restart of the primary",
  },
  "databases.dangerSuspend": {
    message: "데이터베이스 일시 중지",
    description:
      "Detail-page bottom action: hibernate the database (stop compute, keep data)",
  },
  "databases.dangerResume": {
    message: "데이터베이스 재개",
    description: "Detail-page bottom action: wake a suspended database",
  },
  "databases.actionSuspend": {
    message: "일시 중지",
    description: "Row action: hibernate the database (stop compute, keep data)",
  },
  "databases.actionResume": {
    message: "재개",
    description: "Row action: wake a suspended database",
  },
  "databases.actionRestart": {
    message: "다시 시작",
    description: "Row action: rolling restart of the primary",
  },
  "databases.suspendConfirmTitle": {
    message: "{name} 데이터베이스를 일시 중지할까요?",
    description: "Suspend-confirmation dialog title",
  },
  "databases.suspendConfirmBody": {
    message:
      "컴퓨팅이 중지되어 데이터베이스에 접속할 수 없게 되지만 데이터는 보존됩니다. 언제든 재개하여 다시 사용할 수 있습니다.",
    description: "Suspend-confirmation dialog body",
  },
  "databases.restartConfirmTitle": {
    message: "{name} 데이터베이스를 다시 시작할까요?",
    description: "Restart-confirmation dialog title",
  },
  "databases.restartConfirmBody": {
    message:
      "프라이머리가 다시 시작됩니다. 복구되는 동안 연결이 잠시 끊어집니다.",
    description: "Restart-confirmation dialog body",
  },
  "databases.toastSuspendSuccess": {
    message: "{name} 일시 중지 중…",
    description: "Toast after a suspend request is accepted",
  },
  "databases.toastResumeSuccess": {
    message: "{name} 재개 중…",
    description: "Toast after a resume request is accepted",
  },
  "databases.toastRestartSuccess": {
    message: "{name} 다시 시작 중…",
    description: "Toast after a restart request is accepted",
  },
  "databases.toastLifecycleError": {
    message:
      "{name} 데이터베이스에서 작업을 완료하지 못했습니다. 다시 시도하세요.",
    description: "Toast when a lifecycle verb fails",
  },
  "databases.recoveryTitle": {
    message: "복구",
    description: "Recovery panel card title",
  },
  "databases.recoveryDescription": {
    message:
      "특정 시점 복구와 백업입니다. 복원하면 항상 새 데이터베이스가 만들어지며 현재 데이터베이스는 변경되지 않습니다.",
    description: "Recovery panel card description",
  },
  "databases.recoveryDisabled": {
    message:
      "이 요금제에서는 백업이 활성화되어 있지 않아 복구를 사용할 수 없습니다. 특정 시점 복구를 사용하려면 백업이 포함된 요금제로 업그레이드하세요.",
    description: "Recovery panel state when the plan has no backups",
  },
  "databases.recoveryUnavailable": {
    message: "복구 정보를 읽을 수 없습니다. 다시 시도하세요.",
    description: "Recovery panel state when backup metadata could not be read",
  },
  "databases.recoveryEarliest": {
    message: "가장 이른 복원 시점",
    description: "Recovery panel field (earliest recoverable time)",
  },
  "databases.recoveryLatest": {
    message: "가장 최근 복원 시점",
    description: "Recovery panel field (latest recoverable time)",
  },
  "databases.recoveryWindowNotEstablished": {
    message: "아직 설정되지 않음",
    description: "Recovery panel value before the PITR window is established",
  },
  "databases.recoveryRestore": {
    message: "새 인스턴스로 복원",
    description: "Recovery panel button that opens the restore dialog",
  },
  "databases.recoveryRestoreUnavailable": {
    message: "첫 번째 복구 가능 시점이 만들어지면 복원할 수 있습니다.",
    description: "Reason restore is disabled before the PITR window opens",
  },
  "databases.recoveryCreateExport": {
    message: "내보내기 생성",
    description: "Recovery panel button that triggers an on-demand export",
  },
  "databases.recoveryBackups": {
    message: "백업",
    description: "Recovery panel backup-list heading",
  },
  "databases.recoveryNoBackups": {
    message: "아직 백업이 없습니다.",
    description: "Recovery panel empty backup list",
  },
  "databases.recoveryExportScope": {
    message:
      "내보내기에는 {database} 데이터베이스만 포함됩니다. 이 인스턴스에 생성된 다른 데이터베이스는 포함되지 않습니다.",
    description:
      "Recovery panel note naming the one database a logical export contains (w8/258)",
  },
  "databases.recoveryExports": {
    message: "내보내기",
    description: "Recovery panel export-list heading",
  },
  "databases.recoveryNoExports": {
    message: "아직 내보내기가 없습니다.",
    description: "Recovery panel empty export list",
  },
  "databases.recoveryDownloadExport": {
    message: "다운로드",
    description: "Download an available logical database export",
  },
  "databases.recoveryDownloadUnavailable": {
    message: "이 내보내기는 더 이상 다운로드할 수 없습니다.",
    description: "Toast when Download finds the export is no longer available",
  },
  "databases.recoveryDownloadError": {
    message: "다운로드 링크를 가져오지 못했습니다. 다시 시도하세요.",
    description: "Toast when the download link of an export cannot be fetched",
  },
  "databases.recoveryExportInProgress": {
    message: "이미 내보내기가 진행 중입니다.",
    description: "Reason the create-export button is disabled",
  },
  "databases.recoveryExportPreparing": {
    message: "내보내기를 아직 준비하는 중입니다.",
    description: "Reason an in-progress export cannot be downloaded",
  },
  "databases.recoveryExportExpired": {
    message: "이 내보내기는 만료되어 파일이 삭제되었습니다.",
    description: "Reason an expired export cannot be downloaded",
  },
  "databases.recoveryExportUnavailable": {
    message: "이 내보내기는 다운로드할 수 없습니다.",
    description: "Fallback reason an export cannot be downloaded",
  },
  "databases.recoveryExportFailure": {
    message: "내보내기 실패: {reason}",
    description: "Logical export failure reason",
  },
  "databases.recoveryExportStatusCreated": {
    message: "생성됨",
    description: "Logical export status",
  },
  "databases.recoveryExportStatusRunning": {
    message: "실행 중",
    description: "Logical export status",
  },
  "databases.recoveryExportStatusAvailable": {
    message: "사용 가능",
    description: "Logical export status",
  },
  "databases.recoveryExportStatusFailed": {
    message: "실패",
    description: "Logical export status",
  },
  "databases.recoveryExportStatusExpiring": {
    message: "만료 예정",
    description: "Logical export status",
  },
  "databases.recoveryExportStatusExpired": {
    message: "만료됨",
    description: "Logical export status",
  },
  "databases.recoveryRestoreTitle": {
    message: "새 데이터베이스로 복원",
    description: "Restore dialog title",
  },
  "databases.recoveryRestoreBody": {
    message:
      "이 데이터베이스의 백업에서 복원한 새 데이터베이스를 프로비저닝합니다. 원본은 변경되지 않습니다.",
    description: "Restore dialog body",
  },
  "databases.recoveryRestoreName": {
    message: "새 데이터베이스 이름",
    description: "Restore dialog field label",
  },
  "databases.recoveryRestoreTime": {
    message: "시점 (선택 사항)",
    description: "Restore dialog target-time field label",
  },
  "databases.recoveryRestoreTimeHint": {
    message: "비워 두면 사용 가능한 가장 최근 시점으로 복원합니다.",
    description: "Restore dialog target-time helper text",
  },
  "databases.recoveryRestoreTimeZone": {
    message: "시간은 사용자의 현지 시간대({zone}) 기준입니다.",
    description: "Restore dialog target-time zone note",
  },
  "databases.recoveryRestoreTimeOutOfWindow": {
    message: "가장 이른 복원 시점과 10분 전 사이의 시간을 선택하세요.",
    description:
      "Restore dialog target-time out-of-window error (the window ends ten minutes before now, w7/m453)",
  },
  "databases.recoveryRetentionWindow_other": {
    message:
      "{plan} 요금제에서는 지난 {count}일 동안의 모든 시점으로 복원할 수 있습니다.",
    description:
      "Recovery panel hint: how far back the workspace plan may restore a Postgres (w7/m453, from bex-api databaseRecoveryInfo.retention)",
  },
  "databases.recoveryRetentionUpgrade_other": {
    message: "{plan} 요금제는 {count}일입니다.",
    description:
      "Recovery panel hint: the cheapest plan that restores further back, and its days (w7/m453)",
  },
  "databases.recoveryRestoreConfirm": {
    message: "복원",
    description: "Restore dialog confirm button",
  },
  "databases.recoveryExportStarted": {
    message: "내보내기를 시작했습니다.",
    description: "Toast after an export is triggered",
  },
  "databases.recoveryExportError": {
    message: "내보내기를 시작하지 못했습니다. 다시 시도하세요.",
    description: "Toast when an export request fails",
  },
  "databases.recoveryRestoreStarted": {
    message: "{name}(으)로 복원 중…",
    description: "Toast after a restore request is accepted",
  },
  "databases.recoveryRestoreError": {
    message: "복원하지 못했습니다. 다시 시도하세요.",
    description: "Toast when a restore request fails",
  },
  "databases.accessTitle": {
    message: "액세스 제어",
    description: "Access-control panel card title",
  },
  "databases.accessDescription": {
    message:
      "외부 엔드포인트를 제한하고, 연결을 풀링하고, 데이터베이스 사용자를 관리합니다.",
    description: "Access-control panel card description",
  },
  "databases.accessAllowList": {
    message: "IP 허용 목록",
    description:
      "Access panel section heading (external-endpoint CIDR allowlist)",
  },
  "databases.accessAllowListHint": {
    message:
      "CIDR 규칙을 저장하면 일치하는 소스 IP의 외부 액세스가 활성화됩니다. 빈 목록을 저장하면 외부 연결이 차단됩니다. 내부 액세스는 변경되지 않습니다.",
    description: "Access panel IP allowlist helper text",
  },
  "databases.accessAllowListInternalOnly": {
    message:
      "외부 연결이 비활성화되어 있습니다. 액세스를 활성화하려면 IP 규칙을 하나 이상 저장하세요.",
    description: "External access is disabled; saving rules enables it",
  },
  "databases.accessAllowListEmpty": {
    message: "IP 규칙이 없습니다. 외부 연결을 차단하려면 빈 목록을 저장하세요.",
    description: "Empty datastore allowlist draft and the effect of saving it",
  },
  "databases.accessAllowListAdd": {
    message: "추가",
    description: "Access panel button to add a CIDR to the draft allowlist",
  },
  "databases.accessAllowListDescription": {
    message: "설명 (선택 사항)",
    description:
      "Placeholder for the optional per-entry allowlist description input",
  },
  "databases.accessAllowListCIDR": {
    message: "새 CIDR 블록",
    description: "Accessible label for the add-row CIDR input",
  },
  "databases.accessAllowListCIDRRule": {
    message: "규칙 {number}의 CIDR 블록",
    description: "Accessible label for an existing allowlist CIDR row",
  },
  "databases.accessAllowListDescriptionLabel": {
    message: "새 규칙 설명",
    description: "Accessible label for the add-row description input",
  },
  "databases.accessAllowListDescriptionRule": {
    message: "규칙 {number}의 설명",
    description: "Accessible label for an existing allowlist description row",
  },
  "databases.accessAllowListRemove": {
    message: "{cidr} 제거",
    description: "Accessible label to remove a CIDR chip",
  },
  "databases.accessAllowListMoveUp": {
    message: "{cidr} 위로 이동",
    description: "Accessible label to move an allowlist entry earlier",
  },
  "databases.accessAllowListMoveDown": {
    message: "{cidr} 아래로 이동",
    description: "Accessible label to move an allowlist entry later",
  },
  "databases.accessAllowListInvalid": {
    message: "중복되지 않는 올바른 IPv4 또는 IPv6 CIDR 블록을 입력하세요.",
    description: "Validation error for database allowlist entries",
  },
  "databases.accessAllowListSave": {
    message: "허용 목록 저장",
    description: "Access panel button to persist the allowlist",
  },
  "databases.accessAllowListSaved": {
    message: "허용 목록을 업데이트했습니다.",
    description: "Toast after saving the IP allowlist",
  },
  "databases.accessAllowListError": {
    message: "허용 목록을 저장하지 못했습니다. 다시 시도하세요.",
    description: "Toast when saving the allowlist fails",
  },
  "databases.accessCredentials": {
    message: "자격 증명",
    description:
      "Access panel section heading (Postgres credentials, Render's Credentials section)",
  },
  "databases.accessCredentialsHint": {
    message:
      "새 기본 자격 증명을 추가하면 데이터베이스 비밀번호가 교체됩니다. 연결 정보는 새 자격 증명으로 전환되고, 연결된 서비스는 다음 Blueprint 동기화 때 이를 적용합니다. 이전 자격 증명은 삭제하기 전까지 계속 작동합니다.",
    description: "Access panel credentials helper text",
  },
  "databases.accessCredentialDefault": {
    message: "기본",
    description: "Badge on the database's default credential",
  },
  "databases.accessCredentialAdd": {
    message: "새 기본 자격 증명",
    description:
      "Access panel button to create a credential that becomes the default",
  },
  "databases.accessCredentialName": {
    message: "새 자격 증명 사용자 이름",
    description: "Accessible label for the new-credential username input",
  },
  "databases.accessCredentialDelete": {
    message: "{name} 삭제",
    description: "Accessible label to delete a database credential",
  },
  "databases.accessCredentialDeleteTitle": {
    message: "{name} 자격 증명을 삭제할까요?",
    description:
      "Title of the confirmation before deleting a database credential (w4/551)",
  },
  "databases.accessCredentialDeleteBody": {
    message:
      "{name}을(를) 사용하는 모든 연결이 즉시 실패하며, 연결 문자열이 환경 변수에 들어 있는 서비스도 마찬가지입니다. 이 작업은 되돌릴 수 없습니다.",
    description:
      "Body of the credential delete confirmation: retiring the login breaks its clients at once (w4/551)",
  },
  "databases.accessCredentialDeleteConfirm": {
    message: "자격 증명 삭제",
    description:
      "Confirm button of the credential delete confirmation (w4/551)",
  },
  "databases.accessCredentialDefaultLocked": {
    message:
      "기본 자격 증명은 삭제할 수 없습니다. 먼저 새 기본 자격 증명을 추가하세요.",
    description: "Why the delete button is disabled on the default credential",
  },
  "databases.accessCredentialCreated": {
    message:
      "{name}이(가) 기본 자격 증명이 되었습니다. 비밀번호는 연결 정보에서 확인할 수 있습니다.",
    description: "Toast after creating a credential",
  },
  "databases.accessCredentialError": {
    message: "자격 증명을 생성하지 못했습니다. 다시 시도하세요.",
    description: "Toast when creating a credential fails",
  },
  "databases.accessCredentialDeleted": {
    message: "{name} 폐기를 요청했습니다.",
    description:
      "Toast after deleting a credential: the login is retired asynchronously on the next reconcile, so this confirms accepted intent, not a completed revocation",
  },
  "databases.accessCredentialDeleteError": {
    message: "{name} 자격 증명을 삭제하지 못했습니다. 다시 시도하세요.",
    description: "Toast when deleting a credential fails",
  },
  "databases.accessPooler": {
    message: "커넥션 풀링",
    description: "Access panel section heading (PgBouncer pooler strings)",
  },
  "databases.accessPoolerHint": {
    message:
      "연결 수가 많은 클라이언트를 위해 PgBouncer를 거치는 풀링된 연결 문자열입니다.",
    description: "Access panel pooler helper text",
  },
  "databases.accessPoolerReveal": {
    message: "풀링된 문자열 표시",
    description: "Access panel button to fetch the pooled connection strings",
  },
  "databases.accessPoolerDisabled": {
    message: "풀링된 연결 문자열을 받으려면 위에서 커넥션 풀링을 켜세요.",
    description:
      "Access panel shown when no pooler is enabled; points at the Connection Pool switch in the section heading (w2/m307)",
  },
  "databases.accessPoolerInternal": {
    message: "내부 풀링 연결",
    description: "Access panel pooled-string label (in-cluster)",
  },
  "databases.accessPoolerPlanUnsupported": {
    message:
      "이 요금제는 커넥션 풀링을 지원하지 않습니다. 풀러를 활성화하려면 지원하는 요금제를 선택하세요.",
    description: "Pooler empty state on an unsupported plan",
  },
  "databases.accessPoolerExternal": {
    message: "외부 풀링 연결",
    description: "Access panel pooled-string label (public)",
  },
  "databases.accessPoolerReplicaInternal": {
    message: "내부 풀링 연결, 읽기 복제본 {name}",
    description:
      "Access panel pooled-string label for a read replica's in-cluster pooled connection (w6/m458)",
  },
  "databases.accessPoolerReplicaExternal": {
    message: "외부 풀링 연결, 읽기 복제본 {name}",
    description:
      "Access panel pooled-string label for a read replica's public pooled connection (w6/m458)",
  },
  "databases.accessPoolerError": {
    message: "풀링된 연결 문자열을 불러오지 못했습니다.",
    description: "Toast when revealing pooled strings fails",
  },
  "databases.accessPoolerStarting": {
    message:
      "커넥션 풀링을 시작하는 중입니다. PgBouncer가 준비되면 풀링된 연결 문자열이 여기에 표시됩니다.",
    description:
      "Access panel when pooling is on but the operator has not published the pooler hosts yet (w2/m307)",
  },
  "databases.poolerToggleLabel": {
    message: "커넥션 풀",
    description:
      "Accessible label of the Connection Pool switch (Render's Info > Connection Pool, w2/m307)",
  },
  "databases.poolerEnableTitle": {
    message: "{name} 데이터베이스의 커넥션 풀링을 활성화할까요?",
    description:
      "Connection-pool enable confirmation title; {name} is the database",
  },
  "databases.poolerDisableTitle": {
    message: "{name} 데이터베이스의 커넥션 풀링을 비활성화할까요?",
    description:
      "Connection-pool disable confirmation title; {name} is the database",
  },
  "databases.poolerEnableBody": {
    message:
      "커넥션 풀링을 활성화하면 데이터베이스가 다시 시작되며 몇 분간 다운타임이 발생합니다. PgBouncer가 준비되면 풀링된 연결 문자열이 이 섹션에 표시됩니다.",
    description:
      "Connection-pool enable confirmation body for a database without high availability (Render: enabling restarts the database)",
  },
  "databases.poolerEnableBodyHA": {
    message:
      "커넥션 풀링을 활성화하면 데이터베이스를 다시 시작하는 대신 대기 인스턴스로 페일오버하므로 열려 있는 연결이 잠시 끊어집니다. PgBouncer가 준비되면 풀링된 연결 문자열이 이 섹션에 표시됩니다.",
    description:
      "Connection-pool enable confirmation body when high availability is on (Render: a failover instead of a restart)",
  },
  "databases.poolerDisableBody": {
    message:
      "커넥션 풀링을 비활성화하면 데이터베이스가 다시 시작되며 몇 분간 다운타임이 발생합니다. 풀링된 연결 문자열은 더 이상 작동하지 않습니다.",
    description:
      "Connection-pool disable confirmation body for a database without high availability",
  },
  "databases.poolerDisableBodyHA": {
    message:
      "커넥션 풀링을 비활성화하면 데이터베이스를 다시 시작하는 대신 대기 인스턴스로 페일오버하므로 열려 있는 연결이 잠시 끊어집니다. 풀링된 연결 문자열은 더 이상 작동하지 않습니다.",
    description:
      "Connection-pool disable confirmation body when high availability is on",
  },
  "databases.poolerEnableConfirm": {
    message: "커넥션 풀 활성화",
    description: "Connection-pool enable confirmation button (Render's label)",
  },
  "databases.poolerDisableConfirm": {
    message: "커넥션 풀 비활성화",
    description: "Connection-pool disable confirmation button",
  },
  "databases.poolerEnableSuccess": {
    message: "{name} 데이터베이스의 커넥션 풀링을 활성화하는 중입니다.",
    description: "Toast after the connection-pool enable was accepted",
  },
  "databases.poolerDisableSuccess": {
    message: "{name} 데이터베이스의 커넥션 풀링을 비활성화하는 중입니다.",
    description: "Toast after the connection-pool disable was accepted",
  },
  "databases.poolerEnableError": {
    message: "{name} 데이터베이스의 커넥션 풀링을 활성화하지 못했습니다.",
    description: "Fallback toast when enabling the connection pool fails",
  },
  "databases.poolerDisableError": {
    message: "{name} 데이터베이스의 커넥션 풀링을 비활성화하지 못했습니다.",
    description: "Fallback toast when disabling the connection pool fails",
  },
  "databases.haTitle": {
    message: "고가용성",
    description: "HA panel card title",
  },
  "databases.haDescription": {
    message:
      "자동 페일오버를 지원하는 복제 클러스터입니다. 활성화하면 대기 인스턴스가 항상 인계받을 준비를 하고 있습니다.",
    description: "HA panel card description",
  },
  "databases.haStatus": {
    message: "상태",
    description: "HA panel status row label",
  },
  "databases.haEnabled": {
    message: "활성화됨",
    description: "HA panel status value when HA is active",
  },
  "databases.haDisabled": {
    message: "비활성화됨",
    description: "HA panel status value when HA is off",
  },
  "databases.haFailover": {
    message: "페일오버 실행",
    description: "HA panel button to initiate a planned switchover",
  },
  "databases.haFailoverConfirmTitle": {
    message: "{name} 데이터베이스에서 페일오버를 실행할까요?",
    description: "Failover-confirmation dialog title",
  },
  "databases.haFailoverConfirmBody": {
    message:
      "대기 인스턴스가 프라이머리로 승격됩니다. 전환하는 동안 연결이 잠시 끊어집니다. 데이터베이스는 계속 사용할 수 있으며 데이터 손실은 없습니다.",
    description: "Failover-confirmation dialog body",
  },
  "databases.haFailoverCancel": {
    message: "취소",
    description: "Failover-confirmation dialog cancel button",
  },
  "databases.haFailoverConfirm": {
    message: "페일오버 실행",
    description: "Failover-confirmation dialog confirm button",
  },
  "databases.haFailoverSuccess": {
    message: "{name} 데이터베이스에서 페일오버를 실행했습니다.",
    description: "Toast after a failover request is accepted",
  },
  "databases.haFailoverError": {
    message:
      "{name} 데이터베이스에서 페일오버를 실행하지 못했습니다. 다시 시도하세요.",
    description: "Toast when a failover request fails",
  },
  "databases.haReadReplicas": {
    message: "읽기 복제본",
    description: "HA panel section heading for named read replicas",
  },
  "databases.haReadReplicasEmpty": {
    message: "이름이 지정된 읽기 복제본이 없습니다.",
    description: "HA panel empty state for read replicas",
  },
  "databases.haReplicaInternal": {
    message: "내부",
    description: "HA panel replica connection label (in-cluster host)",
  },
  "databases.haReplicaExternal": {
    message: "외부",
    description: "HA panel replica connection label (public SNI host)",
  },
  "databases.haAddReplica": {
    message: "읽기 복제본 추가",
    description:
      "HA panel button that opens the add-read-replica dialog (w5/m138)",
  },
  "databases.haAddReplicaTitle": {
    message: "읽기 복제본 추가",
    description: "Add-read-replica dialog title",
  },
  "databases.haAddReplicaBody": {
    message:
      "읽기 복제본은 프라이머리에서 데이터를 스트리밍받아 자체 주소로 읽기 전용 쿼리를 처리하는, 별도로 과금되는 인스턴스입니다.",
    description: "Add-read-replica dialog description",
  },
  "databases.haReplicaNameLabel": {
    message: "이름",
    description: "Add-read-replica dialog name field label",
  },
  "databases.haReplicaNameHint": {
    message:
      "소문자, 숫자, 하이픈만 사용해 최대 {max}자까지 입력할 수 있습니다. 복제본 호스트 이름의 일부가 됩니다.",
    description: "Add-read-replica name hint; max is the API's length limit",
  },
  "databases.haReplicaNameInvalid": {
    message:
      "소문자, 숫자, 하이픈을 사용해 1~{max}자로 입력하고, 문자 또는 숫자로 시작하고 끝나야 합니다.",
    description:
      "Add-read-replica name error; mirrors the API's POSTGRES_READ_REPLICA_NAME_INVALID",
  },
  "databases.haReplicaNameDuplicate": {
    message:
      "이 데이터베이스에는 이미 {name}이라는 이름의 읽기 복제본이 있습니다.",
    description: "Add-read-replica name error for a name already in use",
  },
  "databases.haAddReplicaSubmit": {
    message: "복제본 추가",
    description: "Add-read-replica dialog submit button",
  },
  "databases.haAddReplicaSuccess": {
    message: "읽기 복제본 {name}을 생성하는 중입니다.",
    description: "Toast after a read replica is added",
  },
  "databases.haAddReplicaError": {
    message: "읽기 복제본 {name}을 추가하지 못했습니다.",
    description: "Toast when adding a read replica fails",
  },
  "databases.haRemoveReplica": {
    message: "제거",
    description: "HA panel per-replica remove button",
  },
  "databases.haRemoveReplicaTitle": {
    message: "읽기 복제본 {name}을 제거할까요?",
    description: "Remove-read-replica confirmation title",
  },
  "databases.haRemoveReplicaBody": {
    message:
      "복제본 인스턴스가 삭제되고 연결 문자열이 더 이상 작동하지 않습니다. 프라이머리는 영향을 받지 않습니다.",
    description: "Remove-read-replica confirmation body",
  },
  "databases.haRemoveReplicaConfirm": {
    message: "복제본 제거",
    description: "Remove-read-replica confirm button",
  },
  "databases.haRemoveReplicaSuccess": {
    message: "읽기 복제본 {name}을 제거하는 중입니다.",
    description: "Toast after a read replica is removed",
  },
  "databases.haRemoveReplicaError": {
    message: "읽기 복제본 {name}을 제거하지 못했습니다.",
    description: "Toast when removing a read replica fails",
  },
  "databases.haReplicaPlanUnsupported": {
    message: "이 데이터베이스의 요금제에서는 읽기 복제본을 사용할 수 없습니다.",
    description: "Why Add read replica is disabled: the plan allows none",
  },
  "databases.haReplicaLimitReached": {
    message: "이 데이터베이스는 읽기 복제본 최대 개수({max}개)에 도달했습니다.",
    description:
      "Why Add read replica is disabled: the plan's replica limit is reached",
  },
  "databases.haReplicaStorageTooSmall": {
    message:
      "읽기 복제본에는 스토리지가 최소 {min} GB 필요하지만 이 데이터베이스는 {current} GB입니다.",
    description:
      "Why Add read replica is disabled: storage is below the replica minimum",
  },
  "databases.haNotEnabled": {
    message: "이 데이터베이스는 고가용성이 활성화되어 있지 않습니다.",
    description: "HA panel line when HA is off on a plan that offers it",
  },
  "databases.haPlanUnsupported": {
    message:
      "고가용성에는 CPU가 1개 이상인 요금제가 필요합니다. 이 데이터베이스의 요금제는 고가용성을 제공하지 않습니다.",
    description:
      "HA panel state when HA is off and the current plan is below 1 CPU (w8/m43, Render's rule)",
  },
  "databases.haToggleLabel": {
    message: "고가용성",
    description: "Accessible name of the HA panel's on/off switch (w2/m171)",
  },
  "databases.haStarting": {
    message:
      "다른 노드에서 대기 인스턴스를 시작하는 중입니다. 준비되면 페일오버를 사용할 수 있습니다.",
    description:
      "HA panel line while HA is on but the standby is not ready yet",
  },
  "databases.haStopping": {
    message: "고가용성을 끄는 중입니다. 대기 인스턴스를 제거하고 있습니다.",
    description: "HA panel line while HA is off but the standby still runs",
  },
  "databases.haEnableTitle": {
    message: "{name} 데이터베이스의 고가용성을 활성화할까요?",
    description: "HA enable confirmation title",
  },
  "databases.haEnableBody": {
    message:
      "다른 노드에서 대기 인스턴스가 시작되며, 프라이머리에 장애가 발생하면 자동으로 인계받습니다. 프라이머리와 동일하게 과금됩니다.",
    description: "HA enable confirmation body",
  },
  "databases.haEnablePrice": {
    message: "대기 인스턴스로 {price}이(가) 추가됩니다.",
    description:
      "HA enable confirmation price note; {price} is a formatted monthly price",
  },
  "databases.haEnableConfirm": {
    message: "고가용성 활성화",
    description: "HA enable confirmation button",
  },
  "databases.haDisableTitle": {
    message: "{name} 데이터베이스의 고가용성을 비활성화할까요?",
    description: "HA disable confirmation title",
  },
  "databases.haDisableBody": {
    message:
      "대기 인스턴스가 제거됩니다. 프라이머리에 장애가 발생하면 복구될 때까지 데이터베이스를 사용할 수 없습니다.",
    description: "HA disable confirmation body",
  },
  "databases.haDisableConfirm": {
    message: "고가용성 비활성화",
    description: "HA disable confirmation button",
  },
  "databases.haEnableSuccess": {
    message: "{name} 데이터베이스의 고가용성을 활성화하는 중입니다.",
    description: "Toast after HA was enabled",
  },
  "databases.haDisableSuccess": {
    message: "{name} 데이터베이스의 고가용성을 비활성화하는 중입니다.",
    description: "Toast after HA was disabled",
  },
  "databases.haEnableError": {
    message: "{name} 데이터베이스의 고가용성을 활성화하지 못했습니다.",
    description: "Fallback toast when enabling HA fails",
  },
  "databases.haDisableError": {
    message: "{name} 데이터베이스의 고가용성을 비활성화하지 못했습니다.",
    description: "Fallback toast when disabling HA fails",
  },
  "databases.planPickerHAUnsupported": {
    message:
      "{name} 요금제는 고가용성을 지원하지 않습니다(CPU가 1개 이상 필요). 먼저 고가용성을 비활성화하세요.",
    description:
      "Plan picker refusal when an HA database selects a plan below 1 CPU; mirrors the API message (w8/m43)",
  },
  "databases.planPickerStorageUnsupported": {
    message:
      "{name} 요금제는 스토리지를 최대 {max} GB까지 허용합니다. 이 데이터베이스는 {current} GB이며 줄일 수 없습니다. 스토리지가 충분한 요금제를 선택하세요.",
    description: "Plan change blocked by the database's existing storage",
  },
  "databases.planPickerReplicasUnsupported": {
    message:
      "{name}의 읽기 복제본 한도: {max}개. 이 데이터베이스에는 {current}개가 있습니다.",
    description: "Plan change blocked by the database's read replicas",
  },
  "databases.planPickerAutoscalingUnsupported": {
    message:
      "{name} 요금제는 디스크 자동 확장을 지원하지 않습니다. 먼저 디스크 자동 확장을 끄세요.",
    description: "Plan change blocked while disk autoscaling is enabled",
  },
  "databases.planPickerPoolerUnsupported": {
    message:
      "{name} 요금제는 커넥션 풀링을 지원하지 않습니다. 먼저 풀러를 비활성화하세요.",
    description: "Plan change blocked while a connection pooler is enabled",
  },
  "databases.planTitle": {
    message: "컴퓨팅 플랜",
    description: "Database detail plan-picker card title",
  },
  "databases.planDescription": {
    message:
      "컴퓨팅 플랜을 변경합니다. 대기 인스턴스가 없는 데이터베이스는 적용을 위해 다시 시작되며 1~2분간 사용할 수 없습니다. 데이터는 보존됩니다.",
    description: "Database detail plan-picker card description",
  },
  "databases.planPickerSave": {
    message: "저장",
    description: "Database plan-picker save button",
  },
  "databases.planPickerCancel": {
    message: "취소",
    description: "Database plan-picker cancel / reset button",
  },
  "databases.planPickerConfirmTitle": {
    message: "{name}(으)로 변경할까요?",
    description: "Database plan-picker confirm dialog title",
  },
  "databases.planPickerConfirmBody": {
    message:
      "새 컴퓨팅 플랜을 적용하기 위해 데이터베이스가 다시 시작되며 1~2분간 사용할 수 없습니다. 연결이 끊어집니다. 데이터는 보존됩니다.",
    description: "Database plan-picker confirm dialog body",
  },
  "databases.planPickerConfirmPrice": {
    message: "새 요금제는 월 ${price}입니다.",
    description:
      "Plan-change confirm: the new plan's monthly price, from the API's price sheet (w4/156)",
  },
  "databases.planPickerSuccess": {
    message: "{name}(으)로 요금제를 업데이트하는 중…",
    description: "Toast after a plan update is accepted",
  },
  "databases.planPickerError": {
    message: "요금제를 업데이트하지 못했습니다. 다시 시도하세요.",
    description: "Toast when a plan update fails",
  },
  "databases.storageTitle": {
    message: "스토리지",
    description:
      "Plan section Storage row heading (Render's Plan page Storage section, w2/m307)",
  },
  "databases.storageSize": {
    message: "{size} GB",
    description: "Plan section Storage row: the allocated storage",
  },
  "databases.storageFixed": {
    message:
      "무료 데이터베이스는 스토리지가 고정되어 있습니다. 스토리지를 늘리려면 유료 요금제로 업그레이드하세요.",
    description:
      "Why the Storage Edit button is disabled on a free database (bex-api POSTGRES_STORAGE_PLAN_UNSUPPORTED)",
  },
  "databases.storageEdit": {
    message: "편집",
    description:
      "Plan section Storage row button that opens the storage dialog",
  },
  "databases.storageEditAria": {
    message: "스토리지 편집",
    description: "Accessible name of the Storage row's Edit button",
  },
  "databases.storageEditTitle": {
    message: "{name} 스토리지 편집",
    description: "Storage dialog title; {name} is the database",
  },
  "databases.storageEditBody": {
    message:
      "현재 {current} GB보다 큰 {step} GB의 배수를 입력하세요. 한 번 늘린 스토리지는 줄일 수 없습니다.",
    description:
      "Storage dialog description (Render: storage only grows, in 5 GB steps)",
  },
  "databases.storageEditLabel": {
    message: "스토리지 (GB)",
    description: "Storage dialog size field label",
  },
  "databases.storageEditStep": {
    message: "{step} GB의 정수 배수를 입력하세요.",
    description: "Storage dialog validation: not a whole multiple of 5 GB",
  },
  "databases.storageEditNotGreater": {
    message:
      "현재 {current} GB보다 큰 값을 입력하세요. 스토리지는 줄일 수 없습니다.",
    description:
      "Storage dialog validation: the size is not above the current storage",
  },
  "databases.storageEditTooLarge": {
    message: "{name} 요금제는 스토리지를 최대 {max} GB까지 허용합니다.",
    description:
      "Storage dialog validation: the size is above the plan's maximum; {name} is the plan",
  },
  "databases.storageEditSave": {
    message: "저장",
    description: "Storage dialog submit button",
  },
  "databases.storageEditSuccess": {
    message: "스토리지를 {size} GB로 늘리는 중…",
    description: "Toast after a storage increase is accepted",
  },
  "databases.storageEditError": {
    message: "스토리지를 업데이트하지 못했습니다. 다시 시도하세요.",
    description:
      "Storage dialog fallback error when the server gives no reason",
  },
  "databases.nameEdit": {
    message: "데이터베이스 이름 편집",
    description: "Accessible label for the database-name edit-in-place pencil",
  },
  "databases.nameDescription": {
    message:
      "표시 이름을 변경합니다. 데이터베이스 ID와 모든 연결 정보는 그대로 유지됩니다.",
    description: "Database detail rename row description",
  },
  "databases.nameSuccess": {
    message: "데이터베이스 이름을 {name}(으)로 변경했습니다.",
    description: "Toast after a database rename succeeds",
  },
  "databases.nameError": {
    message: "데이터베이스 이름을 변경하지 못했습니다. 다시 시도하세요.",
    description: "Toast when a database rename fails unexpectedly",
  },
  "databases.nameConflict": {
    message: "이 워크스페이스에 같은 이름의 데이터베이스가 이미 있습니다.",
    description: "Toast when a database rename collides with another name",
  },
  "databases.nameInvalid": {
    message: "줄 바꿈이나 제어 문자 없이 100자 이하의 이름을 입력하세요.",
    description: "Database rename validation message",
  },
  "databases.insightsTitle": {
    message: "인사이트",
    description: "Database insights panel card title",
  },
  "databases.insightsDescription": {
    message: "프로세스, 쿼리 성능, 스토리지, 구성을 실시간으로 살펴봅니다.",
    description: "Database insights panel card description",
  },
  "databases.insightsRefresh": {
    message: "인사이트 새로 고침",
    description: "Accessible label for the insights refresh button",
  },
  "databases.insightsUnavailable": {
    message:
      "사용할 수 없음 — 데이터베이스가 아직 프로비저닝 중일 수 있습니다.",
    description: "Insights sub-section error state",
  },
  "databases.insightsSizeTitle": {
    message: "스토리지",
    description: "Insights panel — database/table sizes sub-section heading",
  },
  "databases.insightsNoSizes": {
    message: "아직 크기 데이터가 없습니다.",
    description: "Insights storage empty state",
  },
  "databases.insightsProcessesTitle": {
    message: "활성 프로세스",
    description: "Insights panel — pg_stat_activity sub-section heading",
  },
  "databases.insightsNoProcesses": {
    message: "활성 프로세스가 없습니다.",
    description: "Insights processes empty state",
  },
  "databases.insightsTopQueriesTitle": {
    message: "상위 쿼리",
    description: "Insights panel — pg_stat_statements sub-section heading",
  },
  "databases.insightsNoTopQueries": {
    message:
      "아직 쿼리가 없습니다. 애플리케이션과 SQL 콘솔에서 실행한 구문이 여기에 표시됩니다.",
    description:
      "Insights top-queries empty state: the read succeeded and nothing the owner ran is in the list. A failed read, including a missing pg_stat_statements, shows insightsUnavailable instead, so this text must not blame the extension",
  },
  "databases.insightsQueryHidden": {
    message: "숨김",
    description:
      "Insights — query cell for a row whose SQL PostgreSQL hides from the reading role",
  },
  "databases.insightsMaskedNote": {
    message:
      "일부 행은 다른 데이터베이스 역할의 것이어서 PostgreSQL이 이 보기에서 SQL을 숨깁니다. 행의 나머지 정보는 실제 값입니다.",
    description:
      "Insights — explanation shown when at least one process or top-query row has masked SQL",
  },
  "databases.insightsTableScansTitle": {
    message: "테이블 스캔",
    description: "Insights panel — pg_stat_user_tables sub-section heading",
  },
  "databases.insightsNoTableScans": {
    message: "아직 스캔 통계가 있는 사용자 테이블이 없습니다.",
    description: "Insights table-scans empty state",
  },
  "databases.insightsParamsDeclaredValue": {
    message: "선언된 값",
    description: "Declared PostgreSQL parameter observation diagnostic.",
  },
  "databases.insightsParamsObservedValue": {
    message: "관찰된 설정",
    description: "Declared PostgreSQL parameter observation diagnostic.",
  },
  "databases.insightsParamsObservedHint": {
    message:
      "실시간 설정이며, 요청한 값이 적용되었음을 확인해 주지는 않습니다.",
    description: "Declared PostgreSQL parameter observation diagnostic.",
  },
  "databases.insightsParamsNotObserved": {
    message:
      "요청한 재정의가 관찰되지 않았습니다. 보류 중이거나 잘못된 값일 수 있습니다.",
    description: "Declared PostgreSQL parameter observation diagnostic.",
  },
  "databases.insightsParamsObservationUnavailable": {
    message: "지금은 이 재정의를 확인할 수 없습니다.",
    description: "Declared PostgreSQL parameter observation diagnostic.",
  },
  "databases.insightsParamsTitle": {
    message: "매개변수 재정의",
    description:
      "Insights panel — heading for the parameters this database declares (read-only)",
  },
  "databases.insightsParamsDescription": {
    message:
      "매개변수 재정의가 폐지되기 전에 이 데이터베이스에 선언된 설정입니다. 계속 적용되지만 여기에서는 더 이상 변경할 수 없습니다. 세션 기본값은 SQL 콘솔에서 ALTER DATABASE … SET으로 설정하세요.",
    description:
      "Explains that the declared parameters are read-only legacy settings that still apply, and how to set session defaults instead",
  },
  "databases.insightsEffectiveParamsTitle": {
    message: "기본값이 아닌 매개변수",
    description: "Insights panel — pg_settings sub-section heading",
  },
  "databases.insightsEffectiveParamsDescription": {
    message:
      "플랫폼이 관리하는 항목을 포함해 현재 적용 중인 설정 중 기본값이 아닌 모든 항목입니다. 읽기 전용입니다.",
    description:
      "Explains that the pg_settings table is observed configuration, mostly platform-owned, and cannot be edited",
  },
  "databases.insightsNoParams": {
    message: "모든 매개변수가 기본값입니다.",
    description: "Insights parameter-overrides empty state",
  },
  "databases.insightsColTable": {
    message: "테이블",
    description: "Insights table column header (table name)",
  },
  "databases.insightsColRelation": {
    message: "테이블 또는 인덱스",
    description:
      "Insights sizes column header (a table's or an index's name; tables and indexes are listed together)",
  },
  "databases.insightsIndexOn": {
    message: "{table}의 인덱스",
    description:
      "Insights sizes: suffix naming the table an index row belongs to; {table} is the quoted schema-qualified table name",
  },
  "databases.insightsColSize": {
    message: "크기",
    description: "Insights table column header (relation size)",
  },
  "databases.insightsColUser": {
    message: "사용자",
    description: "Insights processes column header (pg role)",
  },
  "databases.insightsColState": {
    message: "상태",
    description: "Insights processes column header (active/idle/…)",
  },
  "databases.insightsColDuration": {
    message: "소요 시간",
    description: "Insights processes column header (seconds since query start)",
  },
  "databases.insightsColQuery": {
    message: "쿼리",
    description: "Insights column header (SQL text, truncated)",
  },
  "databases.insightsColCalls": {
    message: "호출 수",
    description: "Insights top-queries column header (invocation count)",
  },
  "databases.insightsColTotalTime": {
    message: "총 시간",
    description: "Insights top-queries column header (total exec time)",
  },
  "databases.insightsColMeanTime": {
    message: "평균 시간",
    description: "Insights top-queries column header (mean exec time per call)",
  },
  "databases.insightsColSeqScans": {
    message: "순차 스캔",
    description: "Insights table-scans column header (sequential scan count)",
  },
  "databases.insightsColIdxScans": {
    message: "인덱스 스캔",
    description: "Insights table-scans column header (index scan count)",
  },
  "databases.insightsColLiveRows": {
    message: "라이브 행",
    description:
      "Insights table-scans column header (estimated live row count)",
  },
  "databases.insightsColParam": {
    message: "매개변수",
    description:
      "Insights parameter-overrides column header (pg_settings.name)",
  },
  "databases.insightsColSetting": {
    message: "설정값",
    description:
      "Insights parameter-overrides column header (pg_settings.setting)",
  },
  "databases.insightsColSource": {
    message: "출처",
    description:
      "Insights parameter-overrides column header (pg_settings.source)",
  },
  "databases.sqlTitle": {
    message: "SQL 콘솔",
    description: "Database detail SQL console card title",
  },
  "databases.sqlDescription": {
    message:
      "이 데이터베이스에 SQL 구문을 하나씩 실행합니다. 결과는 500행으로 제한되며 쿼리는 10초 후 시간 초과됩니다.",
    description: "Database SQL console safety summary",
  },
  "databases.sqlEditorLabel": {
    message: "SQL 쿼리",
    description: "Accessible label for the SQL editor",
  },
  "databases.sqlShortcut": {
    message: "Ctrl+Enter 또는 ⌘+Enter를 눌러 실행",
    description: "SQL console keyboard shortcut hint",
  },
  "databases.sqlCreating": {
    message:
      "이 데이터베이스는 아직 생성 중입니다. 사용 가능 상태가 되면 SQL을 실행할 수 있으며, 쿼리와 기록은 유지됩니다.",
    description: "SQL console notice while the database is creating (w4/177)",
  },
  "databases.sqlUpgrading": {
    message:
      "오프라인 PostgreSQL 업그레이드가 실행 중입니다. 데이터베이스가 사용 가능 상태가 되면 SQL을 다시 실행할 수 있으며, 쿼리와 기록은 유지됩니다.",
    description: "SQL console notice while the database is upgrading (w4/177)",
  },
  "databases.sqlRestarting": {
    message:
      "이 데이터베이스는 다시 시작 중입니다. 사용 가능 상태가 될 때까지 기다린 후 SQL을 실행하세요. 쿼리와 기록은 유지됩니다.",
    description: "SQL console notice while the database is restarting (w4/177)",
  },
  "databases.sqlSuspended": {
    message:
      "이 데이터베이스는 일시 중지되었습니다. SQL을 실행하려면 재개하세요. 쿼리와 기록은 유지됩니다.",
    description: "SQL console notice while the database is suspended",
  },
  "databases.sqlRun": {
    message: "쿼리 실행",
    description: "SQL console run button",
  },
  "databases.sqlRunning": {
    message: "실행 중…",
    description: "SQL console run button while a query is executing",
  },
  "databases.sqlUnknownError": {
    message: "쿼리를 실행하지 못했습니다. 다시 시도하세요.",
    description: "SQL console fallback error",
  },
  "databases.sqlReturnedRows_other": {
    message: "{count}개 행 반환됨",
    description: "SQL console SELECT result count (plural)",
  },
  "databases.sqlAffectedRows_other": {
    message: "{count}개 행 영향받음",
    description: "SQL console write result count (plural)",
  },
  "databases.sqlTruncated": {
    message: "결과가 {limit}행으로 제한되었습니다",
    description: "SQL console backend row-cap notice",
  },
  "databases.sqlNull": {
    message: "NULL",
    description: "SQL console null cell value",
  },
  "databases.sqlNullLabel": {
    message: "SQL NULL (값 없음)",
    description:
      "SQL console accessible meaning of a null cell, distinct from the literal text NULL",
  },
  "databases.sqlNoRows": {
    message: "쿼리가 행을 반환하지 않았습니다.",
    description: "SQL console empty result message",
  },
  "databases.sqlPrevious": {
    message: "이전",
    description: "SQL result pagination previous button",
  },
  "databases.sqlNext": {
    message: "다음",
    description: "SQL result pagination next button",
  },
  "databases.sqlPage": {
    message: "{page} / {pages} 페이지",
    description: "SQL result pagination position",
  },
  "databases.sqlHistory": {
    message: "쿼리 기록",
    description: "SQL console session history heading",
  },
  "databases.sqlClearHistory": {
    message: "지우기",
    description: "SQL console clear-history button",
  },
  "databases.sqlNoHistory": {
    message: "이 브라우저 세션에서 실행한 쿼리가 여기에 표시됩니다.",
    description: "SQL console empty history message",
  },
  "databases.sqlConfirmTitle": {
    message: "데이터를 수정할 수 있는 구문을 실행할까요?",
    description: "SQL console write confirmation title",
  },
  "databases.sqlConfirmDescription": {
    message:
      "이 구문은 읽기 전용으로 인식되지 않습니다. 데이터베이스 데이터를 변경하거나 삭제할 수 있으며 bex에서 되돌릴 수 없습니다.",
    description: "SQL console write confirmation warning",
  },
  "databases.sqlConfirmCancel": {
    message: "취소",
    description: "SQL console write confirmation cancel button",
  },
  "databases.sqlConfirmRun": {
    message: "구문 실행",
    description: "SQL console confirmed write action",
  },
  "databases.detailNavLabel": {
    message: "데이터베이스 세부 정보",
    description: "Accessible label for the managed Postgres detail tabs",
  },
  "databases.sectionNavigation": {
    message: "데이터베이스 섹션",
    description:
      "Accessible label for the database detail page section navigation",
  },
  "databases.dangerZoneTitle": {
    message: "위험 영역",
    description:
      "Database detail section-nav label for the destructive actions",
  },
  "databases.overviewTab": {
    message: "개요",
    description: "Managed Postgres overview tab",
  },
  "databases.logsTab": {
    message: "로그",
    description: "Managed Postgres logs tab",
  },
  "databases.logsRangeLabel": {
    message: "시간 범위",
    description: "Managed Postgres log range filter label",
  },
  "databases.logsInstanceLabel": {
    message: "데이터베이스 인스턴스",
    description: "Managed Postgres log instance filter label",
  },
  "databases.logsAllInstances": {
    message: "모든 인스턴스",
    description: "Managed Postgres log instance filter all option",
  },
  "databases.logsSearchPlaceholder": {
    message: "데이터베이스 로그 검색",
    description: "Managed Postgres log search placeholder",
  },
  "databases.logsLoading": {
    message: "데이터베이스 로그를 불러오는 중…",
    description: "Managed Postgres logs loading state",
  },
  "databases.logsEmptyTitle": {
    message: "아직 데이터베이스 로그가 없습니다",
    description: "Managed Postgres logs empty-state title",
  },
  "databases.logsEmptyBody": {
    message: "이 시간 범위의 Postgres 출력이 여기에 표시됩니다.",
    description: "Managed Postgres logs empty-state body",
  },
  "databases.logsUnavailableTitle": {
    message: "데이터베이스 로그가 구성되지 않았습니다",
    description: "Managed Postgres logs unavailable-state title",
  },
  "databases.logsUnavailableBody": {
    message:
      "이 설치에는 영구 로그 기록도, 실시간 CNPG 파드 로그 소스도 구성되어 있지 않습니다.",
    description: "Managed Postgres logs unavailable-state body",
  },
  "databases.logsUnauthorizedTitle": {
    message: "이 로그를 볼 수 없습니다",
    description: "Managed Postgres logs unauthorized-state title",
  },
  "databases.logsUnauthorizedBody": {
    message:
      "데이터베이스 로그를 보려면 이 워크스페이스에서 기여자 권한이 필요합니다.",
    description: "Managed Postgres logs unauthorized-state body",
  },
  "databases.logsErrorTitle": {
    message: "데이터베이스 로그를 불러오지 못했습니다",
    description: "Managed Postgres logs generic error title",
  },
};

export default koDatabases;
