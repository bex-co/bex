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
};

export default deOrganizations;
