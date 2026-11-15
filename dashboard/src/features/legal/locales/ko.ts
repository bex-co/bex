import type { TranslationEntry } from "@/i18n";

const koLegal: Record<string, TranslationEntry> = {
  "legal.termsLink": {
    message: "약관",
    description:
      "Link to the agent model usage terms, after a pricing statement",
  },
  "legal.agentModels.title": {
    message: "에이전트 모델 사용 약관",
    description: "Agent model terms page heading and browser title",
  },
  "legal.agentModels.effective": {
    message: "{date} 시행",
    description: "Effective date of the current terms version",
  },
  "legal.agentModels.scopeTitle": {
    message: "적용 범위",
    description: "Terms section heading",
  },
  "legal.agentModels.scopeBody": {
    message:
      "이 약관은 워크스페이스의 에이전트 세션이 워크스페이스 자체 모델 제공업체 키가 아니라 bex에서 제공하는 모델을 사용할 때 적용됩니다.",
    description: "Terms: scope",
  },
  "legal.agentModels.dataTitle": {
    message: "내 데이터",
    description: "Terms section heading",
  },
  "legal.agentModels.dataSend": {
    message:
      "응답을 생성하기 위해 bex는 세션의 프롬프트, 코드, 도구 출력을 OpenRouter를 통해 타사 모델 제공업체에 전송합니다.",
    description: "Terms: what is sent and to whom",
  },
  "legal.agentModels.dataRouting": {
    message:
      "요청은 처리 이후 프롬프트를 저장하지 않고 이를 학습에 사용하지 않는 제공업체로만 라우팅됩니다. bex는 데이터 수집을 거부하도록 설정한 OpenRouter 제공업체 라우팅을 사용합니다.",
    description: "Terms: the routing policy",
  },
  "legal.agentModels.dataMuse": {
    message:
      "Muse Code 세션은 예외입니다. bex는 이 세션을 Meta의 기여자 등급 모델 API로 직접 전송하며, 이 등급에서는 Meta가 전송된 프롬프트, 코드, 파일, 컨텍스트와 반환된 응답을 모델 학습에 사용할 수 있습니다. Muse Code는 관리자가 이를 켜고 이 내용을 확인하기 전까지 워크스페이스에서 꺼져 있습니다.",
    description:
      "Terms: the Muse Code exception to the routing policy (ADR108)",
  },
  "legal.agentModels.dataOwnKey": {
    message:
      "워크스페이스가 자체 제공업체 키를 사용하는 경우, 요청은 해당 제공업체와의 계약에 따라 사용자의 제공업체로 전송되며 bex는 모델 토큰을 청구하지 않습니다.",
    description: "Terms: the bring-your-own-key alternative",
  },
  "legal.agentModels.priceTitle": {
    message: "가격",
    description: "Terms section heading",
  },
  "legal.agentModels.priceBody": {
    message:
      "각 요청은 모델 제공업체의 실제 비용에 30%를 더한 금액으로 청구됩니다. 사용량은 요청별로 측정되며 결제 → 에이전트 모델에서 확인할 수 있습니다.",
    description: "Terms: price",
  },
  "legal.agentModels.capsTitle": {
    message: "지출 한도",
    description: "Terms section heading",
  },
  "legal.agentModels.capsPlan": {
    message:
      "각 워크스페이스에는 요금제에 따라 월 지출 한도가 설정됩니다. Hobby $20, Pro $200, Scale $2,000입니다. 워크스페이스 관리자는 요금제 한도의 최대 10배까지 조정할 수 있습니다.",
    description: "Terms: plan caps",
  },
  "legal.agentModels.capsNew": {
    message:
      "새 워크스페이스는 첫 결제에 성공하기 전까지 bex 제공 모델에서 동시에 두 개의 세션만 실행할 수 있습니다.",
    description: "Terms: new-workspace limits",
  },
  "legal.agentModels.capsReached": {
    message:
      "한도에 도달하면 다음 달이 시작되거나 관리자가 한도를 올릴 때까지 새 모델 요청이 거부됩니다.",
    description: "Terms: what happens at the cap",
  },
  "legal.agentModels.chargesTitle": {
    message: "월중 청구",
    description: "Terms section heading",
  },
  "legal.agentModels.chargesThreshold": {
    message:
      "월별 청구서와 별도로, 워크스페이스의 미청구 에이전트 모델 사용량이 워크스페이스의 청구 임계값에 도달할 때마다 bex가 워크스페이스의 결제 수단으로 요금을 청구하는 것을 승인합니다. 청구가 진행되는 동안에도 워크스페이스는 계속 작동합니다. 임계값은 $10에서 시작하며 워크스페이스의 결제 이력에 따라 상승합니다.",
    description: "Terms: threshold charge authorization",
  },
  "legal.agentModels.chargesFailed": {
    message:
      "청구에 실패하거나 인증이 필요한 경우, 잔액이 결제될 때까지 에이전트 모델 사용이 일시 중지됩니다.",
    description: "Terms: failed charge",
  },
  "legal.agentModels.deletionTitle": {
    message: "워크스페이스 또는 계정 삭제",
    description: "Terms section heading",
  },
  "legal.agentModels.deletionSettle": {
    message:
      "워크스페이스 또는 계정이 삭제되기 전에 bex는 미청구 사용량을 청구합니다. 청구에 실패하면 잔액이 결제될 때까지 삭제가 보류됩니다.",
    description: "Terms: settle before delete",
  },
  "legal.agentModels.deletionAuthorize": {
    message:
      "삭제 이전에 발생한 사용량에 대해 삭제 이후에도 bex가 요금을 청구하는 것을 승인합니다.",
    description: "Terms: charges after deletion",
  },
  "legal.agentModels.paymentTitle": {
    message: "결제 수단",
    description: "Terms section heading",
  },
  "legal.agentModels.paymentBody": {
    message:
      "선불 카드는 사용할 수 없습니다. 미결제 bex 잔액이 있는 카드는 거부될 수 있습니다.",
    description: "Terms: card restrictions",
  },
  "legal.agentModels.recordsTitle": {
    message: "사용 기록",
    description: "Terms section heading",
  },
  "legal.agentModels.recordsBody": {
    message:
      "bex는 모든 요청의 기록(시간, 모델, 토큰, 가격)을 13개월 동안 보관합니다. 이 기록은 청구에 이의가 제기될 때 증거로 사용될 수 있습니다.",
    description: "Terms: usage record retention",
  },
  "legal.agentModels.changesTitle": {
    message: "약관 변경",
    description: "Terms section heading",
  },
  "legal.agentModels.changesBody": {
    message:
      "bex는 이 약관을 업데이트할 수 있습니다. 이 페이지 상단의 시행일이 현재 버전을 나타냅니다.",
    description: "Terms: changes",
  },
};

export default koLegal;
