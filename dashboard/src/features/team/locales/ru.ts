import type { TranslationEntry } from "@/i18n";

const ruTeam: Record<string, TranslationEntry> = {
  "team.title": {
    message: "Команда",
    description: "Settings Team section card title",
  },
  "team.description": {
    message:
      "Люди в этом рабочем пространстве и их возможности. Приглашайте коллег по email и назначайте роль; роли действуют для всех ресурсов.",
    description: "Settings Team section card description",
  },
  "team.searchLabel": {
    message: "Поиск участников",
    description: "Accessible label for the accepted-member search field",
  },
  "team.searchPlaceholder": {
    message: "Поиск участников",
    description: "Placeholder for the accepted-member search field",
  },
  "team.emptyTitle": {
    message: "Участников пока нет",
    description: "Accepted-member table empty-state title",
  },
  "team.emptyBody": {
    message:
      "Пригласите кого-нибудь, чтобы начать совместную работу в этом рабочем пространстве.",
    description: "Accepted-member table empty-state description",
  },
  "team.noMatchesTitle": {
    message: "Нет подходящих участников",
    description: "Accepted-member search no-results title",
  },
  "team.noMatchesBody": {
    message: "Попробуйте другой email или идентификатор.",
    description: "Accepted-member search no-results description",
  },
  "team.colMember": {
    message: "Участник",
    description: "Team table column header — the member identity",
  },
  "team.colRole": {
    message: "Роль",
    description: "Team table column header — the member's role",
  },
  "team.colEmail": {
    message: "Email",
    description: "Pending invites table column header",
  },
  "team.pendingTitle": {
    message: "Ожидающие приглашения",
    description: "Heading above the pending-invites table",
  },
  "team.invite": {
    message: "Пригласить",
    description: "Button that opens the invite dialog",
  },
  "team.inviteTitle": {
    message: "Пригласить коллегу",
    description: "Invite dialog title",
  },
  "team.inviteDescription": {
    message:
      "Он получит письмо и присоединится к этому рабочему пространству при первом входе с этим адресом.",
    description: "Invite dialog description",
  },
  "team.fieldEmail": {
    message: "Адрес email",
    description: "Invite dialog email field label",
  },
  "team.fieldEmailPlaceholder": {
    message: "teammate@example.com",
    description: "Invite dialog email field placeholder",
  },
  "team.fieldEmailInvalid": {
    message: "Введите корректный адрес email, напр. teammate@example.com.",
    description: "Invite dialog validation message for a malformed email",
  },
  "team.fieldRole": {
    message: "Роль",
    description: "Invite dialog role field label",
  },
  "team.inviteCancel": {
    message: "Отмена",
    description: "Invite dialog cancel button",
  },
  "team.inviteSubmit": {
    message: "Отправить приглашение",
    description: "Invite dialog submit button",
  },
  "team.inviteSuccess": {
    message: "Приглашение отправлено на {email}",
    description: "Toast after a successful invite",
  },
  "team.inviteError": {
    message: "Не удалось пригласить {email}",
    description: "Toast after a failed invite",
  },
  "team.inviteErrorPlanTitle": {
    message: "Ваш тариф не позволяет отправить это приглашение",
    description:
      "Title of the inline alert when the workspace plan's member cap or role set blocks an invite",
  },
  "team.inviteErrorPlanCta": {
    message: "Сменить тариф",
    description:
      "Link from the blocked-invite alert to the workspace settings plan section",
  },
  "team.inviteErrorPlanLimitSeats": {
    message:
      "На тарифе {plan} доступно не более {limit} участников рабочего пространства. Перейдите на более высокий тариф, чтобы приглашать больше людей.",
    description:
      "Plan seat-cap refusal in the invite dialog — shown when accepted members + pending invites reach the plan maximum",
  },
  "team.inviteErrorPlanLimitRole": {
    message:
      "На тарифе {plan} эта роль недоступна. Перейдите на более высокий тариф, чтобы получить все роли.",
    description:
      "Plan role-gate refusal in the invite dialog — shown when the chosen role isn't available on the current plan",
  },
  "team.remove": {
    message: "Удалить",
    description: "Remove-member button / confirm label",
  },
  "team.roleFor": {
    message: "Роль: {identity}",
    description:
      "Accessible name of a member row's role picker, naming the member (w4/150)",
  },
  "team.removeMember": {
    message: "Удалить {identity}",
    description:
      "Accessible name of a member row's remove control, naming the member (w4/150)",
  },
  "team.removeTitle": {
    message: "Удалить участника?",
    description: "Remove-member confirmation dialog title",
  },
  "team.removeConfirm": {
    message:
      "{identity} немедленно потеряет доступ к этому рабочему пространству.",
    description: "Remove-member confirmation dialog body",
  },
  "team.removeRevokesKeys": {
    message:
      "Также будут отозваны все API-ключи, созданные здесь пользователем {identity}, — это может нарушить работу автоматизации, которая использует их для аутентификации.",
    description:
      "Remove-member dialog warning (w2/m163): removal disposes of the member's machine credentials, so an admin must see the cost before confirming rather than discovering it as an outage",
  },
  "team.removeCancel": {
    message: "Отмена",
    description: "Remove-member confirmation cancel button",
  },
  "team.removeSuccess": {
    message: "Участник удалён",
    description: "Toast after a successful remove",
  },
  "team.removeError": {
    message: "Не удалось удалить этого участника",
    description: "Toast after a failed remove",
  },
  "team.roleChanged": {
    message: "Роль обновлена",
    description: "Toast after a successful role change",
  },
  "team.roleChangeError": {
    message: "Не удалось изменить эту роль",
    description: "Toast after a failed role change",
  },
  "team.lastAdminError": {
    message:
      "В рабочем пространстве должен остаться хотя бы один администратор.",
    description: "Toast when the last-admin guard refuses a demote/remove",
  },
  "team.revokeInvite": {
    message: "Отозвать",
    description: "Revoke pending-invite button",
  },
  "team.resendInvite": {
    message: "Отправить повторно",
    description: "Resend pending-invite button (fresh email, refreshed expiry)",
  },
  "team.resendInviteSuccess": {
    message: "Приглашение повторно отправлено на {email}",
    description: "Toast after a successful invite resend",
  },
  "team.resendInviteError": {
    message: "Не удалось повторно отправить это приглашение",
    description: "Toast after a failed invite resend",
  },
  "team.seatUsage": {
    message: "{used} из {limit} мест",
    description:
      "Seat usage in the Team card title on a limited plan — accepted members plus pending invites over the plan cap",
  },
  "team.seatCount_one": {
    message: "{count} место занято",
    description:
      "Seat count in the Team card title on an unlimited plan (no cap to show, singular)",
  },
  "team.seatCount_few": {
    message: "{count} места занято",
    description:
      "Seat count in the Team card title on an unlimited plan (no cap to show, plural)",
  },
  "team.seatCount_many": {
    message: "{count} мест занято",
    description:
      "Seat count in the Team card title on an unlimited plan (no cap to show, plural)",
  },
  "team.seatCount_other": {
    message: "{count} места занято",
    description:
      "Seat count in the Team card title on an unlimited plan (no cap to show, plural)",
  },
  "team.seatsFullBody": {
    message:
      "В этом рабочем пространстве заняты все места, предусмотренные тарифом. Перейдите на более высокий тариф, чтобы приглашать больше участников.",
    description:
      "Invite dialog wall when seats are exhausted before composing an invite",
  },
  "team.mfaEnabled": {
    message: "2FA",
    description:
      "Badge on a member row whose account has a second factor enrolled",
  },
  "team.leaveTitle": {
    message: "Покинуть рабочее пространство",
    description: "Title of the leave-workspace card in workspace settings",
  },
  "team.leaveDescription": {
    message:
      "Откажитесь от участия в {workspace}. Вы потеряете доступ к его сервисам и данным, а созданные вами здесь API-ключи перестанут работать. Администратор сможет пригласить вас снова.",
    description: "Description of the leave-workspace card",
  },
  "team.leaveAction": {
    message: "Покинуть рабочее пространство",
    description: "Label of the leave-workspace button",
  },
  "team.leaveConfirmTitle": {
    message: "Покинуть это рабочее пространство?",
    description: "Title of the leave-workspace confirmation dialog",
  },
  "team.leaveConfirm": {
    message:
      "Вы потеряете доступ к {workspace}, а созданные в нём API-ключи будут отозваны. Вернуться можно только по приглашению администратора.",
    description: "Body of the leave-workspace confirmation dialog",
  },
  "team.leaveCancel": {
    message: "Отмена",
    description: "Cancel label in the leave-workspace confirmation dialog",
  },
  "team.leaveErrorTitle": {
    message: "Не удалось покинуть рабочее пространство",
    description: "Title of the inline error when leaving is refused",
  },
  "team.leaveError": {
    message: "Не удалось покинуть рабочее пространство. Повторите попытку.",
    description: "Fallback message when leaving fails without a server reason",
  },
  "team.leaveLastAdminReason": {
    message:
      "Вы последний администратор. Сначала назначьте администратором кого-то другого, чтобы в рабочем пространстве остался администратор.",
    description: "Reason the Leave action is disabled for the last admin",
  },
  "team.you": {
    message: "Вы",
    description: "Badge marking the caller's own row in the members table",
  },
  "team.removeSelfReason": {
    message:
      "Нельзя удалить самого себя. Вместо этого покиньте рабочее пространство.",
    description:
      "Reason the Remove control is disabled on the caller's own row",
  },
  "team.changeOwnRoleReason": {
    message:
      "Нельзя изменить собственную роль. Попросите другого администратора изменить её.",
    description: "Reason the role picker is disabled on the caller's own row",
  },
  "team.mfaEnabledTooltip": {
    message: "Двухфакторная аутентификация включена",
    description: "Tooltip for the member-row 2FA badge",
  },
  "team.identityUnresolved": {
    message: "Не определён",
    description:
      "Badge on a member row whose Kratos identity lookup missed (w4/070)",
  },
  "team.identityUnresolvedTooltip": {
    message:
      "Не удалось найти учётную запись этого участника. Он занимает место, пока администратор не удалит его.",
    description: "Tooltip for the unresolved-identity member badge",
  },
  "team.inviteAccepted": {
    message: "Вы присоединились к {workspace}",
    description: "Toast after an emailed invite link is redeemed successfully",
  },
  "team.inviteAcceptedAlready": {
    message: "Это приглашение уже использовано",
    description: "Toast when an invite token was already redeemed",
  },
  "team.inviteAcceptExpired": {
    message: "Срок действия приглашения истёк — запросите новое",
    description: "Toast when an invite token is past its expiry",
  },
  "team.inviteAcceptError": {
    message: "Не удалось принять это приглашение",
    description: "Toast when redeeming an invite token fails",
  },
  "team.revokeInviteSuccess": {
    message: "Приглашение отозвано",
    description: "Toast after a successful invite revoke",
  },
  "team.revokeInviteError": {
    message: "Не удалось отозвать это приглашение",
    description: "Toast after a failed invite revoke",
  },
  "team.errorTitle": {
    message: "Не удалось загрузить команду",
    description: "Team panel generic error title",
  },
  "team.errorBody": {
    message:
      "Не удалось загрузить участников этого рабочего пространства. Повторите попытку.",
    description: "Team panel generic error body",
  },
  "team.role.VIEWER": {
    message: "Наблюдатель",
    description: "Role label — read-only",
  },
  "team.role.CONTRIBUTOR": {
    message: "Участник-исполнитель",
    description: "Role label — works on resources, no sensitive fields",
  },
  "team.role.DEVELOPER": {
    message: "Разработчик",
    description: "Role label — full resource access",
  },
  "team.role.ADMIN": {
    message: "Администратор",
    description: "Role label — full access incl. settings, members, billing",
  },
  "team.role.BILLING": {
    message: "Оплата",
    description: "Role label — billing only",
  },
  "team.twoFactorRequiredNotEnabled": {
    message: "Требуется 2FA, не включена",
    description:
      "Badge on a member row while the workspace requires 2FA and the member has none (w22/m402)",
  },
  "team.twoFactorRequiredNotEnabledTooltip": {
    message:
      "Этому рабочему пространству требуется двухфакторная аутентификация. Пока участник не включит 2FA, доступа у него не будет.",
    description: "Tooltip for the required-but-not-enabled member badge",
  },
};

export default ruTeam;
