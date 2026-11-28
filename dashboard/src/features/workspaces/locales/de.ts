import type { TranslationEntry } from "@/i18n";

const deWorkspaces: Record<string, TranslationEntry> = {
  "workspaces.idCopied": {
    message: "Workspace-ID kopiert",
    description: "Toast after copying the workspace ID",
  },
  "workspaces.idCopyFailed": {
    message: "Workspace-ID konnte nicht kopiert werden",
    description: "Toast when copying the workspace ID fails",
  },
  "workspaces.switcherEmpty": {
    message: "Workspace auswählen",
    description: "Switcher trigger label before any workspace is selected",
  },
  "workspaces.switcherLabel": {
    message: "Workspaces",
    description: "Switcher dropdown label above the workspace list",
  },
  "workspaces.switcherBilling": {
    message: "Abrechnung",
    description: "Switcher menu item linking to /billing",
  },
  "workspaces.switcherSettings": {
    message: "Workspace-Einstellungen",
    description: "Switcher menu item linking to /workspace/settings",
  },
  "workspaces.switcherNew": {
    message: "+ Neuer Workspace",
    description: "Switcher menu item linking to /new/workspace",
  },
  "workspaces.newTitle": {
    message: "Neuer Workspace",
    description: "New-workspace page document title",
  },
  "workspaces.planPickerLabel": {
    message: "Tarif",
    description: "Accessible label for the plan-card radiogroup",
  },
  "workspaces.planHobbyName": {
    message: "Hobby",
    description: "Workspace plan card name",
  },
  "workspaces.planHobbyBilling": {
    message: "$0/Monat",
    description:
      "Workspace plan card billing label (pricing.yaml hobby usdPerMonth)",
  },
  "workspaces.planHobbyDescription": {
    message: "Für Einzelpersonen zum Einstieg.",
    description: "Workspace plan card one-line pitch",
  },
  "workspaces.planHobbyBulletMembers": {
    message: "1 Mitglied",
    description: "Hobby plan LimitsFor.MaxMembers bullet",
  },
  "workspaces.planHobbyBulletServices": {
    message: "Bis zu 25 Services",
    description: "Hobby plan LimitsFor.MaxServices bullet",
  },
  "workspaces.planHobbyBulletWorkspaces": {
    message: "5 Hobby-Workspaces pro Nutzer",
    description: "Hobby plan LimitsFor.MaxWorkspacesPerUser bullet",
  },
  "workspaces.planProName": {
    message: "Pro",
    description: "Workspace plan card name",
  },
  "workspaces.planProBilling": {
    message: "$17.50/Monat",
    description:
      "Workspace plan card billing label (pricing.yaml pro usdPerMonth)",
  },
  "workspaces.planProDescription": {
    message: "Für kleine Teams, die gemeinsam ausliefern.",
    description: "Workspace plan card one-line pitch",
  },
  "workspaces.planProBulletMembers": {
    message: "Unbegrenzt viele Mitglieder",
    description: "Pro plan unlimited MaxMembers bullet",
  },
  "workspaces.planProBulletServices": {
    message: "Unbegrenzt viele Services",
    description: "Pro plan unlimited MaxServices bullet",
  },
  "workspaces.planScaleName": {
    message: "Scale",
    description: "Workspace plan card name",
  },
  "workspaces.planScaleBilling": {
    message: "$349.30/Monat pro Organisation",
    description:
      "Workspace plan card billing label (pricing.yaml scale usdPerMonth, per: organization; w7/m464)",
  },
  "workspaces.planScaleDescription": {
    message: "Für wachsende Teams, die zusätzliche Rollen brauchen.",
    description: "Workspace plan card one-line pitch",
  },
  "workspaces.planScaleBulletMembers": {
    message: "Unbegrenzt viele Mitglieder",
    description: "Scale plan unlimited MaxMembers bullet",
  },
  "workspaces.planScaleBulletServices": {
    message: "Unbegrenzt viele Services",
    description: "Scale plan unlimited MaxServices bullet",
  },
  "workspaces.planScaleBulletRoles": {
    message: "Zusätzliche Rollen (Contributor, Betrachter, Abrechnung)",
    description: "Scale plan AllowedRoles beyond Pro bullet",
  },
  "workspaces.planEnterpriseName": {
    message: "Enterprise",
    description: "Workspace plan card name",
  },
  "workspaces.planEnterpriseBilling": {
    message: "Individuelle Konditionen pro Organisation",
    description:
      "Workspace plan card billing label: Enterprise is charged once per organization like Scale (w7/m464)",
  },
  "workspaces.planEnterpriseDescription": {
    message: "Individuelle Limits und Support.",
    description: "Workspace plan card one-line pitch",
  },
  "workspaces.planEnterpriseBulletLimits": {
    message: "Individuelle Limits",
    description: "Enterprise plan custom-limits bullet",
  },
  "workspaces.planEnterpriseBulletSupport": {
    message: "Individueller Support",
    description: "Enterprise plan custom-support bullet",
  },
  "workspaces.planSelect": {
    message: "Tarif auswählen",
    description: "Unselected plan card action label",
  },
  "workspaces.planSelected": {
    message: "Tarif ausgewählt",
    description: "Selected plan card action label",
  },
  "workspaces.planUsageBillingNote": {
    message:
      "Die Nutzung von Services und Datastores wird separat nach Ressourcenstufe abgerechnet.",
    description: "Billing note below the workspace plan picker",
  },
  "workspaces.createTitle": {
    message: "Workspace erstellen",
    description: "/new/workspace page heading",
  },
  "workspaces.createDescription": {
    message: "Wähle Details, Rechnungskontakt, Tarif und Zahlungsmethode.",
    description: "/new/workspace page subtitle",
  },
  "workspaces.detailsTitle": {
    message: "Workspace-Details",
    description: "/new/workspace details section heading",
  },
  "workspaces.billingEmail": {
    message: "Abrechnungs-E-Mail",
    description: "/new/workspace billing email label",
  },
  "workspaces.billingEmailHelp": {
    message:
      "Belege und Abrechnungshinweise für diesen Workspace gehen hierhin.",
    description: "Editable paid-plan billing email help",
  },
  "workspaces.billingEmailHobbyHelp": {
    message:
      "Bei Hobby-Workspaces ist die Abrechnungs-E-Mail deine Konto-E-Mail.",
    description: "Read-only Hobby billing email help",
  },
  "workspaces.billingEmailError": {
    message: "Gib eine gültige Abrechnungs-E-Mail ein.",
    description: "Billing email validation error",
  },
  "workspaces.fieldSlug": {
    message: "Workspace-Slug",
    description: "/new/workspace slug field label",
  },
  "workspaces.fieldSlugHelp": {
    message:
      "Ein eindeutiger Schlüssel für CLI und API. Kleinbuchstaben, Ziffern und Bindestriche, 1–30 Zeichen.",
    description: "/new/workspace slug helper text",
  },
  "workspaces.fieldSlugError": {
    message:
      "Nur Kleinbuchstaben, Ziffern und Bindestriche, 1-30 Zeichen, kein Bindestrich am Anfang oder Ende.",
    description: "Workspace slug validation error",
  },
  "workspaces.paymentTitle": {
    message: "Zahlungsmethode",
    description: "/new/workspace payment panel heading",
  },
  "workspaces.paymentDescription": {
    message:
      "Die Abrechnung ist für jeden Workspace eigenständig. Diese Zahlungsmethode gehört nur zum neuen Workspace.",
    description: "/new/workspace workspace-specific payment copy",
  },
  "workspaces.paymentRequired": {
    message: "Für diesen Workspace ist eine Zahlungsmethode erforderlich.",
    description: "Required payment policy copy",
  },
  "workspaces.paymentOptional": {
    message: "Füge jetzt eine Zahlungsmethode hinzu oder fahre ohne fort.",
    description: "Optional payment policy copy",
  },
  "workspaces.paymentSelfHosted": {
    message:
      "Der Zahlungseinzug ist in dieser selbst gehosteten Installation deaktiviert.",
    description: "Billing-off create-flow copy",
  },
  "workspaces.paymentAdd": {
    message: "Zahlungsmethode hinzufügen",
    description: "Open Payment Element action",
  },
  "workspaces.paymentSave": {
    message: "Zahlungsmethode speichern",
    description: "Confirm SetupIntent action",
  },
  "workspaces.paymentAdded": {
    message: "Zahlungsmethode für diesen Workspace verifiziert.",
    description: "Successful payment setup state",
  },
  "workspaces.paymentError": {
    message:
      "Diese Zahlungsmethode konnte nicht verifiziert werden. Versuche es erneut.",
    description: "Payment Element fallback error",
  },
  "workspaces.fieldName": {
    message: "Name",
    description: "Workspace name field label (shared by create + settings)",
  },
  "workspaces.fieldDisplayNamePlaceholder": {
    message: "z. B. Acme Staging",
    description: "/new/workspace freeform name placeholder",
  },
  "workspaces.fieldNamePlaceholder": {
    message: "z. B. acme-staging",
    description: "Workspace name field placeholder",
  },
  "workspaces.fieldNameError": {
    message: "Gib einen Namen mit 1–100 Zeichen ohne Zeilenumbrüche ein.",
    description: "Workspace name validation error",
  },
  "workspaces.fieldPlan": {
    message: "Tarif",
    description: "Workspace plan field label (create picker + settings badge)",
  },
  "workspaces.createErrorTitle": {
    message: "Workspace konnte nicht erstellt werden",
    description: "/new/workspace inline error alert title",
  },
  "workspaces.createCancel": {
    message: "Abbrechen",
    description: "/new/workspace cancel button",
  },
  "workspaces.createCancelError": {
    message:
      "Das Erstellen des Workspace konnte nicht abgebrochen werden. Versuche es erneut.",
    description:
      "/new/workspace toast when cancelling the pending creation fails",
  },
  "workspaces.createSubmit": {
    message: "Workspace erstellen",
    description: "/new/workspace submit button",
  },
  "workspaces.createSuccess": {
    message: "{name} erstellt",
    description: "Toast on a successful workspace create",
  },
  "workspaces.createError": {
    message: "Der Workspace konnte nicht erstellt werden",
    description: "Fallback toast/inline message on a failed create",
  },
  "workspaces.creationLoadError": {
    message:
      "Die Workspace-Einrichtung konnte nicht geladen werden. Lade die Seite neu und versuche es erneut.",
    description:
      "Inline message when the workspace-creation plan policy or a resumed setup attempt fails to load",
  },
  "workspaces.settingsTitle": {
    message: "Workspace-Einstellungen",
    description: "Workspace settings page and card title",
  },
  "workspaces.settingsDescription": {
    message:
      "Benenne diesen Workspace um oder sieh dir seinen Tarif und seine Metadaten an.",
    description: "Workspace settings card description",
  },
  "workspaces.settingsEmpty": {
    message: "Kein Workspace ausgewählt.",
    description: "Workspace settings page empty state",
  },
  "workspaces.settingsNavigation": {
    message: "Einstellungsbereiche",
    description:
      "Accessible label for the workspace settings section navigation",
  },
  "workspaces.renameSubmit": {
    message: "Speichern",
    description: "Workspace rename form submit button",
  },
  "workspaces.renameErrorTitle": {
    message: "Workspace konnte nicht umbenannt werden",
    description: "Workspace rename inline error alert title",
  },
  "workspaces.renameSuccess": {
    message: "Umbenannt in {name}",
    description: "Toast on a successful rename",
  },
  "workspaces.renameError": {
    message: "Der Workspace konnte nicht umbenannt werden",
    description: "Fallback toast/inline message on a failed rename",
  },
  "workspaces.fieldId": {
    message: "Workspace-ID",
    description: "Workspace settings metadata field label",
  },
  "workspaces.fieldCreatedAt": {
    message: "Erstellt",
    description: "Workspace settings metadata field label",
  },
  "workspaces.dangerZoneTitle": {
    message: "Gefahrenzone",
    description: "Workspace settings delete section title",
  },
  "workspaces.dangerZoneDescription": {
    message:
      "Dadurch werden alle Ressourcen und Daten deines Workspace gelöscht. Alle Services, Datastores und Umgebungsvariablen gehen verloren. Das kann nicht rückgängig gemacht werden.",
    description: "Workspace settings delete section description",
  },
  "workspaces.deleteConfirmLabel": {
    message: "Gib zur Bestätigung unten {phrase} ein.",
    description:
      "Body prompt naming the exact 'sudo delete workspace <name>' phrase (rendered bold by SudoCommandField)",
  },
  "workspaces.deleteErrorTitle": {
    message: "Workspace konnte nicht gelöscht werden",
    description: "Delete danger-zone inline error alert title",
  },
  "workspaces.deleteSubmit": {
    message: "Workspace löschen",
    description: "Delete danger-zone submit button",
  },
  "workspaces.deleteSuccess": {
    message: "{name} gelöscht",
    description: "Toast on a successful delete",
  },
  "workspaces.deleteError": {
    message: "Der Workspace konnte nicht gelöscht werden",
    description: "Fallback toast/inline message on a failed delete",
  },
  "workspaces.changePlanTrigger": {
    message: "Tarif ändern",
    description:
      "Plan link on Workspace Settings, the plan gates and the webhooks gate, opening Render's plan page (w3/m708)",
  },
  "workspaces.generalTitle": {
    message: "Allgemein",
    description: "Title of the general card on the workspace settings page",
  },
  "workspaces.ipRulesTitle": {
    message: "Eingehende IP-Regeln",
    description:
      "Workspace Settings → Inbound IP Rules card title and section link (w9/m388)",
  },
  "workspaces.ipRulesDescription": {
    message:
      "Lege fest, welche IP-Adressen jeden Web Service, jede Static Site, jede Postgres-Datenbank und jede Key-Value-Instanz in diesem Workspace erreichen können. Diese Regeln gelten zusätzlich zu den Umgebungs- und Service-Regeln, daher muss eine Quelle auf jeder Ebene erlaubt sein. Private Services, Background Worker und Cron Jobs sind nicht betroffen.",
    description:
      "Inbound IP Rules card description: scope and layering of workspace rules (Render's workspace-level inbound IP rules, w9/m388)",
  },
  "workspaces.ipRulesHint": {
    message:
      "Gib einen CIDR-Block ein (z. B. 203.0.113.0/24) und drücke auf Quelle hinzufügen. Ohne Regeln wird gesamter eingehender Traffic blockiert; 0.0.0.0/0 und ::/0 erlauben jede IPv4- und IPv6-Quelle.",
    description: "Hint above the editable workspace inbound IP rules (w9/m388)",
  },
  "workspaces.ipRulesBlocked": {
    message:
      "Keine Regeln: Gesamter eingehender Traffic zu den Services und Datastores dieses Workspace wird blockiert. Füge 0.0.0.0/0 und ::/0 hinzu, um alle Quellen zu erlauben.",
    description:
      "Warning shown when the workspace rule list is empty (deny-all), saved or about to be saved (w9/m388)",
  },
  "workspaces.ipRulesOpen": {
    message: "Offen für alle Quell-IPs",
    description:
      "Shown in the read-only view when the workspace rules restrict nothing (Render's allow-all default, w9/m388)",
  },
  "workspaces.ipRulesScalePlanOnly": {
    message:
      "Eingehende IP-Regeln für einen Workspace erfordern den Tarif Scale oder höher. Bereits festgelegte Regeln bleiben aktiv und können auf „alles erlauben“ zurückgesetzt werden.",
    description:
      "Notice replacing the editable workspace rules below the Scale plan (w9/m388)",
  },
  "workspaces.ipRulesAdminOnly": {
    message: "Nur Workspace-Admins können diese Regeln ändern.",
    description:
      "Notice for a member who cannot edit the workspace inbound IP rules, and the forbidden-save explanation (w9/m388)",
  },
  "workspaces.ipRulesPlanRefused": {
    message:
      "Der Tarif dieses Workspace enthält keine eingehenden IP-Regeln. Upgrade auf Scale oder höher, um Traffic einzuschränken; du kannst die Regeln jederzeit auf „alles erlauben“ zurücksetzen.",
    description:
      "Inline explanation when bex-api refuses a workspace rule change with PLAN_FEATURE_REQUIRED (w9/m388)",
  },
  "workspaces.ipRulesUnreadable": {
    message:
      "Die eingehenden IP-Regeln dieses Workspace konnten nicht geladen werden.",
    description:
      "Shown in place of the rules when the workspace inbound IP rules read fails (w9/m388)",
  },
  "workspaces.ipRulesResetAllowAll": {
    message: "Auf „alles erlauben“ zurücksetzen",
    description:
      "Button replacing the workspace inbound IP rules with the allow-all pair below the Scale plan (w9/m388)",
  },
  "workspaces.ipRulesCIDR": {
    message: "Neuer CIDR-Block",
    description:
      "Accessible label for the add-row CIDR input of the workspace inbound IP rules",
  },
  "workspaces.ipRulesCIDRRule": {
    message: "CIDR-Block für Regel {number}",
    description:
      "Accessible label for an existing workspace inbound IP rule CIDR",
  },
  "workspaces.ipRulesDescriptionLabel": {
    message: "Beschreibung der neuen Regel",
    description:
      "Accessible label for the add-row description input of the workspace inbound IP rules",
  },
  "workspaces.ipRulesDescriptionRule": {
    message: "Beschreibung für Regel {number}",
    description:
      "Accessible label for an existing workspace inbound IP rule description",
  },
  "workspaces.ipRulesDescriptionPlaceholder": {
    message: "Beschreibung (optional)",
    description: "Placeholder for a workspace inbound IP rule description",
  },
  "workspaces.ipRulesAdd": {
    message: "Quelle hinzufügen",
    description: "Button adding a CIDR to the draft workspace inbound IP rules",
  },
  "workspaces.ipRulesCancel": {
    message: "Abbrechen",
    description:
      "Button discarding unsaved changes to the workspace inbound IP rules",
  },
  "workspaces.ipRulesSave": {
    message: "Regeln speichern",
    description:
      "Button saving the workspace inbound IP rules (setWorkspaceIPAllowList)",
  },
  "workspaces.ipRulesInvalid": {
    message: "Gib eindeutige, gültige IPv4- oder IPv6-CIDR-Blöcke ein.",
    description:
      "Inline validation when a workspace rule block is not a unique, valid CIDR",
  },
  "workspaces.ipRulesRemove": {
    message: "{cidr} entfernen",
    description:
      "Accessible label for a workspace inbound IP rule remove button",
  },
  "workspaces.ipRulesMoveUp": {
    message: "{cidr} nach oben verschieben",
    description: "Accessible label to move a workspace inbound IP rule earlier",
  },
  "workspaces.ipRulesMoveDown": {
    message: "{cidr} nach unten verschieben",
    description: "Accessible label to move a workspace inbound IP rule later",
  },
  "workspaces.ipRulesSaved": {
    message: "Eingehende IP-Regeln gespeichert",
    description: "Toast after setWorkspaceIPAllowList succeeds (w9/m388)",
  },
  "workspaces.ipRulesSaveErrorTitle": {
    message: "Eingehende IP-Regeln konnten nicht gespeichert werden",
    description:
      "Title of the inline alert when a workspace rule change is refused (w9/m388)",
  },
  "workspaces.ipRulesSaveError": {
    message:
      "Die eingehenden IP-Regeln konnten nicht gespeichert werden. Bitte versuche es erneut.",
    description:
      "Fallback inline message when a workspace rule change fails without a server reason (w9/m388)",
  },
  "workspaces.twoFactorTitle": {
    message: "Zwei-Faktor-Authentifizierung",
    description:
      "Title of the two-factor card on the workspace settings page, and its quick-nav label (w22/m402)",
  },
  "workspaces.twoFactorDescription": {
    message:
      "Mitglieder ohne Zwei-Faktor-Authentifizierung (2FA) können nicht auf die Ressourcen oder Einstellungen dieses Workspace zugreifen, bis sie sie aktivieren. Ihre API-Schlüssel und SSH-Schlüssel sind bis dahin ebenfalls gesperrt.",
    description:
      "Description under the two-factor card title; follows Render's login-settings wording",
  },
  "workspaces.twoFactorSwitchLabel": {
    message: "Zwei-Faktor-Authentifizierung (2FA) erzwingen",
    description:
      "Label of the switch that requires 2FA of every member (Render's control name)",
  },
  "workspaces.loginRequirementStatusRequired": {
    message: "Erforderlich",
    description:
      "Read-only status shown to a non-admin when the workspace requires 2FA",
  },
  "workspaces.loginRequirementStatusNotRequired": {
    message: "Nicht erforderlich",
    description:
      "Read-only status shown to a non-admin when the workspace does not require 2FA",
  },
  "workspaces.loginRequirementAdminOnly": {
    message: "Nur Workspace-Admins können diese Einstellung ändern.",
    description: "Hint shown to a non-admin in place of the switch",
  },
  "workspaces.twoFactorProPlanOnly": {
    message:
      "Das Erzwingen der Zwei-Faktor-Authentifizierung erfordert den Tarif Pro oder höher. Ein Workspace, der sie bereits erzwingt, behält sie, bis ein Admin sie ausschaltet.",
    description:
      "Notice on the two-factor card below the Pro plan, and the refusal for code PLAN_FEATURE_REQUIRED (Render's plan table, w7/070)",
  },
  "workspaces.twoFactorEnabledToast": {
    message: "Zwei-Faktor-Authentifizierung ist in {name} jetzt erforderlich",
    description: "Toast after an admin turns the 2FA requirement on",
  },
  "workspaces.twoFactorDisabledToast": {
    message:
      "Zwei-Faktor-Authentifizierung ist in {name} nicht mehr erforderlich",
    description: "Toast after an admin turns the 2FA requirement off",
  },
  "workspaces.twoFactorErrorTitle": {
    message: "Die Zwei-Faktor-Einstellung konnte nicht geändert werden",
    description: "Title of the inline error when the 2FA switch is refused",
  },
  "workspaces.twoFactorError": {
    message:
      "Die Zwei-Faktor-Einstellung konnte nicht geändert werden. Versuche es erneut.",
    description:
      "Generic message when the 2FA switch fails without a recognised code",
  },
  "workspaces.twoFactorErrorAdminNotEnrolled": {
    message:
      "Aktiviere die Zwei-Faktor-Authentifizierung für dein eigenes Konto, bevor du sie in diesem Workspace erzwingst, sonst sperrst du dich selbst aus.",
    description:
      "Refusal for code WORKSPACE_2FA_ADMIN_NOT_ENROLLED: the acting admin has no second factor",
  },
  "workspaces.twoFactorErrorUnavailable": {
    message:
      "Die Erzwingung der Zwei-Faktor-Authentifizierung ist in dieser Installation nicht verfügbar und kann daher nicht aktiviert werden.",
    description:
      "Refusal for code WORKSPACE_2FA_UNAVAILABLE: the deployment has no identity lookup",
  },
  "workspaces.twoFactorEnrollLink": {
    message: "Zwei-Faktor-Authentifizierung einrichten",
    description:
      "Link to account security, in the admin-not-enrolled refusal and on the blocked page",
  },
  "workspaces.twoFactorBlockedTitle": {
    message: "{workspace} erfordert Zwei-Faktor-Authentifizierung",
    description: "Heading of the page a blocked member sees (w22/m402)",
  },
  "workspaces.twoFactorBlockedBody": {
    message:
      "Ein Admin dieses Workspace verlangt von jedem Mitglied die Zwei-Faktor-Authentifizierung (2FA). Bis du sie aktivierst, kannst du nicht auf die Ressourcen oder Einstellungen dieses Workspace zugreifen, und deine API-Schlüssel und SSH-Schlüssel können dafür nicht verwendet werden.",
    description: "Why the blocked page is shown",
  },
  "workspaces.twoFactorBlockedRecheck": {
    message: "Ich habe sie aktiviert, erneut prüfen",
    description:
      "Button on the blocked page that retries the workspace after enrolling",
  },
  "workspaces.twoFactorBlockedRecheckHint": {
    message:
      "Nach dem Aktivieren von 2FA kann es bis zu eine Minute dauern, bis der Zugriff zurückkehrt.",
    description: "Hint under the blocked page's check-again button",
  },
  "workspaces.loginBlockedSwitchLabel": {
    message: "Zu einem anderen Workspace wechseln",
    description: "Label of the workspace picker on the blocked page",
  },
  "workspaces.loginBlockedSwitchPlaceholder": {
    message: "Workspace auswählen",
    description: "Placeholder of the workspace picker on the blocked page",
  },
  "workspaces.loginBlockedNoOtherWorkspaces": {
    message: "Keine anderen Workspaces",
    description:
      "Placeholder of the blocked page's workspace picker when the member belongs to no other workspace",
  },
  "workspaces.googleLoginTitle": {
    message: "Google-Anmeldung",
    description:
      "Title of the Google login card on the workspace settings page, and its quick-nav label (w7/m465)",
  },
  "workspaces.googleLoginDescription": {
    message:
      "Mitglieder, die sich nicht mit dem Google-Konto ihrer bex-E-Mail-Adresse anmelden, können nicht auf die Ressourcen oder Einstellungen dieses Workspace zugreifen. API-Schlüssel und SSH-Schlüssel, die bei einer anderen Anmeldung erstellt wurden, sind ebenfalls gesperrt.",
    description:
      "Description under the Google login card title; follows Render's login-settings wording",
  },
  "workspaces.googleLoginSwitchLabel": {
    message: "Google-Anmeldung erzwingen",
    description:
      "Label of the switch that requires every member to sign in with Google (Render's control name)",
  },
  "workspaces.googleLoginProPlanOnly": {
    message:
      "Das Erzwingen der Google-Anmeldung erfordert den Tarif Pro oder höher. Ein Workspace, der sie bereits erzwingt, behält sie, bis ein Admin sie ausschaltet.",
    description:
      "Notice on the Google login card below the Pro plan, and the refusal for code PLAN_FEATURE_REQUIRED (Render's plan table)",
  },
  "workspaces.googleLoginEnabledToast": {
    message: "Google-Anmeldung ist in {name} jetzt erforderlich",
    description: "Toast after an admin turns the Google login requirement on",
  },
  "workspaces.googleLoginDisabledToast": {
    message: "Google-Anmeldung ist in {name} nicht mehr erforderlich",
    description: "Toast after an admin turns the Google login requirement off",
  },
  "workspaces.googleLoginErrorTitle": {
    message:
      "Die Einstellung zur Google-Anmeldung konnte nicht geändert werden",
    description:
      "Title of the inline error when the Google login switch is refused",
  },
  "workspaces.googleLoginError": {
    message:
      "Die Einstellung zur Google-Anmeldung konnte nicht geändert werden. Versuche es erneut.",
    description:
      "Generic message when the Google login switch fails without a recognised code",
  },
  "workspaces.googleLoginErrorAdminNotSignedIn": {
    message:
      "Melde dich mit dem Google-Konto deiner bex-E-Mail-Adresse an, bevor du die Google-Anmeldung in diesem Workspace erzwingst, sonst sperrst du dich selbst aus.",
    description:
      "Refusal for code WORKSPACE_GOOGLE_LOGIN_ADMIN_NOT_SIGNED_IN: the acting admin is not signed in with Google",
  },
  "workspaces.googleLoginErrorUnavailable": {
    message:
      "Die Erzwingung der Google-Anmeldung ist in dieser Installation nicht verfügbar und kann daher nicht aktiviert werden.",
    description:
      "Refusal for code WORKSPACE_GOOGLE_LOGIN_UNAVAILABLE: the deployment cannot look up Google accounts",
  },
  "workspaces.googleLoginSignIn": {
    message: "Mit Google anmelden",
    description:
      "Button that signs the session in again with Google and returns to the page: on the blocked page and in the admin's lock-out refusal",
  },
  "workspaces.googleLoginSignInError": {
    message:
      "Die Anmeldung mit Google konnte nicht gestartet werden. Versuche es erneut.",
    description: "Shown when Kratos did not start the Google sign-in",
  },
  "workspaces.googleLoginBlockedTitle": {
    message: "{workspace} erfordert die Google-Anmeldung",
    description:
      "Heading of the page a member who did not sign in with Google sees (w7/m465)",
  },
  "workspaces.googleLoginBlockedBody": {
    message:
      "Ein Admin dieses Workspace verlangt von jedem Mitglied die Anmeldung mit Google. Bis du dich mit dem Google-Konto deiner bex-E-Mail-Adresse anmeldest, kannst du nicht auf die Ressourcen oder Einstellungen dieses Workspace zugreifen, und API-Schlüssel und SSH-Schlüssel, die du bei einer anderen Anmeldung erstellt hast, können dafür nicht verwendet werden.",
    description: "Why the Google login blocked page is shown",
  },
  "workspaces.googleLoginBlockedHint": {
    message: "Nach der Anmeldung kommst du zu dieser Seite zurück.",
    description: "Hint under the blocked page's Sign in with Google button",
  },
  "workspaces.requireTwoFactorConfirmTitle": {
    message: "Zwei-Faktor-Authentifizierung in {workspace} verlangen?",
    description:
      "Title of the confirmation before an admin requires 2FA in the workspace (w7/m471)",
  },
  "workspaces.requireTwoFactorConfirm": {
    message:
      "Mitglieder ohne Zwei-Faktor-Authentifizierung verlieren den Zugriff auf diesen Workspace, bis sie sie aktivieren, und ihre API-Schlüssel und SSH-Schlüssel funktionieren dort nicht mehr.",
    description:
      "Body of the confirmation before an admin requires 2FA in the workspace (w7/m471)",
  },
  "workspaces.requireGoogleLoginConfirmTitle": {
    message: "Google-Anmeldung in {workspace} verlangen?",
    description:
      "Title of the confirmation before an admin requires Google login in the workspace (w7/m471)",
  },
  "workspaces.requireGoogleLoginConfirm": {
    message:
      "Mitglieder, die nicht mit dem Google-Konto ihrer bex-E-Mail-Adresse angemeldet sind, verlieren den Zugriff auf diesen Workspace, bis sie sich so anmelden, und API-Schlüssel und SSH-Schlüssel, die bei einer anderen Anmeldung erstellt wurden, funktionieren dort nicht mehr.",
    description:
      "Body of the confirmation before an admin requires Google login in the workspace (w7/m471)",
  },
  "workspaces.requireConfirmAction": {
    message: "Verlangen",
    description:
      "Confirm button of the workspace's require-2FA and require-Google-login confirmations (w7/m471)",
  },
  "workspaces.loginImpactRegionLabel": {
    message: "Wer den Zugriff verlieren würde",
    description:
      "Accessible name of the confirmation's list of members and keys a login requirement would block (w7/m471)",
  },
  "workspaces.loginImpactChecking": {
    message: "Prüfe, wer den Zugriff verlieren würde …",
    description:
      "Screen-reader text while the confirmation checks who a login requirement would block",
  },
  "workspaces.loginImpactError": {
    message:
      "Konnte nicht prüfen, wer den Zugriff verlieren würde. Du kannst es trotzdem aktivieren.",
    description:
      "Shown in the confirmation when bex could not check who a login requirement would block; confirming stays possible",
  },
  "workspaces.loginImpactNobody": {
    message:
      "Niemand würde den Zugriff verlieren: Alle Mitglieder erfüllen diese Anforderung, und kein API-Schlüssel oder SSH-Schlüssel würde aufhören zu funktionieren.",
    description:
      "Shown in the confirmation when a login requirement would block nobody",
  },
  "workspaces.loginImpactMembersTwoFactor": {
    message:
      "Jedes aufgeführte Mitglied verliert den Zugriff, bis es die Zwei-Faktor-Authentifizierung aktiviert. Jeder aufgeführte API-Schlüssel und SSH-Schlüssel funktioniert nicht mehr, bis sein Besitzer sie aktiviert.",
    description:
      "Sentence above the members and keys requiring 2FA would block, after Render's SSO enforcement list",
  },
  "workspaces.loginImpactMembersGoogleLogin": {
    message:
      "Jedes aufgeführte Mitglied verliert den Zugriff, bis es sich mit dem Google-Konto seiner bex-E-Mail-Adresse anmeldet. Jeder aufgeführte API-Schlüssel und SSH-Schlüssel funktioniert nicht mehr; sein Besitzer kann nach der Anmeldung mit Google einen neuen erstellen.",
    description:
      "Sentence above the members and keys requiring Google login would block, after Render's SSO enforcement list",
  },
  "workspaces.loginImpactLosesAccess": {
    message: "Verliert Zugriff",
    description: "Badge on a listed member who would lose access",
  },
  "workspaces.loginImpactKeepsAccess": {
    message: "Behält Zugriff",
    description:
      "Badge on a listed member who keeps access while their listed keys stop working",
  },
  "workspaces.loginImpactApiKey": {
    message: "API-Schlüssel {name}",
    description:
      "A listed API key that would stop working; {name} is the key's name",
  },
  "workspaces.loginImpactApiKeyInWorkspace": {
    message: "API-Schlüssel {name} in {workspace}",
    description:
      "A listed API key on an org's confirmation; {workspace} is the workspace it belongs to",
  },
  "workspaces.loginImpactSshKey": {
    message: "SSH-Schlüssel {name}",
    description:
      "A listed SSH key that would stop working; {name} is the key's name",
  },
  "workspaces.loginImpactUnattributed": {
    message:
      "API-Schlüssel ohne erfassten Ersteller oder mit einem Ersteller, der kein Mitglied mehr ist, funktionieren nicht mehr:",
    description:
      "Heading of the API keys no current member created, which would stop working",
  },
  "workspaces.loginImpactGoogleSessions": {
    message:
      "Mitglieder, die die Anforderung erfüllen, aber gerade anders angemeldet sind, werden gebeten, sich mit Google anzumelden.",
    description:
      "One line under the Google login confirmation's list: who the list cannot show",
  },
  "workspaces.loginNoticeMembers_one": {
    message: "{count} Mitglied",
    description:
      "The bold count in the notice under an enforced login requirement: how many members do not meet it (w1/m647, Render's wording)",
  },
  "workspaces.loginNoticeMembers_other": {
    message: "{count} Mitglieder",
    description:
      "The bold count in the notice under an enforced login requirement: how many members do not meet it (w1/m647, Render's wording)",
  },
  "workspaces.loginNoticeTwoFactor_one": {
    message:
      "{members} deines Teams hat die Zwei-Faktor-Authentifizierung noch nicht aktiviert.",
    description:
      "Notice under enforced 2FA when members have not enabled it; {members} is the bold count (Render's wording)",
  },
  "workspaces.loginNoticeTwoFactor_other": {
    message:
      "{members} deines Teams haben die Zwei-Faktor-Authentifizierung noch nicht aktiviert.",
    description:
      "Notice under enforced 2FA when members have not enabled it; {members} is the bold count (Render's wording)",
  },
  "workspaces.loginNoticeGoogleLogin_one": {
    message:
      "{members} deines Teams hat sich noch nicht mit Google angemeldet.",
    description:
      "Notice under enforced Google login when members have not signed in with Google; {members} is the bold count (Render's wording)",
  },
  "workspaces.loginNoticeGoogleLogin_other": {
    message:
      "{members} deines Teams haben sich noch nicht mit Google angemeldet.",
    description:
      "Notice under enforced Google login when members have not signed in with Google; {members} is the bold count (Render's wording)",
  },
  "workspaces.loginNoticeViewMembers_one": {
    message: "Mitglied anzeigen",
    description:
      "Button on the notice that opens the list of members who do not meet the requirement (Render's wording)",
  },
  "workspaces.loginNoticeViewMembers_other": {
    message: "Mitglieder anzeigen",
    description:
      "Button on the notice that opens the list of members who do not meet the requirement (Render's wording)",
  },
  "workspaces.loginNoticeAllMembers": {
    message: "Alle Mitglieder",
    description:
      "The bold words of the all-compliant notice (Render's wording)",
  },
  "workspaces.loginNoticeAllTwoFactor": {
    message:
      "{members} deines Teams haben die Zwei-Faktor-Authentifizierung aktiviert!",
    description:
      "Notice under enforced 2FA when every member has enabled it; {members} is the bold 'All members' (Render's wording)",
  },
  "workspaces.loginNoticeAllGoogleLogin": {
    message: "{members} deines Teams haben sich mit Google angemeldet!",
    description:
      "Notice under enforced Google login when every member has signed in with Google; {members} is the bold 'All members' (Render's wording)",
  },
  "workspaces.loginMembersTitleTwoFactor": {
    message: "Mitglieder ohne Zwei-Faktor-Authentifizierung",
    description:
      "Title of the dialog listing members without 2FA (Render's wording)",
  },
  "workspaces.loginMembersTitleGoogleLogin": {
    message: "Mitglieder ohne Google-Anmeldung",
    description:
      "Title of the dialog listing members who have not signed in with Google (Render's wording)",
  },
  "workspaces.loginMembersDescription_one": {
    message:
      "Diesem Mitglied ist der Zugriff auf diesen Team-Workspace und seine Ressourcen derzeit untersagt.",
    description:
      "Description of the dialog listing members who do not meet the enforced requirement (Render's wording)",
  },
  "workspaces.loginMembersDescription_other": {
    message:
      "Diesen Mitgliedern ist der Zugriff auf diesen Team-Workspace und seine Ressourcen derzeit untersagt.",
    description:
      "Description of the dialog listing members who do not meet the enforced requirement (Render's wording)",
  },
  "workspaces.twoFactorRemindTitle_one": {
    message: "Diesen Benutzer daran erinnern, 2FA zu aktivieren?",
    description:
      "Title of the reminder callout in the members-without-2FA dialog (Render's wording)",
  },
  "workspaces.twoFactorRemindTitle_other": {
    message: "Diese Benutzer daran erinnern, 2FA zu aktivieren?",
    description:
      "Title of the reminder callout in the members-without-2FA dialog (Render's wording)",
  },
  "workspaces.twoFactorRemindBody_one": {
    message: "Sende diesem Benutzer eine Erinnerung per E-Mail.",
    description:
      "Body of the reminder callout in the members-without-2FA dialog (Render's wording)",
  },
  "workspaces.twoFactorRemindBody_other": {
    message: "Sende diesen Benutzern Erinnerungen per E-Mail.",
    description:
      "Body of the reminder callout in the members-without-2FA dialog (Render's wording)",
  },
  "workspaces.twoFactorRemindAction_one": {
    message: "Erinnerung senden",
    description: "Button that emails the 2FA reminders (Render's wording)",
  },
  "workspaces.twoFactorRemindAction_other": {
    message: "Erinnerungen senden",
    description: "Button that emails the 2FA reminders (Render's wording)",
  },
  "workspaces.twoFactorRemindError": {
    message:
      "Das tut uns leid. Wir konnten diesen Teammitgliedern keine Erinnerungen senden. Erneut versuchen?",
    description:
      "Shown in the members dialog when the reminders could not be sent (Render's wording)",
  },
  "workspaces.twoFactorRemindSentToast": {
    message:
      "Erinnerungen per E-Mail an Mitglieder ohne Zwei-Faktor-Authentifizierung gesendet!",
    description: "Toast after the 2FA reminders are sent (Render's wording)",
  },
  "workspaces.twoFactorRemindLimitTitle": {
    message: "Erinnerungslimit erreicht!",
    description:
      "Title of the dialog shown when reminders were already sent today (Render's wording)",
  },
  "workspaces.twoFactorRemindLimitBody": {
    message:
      "Wir haben heute bereits Erinnerungen gesendet. Du kannst morgen wieder Erinnerungen senden.",
    description:
      "Body of the dialog shown when reminders were already sent today; the day is UTC (Render's wording)",
  },
  "workspaces.twoFactorConfirmReminder_one": {
    message: "Dieses Mitglied erhält eine Erinnerung, 2FA zu aktivieren.",
    description:
      "Line in the require-2FA confirmation: turning it on emails a reminder to each member listed (Render's wording)",
  },
  "workspaces.twoFactorConfirmReminder_other": {
    message: "Diese Mitglieder erhalten Erinnerungen, 2FA zu aktivieren.",
    description:
      "Line in the require-2FA confirmation: turning it on emails a reminder to each member listed (Render's wording)",
  },
  "workspaces.overlappingDeployPolicyTitle": {
    message: "Richtlinie für überlappende Deployments",
    description:
      "Workspace Settings section title, Render's name for the setting (w4/653)",
  },
  "workspaces.overlappingDeployPolicyDescription": {
    message:
      "Lege fest, was passiert, wenn das Deployment eines Service ausgelöst wird, während noch ein anderes Deployment desselben Service läuft.",
    description:
      "Workspace Settings → Overlapping Deploy Policy description (w4/653)",
  },
  "workspaces.overlappingDeployPolicyWait": {
    message: "Warten",
    description: "The Wait policy's label, Render's wording (w4/653)",
  },
  "workspaces.overlappingDeployPolicyWaitDescription": {
    message:
      "Lasse das laufende Deployment abschließen und fahre dann direkt mit dem zuletzt ausgelösten Deployment fort.",
    description:
      "The Wait policy's sentence, Render's wording (render.com/docs/deploys, w4/653)",
  },
  "workspaces.overlappingDeployPolicyOverride": {
    message: "Überschreiben",
    description: "The Override policy's label, Render's wording (w4/653)",
  },
  "workspaces.overlappingDeployPolicyOverrideDescription": {
    message: "Das laufende Deployment sofort abbrechen und das neue starten.",
    description:
      "The Override policy's sentence, Render's wording (render.com/docs/deploys, w4/653)",
  },
  "workspaces.overlappingDeployPolicySave": {
    message: "Speichern",
    description: "Saves the selected Overlapping Deploy Policy (w4/653)",
  },
  "workspaces.overlappingDeployPolicySaving": {
    message: "Wird gespeichert…",
    description: "The Save button while the policy saves (w4/653)",
  },
  "workspaces.overlappingDeployPolicySaved": {
    message: "Richtlinie für überlappende Deployments gespeichert",
    description:
      "Toast after setWorkspaceOverlappingDeployPolicy succeeds (w4/653)",
  },
  "workspaces.overlappingDeployPolicyError": {
    message:
      "Die Richtlinie für überlappende Deployments konnte nicht gespeichert werden. Bitte versuche es erneut.",
    description:
      "Fallback toast when a policy save fails without a server reason (w4/653)",
  },
  "workspaces.overlappingDeployPolicyAdminOnly": {
    message: "Nur Workspace-Admins können diese Richtlinie ändern.",
    description:
      "Shown in place of Save to a member who cannot manage the workspace (w4/653)",
  },
  "workspaces.orgDowngradePersonRole": {
    message: "{person} ({role})",
    description:
      "A member or invitee in a downgrade precondition: {person} is their id or email, {role} the role to change (w7/m464)",
  },
  "workspaces.orgDowngradeIpRuleWorkspace": {
    message: "dieser Workspace",
    description:
      "The workspace's own inbound IP rules, in the inbound_ip_rules precondition (w7/m464)",
  },
  "workspaces.orgDowngradeIpRuleEnvironment": {
    message: "Umgebung {name}",
    description:
      "An environment with inbound IP rules, in the inbound_ip_rules precondition (w7/m464)",
  },
  "workspaces.orgDowngradeIpRuleService": {
    message: "Service {name}",
    description:
      "A service with inbound IP rules, in the inbound_ip_rules precondition (w7/m464)",
  },
  "workspaces.orgPlanOnPrimaryError": {
    message:
      "Der Tarif dieses Workspace ist der Tarif seiner Organisation. Ändere ihn im primären Workspace der Organisation.",
    description:
      "Refusal for code ORGANIZATION_PLAN_ON_PRIMARY: a secondary workspace's plan changes only on the org's primary (w7/m464)",
  },
  "workspaces.orgPlanSecondaryNote": {
    message:
      "Im Tarif deiner Organisation enthalten. Der primäre Workspace der Organisation verwaltet Tarif und Abrechnung.",
    description:
      "Workspace Settings and Billing on an org's secondary workspace: its plan is the org's, managed on the primary (ADR115, w7/m464)",
  },
  "workspaces.orgPlanPrimarySettingsLink": {
    message: "Zum primären Workspace",
    description:
      "Link from a secondary workspace's settings to the org's primary workspace settings (w7/m464)",
  },
  "workspaces.orgPlanPrimaryBillingLink": {
    message: "Zur Abrechnung des primären Workspace",
    description:
      "Button on a secondary workspace's Billing plan card linking to the org's primary workspace's Billing (w7/m464)",
  },
  "workspaces.privacyTitle": {
    message: "Workspace-Sichtbarkeit",
    description:
      "Workspace Settings → Workspace privacy card title, radio group name and section link (w7/m464)",
  },
  "workspaces.privacyDescription": {
    message:
      "Wähle, wer aus deiner Organisation diesem Workspace beitreten kann.",
    description: "Description under the Workspace privacy card title (w7/m464)",
  },
  "workspaces.privacyPublic": {
    message: "Öffentlich",
    description: "The Public privacy setting's label, Render's name (w7/m464)",
  },
  "workspaces.privacyPublicDescription": {
    message:
      "Jedes Mitglied deiner Organisation kann sich selbst zu diesem Workspace hinzufügen.",
    description:
      "What the Public privacy setting allows, Render's wording (render.com/docs/organizations, w7/m464)",
  },
  "workspaces.privacyInviteOnly": {
    message: "Nur auf Einladung",
    description:
      "The Invite-only privacy setting's label, Render's name (w7/m464)",
  },
  "workspaces.privacyInviteOnlyDescription": {
    message:
      "Nur Workspace-Admins können Personen zu diesem Workspace einladen.",
    description:
      "What the Invite-only privacy setting allows, Render's wording (render.com/docs/organizations, w7/m464)",
  },
  "workspaces.privacySave": {
    message: "Speichern",
    description: "Saves the selected workspace privacy (w7/m464)",
  },
  "workspaces.privacySaving": {
    message: "Wird gespeichert…",
    description: "The Save button while the workspace privacy saves (w7/m464)",
  },
  "workspaces.privacySaved": {
    message: "Workspace-Sichtbarkeit gespeichert",
    description: "Toast after setWorkspacePrivacy succeeds (w7/m464)",
  },
  "workspaces.privacyError": {
    message:
      "Die Workspace-Sichtbarkeit konnte nicht gespeichert werden. Bitte versuche es erneut.",
    description:
      "Fallback toast when a privacy save fails without a server reason (w7/m464)",
  },
  "workspaces.loginRequirementRequiredByOrganization": {
    message:
      "Von {organization} in allen Workspaces der Organisation verlangt.",
    description:
      "Workspace Settings, under the 2FA or Google login switch: why it is on and locked, the workspace's organization requires it in all of its workspaces (w7/m466)",
  },
  "workspaces.loginRequirementOrganizationSettingsLink": {
    message: "In den Einstellungen der Organisation ändern",
    description:
      "Link for an org Owner after the required-by-organization note, to the organization's Settings page (w7/m466)",
  },
  "workspaces.twoFactorBlockedOrganizationTitle": {
    message:
      "{organization} erfordert Zwei-Faktor-Authentifizierung in allen Workspaces",
    description:
      "Heading of the 2FA blocked page when the organization's setting refuses the member, on a workspace or on the org's own pages (w7/m466)",
  },
  "workspaces.twoFactorBlockedOrganizationBody": {
    message:
      "Ein Inhaber dieser Organisation verlangt von jedem Mitglied die Zwei-Faktor-Authentifizierung (2FA) in der Organisation und allen ihren Workspaces. Bis du sie aktivierst, kannst du nicht darauf zugreifen, und deine API-Schlüssel und SSH-Schlüssel können dafür nicht verwendet werden.",
    description:
      "Why the 2FA blocked page is shown when the organization's setting refuses the member (w7/m466)",
  },
  "workspaces.googleLoginBlockedOrganizationTitle": {
    message:
      "{organization} erfordert die Google-Anmeldung in allen Workspaces",
    description:
      "Heading of the Google login blocked page when the organization's setting refuses the member (w7/m466)",
  },
  "workspaces.googleLoginBlockedOrganizationBody": {
    message:
      "Ein Inhaber dieser Organisation verlangt von jedem Mitglied die Anmeldung mit Google in der Organisation und allen ihren Workspaces. Bis du dich mit dem Google-Konto deiner bex-E-Mail-Adresse anmeldest, kannst du nicht darauf zugreifen, und API-Schlüssel und SSH-Schlüssel, die du bei einer anderen Anmeldung erstellt hast, können dafür nicht verwendet werden.",
    description:
      "Why the Google login blocked page is shown when the organization's setting refuses the member (w7/m466)",
  },
  "workspaces.updatePlanPageTitle": {
    message: "Plan ändern",
    description: "Document title of the plan pages (w3/m708)",
  },
  "workspaces.updatePlanTitle": {
    message: "Plan für {name} ändern",
    description:
      "Render's plan page, /billing/update-plan (w3/m708): page title with the workspace's name",
  },
  "workspaces.updatePlanConfirmTitle": {
    message: "Plan für {name} ändern",
    description:
      "Render's plan confirmation, /billing/update-plan-confirm (w3/m708): page title with the workspace's name (Render capitalizes Plan here)",
  },
  "workspaces.updatePlanBack": {
    message: "Zurück",
    description:
      "Accessible label of the plan pages' Back button, and the confirmation's Back action (w3/m708)",
  },
  "workspaces.updatePlanSelectTitle": {
    message: "Plan auswählen",
    description:
      "Render's plan page, /billing/update-plan (w3/m708): heading of the plan cards",
  },
  "workspaces.updatePlanCompare": {
    message:
      "Mehr über die Pläne erfährst du und vergleichen kannst du sie auf unserer {link}.",
    description:
      "Render's plan page, /billing/update-plan (w3/m708): line under Select a plan; {link} is the updatePlanPricingLink link to bex's pricing page",
  },
  "workspaces.updatePlanPricingLink": {
    message: "Preisseite",
    description:
      "Render's plan page, /billing/update-plan (w3/m708): the link in updatePlanCompare",
  },
  "workspaces.updatePlanPaymentTitle": {
    message: "Zahlungsmethode",
    description:
      "Render's plan page, /billing/update-plan (w3/m708): heading of the payment method section",
  },
  "workspaces.updatePlanCardRequired": {
    message:
      "Für diesen Workspace-Plan ist eine Karte für kostenpflichtige Services und Rechenleistung erforderlich.",
    description:
      "Render's plan page, /billing/update-plan (w3/m708): notice while a plan other than Hobby is selected",
  },
  "workspaces.updatePlanHowBillingWorks": {
    message: "So funktioniert die Abrechnung",
    description:
      "Render's plan page, /billing/update-plan (w3/m708): link beside updatePlanCardRequired to bex's billing docs",
  },
  "workspaces.updatePlanWorkspaceBillingTitle": {
    message: "Workspace-Abrechnung",
    description:
      "Render's plan page, /billing/update-plan (w3/m708): heading beside the card on file",
  },
  "workspaces.updatePlanWorkspaceBillingBody": {
    message:
      "Die Abrechnung erfolgt für jeden Workspace einzeln. Deine Karte wird mit den Gebühren für kostenpflichtige Services oder Rechenleistung in diesem Workspace belastet.",
    description:
      "Render's plan page, /billing/update-plan (w3/m708): line under updatePlanWorkspaceBillingTitle",
  },
  "workspaces.updatePlanErrorsSummary": {
    message:
      "Deine Auswahl enthält Fehler. Bitte prüfe und behebe sie, um fortzufahren.",
    description:
      "Render's plan page, /billing/update-plan (w3/m708): summary under the plan rules a selection breaks",
  },
  "workspaces.updatePlanErrorPrefix": {
    message: "Ein Fehler ist aufgetreten. {message}",
    description:
      "Render's plan page, /billing/update-plan (w3/m708): a refused change; {message} is the reason",
  },
  "workspaces.updatePlanPaymentRequired": {
    message:
      "Füge eine gültige Karte hinzu, um auf einen kostenpflichtigen Plan zu wechseln.",
    description:
      "The plan pages' message when bex-api refuses a paid plan with PAYMENT_REQUIRED (w3/m708)",
  },
  "workspaces.updatePlanServerError": {
    message: "Serverfehler beim Absenden. Bitte versuche es erneut",
    description:
      "Render's plan page, /billing/update-plan (w3/m708): fallback reason when an upgrade fails without one",
  },
  "workspaces.updatePlanSubmit": {
    message: "Plan auswählen",
    description:
      "Render's plan page, /billing/update-plan (w3/m708): submit button",
  },
  "workspaces.updatePlanCancel": {
    message: "Abbrechen",
    description:
      "Render's plan page, /billing/update-plan (w3/m708): cancel button, back to where the page was opened from",
  },
  "workspaces.updatePlanNoPermission": {
    message:
      "Deine Rolle kann den Plan dieses Workspace nicht ändern. Wende dich an einen Workspace-Admin.",
    description:
      "The plan pages' notice for a member who cannot change the plan (can_manage) (w3/m708)",
  },
  "workspaces.planCurrentBadge": {
    message: "Aktueller Plan",
    description:
      "Badge on the workspace's own plan card on the plan page (Render's TeamPlanPicker, w3/m708)",
  },
  "workspaces.planNewBadge": {
    message: "Neuer Plan",
    description:
      "Badge on the chosen plan card on the plan page (Render's TeamPlanPicker, w3/m708)",
  },
  "workspaces.planContact": {
    message: "Kontakt aufnehmen",
    description:
      "Enterprise card's contact action on the plan page, Render's Get in touch (w3/m708)",
  },
  "workspaces.updatePlanSummaryTitle": {
    message: "Übersicht des neuen Plans",
    description:
      "Render's plan confirmation, /billing/update-plan-confirm (w3/m708): heading of the plan comparison and the card on file",
  },
  "workspaces.updatePlanConfirmDowngradeTitle": {
    message: "Downgrade bestätigen",
    description:
      "Render's plan confirmation, /billing/update-plan-confirm (w3/m708): heading of the typed confirmation a downgrade asks for",
  },
  "workspaces.updatePlanConfirmChangesIncluding": {
    message: "Um die Änderungen an deinem Plan einzuleiten, darunter:",
    description:
      "Render's plan confirmation, /billing/update-plan-confirm (w3/m708): lead-in above the list of what a downgrade changes",
  },
  "workspaces.updatePlanConfirmTypeAfterList": {
    message:
      "Gib {phrase} in das Textfeld ein und klicke auf die Bestätigungsschaltfläche.",
    description:
      "Render's plan confirmation, /billing/update-plan-confirm (w3/m708): the typed phrase after the list of changes; {phrase} is 'sudo downgrade workspace <name>' (rendered bold)",
  },
  "workspaces.updatePlanConfirmType": {
    message:
      "Um die Änderungen an deinem Plan einzuleiten, gib {phrase} in das Textfeld ein und klicke auf die Bestätigungsschaltfläche.",
    description:
      "Render's plan confirmation, /billing/update-plan-confirm (w3/m708): the typed phrase when the downgrade lists no changes; {phrase} is 'sudo downgrade workspace <name>' (rendered bold)",
  },
  "workspaces.updatePlanConfirmSubmit": {
    message: "Bestätigen",
    description:
      "Render's plan confirmation, /billing/update-plan-confirm (w3/m708): confirm button",
  },
  "workspaces.updatePlanErrorTitle": {
    message: "Ein Fehler ist aufgetreten.",
    description:
      "Render's plan confirmation, /billing/update-plan-confirm (w3/m708): title of a refused change",
  },
  "workspaces.updatePlanError": {
    message: "Fehler beim Aktualisieren des Plans.",
    description:
      "Render's plan confirmation, /billing/update-plan-confirm (w3/m708): fallback reason when the change fails without one",
  },
  "workspaces.updatePlanRemovesOrganization": {
    message: "Die Organisation wird entfernt.",
    description:
      "Render's plan confirmation, /billing/update-plan-confirm (w3/m708): bold start of the change an org primary's downgrade below Scale makes (ADR115 D6)",
  },
  "workspaces.updatePlanRemovesOrganizationDetail": {
    message:
      "Ihre Funktionen auf Organisationsebene enden, und dieser Workspace besteht als eigenständiger Workspace weiter.",
    description:
      "Render's plan confirmation, /billing/update-plan-confirm (w3/m708): rest of the org removal line after updatePlanRemovesOrganization",
  },
  "workspaces.planUpdated": {
    message: "Dein Workspace nutzt jetzt den Plan {plan}.",
    description:
      "Toast after a plan change, Render's plan-updated toast; {plan} is the plan's name (w3/m708)",
  },
  "workspaces.planUpdatedWelcome": {
    message: "Willkommen im Plan {plan}.",
    description:
      "Toast after a change to Scale, Render's welcome; {plan} is the plan's name (w3/m708)",
  },
  "workspaces.planComparisonCurrent": {
    message: "Aktueller Plan",
    description:
      "Render's plan confirmation, /billing/update-plan-confirm (w3/m708): badge on the current plan's side of the comparison",
  },
  "workspaces.planComparisonNew": {
    message: "Neuer Plan",
    description:
      "Render's plan confirmation, /billing/update-plan-confirm (w3/m708): badge on the new plan's side of the comparison",
  },
  "workspaces.planComparisonHighlights": {
    message: "Highlights",
    description:
      "Render's plan confirmation, /billing/update-plan-confirm (w3/m708): label above each side's highlights (shown in capitals)",
  },
  "workspaces.planHighlightJustForMe": {
    message: "Nur für mich",
    description:
      "A highlight in the confirmation's plan comparison (w3/m708): Hobby's one seat",
  },
  "workspaces.planHighlightUnlimitedMembers": {
    message: "Unbegrenzt viele Teammitglieder",
    description:
      "A highlight in the confirmation's plan comparison (w3/m708): unlimited seats",
  },
  "workspaces.planHighlightIncludedDomains": {
    message: "{included} Custom Domains inklusive, danach {price}/Domain",
    description:
      "A highlight in the confirmation's plan comparison (w3/m708): custom domains included before they are metered; {included} is the count, {price} bex's price per domain (ADR030 §16)",
  },
  "workspaces.planHighlightUnlimitedDomains": {
    message: "Unbegrenzt viele Custom Domains",
    description:
      "A highlight in the confirmation's plan comparison (w3/m708): every custom domain included (Enterprise)",
  },
  "workspaces.planHighlightTwoEnvironments": {
    message: "2 Umgebungen pro Projekt",
    description:
      "A highlight in the confirmation's plan comparison (w3/m708): Hobby's environments per project",
  },
  "workspaces.planHighlightUnlimitedEnvironments": {
    message: "Unbegrenzt viele Projekte und Umgebungen",
    description:
      "A highlight in the confirmation's plan comparison (w3/m708): unlimited projects and environments",
  },
  "workspaces.planHighlightRetention": {
    message: "{days}-Tage-Fenster für Logs und Metriken",
    description:
      "A highlight in the confirmation's plan comparison (w3/m708): how far back logs and metrics reach; {days} is the number of days",
  },
  "workspaces.planHighlightBasicLogs": {
    message: "Einfache Logs und Metriken",
    description:
      "A highlight in the confirmation's plan comparison (w3/m708): Hobby's logs and metrics, without request logs",
  },
  "workspaces.planHighlightRequestLogs": {
    message: "HTTP-Request-Logs und Antwortzeit-Metriken",
    description:
      "A highlight in the confirmation's plan comparison (w3/m708): Pro's HTTP request logs and response time metrics",
  },
  "workspaces.planHighlightKeptBuilds": {
    message: "{builds} aufbewahrte Builds für sofortiges Rollback",
    description:
      "A highlight in the confirmation's plan comparison (w3/m708): builds kept for rollback; {builds} is the count (core.KeptBuilds)",
  },
  "workspaces.planHighlightAuditLogs": {
    message: "Workspace-Audit-Logs",
    description:
      "A highlight in the confirmation's plan comparison (w3/m708): workspace audit logs (Pro and higher)",
  },
  "workspaces.planHighlightOrganization": {
    message:
      "Eine Organisation mit weiteren Workspaces, zusätzlichen Rollen und Regeln für eingehende IPs",
    description:
      "A highlight in the confirmation's plan comparison (w3/m708): what an organization adds (Scale and higher, ADR115)",
  },
  "workspaces.planBlockerMemberLimitTitle": {
    message: "Zu viele Benutzer im Team.",
    description:
      "Render's plan rule on its plan page (w3/m708): title when the workspace has more people than the plan seats",
  },
  "workspaces.planBlockerMemberLimitOne": {
    message: "{plan}-Workspaces können nur 1 Benutzer haben.",
    description:
      "Render's plan rule on its plan page (w3/m708): a plan with one seat; {plan} is the plan's name",
  },
  "workspaces.planBlockerMemberLimitMany": {
    message: "{plan}-Workspaces können höchstens {limit} Benutzer haben.",
    description:
      "Render's plan rule on its plan page (w3/m708): a plan with several seats; {plan} is the plan's name, {limit} its seats",
  },
  "workspaces.planBlockerUpdateTeamMembers": {
    message: "Teammitglieder bearbeiten",
    description:
      "Render's plan rule on its plan page (w3/m708): link to Workspace Settings' Team section",
  },
  "workspaces.planBlockerServiceLimitTitle": {
    message: "Zu viele Services im Workspace.",
    description:
      "Render's plan rule on its plan page (w3/m708): title when the workspace has more services than the plan allows",
  },
  "workspaces.planBlockerServiceLimit": {
    message: "{plan}-Workspaces können höchstens {limit} Services haben.",
    description:
      "Render's plan rule on its plan page (w3/m708): the plan's service cap; {plan} is the plan's name, {limit} its cap",
  },
  "workspaces.planBlockerUpdateServices": {
    message: "Services bearbeiten",
    description:
      "Render's plan rule on its plan page (w3/m708): link to the workspace's services",
  },
  "workspaces.planBlockerWorkspaceLimitTitle": {
    message: "Zu viele Hobby-Workspaces.",
    description:
      "Render's plan rule on its plan page (w3/m708): title when the caller already has the most Hobby workspaces allowed",
  },
  "workspaces.planBlockerWorkspaceLimit": {
    message:
      "Du kannst nicht mehr als {limit} Hobby-Workspaces haben. Wechsle auf einen höheren Plan oder lösche einen anderen Hobby-Workspace.",
    description:
      "Render's plan rule on its plan page (w3/m708): the cap on a person's Hobby workspaces; {limit} is the cap",
  },
  "workspaces.planBlockerWorkspaceLimitsDocs": {
    message: "Workspace-Limits",
    description:
      "Render's plan rule on its plan page (w3/m708): link to bex's docs on workspace limits",
  },
  "workspaces.planBlockerWorkspacesTitle": {
    message: "Zu viele Workspaces.",
    description:
      "Render's plan rule on its plan page (w3/m708): title when an org primary's downgrade leaves secondary workspaces",
  },
  "workspaces.planBlockerWorkspaces": {
    message: "{plan} unterstützt nicht mehr als einen Workspace.",
    description:
      "Render's plan rule on its plan page (w3/m708): a plan without an organization; {plan} is the plan's name",
  },
  "workspaces.planBlockerUpdateWorkspaces": {
    message: "Workspaces bearbeiten",
    description:
      "Render's plan rule on its plan page (w3/m708): link to the organization's workspaces",
  },
  "workspaces.planBlockerRolesTitle": {
    message:
      "Downgrade nicht möglich, solange nicht verfügbare Rollen verwendet werden",
    description:
      "Render's plan rule on its plan page (w3/m708): title when members or invites hold roles the plan does not offer",
  },
  "workspaces.planBlockerRolesOne": {
    message:
      "{plan} unterstützt die Rolle {roles} nicht. Um deinen Workspace herabzustufen, ändere zuerst betroffene Teammitglieder und ausstehende Einladungen auf Developer oder Admin.",
    description:
      "Render's plan rule on its plan page (w3/m708): one role the plan lacks; {plan} is the plan's name, {roles} the role",
  },
  "workspaces.planBlockerRolesMany": {
    message:
      "{plan} unterstützt die Rollen {roles} nicht. Um deinen Workspace herabzustufen, ändere zuerst betroffene Teammitglieder und ausstehende Einladungen auf Developer oder Admin.",
    description:
      "Render's plan rule on its plan page (w3/m708): several roles the plan lacks; {plan} is the plan's name, {roles} the roles",
  },
  "workspaces.planBlockerIpRulesTitle": {
    message: "IP-Allowlists müssen vor dem Downgrade entfernt werden.",
    description:
      "Render's plan rule on its plan page (w3/m708): title when inbound IP rules stand in the way (Render's words for them on its confirmation)",
  },
  "workspaces.planBlockerIpRules": {
    message:
      "Entferne vor dem Planwechsel alle IP-Allowlist-Konfigurationen aus deinem Workspace, deinen Umgebungen und Services.",
    description:
      "Render's plan rule on its plan page (w3/m708): what to remove for planBlockerIpRulesTitle",
  },
  "workspaces.planBlockerUpdateIpRules": {
    message: "Regeln für eingehende IPs bearbeiten",
    description:
      "Render's plan rule on its plan page (w3/m708): link to Workspace Settings' inbound IP rules",
  },
  "workspaces.planBlockerOtherTitle": {
    message: "Dieser Plan kann noch nicht ausgewählt werden.",
    description:
      "Render's plan rule on its plan page (w3/m708): title of a refusal kind this dashboard does not know yet",
  },
  "workspaces.planBlockerOther": {
    message: "Dieser Workspace erfüllt die Limits des Plans {plan} noch nicht.",
    description:
      "Render's plan rule on its plan page (w3/m708): line under planBlockerOtherTitle; {plan} is the plan's name",
  },
};

export default deWorkspaces;
