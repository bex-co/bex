import type { TranslationEntry } from "@/i18n";

const koAgentEnvironment: Record<string, TranslationEntry> = {
  "agentEnvironment.title": {
    message: "에이전트",
    description: "Settings → Agents card title and section-nav label",
  },
  "agentEnvironment.description": {
    message:
      "이 워크스페이스의 에이전트 세션이 네트워크에 접근하는 방식과 사용하는 모델을 설정합니다. 모든 새 세션에 적용됩니다.",
    description: "Agents settings card description",
  },
  "agentEnvironment.loadFailed": {
    message: "에이전트 설정을 불러오지 못했습니다.",
    description: "Agents card error when the environment query fails",
  },
  "agentEnvironment.retry": {
    message: "다시 시도",
    description: "Retry button on the Agents card load error",
  },
  "agentEnvironment.readOnlyNote": {
    message: "워크스페이스 관리자만 에이전트 설정을 변경할 수 있습니다.",
    description: "Read-only notice shown to non-admin members",
  },
  "agentEnvironment.networkTitle": {
    message: "네트워크 접근",
    description: "Heading (and radiogroup label) of the network tier picker",
  },
  "agentEnvironment.networkDescription": {
    message:
      "GitHub와 모델 제공업체에는 항상 접근할 수 있습니다. 에이전트 샌드박스가 그 외에 접근할 수 있는 대상을 선택하세요.",
    description: "Network tier picker description",
  },
  "agentEnvironment.tierRegistriesTitle": {
    message: "패키지 레지스트리 (권장)",
    description: "Network tier option: built-in package registries",
  },
  "agentEnvironment.tierRegistriesDescription": {
    message:
      "에이전트가 npm, PyPI, Go, Maven, crates, RubyGems, Docker Hub에서 의존성을 설치할 수 있어 코드를 빌드하고 테스트할 수 있습니다.",
    description: "Description of the registries network tier",
  },
  "agentEnvironment.tierCustomTitle": {
    message: "사용자 지정",
    description: "Network tier option: registries plus listed hosts",
  },
  "agentEnvironment.tierCustomDescription": {
    message:
      "패키지 레지스트리와 아래에 입력한 호스트 이름에 접근할 수 있습니다.",
    description: "Description of the custom network tier",
  },
  "agentEnvironment.tierFullTitle": {
    message: "전체 인터넷",
    description: "Network tier option: any public host over HTTP(S)",
  },
  "agentEnvironment.tierFullDescription": {
    message:
      "bex 이그레스 프록시를 통해 HTTP(S)로 모든 공개 웹사이트에 접근할 수 있습니다. 모델 제공업체 API에 대한 직접 호출은 계속 차단되며(에이전트 자체 모델은 정상 작동), SSH와 그 외 비 HTTP 프로토콜은 사용할 수 없습니다.",
    description:
      "Description of the full-internet tier: HTTP(S) via the egress proxy, direct model-provider API calls blocked, no SSH/non-HTTP",
  },
  "agentEnvironment.registryHostsSummary_other": {
    message: "포함된 패키지 레지스트리 (호스트 {count}개)",
    description:
      "Disclosure summary listing the built-in registry hosts (many)",
  },
  "agentEnvironment.hostsLabel": {
    message: "추가 호스트 이름",
    description: "Label of the custom-tier hostname textarea",
  },
  "agentEnvironment.hostsHelp": {
    message:
      "한 줄에 호스트 이름 하나, 최대 {max}개. 정확한 이름만 입력하세요. 스킴, 포트, 경로, 와일드카드는 사용할 수 없습니다.",
    description: "Help under the custom hostname textarea",
  },
  "agentEnvironment.hostsError.too_many_entries": {
    message: "호스트 이름은 최대 {max}개까지 입력할 수 있습니다.",
    description: "Custom hosts error: too many entries",
  },
  "agentEnvironment.hostsError.empty": {
    message: "호스트 이름은 비워 둘 수 없습니다.",
    description: "Custom hosts error: empty entry",
  },
  "agentEnvironment.hostsError.whitespace": {
    message: '"{entry}" 앞뒤에 공백이 있습니다.',
    description: "Custom hosts error: surrounding whitespace",
  },
  "agentEnvironment.hostsError.not_a_hostname": {
    message:
      '"{entry}" 항목은 정확한 호스트 이름이 아닙니다. 스킴, 포트, 경로, 와일드카드 없이 api.example.com 같은 이름을 사용하세요.',
    description: "Custom hosts error: not a hostname",
  },
  "agentEnvironment.hostsError.ip_literal": {
    message: '"{entry}" 항목은 IP 주소입니다. 대신 호스트 이름을 사용하세요.',
    description: "Custom hosts error: IP literal",
  },
  "agentEnvironment.hostsError.private_name": {
    message:
      '"{entry}" 항목은 프라이빗 또는 클러스터 로컬 이름이므로 허용할 수 없습니다.',
    description: "Custom hosts error: private name",
  },
  "agentEnvironment.hostsError.already_baseline": {
    message:
      '"{entry}" 항목은 이미 접근할 수 있습니다(또는 모델 프록시를 통해서만 접근 가능). 목록에서 제거하세요.',
    description: "Custom hosts error: already in the baseline",
  },
  "agentEnvironment.hostsError.duplicate": {
    message: '"{entry}" 항목이 두 번 이상 나열되어 있습니다.',
    description: "Custom hosts error: duplicate",
  },
  "agentEnvironment.hostsError.invalid": {
    message: "호스트 이름 목록이 올바르지 않습니다.",
    description: "Custom hosts error: unknown reason",
  },
  "agentEnvironment.modelLabel": {
    message: "기본 모델",
    description: "Label of the default model field",
  },
  "agentEnvironment.modelPlaceholder": {
    message: "제공업체 기본값",
    description: "Placeholder of the empty default model field",
  },
  "agentEnvironment.modelHelp": {
    message:
      "새 세션이 사용할 모델 ID입니다(예: claude-sonnet-4-5). 비워 두면 제공업체 기본값을 사용합니다.",
    description: "Help under the default model field",
  },
  "agentEnvironment.modelPlatformDefault": {
    message: "플랫폼 기본값 ({model})",
    description:
      "Default model picker — no default: the platform catalog's first model (w10/m516)",
  },
  "agentEnvironment.modelCatalogHelp": {
    message:
      "모델을 지정하지 않은 새 세션과 자동화는 이 모델을 사용합니다. 멤버는 세션마다 플랫폼 카탈로그의 다른 모델을 선택할 수 있습니다.",
    description: "Help under the platform model catalog picker (w10/m516)",
  },
  "agentEnvironment.modelInvalid": {
    message: "공백이 없는 모델 ID를 사용하세요(최대 255자).",
    description: "Default model validation error",
  },
  "agentEnvironment.modelNotOfferedError": {
    message:
      "플랫폼 모델은 {model}을(를) 제공하지 않습니다. 목록에서 선택하세요.",
    description:
      "Server refusal: the default model is not in the platform catalog (w10/m516)",
  },
  "agentEnvironment.templateLabel": {
    message: "기본 샌드박스",
    description: "Label of the default sandbox template picker (w10/m519)",
  },
  "agentEnvironment.templateHelp": {
    message:
      "샌드박스를 지정하지 않은 새 세션과 자동화는 이 샌드박스에서 실행됩니다. 이후에 시작하는 세션에 적용되며, 멤버는 세션마다 다른 샌드박스를 고를 수도 있습니다.",
    description: "Help under the default sandbox template picker (w10/m519)",
  },
  "agentEnvironment.templateInvalidError": {
    message:
      "{template}은(는) 샌드박스 템플릿이 아닙니다. 다음 중 하나를 고르세요: {templates}",
    description:
      "Server refusal: the default sandbox template is not a registered one; {templates} lists those (w10/m519)",
  },
  "agentEnvironment.tierInvalid": {
    message: "지원하지 않는 네트워크 접근 옵션입니다.",
    description: "Server refusal: invalid tier",
  },
  "agentEnvironment.forbidden": {
    message: "워크스페이스 관리자만 에이전트 설정을 변경할 수 있습니다.",
    description: "Server refusal: caller is not an admin",
  },
  "agentEnvironment.saveFailed": {
    message: "에이전트 설정을 저장하지 못했습니다. 다시 시도하세요.",
    description: "Generic save failure",
  },
  "agentEnvironment.saveFailedReason": {
    message: "에이전트 설정을 저장하지 못했습니다: {reason}",
    description: "Save failure with the server's reason",
  },
  "agentEnvironment.save": {
    message: "저장",
    description: "Agents card save button",
  },
  "agentEnvironment.saved": {
    message: "에이전트 설정을 저장했습니다",
    description: "Success toast after saving",
  },
  "agentEnvironment.modelKeyTitle": {
    message: "모델 제공업체",
    description: "Heading of the model source / provider key section",
  },
  "agentEnvironment.modelKeyDescription": {
    message:
      "워크스페이스에서 자체 제공업체 키를 추가하지 않으면 에이전트 세션은 bex가 제공하는 모델을 사용합니다. 자체 키를 사용하면 세션이 제공업체를 직접 호출하며 bex는 샌드박스 컴퓨팅 비용만 청구합니다.",
    description: "Explains the model source choice (ADR091 D7)",
  },
  "agentEnvironment.modelSourceLabel": {
    message: "모델 소스:",
    description: "Label before the current model source",
  },
  "agentEnvironment.modelSourcePlatform": {
    message: "bex 제공 (모델 제공업체 비용 + 30%로 청구)",
    description: "Model source: the platform model",
  },
  "agentEnvironment.modelSourceByo": {
    message: "자체 키 (bex의 토큰 요금 없음)",
    description: "Model source: the workspace's own key",
  },
  "agentEnvironment.modelSourceNone": {
    message: "없음: 에이전트 세션을 시작하려면 제공업체 키를 추가하세요",
    description: "Model source: no key available",
  },
  "agentEnvironment.modelKeyLabel": {
    message: "제공업체 API 키",
    description: "Label of the write-only key field when none is set",
  },
  "agentEnvironment.modelKeyReplaceLabel": {
    message: "제공업체 API 키 교체",
    description: "Label of the write-only key field when a key is set",
  },
  "agentEnvironment.modelKeyPlaceholder": {
    message: "sk-ant-…",
    description: "Placeholder of the key field when none is set",
  },
  "agentEnvironment.modelKeyPlaceholderSet": {
    message: "키가 저장되어 있습니다. 교체하려면 새 키를 입력하세요",
    description:
      "Placeholder of the key field when a key is set (the key is never shown)",
  },
  "agentEnvironment.modelKeyHelp": {
    message: "쓰기 전용: 키는 안전하게 저장되며 다시 표시되지 않습니다.",
    description: "Help under the key field",
  },
  "agentEnvironment.modelKeySave": {
    message: "키 저장",
    description: "Button that saves the model key",
  },
  "agentEnvironment.modelKeyClear": {
    message: "키 제거",
    description: "Button that opens the remove-key confirmation",
  },
  "agentEnvironment.modelKeyClearTitle": {
    message: "모델 제공업체 키를 제거하시겠습니까?",
    description: "Remove-key confirmation title",
  },
  "agentEnvironment.modelKeyClearBody": {
    message:
      "에이전트 세션은 다음 모델 요청부터 이 키를 사용하지 않습니다. 키는 다시 표시할 수 없으므로 복원하려면 직접 붙여넣어야 합니다.",
    description: "Remove-key confirmation body",
  },
  "agentEnvironment.modelKeyClearConfirm": {
    message: "키 제거",
    description: "Remove-key confirmation button",
  },
  "agentEnvironment.modelKeySaved": {
    message: "모델 키를 저장했습니다",
    description: "Toast after saving the key",
  },
  "agentEnvironment.modelKeyCleared": {
    message: "모델 키를 제거했습니다",
    description: "Toast after removing the key",
  },
  "agentEnvironment.modelKeyInvalid": {
    message: "공백 없이 키를 입력하세요(최대 4,096자).",
    description: "Client/server refusal of a malformed key",
  },
  "agentEnvironment.modelKeySaveFailed": {
    message: "모델 키를 저장하지 못했습니다. 다시 시도하세요.",
    description: "Generic model key write failure",
  },
  "agentEnvironment.spendTitle": {
    message: "에이전트 모델 지출",
    description: "Heading of the agent-model spend section (ADR091 D6)",
  },
  "agentEnvironment.spendPricing": {
    message:
      "모델 제공업체 비용 + 30%로 청구됩니다. 한도에 도달하면 이번 달 남은 기간 동안 새 모델 요청이 중단됩니다.",
    description: "Pricing statement under the spend heading",
  },
  "agentEnvironment.spendSummary": {
    message: "이번 달 {cap} 중 {used} 사용",
    description: "Month-to-date spend against the effective monthly cap",
  },
  "agentEnvironment.spendSummaryCeiling": {
    message: "결제 크레딧 초과 지출 {cap} 중 {used}, 이번 달 총 {total}",
    description:
      "Spend summary and meter value when the trust tier's ceiling binds on the spend billing credits leave ({used}), which differs from month-to-date ({total})",
  },
  "agentEnvironment.spendMeterValue": {
    message: "{cap} 중 {used}",
    description: "Screen-reader value of the spend progress bar",
  },
  "agentEnvironment.spendUnavailable": {
    message: "현재 지출 정보를 확인할 수 없습니다.",
    description: "Shown when the spend fields could not be read",
  },
  "agentEnvironment.spendTierNextCharge": {
    message:
      "신뢰 등급 {tier}: 결제 크레딧이 에이전트 모델 지출을 먼저 충당하며, 남은 지출이 {amount}에 도달하면 카드로 청구됩니다.",
    description: "Trust tier with its mid-month charge point (ADR091 D10.1)",
  },
  "agentEnvironment.spendTierNextChargeCredit": {
    message:
      "신뢰 등급 {tier}: 이번 결제 기간의 에이전트 모델 지출 중 처음 {credit}은 결제 크레딧이 충당하며, 그 초과분이 {amount}에 도달하면 카드로 청구됩니다.",
    description:
      "Trust tier with its charge point when billing credits cover agent model spend first (ADR091 D10.2a). {credit} is the credit counted in this billing period, {amount} the charge point",
  },
  "agentEnvironment.spendTierMonthEnd": {
    message: "신뢰 등급 {tier}: 에이전트 모델 지출은 월말에 청구됩니다.",
    description: "Top trust tier, charged only at month end",
  },
  "agentEnvironment.spendTierNoCharge": {
    message:
      "신뢰 등급 {tier}: 에이전트 모델 지출은 월 중간에 카드로 청구되지 않습니다.",
    description:
      "Trust tier with no mid-cycle card charge: a payment hold, an unpaid invoice, the workspace's own key, or an unchargeable workspace (ADR091 D10.2a)",
  },
  "agentEnvironment.spendTierNew": {
    message:
      "새 워크스페이스는 결제 크레딧을 제외한 에이전트 모델 지출이 {amount}에 도달하면 청구됩니다. 도달하는 즉시 카드가 청구되고, 결제에 성공하면 세션이 계속 실행되며 한도는 요금제의 상한까지 올라갑니다.",
    description:
      "Explains T0: charged at its threshold, working while the charge settles (ADR091 D10.2a)",
  },
  "agentEnvironment.spendTierLocked": {
    message:
      "결제 실패 또는 분쟁으로 이 워크스페이스가 제한되었습니다. 결제에서 해결하면 제한이 해제됩니다.",
    description: "Explains a payment-locked trust tier",
  },
  "agentEnvironment.spendCapLabel": {
    message: "월 지출 한도 (USD)",
    description: "Label of the admin-editable cap field",
  },
  "agentEnvironment.spendCapSave": {
    message: "한도 저장",
    description: "Button that saves the cap",
  },
  "agentEnvironment.spendCapReset": {
    message: "요금제 기본값으로 재설정",
    description: "Button that returns the cap to the plan default",
  },
  "agentEnvironment.spendCapHelp": {
    message:
      "요금제 기본값 {plan}. 관리자는 최대 {max}까지 설정할 수 있습니다.",
    description: "Help under the cap field",
  },
  "agentEnvironment.spendCapInvalid": {
    message: "$0보다 크고 {max} 이하인 금액을 입력하세요.",
    description: "Client/server refusal of a cap value",
  },
  "agentEnvironment.spendCapSaved": {
    message: "지출 한도를 저장했습니다",
    description: "Toast after saving the cap",
  },
  "agentEnvironment.spendCapResetDone": {
    message: "지출 한도를 요금제 기본값으로 재설정했습니다",
    description: "Toast after resetting the cap",
  },
  "agentEnvironment.spendCapSaveFailed": {
    message: "지출 한도를 저장하지 못했습니다. 다시 시도하세요.",
    description: "Generic cap write failure",
  },
  "agentEnvironment.secrets.title": {
    message: "에이전트 시크릿",
    description: "Settings → Agent secrets card title (ADR098)",
  },
  "agentEnvironment.secrets.description": {
    message:
      "세션에 환경 변수로 제공할 수 있는 API 키와 토큰입니다. 값은 쓰기 전용이라 다시 읽을 수 없으며, 대화 기록에서는 가려집니다.",
    description: "Agent secrets card description",
  },
  "agentEnvironment.secrets.loadFailed": {
    message: "에이전트 시크릿을 불러오지 못했습니다.",
    description: "Agent secrets card error when the list query fails",
  },
  "agentEnvironment.secrets.empty": {
    message: "아직 에이전트 시크릿이 없습니다.",
    description: "Agent secrets card empty state",
  },
  "agentEnvironment.secrets.deleteLabel": {
    message: "{name} 삭제",
    description: "Accessible label of a secret's delete button",
  },
  "agentEnvironment.secrets.deleteTitle": {
    message: "{name} 시크릿을 삭제하시겠습니까?",
    description: "Delete-secret confirmation title",
  },
  "agentEnvironment.secrets.deleteBody": {
    message:
      "이 시크릿을 사용하는 세션은 다시 설정할 때까지 다음 턴에서 실패합니다.",
    description: "Delete-secret confirmation body",
  },
  "agentEnvironment.secrets.deleteConfirm": {
    message: "시크릿 삭제",
    description: "Delete-secret confirmation button",
  },
  "agentEnvironment.secrets.nameLabel": {
    message: "시크릿 이름",
    description: "Agent secrets card: name field label",
  },
  "agentEnvironment.secrets.namePlaceholder": {
    message: "STRIPE_API_KEY",
    description: "Agent secrets card: name field placeholder (an env var name)",
  },
  "agentEnvironment.secrets.valueLabel": {
    message: "시크릿 값",
    description: "Agent secrets card: value field label",
  },
  "agentEnvironment.secrets.valuePlaceholder": {
    message: "값",
    description: "Agent secrets card: value field placeholder",
  },
  "agentEnvironment.secrets.save": {
    message: "저장",
    description: "Agent secrets card: save (create or replace) button",
  },
  "agentEnvironment.secrets.help": {
    message:
      "기존 이름으로 저장하면 값이 교체됩니다. 세션은 다음 턴부터 변경 사항을 반영합니다.",
    description: "Agent secrets card help line for admins",
  },
  "agentEnvironment.secrets.readOnly": {
    message:
      "워크스페이스 관리자만 에이전트 시크릿을 추가, 변경, 삭제할 수 있습니다.",
    description: "Agent secrets card help line for non-admins",
  },
  "agentEnvironment.secrets.saved": {
    message: "{name} 시크릿을 저장했습니다",
    description: "Toast after a secret is saved",
  },
  "agentEnvironment.secrets.deleted": {
    message: "{name} 시크릿을 삭제했습니다",
    description: "Toast after a secret is deleted",
  },
  "agentEnvironment.secrets.nameInvalid": {
    message: "문자, 숫자, _만 사용하고 문자 또는 _로 시작하세요(최대 128자).",
    description: "Secret name validation error",
  },
  "agentEnvironment.secrets.nameReserved": {
    message:
      "bex가 직접 설정하는 환경 변수이므로 시크릿 이름으로 사용할 수 없습니다.",
    description: "Secret name reserved error",
  },
  "agentEnvironment.secrets.valueRequired": {
    message: "값을 입력하세요.",
    description: "Secret value required error",
  },
  "agentEnvironment.secrets.valueInvalid": {
    message: "값은 최대 32 KiB까지 가능합니다.",
    description: "Secret value too long (or contains a NUL byte)",
  },
  "agentEnvironment.secrets.limitExceeded": {
    message: "에이전트 시크릿 한도에 도달했습니다.",
    description: "Secret count or size limit error",
  },
  "agentEnvironment.secrets.notFound": {
    message:
      "이 시크릿이 더 이상 존재하지 않습니다. 다른 사람이 방금 삭제했을 수 있습니다.",
    description: "AGENT_SECRET_NOT_FOUND error (deleted concurrently)",
  },
  "agentEnvironment.secrets.unavailable": {
    message: "이 배포에서는 에이전트 시크릿을 사용할 수 없습니다.",
    description: "AGENT_SECRETS_UNAVAILABLE error",
  },
  "agentEnvironment.secrets.forbidden": {
    message: "워크스페이스 관리자만 에이전트 시크릿을 변경할 수 있습니다.",
    description: "Secret write refused for a non-admin",
  },
  "agentEnvironment.secrets.saveFailed": {
    message: "변경 사항을 저장하지 못했습니다. 다시 시도하세요.",
    description: "Generic secret write failure",
  },
  "agentEnvironment.secrets.valueKeepPlaceholder": {
    message: "현재 값 유지",
    description:
      "Agent secrets card: value placeholder when the name is an existing secret (empty keeps its value)",
  },
  "agentEnvironment.secrets.editLabel": {
    message: "{name} 편집",
    description:
      "Accessible label of a secret's edit button (loads it into the form to rotate or rescope)",
  },
  "agentEnvironment.secrets.scopeLabel": {
    message: "범위",
    description:
      "Agent secrets card: accessible label of the scope picker (ADR098 D9)",
  },
  "agentEnvironment.secrets.scopeWorkspace": {
    message: "모든 세션",
    description:
      "Scope picker option: every session in the workspace receives the secret",
  },
  "agentEnvironment.secrets.scopeRepo": {
    message: "저장소 하나",
    description:
      "Scope picker option: sessions on one repository receive the secret",
  },
  "agentEnvironment.secrets.scopeExplicit": {
    message: "선택한 경우에만",
    description:
      "Scope picker option: only sessions that pick the secret receive it",
  },
  "agentEnvironment.secrets.scopeBadgeRepo": {
    message: "저장소: {repo}",
    description:
      "Secret row badge: repo scope, naming the repository (owner/name)",
  },
  "agentEnvironment.secrets.repoLabel": {
    message: "저장소",
    description:
      "Agent secrets card: accessible label of the repo scope's repository field",
  },
  "agentEnvironment.secrets.repoPlaceholder": {
    message: "owner/name",
    description: "Agent secrets card: repository field placeholder",
  },
  "agentEnvironment.secrets.repoEmpty": {
    message: "일치하는 연결된 저장소가 없습니다.",
    description:
      "Agent secrets card: repository picker with no matching connected repository",
  },
  "agentEnvironment.secrets.repoInvalid": {
    message: "저장소를 owner/name 형식으로 입력하세요.",
    description:
      "Repo scope validation error (also AGENT_SECRET_SCOPE_INVALID)",
  },
  "agentEnvironment.repoEnvs.title": {
    message: "저장소 환경",
    description: "Settings → Repository environments card title (ADR109)",
  },
  "agentEnvironment.repoEnvs.description": {
    message:
      "에이전트 세션이 각 저장소를 준비하는 방식입니다. 설정 단계는 각 세션의 에이전트가 시작되기 전에 저장소 루트에서 세션의 시크릿과 함께 순서대로 실행됩니다. 검사는 에이전트가 결과물을 전달하기 전에 실행하는 lint, test 같은 명령입니다.",
    description:
      "Repository environments card description: what setup steps and checks are",
  },
  "agentEnvironment.repoEnvs.loadFailed": {
    message: "저장소 환경을 불러오지 못했습니다.",
    description: "Repository environments card error when the list query fails",
  },
  "agentEnvironment.repoEnvs.emptyList": {
    message: "아직 저장소 환경이 없습니다.",
    description: "Repository environments card empty state",
  },
  "agentEnvironment.repoEnvs.setupSummary": {
    message: "설정: {names}",
    description:
      "Repository row summary: the setup step names, comma-separated",
  },
  "agentEnvironment.repoEnvs.checksSummary": {
    message: "검사: {names}",
    description: "Repository row summary: the check names, comma-separated",
  },
  "agentEnvironment.repoEnvs.noSetup": {
    message: "설정 단계 없음",
    description:
      "Repository row summary (and the read-only dialog) when the environment has no setup steps",
  },
  "agentEnvironment.repoEnvs.noChecks": {
    message: "검사 없음",
    description:
      "Repository row summary (and the read-only dialog) when the environment has no checks",
  },
  "agentEnvironment.repoEnvs.editLabel": {
    message: "{repo} 편집",
    description:
      "Accessible label of a repository environment's edit button (repo is owner/name)",
  },
  "agentEnvironment.repoEnvs.viewLabel": {
    message: "{repo} 보기",
    description:
      "Accessible label of a repository environment's view button, for non-admins",
  },
  "agentEnvironment.repoEnvs.deleteLabel": {
    message: "{repo} 환경 삭제",
    description: "Accessible label of a repository environment's delete button",
  },
  "agentEnvironment.repoEnvs.deleteTitle": {
    message: "{repo} 환경을 삭제하시겠습니까?",
    description: "Delete-environment confirmation title",
  },
  "agentEnvironment.repoEnvs.deleteBody": {
    message: "다음 턴부터 이 저장소의 세션은 설정 단계와 검사 없이 시작됩니다.",
    description: "Delete-environment confirmation body",
  },
  "agentEnvironment.repoEnvs.deleteConfirm": {
    message: "환경 삭제",
    description: "Delete-environment confirmation button",
  },
  "agentEnvironment.repoEnvs.add": {
    message: "환경 추가",
    description:
      "Repository environments card: button that opens the add-environment dialog",
  },
  "agentEnvironment.repoEnvs.help": {
    message:
      "저장하면 저장소의 환경 전체가 교체됩니다. 세션은 다음 턴부터 변경 사항을 반영합니다.",
    description: "Repository environments card help line for admins",
  },
  "agentEnvironment.repoEnvs.readOnly": {
    message:
      "워크스페이스 관리자만 저장소 환경을 추가, 변경, 삭제할 수 있습니다.",
    description: "Repository environments card help line for non-admins",
  },
  "agentEnvironment.repoEnvs.saved": {
    message: "{repo} 환경을 저장했습니다",
    description: "Toast after an environment is saved",
  },
  "agentEnvironment.repoEnvs.deleted": {
    message: "{repo} 환경을 삭제했습니다",
    description: "Toast after an environment is deleted",
  },
  "agentEnvironment.repoEnvs.createTitle": {
    message: "저장소 환경 추가",
    description: "Add-environment dialog title",
  },
  "agentEnvironment.repoEnvs.editTitle": {
    message: "{repo} 편집",
    description: "Edit-environment dialog title (repo is owner/name)",
  },
  "agentEnvironment.repoEnvs.dialogDescription": {
    message:
      "설정 단계는 각 세션의 에이전트가 시작되기 전에 저장소 루트에서 세션의 시크릿과 함께 순서대로 실행됩니다. 단계가 실패하면 에이전트가 시작되기 전에 턴이 종료됩니다. 검사는 에이전트가 결과물을 전달하기 전에 실행하는 명령입니다.",
    description: "Environment dialog description",
  },
  "agentEnvironment.repoEnvs.repoLabel": {
    message: "저장소",
    description: "Environment dialog: repository field label",
  },
  "agentEnvironment.repoEnvs.repoPlaceholder": {
    message: "owner/name",
    description: "Environment dialog: repository field placeholder",
  },
  "agentEnvironment.repoEnvs.repoEmpty": {
    message: "일치하는 연결된 저장소가 없습니다.",
    description:
      "Environment dialog: repository picker with no matching connected repository",
  },
  "agentEnvironment.repoEnvs.setupTitle": {
    message: "설정 단계",
    description: "Environment dialog: setup steps section heading",
  },
  "agentEnvironment.repoEnvs.setupHint": {
    message:
      "도구와 의존성을 설치합니다. 각 단계는 bash로 순서대로 실행되며, 최대 10단계, 단계당 10분입니다.",
    description: "Environment dialog: setup steps section hint",
  },
  "agentEnvironment.repoEnvs.checksTitle": {
    message: "검사",
    description: "Environment dialog: checks section heading",
  },
  "agentEnvironment.repoEnvs.checksHint": {
    message:
      "lint, test, build 같은 명령입니다. 에이전트가 결과물을 전달하기 전에 저장소 루트에서 각 명령을 실행하고 결과를 보고하며, bex가 직접 실행하지는 않습니다. 최대 10개.",
    description: "Environment dialog: checks section hint",
  },
  "agentEnvironment.repoEnvs.addSetup": {
    message: "설정 단계 추가",
    description: "Environment dialog: add a setup step row",
  },
  "agentEnvironment.repoEnvs.addCheck": {
    message: "검사 추가",
    description: "Environment dialog: add a check row",
  },
  "agentEnvironment.repoEnvs.setupNameLabel": {
    message: "설정 단계 {position} 이름",
    description:
      "Accessible label of a setup step's name field (position is its 1-based place in the list)",
  },
  "agentEnvironment.repoEnvs.setupRunLabel": {
    message: "설정 단계 {position} 명령",
    description: "Accessible label of a setup step's command field",
  },
  "agentEnvironment.repoEnvs.setupMoveUp": {
    message: "설정 단계 {position} 위로 이동",
    description: "Accessible label: move a setup step one place earlier",
  },
  "agentEnvironment.repoEnvs.setupMoveDown": {
    message: "설정 단계 {position} 아래로 이동",
    description: "Accessible label: move a setup step one place later",
  },
  "agentEnvironment.repoEnvs.setupRemove": {
    message: "설정 단계 {position} 제거",
    description: "Accessible label: remove a setup step row",
  },
  "agentEnvironment.repoEnvs.checkNameLabel": {
    message: "검사 {position} 이름",
    description:
      "Accessible label of a check's name field (position is its 1-based place in the list)",
  },
  "agentEnvironment.repoEnvs.checkRunLabel": {
    message: "검사 {position} 명령",
    description: "Accessible label of a check's command field",
  },
  "agentEnvironment.repoEnvs.checkMoveUp": {
    message: "검사 {position} 위로 이동",
    description: "Accessible label: move a check one place earlier",
  },
  "agentEnvironment.repoEnvs.checkMoveDown": {
    message: "검사 {position} 아래로 이동",
    description: "Accessible label: move a check one place later",
  },
  "agentEnvironment.repoEnvs.checkRemove": {
    message: "검사 {position} 제거",
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
    message: "닫기",
    description: "Read-only environment dialog: close button",
  },
  "agentEnvironment.repoEnvs.cancel": {
    message: "취소",
    description: "Environment dialog: cancel button",
  },
  "agentEnvironment.repoEnvs.save": {
    message: "저장",
    description: "Environment dialog: save (create or replace) button",
  },
  "agentEnvironment.repoEnvs.nameRequired": {
    message: "이름을 입력하세요.",
    description: "Step or check name required error",
  },
  "agentEnvironment.repoEnvs.nameInvalid": {
    message:
      "이름은 문자 또는 숫자로 시작해야 하며 문자, 숫자, 공백, '.', '_', '-'만 사용할 수 있습니다(최대 64자).",
    description: "Step or check name format error",
  },
  "agentEnvironment.repoEnvs.nameDuplicate": {
    message: "이 목록의 다른 행이 이미 이 이름을 사용하고 있습니다.",
    description: "Step or check name repeated within its list",
  },
  "agentEnvironment.repoEnvs.runRequired": {
    message: "명령을 입력하세요.",
    description: "Step or check command required error",
  },
  "agentEnvironment.repoEnvs.setupRunTooLong": {
    message: "설정 명령은 최대 8 KiB까지 가능합니다.",
    description: "Setup step command too long (or contains a NUL byte)",
  },
  "agentEnvironment.repoEnvs.checkRunTooLong": {
    message: "검사 명령은 최대 2 KiB까지 가능합니다.",
    description: "Check command too long (or contains a NUL byte)",
  },
  "agentEnvironment.repoEnvs.setupTooMany": {
    message: "환경에는 설정 단계를 최대 10개까지 둘 수 있습니다.",
    description: "Too many setup steps",
  },
  "agentEnvironment.repoEnvs.checksTooMany": {
    message: "환경에는 검사를 최대 10개까지 둘 수 있습니다.",
    description: "Too many checks",
  },
  "agentEnvironment.repoEnvs.environmentEmpty": {
    message:
      "설정 단계나 검사를 하나 이상 추가하세요. 모두 제거하려면 대신 환경을 삭제하세요.",
    description: "Environment with neither setup steps nor checks",
  },
  "agentEnvironment.repoEnvs.repoInvalid": {
    message: "저장소를 owner/name 형식으로 입력하세요.",
    description:
      "Repository validation error (also AGENT_REPO_ENVIRONMENT_INVALID on the repository)",
  },
  "agentEnvironment.repoEnvs.repoExists": {
    message: "이 저장소에는 이미 환경이 있습니다. 목록에서 편집하세요.",
    description: "Adding an environment for a repository that already has one",
  },
  "agentEnvironment.repoEnvs.setupRefused": {
    message:
      "설정 단계가 거부되었습니다. 각 이름이 올바르고 고유한지, 각 명령이 8 KiB 이하인지 확인하세요.",
    description: "AGENT_REPO_ENVIRONMENT_INVALID on the setup steps",
  },
  "agentEnvironment.repoEnvs.checksRefused": {
    message:
      "검사가 거부되었습니다. 각 이름이 올바르고 고유한지, 각 명령이 2 KiB 이하인지 확인하세요.",
    description: "AGENT_REPO_ENVIRONMENT_INVALID on the checks",
  },
  "agentEnvironment.repoEnvs.invalid": {
    message: "이 환경이 거부되었습니다. 저장소, 설정 단계, 검사를 확인하세요.",
    description: "AGENT_REPO_ENVIRONMENT_INVALID without a recognized field",
  },
  "agentEnvironment.repoEnvs.limitExceeded": {
    message:
      "이 워크스페이스가 저장소 환경 50개 한도에 도달했습니다. 하나를 삭제한 후 추가하세요.",
    description:
      "AGENT_REPO_ENVIRONMENT_LIMIT_EXCEEDED (also checked before saving)",
  },
  "agentEnvironment.repoEnvs.notFound": {
    message:
      "이 저장소에 더 이상 환경이 없습니다. 다른 사람이 방금 삭제했을 수 있습니다.",
    description:
      "AGENT_REPO_ENVIRONMENT_NOT_FOUND error (deleted concurrently)",
  },
  "agentEnvironment.repoEnvs.unavailable": {
    message: "이 배포에서는 저장소 환경을 사용할 수 없습니다.",
    description: "AGENT_REPO_ENVIRONMENTS_UNAVAILABLE error",
  },
  "agentEnvironment.repoEnvs.forbidden": {
    message: "워크스페이스 관리자만 저장소 환경을 변경할 수 있습니다.",
    description: "Environment write refused for a non-admin",
  },
  "agentEnvironment.repoEnvs.saveFailed": {
    message: "변경 사항을 저장하지 못했습니다. 다시 시도하세요.",
    description: "Generic environment write failure",
  },
  "agentEnvironment.repoEnvs.builds.status.queued": {
    message: "대기 중",
    description: "Build status badge: waiting to start (ADR111 D2)",
  },
  "agentEnvironment.repoEnvs.builds.status.building": {
    message: "빌드 중",
    description: "Build status badge: the setup steps are running",
  },
  "agentEnvironment.repoEnvs.builds.status.success": {
    message: "준비됨",
    description:
      "Build status badge: success — sessions can start from this snapshot",
  },
  "agentEnvironment.repoEnvs.builds.status.failed": {
    message: "실패",
    description: "Build status badge: the build failed",
  },
  "agentEnvironment.repoEnvs.builds.status.cancelled": {
    message: "취소됨",
    description:
      "Build status badge: a newer build replaced it, or its environment was deleted",
  },
  "agentEnvironment.repoEnvs.builds.trigger.environmentSaved": {
    message: "설정 단계 변경됨",
    description:
      "What queued a build: the environment was saved with changed setup steps",
  },
  "agentEnvironment.repoEnvs.builds.trigger.secretChanged": {
    message: "저장소 시크릿 변경됨",
    description: "What queued a build: a repo-scoped secret was set or changed",
  },
  "agentEnvironment.repoEnvs.builds.trigger.manual": {
    message: "수동 빌드",
    description:
      "What queued a build: an admin clicked Build now (or used the API)",
  },
  "agentEnvironment.repoEnvs.builds.trigger.refresh": {
    message: "매일 새로 고침",
    description: "What queued a build: the daily rebuild",
  },
  "agentEnvironment.repoEnvs.builds.failure.setupFailed": {
    message: "설정 단계가 실패했습니다.",
    description:
      "Failed build, failureCode setup_failed (the reason below names the step and exit code)",
  },
  "agentEnvironment.repoEnvs.builds.failure.stepTimeout": {
    message: "설정 단계가 3시간 제한을 초과했습니다.",
    description: "Failed build, failureCode step_timeout",
  },
  "agentEnvironment.repoEnvs.builds.failure.buildTimeout": {
    message: "빌드가 3시간 제한을 초과했습니다.",
    description: "Failed build, failureCode build_timeout",
  },
  "agentEnvironment.repoEnvs.builds.failure.checkoutFailed": {
    message: "저장소의 기본 브랜치를 체크아웃하지 못했습니다.",
    description: "Failed build, failureCode checkout_failed",
  },
  "agentEnvironment.repoEnvs.builds.failure.storageLimitReached": {
    message:
      "스냅샷이 이 워크스페이스의 샌드박스 스토리지 한도를 초과하게 됩니다.",
    description: "Failed build, failureCode storage_limit_reached",
  },
  "agentEnvironment.repoEnvs.builds.failure.internalError": {
    message: "bex 측에서 빌드를 완료하지 못했습니다. 다시 시도하세요.",
    description: "Failed build, failureCode internal_error",
  },
  "agentEnvironment.repoEnvs.builds.failure.unknown": {
    message: "빌드에 실패했습니다.",
    description: "Failed build with no recognized failureCode",
  },
  "agentEnvironment.repoEnvs.builds.title": {
    message: "{repo}의 빌드",
    description: "Build log dialog title (repo is owner/name)",
  },
  "agentEnvironment.repoEnvs.builds.openLabel": {
    message: "{repo}의 빌드",
    description:
      "Accessible label of a repository row's button that opens its builds and logs",
  },
  "agentEnvironment.repoEnvs.builds.description": {
    message:
      "세션은 설정 단계가 현재와 일치하는 가장 최근의 성공한 빌드에서 시작됩니다. 일치하는 빌드가 없으면 각 턴을 시작할 때 설정 단계를 대신 실행합니다.",
    description:
      "Build log dialog description: how sessions use snapshots and the fallback (ADR111 D4)",
  },
  "agentEnvironment.repoEnvs.builds.buildNow": {
    message: "지금 빌드",
    description:
      "Button that queues a build of the repository's setup steps (admins only)",
  },
  "agentEnvironment.repoEnvs.builds.snapshotNote": {
    message:
      "설정 단계가 쓰는 파일은 스냅샷에 보관되므로 자격 증명을 파일에 쓰지 마세요. 시크릿은 환경 변수로 각 단계에 전달되며 보관되지 않습니다.",
    description: "Note beside Build now (Devin's warning, ADR111 D6)",
  },
  "agentEnvironment.repoEnvs.builds.readOnly": {
    message: "워크스페이스 관리자만 빌드를 시작할 수 있습니다.",
    description: "Shown under a disabled Build now for non-admins",
  },
  "agentEnvironment.repoEnvs.builds.forbidden": {
    message: "워크스페이스 관리자만 빌드를 시작할 수 있습니다.",
    description: "Build now refused with FORBIDDEN",
  },
  "agentEnvironment.repoEnvs.builds.nothingToBuild": {
    message:
      "이 환경에는 설정 단계가 없어 빌드할 것이 없습니다. 검사는 빌드가 필요하지 않습니다.",
    description:
      "Build now on a checks-only environment (also AGENT_REPO_ENVIRONMENT_NOTHING_TO_BUILD)",
  },
  "agentEnvironment.repoEnvs.builds.queued": {
    message: "{repo}의 빌드를 대기열에 추가했습니다",
    description: "Toast after Build now succeeds",
  },
  "agentEnvironment.repoEnvs.builds.rateLimited": {
    message:
      "이 저장소는 최근 24시간 동안 이미 12번 빌드를 시작했습니다. 나중에 다시 시도하세요.",
    description:
      "AGENT_REPO_ENVIRONMENT_BUILD_RATE_LIMITED without a usable nextBuildAt",
  },
  "agentEnvironment.repoEnvs.builds.rateLimitedUntil": {
    message:
      "이 저장소는 최근 24시간 동안 이미 12번 빌드를 시작했습니다. 다음 빌드는 {time}에 시작할 수 있습니다.",
    description:
      "AGENT_REPO_ENVIRONMENT_BUILD_RATE_LIMITED; time is nextBuildAt in the viewer's local date and time",
  },
  "agentEnvironment.repoEnvs.builds.unavailable": {
    message: "이 배포에서는 환경 빌드를 사용할 수 없습니다.",
    description: "AGENT_REPO_ENVIRONMENT_BUILDS_UNAVAILABLE",
  },
  "agentEnvironment.repoEnvs.builds.buildFailed": {
    message: "빌드를 시작하지 못했습니다. 다시 시도하세요.",
    description: "Generic Build now failure",
  },
  "agentEnvironment.repoEnvs.builds.loadFailed": {
    message: "빌드를 불러오지 못했습니다.",
    description: "Build history query failed",
  },
  "agentEnvironment.repoEnvs.builds.none": {
    message: "아직 빌드가 없습니다.",
    description: "Build history list with no builds",
  },
  "agentEnvironment.repoEnvs.builds.noneLog": {
    message:
      "이 저장소는 아직 빌드된 적이 없습니다. 지금 빌드를 실행하면 설정 단계를 한 번 실행하고 그 결과를 스냅샷으로 보관합니다.",
    description: "Build log panel when the repository has no builds",
  },
  "agentEnvironment.repoEnvs.builds.historyLabel": {
    message: "빌드 기록",
    description: "Accessible label of the build history list",
  },
  "agentEnvironment.repoEnvs.builds.logTitle": {
    message: "빌드 로그",
    description: "Build log panel header when no build is selected",
  },
  "agentEnvironment.repoEnvs.builds.logFailed": {
    message: "이 빌드의 로그를 불러오지 못했습니다.",
    description: "Single build (log) query failed",
  },
  "agentEnvironment.repoEnvs.builds.took": {
    message: "{duration} 소요",
    description:
      "A finished build's wall time, e.g. 'took 3m 12s' (joined with ' · ')",
  },
  "agentEnvironment.repoEnvs.builds.imageRemoved": {
    message: "스냅샷 삭제됨",
    description:
      "A successful build whose snapshot image was since deleted (older builds keep no image)",
  },
  "agentEnvironment.repoEnvs.builds.notBuilt": {
    message: "아직 빌드되지 않음",
    description: "Repository row: has setup steps but no build yet",
  },
  "agentEnvironment.repoEnvs.builds.noSteps.queued": {
    message:
      "이 빌드는 시작을 기다리고 있습니다. 빌드는 워크스페이스의 에이전트 샌드박스 한도를 세션과 공유합니다.",
    description: "Build log of a queued build",
  },
  "agentEnvironment.repoEnvs.builds.noSteps.building": {
    message:
      "설정 단계를 실행 중입니다. 빌드가 끝나면 로그가 여기에 표시됩니다.",
    description: "Build log of a running build",
  },
  "agentEnvironment.repoEnvs.builds.noSteps.cancelled": {
    message:
      "이 빌드는 취소되었습니다. 더 새로운 빌드로 대체되었거나 해당 환경이 삭제되었습니다.",
    description: "Build log of a cancelled build",
  },
  "agentEnvironment.repoEnvs.builds.noSteps.failed": {
    message: "실행된 설정 단계가 없습니다.",
    description:
      "Build log of a build that failed before any step ran (e.g. the checkout failed)",
  },
  "agentEnvironment.repoEnvs.builds.noSteps.other": {
    message: "이 빌드에는 기록된 단계가 없습니다.",
    description: "Build log with no steps, any other status",
  },
  "agentEnvironment.repoEnvs.builds.stepExit": {
    message: "종료 {code}",
    description: "A setup step's exit code badge",
  },
  "agentEnvironment.repoEnvs.builds.stepKilled": {
    message: "중지됨",
    description: "A setup step with no exit code: killed or never started",
  },
  "agentEnvironment.repoEnvs.builds.noOutput": {
    message: "출력 없음.",
    description: "A setup step that printed nothing",
  },
  "agentEnvironment.skills.title": {
    message: "에이전트 스킬",
    description: "Settings → Agent skills card title (ADR099)",
  },
  "agentEnvironment.skills.description": {
    message:
      "이 워크스페이스의 모든 에이전트 세션이 SKILL.md 파일로 받는 재사용 가능한 지침입니다. 에이전트는 작업에 맞을 때 또는 요청받았을 때만 스킬을 적용합니다.",
    description: "Agent skills card description",
  },
  "agentEnvironment.skills.loadFailed": {
    message: "에이전트 스킬을 불러오지 못했습니다.",
    description: "Agent skills card error when the list query fails",
  },
  "agentEnvironment.skills.empty": {
    message: "아직 에이전트 스킬이 없습니다.",
    description: "Agent skills card empty state",
  },
  "agentEnvironment.skills.onlyWhenAsked": {
    message: "요청 시에만",
    description:
      "Badge on a skill the agent applies only when a prompt invokes it",
  },
  "agentEnvironment.skills.enabledLabel": {
    message: "{name} 사용",
    description: "Accessible label of a skill's enabled switch",
  },
  "agentEnvironment.skills.editLabel": {
    message: "{name} 편집",
    description: "Accessible label of a skill's edit button",
  },
  "agentEnvironment.skills.viewLabel": {
    message: "{name} 보기",
    description: "Accessible label of a skill's view button (non-admins)",
  },
  "agentEnvironment.skills.exportLabel": {
    message: "{name} 다운로드 (SKILL.md)",
    description: "Accessible label of a skill's export button",
  },
  "agentEnvironment.skills.deleteLabel": {
    message: "{name} 삭제",
    description: "Accessible label of a skill's delete button",
  },
  "agentEnvironment.skills.deleteTitle": {
    message: "{name} 스킬을 삭제하시겠습니까?",
    description: "Delete skill confirm title",
  },
  "agentEnvironment.skills.deleteBody": {
    message:
      "새 에이전트 턴부터 이 스킬을 받지 않습니다. 이미 실행된 세션의 대화 기록은 유지됩니다.",
    description: "Delete skill confirm body",
  },
  "agentEnvironment.skills.deleteConfirm": {
    message: "스킬 삭제",
    description: "Delete skill confirm button",
  },
  "agentEnvironment.skills.new": {
    message: "새 스킬",
    description: "Button that opens the create-skill dialog",
  },
  "agentEnvironment.skills.import": {
    message: "SKILL.md 가져오기",
    description: "Button that imports a SKILL.md file",
  },
  "agentEnvironment.skills.help": {
    message:
      "멤버는 스킬을 사용할 수 있으며, 관리자만 추가하거나 변경할 수 있습니다.",
    description: "Agent skills help line for admins",
  },
  "agentEnvironment.skills.readOnly": {
    message:
      "워크스페이스 관리자만 에이전트 스킬을 추가하거나 변경할 수 있습니다.",
    description: "Agent skills help line for non-admins",
  },
  "agentEnvironment.skills.createTitle": {
    message: "새 스킬",
    description: "Create-skill dialog title",
  },
  "agentEnvironment.skills.editTitle": {
    message: "{name} 편집",
    description: "Edit-skill dialog title",
  },
  "agentEnvironment.skills.viewTitle": {
    message: "{name}",
    description: "View-skill dialog title (read-only)",
  },
  "agentEnvironment.skills.dialogDescription": {
    message:
      "에이전트는 설명을 읽고 스킬을 적용할 시점을 판단하며, 적용할 때 지침을 따릅니다.",
    description: "Skill dialog description",
  },
  "agentEnvironment.skills.nameLabel": {
    message: "이름",
    description: "Skill name field label",
  },
  "agentEnvironment.skills.namePlaceholder": {
    message: "run-integration-tests",
    description: "Skill name field placeholder",
  },
  "agentEnvironment.skills.descriptionLabel": {
    message: "설명",
    description: "Skill description field label",
  },
  "agentEnvironment.skills.descriptionPlaceholder": {
    message: "이 스킬을 사용할 시점을 한 줄로 입력하세요",
    description: "Skill description field placeholder",
  },
  "agentEnvironment.skills.onlyWhenAskedLabel": {
    message: "내가 요청할 때만",
    description: "Toggle: the agent applies the skill only when invoked",
  },
  "agentEnvironment.skills.onlyWhenAskedHint": {
    message: "작성 창에서 호출하거나 프롬프트에 @skills:name을 입력하세요.",
    description: "Hint under the only-when-asked toggle",
  },
  "agentEnvironment.skills.bodyLabel": {
    message: "지침",
    description: "Skill body field label",
  },
  "agentEnvironment.skills.bodyPlaceholder": {
    message: "에이전트가 따를 단계를 Markdown으로 작성하세요.",
    description: "Skill body field placeholder",
  },
  "agentEnvironment.skills.contentLabel": {
    message: "SKILL.md",
    description: "Whole-file editor label",
  },
  "agentEnvironment.skills.editAsFile": {
    message: "SKILL.md로 편집",
    description: "Switch the dialog to the whole-file editor",
  },
  "agentEnvironment.skills.editAsFields": {
    message: "필드로 편집",
    description: "Switch the dialog back to separate fields",
  },
  "agentEnvironment.skills.history.tab": {
    message: "버전 기록",
    description: "Skill dialog tab listing the saved versions",
  },
  "agentEnvironment.skills.history.editTab": {
    message: "편집",
    description: "Skill dialog tab with the editing form",
  },
  "agentEnvironment.skills.history.current": {
    message: "현재",
    description: "Badge on the newest saved version",
  },
  "agentEnvironment.skills.history.changes": {
    message: "변경 사항",
    description: "Version view: line diff against the previous version",
  },
  "agentEnvironment.skills.history.content": {
    message: "내용",
    description: "Version view: the full SKILL.md",
  },
  "agentEnvironment.skills.history.load": {
    message: "편집기로 불러오기",
    description: "Button that fills the editor with the selected version",
  },
  "agentEnvironment.skills.history.loadMore": {
    message: "더 보기",
    description: "Button that loads the next page of versions",
  },
  "agentEnvironment.skills.history.noChanges": {
    message: "이 버전은 변경된 내용이 없습니다.",
    description: "Changes view of a version identical to the previous one",
  },
  "agentEnvironment.skills.history.loadFailed": {
    message: "버전 기록을 불러오지 못했습니다.",
    description: "Version history list query failed",
  },
  "agentEnvironment.skills.history.versionFailed": {
    message: "이 버전을 불러오지 못했습니다.",
    description: "Single version query failed",
  },
  "agentEnvironment.skills.history.loadedNotice": {
    message: "불러온 버전: {version}. 저장하면 새 버전으로 보관됩니다.",
    description: "Notice in the editor after Load into editor",
  },
  "agentEnvironment.skills.viewTab": {
    message: "미리보기",
    description: "Read-only skill dialog tab showing the skill",
  },
  "agentEnvironment.skills.save": {
    message: "스킬 저장",
    description: "Skill dialog save button",
  },
  "agentEnvironment.skills.cancel": {
    message: "취소",
    description: "Skill dialog cancel button",
  },
  "agentEnvironment.skills.close": {
    message: "닫기",
    description: "Skill dialog close button (read-only)",
  },
  "agentEnvironment.skills.saved": {
    message: "{name} 스킬을 저장했습니다",
    description: "Toast after a skill is saved",
  },
  "agentEnvironment.skills.deleted": {
    message: "{name} 스킬을 삭제했습니다",
    description: "Toast after a skill is deleted",
  },
  "agentEnvironment.skills.readFailed": {
    message: "스킬을 열지 못했습니다. 다시 시도하세요.",
    description: "Error when a single skill read fails",
  },
  "agentEnvironment.skills.importFailed": {
    message: "해당 파일을 읽지 못했습니다.",
    description: "Error when an imported file can't be read",
  },
  "agentEnvironment.skills.nameRequired": {
    message: "이름을 입력하세요.",
    description: "Skill name empty",
  },
  "agentEnvironment.skills.nameInvalid": {
    message: "소문자, 숫자, 단일 하이픈만 사용할 수 있으며 최대 64자입니다.",
    description: "Skill name fails the Agent Skills format",
  },
  "agentEnvironment.skills.descriptionRequired": {
    message: "한 줄 설명을 입력하세요.",
    description: "Skill description empty",
  },
  "agentEnvironment.skills.descriptionInvalid": {
    message: "설명은 최대 1,024자의 한 줄로 작성하세요.",
    description: "Skill description too long or multi-line",
  },
  "agentEnvironment.skills.bodyInvalid": {
    message: "지침은 64 KiB로 제한됩니다.",
    description: "Skill body too large",
  },
  "agentEnvironment.skills.invalid": {
    message: "스킬이 올바르지 않습니다. 이름, 설명, 지침을 확인하세요.",
    description: "AGENT_SKILL_INVALID error",
  },
  "agentEnvironment.skills.limitExceeded": {
    message:
      "에이전트 스킬 한도에 도달했습니다(워크스페이스당 100개, 활성화된 스킬 합계 1 MiB).",
    description: "AGENT_SKILL_LIMIT_EXCEEDED error",
  },
  "agentEnvironment.skills.conflict": {
    message:
      "스킬을 연 이후 내용이 변경되었거나 이미 사용 중인 이름입니다. 닫았다가 다시 열어 최신 내용을 확인하세요.",
    description: "AGENT_SKILL_CONFLICT error",
  },
  "agentEnvironment.skills.notFound": {
    message: "해당 스킬이 더 이상 존재하지 않습니다.",
    description: "AGENT_SKILL_NOT_FOUND error",
  },
  "agentEnvironment.skills.unavailable": {
    message: "이 배포에서는 에이전트 스킬을 사용할 수 없습니다.",
    description: "AGENT_SKILLS_UNAVAILABLE error",
  },
  "agentEnvironment.skills.forbidden": {
    message: "워크스페이스 관리자만 에이전트 스킬을 변경할 수 있습니다.",
    description: "Skill write refused for a non-admin",
  },
  "agentEnvironment.skills.saveFailed": {
    message: "변경 사항을 저장하지 못했습니다. 다시 시도하세요.",
    description: "Generic skill write failure",
  },
  "agentEnvironment.agentsTitle": {
    message: "사용 가능한 에이전트",
    description: "Settings → Agents: availability section heading",
  },
  "agentEnvironment.agentsDescription": {
    message:
      "멤버가 세션을 시작할 때 사용할 수 있는 에이전트를 선택합니다. 에이전트를 꺼도 실행 중인 세션은 계속 실행됩니다.",
    description: "Settings → Agents: availability section description",
  },
  "agentEnvironment.agentsDataUseNote": {
    message:
      "모델 제공업체가 세션에서 전송된 내용을 학습에 사용할 수 있습니다.",
    description:
      "Settings → Agents: note under an agent whose provider trains on session data (Muse Code)",
  },
  "agentEnvironment.agentsDataUseTitle": {
    message: "{agent} 에이전트를 켜시겠습니까?",
    description:
      "Confirm title before turning on an agent whose provider trains on session data",
  },
  "agentEnvironment.agentsDataUseBody.muse": {
    message:
      "이 에이전트는 Meta의 기여자 티어에서 실행됩니다. Meta는 세션이 전송하는 프롬프트, 코드, 파일, 컨텍스트와 그에 대한 응답을 모델 학습에 사용할 수 있습니다. 켜면 관리자인 회원님이 워크스페이스를 대신하여 이를 인정한다는 사실이 기록됩니다.",
    description: "Confirm body: Muse Code's data-use acknowledgement (ADR108)",
  },
  "agentEnvironment.agentsDataUseConfirm": {
    message: "확인하고 켜기",
    description: "Confirm button: acknowledge data use and enable the agent",
  },
  "agentEnvironment.agentsSaved": {
    message: "사용 가능한 에이전트를 업데이트했습니다",
    description: "Toast after an availability change",
  },
  "agentEnvironment.agentsSaveFailed": {
    message: "사용 가능한 에이전트를 업데이트하지 못했습니다. 다시 시도하세요.",
    description: "Error after a failed availability change",
  },
  "agentEnvironment.githubCliLabel": {
    message: "GitHub CLI (gh)",
    description:
      "Label of the switch that gives the workspace's agent sessions gh, the GitHub CLI (w10/m524)",
  },
  "agentEnvironment.githubCliHelp": {
    message:
      "끄면 이 워크스페이스의 모든 에이전트 세션(실행 중인 세션 포함)에서 gh를 사용할 수 없습니다. 세션 브랜치로의 Git 푸시와 bex의 풀 리퀘스트 도구는 계속 작동합니다.",
    description:
      "Help under the GitHub CLI switch: what turning it off does (w10/m524)",
  },
  "agentEnvironment.githubCliTurnedOn": {
    message: "에이전트 세션의 GitHub CLI를 켰습니다",
    description:
      "Toast after an admin turns the sessions' GitHub CLI on (w10/m524)",
  },
  "agentEnvironment.githubCliTurnedOff": {
    message: "에이전트 세션의 GitHub CLI를 껐습니다",
    description:
      "Toast after an admin turns the sessions' GitHub CLI off (w10/m524)",
  },
  "agentEnvironment.githubCliSaveFailed": {
    message: "GitHub CLI 설정을 변경하지 못했습니다. 다시 시도하세요.",
    description: "Error after a failed GitHub CLI switch (w10/m524)",
  },
  "agentEnvironment.githubCliMandatoryLabel": {
    message: "끄기를 필수로 설정",
    description:
      "Label of the switch that makes the workspace's GitHub CLI off a floor no automation or session can turn back on (w10/m561)",
  },
  "agentEnvironment.githubCliMandatoryHelp": {
    message:
      "GitHub CLI가 꺼져 있는 동안 적용됩니다. 이때는 어떤 자동화나 세션도 gh를 다시 켤 수 없습니다.",
    description:
      "Help under the mandatory switch: it applies only while GitHub CLI is off (w10/m561)",
  },
  "agentEnvironment.githubCliMandatoryTurnedOn": {
    message: "GitHub CLI 끄기를 필수로 설정했습니다",
    description:
      "Toast after an admin makes the workspace's GitHub CLI off mandatory (w10/m561)",
  },
  "agentEnvironment.githubCliMandatoryTurnedOff": {
    message: "GitHub CLI 끄기 필수 설정을 해제했습니다",
    description:
      "Toast after an admin makes the workspace's GitHub CLI off no longer mandatory (w10/m561)",
  },
  "agentEnvironment.githubCliMandatorySaveFailed": {
    message:
      "GitHub CLI 끄기 필수 설정을 변경하지 못했습니다. 다시 시도하세요.",
    description: "Error after a failed mandatory switch (w10/m561)",
  },
  "agentEnvironment.githubCliAutomationsLabel": {
    message: "자동화의 GitHub CLI",
    description:
      "Label of the select that binds the gh of the sessions automations start: inherit, on or off (w10/m561)",
  },
  "agentEnvironment.githubCliAutomationsHelp": {
    message:
      "자동화가 시작하는 세션의 gh입니다. 자동화에서 따로 설정하지 않으면 이 값을 사용합니다. 상속은 위 스위치를 따릅니다.",
    description:
      "Help under the automations GitHub CLI select: an automation's own binding wins, and inherit follows the workspace switch (w10/m561)",
  },
  "agentEnvironment.githubCliAutomationsInherited": {
    message: "이제 자동화가 GitHub CLI 설정을 상속합니다",
    description:
      "Toast after an admin returns automations to inheriting the workspace's GitHub CLI switch (w10/m561)",
  },
  "agentEnvironment.githubCliAutomationsTurnedOn": {
    message: "자동화의 GitHub CLI를 켰습니다",
    description:
      "Toast after an admin turns GitHub CLI on for the sessions automations start (w10/m561)",
  },
  "agentEnvironment.githubCliAutomationsTurnedOff": {
    message: "자동화의 GitHub CLI를 껐습니다",
    description:
      "Toast after an admin turns GitHub CLI off for the sessions automations start (w10/m561)",
  },
  "agentEnvironment.githubCliAutomationsSaveFailed": {
    message: "자동화의 GitHub CLI를 변경하지 못했습니다. 다시 시도하세요.",
    description:
      "Error after a failed automations GitHub CLI choice (w10/m561)",
  },
  "agentEnvironment.githubCliAutomationsLocked": {
    message: "GitHub CLI 끄기가 필수이므로 자동화에서 gh를 켤 수 없습니다.",
    description:
      "Mapped message for AGENT_ENVIRONMENT_GITHUB_CLI_LOCKED: automations cannot be bound on under a mandatory off (w10/m561)",
  },
  "agentEnvironment.modelsTitle": {
    message: "멤버가 선택할 수 있는 모델",
    description:
      "Settings → Agents: heading of the list of platform models members can choose (w10/m527)",
  },
  "agentEnvironment.modelsDescription": {
    message:
      "멤버가 세션, 메시지, 자동화에서 선택할 수 있는 플랫폼 모델을 고릅니다. 전체 모델을 허용하거나, 모델별 또는 제공업체별로 일부만 허용할 수 있습니다. 끈 모델을 이미 사용 중인 세션이나 자동화는 다음 턴부터 기본 모델로 실행됩니다.",
    description:
      "Settings → Agents: description of the model list — what turning a model off does (w10/m527), and that it allows all models (w10/m592) or filters them by model or by provider (w10/m573)",
  },
  "agentEnvironment.modelsDefaultLocked": {
    message:
      "세션이 대신 사용하는 모델이므로 플랫폼 기본 모델은 항상 켜져 있습니다.",
    description:
      "Note under the platform catalog's default model, whose switch is locked on (w10/m527)",
  },
  "agentEnvironment.modelsNew": {
    message: "카탈로그에 새로 추가됨",
    description:
      "Note under a platform model that is off because it joined the catalog after an admin narrowed the list; an admin turns it on with its switch (w10/m568)",
  },
  "agentEnvironment.modelsTurnedOn": {
    message: "멤버용 {model} 모델을 켰습니다",
    description:
      "Toast after an admin turns a model on for members; {model} is its id (w10/m527)",
  },
  "agentEnvironment.modelsTurnedOff": {
    message: "멤버용 {model} 모델을 껐습니다",
    description:
      "Toast after an admin turns a model off for members; {model} is its id (w10/m527)",
  },
  "agentEnvironment.modelsInvalid": {
    message:
      "모델을 업데이트하지 못했습니다. 플랫폼 기본 모델은 켜져 있어야 하며, 목록에는 플랫폼 카탈로그의 모델만 넣을 수 있습니다. 페이지를 새로고침해 현재 카탈로그를 확인하세요.",
    description:
      "Server refusal AGENT_ENVIRONMENT_MODELS_INVALID of a model list change (w10/m527)",
  },
  "agentEnvironment.modelsSaveFailed": {
    message:
      "멤버가 선택할 수 있는 모델을 업데이트하지 못했습니다. 다시 시도하세요.",
    description: "Error after a failed model switch (w10/m527)",
  },
  "agentEnvironment.modelsFilterLabel": {
    message: "멤버가 선택할 수 있는 모델 필터 방식",
    description:
      "Settings → Agents: accessible name of the choice between every platform model (w10/m592) and filtering them by model or by provider (w10/m573)",
  },
  "agentEnvironment.modelsAll": {
    message: "전체 모델",
    description:
      "Settings → Agents: option that lets members choose every platform model, one added to the catalog later included — Devin's “Allow all models”, every workspace's default (w10/m592). Keep it short: the three options share one line on a 390 px phone",
  },
  "agentEnvironment.modelsByModel": {
    message: "모델별",
    description:
      "Settings → Agents: option that filters the platform models members can choose one model at a time (w10/m573)",
  },
  "agentEnvironment.modelsByProvider": {
    message: "제공업체별",
    description:
      "Settings → Agents: option that filters the platform models members can choose by provider, the prefix of a model id before / (w10/m573)",
  },
  "agentEnvironment.modelsAllHelp": {
    message:
      "멤버는 이후 플랫폼에 추가되는 모델을 포함해 모든 플랫폼 모델을 선택할 수 있습니다. 모델을 끄려면 모델별 또는 제공업체별로 필터링하세요.",
    description:
      "Settings → Agents: help under the choice while it is All models — every platform model, a later one included; filtering by model or by provider turns models off (w10/m592)",
  },
  "agentEnvironment.modelsByModelHelp": {
    message:
      "멤버는 여기에서 켠 모델만 선택할 수 있습니다. 이후 플랫폼에 추가되는 모델은 여기의 모든 모델이 켜져 있어도 직접 켤 때까지 꺼진 상태로 유지됩니다.",
    description:
      "Settings → Agents: help under the choice while it is by model — an allowlist: members pick only the models on, and a model added to the catalog later stays off even while every listed model is on (w10/m568, w10/m573, w10/m592)",
  },
  "agentEnvironment.modelsByProviderHelp": {
    message:
      "멤버는 허용한 제공업체의 모든 모델을 선택할 수 있으며, 이후 플랫폼에 추가되는 모델도 포함됩니다. 다른 제공업체의 모델은 꺼진 상태로 유지됩니다.",
    description:
      "Settings → Agents: help under the filter choice while it is by provider — every model of an allowed provider, a later one included, and other providers' models off (w10/m573)",
  },
  "agentEnvironment.modelsProviderDefault": {
    message: "{model} (기본 모델, 항상 켜짐)",
    description:
      "Settings → Agents, by provider: the catalog's default model in the list under its provider's switch; {model} is its id (w10/m573)",
  },
  "agentEnvironment.modelsProviderLastLocked": {
    message:
      "제공업체는 최소 하나 이상 허용되어야 합니다. 기본 모델만 제공하려면 모델별로 필터링하세요.",
    description:
      "Settings → Agents, by provider: note under the one provider still allowed, whose switch is locked on (w10/m573)",
  },
  "agentEnvironment.modelsProviderTurnedOn": {
    message: "멤버용 {provider} 모델을 켰습니다",
    description:
      "Toast after an admin allows a provider's models for members; {provider} is the provider id, e.g. anthropic (w10/m573)",
  },
  "agentEnvironment.modelsProviderTurnedOff": {
    message: "멤버용 {provider} 모델을 껐습니다",
    description:
      "Toast after an admin stops allowing a provider's models for members; {provider} is the provider id (w10/m573)",
  },
  "agentEnvironment.modelsAllowedAll": {
    message:
      "이제 멤버는 이후 플랫폼에 추가되는 모델을 포함해 모든 모델 중에서 선택합니다",
    description:
      "Toast after an admin switches the model list to All models (w10/m592)",
  },
  "agentEnvironment.modelsFilteredByProvider": {
    message: "이제 멤버는 허용한 제공업체의 모델 중에서 선택합니다",
    description:
      "Toast after an admin switches the model list to filtering by provider (w10/m573)",
  },
  "agentEnvironment.modelsFilteredByModel": {
    message: "이제 멤버는 허용한 모델 중에서 선택합니다",
    description:
      "Toast after an admin switches the model list to filtering by model, from All models or from by provider (w10/m573, w10/m592)",
  },
  "agentEnvironment.modelsProvidersInvalid": {
    message:
      "제공업체를 업데이트하지 못했습니다. 목록에는 플랫폼 카탈로그의 제공업체가 하나 이상 있어야 합니다. 페이지를 새로고침해 현재 카탈로그를 확인하세요.",
    description:
      "Server refusal AGENT_ENVIRONMENT_MODELS_INVALID of a provider filter change (field enabledProviders, w10/m573)",
  },
  "agentEnvironment.modelsAllowAllInvalid": {
    message:
      "멤버가 선택할 수 있는 모델을 변경하지 못했습니다. 이 워크스페이스는 더 이상 플랫폼 모델 카탈로그를 사용하지 않습니다. 페이지를 새로고침해 현재 설정을 확인하세요.",
    description:
      "Server refusal AGENT_ENVIRONMENT_MODELS_INVALID of a switch to or from All models (field allowAllModels): the workspace no longer uses the platform model catalog (w10/m592)",
  },
  "agentEnvironment.modelOffOption": {
    message: "{model} (꺼짐)",
    description:
      "Default model picker: the saved default, which an admin turned off (w10/m527)",
  },
  "agentEnvironment.modelDefaultOffNote": {
    message:
      "{model} 모델이 꺼져 있으므로, 모델을 지정하지 않은 세션과 자동화는 관리자가 켜진 모델을 선택하거나 {model} 모델을 다시 켤 때까지 플랫폼 기본 모델인 {fallback} 모델을 사용합니다.",
    description:
      "Note under the default model picker when the saved default is turned off; {fallback} is the platform default that runs instead (w10/m527)",
  },
  "agentEnvironment.modelDefaultRetiredNote": {
    message:
      "{model} 모델이 더 이상 플랫폼 카탈로그에 없으므로, 모델을 지정하지 않은 세션과 자동화는 관리자가 다른 모델을 선택할 때까지 플랫폼 기본 모델인 {fallback} 모델을 사용합니다.",
    description:
      "Note under the default model picker when the saved default left the platform catalog; {fallback} is the platform default that runs instead (w10/m527)",
  },
  "agentEnvironment.idleTimeoutTitle": {
    message: "유휴 시간 제한",
    description: "Heading of the Enterprise idle timeout section (w6/m563)",
  },
  "agentEnvironment.idleTimeoutDescription": {
    message:
      "턴이 끝나거나 마지막 편집기 연결이 끊긴 뒤 세션의 샌드박스가 유지되는 시간입니다. 닫힌 세션에 메시지를 보내면 대화를 이어 가는 새 샌드박스가 시작됩니다. 자동화의 세션과 다른 세션이 시작한 세션의 샌드박스는 턴이 끝나는 즉시 닫힙니다.",
    description:
      "Description of the idle timeout section: what it bounds, and that automations' and child sessions' sandboxes close at once",
  },
  "agentEnvironment.idleTimeoutLabel": {
    message: "분",
    description: "Label of the idle timeout's minutes field",
  },
  "agentEnvironment.idleTimeoutSave": {
    message: "시간 제한 저장",
    description: "Button that saves the idle timeout",
  },
  "agentEnvironment.idleTimeoutHelp": {
    message:
      "{min}~{max}분. 설정하지 않은 워크스페이스는 {min}분을 사용합니다.",
    description: "Help under the idle timeout field; {min} is also the default",
  },
  "agentEnvironment.idleTimeoutInvalid": {
    message: "{min}에서 {max} 사이의 정수 분을 입력하세요.",
    description:
      "Client/server refusal of an idle timeout outside {min}-{max} minutes",
  },
  "agentEnvironment.idleTimeoutEnterpriseOnly": {
    message: "Enterprise 워크스페이스만 유휴 시간 제한을 변경할 수 있습니다.",
    description:
      "Server refusal of an idle timeout on a workspace no longer on Enterprise",
  },
  "agentEnvironment.idleTimeoutSaved": {
    message: "유휴 시간 제한을 {minutes}분으로 설정했습니다",
    description: "Toast after the idle timeout saved",
  },
  "agentEnvironment.idleTimeoutSaveFailed": {
    message: "유휴 시간 제한을 저장하지 못했습니다. 다시 시도하세요.",
    description: "Generic idle timeout write failure",
  },
};

export default koAgentEnvironment;
