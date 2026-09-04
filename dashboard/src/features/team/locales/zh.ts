import type { TranslationEntry } from "@/i18n";

const zhTeam: Record<string, TranslationEntry> = {
  "team.title": {
    message: "团队",
    description: "Settings Team section card title",
  },
  "team.description": {
    message:
      "此工作区中的成员及其权限。通过邮箱邀请队友并分配角色；角色会在所有资源上强制执行。",
    description: "Settings Team section card description",
  },
  "team.searchLabel": {
    message: "搜索成员",
    description: "Accessible label for the accepted-member search field",
  },
  "team.searchPlaceholder": {
    message: "搜索成员",
    description: "Placeholder for the accepted-member search field",
  },
  "team.emptyTitle": {
    message: "暂无成员",
    description: "Accepted-member table empty-state title",
  },
  "team.emptyBody": {
    message: "邀请成员加入此工作区开始协作。",
    description: "Accepted-member table empty-state description",
  },
  "team.noMatchesTitle": {
    message: "没有匹配的成员",
    description: "Accepted-member search no-results title",
  },
  "team.noMatchesBody": {
    message: "请尝试其他邮箱或身份信息。",
    description: "Accepted-member search no-results description",
  },
  "team.colMember": {
    message: "成员",
    description: "Team table column header — the member identity",
  },
  "team.colRole": {
    message: "角色",
    description: "Team table column header — the member's role",
  },
  "team.colEmail": {
    message: "邮箱",
    description: "Pending invites table column header",
  },
  "team.pendingTitle": {
    message: "待接受的邀请",
    description: "Heading above the pending-invites table",
  },
  "team.invite": {
    message: "邀请",
    description: "Button that opens the invite dialog",
  },
  "team.inviteTitle": {
    message: "邀请队友",
    description: "Invite dialog title",
  },
  "team.inviteDescription": {
    message: "他们将收到一封邮件，并在首次使用该邮箱登录时加入此工作区。",
    description: "Invite dialog description",
  },
  "team.fieldEmail": {
    message: "邮箱地址",
    description: "Invite dialog email field label",
  },
  "team.fieldEmailPlaceholder": {
    message: "teammate@example.com",
    description: "Invite dialog email field placeholder",
  },
  "team.fieldEmailInvalid": {
    message: "请输入有效的邮箱地址，例如 teammate@example.com。",
    description: "Invite dialog validation message for a malformed email",
  },
  "team.fieldRole": {
    message: "角色",
    description: "Invite dialog role field label",
  },
  "team.inviteCancel": {
    message: "取消",
    description: "Invite dialog cancel button",
  },
  "team.inviteSubmit": {
    message: "发送邀请",
    description: "Invite dialog submit button",
  },
  "team.inviteSuccess": {
    message: "已向 {email} 发送邀请",
    description: "Toast after a successful invite",
  },
  "team.inviteError": {
    message: "无法邀请 {email}",
    description: "Toast after a failed invite",
  },
  "team.inviteErrorPlanTitle": {
    message: "当前套餐无法接受此邀请",
    description:
      "Title of the inline alert when the workspace plan's member cap or role set blocks an invite",
  },
  "team.inviteErrorPlanCta": {
    message: "更改套餐",
    description:
      "Link from the blocked-invite alert to the workspace settings plan section",
  },
  "team.inviteErrorPlanLimitSeats": {
    message: "{plan} 套餐最多支持 {limit} 名工作区成员，升级以邀请更多人。",
    description:
      "Plan seat-cap refusal in the invite dialog — shown when accepted members + pending invites reach the plan maximum",
  },
  "team.inviteErrorPlanLimitRole": {
    message: "{plan} 套餐不支持此角色，升级以使用所有角色。",
    description:
      "Plan role-gate refusal in the invite dialog — shown when the chosen role isn't available on the current plan",
  },
  "team.remove": {
    message: "移除",
    description: "Remove-member button / confirm label",
  },
  "team.removeTitle": {
    message: "移除成员？",
    description: "Remove-member confirmation dialog title",
  },
  "team.removeConfirm": {
    message: "{identity} 将立即失去对此工作区的访问权限。",
    description: "Remove-member confirmation dialog body",
  },
  "team.removeCancel": {
    message: "取消",
    description: "Remove-member confirmation cancel button",
  },
  "team.removeSuccess": {
    message: "已移除成员",
    description: "Toast after a successful remove",
  },
  "team.removeError": {
    message: "无法移除该成员",
    description: "Toast after a failed remove",
  },
  "team.roleChanged": {
    message: "角色已更新",
    description: "Toast after a successful role change",
  },
  "team.roleChangeError": {
    message: "无法更改该角色",
    description: "Toast after a failed role change",
  },
  "team.lastAdminError": {
    message: "工作区必须至少保留一名管理员。",
    description: "Toast when the last-admin guard refuses a demote/remove",
  },
  "team.revokeInvite": {
    message: "撤销",
    description: "Revoke pending-invite button",
  },
  "team.resendInvite": {
    message: "重发",
    description: "Resend pending-invite button (fresh email, refreshed expiry)",
  },
  "team.resendInviteSuccess": {
    message: "已重新发送邀请至 {email}",
    description: "Toast after a successful invite resend",
  },
  "team.resendInviteError": {
    message: "无法重新发送该邀请",
    description: "Toast after a failed invite resend",
  },
  "team.seatUsage": {
    message: "已用 {used} / {limit} 席位",
    description:
      "Seat usage in the Team card title on a limited plan — accepted members plus pending invites over the plan cap",
  },
  "team.seatCount_other": {
    message: "已用 {count} 个席位",
    description:
      "Seat count in the Team card title on an unlimited plan (no cap to show)",
  },
  "team.seatsFullBody": {
    message: "此工作区已用完当前套餐的全部席位。升级套餐以邀请更多成员。",
    description:
      "Invite dialog wall when seats are exhausted before composing an invite",
  },
  "team.mfaEnabled": {
    message: "两步验证",
    description:
      "Badge on a member row whose account has a second factor enrolled",
  },
  "team.mfaEnabledTooltip": {
    message: "已启用两步验证",
    description: "Tooltip for the member-row 2FA badge",
  },
  "team.inviteAccepted": {
    message: "你已加入 {workspace}",
    description: "Toast after an emailed invite link is redeemed successfully",
  },
  "team.inviteAcceptedAlready": {
    message: "该邀请已被使用",
    description: "Toast when an invite token was already redeemed",
  },
  "team.inviteAcceptExpired": {
    message: "该邀请已过期——请索取新的邀请",
    description: "Toast when an invite token is past its expiry",
  },
  "team.inviteAcceptError": {
    message: "无法接受该邀请",
    description: "Toast when redeeming an invite token fails",
  },
  "team.inviteConfirmTitle": {
    message: "加入此工作区？",
    description: "Title for the explicit invite-acceptance banner",
  },
  "team.inviteConfirmDescription": {
    message: "有人邀请你加入一个工作区。仅在你确认该邀请时再接受。",
    description: "Body for the explicit invite-acceptance banner",
  },
  "team.inviteConfirmAccept": {
    message: "接受邀请",
    description: "Primary button on the invite-acceptance banner",
  },
  "team.inviteConfirmDecline": {
    message: "拒绝",
    description: "Secondary button that clears a pending invite",
  },
  "team.revokeInviteSuccess": {
    message: "已撤销邀请",
    description: "Toast after a successful invite revoke",
  },
  "team.revokeInviteError": {
    message: "无法撤销该邀请",
    description: "Toast after a failed invite revoke",
  },
  "team.errorTitle": {
    message: "无法加载团队",
    description: "Team panel generic error title",
  },
  "team.errorBody": {
    message: "加载此工作区成员时出错，请重试。",
    description: "Team panel generic error body",
  },
  "team.role.VIEWER": {
    message: "查看者",
    description: "Role label — read-only",
  },
  "team.role.CONTRIBUTOR": {
    message: "贡献者",
    description: "Role label — works on resources, no sensitive fields",
  },
  "team.role.DEVELOPER": {
    message: "开发者",
    description: "Role label — full resource access",
  },
  "team.role.ADMIN": {
    message: "管理员",
    description: "Role label — full access incl. settings, members, billing",
  },
  "team.role.BILLING": {
    message: "账单",
    description: "Role label — billing only",
  },
};

export default zhTeam;
