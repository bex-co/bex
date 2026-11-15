import type { TranslationEntry } from "@/i18n";

const koTeam: Record<string, TranslationEntry> = {
  "team.title": {
    message: "팀",
    description: "Settings Team section card title",
  },
  "team.description": {
    message:
      "이 워크스페이스의 구성원과 각자가 할 수 있는 작업입니다. 이메일로 팀원을 초대하고 역할을 지정하세요. 역할은 모든 리소스에 적용됩니다.",
    description: "Settings Team section card description",
  },
  "team.searchLabel": {
    message: "멤버 검색",
    description: "Accessible label for the accepted-member search field",
  },
  "team.searchPlaceholder": {
    message: "멤버 검색",
    description: "Placeholder for the accepted-member search field",
  },
  "team.emptyTitle": {
    message: "아직 멤버가 없습니다",
    description: "Accepted-member table empty-state title",
  },
  "team.emptyBody": {
    message: "이 워크스페이스에서 협업을 시작하려면 누군가를 초대하세요.",
    description: "Accepted-member table empty-state description",
  },
  "team.noMatchesTitle": {
    message: "일치하는 멤버 없음",
    description: "Accepted-member search no-results title",
  },
  "team.noMatchesBody": {
    message: "다른 이메일이나 ID로 검색해 보세요.",
    description: "Accepted-member search no-results description",
  },
  "team.colMember": {
    message: "멤버",
    description: "Team table column header — the member identity",
  },
  "team.colRole": {
    message: "역할",
    description: "Team table column header — the member's role",
  },
  "team.colEmail": {
    message: "이메일",
    description: "Pending invites table column header",
  },
  "team.pendingTitle": {
    message: "대기 중인 초대",
    description: "Heading above the pending-invites table",
  },
  "team.invite": {
    message: "초대",
    description: "Button that opens the invite dialog",
  },
  "team.inviteTitle": {
    message: "팀원 초대",
    description: "Invite dialog title",
  },
  "team.inviteDescription": {
    message:
      "해당 주소로 처음 로그인하면 이메일을 받고 이 워크스페이스에 참여하게 됩니다.",
    description: "Invite dialog description",
  },
  "team.fieldEmail": {
    message: "이메일 주소",
    description: "Invite dialog email field label",
  },
  "team.fieldEmailPlaceholder": {
    message: "teammate@example.com",
    description: "Invite dialog email field placeholder",
  },
  "team.fieldEmailInvalid": {
    message: "올바른 이메일 주소를 입력하세요. 예: teammate@example.com",
    description: "Invite dialog validation message for a malformed email",
  },
  "team.fieldRole": {
    message: "역할",
    description: "Invite dialog role field label",
  },
  "team.inviteCancel": {
    message: "취소",
    description: "Invite dialog cancel button",
  },
  "team.inviteSubmit": {
    message: "초대 보내기",
    description: "Invite dialog submit button",
  },
  "team.inviteSuccess": {
    message: "{email}로 초대를 보냈습니다",
    description: "Toast after a successful invite",
  },
  "team.inviteError": {
    message: "{email} 초대에 실패했습니다",
    description: "Toast after a failed invite",
  },
  "team.inviteErrorPlanTitle": {
    message: "현재 요금제에서는 이 초대를 보낼 수 없습니다",
    description:
      "Title of the inline alert when the workspace plan's member cap or role set blocks an invite",
  },
  "team.inviteErrorPlanCta": {
    message: "요금제 변경",
    description:
      "Link from the blocked-invite alert to the workspace settings plan section",
  },
  "team.inviteErrorPlanLimitSeats": {
    message:
      "{plan} 요금제는 워크스페이스 멤버를 {limit}명으로 제한합니다. 더 많이 초대하려면 업그레이드하세요.",
    description:
      "Plan seat-cap refusal in the invite dialog — shown when accepted members + pending invites reach the plan maximum",
  },
  "team.inviteErrorPlanLimitRole": {
    message:
      "{plan} 요금제에서는 이 역할을 사용할 수 없습니다. 모든 역할을 사용하려면 업그레이드하세요.",
    description:
      "Plan role-gate refusal in the invite dialog — shown when the chosen role isn't available on the current plan",
  },
  "team.remove": {
    message: "제거",
    description: "Remove-member button / confirm label",
  },
  "team.roleFor": {
    message: "{identity}의 역할",
    description:
      "Accessible name of a member row's role picker, naming the member (w4/150)",
  },
  "team.removeMember": {
    message: "{identity} 제거",
    description:
      "Accessible name of a member row's remove control, naming the member (w4/150)",
  },
  "team.removeTitle": {
    message: "멤버를 제거할까요?",
    description: "Remove-member confirmation dialog title",
  },
  "team.removeConfirm": {
    message:
      "{identity} 계정은 이 워크스페이스에 대한 접근 권한을 즉시 잃습니다.",
    description: "Remove-member confirmation dialog body",
  },
  "team.removeRevokesKeys": {
    message:
      "{identity} 계정이 여기에서 만든 API 키도 모두 폐기되며, 해당 키로 인증하는 자동화가 중단될 수 있습니다.",
    description:
      "Remove-member dialog warning (w2/m163): removal disposes of the member's machine credentials, so an admin must see the cost before confirming rather than discovering it as an outage",
  },
  "team.removeCancel": {
    message: "취소",
    description: "Remove-member confirmation cancel button",
  },
  "team.removeSuccess": {
    message: "멤버를 제거했습니다",
    description: "Toast after a successful remove",
  },
  "team.removeError": {
    message: "해당 멤버를 제거하지 못했습니다",
    description: "Toast after a failed remove",
  },
  "team.roleChanged": {
    message: "역할을 업데이트했습니다",
    description: "Toast after a successful role change",
  },
  "team.roleChangeError": {
    message: "해당 역할을 변경하지 못했습니다",
    description: "Toast after a failed role change",
  },
  "team.lastAdminError": {
    message: "워크스페이스에는 관리자가 최소 한 명 있어야 합니다.",
    description: "Toast when the last-admin guard refuses a demote/remove",
  },
  "team.revokeInvite": {
    message: "취소",
    description: "Revoke pending-invite button",
  },
  "team.resendInvite": {
    message: "다시 보내기",
    description: "Resend pending-invite button (fresh email, refreshed expiry)",
  },
  "team.resendInviteSuccess": {
    message: "{email}로 초대를 다시 보냈습니다",
    description: "Toast after a successful invite resend",
  },
  "team.resendInviteError": {
    message: "초대를 다시 보내지 못했습니다",
    description: "Toast after a failed invite resend",
  },
  "team.seatUsage": {
    message: "{used}/{limit}석",
    description:
      "Seat usage in the Team card title on a limited plan — accepted members plus pending invites over the plan cap",
  },
  "team.seatCount_other": {
    message: "{count}석 사용 중",
    description:
      "Seat count in the Team card title on an unlimited plan (no cap to show, plural)",
  },
  "team.seatsFullBody": {
    message:
      "이 워크스페이스는 요금제에서 제공하는 모든 좌석을 사용했습니다. 더 많은 멤버를 초대하려면 업그레이드하세요.",
    description:
      "Invite dialog wall when seats are exhausted before composing an invite",
  },
  "team.mfaEnabled": {
    message: "2FA",
    description:
      "Badge on a member row whose account has a second factor enrolled",
  },
  "team.leaveTitle": {
    message: "워크스페이스 나가기",
    description: "Title of the leave-workspace card in workspace settings",
  },
  "team.leaveDescription": {
    message:
      "{workspace} 워크스페이스의 멤버십을 포기합니다. 이 워크스페이스의 서비스와 데이터에 대한 접근 권한을 잃고, 여기에서 만든 API 키는 작동하지 않게 됩니다. 관리자가 다시 초대할 수 있습니다.",
    description: "Description of the leave-workspace card",
  },
  "team.leaveAction": {
    message: "워크스페이스 나가기",
    description: "Label of the leave-workspace button",
  },
  "team.leaveConfirmTitle": {
    message: "이 워크스페이스를 나갈까요?",
    description: "Title of the leave-workspace confirmation dialog",
  },
  "team.leaveConfirm": {
    message:
      "{workspace} 워크스페이스에 대한 접근 권한을 잃고, 여기에서 만든 API 키는 폐기됩니다. 관리자가 초대해야만 다시 참여할 수 있습니다.",
    description: "Body of the leave-workspace confirmation dialog",
  },
  "team.leaveCancel": {
    message: "취소",
    description: "Cancel label in the leave-workspace confirmation dialog",
  },
  "team.leaveErrorTitle": {
    message: "워크스페이스를 나갈 수 없습니다",
    description: "Title of the inline error when leaving is refused",
  },
  "team.leaveError": {
    message: "워크스페이스 나가기에 실패했습니다. 다시 시도하세요.",
    description: "Fallback message when leaving fails without a server reason",
  },
  "team.leaveLastAdminReason": {
    message:
      "마지막 관리자입니다. 워크스페이스에 관리자가 계속 남아 있도록 먼저 다른 사람을 관리자로 지정하세요.",
    description: "Reason the Leave action is disabled for the last admin",
  },
  "team.you": {
    message: "나",
    description: "Badge marking the caller's own row in the members table",
  },
  "team.removeSelfReason": {
    message: "본인은 제거할 수 없습니다. 대신 워크스페이스를 나가세요.",
    description:
      "Reason the Remove control is disabled on the caller's own row",
  },
  "team.changeOwnRoleReason": {
    message:
      "본인의 역할은 변경할 수 없습니다. 다른 관리자에게 변경을 요청하세요.",
    description: "Reason the role picker is disabled on the caller's own row",
  },
  "team.mfaEnabledTooltip": {
    message: "2단계 인증 사용 중",
    description: "Tooltip for the member-row 2FA badge",
  },
  "team.identityUnresolved": {
    message: "확인 불가",
    description:
      "Badge on a member row whose Kratos identity lookup missed (w4/070)",
  },
  "team.identityUnresolvedTooltip": {
    message:
      "이 멤버의 계정을 조회할 수 없습니다. 관리자가 제거하기 전까지 좌석을 계속 차지합니다.",
    description: "Tooltip for the unresolved-identity member badge",
  },
  "team.inviteAccepted": {
    message: "{workspace} 워크스페이스에 참여했습니다",
    description: "Toast after an emailed invite link is redeemed successfully",
  },
  "team.inviteAcceptedAlready": {
    message: "이미 사용된 초대입니다",
    description: "Toast when an invite token was already redeemed",
  },
  "team.inviteAcceptExpired": {
    message: "초대가 만료되었습니다. 새 초대를 요청하세요",
    description: "Toast when an invite token is past its expiry",
  },
  "team.inviteAcceptError": {
    message: "초대를 수락하지 못했습니다",
    description: "Toast when redeeming an invite token fails",
  },
  "team.revokeInviteSuccess": {
    message: "초대를 취소했습니다",
    description: "Toast after a successful invite revoke",
  },
  "team.revokeInviteError": {
    message: "초대를 취소하지 못했습니다",
    description: "Toast after a failed invite revoke",
  },
  "team.errorTitle": {
    message: "팀을 불러오지 못했습니다",
    description: "Team panel generic error title",
  },
  "team.errorBody": {
    message:
      "이 워크스페이스의 멤버를 불러오는 중 문제가 발생했습니다. 다시 시도하세요.",
    description: "Team panel generic error body",
  },
  "team.role.VIEWER": {
    message: "뷰어",
    description: "Role label — read-only",
  },
  "team.role.CONTRIBUTOR": {
    message: "기여자",
    description: "Role label — works on resources, no sensitive fields",
  },
  "team.role.DEVELOPER": {
    message: "개발자",
    description: "Role label — full resource access",
  },
  "team.role.ADMIN": {
    message: "관리자",
    description: "Role label — full access incl. settings, members, billing",
  },
  "team.role.BILLING": {
    message: "결제 담당자",
    description: "Role label — billing only",
  },
  "team.twoFactorRequiredNotEnabled": {
    message: "2단계 인증 필요, 미사용",
    description:
      "Badge on a member row while the workspace requires 2FA and the member has none (w22/m402)",
  },
  "team.twoFactorRequiredNotEnabledTooltip": {
    message:
      "이 워크스페이스는 2단계 인증을 요구합니다. 이 멤버는 2FA를 사용 설정하기 전에는 접근할 수 없습니다.",
    description: "Tooltip for the required-but-not-enabled member badge",
  },
};

export default koTeam;
