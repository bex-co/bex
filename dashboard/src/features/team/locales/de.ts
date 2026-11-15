import type { TranslationEntry } from "@/i18n";

const deTeam: Record<string, TranslationEntry> = {
  "team.title": {
    message: "Team",
    description: "Settings Team section card title",
  },
  "team.description": {
    message:
      "Die Personen in diesem Workspace und was sie tun können. Lade Teammitglieder per E-Mail ein und weise ihnen eine Rolle zu; Rollen gelten für jede Ressource.",
    description: "Settings Team section card description",
  },
  "team.searchLabel": {
    message: "Mitglieder suchen",
    description: "Accessible label for the accepted-member search field",
  },
  "team.searchPlaceholder": {
    message: "Mitglieder suchen",
    description: "Placeholder for the accepted-member search field",
  },
  "team.emptyTitle": {
    message: "Noch keine Mitglieder",
    description: "Accepted-member table empty-state title",
  },
  "team.emptyBody": {
    message: "Lade jemanden ein, um in diesem Workspace zusammenzuarbeiten.",
    description: "Accepted-member table empty-state description",
  },
  "team.noMatchesTitle": {
    message: "Keine passenden Mitglieder",
    description: "Accepted-member search no-results title",
  },
  "team.noMatchesBody": {
    message: "Versuche eine andere E-Mail-Adresse oder Identität.",
    description: "Accepted-member search no-results description",
  },
  "team.colMember": {
    message: "Mitglied",
    description: "Team table column header — the member identity",
  },
  "team.colRole": {
    message: "Rolle",
    description: "Team table column header — the member's role",
  },
  "team.colEmail": {
    message: "E-Mail",
    description: "Pending invites table column header",
  },
  "team.pendingTitle": {
    message: "Ausstehende Einladungen",
    description: "Heading above the pending-invites table",
  },
  "team.invite": {
    message: "Einladen",
    description: "Button that opens the invite dialog",
  },
  "team.inviteTitle": {
    message: "Teammitglied einladen",
    description: "Invite dialog title",
  },
  "team.inviteDescription": {
    message:
      "Die Person erhält eine E-Mail und tritt diesem Workspace bei, sobald sie sich zum ersten Mal mit dieser Adresse anmeldet.",
    description: "Invite dialog description",
  },
  "team.fieldEmail": {
    message: "E-Mail-Adresse",
    description: "Invite dialog email field label",
  },
  "team.fieldEmailPlaceholder": {
    message: "teammate@example.com",
    description: "Invite dialog email field placeholder",
  },
  "team.fieldEmailInvalid": {
    message: "Gib eine gültige E-Mail-Adresse ein, z. B. teammate@example.com.",
    description: "Invite dialog validation message for a malformed email",
  },
  "team.fieldRole": {
    message: "Rolle",
    description: "Invite dialog role field label",
  },
  "team.inviteCancel": {
    message: "Abbrechen",
    description: "Invite dialog cancel button",
  },
  "team.inviteSubmit": {
    message: "Einladung senden",
    description: "Invite dialog submit button",
  },
  "team.inviteSuccess": {
    message: "Einladung an {email} gesendet",
    description: "Toast after a successful invite",
  },
  "team.inviteError": {
    message: "{email} konnte nicht eingeladen werden",
    description: "Toast after a failed invite",
  },
  "team.inviteErrorPlanTitle": {
    message: "Dein Tarif lässt diese Einladung nicht zu",
    description:
      "Title of the inline alert when the workspace plan's member cap or role set blocks an invite",
  },
  "team.inviteErrorPlanCta": {
    message: "Tarif ändern",
    description:
      "Link from the blocked-invite alert to the workspace settings plan section",
  },
  "team.inviteErrorPlanLimitSeats": {
    message:
      "Der Tarif {plan} ist auf {limit} Workspace-Mitglied(er) begrenzt. Upgrade, um mehr einzuladen.",
    description:
      "Plan seat-cap refusal in the invite dialog — shown when accepted members + pending invites reach the plan maximum",
  },
  "team.inviteErrorPlanLimitRole": {
    message:
      "Der Tarif {plan} bietet diese Rolle nicht an. Upgrade, um alle Rollen zu nutzen.",
    description:
      "Plan role-gate refusal in the invite dialog — shown when the chosen role isn't available on the current plan",
  },
  "team.remove": {
    message: "Entfernen",
    description: "Remove-member button / confirm label",
  },
  "team.roleFor": {
    message: "Rolle für {identity}",
    description:
      "Accessible name of a member row's role picker, naming the member (w4/150)",
  },
  "team.removeMember": {
    message: "{identity} entfernen",
    description:
      "Accessible name of a member row's remove control, naming the member (w4/150)",
  },
  "team.removeTitle": {
    message: "Mitglied entfernen?",
    description: "Remove-member confirmation dialog title",
  },
  "team.removeConfirm": {
    message: "{identity} verliert sofort den Zugriff auf diesen Workspace.",
    description: "Remove-member confirmation dialog body",
  },
  "team.removeRevokesKeys": {
    message:
      "Dadurch werden auch alle API-Schlüssel widerrufen, die {identity} hier erstellt hat. Automatisierungen, die sich damit authentifizieren, können dadurch ausfallen.",
    description:
      "Remove-member dialog warning (w2/m163): removal disposes of the member's machine credentials, so an admin must see the cost before confirming rather than discovering it as an outage",
  },
  "team.removeCancel": {
    message: "Abbrechen",
    description: "Remove-member confirmation cancel button",
  },
  "team.removeSuccess": {
    message: "Mitglied entfernt",
    description: "Toast after a successful remove",
  },
  "team.removeError": {
    message: "Dieses Mitglied konnte nicht entfernt werden",
    description: "Toast after a failed remove",
  },
  "team.roleChanged": {
    message: "Rolle aktualisiert",
    description: "Toast after a successful role change",
  },
  "team.roleChangeError": {
    message: "Diese Rolle konnte nicht geändert werden",
    description: "Toast after a failed role change",
  },
  "team.lastAdminError": {
    message: "Ein Workspace muss mindestens einen Admin behalten.",
    description: "Toast when the last-admin guard refuses a demote/remove",
  },
  "team.revokeInvite": {
    message: "Widerrufen",
    description: "Revoke pending-invite button",
  },
  "team.resendInvite": {
    message: "Erneut senden",
    description: "Resend pending-invite button (fresh email, refreshed expiry)",
  },
  "team.resendInviteSuccess": {
    message: "Einladung erneut an {email} gesendet",
    description: "Toast after a successful invite resend",
  },
  "team.resendInviteError": {
    message: "Diese Einladung konnte nicht erneut gesendet werden",
    description: "Toast after a failed invite resend",
  },
  "team.seatUsage": {
    message: "{used} von {limit} Plätzen",
    description:
      "Seat usage in the Team card title on a limited plan — accepted members plus pending invites over the plan cap",
  },
  "team.seatCount_one": {
    message: "{count} Platz belegt",
    description:
      "Seat count in the Team card title on an unlimited plan (no cap to show, singular)",
  },
  "team.seatCount_other": {
    message: "{count} Plätze belegt",
    description:
      "Seat count in the Team card title on an unlimited plan (no cap to show, plural)",
  },
  "team.seatsFullBody": {
    message:
      "Dieser Workspace hat alle Plätze seines Tarifs belegt. Upgrade, um mehr Mitglieder einzuladen.",
    description:
      "Invite dialog wall when seats are exhausted before composing an invite",
  },
  "team.mfaEnabled": {
    message: "2FA",
    description:
      "Badge on a member row whose account has a second factor enrolled",
  },
  "team.leaveTitle": {
    message: "Workspace verlassen",
    description: "Title of the leave-workspace card in workspace settings",
  },
  "team.leaveDescription": {
    message:
      "Beende deine Mitgliedschaft in {workspace}. Du verlierst den Zugriff auf seine Services und Daten, und alle API-Schlüssel, die du hier erstellt hast, funktionieren nicht mehr. Ein Admin kann dich wieder einladen.",
    description: "Description of the leave-workspace card",
  },
  "team.leaveAction": {
    message: "Workspace verlassen",
    description: "Label of the leave-workspace button",
  },
  "team.leaveConfirmTitle": {
    message: "Diesen Workspace verlassen?",
    description: "Title of the leave-workspace confirmation dialog",
  },
  "team.leaveConfirm": {
    message:
      "Du verlierst den Zugriff auf {workspace} und die API-Schlüssel, die du darin erstellt hast, werden widerrufen. Du kannst nur wieder beitreten, wenn ein Admin dich einlädt.",
    description: "Body of the leave-workspace confirmation dialog",
  },
  "team.leaveCancel": {
    message: "Abbrechen",
    description: "Cancel label in the leave-workspace confirmation dialog",
  },
  "team.leaveErrorTitle": {
    message: "Der Workspace konnte nicht verlassen werden",
    description: "Title of the inline error when leaving is refused",
  },
  "team.leaveError": {
    message:
      "Das Verlassen des Workspace ist fehlgeschlagen. Bitte versuche es erneut.",
    description: "Fallback message when leaving fails without a server reason",
  },
  "team.leaveLastAdminReason": {
    message:
      "Du bist der letzte Admin. Mache zuerst jemand anderen zum Admin, damit der Workspace weiterhin einen hat.",
    description: "Reason the Leave action is disabled for the last admin",
  },
  "team.you": {
    message: "Du",
    description: "Badge marking the caller's own row in the members table",
  },
  "team.removeSelfReason": {
    message:
      "Du kannst dich nicht selbst entfernen. Verlasse stattdessen den Workspace.",
    description:
      "Reason the Remove control is disabled on the caller's own row",
  },
  "team.changeOwnRoleReason": {
    message:
      "Du kannst deine eigene Rolle nicht ändern. Bitte einen anderen Admin, sie zu ändern.",
    description: "Reason the role picker is disabled on the caller's own row",
  },
  "team.mfaEnabledTooltip": {
    message: "Zwei-Faktor-Authentifizierung aktiviert",
    description: "Tooltip for the member-row 2FA badge",
  },
  "team.identityUnresolved": {
    message: "Nicht aufgelöst",
    description:
      "Badge on a member row whose Kratos identity lookup missed (w4/070)",
  },
  "team.identityUnresolvedTooltip": {
    message:
      "Das Konto dieses Mitglieds konnte nicht abgerufen werden. Es belegt weiterhin einen Platz, bis ein Admin es entfernt.",
    description: "Tooltip for the unresolved-identity member badge",
  },
  "team.inviteAccepted": {
    message: "Du bist {workspace} beigetreten",
    description: "Toast after an emailed invite link is redeemed successfully",
  },
  "team.inviteAcceptedAlready": {
    message: "Diese Einladung wurde bereits verwendet",
    description: "Toast when an invite token was already redeemed",
  },
  "team.inviteAcceptExpired": {
    message: "Diese Einladung ist abgelaufen — bitte um eine neue",
    description: "Toast when an invite token is past its expiry",
  },
  "team.inviteAcceptError": {
    message: "Diese Einladung konnte nicht angenommen werden",
    description: "Toast when redeeming an invite token fails",
  },
  "team.revokeInviteSuccess": {
    message: "Einladung widerrufen",
    description: "Toast after a successful invite revoke",
  },
  "team.revokeInviteError": {
    message: "Diese Einladung konnte nicht widerrufen werden",
    description: "Toast after a failed invite revoke",
  },
  "team.errorTitle": {
    message: "Das Team konnte nicht geladen werden",
    description: "Team panel generic error title",
  },
  "team.errorBody": {
    message:
      "Beim Laden der Mitglieder dieses Workspace ist etwas schiefgelaufen. Versuche es erneut.",
    description: "Team panel generic error body",
  },
  "team.role.VIEWER": {
    message: "Betrachter",
    description: "Role label — read-only",
  },
  "team.role.CONTRIBUTOR": {
    message: "Contributor",
    description: "Role label — works on resources, no sensitive fields",
  },
  "team.role.DEVELOPER": {
    message: "Entwickler",
    description: "Role label — full resource access",
  },
  "team.role.ADMIN": {
    message: "Admin",
    description: "Role label — full access incl. settings, members, billing",
  },
  "team.role.BILLING": {
    message: "Abrechnung",
    description: "Role label — billing only",
  },
  "team.twoFactorRequiredNotEnabled": {
    message: "2FA erforderlich, nicht aktiviert",
    description:
      "Badge on a member row while the workspace requires 2FA and the member has none (w22/m402)",
  },
  "team.twoFactorRequiredNotEnabledTooltip": {
    message:
      "Dieser Workspace erfordert Zwei-Faktor-Authentifizierung. Dieses Mitglied hat keinen Zugriff, bis es 2FA aktiviert.",
    description: "Tooltip for the required-but-not-enabled member badge",
  },
};

export default deTeam;
