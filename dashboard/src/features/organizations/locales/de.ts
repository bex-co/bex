import type { TranslationEntry } from "@/i18n";

const deOrganizations: Record<string, TranslationEntry> = {
  "organizations.switcherOrganization": {
    message: "Organisation",
    description:
      "Workspace switcher: the sublabel under an organization's name, which opens the org's Workspaces page",
  },
  "organizations.navLabel": {
    message: "Seiten der Organisation",
    description:
      "Accessible name of the org page navigation (Workspaces, Members, Settings)",
  },
  "organizations.navWorkspaces": {
    message: "Workspaces",
    description: "Org page navigation: the Workspaces page",
  },
  "organizations.navMembers": {
    message: "Mitglieder",
    description: "Org page navigation: the Members page",
  },
  "organizations.navSettings": {
    message: "Einstellungen",
    description: "Org page navigation: the Settings page",
  },
  "organizations.untitled": {
    message: "Organisation",
    description: "Org page title when the organization could not be read",
  },
  "organizations.workspacesTitle": {
    message: "Workspaces der Organisation",
    description: "Document title of the org's Workspaces page",
  },
  "organizations.membersTitle": {
    message: "Mitglieder der Organisation",
    description: "Document title of the org's Members page",
  },
  "organizations.settingsTitle": {
    message: "Einstellungen der Organisation",
    description: "Document title of the org's Settings page",
  },
  "organizations.workspacesDescription": {
    message:
      "Eine Organisation fasst beliebig viele Workspaces, jeden im Plan der Organisation und ohne Aufpreis.",
    description:
      "Org Workspaces page header description (Render's organizations doc)",
  },
  "organizations.membersDescription": {
    message:
      "Inhaber verwalten die Organisation, Mitglieder treten ihren Workspaces bei, und Gäste erreichen einen einzigen Workspace.",
    description: "Org Members page header description",
  },
  "organizations.settingsDescription": {
    message: "Der Name der Organisation und ihr Audit-Log.",
    description: "Org Settings page header description",
  },
  "organizations.roleOwner": {
    message: "Inhaber",
    description: "Org role: Owner (Render's Org Owner)",
  },
  "organizations.roleMember": {
    message: "Mitglied",
    description: "Org role: Member (a standard org member)",
  },
  "organizations.roleGuest": {
    message: "Gast",
    description: "Org role: Guest (reaches a single workspace)",
  },
  "organizations.notFoundTitle": {
    message: "Organisation nicht gefunden",
    description: "Org pages: ORGANIZATION_NOT_FOUND state title",
  },
  "organizations.notFoundBody": {
    message: "Sie existiert nicht, oder du bist kein Mitglied.",
    description: "Org pages: ORGANIZATION_NOT_FOUND state body",
  },
  "organizations.errorTitle": {
    message: "Die Organisation konnte nicht geladen werden",
    description: "Org pages: read failure state title",
  },
  "organizations.errorBody": {
    message: "Versuche es gleich noch einmal.",
    description: "Org pages: read failure state body",
  },

  "organizations.workspacesCardTitle": {
    message: "Workspaces",
    description: "Org Workspaces page: the workspace list card title",
  },
  "organizations.workspacesCardDescription": {
    message:
      "Jedes Mitglied kann einem öffentlichen Workspace beitreten. Für einen Workspace nur auf Einladung braucht es die Einladung eines seiner Admins.",
    description: "Org Workspaces page: the workspace list card description",
  },
  "organizations.colWorkspace": {
    message: "Workspace",
    description: "Org workspace table column: the workspace name",
  },
  "organizations.colPrivacy": {
    message: "Zugang",
    description: "Org workspace table column: Public or Invite-only",
  },
  "organizations.primary": {
    message: "Primär",
    description:
      "Badge on the org's primary workspace (the one that holds the org's plan and billing)",
  },
  "organizations.workspacesEmptyTitle": {
    message: "Keine Workspaces zum Anzeigen",
    description: "Org Workspaces page: empty list title",
  },
  "organizations.workspacesEmptyBody": {
    message:
      "Hier erscheinen die Workspaces dieser Organisation, die du sehen kannst.",
    description: "Org Workspaces page: empty list body",
  },
  "organizations.open": {
    message: "Öffnen",
    description:
      "Org workspace row action: switch to a workspace the caller belongs to",
  },
  "organizations.join": {
    message: "Beitreten",
    description:
      "Org workspace row action: a Member or Owner adds themselves to a Public workspace (Developer role)",
  },
  "organizations.joinAsAdmin": {
    message: "Als Admin beitreten",
    description:
      "Org workspace row action: an Owner adds themselves to any org workspace as admin",
  },
  "organizations.joined": {
    message: "Du bist {workspace} beigetreten.",
    description: "Toast after joining a Public workspace",
  },
  "organizations.joinedAsAdmin": {
    message: "Du bist jetzt Admin von {workspace}.",
    description: "Toast after an Owner joins a workspace as admin",
  },
  "organizations.joinFailed": {
    message: "Beitritt zu {workspace} fehlgeschlagen.",
    description: "Toast when joining a workspace failed for no stated reason",
  },
  "organizations.newWorkspace": {
    message: "Neuer Workspace",
    description:
      "Org Workspaces page: the Owner's button that opens the create dialog (Render: + New Workspace)",
  },
  "organizations.createTitle": {
    message: "Neuer Workspace in {organization}",
    description: "Create org workspace dialog title",
  },
  "organizations.createDescription": {
    message: "Du wirst sein erster Admin und kannst sofort Services anlegen.",
    description: "Create org workspace dialog description",
  },
  "organizations.fieldName": {
    message: "Name",
    description: "Create workspace / rename organization: the name field label",
  },
  "organizations.fieldPrivacy": {
    message: "Zugang",
    description: "Create workspace dialog: the privacy choice's legend",
  },
  "organizations.planIncluded": {
    message: "Im Plan deiner Organisation enthalten",
    description:
      "Create workspace dialog: the new workspace takes the org's plan, no price and no payment step",
  },
  "organizations.planIncludedDetail": {
    message: "Plan {plan}, ohne Aufpreis",
    description:
      "Create workspace dialog: the org's plan name under 'Included in your organization's plan'",
  },
  "organizations.createSubmit": {
    message: "Workspace erstellen",
    description: "Create org workspace dialog: submit button (Render's label)",
  },
  "organizations.cancel": {
    message: "Abbrechen",
    description: "Org dialogs: cancel button",
  },
  "organizations.workspaceCreated": {
    message: "{workspace} wurde erstellt.",
    description: "Toast after creating an org workspace",
  },
  "organizations.createFailed": {
    message: "Der Workspace konnte nicht erstellt werden.",
    description: "Create org workspace: failure with no stated reason",
  },

  "organizations.membersCardTitle": {
    message: "Mitglieder",
    description: "Org Members page: the member list card title",
  },
  "organizations.membersCardDescription": {
    message:
      "Alle Personen der Organisation mit ihrer Rolle darin. Workspace-Rollen legst du im jeweiligen Workspace fest.",
    description: "Org Members page: the member list card description",
  },
  "organizations.colMember": {
    message: "Mitglied",
    description: "Org member table column: the person",
  },
  "organizations.colRole": {
    message: "Rolle",
    description: "Org member table column: the org role",
  },
  "organizations.guestWorkspace": {
    message: "Im Workspace {workspace}",
    description: "Org member table: the one workspace a Guest belongs to",
  },
  "organizations.guestNoWorkspace": {
    message: "Noch in keinem Workspace",
    description: "Org member table: a Guest who belongs to no workspace",
  },
  "organizations.roleFor": {
    message: "Rolle von {identity} in der Organisation",
    description: "Accessible name of an org member's role select",
  },
  "organizations.removeMember": {
    message: "{identity} aus der Organisation entfernen",
    description: "Accessible name of an org member row's remove button",
  },
  "organizations.removeTitle": {
    message: "Aus der Organisation entfernen?",
    description: "Org member remove confirmation title",
  },
  "organizations.removeConfirm": {
    message:
      "{identity} verliert den Zugriff auf die Organisation und alle ihre Workspaces, und die API-Schlüssel dieser Person in diesen Workspaces werden widerrufen.",
    description: "Org member remove confirmation description",
  },
  "organizations.remove": {
    message: "Entfernen",
    description: "Org member remove confirmation: confirm button",
  },
  "organizations.removed": {
    message: "{identity} wurde aus der Organisation entfernt.",
    description: "Toast after removing an org member",
  },
  "organizations.removeFailed": {
    message: "{identity} konnte nicht entfernt werden.",
    description: "Org member removal failure with no stated reason",
  },
  "organizations.roleChanged": {
    message: "{identity} ist jetzt {role}.",
    description: "Toast after changing an org member's role",
  },
  "organizations.roleChangeFailed": {
    message: "Die Rolle konnte nicht geändert werden.",
    description: "Org role change failure with no stated reason",
  },
  "organizations.membersForbiddenTitle": {
    message: "Gäste sehen keine Mitglieder",
    description:
      "Org Members page: a Guest's state (the API answers FORBIDDEN)",
  },
  "organizations.membersForbiddenBody": {
    message:
      "Nur Inhaber und Mitglieder der Organisation sehen, wer dazugehört.",
    description: "Org Members page: a Guest's state body",
  },
  "organizations.membersErrorTitle": {
    message: "Die Mitglieder konnten nicht geladen werden",
    description: "Org Members page: read failure title",
  },
  "organizations.invite": {
    message: "Einladen",
    description: "Org Members page: the Owner's invite button",
  },
  "organizations.inviteTitle": {
    message: "In die Organisation einladen",
    description: "Org invite dialog title",
  },
  "organizations.inviteDescription": {
    message:
      "Die Person bekommt eine E-Mail mit einem Link, um als Inhaber oder Mitglied beizutreten. Workspace-Admins laden Personen in ihre eigenen Workspaces ein.",
    description: "Org invite dialog description",
  },
  "organizations.fieldEmail": {
    message: "E-Mail-Adresse",
    description: "Org invite dialog: email field label",
  },
  "organizations.fieldEmailInvalid": {
    message: "Gib eine gültige E-Mail-Adresse ein.",
    description: "Org invite dialog: malformed email",
  },
  "organizations.fieldRole": {
    message: "Rolle",
    description: "Org invite dialog: role field label",
  },
  "organizations.inviteSubmit": {
    message: "Einladung senden",
    description: "Org invite dialog: submit button",
  },
  "organizations.invited": {
    message: "{email} wurde eingeladen.",
    description: "Toast after sending an org invite",
  },
  "organizations.inviteFailed": {
    message: "Die Einladung konnte nicht gesendet werden.",
    description: "Org invite failure with no stated reason",
  },
  "organizations.pendingTitle": {
    message: "Ausstehende Einladungen",
    description: "Org Members page: the pending invite list heading",
  },
  "organizations.colEmail": {
    message: "E-Mail",
    description: "Org pending invite table column: the invitee's email",
  },
  "organizations.revokeInvite": {
    message: "Widerrufen",
    description: "Org pending invite row action",
  },
  "organizations.inviteRevoked": {
    message: "Die Einladung an {email} wurde widerrufen.",
    description: "Toast after revoking an org invite",
  },
  "organizations.revokeFailed": {
    message: "Die Einladung konnte nicht widerrufen werden.",
    description: "Org invite revoke failure with no stated reason",
  },

  "organizations.nameTitle": {
    message: "Name der Organisation",
    description: "Org Settings page: the name card title",
  },
  "organizations.nameDescription": {
    message: "Erscheint für alle in der Organisation im Workspace-Menü.",
    description: "Org Settings page: the name card description",
  },
  "organizations.nameSave": {
    message: "Speichern",
    description: "Org Settings page: save the new name",
  },
  "organizations.nameSaved": {
    message: "Die Organisation heißt jetzt {name}.",
    description: "Toast after renaming the org",
  },
  "organizations.renameFailed": {
    message: "Die Organisation konnte nicht umbenannt werden.",
    description: "Org rename failure with no stated reason",
  },
  "organizations.nameOwnerOnly": {
    message: "Nur Inhaber können die Organisation umbenennen.",
    description:
      "Org Settings page: why the name is read-only for a Member or Guest",
  },
  "organizations.auditTitle": {
    message: "Audit-Logs",
    description:
      "Org Settings page: the audit log section title (Render's label)",
  },
  "organizations.auditDescription": {
    message:
      "Exportiere die eigenen Ereignisse der Organisation, etwa zu Mitgliedern, Workspaces und Einstellungen, als CSV. Sie stehen in keinem Workspace-Log, und jeder Export umfasst höchstens {days} Tage.",
    description: "Org Settings page: the audit log section description",
  },

  "organizations.errorNotFound": {
    message: "Diese Organisation existiert nicht, oder du bist kein Mitglied.",
    description: "ORGANIZATION_NOT_FOUND refusal",
  },
  "organizations.errorForbidden": {
    message: "Dazu hast du in dieser Organisation keine Berechtigung.",
    description: "FORBIDDEN refusal of an org action",
  },
  "organizations.errorLastOwner": {
    message:
      "Eine Organisation braucht mindestens einen Inhaber. Mach zuerst jemand anderen zum Inhaber.",
    description: "ORGANIZATION_LAST_OWNER refusal",
  },
  "organizations.errorMemberLastAdmin": {
    message:
      "Diese Person ist der letzte Admin von {workspace}. Mach zuerst jemand anderen zum Admin.",
    description: "ORGANIZATION_MEMBER_LAST_ADMIN refusal, naming the workspace",
  },
  "organizations.errorAlreadyMember": {
    message: "Diese Person ist bereits in der Organisation.",
    description: "ORGANIZATION_ALREADY_MEMBER refusal of an org invite",
  },
  "organizations.errorGuestSingleWorkspace": {
    message:
      "Ein Gast gehört zu genau einem Workspace der Organisation. Mach die Person zum Mitglied, um ihr mehr zu geben.",
    description: "ORGANIZATION_GUEST_SINGLE_WORKSPACE refusal",
  },
  "organizations.errorOwnRole": {
    message:
      "Du kannst deine eigene Rolle in der Organisation nicht ändern. Bitte einen anderen Inhaber darum.",
    description: "CANNOT_CHANGE_OWN_ROLE refusal",
  },
  "organizations.errorRemoveSelf": {
    message: "Du kannst dich nicht selbst aus der Organisation entfernen.",
    description: "CANNOT_REMOVE_SELF refusal",
  },
  "organizations.errorInvalidPrivacy": {
    message: "Wähle Öffentlich oder Nur auf Einladung.",
    description: "INVALID_PRIVACY refusal",
  },
  "organizations.securityTitle": {
    message: "Sicherheit",
    description:
      "Org Settings page: title of the login requirements card (Render's section name, where it puts Require Login Method; w7/m466)",
  },
  "organizations.securityDescription": {
    message:
      "Wie sich Mitglieder anmelden müssen, in jedem Workspace der Organisation und auf ihren eigenen Seiten, zusätzlich zu den Einstellungen jedes Workspace.",
    description: "Org Settings page: what the Security card's settings do",
  },
  "organizations.securityOwnerOnly": {
    message: "Nur Inhaber können diese Einstellungen ändern.",
    description:
      "Org Settings page: why the Security settings are read-only for a Member or Guest",
  },
  "organizations.requireTwoFactorLabel": {
    message: "Zwei-Faktor-Authentifizierung verlangen",
    description:
      "Org Settings page: label of the switch that requires 2FA in every workspace of the org",
  },
  "organizations.requireTwoFactorDescription": {
    message:
      "Mitglieder ohne Zwei-Faktor-Authentifizierung (2FA) können nicht auf die Organisation oder ihre Workspaces zugreifen, bis sie sie aktivieren. Ihre API-Schlüssel und SSH-Schlüssel sind bis dahin ebenfalls gesperrt.",
    description: "Org Settings page: what requiring 2FA does",
  },
  "organizations.requireGoogleLoginLabel": {
    message: "Google-Anmeldung verlangen",
    description:
      "Org Settings page: label of the switch that requires signing in with Google in every workspace of the org",
  },
  "organizations.requireGoogleLoginDescription": {
    message:
      "Mitglieder, die sich nicht mit dem Google-Konto ihrer bex-E-Mail-Adresse anmelden, können nicht auf die Organisation oder ihre Workspaces zugreifen. API-Schlüssel und SSH-Schlüssel, die bei einer anderen Anmeldung erstellt wurden, sind ebenfalls gesperrt.",
    description: "Org Settings page: what requiring Google login does",
  },
  "organizations.requireTwoFactorConfirmTitle": {
    message: "Zwei-Faktor-Authentifizierung in {organization} verlangen?",
    description:
      "Title of the confirmation before an Owner requires 2FA in the org",
  },
  "organizations.requireTwoFactorConfirm": {
    message:
      "Mitglieder ohne Zwei-Faktor-Authentifizierung verlieren den Zugriff auf die Organisation und alle ihre Workspaces, bis sie sie aktivieren, und ihre API-Schlüssel und SSH-Schlüssel funktionieren dort nicht mehr. Die eigene Einstellung jedes Workspace bleibt, wie sie ist.",
    description:
      "Body of the confirmation before an Owner requires 2FA in the org",
  },
  "organizations.requireGoogleLoginConfirmTitle": {
    message: "Google-Anmeldung in {organization} verlangen?",
    description:
      "Title of the confirmation before an Owner requires Google login in the org",
  },
  "organizations.requireGoogleLoginConfirm": {
    message:
      "Mitglieder, die nicht mit dem Google-Konto ihrer bex-E-Mail-Adresse angemeldet sind, verlieren den Zugriff auf die Organisation und alle ihre Workspaces, bis sie sich so anmelden, und API-Schlüssel und SSH-Schlüssel, die bei einer anderen Anmeldung erstellt wurden, funktionieren dort nicht mehr. Die eigene Einstellung jedes Workspace bleibt, wie sie ist.",
    description:
      "Body of the confirmation before an Owner requires Google login in the org",
  },
  "organizations.requireConfirmAction": {
    message: "Verlangen",
    description:
      "Confirm button of the require-2FA and require-Google-login confirmations",
  },
  "organizations.requireTwoFactorEnabledToast": {
    message:
      "Zwei-Faktor-Authentifizierung ist jetzt in jedem Workspace von {name} erforderlich",
    description: "Toast after an Owner turns the org's 2FA requirement on",
  },
  "organizations.requireTwoFactorDisabledToast": {
    message: "{name} verlangt keine Zwei-Faktor-Authentifizierung mehr",
    description: "Toast after an Owner turns the org's 2FA requirement off",
  },
  "organizations.requireGoogleLoginEnabledToast": {
    message:
      "Die Google-Anmeldung ist jetzt in jedem Workspace von {name} erforderlich",
    description:
      "Toast after an Owner turns the org's Google login requirement on",
  },
  "organizations.requireGoogleLoginDisabledToast": {
    message: "{name} verlangt keine Google-Anmeldung mehr",
    description:
      "Toast after an Owner turns the org's Google login requirement off",
  },
  "organizations.requireTwoFactorErrorOwnerNotEnrolled": {
    message:
      "Aktiviere die Zwei-Faktor-Authentifizierung für dein eigenes Konto, bevor du sie in dieser Organisation verlangst, sonst sperrst du dich selbst aus.",
    description:
      "Refusal for code WORKSPACE_2FA_ADMIN_NOT_ENROLLED on the org setting: the acting Owner has no second factor",
  },
  "organizations.requireGoogleLoginErrorOwnerNotSignedIn": {
    message:
      "Melde dich mit dem Google-Konto deiner bex-E-Mail-Adresse an, bevor du die Google-Anmeldung in dieser Organisation verlangst, sonst sperrst du dich selbst aus.",
    description:
      "Refusal for code WORKSPACE_GOOGLE_LOGIN_ADMIN_NOT_SIGNED_IN on the org setting: the acting Owner is not signed in with Google",
  },
};

export default deOrganizations;
