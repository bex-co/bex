import type { TranslationEntry } from "@/i18n";

const jaTeam: Record<string, TranslationEntry> = {
  "team.title": {
    message: "チーム",
    description: "Settings Team section card title",
  },
  "team.description": {
    message:
      "このワークスペースのメンバーと、それぞれができる操作です。メールでチームメンバーを招待してロールを割り当てます。ロールはすべてのリソースに適用されます。",
    description: "Settings Team section card description",
  },
  "team.searchLabel": {
    message: "メンバーを検索",
    description: "Accessible label for the accepted-member search field",
  },
  "team.searchPlaceholder": {
    message: "メンバーを検索",
    description: "Placeholder for the accepted-member search field",
  },
  "team.emptyTitle": {
    message: "メンバーはまだいません",
    description: "Accepted-member table empty-state title",
  },
  "team.emptyBody": {
    message:
      "このワークスペースで共同作業を始めるには、メンバーを招待してください。",
    description: "Accepted-member table empty-state description",
  },
  "team.noMatchesTitle": {
    message: "一致するメンバーがいません",
    description: "Accepted-member search no-results title",
  },
  "team.noMatchesBody": {
    message: "別のメールアドレスまたは ID でお試しください。",
    description: "Accepted-member search no-results description",
  },
  "team.colMember": {
    message: "メンバー",
    description: "Team table column header — the member identity",
  },
  "team.colRole": {
    message: "ロール",
    description: "Team table column header — the member's role",
  },
  "team.colEmail": {
    message: "メールアドレス",
    description: "Pending invites table column header",
  },
  "team.pendingTitle": {
    message: "保留中の招待",
    description: "Heading above the pending-invites table",
  },
  "team.invite": {
    message: "招待",
    description: "Button that opens the invite dialog",
  },
  "team.inviteTitle": {
    message: "チームメンバーを招待",
    description: "Invite dialog title",
  },
  "team.inviteDescription": {
    message:
      "相手にメールが送信され、そのアドレスで初めてログインしたときにこのワークスペースに参加します。",
    description: "Invite dialog description",
  },
  "team.fieldEmail": {
    message: "メールアドレス",
    description: "Invite dialog email field label",
  },
  "team.fieldEmailPlaceholder": {
    message: "teammate@example.com",
    description: "Invite dialog email field placeholder",
  },
  "team.fieldEmailInvalid": {
    message: "有効なメールアドレスを入力してください。例: teammate@example.com",
    description: "Invite dialog validation message for a malformed email",
  },
  "team.fieldRole": {
    message: "ロール",
    description: "Invite dialog role field label",
  },
  "team.inviteCancel": {
    message: "キャンセル",
    description: "Invite dialog cancel button",
  },
  "team.inviteSubmit": {
    message: "招待を送信",
    description: "Invite dialog submit button",
  },
  "team.inviteSuccess": {
    message: "{email} に招待を送信しました",
    description: "Toast after a successful invite",
  },
  "team.inviteError": {
    message: "{email} を招待できませんでした",
    description: "Toast after a failed invite",
  },
  "team.inviteErrorPlanTitle": {
    message: "お使いのプランではこの招待を受け付けられません",
    description:
      "Title of the inline alert when the workspace plan's member cap or role set blocks an invite",
  },
  "team.inviteErrorPlanCta": {
    message: "プランを変更",
    description:
      "Link from the blocked-invite alert to the workspace settings plan section",
  },
  "team.inviteErrorPlanLimitSeats": {
    message:
      "{plan} プランのワークスペースメンバーは {limit} 人までです。さらに招待するにはアップグレードしてください。",
    description:
      "Plan seat-cap refusal in the invite dialog — shown when accepted members + pending invites reach the plan maximum",
  },
  "team.inviteErrorPlanLimitRole": {
    message:
      "{plan} プランではこのロールを利用できません。すべてのロールを利用するにはアップグレードしてください。",
    description:
      "Plan role-gate refusal in the invite dialog — shown when the chosen role isn't available on the current plan",
  },
  "team.remove": {
    message: "削除",
    description: "Remove-member button / confirm label",
  },
  "team.roleFor": {
    message: "{identity} のロール",
    description:
      "Accessible name of a member row's role picker, naming the member (w4/150)",
  },
  "team.removeMember": {
    message: "{identity} を削除",
    description:
      "Accessible name of a member row's remove control, naming the member (w4/150)",
  },
  "team.removeTitle": {
    message: "メンバーを削除しますか？",
    description: "Remove-member confirmation dialog title",
  },
  "team.removeConfirm": {
    message:
      "{identity} は、このワークスペースにただちにアクセスできなくなります。",
    description: "Remove-member confirmation dialog body",
  },
  "team.removeRevokesKeys": {
    message:
      "{identity} がここで作成した API キーもすべて無効になります。これらで認証しているオートメーションが動作しなくなる可能性があります。",
    description:
      "Remove-member dialog warning (w2/m163): removal disposes of the member's machine credentials, so an admin must see the cost before confirming rather than discovering it as an outage",
  },
  "team.removeCancel": {
    message: "キャンセル",
    description: "Remove-member confirmation cancel button",
  },
  "team.removeSuccess": {
    message: "メンバーを削除しました",
    description: "Toast after a successful remove",
  },
  "team.removeError": {
    message: "そのメンバーを削除できませんでした",
    description: "Toast after a failed remove",
  },
  "team.roleChanged": {
    message: "ロールを更新しました",
    description: "Toast after a successful role change",
  },
  "team.roleChangeError": {
    message: "そのロールを変更できませんでした",
    description: "Toast after a failed role change",
  },
  "team.lastAdminError": {
    message: "ワークスペースには、少なくとも 1 人の管理者が必要です。",
    description: "Toast when the last-admin guard refuses a demote/remove",
  },
  "team.revokeInvite": {
    message: "取り消す",
    description: "Revoke pending-invite button",
  },
  "team.resendInvite": {
    message: "再送信",
    description: "Resend pending-invite button (fresh email, refreshed expiry)",
  },
  "team.resendInviteSuccess": {
    message: "{email} に招待を再送信しました",
    description: "Toast after a successful invite resend",
  },
  "team.resendInviteError": {
    message: "その招待を再送信できませんでした",
    description: "Toast after a failed invite resend",
  },
  "team.seatUsage": {
    message: "{used} / {limit} シート",
    description:
      "Seat usage in the Team card title on a limited plan — accepted members plus pending invites over the plan cap",
  },
  "team.seatCount_other": {
    message: "{count} シート使用中",
    description:
      "Seat count in the Team card title on an unlimited plan (no cap to show, plural)",
  },
  "team.seatsFullBody": {
    message:
      "このワークスペースは、プランで利用できるシートをすべて使用しています。さらにメンバーを招待するにはアップグレードしてください。",
    description:
      "Invite dialog wall when seats are exhausted before composing an invite",
  },
  "team.mfaEnabled": {
    message: "2FA",
    description:
      "Badge on a member row whose account has a second factor enrolled",
  },
  "team.leaveTitle": {
    message: "ワークスペースから退出",
    description: "Title of the leave-workspace card in workspace settings",
  },
  "team.leaveDescription": {
    message:
      "{workspace} のメンバーシップを放棄します。サービスとデータにアクセスできなくなり、ここで作成した API キーも動作しなくなります。管理者は、あなたをもう一度招待できます。",
    description: "Description of the leave-workspace card",
  },
  "team.leaveAction": {
    message: "ワークスペースから退出",
    description: "Label of the leave-workspace button",
  },
  "team.leaveConfirmTitle": {
    message: "このワークスペースから退出しますか？",
    description: "Title of the leave-workspace confirmation dialog",
  },
  "team.leaveConfirm": {
    message:
      "{workspace} にアクセスできなくなり、ここで作成した API キーは無効になります。再び参加するには、管理者からの招待が必要です。",
    description: "Body of the leave-workspace confirmation dialog",
  },
  "team.leaveCancel": {
    message: "キャンセル",
    description: "Cancel label in the leave-workspace confirmation dialog",
  },
  "team.leaveErrorTitle": {
    message: "ワークスペースから退出できませんでした",
    description: "Title of the inline error when leaving is refused",
  },
  "team.leaveError": {
    message: "ワークスペースからの退出に失敗しました。もう一度お試しください。",
    description: "Fallback message when leaving fails without a server reason",
  },
  "team.leaveLastAdminReason": {
    message:
      "あなたは最後の管理者です。ワークスペースに管理者が残るよう、先に別のメンバーを管理者にしてください。",
    description: "Reason the Leave action is disabled for the last admin",
  },
  "team.you": {
    message: "あなた",
    description: "Badge marking the caller's own row in the members table",
  },
  "team.removeSelfReason": {
    message:
      "自分自身を削除することはできません。代わりにワークスペースから退出してください。",
    description:
      "Reason the Remove control is disabled on the caller's own row",
  },
  "team.changeOwnRoleReason": {
    message:
      "自分のロールは変更できません。別の管理者に変更を依頼してください。",
    description: "Reason the role picker is disabled on the caller's own row",
  },
  "team.mfaEnabledTooltip": {
    message: "二要素認証が有効です",
    description: "Tooltip for the member-row 2FA badge",
  },
  "team.identityUnresolved": {
    message: "未解決",
    description:
      "Badge on a member row whose Kratos identity lookup missed (w4/070)",
  },
  "team.identityUnresolvedTooltip": {
    message:
      "このメンバーのアカウントを照会できませんでした。管理者が削除するまで、シートは引き続き使用されます。",
    description: "Tooltip for the unresolved-identity member badge",
  },
  "team.inviteAccepted": {
    message: "{workspace} に参加しました",
    description: "Toast after an emailed invite link is redeemed successfully",
  },
  "team.inviteAcceptedAlready": {
    message: "その招待はすでに使用されています",
    description: "Toast when an invite token was already redeemed",
  },
  "team.inviteAcceptExpired": {
    message: "その招待は有効期限が切れています。新しい招待を依頼してください",
    description: "Toast when an invite token is past its expiry",
  },
  "team.inviteAcceptError": {
    message: "その招待を承諾できませんでした",
    description: "Toast when redeeming an invite token fails",
  },
  "team.revokeInviteSuccess": {
    message: "招待を取り消しました",
    description: "Toast after a successful invite revoke",
  },
  "team.revokeInviteError": {
    message: "その招待を取り消せませんでした",
    description: "Toast after a failed invite revoke",
  },
  "team.errorTitle": {
    message: "チームを読み込めませんでした",
    description: "Team panel generic error title",
  },
  "team.errorBody": {
    message:
      "このワークスペースのメンバーの読み込み中に問題が発生しました。もう一度お試しください。",
    description: "Team panel generic error body",
  },
  "team.role.VIEWER": {
    message: "閲覧者",
    description: "Role label — read-only",
  },
  "team.role.CONTRIBUTOR": {
    message: "コントリビューター",
    description: "Role label — works on resources, no sensitive fields",
  },
  "team.role.DEVELOPER": {
    message: "開発者",
    description: "Role label — full resource access",
  },
  "team.role.ADMIN": {
    message: "管理者",
    description: "Role label — full access incl. settings, members, billing",
  },
  "team.role.BILLING": {
    message: "請求担当",
    description: "Role label — billing only",
  },
  "team.twoFactorRequiredNotEnabled": {
    message: "2FA が必要、未有効",
    description:
      "Badge on a member row while the workspace requires 2FA and the member has none (w22/m402)",
  },
  "team.twoFactorRequiredNotEnabledTooltip": {
    message:
      "このワークスペースでは二要素認証が必須です。このメンバーは、2FA を有効にするまでアクセスできません。",
    description: "Tooltip for the required-but-not-enabled member badge",
  },
};

export default jaTeam;
