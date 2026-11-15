import type { TranslationEntry } from "@/i18n";

const koInvites: Record<string, TranslationEntry> = {
  "invites.title": {
    message: "워크스페이스 초대",
    description: "Invitation review: title",
  },
  "invites.authenticate": {
    message: "워크스페이스 초대를 확인하려면 계정을 만들거나 로그인하세요.",
    description: "Invitation review: authenticate",
  },
  "invites.signUp": {
    message: "가입",
    description: "Invitation review: signUp",
  },
  "invites.signIn": {
    message: "로그인",
    description: "Invitation review: signIn",
  },
  "invites.joinTitle": {
    message: "{workspace} 참여",
    description: "Invitation review: joinTitle",
  },
  "invites.memberTitle": {
    message: "{workspace}의 멤버입니다",
    description: "Invitation review: memberTitle",
  },
  "invites.role": {
    message: "{role} 역할로 참여합니다.",
    description: "Invitation review: role",
  },
  "invites.memberRole": {
    message: "내 역할은 {role}입니다.",
    description: "Invitation review: memberRole",
  },
  "invites.inviter": {
    message: "{email}님이 초대함",
    description: "Invitation review: inviter",
  },
  "invites.account": {
    message: "{email}(으)로 로그인됨",
    description: "Invitation review: account",
  },
  "invites.join": {
    message: "워크스페이스 참여",
    description: "Invitation review: join",
  },
  "invites.joining": {
    message: "참여하는 중…",
    description: "Invitation review: joining",
  },
  "invites.open": {
    message: "워크스페이스 열기",
    description: "Invitation review: open",
  },
  "invites.opening": {
    message: "여는 중…",
    description: "Invitation review: opening",
  },
  "invites.notNow": {
    message: "나중에",
    description: "Invitation review: notNow",
  },
  "invites.continue": {
    message: "bex로 계속",
    description: "Invitation review: continue",
  },
  "invites.unavailableTitle": {
    message: "초대를 사용할 수 없습니다",
    description: "Invitation review: unavailableTitle",
  },
  "invites.invalid": {
    message:
      "이 초대 링크가 올바르지 않거나 취소되었습니다. 워크스페이스 관리자에게 새 초대를 요청하세요.",
    description: "Invitation review: invalid",
  },
  "invites.expired": {
    message:
      "이 초대가 만료되었습니다. 워크스페이스 관리자에게 다시 보내 달라고 요청하세요.",
    description: "Invitation review: expired",
  },
  "invites.used": {
    message:
      "이 초대는 이미 사용되었습니다. 참여한 계정으로 로그인하거나 새 초대를 요청하세요.",
    description: "Invitation review: used",
  },
  "invites.planLimit": {
    message:
      "이 워크스페이스는 현재 요금제에서 멤버를 더 받을 수 없습니다. 워크스페이스 관리자에게 요금제와 좌석을 확인해 달라고 요청한 후 다시 시도하세요.",
    description: "Invitation review: planLimit",
  },
  "invites.retryError": {
    message:
      "요청을 완료하지 못했습니다. 초대는 이 탭에 저장되어 있습니다. 다시 시도하세요.",
    description: "Invitation review: retryError",
  },
  "invites.accessPending": {
    message:
      "멤버십은 확인되었지만 워크스페이스 액세스가 아직 준비되지 않았습니다. 워크스페이스를 다시 열어 보세요.",
    description: "Invitation review: accessPending",
  },
  "invites.storageUnavailable": {
    message:
      "이 브라우저에 초대를 저장하지 못했습니다. 사이트 저장소를 허용한 후 초대 링크를 다시 여세요.",
    description: "Invitation review: storageUnavailable",
  },
  "invites.retry": {
    message: "다시 시도",
    description: "Invitation review: retry",
  },
};

export default koInvites;
