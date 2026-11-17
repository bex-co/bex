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
    message: "$349.30/Monat",
    description:
      "Workspace plan card billing label (pricing.yaml scale usdPerMonth)",
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
    message: "Individuelle Konditionen",
    description: "Workspace plan card billing label",
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
      "Workspace settings plan-badge link opening the change-plan dialog",
  },
  "workspaces.changePlanTitle": {
    message: "Tarif ändern",
    description: "Change-plan dialog title",
  },
  "workspaces.changePlanDescription": {
    message:
      "Wähle einen neuen Tarif für diesen Workspace. Es gibt keinen Zahlungsschritt — der Tarif ändert sich sofort.",
    description: "Change-plan dialog description",
  },
  "workspaces.changePlanCancel": {
    message: "Abbrechen",
    description: "Change-plan dialog cancel button",
  },
  "workspaces.changePlanSubmit": {
    message: "Tarif ändern",
    description: "Change-plan dialog submit button",
  },
  "workspaces.changePlanErrorTitle": {
    message: "Tarif konnte nicht geändert werden",
    description: "Change-plan dialog inline error alert title",
  },
  "workspaces.changePlanSuccess": {
    message: "Tarif auf {plan} geändert",
    description: "Toast on a successful plan change",
  },
  "workspaces.changePlanError": {
    message: "Der Tarif des Workspace konnte nicht geändert werden",
    description: "Fallback toast/inline message on a failed plan change",
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
  "workspaces.twoFactorStatusRequired": {
    message: "Erforderlich",
    description:
      "Read-only status shown to a non-admin when the workspace requires 2FA",
  },
  "workspaces.twoFactorStatusNotRequired": {
    message: "Nicht erforderlich",
    description:
      "Read-only status shown to a non-admin when the workspace does not require 2FA",
  },
  "workspaces.twoFactorAdminOnly": {
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
  "workspaces.twoFactorBlockedSwitchLabel": {
    message: "Zu einem anderen Workspace wechseln",
    description: "Label of the workspace picker on the blocked page",
  },
  "workspaces.twoFactorBlockedSwitchPlaceholder": {
    message: "Workspace auswählen",
    description: "Placeholder of the workspace picker on the blocked page",
  },
  "workspaces.twoFactorBlockedNoOtherWorkspaces": {
    message: "Keine anderen Workspaces",
    description:
      "Placeholder of the blocked page's workspace picker when the member belongs to no other workspace",
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
};

export default deWorkspaces;
