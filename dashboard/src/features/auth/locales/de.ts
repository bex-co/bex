import type { TranslationEntry } from "@/i18n";

const deAuth: Record<string, TranslationEntry> = {
  "auth.loginTitle": {
    message: "Willkommen zurück",
    description: "Login page hero title",
  },
  "auth.loginSubtitle": {
    message: "Melde dich bei deinem Konto an",
    description: "Login page hero subtitle",
  },
  "auth.registerTitle": {
    message: "Erstelle dein Konto",
    description: "Registration page hero title",
  },
  "auth.registerSubtitle": {
    message: "Gib deine Daten ein, um loszulegen",
    description: "Registration page hero subtitle",
  },
  "auth.forgotPasswordTitle": {
    message: "Setze dein Passwort zurück",
    description: "Forgot-password page hero title",
  },
  "auth.forgotPasswordSubtitle": {
    message:
      "Gib die E-Mail-Adresse deines Kontos ein, um einen einmaligen Zugangscode zu erhalten",
    description: "Forgot-password page hero subtitle",
  },
  "auth.resetPasswordTitle": {
    message: "Neues Passwort festlegen",
    description:
      "Document title for the post-recovery reset-password landing page (renders the settings flow's password field)",
  },
  "auth.verificationTitle": {
    message: "Bestätige deine E-Mail-Adresse",
    description: "Verification page hero title",
  },
  "auth.verificationSubtitle": {
    message:
      "Gib die mit deinem Konto verknüpfte E-Mail-Adresse ein, um fortzufahren",
    description:
      "Verification page hero subtitle (email-first Ory step; code arrives after)",
  },
  "auth.verificationRequiredSubtitle": {
    message: "Bestätige deine E-Mail-Adresse, um bex weiter zu nutzen",
    description:
      "Verification page hero subtitle for a signed-in session whose email is not verified yet (ADR075 D8 verification wall)",
  },
  "auth.verificationWrongAccount": {
    message: "Nicht dein Konto?",
    description:
      "Verification wall: lead-in before the sign-out link, shown only to a signed-in session",
  },
  "auth.verificationSignOut": {
    message: "Abmelden",
    description:
      "Verification wall: sign-out link, shown only to a signed-in session",
  },
  "auth.settingsTitle": {
    message: "Einstellungen",
    description: "Account settings page heading",
  },
  "auth.settingsSubtitle": {
    message:
      "Verwalte deine Identität, Integrationen, Zugriffe und Sicherheit.",
    description: "Account settings page subheading",
  },
  "auth.settingsNavigation": {
    message: "Abschnitte der Einstellungen",
    description: "Accessible label for the settings page section navigation",
  },
  "auth.accountSection": {
    message: "Konto",
    description: "Settings page account section heading and navigation label",
  },
  "auth.accountSectionSubtitle": {
    message:
      "Aktualisiere dein Profil, dein Passwort und die Zwei-Faktor-Authentifizierung.",
    description: "Settings page account section description",
  },
  "auth.integrationsSection": {
    message: "Integrationen",
    description:
      "Settings page integrations section heading and navigation label",
  },
  "auth.integrationsSectionSubtitle": {
    message: "Verbinde die Quellcodeverwaltung.",
    description: "Settings page integrations section description",
  },
  "auth.accessSection": {
    message: "Zugangsdaten",
    description: "Settings page access section heading and navigation label",
  },
  "auth.accessSectionSubtitle": {
    message: "Verwalte programmatischen Zugriff und SSH-Identitäten.",
    description: "Settings page access section description",
  },
  "auth.securityComplianceSection": {
    message: "Sicherheit",
    description:
      "Settings page section heading grouping the account's security cards",
  },
  "auth.securityComplianceSectionSubtitle": {
    message:
      "Sicherheitseinstellungen des Kontos: verbundene Agents und aktive Sitzungen.",
    description: "Settings page Security & Compliance section description",
  },
  "auth.dangerZoneSection": {
    message: "Gefahrenzone",
    description: "Account settings destructive-actions section",
  },
  "auth.dangerZoneSectionSubtitle": {
    message: "Entferne dein Konto dauerhaft und widerrufe alle Zugriffe.",
    description: "Account deletion section description",
  },
  "auth.deleteAccountTitle": {
    message: "Konto löschen",
    description: "Account deletion card title",
  },
  "auth.deleteAccountDescription": {
    message:
      "Dadurch werden deine Identität, deine Zugangsdaten und Workspaces, in denen du das einzige Mitglied bist, dauerhaft gelöscht. Geteilte Workspaces bleiben für ihre anderen Admins verfügbar.",
    description: "Account deletion consequences",
  },
  "auth.deleteAccountWillDelete": {
    message: "Diese Workspaces werden gelöscht",
    description: "Account deletion preview heading for sole-member workspaces",
  },
  "auth.deleteAccountWillLeave": {
    message: "Du verlässt diese Workspaces",
    description: "Account deletion preview heading for shared workspaces",
  },
  "auth.deleteAccountBlockedTitle": {
    message: "Kläre zuerst die Workspace-Inhaberschaft",
    description: "Account deletion blocker heading",
  },
  "auth.deleteAccountBlockedDescription": {
    message:
      "Jeder Workspace unten hat weitere Mitglieder, aber keinen weiteren Admin. Befördere ein anderes Mitglied, entferne die anderen Mitglieder oder lösche den Workspace, bevor du dein Konto löschst.",
    description: "Actionable account deletion blocker guidance",
  },
  "auth.deleteAccountBlockedAction": {
    message: "Einstellungen öffnen",
    description: "Link to resolve an account deletion workspace blocker",
  },
  "auth.deleteAccountConfirmLabel": {
    message: "Gib unten {phrase} zur Bestätigung ein.",
    description: "Exact account deletion confirmation prompt",
  },
  "auth.deleteAccountSubmit": {
    message: "Mein Konto löschen",
    description: "Account deletion submit button",
  },
  "auth.deleteAccountErrorTitle": {
    message: "Die Kontolöschung konnte nicht gestartet werden",
    description: "Account deletion mutation error heading",
  },
  "auth.deleteAccountError": {
    message:
      "Wir konnten die Kontolöschung nicht starten. Bitte versuche es erneut.",
    description: "Generic account deletion mutation error",
  },
  "auth.deleteAccountPreviewErrorTitle": {
    message: "Vorschau der Löschung nicht verfügbar",
    description: "Account deletion preview error heading",
  },
  "auth.deleteAccountPreviewError": {
    message:
      "Wir konnten nicht sicher ermitteln, was mit deinen Workspaces geschieht. Es wurde nichts gelöscht.",
    description: "Account deletion preview failure explanation",
  },
  "auth.deleteAccountRetry": {
    message: "Erneut versuchen",
    description: "Account deletion preview retry button",
  },
  "auth.errorTitle": {
    message: "Etwas ist schiefgelaufen",
    description:
      "Title of /auth/error, where Kratos sends a self-service error (w5/161)",
  },
  "auth.errorSubtitle": {
    message: "Wir konnten diesen Schritt nicht abschließen.",
    description: "Subtitle of /auth/error",
  },
  "auth.errorGeneric": {
    message:
      "Der Anmeldeschritt ist abgelaufen oder konnte nicht abgeschlossen werden. Beginne erneut auf der Anmeldeseite.",
    description:
      "/auth/error body when Kratos's error has no readable reason or the id is unknown",
  },
  "auth.errorBackToSignIn": {
    message: "Zurück zur Anmeldung",
    description: "/auth/error button to the sign-in page",
  },
  "auth.errorFlowExpired": {
    message:
      "Dieser Anmeldeschritt hat zu lange gedauert und ist abgelaufen. Beginne erneut auf der Anmeldeseite.",
    description: "/auth/error message for Kratos self_service_flow_expired",
  },
  "auth.errorCsrf": {
    message:
      "Die Sicherheitsprüfung deines Browsers stimmte nicht überein — meist wegen eines zweiten Tabs oder gelöschter Cookies. Beginne erneut auf der Anmeldeseite.",
    description: "/auth/error message for Kratos security_csrf_violation",
  },
  "auth.errorIdentityMismatch": {
    message:
      "Dieser Schritt gehört zu einem anderen Konto als dem angemeldeten. Melde dich ab und beginne dann erneut.",
    description: "/auth/error message for Kratos security_identity_mismatch",
  },
  "auth.errorReturnToForbidden": {
    message:
      "Die Adresse, zu der du nach der Anmeldung zurückkehren sollst, ist nicht erlaubt.",
    description:
      "/auth/error message for Kratos self_service_flow_return_to_forbidden",
  },
  "auth.accountDeletedTitle": {
    message: "Kontolöschung gestartet",
    description: "Accepted account deletion terminal page title",
  },
  "auth.accountDeletedSubtitle": {
    message: "Du wurdest sicher abgemeldet.",
    description: "Accepted account deletion terminal page subtitle",
  },
  "auth.accountDeletedStatus": {
    message:
      "Deine Workspaces und Zugangsdaten werden entfernt. Du kannst diese Seite schließen; weitere Schritte sind nicht nötig.",
    description: "Accepted account deletion background completion status",
  },
  "auth.accountDeletedHome": {
    message: "Zurück zu bex",
    description: "Accepted account deletion terminal page home link",
  },
  "auth.loggingOutTitle": {
    message: "Abmeldung läuft...",
    description: "Logout page heading while the logout request is in flight",
  },
  "auth.loggingOutSubtitle": {
    message: "Deine Sitzung wird beendet, einen Moment.",
    description: "Logout page subtext while the logout request is in flight",
  },
  "auth.loggedOutTitle": {
    message: "Abgemeldet",
    description: "Logout page heading once logout has completed",
  },
  "auth.loggedOutSubtitle": {
    message: "Weiterleitung zur Anmeldung…",
    description: "Logout page subtext once logout has completed",
  },
  "auth.logoutFailedTitle": {
    message: "Abmeldung fehlgeschlagen",
    description: "Logout page heading when the provider logout request failed",
  },
  "auth.logoutFailedSubtitle": {
    message:
      "Wir konnten deine Sitzung nicht beenden. Möglicherweise bist du noch angemeldet — bitte versuche es erneut.",
    description: "Logout page subtext when the provider logout request failed",
  },
  "auth.logoutRetry": {
    message: "Erneut versuchen",
    description: "Logout page retry button after a failed sign-out",
  },
  "auth.logoutConfirmTitle": {
    message: "Abmelden?",
    description:
      "Logout page heading on the confirmation screen (codex #12 CSRF logout fix)",
  },
  "auth.logoutConfirmSubtitle": {
    message: "Möchtest du deine Sitzung wirklich beenden?",
    description: "Logout page subtext on the confirmation screen",
  },
  "auth.logoutConfirm": {
    message: "Abmelden",
    description: "Logout page confirm button that triggers the actual logout",
  },
  "auth.logoutCancel": {
    message: "Abbrechen",
    description: "Logout page cancel button that returns to the dashboard",
  },
  "auth.logoutTitle": {
    message: "Abmelden",
    description: "Logout page document title",
  },
  "auth.featureSecureTitle": {
    message: "Standardmäßig sicher",
    description: "Auth hero feature bullet title",
  },
  "auth.featureSecureDescription": {
    message:
      "Sitzungen werden von Ory Kratos verwaltet — erprobte Identitätsinfrastruktur, kein selbstgebautes Auth-System.",
    description: "Auth hero feature bullet description",
  },
  "auth.featureDashboardTitle": {
    message: "Ein Dashboard für jeden Service",
    description: "Auth hero feature bullet title",
  },
  "auth.featureDashboardDescription": {
    message:
      "Deploye, überwache und verwalte alles, was auf bex läuft, an einem Ort.",
    description: "Auth hero feature bullet description",
  },
  "auth.featureOpenSourceTitle": {
    message: "Offen entwickelt",
    description: "Auth hero feature bullet title",
  },
  "auth.featureOpenSourceDescription": {
    message: "bex ist die quelloffene Render-Alternative.",
    description: "Auth hero feature bullet description",
  },
  "auth.consentTitle": {
    message: "Zugriff autorisieren",
    description: "OAuth2 consent page hero title",
  },
  "auth.consentSubtitle": {
    message: "{client} möchte in bex in deinem Namen handeln.",
    description: "OAuth2 consent page hero subtitle, names the OAuth client",
  },
  "auth.consentUnverifiedClient": {
    message: "Nicht verifizierte Drittanbieter-App",
    description:
      "Warning label for an OAuth client whose self-provided branding is not trusted",
  },
  "auth.consentClientId": {
    message: "Client-ID",
    description: "Label for the immutable OAuth client identifier",
  },
  "auth.consentRedirectOrigin": {
    message: "Ziel des Autorisierungscodes",
    description: "Label for the origin that will receive the OAuth code",
  },
  "auth.consentNoRedirectOrigin": {
    message: "Keine Weiterleitung (Device Flow)",
    description: "Shown when an OAuth device flow has no redirect origin",
  },
  "auth.consentScopesTitle": {
    message: "Angefordert wird",
    description: "Heading above the list of requested OAuth scopes",
  },
  "auth.consentScopeOpenid": {
    message: "Bestätigen, wer du bist",
    description: "Human description of the openid scope",
  },
  "auth.consentScopeOfflineAccess": {
    message: "Verbunden bleiben, ohne dich erneut zu fragen (Refresh-Token)",
    description: "Human description of the offline_access scope",
  },
  "auth.consentScopeProfile": {
    message: "Dein Basisprofil lesen",
    description: "Human description of the profile scope",
  },
  "auth.consentScopeEmail": {
    message: "Deine E-Mail-Adresse lesen",
    description: "Human description of the email scope",
  },
  "auth.consentScopeRead": {
    message:
      "Gewöhnliche Workspace-Ressourcen lesen (Services, Logs, Metriken)",
    description: "Human description of the bex.read OAuth capability",
  },
  "auth.consentScopeWrite": {
    message:
      "Workspace-Ressourcen ändern (deployen, neu starten, erstellen, löschen, Abrechnung)",
    description: "Human description of the bex.write OAuth capability",
  },
  "auth.consentScopeSensitive": {
    message:
      "Secrets und Verbindungs-Strings lesen (Umgebungsvariablen, Datenbank-URLs, Dateien)",
    description: "Human description of the bex.sensitive OAuth capability",
  },
  "auth.consentScopeCloudAgents": {
    message:
      "Cloud-Coding-Agents in deinem Namen ausführen (startet Aufgaben, die Rechenleistung und Modellguthaben verbrauchen und einen Branch pushen können)",
    description: "Human description of the bex.cloud_agents OAuth capability",
  },
  "auth.consentScopeApiCompat": {
    message:
      "Voller Zugriff auf die Control Plane (Kompatibilität mit älteren Plattform-Clients)",
    description: "Human description of the bex.api compatibility scope",
  },
  "auth.consentAudienceTitle": {
    message: "Wo das Token funktioniert",
    description: "Heading above the requested access-token audience",
  },
  "auth.consentRememberHint": {
    message:
      "Die Freigabe wird eine Stunde lang gespeichert — danach fragt dieser Agent erneut.",
    description: "Consent page footnote about the remember window",
  },
  "auth.consentApprove": {
    message: "Genehmigen",
    description: "OAuth2 consent page approve button",
  },
  "auth.consentDeny": {
    message: "Ablehnen",
    description: "OAuth2 consent page deny button",
  },
  "auth.consentFailed": {
    message: "Beim Abschließen ist etwas schiefgelaufen. Versuche es erneut.",
    description: "Consent page error banner after a failed accept/reject",
  },
  "auth.consentExpiredTitle": {
    message: "Autorisierung abgelaufen",
    description: "Consent page heading when there is no live consent request",
  },
  "auth.consentExpiredSubtitle": {
    message:
      "Starte die Verbindung erneut in der App, die dich hierher geschickt hat.",
    description:
      "Consent page subtext when there is no live consent request to decide",
  },
  "auth.consentInvalidRequestTitle": {
    message: "Ungültige Autorisierungsanfrage",
    description:
      "Consent page heading when the requesting client's authorization request fails a protocol check (PKCE or scope)",
  },
  "auth.consentPkceRequiredSubtitle": {
    message:
      "Der Anfrage dieser App fehlen erforderliche Sicherheitsparameter. Wende dich an den Entwickler der App.",
    description:
      "Consent page subtext when the authorization request lacks PKCE with S256",
  },
  "auth.consentScopeRequiredSubtitle": {
    message:
      "Diese App hat API-Zugriff angefordert, ohne anzugeben, was sie braucht. Sie muss mindestens einen dieser Scopes anfordern: {scopes}.",
    description:
      "Consent page subtext when an audience request lacks a granular capability scope (round-14 #1); {scopes} is the comma-separated list of accepted capability scopes",
  },
  "auth.consentUnavailableTitle": {
    message: "Autorisierung nicht verfügbar",
    description:
      "Consent page heading when the authorization provider is unreachable, misconfigured, or a headless accept failed",
  },
  "auth.consentUnavailableSubtitle": {
    message: "Das sollte sich bald beheben — versuche es gleich noch einmal.",
    description:
      "Consent page subtext when the authorization provider is unreachable, misconfigured, or a headless accept failed",
  },
  "auth.consentWrongUserTitle": {
    message: "Mit dem falschen Konto angemeldet",
    description:
      "Consent page heading when the browser's session belongs to a different account than the one that started this authorization",
  },
  "auth.consentWrongUserSubtitle": {
    message:
      "Die Sitzung dieses Browsers gehört zu einem anderen Konto als dem, das diese Autorisierung gestartet hat. Melde dich ab und versuche es erneut.",
    description:
      "Consent page subtext when the browser's session belongs to a different account than the one that started this authorization",
  },
  "auth.deviceSuccessTitle": {
    message: "Gerät verbunden",
    description: "Device authorization success page title",
  },
  "auth.deviceTitle": {
    message: "Gerät verbinden",
    description: "Device verification route document title",
  },
  "auth.deviceConfirmSubtitle": {
    message:
      "Ein Gerät, an dem du dich anmeldest, möchte auf dein Konto zugreifen.",
    description: "Device confirm page hero subtitle",
  },
  "auth.deviceConfirmHeading": {
    message: "Dieses Gerät autorisieren",
    description: "Device confirm page card heading",
  },
  "auth.deviceConfirmDescription": {
    message:
      "Bestätige, dass der Code unten mit dem auf deinem Gerät angezeigten übereinstimmt:",
    description: "Device confirm page card description above the code",
  },
  "auth.deviceConfirmHint": {
    message:
      "Autorisiere dies nur, wenn du diese Anmeldung selbst gestartet hast.",
    description:
      "Device confirm page footnote warning against unsolicited requests",
  },
  "auth.deviceConfirmCancel": {
    message: "Abbrechen",
    description: "Device confirm page cancel button",
  },
  "auth.deviceConfirmAuthorize": {
    message: "Gerät autorisieren",
    description: "Device confirm page authorize/submit button",
  },
  "auth.deviceExpiredTitle": {
    message: "Gerätecode abgelaufen",
    description:
      "Device confirm page heading when there is no live device request",
  },
  "auth.deviceExpiredSubtitle": {
    message:
      "Diese Seite braucht einen Gerätecode, oder der Code ist abgelaufen. Starte die Anmeldung erneut auf deinem Gerät.",
    description:
      "Device confirm page subtext when there is no live device request to confirm",
  },
  "auth.deviceUnavailableTitle": {
    message: "Geräteautorisierung nicht verfügbar",
    description:
      "Device confirm page heading when the authorization provider is unreachable or misconfigured",
  },
  "auth.deviceUnavailableSubtitle": {
    message: "Das sollte sich bald beheben — versuche es gleich noch einmal.",
    description:
      "Device confirm page subtext when the authorization provider is unreachable or misconfigured",
  },
  "auth.deviceRefusedTitle": {
    message: "Autorisierung abgelehnt",
    description:
      "Device confirm page heading when the device request names a client bex did not expect",
  },
  "auth.deviceRefusedSubtitle": {
    message:
      "Diese Geräteanfrage konnte nicht verifiziert werden. Starte die Anmeldung erneut auf deinem Gerät.",
    description:
      "Device confirm page subtext when the device request names a client bex did not expect",
  },
  "auth.deviceSuccessSubtitle": {
    message: "Deine Browser-Autorisierung ist abgeschlossen.",
    description: "Device authorization success page subtitle",
  },
  "auth.deviceSuccessHint": {
    message:
      "Kehre zur App zurück, bei der du dich anmeldest, um fortzufahren.",
    description: "Device authorization success page hint",
  },
  "auth.deviceSuccessWaiting": {
    message: "Warte auf Autorisierung im Browser…",
    description:
      "Device success page terminal replica: the CLI line shown while it polls for tokens",
  },
  "auth.deviceSuccessDone": {
    message: "Autorisierung abgeschlossen. Du bist angemeldet.",
    description:
      "Device success page terminal replica: the CLI success line (rendered after a ✓ glyph)",
  },
  "auth.deviceSuccessClose": {
    message: "Du kannst diesen Tab schließen.",
    description:
      "Device success page: follow-up sentence after the return-to-terminal hint",
  },
  "auth.deviceSuccessDashboard": {
    message: "Dashboard öffnen",
    description:
      "Device success page: quiet link to the dashboard home for users staying in the browser",
  },
  "auth.profileEmailLabel": {
    message: "E-Mail",
    description:
      "Account settings profile form: the email field label (the identity schema title stays as Kratos uses it)",
  },
  "auth.profileEmailPlaceholder": {
    message: "E-Mail-Adresse eingeben",
    description: "Account settings profile form: the email field placeholder",
  },
  "auth.profileNameLabel": {
    message: "Name",
    description:
      "Account settings profile form: the name field label (the identity schema title stays as Kratos uses it)",
  },
  "auth.profileNamePlaceholder": {
    message: "Namen eingeben",
    description: "Account settings profile form: the name field placeholder",
  },
  "auth.inputPlaceholder": {
    message: "{placeholder} eingeben",
    description:
      "Auth forms: the placeholder of a field with no wording of its own, e.g. the password; {placeholder} is the field's label",
  },
  "auth.showPassword": {
    message: "Passwort anzeigen",
    description:
      "Accessible name of the password field toggle while the password is hidden",
  },
  "auth.hidePassword": {
    message: "Passwort ausblenden",
    description:
      "Accessible name of the password field toggle while the password is shown",
  },
  "auth.settingsGuardEmailChangeRefused": {
    message:
      "Du kannst deine E-Mail-Adresse nicht ändern, solange du zu einem Workspace gehörst, der die Google-Anmeldung erfordert. Verlasse zuerst diese Workspaces: {workspaces}.",
    description:
      "Account settings refusal of an email change (Kratos message 4100001) while the member belongs to a workspace that requires Google login; {workspaces} is the list of those workspaces (w7/m465)",
  },
  "auth.settingsGuardUnlinkRefused": {
    message:
      "Du kannst keinen Anmeldeanbieter trennen, solange du zu einem Workspace gehörst, der die Google-Anmeldung erfordert. Verlasse zuerst diese Workspaces: {workspaces}.",
    description:
      "Account settings refusal of disconnecting a sign-in provider (Kratos message 4100002) while the member belongs to a workspace that requires Google login; {workspaces} is the list of those workspaces (w7/m465)",
  },
};

export default deAuth;
