import type { TranslationEntry } from "@/i18n";

const ptTeam: Record<string, TranslationEntry> = {
  "team.title": {
    message: "Equipe",
    description: "Settings Team section card title",
  },
  "team.description": {
    message:
      "As pessoas deste workspace e o que elas podem fazer. Convide colegas por e-mail e atribua uma função; as funções são aplicadas em todos os recursos.",
    description: "Settings Team section card description",
  },
  "team.searchLabel": {
    message: "Buscar membros",
    description: "Accessible label for the accepted-member search field",
  },
  "team.searchPlaceholder": {
    message: "Buscar membros",
    description: "Placeholder for the accepted-member search field",
  },
  "team.emptyTitle": {
    message: "Ainda não há membros",
    description: "Accepted-member table empty-state title",
  },
  "team.emptyBody": {
    message: "Convide alguém para começar a colaborar neste workspace.",
    description: "Accepted-member table empty-state description",
  },
  "team.noMatchesTitle": {
    message: "Nenhum membro correspondente",
    description: "Accepted-member search no-results title",
  },
  "team.noMatchesBody": {
    message: "Tente outro e-mail ou identidade.",
    description: "Accepted-member search no-results description",
  },
  "team.colMember": {
    message: "Membro",
    description: "Team table column header — the member identity",
  },
  "team.colRole": {
    message: "Função",
    description: "Team table column header — the member's role",
  },
  "team.colEmail": {
    message: "E-mail",
    description: "Pending invites table column header",
  },
  "team.pendingTitle": {
    message: "Convites pendentes",
    description: "Heading above the pending-invites table",
  },
  "team.invite": {
    message: "Convidar",
    description: "Button that opens the invite dialog",
  },
  "team.inviteTitle": {
    message: "Convidar um colega",
    description: "Invite dialog title",
  },
  "team.inviteDescription": {
    message:
      "A pessoa receberá um e-mail e entrará neste workspace na primeira vez que fizer login com esse endereço.",
    description: "Invite dialog description",
  },
  "team.fieldEmail": {
    message: "Endereço de e-mail",
    description: "Invite dialog email field label",
  },
  "team.fieldEmailPlaceholder": {
    message: "teammate@example.com",
    description: "Invite dialog email field placeholder",
  },
  "team.fieldEmailInvalid": {
    message: "Informe um endereço de e-mail válido, ex.: teammate@example.com.",
    description: "Invite dialog validation message for a malformed email",
  },
  "team.fieldRole": {
    message: "Função",
    description: "Invite dialog role field label",
  },
  "team.inviteCancel": {
    message: "Cancelar",
    description: "Invite dialog cancel button",
  },
  "team.inviteSubmit": {
    message: "Enviar convite",
    description: "Invite dialog submit button",
  },
  "team.inviteSuccess": {
    message: "Convite enviado para {email}",
    description: "Toast after a successful invite",
  },
  "team.inviteError": {
    message: "Não foi possível convidar {email}",
    description: "Toast after a failed invite",
  },
  "team.inviteErrorPlanTitle": {
    message: "Seu plano não comporta este convite",
    description:
      "Title of the inline alert when the workspace plan's member cap or role set blocks an invite",
  },
  "team.inviteErrorPlanCta": {
    message: "Alterar plano",
    description:
      "Link from the blocked-invite alert to the workspace settings plan section",
  },
  "team.inviteErrorPlanLimitSeats": {
    message:
      "O plano {plan} é limitado a {limit} membro(s) do workspace. Faça upgrade para convidar mais.",
    description:
      "Plan seat-cap refusal in the invite dialog — shown when accepted members + pending invites reach the plan maximum",
  },
  "team.inviteErrorPlanLimitRole": {
    message:
      "O plano {plan} não oferece esta função. Faça upgrade para acessar todas as funções.",
    description:
      "Plan role-gate refusal in the invite dialog — shown when the chosen role isn't available on the current plan",
  },
  "team.remove": {
    message: "Remover",
    description: "Remove-member button / confirm label",
  },
  "team.roleFor": {
    message: "Função de {identity}",
    description:
      "Accessible name of a member row's role picker, naming the member (w4/150)",
  },
  "team.removeMember": {
    message: "Remover {identity}",
    description:
      "Accessible name of a member row's remove control, naming the member (w4/150)",
  },
  "team.removeTitle": {
    message: "Remover membro?",
    description: "Remove-member confirmation dialog title",
  },
  "team.removeConfirm": {
    message: "{identity} perderá o acesso a este workspace imediatamente.",
    description: "Remove-member confirmation dialog body",
  },
  "team.removeRevokesKeys": {
    message:
      "Isso também revoga todas as chaves de API que {identity} criou aqui, o que pode quebrar automações que se autenticam com elas.",
    description:
      "Remove-member dialog warning (w2/m163): removal disposes of the member's machine credentials, so an admin must see the cost before confirming rather than discovering it as an outage",
  },
  "team.removeCancel": {
    message: "Cancelar",
    description: "Remove-member confirmation cancel button",
  },
  "team.removeSuccess": {
    message: "Membro removido",
    description: "Toast after a successful remove",
  },
  "team.removeError": {
    message: "Não foi possível remover esse membro",
    description: "Toast after a failed remove",
  },
  "team.roleChanged": {
    message: "Função atualizada",
    description: "Toast after a successful role change",
  },
  "team.roleChangeError": {
    message: "Não foi possível alterar essa função",
    description: "Toast after a failed role change",
  },
  "team.lastAdminError": {
    message: "Um workspace deve manter pelo menos um administrador.",
    description: "Toast when the last-admin guard refuses a demote/remove",
  },
  "team.revokeInvite": {
    message: "Revogar",
    description: "Revoke pending-invite button",
  },
  "team.resendInvite": {
    message: "Reenviar",
    description: "Resend pending-invite button (fresh email, refreshed expiry)",
  },
  "team.resendInviteSuccess": {
    message: "Convite reenviado para {email}",
    description: "Toast after a successful invite resend",
  },
  "team.resendInviteError": {
    message: "Não foi possível reenviar esse convite",
    description: "Toast after a failed invite resend",
  },
  "team.seatUsage": {
    message: "{used} de {limit} vagas",
    description:
      "Seat usage in the Team card title on a limited plan — accepted members plus pending invites over the plan cap",
  },
  "team.seatCount_one": {
    message: "{count} vaga usada",
    description:
      "Seat count in the Team card title on an unlimited plan (no cap to show, singular)",
  },
  "team.seatCount_many": {
    message: "{count} de vagas usadas",
    description:
      "Seat count in the Team card title on an unlimited plan (no cap to show, plural)",
  },
  "team.seatCount_other": {
    message: "{count} vagas usadas",
    description:
      "Seat count in the Team card title on an unlimited plan (no cap to show, plural)",
  },
  "team.seatsFullBody": {
    message:
      "Este workspace usou todas as vagas que o plano oferece. Faça upgrade para convidar mais membros.",
    description:
      "Invite dialog wall when seats are exhausted before composing an invite",
  },
  "team.mfaEnabled": {
    message: "2FA",
    description:
      "Badge on a member row whose account has a second factor enrolled",
  },
  "team.leaveTitle": {
    message: "Sair do workspace",
    description: "Title of the leave-workspace card in workspace settings",
  },
  "team.leaveDescription": {
    message:
      "Abra mão da sua participação em {workspace}. Você perderá o acesso aos serviços e dados dele, e as chaves de API que você criou aqui deixarão de funcionar. Um administrador pode convidar você de volta.",
    description: "Description of the leave-workspace card",
  },
  "team.leaveAction": {
    message: "Sair do workspace",
    description: "Label of the leave-workspace button",
  },
  "team.leaveConfirmTitle": {
    message: "Sair deste workspace?",
    description: "Title of the leave-workspace confirmation dialog",
  },
  "team.leaveConfirm": {
    message:
      "Você perderá o acesso a {workspace} e as chaves de API que você criou nele serão revogadas. Você só poderá voltar se um administrador convidar você.",
    description: "Body of the leave-workspace confirmation dialog",
  },
  "team.leaveCancel": {
    message: "Cancelar",
    description: "Cancel label in the leave-workspace confirmation dialog",
  },
  "team.leaveErrorTitle": {
    message: "Não foi possível sair do workspace",
    description: "Title of the inline error when leaving is refused",
  },
  "team.leaveError": {
    message: "Falha ao sair do workspace. Tente novamente.",
    description: "Fallback message when leaving fails without a server reason",
  },
  "team.leaveLastAdminReason": {
    message:
      "Você é o último administrador. Torne outra pessoa administradora primeiro, para que o workspace continue com um.",
    description: "Reason the Leave action is disabled for the last admin",
  },
  "team.you": {
    message: "Você",
    description: "Badge marking the caller's own row in the members table",
  },
  "team.removeSelfReason": {
    message: "Você não pode remover a si mesmo. Saia do workspace.",
    description:
      "Reason the Remove control is disabled on the caller's own row",
  },
  "team.changeOwnRoleReason": {
    message:
      "Você não pode alterar sua própria função. Peça a outro administrador para alterá-la.",
    description: "Reason the role picker is disabled on the caller's own row",
  },
  "team.mfaEnabledTooltip": {
    message: "Autenticação de dois fatores ativada",
    description: "Tooltip for the member-row 2FA badge",
  },
  "team.identityUnresolved": {
    message: "Não resolvida",
    description:
      "Badge on a member row whose Kratos identity lookup missed (w4/070)",
  },
  "team.identityUnresolvedTooltip": {
    message:
      "Não foi possível consultar a conta deste membro. Ele continua ocupando uma vaga até que um administrador o remova.",
    description: "Tooltip for the unresolved-identity member badge",
  },
  "team.inviteAccepted": {
    message: "Você entrou em {workspace}",
    description: "Toast after an emailed invite link is redeemed successfully",
  },
  "team.inviteAcceptedAlready": {
    message: "Esse convite já foi usado",
    description: "Toast when an invite token was already redeemed",
  },
  "team.inviteAcceptExpired": {
    message: "Esse convite expirou — peça um novo",
    description: "Toast when an invite token is past its expiry",
  },
  "team.inviteAcceptError": {
    message: "Não foi possível aceitar esse convite",
    description: "Toast when redeeming an invite token fails",
  },
  "team.revokeInviteSuccess": {
    message: "Convite revogado",
    description: "Toast after a successful invite revoke",
  },
  "team.revokeInviteError": {
    message: "Não foi possível revogar esse convite",
    description: "Toast after a failed invite revoke",
  },
  "team.errorTitle": {
    message: "Não foi possível carregar a equipe",
    description: "Team panel generic error title",
  },
  "team.errorBody": {
    message:
      "Algo deu errado ao carregar os membros deste workspace. Tente novamente.",
    description: "Team panel generic error body",
  },
  "team.role.VIEWER": {
    message: "Visualizador",
    description: "Role label — read-only",
  },
  "team.role.CONTRIBUTOR": {
    message: "Colaborador",
    description: "Role label — works on resources, no sensitive fields",
  },
  "team.role.DEVELOPER": {
    message: "Desenvolvedor",
    description: "Role label — full resource access",
  },
  "team.role.ADMIN": {
    message: "Administrador",
    description: "Role label — full access incl. settings, members, billing",
  },
  "team.role.BILLING": {
    message: "Faturamento",
    description: "Role label — billing only",
  },
  "team.twoFactorRequiredNotEnabled": {
    message: "2FA exigida, não ativada",
    description:
      "Badge on a member row while the workspace requires 2FA and the member has none (w22/m402)",
  },
  "team.twoFactorRequiredNotEnabledTooltip": {
    message:
      "Este workspace exige autenticação em duas etapas. Este membro não pode acessá-lo até ativar a 2FA.",
    description: "Tooltip for the required-but-not-enabled member badge",
  },
};

export default ptTeam;
