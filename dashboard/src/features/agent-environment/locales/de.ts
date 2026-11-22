import type { TranslationEntry } from "@/i18n";

const deAgentEnvironment: Record<string, TranslationEntry> = {
  "agentEnvironment.title": {
    message: "Agents",
    description: "Settings → Agents card title and section-nav label",
  },
  "agentEnvironment.description": {
    message:
      "Wie Agent-Sitzungen in diesem Workspace auf das Netzwerk zugreifen und welches Modell sie verwenden. Gilt für jede neue Sitzung.",
    description: "Agents settings card description",
  },
  "agentEnvironment.loadFailed": {
    message: "Die Agent-Einstellungen konnten nicht geladen werden.",
    description: "Agents card error when the environment query fails",
  },
  "agentEnvironment.retry": {
    message: "Erneut versuchen",
    description: "Retry button on the Agents card load error",
  },
  "agentEnvironment.readOnlyNote": {
    message: "Nur Workspace-Admins können Agent-Einstellungen ändern.",
    description: "Read-only notice shown to non-admin members",
  },
  "agentEnvironment.networkTitle": {
    message: "Netzwerkzugriff",
    description: "Heading (and radiogroup label) of the network tier picker",
  },
  "agentEnvironment.networkDescription": {
    message:
      "GitHub und der Modellanbieter sind immer erreichbar. Wähle, was die Sandbox des Agents sonst noch erreichen kann.",
    description: "Network tier picker description",
  },
  "agentEnvironment.tierRegistriesTitle": {
    message: "Paket-Registries (empfohlen)",
    description: "Network tier option: built-in package registries",
  },
  "agentEnvironment.tierRegistriesDescription": {
    message:
      "Der Agent kann Abhängigkeiten von npm, PyPI, Go, Maven, crates, RubyGems und Docker Hub installieren und so deinen Code bauen und testen.",
    description: "Description of the registries network tier",
  },
  "agentEnvironment.tierCustomTitle": {
    message: "Benutzerdefiniert",
    description: "Network tier option: registries plus listed hosts",
  },
  "agentEnvironment.tierCustomDescription": {
    message: "Paket-Registries plus die Hostnamen, die du unten auflistest.",
    description: "Description of the custom network tier",
  },
  "agentEnvironment.tierFullTitle": {
    message: "Volles Internet",
    description: "Network tier option: any public host over HTTP(S)",
  },
  "agentEnvironment.tierFullDescription": {
    message:
      "Jede öffentliche Website über HTTP(S) durch den bex-Egress-Proxy. Direkte Aufrufe von Modellanbieter-APIs bleiben blockiert (das eigene Modell des Agents funktioniert weiterhin), und SSH sowie andere Nicht-HTTP-Protokolle sind nicht verfügbar.",
    description:
      "Description of the full-internet tier: HTTP(S) via the egress proxy, direct model-provider API calls blocked, no SSH/non-HTTP",
  },
  "agentEnvironment.registryHostsSummary_one": {
    message: "Enthaltene Paket-Registries ({count} Host)",
    description: "Disclosure summary listing the built-in registry hosts (one)",
  },
  "agentEnvironment.registryHostsSummary_other": {
    message: "Enthaltene Paket-Registries ({count} Hosts)",
    description:
      "Disclosure summary listing the built-in registry hosts (many)",
  },
  "agentEnvironment.hostsLabel": {
    message: "Zusätzliche Hostnamen",
    description: "Label of the custom-tier hostname textarea",
  },
  "agentEnvironment.hostsHelp": {
    message:
      "Ein Hostname pro Zeile, bis zu {max}. Nur exakte Namen: kein Schema, kein Port, kein Pfad und kein Platzhalter.",
    description: "Help under the custom hostname textarea",
  },
  "agentEnvironment.hostsError.too_many_entries": {
    message: "Führe höchstens {max} Hostnamen auf.",
    description: "Custom hosts error: too many entries",
  },
  "agentEnvironment.hostsError.empty": {
    message: "Hostnamen dürfen nicht leer sein.",
    description: "Custom hosts error: empty entry",
  },
  "agentEnvironment.hostsError.whitespace": {
    message: "„{entry}“ hat umgebende Leerzeichen.",
    description: "Custom hosts error: surrounding whitespace",
  },
  "agentEnvironment.hostsError.not_a_hostname": {
    message:
      "„{entry}“ ist kein exakter Hostname. Verwende einen Namen wie api.example.com, ohne Schema, Port, Pfad oder Platzhalter.",
    description: "Custom hosts error: not a hostname",
  },
  "agentEnvironment.hostsError.ip_literal": {
    message:
      "„{entry}“ ist eine IP-Adresse. Verwende stattdessen einen Hostnamen.",
    description: "Custom hosts error: IP literal",
  },
  "agentEnvironment.hostsError.private_name": {
    message:
      "„{entry}“ ist ein privater oder clusterlokaler Name und kann nicht erlaubt werden.",
    description: "Custom hosts error: private name",
  },
  "agentEnvironment.hostsError.already_baseline": {
    message:
      "„{entry}“ ist bereits erreichbar (oder nur über den Modell-Proxy erreichbar). Entferne es aus der Liste.",
    description: "Custom hosts error: already in the baseline",
  },
  "agentEnvironment.hostsError.duplicate": {
    message: "„{entry}“ ist mehrfach aufgeführt.",
    description: "Custom hosts error: duplicate",
  },
  "agentEnvironment.hostsError.invalid": {
    message: "Die Hostnamen-Liste ist ungültig.",
    description: "Custom hosts error: unknown reason",
  },
  "agentEnvironment.modelLabel": {
    message: "Standardmodell",
    description: "Label of the default model field",
  },
  "agentEnvironment.modelPlaceholder": {
    message: "Standard des Anbieters",
    description: "Placeholder of the empty default model field",
  },
  "agentEnvironment.modelHelp": {
    message:
      "Modell-ID, die neue Sitzungen verwenden, z. B. claude-sonnet-4-5. Leer lassen für den Standard des Anbieters.",
    description: "Help under the default model field",
  },
  "agentEnvironment.modelPlatformDefault": {
    message: "Plattform-Standard ({model})",
    description:
      "Default model picker — no default: the platform catalog's first model (w10/m516)",
  },
  "agentEnvironment.modelCatalogHelp": {
    message:
      "Neue Sitzungen und Automationen ohne eigenes Modell nutzen dieses. Mitglieder können für eine Sitzung weiterhin ein anderes Modell des Plattformkatalogs wählen.",
    description: "Help under the platform model catalog picker (w10/m516)",
  },
  "agentEnvironment.modelInvalid": {
    message:
      "Verwende eine Modell-ID ohne Leerzeichen (höchstens 255 Zeichen).",
    description: "Default model validation error",
  },
  "agentEnvironment.modelNotOfferedError": {
    message:
      "Das Plattformmodell bietet {model} nicht an. Wähle eines aus der Liste.",
    description:
      "Server refusal: the default model is not in the platform catalog (w10/m516)",
  },
  "agentEnvironment.templateLabel": {
    message: "Standard-Sandbox",
    description: "Label of the default sandbox template picker (w10/m519)",
  },
  "agentEnvironment.templateHelp": {
    message:
      "Neue Sitzungen und Automationen ohne eigene Sandbox laufen auf dieser. Sie gilt für danach gestartete Sitzungen; Mitglieder können für eine Sitzung weiterhin eine andere wählen.",
    description: "Help under the default sandbox template picker (w10/m519)",
  },
  "agentEnvironment.templateInvalidError": {
    message:
      "{template} ist keine Sandbox-Vorlage. Wähle eine von: {templates}.",
    description:
      "Server refusal: the default sandbox template is not a registered one; {templates} lists those (w10/m519)",
  },
  "agentEnvironment.tierInvalid": {
    message: "Diese Option für den Netzwerkzugriff wird nicht unterstützt.",
    description: "Server refusal: invalid tier",
  },
  "agentEnvironment.forbidden": {
    message: "Nur Workspace-Admins können Agent-Einstellungen ändern.",
    description: "Server refusal: caller is not an admin",
  },
  "agentEnvironment.saveFailed": {
    message:
      "Die Agent-Einstellungen konnten nicht gespeichert werden. Versuche es erneut.",
    description: "Generic save failure",
  },
  "agentEnvironment.saveFailedReason": {
    message:
      "Die Agent-Einstellungen konnten nicht gespeichert werden: {reason}",
    description: "Save failure with the server's reason",
  },
  "agentEnvironment.save": {
    message: "Speichern",
    description: "Agents card save button",
  },
  "agentEnvironment.saved": {
    message: "Agent-Einstellungen gespeichert",
    description: "Success toast after saving",
  },
  "agentEnvironment.modelKeyTitle": {
    message: "Modellanbieter",
    description: "Heading of the model source / provider key section",
  },
  "agentEnvironment.modelKeyDescription": {
    message:
      "Agent-Sitzungen verwenden das von bex bereitgestellte Modell, solange der Workspace keinen eigenen Anbieter-Schlüssel hinzufügt. Mit deinem eigenen Schlüssel rufen Sitzungen deinen Anbieter direkt auf, und bex berechnet nur die Sandbox-Rechenleistung.",
    description: "Explains the model source choice (ADR091 D7)",
  },
  "agentEnvironment.modelSourceLabel": {
    message: "Modellquelle:",
    description: "Label before the current model source",
  },
  "agentEnvironment.modelSourcePlatform": {
    message:
      "Von bex bereitgestellt (abgerechnet zu den Kosten des Modellanbieters + 30 %)",
    description: "Model source: the platform model",
  },
  "agentEnvironment.modelSourceByo": {
    message: "Dein eigener Schlüssel (keine Token-Kosten von bex)",
    description: "Model source: the workspace's own key",
  },
  "agentEnvironment.modelSourceNone": {
    message:
      "Keine: Füge einen Anbieter-Schlüssel hinzu, um Agent-Sitzungen zu starten",
    description: "Model source: no key available",
  },
  "agentEnvironment.modelKeyLabel": {
    message: "API-Schlüssel des Anbieters",
    description: "Label of the write-only key field when none is set",
  },
  "agentEnvironment.modelKeyReplaceLabel": {
    message: "API-Schlüssel des Anbieters ersetzen",
    description: "Label of the write-only key field when a key is set",
  },
  "agentEnvironment.modelKeyPlaceholder": {
    message: "sk-ant-…",
    description: "Placeholder of the key field when none is set",
  },
  "agentEnvironment.modelKeyPlaceholderSet": {
    message:
      "Ein Schlüssel ist gespeichert; gib einen neuen ein, um ihn zu ersetzen",
    description:
      "Placeholder of the key field when a key is set (the key is never shown)",
  },
  "agentEnvironment.modelKeyHelp": {
    message:
      "Nur schreibbar: Der Schlüssel wird sicher gespeichert und nie wieder angezeigt.",
    description: "Help under the key field",
  },
  "agentEnvironment.modelKeySave": {
    message: "Schlüssel speichern",
    description: "Button that saves the model key",
  },
  "agentEnvironment.modelKeyClear": {
    message: "Schlüssel entfernen",
    description: "Button that opens the remove-key confirmation",
  },
  "agentEnvironment.modelKeyClearTitle": {
    message: "Schlüssel des Modellanbieters entfernen?",
    description: "Remove-key confirmation title",
  },
  "agentEnvironment.modelKeyClearBody": {
    message:
      "Agent-Sitzungen verwenden diesen Schlüssel ab ihrer nächsten Modellanfrage nicht mehr. Der Schlüssel kann nicht erneut angezeigt werden, du müsstest ihn also einfügen, um ihn wiederherzustellen.",
    description: "Remove-key confirmation body",
  },
  "agentEnvironment.modelKeyClearConfirm": {
    message: "Schlüssel entfernen",
    description: "Remove-key confirmation button",
  },
  "agentEnvironment.modelKeySaved": {
    message: "Modellschlüssel gespeichert",
    description: "Toast after saving the key",
  },
  "agentEnvironment.modelKeyCleared": {
    message: "Modellschlüssel entfernt",
    description: "Toast after removing the key",
  },
  "agentEnvironment.modelKeyInvalid": {
    message:
      "Gib den Schlüssel ohne Leerzeichen ein (höchstens 4.096 Zeichen).",
    description: "Client/server refusal of a malformed key",
  },
  "agentEnvironment.modelKeySaveFailed": {
    message:
      "Der Modellschlüssel konnte nicht gespeichert werden. Versuche es erneut.",
    description: "Generic model key write failure",
  },
  "agentEnvironment.spendTitle": {
    message: "Ausgaben für Agent-Modelle",
    description: "Heading of the agent-model spend section (ADR091 D6)",
  },
  "agentEnvironment.spendPricing": {
    message:
      "Abgerechnet zu den Kosten des Modellanbieters + 30 %. Das Limit stoppt neue Modellanfragen für den Rest des Monats.",
    description: "Pricing statement under the spend heading",
  },
  "agentEnvironment.spendSummary": {
    message: "{used} von {cap} in diesem Monat",
    description: "Month-to-date spend against the effective monthly cap",
  },
  "agentEnvironment.spendSummaryCeiling": {
    message:
      "{used} von {cap} über das Abrechnungsguthaben hinaus, {total} in diesem Monat",
    description:
      "Spend summary and meter value when the trust tier's ceiling binds on the spend billing credits leave ({used}), which differs from month-to-date ({total})",
  },
  "agentEnvironment.spendMeterValue": {
    message: "{used} von {cap}",
    description: "Screen-reader value of the spend progress bar",
  },
  "agentEnvironment.spendUnavailable": {
    message: "Die Ausgaben sind gerade nicht verfügbar.",
    description: "Shown when the spend fields could not be read",
  },
  "agentEnvironment.spendTierNextCharge": {
    message:
      "Vertrauensstufe {tier}: Abrechnungsguthaben bezahlen die Ausgaben für Agent-Modelle zuerst; deine Karte wird belastet, sobald die verbleibenden Ausgaben {amount} erreichen.",
    description: "Trust tier with its mid-month charge point (ADR091 D10.1)",
  },
  "agentEnvironment.spendTierNextChargeCredit": {
    message:
      "Vertrauensstufe {tier}: Abrechnungsguthaben decken die ersten {credit} der Ausgaben für Agent-Modelle in diesem Abrechnungszeitraum; deine Karte wird belastet, sobald die darüber hinausgehenden Ausgaben {amount} erreichen.",
    description:
      "Trust tier with its charge point when billing credits cover agent model spend first (ADR091 D10.2a). {credit} is the credit counted in this billing period, {amount} the charge point",
  },
  "agentEnvironment.spendTierMonthEnd": {
    message:
      "Vertrauensstufe {tier}: Die Ausgaben für Agent-Modelle werden zum Monatsende abgerechnet.",
    description: "Top trust tier, charged only at month end",
  },
  "agentEnvironment.spendTierNew": {
    message:
      "Neue Workspaces werden belastet, sobald die Ausgaben für Agent-Modelle über etwaiges Abrechnungsguthaben hinaus {amount} erreichen: Die Karte wird sofort belastet, Sitzungen laufen nach erfolgreicher Zahlung weiter, und das Limit steigt auf die Obergrenze deines Tarifs.",
    description:
      "Explains T0: charged at its threshold, working while the charge settles (ADR091 D10.2a)",
  },
  "agentEnvironment.spendTierLocked": {
    message:
      "Eine fehlgeschlagene oder angefochtene Zahlung schränkt diesen Workspace ein; begleiche sie unter Abrechnung, um die Einschränkung aufzuheben.",
    description: "Explains a payment-locked trust tier",
  },
  "agentEnvironment.spendCapLabel": {
    message: "Monatliches Ausgabenlimit (USD)",
    description: "Label of the admin-editable cap field",
  },
  "agentEnvironment.spendCapSave": {
    message: "Limit speichern",
    description: "Button that saves the cap",
  },
  "agentEnvironment.spendCapReset": {
    message: "Auf Tarifstandard zurücksetzen",
    description: "Button that returns the cap to the plan default",
  },
  "agentEnvironment.spendCapHelp": {
    message: "Tarifstandard {plan}. Admins können bis zu {max} festlegen.",
    description: "Help under the cap field",
  },
  "agentEnvironment.spendCapInvalid": {
    message: "Gib einen Betrag über 0 $ und höchstens {max} ein.",
    description: "Client/server refusal of a cap value",
  },
  "agentEnvironment.spendCapSaved": {
    message: "Ausgabenlimit gespeichert",
    description: "Toast after saving the cap",
  },
  "agentEnvironment.spendCapResetDone": {
    message: "Ausgabenlimit auf den Tarifstandard zurückgesetzt",
    description: "Toast after resetting the cap",
  },
  "agentEnvironment.spendCapSaveFailed": {
    message:
      "Das Ausgabenlimit konnte nicht gespeichert werden. Versuche es erneut.",
    description: "Generic cap write failure",
  },
  "agentEnvironment.secrets.title": {
    message: "Agent-Secrets",
    description: "Settings → Agent secrets card title (ADR098)",
  },
  "agentEnvironment.secrets.description": {
    message:
      "API-Schlüssel und Tokens, die einer Sitzung als Umgebungsvariablen mitgegeben werden können. Werte sind nur schreibbar: Niemand kann sie zurücklesen, und in Transkripten werden sie geschwärzt.",
    description: "Agent secrets card description",
  },
  "agentEnvironment.secrets.loadFailed": {
    message: "Die Agent-Secrets konnten nicht geladen werden.",
    description: "Agent secrets card error when the list query fails",
  },
  "agentEnvironment.secrets.empty": {
    message: "Noch keine Agent-Secrets.",
    description: "Agent secrets card empty state",
  },
  "agentEnvironment.secrets.deleteLabel": {
    message: "{name} löschen",
    description: "Accessible label of a secret's delete button",
  },
  "agentEnvironment.secrets.deleteTitle": {
    message: "{name} löschen?",
    description: "Delete-secret confirmation title",
  },
  "agentEnvironment.secrets.deleteBody": {
    message:
      "Sitzungen, die dieses Secret verwenden, schlagen im nächsten Durchgang fehl, bis es wieder gesetzt ist.",
    description: "Delete-secret confirmation body",
  },
  "agentEnvironment.secrets.deleteConfirm": {
    message: "Secret löschen",
    description: "Delete-secret confirmation button",
  },
  "agentEnvironment.secrets.nameLabel": {
    message: "Secret-Name",
    description: "Agent secrets card: name field label",
  },
  "agentEnvironment.secrets.namePlaceholder": {
    message: "STRIPE_API_KEY",
    description: "Agent secrets card: name field placeholder (an env var name)",
  },
  "agentEnvironment.secrets.valueLabel": {
    message: "Secret-Wert",
    description: "Agent secrets card: value field label",
  },
  "agentEnvironment.secrets.valuePlaceholder": {
    message: "Wert",
    description: "Agent secrets card: value field placeholder",
  },
  "agentEnvironment.secrets.save": {
    message: "Speichern",
    description: "Agent secrets card: save (create or replace) button",
  },
  "agentEnvironment.secrets.help": {
    message:
      "Beim Speichern eines vorhandenen Namens wird dessen Wert ersetzt. Sitzungen übernehmen Änderungen ab ihrem nächsten Durchgang.",
    description: "Agent secrets card help line for admins",
  },
  "agentEnvironment.secrets.readOnly": {
    message:
      "Nur Workspace-Admins können Agent-Secrets hinzufügen, ändern oder löschen.",
    description: "Agent secrets card help line for non-admins",
  },
  "agentEnvironment.secrets.saved": {
    message: "{name} gespeichert",
    description: "Toast after a secret is saved",
  },
  "agentEnvironment.secrets.deleted": {
    message: "{name} gelöscht",
    description: "Toast after a secret is deleted",
  },
  "agentEnvironment.secrets.nameInvalid": {
    message:
      "Verwende Buchstaben, Ziffern und _, beginnend mit einem Buchstaben oder _ (höchstens 128).",
    description: "Secret name validation error",
  },
  "agentEnvironment.secrets.nameReserved": {
    message:
      "bex setzt diese Umgebungsvariable selbst, daher kann ein Secret diesen Namen nicht verwenden.",
    description: "Secret name reserved error",
  },
  "agentEnvironment.secrets.valueRequired": {
    message: "Gib einen Wert ein.",
    description: "Secret value required error",
  },
  "agentEnvironment.secrets.valueInvalid": {
    message: "Ein Wert darf höchstens 32 KiB groß sein.",
    description: "Secret value too long (or contains a NUL byte)",
  },
  "agentEnvironment.secrets.limitExceeded": {
    message: "Das Limit für Agent-Secrets ist erreicht.",
    description: "Secret count or size limit error",
  },
  "agentEnvironment.secrets.notFound": {
    message:
      "Dieses Secret existiert nicht mehr; möglicherweise hat es gerade jemand gelöscht.",
    description: "AGENT_SECRET_NOT_FOUND error (deleted concurrently)",
  },
  "agentEnvironment.secrets.unavailable": {
    message: "Agent-Secrets sind in diesem Deployment nicht verfügbar.",
    description: "AGENT_SECRETS_UNAVAILABLE error",
  },
  "agentEnvironment.secrets.forbidden": {
    message: "Nur Workspace-Admins können Agent-Secrets ändern.",
    description: "Secret write refused for a non-admin",
  },
  "agentEnvironment.secrets.saveFailed": {
    message:
      "Die Änderung konnte nicht gespeichert werden. Versuche es erneut.",
    description: "Generic secret write failure",
  },
  "agentEnvironment.secrets.valueKeepPlaceholder": {
    message: "Aktuellen Wert beibehalten",
    description:
      "Agent secrets card: value placeholder when the name is an existing secret (empty keeps its value)",
  },
  "agentEnvironment.secrets.editLabel": {
    message: "{name} bearbeiten",
    description:
      "Accessible label of a secret's edit button (loads it into the form to rotate or rescope)",
  },
  "agentEnvironment.secrets.scopeLabel": {
    message: "Geltungsbereich",
    description:
      "Agent secrets card: accessible label of the scope picker (ADR098 D9)",
  },
  "agentEnvironment.secrets.scopeWorkspace": {
    message: "Alle Sitzungen",
    description:
      "Scope picker option: every session in the workspace receives the secret",
  },
  "agentEnvironment.secrets.scopeRepo": {
    message: "Ein Repository",
    description:
      "Scope picker option: sessions on one repository receive the secret",
  },
  "agentEnvironment.secrets.scopeExplicit": {
    message: "Nur wenn ausgewählt",
    description:
      "Scope picker option: only sessions that pick the secret receive it",
  },
  "agentEnvironment.secrets.scopeBadgeRepo": {
    message: "Repository: {repo}",
    description:
      "Secret row badge: repo scope, naming the repository (owner/name)",
  },
  "agentEnvironment.secrets.repoLabel": {
    message: "Repository",
    description:
      "Agent secrets card: accessible label of the repo scope's repository field",
  },
  "agentEnvironment.secrets.repoPlaceholder": {
    message: "owner/name",
    description: "Agent secrets card: repository field placeholder",
  },
  "agentEnvironment.secrets.repoEmpty": {
    message: "Kein verbundenes Repository passt.",
    description:
      "Agent secrets card: repository picker with no matching connected repository",
  },
  "agentEnvironment.secrets.repoInvalid": {
    message: "Gib das Repository als owner/name ein.",
    description:
      "Repo scope validation error (also AGENT_SECRET_SCOPE_INVALID)",
  },
  "agentEnvironment.repoEnvs.title": {
    message: "Repository-Umgebungen",
    description: "Settings → Repository environments card title (ADR109)",
  },
  "agentEnvironment.repoEnvs.description": {
    message:
      "Wie Agent-Sitzungen jedes Repository vorbereiten. Setup-Schritte laufen der Reihe nach im Repository-Stammverzeichnis, bevor der Agent einer Sitzung startet, und zwar mit den Secrets der Sitzung. Checks sind die Befehle, die der Agent vor der Abgabe ausführt, etwa Lint und Test.",
    description:
      "Repository environments card description: what setup steps and checks are",
  },
  "agentEnvironment.repoEnvs.loadFailed": {
    message: "Die Repository-Umgebungen konnten nicht geladen werden.",
    description: "Repository environments card error when the list query fails",
  },
  "agentEnvironment.repoEnvs.emptyList": {
    message: "Noch keine Repository-Umgebungen.",
    description: "Repository environments card empty state",
  },
  "agentEnvironment.repoEnvs.setupSummary": {
    message: "Setup: {names}",
    description:
      "Repository row summary: the setup step names, comma-separated",
  },
  "agentEnvironment.repoEnvs.checksSummary": {
    message: "Checks: {names}",
    description: "Repository row summary: the check names, comma-separated",
  },
  "agentEnvironment.repoEnvs.noSetup": {
    message: "Keine Setup-Schritte",
    description:
      "Repository row summary (and the read-only dialog) when the environment has no setup steps",
  },
  "agentEnvironment.repoEnvs.noChecks": {
    message: "Keine Checks",
    description:
      "Repository row summary (and the read-only dialog) when the environment has no checks",
  },
  "agentEnvironment.repoEnvs.editLabel": {
    message: "{repo} bearbeiten",
    description:
      "Accessible label of a repository environment's edit button (repo is owner/name)",
  },
  "agentEnvironment.repoEnvs.viewLabel": {
    message: "{repo} ansehen",
    description:
      "Accessible label of a repository environment's view button, for non-admins",
  },
  "agentEnvironment.repoEnvs.deleteLabel": {
    message: "Umgebung für {repo} löschen",
    description: "Accessible label of a repository environment's delete button",
  },
  "agentEnvironment.repoEnvs.deleteTitle": {
    message: "Umgebung für {repo} löschen?",
    description: "Delete-environment confirmation title",
  },
  "agentEnvironment.repoEnvs.deleteBody": {
    message:
      "Ab ihrem nächsten Durchgang starten Sitzungen für dieses Repository ohne dessen Setup-Schritte und Checks.",
    description: "Delete-environment confirmation body",
  },
  "agentEnvironment.repoEnvs.deleteConfirm": {
    message: "Umgebung löschen",
    description: "Delete-environment confirmation button",
  },
  "agentEnvironment.repoEnvs.add": {
    message: "Umgebung hinzufügen",
    description:
      "Repository environments card: button that opens the add-environment dialog",
  },
  "agentEnvironment.repoEnvs.help": {
    message:
      "Beim Speichern wird die gesamte Umgebung des Repositorys ersetzt. Sitzungen übernehmen Änderungen ab ihrem nächsten Durchgang.",
    description: "Repository environments card help line for admins",
  },
  "agentEnvironment.repoEnvs.readOnly": {
    message:
      "Nur Workspace-Admins können Repository-Umgebungen hinzufügen, ändern oder löschen.",
    description: "Repository environments card help line for non-admins",
  },
  "agentEnvironment.repoEnvs.saved": {
    message: "Umgebung für {repo} gespeichert",
    description: "Toast after an environment is saved",
  },
  "agentEnvironment.repoEnvs.deleted": {
    message: "Umgebung für {repo} gelöscht",
    description: "Toast after an environment is deleted",
  },
  "agentEnvironment.repoEnvs.createTitle": {
    message: "Repository-Umgebung hinzufügen",
    description: "Add-environment dialog title",
  },
  "agentEnvironment.repoEnvs.editTitle": {
    message: "{repo} bearbeiten",
    description: "Edit-environment dialog title (repo is owner/name)",
  },
  "agentEnvironment.repoEnvs.dialogDescription": {
    message:
      "Setup-Schritte laufen der Reihe nach im Repository-Stammverzeichnis, bevor der Agent einer Sitzung startet, und zwar mit den Secrets der Sitzung. Schlägt ein Schritt fehl, endet der Durchgang, bevor der Agent startet. Checks führt der Agent vor der Abgabe aus.",
    description: "Environment dialog description",
  },
  "agentEnvironment.repoEnvs.repoLabel": {
    message: "Repository",
    description: "Environment dialog: repository field label",
  },
  "agentEnvironment.repoEnvs.repoPlaceholder": {
    message: "owner/name",
    description: "Environment dialog: repository field placeholder",
  },
  "agentEnvironment.repoEnvs.repoEmpty": {
    message: "Kein verbundenes Repository passt.",
    description:
      "Environment dialog: repository picker with no matching connected repository",
  },
  "agentEnvironment.repoEnvs.setupTitle": {
    message: "Setup-Schritte",
    description: "Environment dialog: setup steps section heading",
  },
  "agentEnvironment.repoEnvs.setupHint": {
    message:
      "Installiere Tools und Abhängigkeiten. Jeder Schritt läuft der Reihe nach mit bash; höchstens 10 Schritte, je 10 Minuten.",
    description: "Environment dialog: setup steps section hint",
  },
  "agentEnvironment.repoEnvs.checksTitle": {
    message: "Checks",
    description: "Environment dialog: checks section heading",
  },
  "agentEnvironment.repoEnvs.checksHint": {
    message:
      "Befehle wie Lint, Test und Build. Der Agent führt jeden im Repository-Stammverzeichnis aus, bevor er abgibt, und meldet die Ergebnisse; bex führt sie nicht selbst aus. Höchstens 10.",
    description: "Environment dialog: checks section hint",
  },
  "agentEnvironment.repoEnvs.addSetup": {
    message: "Setup-Schritt hinzufügen",
    description: "Environment dialog: add a setup step row",
  },
  "agentEnvironment.repoEnvs.addCheck": {
    message: "Check hinzufügen",
    description: "Environment dialog: add a check row",
  },
  "agentEnvironment.repoEnvs.setupNameLabel": {
    message: "Name von Setup-Schritt {position}",
    description:
      "Accessible label of a setup step's name field (position is its 1-based place in the list)",
  },
  "agentEnvironment.repoEnvs.setupRunLabel": {
    message: "Befehl von Setup-Schritt {position}",
    description: "Accessible label of a setup step's command field",
  },
  "agentEnvironment.repoEnvs.setupMoveUp": {
    message: "Setup-Schritt {position} nach oben verschieben",
    description: "Accessible label: move a setup step one place earlier",
  },
  "agentEnvironment.repoEnvs.setupMoveDown": {
    message: "Setup-Schritt {position} nach unten verschieben",
    description: "Accessible label: move a setup step one place later",
  },
  "agentEnvironment.repoEnvs.setupRemove": {
    message: "Setup-Schritt {position} entfernen",
    description: "Accessible label: remove a setup step row",
  },
  "agentEnvironment.repoEnvs.checkNameLabel": {
    message: "Name von Check {position}",
    description:
      "Accessible label of a check's name field (position is its 1-based place in the list)",
  },
  "agentEnvironment.repoEnvs.checkRunLabel": {
    message: "Befehl von Check {position}",
    description: "Accessible label of a check's command field",
  },
  "agentEnvironment.repoEnvs.checkMoveUp": {
    message: "Check {position} nach oben verschieben",
    description: "Accessible label: move a check one place earlier",
  },
  "agentEnvironment.repoEnvs.checkMoveDown": {
    message: "Check {position} nach unten verschieben",
    description: "Accessible label: move a check one place later",
  },
  "agentEnvironment.repoEnvs.checkRemove": {
    message: "Check {position} entfernen",
    description: "Accessible label: remove a check row",
  },
  "agentEnvironment.repoEnvs.setupNamePlaceholder": {
    message: "deps",
    description: "Setup step name placeholder (an example step name)",
  },
  "agentEnvironment.repoEnvs.setupRunPlaceholder": {
    message: "npm ci",
    description: "Setup step command placeholder (an example shell command)",
  },
  "agentEnvironment.repoEnvs.checkNamePlaceholder": {
    message: "test",
    description: "Check name placeholder (an example check name)",
  },
  "agentEnvironment.repoEnvs.checkRunPlaceholder": {
    message: "npm test",
    description: "Check command placeholder (an example shell command)",
  },
  "agentEnvironment.repoEnvs.close": {
    message: "Schließen",
    description: "Read-only environment dialog: close button",
  },
  "agentEnvironment.repoEnvs.cancel": {
    message: "Abbrechen",
    description: "Environment dialog: cancel button",
  },
  "agentEnvironment.repoEnvs.save": {
    message: "Speichern",
    description: "Environment dialog: save (create or replace) button",
  },
  "agentEnvironment.repoEnvs.nameRequired": {
    message: "Gib einen Namen ein.",
    description: "Step or check name required error",
  },
  "agentEnvironment.repoEnvs.nameInvalid": {
    message:
      "Ein Name beginnt mit einem Buchstaben oder einer Ziffer und besteht aus Buchstaben, Ziffern, Leerzeichen, '.', '_' oder '-' (höchstens 64).",
    description: "Step or check name format error",
  },
  "agentEnvironment.repoEnvs.nameDuplicate": {
    message:
      "Eine andere Zeile in dieser Liste verwendet diesen Namen bereits.",
    description: "Step or check name repeated within its list",
  },
  "agentEnvironment.repoEnvs.runRequired": {
    message: "Gib einen Befehl ein.",
    description: "Step or check command required error",
  },
  "agentEnvironment.repoEnvs.setupRunTooLong": {
    message: "Ein Setup-Befehl darf höchstens 8 KiB groß sein.",
    description: "Setup step command too long (or contains a NUL byte)",
  },
  "agentEnvironment.repoEnvs.checkRunTooLong": {
    message: "Ein Check-Befehl darf höchstens 2 KiB groß sein.",
    description: "Check command too long (or contains a NUL byte)",
  },
  "agentEnvironment.repoEnvs.setupTooMany": {
    message: "Eine Umgebung kann höchstens 10 Setup-Schritte haben.",
    description: "Too many setup steps",
  },
  "agentEnvironment.repoEnvs.checksTooMany": {
    message: "Eine Umgebung kann höchstens 10 Checks haben.",
    description: "Too many checks",
  },
  "agentEnvironment.repoEnvs.environmentEmpty": {
    message:
      "Füge mindestens einen Setup-Schritt oder Check hinzu. Um alle zu entfernen, lösche stattdessen die Umgebung.",
    description: "Environment with neither setup steps nor checks",
  },
  "agentEnvironment.repoEnvs.repoInvalid": {
    message: "Gib das Repository als owner/name ein.",
    description:
      "Repository validation error (also AGENT_REPO_ENVIRONMENT_INVALID on the repository)",
  },
  "agentEnvironment.repoEnvs.repoExists": {
    message:
      "Dieses Repository hat bereits eine Umgebung. Bearbeite sie stattdessen in der Liste.",
    description: "Adding an environment for a repository that already has one",
  },
  "agentEnvironment.repoEnvs.setupRefused": {
    message:
      "Ein Setup-Schritt wurde abgelehnt. Prüfe, ob jeder Name gültig und eindeutig ist und jeder Befehl höchstens 8 KiB groß ist.",
    description: "AGENT_REPO_ENVIRONMENT_INVALID on the setup steps",
  },
  "agentEnvironment.repoEnvs.checksRefused": {
    message:
      "Ein Check wurde abgelehnt. Prüfe, ob jeder Name gültig und eindeutig ist und jeder Befehl höchstens 2 KiB groß ist.",
    description: "AGENT_REPO_ENVIRONMENT_INVALID on the checks",
  },
  "agentEnvironment.repoEnvs.invalid": {
    message:
      "Diese Umgebung wurde abgelehnt. Prüfe ihr Repository, ihre Setup-Schritte und ihre Checks.",
    description: "AGENT_REPO_ENVIRONMENT_INVALID without a recognized field",
  },
  "agentEnvironment.repoEnvs.limitExceeded": {
    message:
      "Dieser Workspace hat sein Limit von 50 Repository-Umgebungen erreicht. Lösche eine, um eine weitere hinzuzufügen.",
    description:
      "AGENT_REPO_ENVIRONMENT_LIMIT_EXCEEDED (also checked before saving)",
  },
  "agentEnvironment.repoEnvs.notFound": {
    message:
      "Dieses Repository hat keine Umgebung mehr; möglicherweise hat sie gerade jemand gelöscht.",
    description:
      "AGENT_REPO_ENVIRONMENT_NOT_FOUND error (deleted concurrently)",
  },
  "agentEnvironment.repoEnvs.unavailable": {
    message: "Repository-Umgebungen sind in diesem Deployment nicht verfügbar.",
    description: "AGENT_REPO_ENVIRONMENTS_UNAVAILABLE error",
  },
  "agentEnvironment.repoEnvs.forbidden": {
    message: "Nur Workspace-Admins können Repository-Umgebungen ändern.",
    description: "Environment write refused for a non-admin",
  },
  "agentEnvironment.repoEnvs.saveFailed": {
    message:
      "Die Änderung konnte nicht gespeichert werden. Versuche es erneut.",
    description: "Generic environment write failure",
  },
  "agentEnvironment.repoEnvs.builds.status.queued": {
    message: "In Warteschlange",
    description: "Build status badge: waiting to start (ADR111 D2)",
  },
  "agentEnvironment.repoEnvs.builds.status.building": {
    message: "Wird gebaut",
    description: "Build status badge: the setup steps are running",
  },
  "agentEnvironment.repoEnvs.builds.status.success": {
    message: "Bereit",
    description:
      "Build status badge: success — sessions can start from this snapshot",
  },
  "agentEnvironment.repoEnvs.builds.status.failed": {
    message: "Fehlgeschlagen",
    description: "Build status badge: the build failed",
  },
  "agentEnvironment.repoEnvs.builds.status.cancelled": {
    message: "Abgebrochen",
    description:
      "Build status badge: a newer build replaced it, or its environment was deleted",
  },
  "agentEnvironment.repoEnvs.builds.trigger.environmentSaved": {
    message: "Setup-Schritte geändert",
    description:
      "What queued a build: the environment was saved with changed setup steps",
  },
  "agentEnvironment.repoEnvs.builds.trigger.secretChanged": {
    message: "Repository-Secret geändert",
    description: "What queued a build: a repo-scoped secret was set or changed",
  },
  "agentEnvironment.repoEnvs.builds.trigger.manual": {
    message: "Manueller Build",
    description:
      "What queued a build: an admin clicked Build now (or used the API)",
  },
  "agentEnvironment.repoEnvs.builds.trigger.refresh": {
    message: "Tägliche Aktualisierung",
    description: "What queued a build: the daily rebuild",
  },
  "agentEnvironment.repoEnvs.builds.failure.setupFailed": {
    message: "Ein Setup-Schritt ist fehlgeschlagen.",
    description:
      "Failed build, failureCode setup_failed (the reason below names the step and exit code)",
  },
  "agentEnvironment.repoEnvs.builds.failure.stepTimeout": {
    message: "Ein Setup-Schritt hat sein Limit von 3 Stunden überschritten.",
    description: "Failed build, failureCode step_timeout",
  },
  "agentEnvironment.repoEnvs.builds.failure.buildTimeout": {
    message: "Der Build hat sein Limit von 3 Stunden überschritten.",
    description: "Failed build, failureCode build_timeout",
  },
  "agentEnvironment.repoEnvs.builds.failure.checkoutFailed": {
    message:
      "Der Standard-Branch des Repositorys konnte nicht ausgecheckt werden.",
    description: "Failed build, failureCode checkout_failed",
  },
  "agentEnvironment.repoEnvs.builds.failure.storageLimitReached": {
    message:
      "Der Snapshot würde das Sandbox-Speicherlimit dieses Workspace überschreiten.",
    description: "Failed build, failureCode storage_limit_reached",
  },
  "agentEnvironment.repoEnvs.builds.failure.internalError": {
    message:
      "bex konnte den Build auf seiner Seite nicht abschließen. Versuche es erneut.",
    description: "Failed build, failureCode internal_error",
  },
  "agentEnvironment.repoEnvs.builds.failure.unknown": {
    message: "Der Build ist fehlgeschlagen.",
    description: "Failed build with no recognized failureCode",
  },
  "agentEnvironment.repoEnvs.builds.title": {
    message: "Builds von {repo}",
    description: "Build log dialog title (repo is owner/name)",
  },
  "agentEnvironment.repoEnvs.builds.openLabel": {
    message: "Builds von {repo}",
    description:
      "Accessible label of a repository row's button that opens its builds and logs",
  },
  "agentEnvironment.repoEnvs.builds.description": {
    message:
      "Sitzungen starten vom neuesten erfolgreichen Build, dessen Setup-Schritte mit den aktuellen übereinstimmen. Gibt es keinen solchen, führen sie stattdessen zu Beginn jedes Durchgangs die Setup-Schritte aus.",
    description:
      "Build log dialog description: how sessions use snapshots and the fallback (ADR111 D4)",
  },
  "agentEnvironment.repoEnvs.builds.buildNow": {
    message: "Jetzt bauen",
    description:
      "Button that queues a build of the repository's setup steps (admins only)",
  },
  "agentEnvironment.repoEnvs.builds.snapshotNote": {
    message:
      "Dateien, die die Setup-Schritte schreiben, bleiben im Snapshot erhalten. Schreibe daher keine Zugangsdaten in Dateien. Secrets erreichen die Schritte als Umgebungsvariablen, die nicht gespeichert werden.",
    description: "Note beside Build now (Devin's warning, ADR111 D6)",
  },
  "agentEnvironment.repoEnvs.builds.readOnly": {
    message: "Nur Workspace-Admins können einen Build starten.",
    description: "Shown under a disabled Build now for non-admins",
  },
  "agentEnvironment.repoEnvs.builds.forbidden": {
    message: "Nur Workspace-Admins können einen Build starten.",
    description: "Build now refused with FORBIDDEN",
  },
  "agentEnvironment.repoEnvs.builds.nothingToBuild": {
    message:
      "Diese Umgebung hat keine Setup-Schritte, daher gibt es nichts zu bauen. Checks benötigen keinen Build.",
    description:
      "Build now on a checks-only environment (also AGENT_REPO_ENVIRONMENT_NOTHING_TO_BUILD)",
  },
  "agentEnvironment.repoEnvs.builds.queued": {
    message: "Build von {repo} in die Warteschlange gestellt",
    description: "Toast after Build now succeeds",
  },
  "agentEnvironment.repoEnvs.builds.rateLimited": {
    message:
      "Dieses Repository hat in den letzten 24 Stunden bereits 12 Builds gestartet. Versuche es später erneut.",
    description:
      "AGENT_REPO_ENVIRONMENT_BUILD_RATE_LIMITED without a usable nextBuildAt",
  },
  "agentEnvironment.repoEnvs.builds.rateLimitedUntil": {
    message:
      "Dieses Repository hat in den letzten 24 Stunden bereits 12 Builds gestartet. Der nächste Build kann um {time} starten.",
    description:
      "AGENT_REPO_ENVIRONMENT_BUILD_RATE_LIMITED; time is nextBuildAt in the viewer's local date and time",
  },
  "agentEnvironment.repoEnvs.builds.unavailable": {
    message: "Umgebungs-Builds sind in diesem Deployment nicht verfügbar.",
    description: "AGENT_REPO_ENVIRONMENT_BUILDS_UNAVAILABLE",
  },
  "agentEnvironment.repoEnvs.builds.buildFailed": {
    message: "Der Build konnte nicht gestartet werden. Versuche es erneut.",
    description: "Generic Build now failure",
  },
  "agentEnvironment.repoEnvs.builds.loadFailed": {
    message: "Die Builds konnten nicht geladen werden.",
    description: "Build history query failed",
  },
  "agentEnvironment.repoEnvs.builds.none": {
    message: "Noch keine Builds.",
    description: "Build history list with no builds",
  },
  "agentEnvironment.repoEnvs.builds.noneLog": {
    message:
      "Dieses Repository wurde noch nicht gebaut. „Jetzt bauen“ führt seine Setup-Schritte einmal aus und behält das Ergebnis als Snapshot.",
    description: "Build log panel when the repository has no builds",
  },
  "agentEnvironment.repoEnvs.builds.historyLabel": {
    message: "Build-Verlauf",
    description: "Accessible label of the build history list",
  },
  "agentEnvironment.repoEnvs.builds.logTitle": {
    message: "Build-Log",
    description: "Build log panel header when no build is selected",
  },
  "agentEnvironment.repoEnvs.builds.logFailed": {
    message: "Das Log dieses Builds konnte nicht geladen werden.",
    description: "Single build (log) query failed",
  },
  "agentEnvironment.repoEnvs.builds.took": {
    message: "dauerte {duration}",
    description:
      "A finished build's wall time, e.g. 'took 3m 12s' (joined with ' · ')",
  },
  "agentEnvironment.repoEnvs.builds.imageRemoved": {
    message: "Snapshot entfernt",
    description:
      "A successful build whose snapshot image was since deleted (older builds keep no image)",
  },
  "agentEnvironment.repoEnvs.builds.notBuilt": {
    message: "Noch nicht gebaut",
    description: "Repository row: has setup steps but no build yet",
  },
  "agentEnvironment.repoEnvs.builds.noSteps.queued": {
    message:
      "Dieser Build wartet auf den Start. Builds teilen sich das Agent-Sandbox-Limit des Workspace mit Sitzungen.",
    description: "Build log of a queued build",
  },
  "agentEnvironment.repoEnvs.builds.noSteps.building": {
    message:
      "Die Setup-Schritte laufen. Ihr Log erscheint hier, sobald der Build abgeschlossen ist.",
    description: "Build log of a running build",
  },
  "agentEnvironment.repoEnvs.builds.noSteps.cancelled": {
    message:
      "Dieser Build wurde abgebrochen: Ein neuerer Build hat ihn ersetzt, oder seine Umgebung wurde gelöscht.",
    description: "Build log of a cancelled build",
  },
  "agentEnvironment.repoEnvs.builds.noSteps.failed": {
    message: "Es wurde kein Setup-Schritt ausgeführt.",
    description:
      "Build log of a build that failed before any step ran (e.g. the checkout failed)",
  },
  "agentEnvironment.repoEnvs.builds.noSteps.other": {
    message: "Dieser Build hat keine Schritte aufgezeichnet.",
    description: "Build log with no steps, any other status",
  },
  "agentEnvironment.repoEnvs.builds.stepExit": {
    message: "Exit {code}",
    description: "A setup step's exit code badge",
  },
  "agentEnvironment.repoEnvs.builds.stepKilled": {
    message: "gestoppt",
    description: "A setup step with no exit code: killed or never started",
  },
  "agentEnvironment.repoEnvs.builds.noOutput": {
    message: "Keine Ausgabe.",
    description: "A setup step that printed nothing",
  },
  "agentEnvironment.skills.title": {
    message: "Agent-Skills",
    description: "Settings → Agent skills card title (ADR099)",
  },
  "agentEnvironment.skills.description": {
    message:
      "Wiederverwendbare Anweisungen, die jede Agent-Sitzung in diesem Workspace als SKILL.md-Dateien erhält. Der Agent wendet einen Skill an, wenn er zur Aufgabe passt, oder nur auf Anfrage.",
    description: "Agent skills card description",
  },
  "agentEnvironment.skills.loadFailed": {
    message: "Die Agent-Skills konnten nicht geladen werden.",
    description: "Agent skills card error when the list query fails",
  },
  "agentEnvironment.skills.empty": {
    message: "Noch keine Agent-Skills.",
    description: "Agent skills card empty state",
  },
  "agentEnvironment.skills.onlyWhenAsked": {
    message: "Nur auf Anfrage",
    description:
      "Badge on a skill the agent applies only when a prompt invokes it",
  },
  "agentEnvironment.skills.enabledLabel": {
    message: "{name} aktivieren",
    description: "Accessible label of a skill's enabled switch",
  },
  "agentEnvironment.skills.editLabel": {
    message: "{name} bearbeiten",
    description: "Accessible label of a skill's edit button",
  },
  "agentEnvironment.skills.viewLabel": {
    message: "{name} ansehen",
    description: "Accessible label of a skill's view button (non-admins)",
  },
  "agentEnvironment.skills.exportLabel": {
    message: "{name} als SKILL.md herunterladen",
    description: "Accessible label of a skill's export button",
  },
  "agentEnvironment.skills.deleteLabel": {
    message: "{name} löschen",
    description: "Accessible label of a skill's delete button",
  },
  "agentEnvironment.skills.deleteTitle": {
    message: "Skill {name} löschen?",
    description: "Delete skill confirm title",
  },
  "agentEnvironment.skills.deleteBody": {
    message:
      "Neue Agent-Durchgänge erhalten ihn nicht mehr. Bereits gelaufene Sitzungen behalten ihr Transkript.",
    description: "Delete skill confirm body",
  },
  "agentEnvironment.skills.deleteConfirm": {
    message: "Skill löschen",
    description: "Delete skill confirm button",
  },
  "agentEnvironment.skills.new": {
    message: "Neuer Skill",
    description: "Button that opens the create-skill dialog",
  },
  "agentEnvironment.skills.import": {
    message: "SKILL.md importieren",
    description: "Button that imports a SKILL.md file",
  },
  "agentEnvironment.skills.help": {
    message:
      "Mitglieder können Skills verwenden; nur Admins können sie hinzufügen oder ändern.",
    description: "Agent skills help line for admins",
  },
  "agentEnvironment.skills.readOnly": {
    message: "Nur Workspace-Admins können Agent-Skills hinzufügen oder ändern.",
    description: "Agent skills help line for non-admins",
  },
  "agentEnvironment.skills.createTitle": {
    message: "Neuer Skill",
    description: "Create-skill dialog title",
  },
  "agentEnvironment.skills.editTitle": {
    message: "{name} bearbeiten",
    description: "Edit-skill dialog title",
  },
  "agentEnvironment.skills.viewTitle": {
    message: "{name}",
    description: "View-skill dialog title (read-only)",
  },
  "agentEnvironment.skills.dialogDescription": {
    message:
      "Der Agent liest die Beschreibung, um zu entscheiden, wann der Skill zutrifft, und befolgt die Anweisungen, wenn das der Fall ist.",
    description: "Skill dialog description",
  },
  "agentEnvironment.skills.nameLabel": {
    message: "Name",
    description: "Skill name field label",
  },
  "agentEnvironment.skills.namePlaceholder": {
    message: "run-integration-tests",
    description: "Skill name field placeholder",
  },
  "agentEnvironment.skills.descriptionLabel": {
    message: "Beschreibung",
    description: "Skill description field label",
  },
  "agentEnvironment.skills.descriptionPlaceholder": {
    message: "Wann dieser Skill verwendet wird, in einer Zeile",
    description: "Skill description field placeholder",
  },
  "agentEnvironment.skills.onlyWhenAskedLabel": {
    message: "Nur wenn ich es verlange",
    description: "Toggle: the agent applies the skill only when invoked",
  },
  "agentEnvironment.skills.onlyWhenAskedHint": {
    message:
      "Rufe ihn über das Eingabefeld oder mit @skills:name in einem Prompt auf.",
    description: "Hint under the only-when-asked toggle",
  },
  "agentEnvironment.skills.bodyLabel": {
    message: "Anweisungen",
    description: "Skill body field label",
  },
  "agentEnvironment.skills.bodyPlaceholder": {
    message: "Schritte, denen der Agent folgt, in Markdown.",
    description: "Skill body field placeholder",
  },
  "agentEnvironment.skills.contentLabel": {
    message: "SKILL.md",
    description: "Whole-file editor label",
  },
  "agentEnvironment.skills.editAsFile": {
    message: "Als SKILL.md bearbeiten",
    description: "Switch the dialog to the whole-file editor",
  },
  "agentEnvironment.skills.editAsFields": {
    message: "Felder bearbeiten",
    description: "Switch the dialog back to separate fields",
  },
  "agentEnvironment.skills.history.tab": {
    message: "Versionsverlauf",
    description: "Skill dialog tab listing the saved versions",
  },
  "agentEnvironment.skills.history.editTab": {
    message: "Bearbeiten",
    description: "Skill dialog tab with the editing form",
  },
  "agentEnvironment.skills.history.current": {
    message: "Aktuell",
    description: "Badge on the newest saved version",
  },
  "agentEnvironment.skills.history.changes": {
    message: "Änderungen",
    description: "Version view: line diff against the previous version",
  },
  "agentEnvironment.skills.history.content": {
    message: "Inhalt",
    description: "Version view: the full SKILL.md",
  },
  "agentEnvironment.skills.history.load": {
    message: "In den Editor laden",
    description: "Button that fills the editor with the selected version",
  },
  "agentEnvironment.skills.history.loadMore": {
    message: "Mehr laden",
    description: "Button that loads the next page of versions",
  },
  "agentEnvironment.skills.history.noChanges": {
    message: "Diese Version hat nichts geändert.",
    description: "Changes view of a version identical to the previous one",
  },
  "agentEnvironment.skills.history.loadFailed": {
    message: "Der Versionsverlauf konnte nicht geladen werden.",
    description: "Version history list query failed",
  },
  "agentEnvironment.skills.history.versionFailed": {
    message: "Diese Version konnte nicht geladen werden.",
    description: "Single version query failed",
  },
  "agentEnvironment.skills.history.loadedNotice": {
    message:
      "Version {version} geladen. Speichere, um sie als neue Version zu behalten.",
    description: "Notice in the editor after Load into editor",
  },
  "agentEnvironment.skills.viewTab": {
    message: "Vorschau",
    description: "Read-only skill dialog tab showing the skill",
  },
  "agentEnvironment.skills.save": {
    message: "Skill speichern",
    description: "Skill dialog save button",
  },
  "agentEnvironment.skills.cancel": {
    message: "Abbrechen",
    description: "Skill dialog cancel button",
  },
  "agentEnvironment.skills.close": {
    message: "Schließen",
    description: "Skill dialog close button (read-only)",
  },
  "agentEnvironment.skills.saved": {
    message: "{name} gespeichert",
    description: "Toast after a skill is saved",
  },
  "agentEnvironment.skills.deleted": {
    message: "{name} gelöscht",
    description: "Toast after a skill is deleted",
  },
  "agentEnvironment.skills.readFailed": {
    message: "Der Skill konnte nicht geöffnet werden. Versuche es erneut.",
    description: "Error when a single skill read fails",
  },
  "agentEnvironment.skills.importFailed": {
    message: "Diese Datei konnte nicht gelesen werden.",
    description: "Error when an imported file can't be read",
  },
  "agentEnvironment.skills.nameRequired": {
    message: "Gib einen Namen ein.",
    description: "Skill name empty",
  },
  "agentEnvironment.skills.nameInvalid": {
    message:
      "Verwende Kleinbuchstaben, Ziffern und einzelne Bindestriche, bis zu 64 Zeichen.",
    description: "Skill name fails the Agent Skills format",
  },
  "agentEnvironment.skills.descriptionRequired": {
    message: "Gib eine einzeilige Beschreibung ein.",
    description: "Skill description empty",
  },
  "agentEnvironment.skills.descriptionInvalid": {
    message:
      "Halte die Beschreibung auf eine Zeile mit höchstens 1.024 Zeichen.",
    description: "Skill description too long or multi-line",
  },
  "agentEnvironment.skills.bodyInvalid": {
    message: "Anweisungen sind auf 64 KiB begrenzt.",
    description: "Skill body too large",
  },
  "agentEnvironment.skills.invalid": {
    message:
      "Der Skill ist ungültig. Prüfe Name, Beschreibung und Anweisungen.",
    description: "AGENT_SKILL_INVALID error",
  },
  "agentEnvironment.skills.limitExceeded": {
    message:
      "Das Limit für Agent-Skills ist erreicht (100 pro Workspace, insgesamt 1 MiB aktiviert).",
    description: "AGENT_SKILL_LIMIT_EXCEEDED error",
  },
  "agentEnvironment.skills.conflict": {
    message:
      "Dieser Skill hat sich geändert, seit du ihn geöffnet hast, oder der Name ist vergeben. Schließe ihn und öffne ihn erneut, um den aktuellen Stand zu sehen.",
    description: "AGENT_SKILL_CONFLICT error",
  },
  "agentEnvironment.skills.notFound": {
    message: "Dieser Skill existiert nicht mehr.",
    description: "AGENT_SKILL_NOT_FOUND error",
  },
  "agentEnvironment.skills.unavailable": {
    message: "Agent-Skills sind in diesem Deployment nicht verfügbar.",
    description: "AGENT_SKILLS_UNAVAILABLE error",
  },
  "agentEnvironment.skills.forbidden": {
    message: "Nur Workspace-Admins können Agent-Skills ändern.",
    description: "Skill write refused for a non-admin",
  },
  "agentEnvironment.skills.saveFailed": {
    message:
      "Die Änderung konnte nicht gespeichert werden. Versuche es erneut.",
    description: "Generic skill write failure",
  },
  "agentEnvironment.agentsTitle": {
    message: "Verfügbare Agents",
    description: "Settings → Agents: availability section heading",
  },
  "agentEnvironment.agentsDescription": {
    message:
      "Wähle, mit welchen Agents Mitglieder Sitzungen starten können. Laufende Sitzungen laufen weiter, wenn du einen Agent ausschaltest.",
    description: "Settings → Agents: availability section description",
  },
  "agentEnvironment.agentsDataUseNote": {
    message:
      "Sein Modellanbieter darf mit dem trainieren, was seine Sitzungen senden.",
    description:
      "Settings → Agents: note under an agent whose provider trains on session data (Muse Code)",
  },
  "agentEnvironment.agentsDataUseTitle": {
    message: "{agent} einschalten?",
    description:
      "Confirm title before turning on an agent whose provider trains on session data",
  },
  "agentEnvironment.agentsDataUseBody.muse": {
    message:
      "Dieser Agent läuft auf der Contributor-Stufe von Meta. Meta darf die Prompts, den Code, die Dateien und den Kontext, die seine Sitzungen senden, sowie die erhaltenen Antworten zum Training seiner Modelle verwenden. Beim Einschalten wird festgehalten, dass du als Admin dies für den Workspace zur Kenntnis nimmst.",
    description: "Confirm body: Muse Code's data-use acknowledgement (ADR108)",
  },
  "agentEnvironment.agentsDataUseConfirm": {
    message: "Zur Kenntnis nehmen und einschalten",
    description: "Confirm button: acknowledge data use and enable the agent",
  },
  "agentEnvironment.agentsSaved": {
    message: "Verfügbare Agents aktualisiert",
    description: "Toast after an availability change",
  },
  "agentEnvironment.agentsSaveFailed": {
    message:
      "Die verfügbaren Agents konnten nicht aktualisiert werden. Versuche es erneut.",
    description: "Error after a failed availability change",
  },
  "agentEnvironment.githubCliLabel": {
    message: "GitHub CLI (gh)",
    description:
      "Label of the switch that gives the workspace's agent sessions gh, the GitHub CLI (w10/m524)",
  },
  "agentEnvironment.githubCliHelp": {
    message:
      "Ausgeschaltet steht gh in keiner Agent-Sitzung dieses Workspaces mehr zur Verfügung, auch nicht in laufenden; Git-Pushes auf den Sitzungs-Branch und die Pull-Request-Tools von bex funktionieren weiter.",
    description:
      "Help under the GitHub CLI switch: what turning it off does (w10/m524)",
  },
  "agentEnvironment.githubCliTurnedOn": {
    message: "GitHub CLI für Agent-Sitzungen eingeschaltet",
    description:
      "Toast after an admin turns the sessions' GitHub CLI on (w10/m524)",
  },
  "agentEnvironment.githubCliTurnedOff": {
    message: "GitHub CLI für Agent-Sitzungen ausgeschaltet",
    description:
      "Toast after an admin turns the sessions' GitHub CLI off (w10/m524)",
  },
  "agentEnvironment.githubCliSaveFailed": {
    message:
      "Die GitHub-CLI-Einstellung konnte nicht geändert werden. Versuche es erneut.",
    description: "Error after a failed GitHub CLI switch (w10/m524)",
  },
  "agentEnvironment.githubCliMandatoryLabel": {
    message: "Aus verbindlich machen",
    description:
      "Label of the switch that makes the workspace's GitHub CLI off a floor no automation or session can turn back on (w10/m561)",
  },
  "agentEnvironment.githubCliMandatoryHelp": {
    message:
      "Greift, solange GitHub CLI aus ist: Dann kann keine Automatisierung und keine Sitzung gh wieder einschalten.",
    description:
      "Help under the mandatory switch: it applies only while GitHub CLI is off (w10/m561)",
  },
  "agentEnvironment.githubCliMandatoryTurnedOn": {
    message: "GitHub CLI aus ist jetzt verbindlich",
    description:
      "Toast after an admin makes the workspace's GitHub CLI off mandatory (w10/m561)",
  },
  "agentEnvironment.githubCliMandatoryTurnedOff": {
    message: "GitHub CLI aus ist nicht mehr verbindlich",
    description:
      "Toast after an admin makes the workspace's GitHub CLI off no longer mandatory (w10/m561)",
  },
  "agentEnvironment.githubCliMandatorySaveFailed": {
    message:
      "Ob GitHub CLI aus verbindlich ist, konnte nicht geändert werden. Versuche es erneut.",
    description: "Error after a failed mandatory switch (w10/m561)",
  },
  "agentEnvironment.githubCliAutomationsLabel": {
    message: "GitHub CLI für Automatisierungen",
    description:
      "Label of the select that binds the gh of the sessions automations start: inherit, on or off (w10/m561)",
  },
  "agentEnvironment.githubCliAutomationsHelp": {
    message:
      "Das gh der Sitzungen, die Automatisierungen starten, sofern eine Automatisierung nichts Eigenes festlegt. „Übernehmen“ folgt dem Schalter oben.",
    description:
      "Help under the automations GitHub CLI select: an automation's own binding wins, and inherit follows the workspace switch (w10/m561)",
  },
  "agentEnvironment.githubCliAutomationsInherited": {
    message: "Automatisierungen übernehmen jetzt die GitHub-CLI-Einstellung",
    description:
      "Toast after an admin returns automations to inheriting the workspace's GitHub CLI switch (w10/m561)",
  },
  "agentEnvironment.githubCliAutomationsTurnedOn": {
    message: "GitHub CLI für Automatisierungen eingeschaltet",
    description:
      "Toast after an admin turns GitHub CLI on for the sessions automations start (w10/m561)",
  },
  "agentEnvironment.githubCliAutomationsTurnedOff": {
    message: "GitHub CLI für Automatisierungen ausgeschaltet",
    description:
      "Toast after an admin turns GitHub CLI off for the sessions automations start (w10/m561)",
  },
  "agentEnvironment.githubCliAutomationsSaveFailed": {
    message:
      "GitHub CLI für Automatisierungen konnte nicht geändert werden. Versuche es erneut.",
    description:
      "Error after a failed automations GitHub CLI choice (w10/m561)",
  },
  "agentEnvironment.githubCliAutomationsLocked": {
    message:
      "GitHub CLI aus ist verbindlich, daher können Automatisierungen gh nicht einschalten.",
    description:
      "Mapped message for AGENT_ENVIRONMENT_GITHUB_CLI_LOCKED: automations cannot be bound on under a mandatory off (w10/m561)",
  },
  "agentEnvironment.modelsTitle": {
    message: "Modelle, die Mitglieder wählen können",
    description:
      "Settings → Agents: heading of the list of platform models members can choose (w10/m527)",
  },
  "agentEnvironment.modelsDescription": {
    message:
      "Wähle, welche Plattformmodelle Mitglieder für Sitzungen, Nachrichten und Automationen nutzen können. Eine Sitzung oder Automation, die schon ein Modell nutzt, das du ausschaltest, läuft ab ihrem nächsten Durchgang mit dem Standardmodell.",
    description:
      "Settings → Agents: description of the model list — what turning a model off does (w10/m527)",
  },
  "agentEnvironment.modelsDefaultLocked": {
    message:
      "Das Standardmodell der Plattform bleibt immer eingeschaltet, weil Sitzungen darauf zurückfallen.",
    description:
      "Note under the platform catalog's default model, whose switch is locked on (w10/m527)",
  },
  "agentEnvironment.modelsTurnedOn": {
    message: "{model} für Mitglieder eingeschaltet",
    description:
      "Toast after an admin turns a model on for members; {model} is its id (w10/m527)",
  },
  "agentEnvironment.modelsTurnedOff": {
    message: "{model} für Mitglieder ausgeschaltet",
    description:
      "Toast after an admin turns a model off for members; {model} is its id (w10/m527)",
  },
  "agentEnvironment.modelsInvalid": {
    message:
      "Die Modelle konnten nicht aktualisiert werden: Das Standardmodell der Plattform muss eingeschaltet bleiben, und die Liste darf nur Modelle des Plattformkatalogs enthalten. Lade die Seite neu, um den aktuellen Katalog zu sehen.",
    description:
      "Server refusal AGENT_ENVIRONMENT_MODELS_INVALID of a model list change (w10/m527)",
  },
  "agentEnvironment.modelsSaveFailed": {
    message:
      "Die Modelle, die Mitglieder wählen können, konnten nicht aktualisiert werden. Versuche es erneut.",
    description: "Error after a failed model switch (w10/m527)",
  },
  "agentEnvironment.modelOffOption": {
    message: "{model} (ausgeschaltet)",
    description:
      "Default model picker: the saved default, which an admin turned off (w10/m527)",
  },
  "agentEnvironment.modelDefaultOffNote": {
    message:
      "{model} ist ausgeschaltet, daher nutzen Sitzungen und Automationen ohne eigenes Modell den Plattform-Standard {fallback}, bis ein Admin ein eingeschaltetes Modell wählt oder {model} wieder einschaltet.",
    description:
      "Note under the default model picker when the saved default is turned off; {fallback} is the platform default that runs instead (w10/m527)",
  },
  "agentEnvironment.modelDefaultRetiredNote": {
    message:
      "{model} ist nicht mehr im Plattformkatalog, daher nutzen Sitzungen und Automationen ohne eigenes Modell den Plattform-Standard {fallback}, bis ein Admin ein anderes Modell wählt.",
    description:
      "Note under the default model picker when the saved default left the platform catalog; {fallback} is the platform default that runs instead (w10/m527)",
  },
};

export default deAgentEnvironment;
