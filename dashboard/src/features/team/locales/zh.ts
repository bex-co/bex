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
  "team.inviteErrorPlan": {
    message: "已达到当前套餐的成员上限——升级后可邀请更多成员。",
    description: "Toast when the workspace plan's member cap blocks an invite",
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
