import type { OrySettingsCopy } from "./keys";

// Ory Elements' own `de` catalog entries, copied verbatim (see ./keys.ts).
const de: OrySettingsCopy = {
  "orySettings.profileTitle": "Profileinstellungen",
  "orySettings.profileDescription":
    "Aktualisieren Sie Ihre Profilinformationen",
  "orySettings.passwordTitle": "Passwort ändern",
  "orySettings.passwordDescription": "Ändern Sie Ihr Passwort",
  "orySettings.passwordLabel": "Passwort",
  "orySettings.oidcTitle": "Social-Login",
  "orySettings.oidcDescription":
    "Verwalten Sie Ihre Einstellungen für den Social-Login",
  "orySettings.oidcInfo":
    "Verbundene Konten dieser Anbieter können für die Anmeldung bei Ihrem Konto verwendet werden",
  "orySettings.lookupSecretTitle": "Wiederherstellungscodes",
  "orySettings.lookupSecretDescription":
    "Verwalten Sie Ihre 2FA-Backup-Wiederherstellungscodes",
  "orySettings.lookupSecretEnable":
    "Neue Backup-Wiederherstellungscodes generieren",
  "orySettings.webauthnTitle": "Hardware-Token verwalten",
  "orySettings.webauthnDescription":
    "Verwalten Sie Ihre Hardware-Token-Einstellungen",
  "orySettings.webauthnNameLabel": "Name des Sicherheitsschlüssels",
  "orySettings.webauthnAdd": "Sicherheitsschlüssel hinzufügen",
  "orySettings.webauthnInfo":
    "Hardware-Tokens werden für die Zweitfaktor-Authentifizierung oder als Erstfaktor-Authentifizierung mit Passkeys verwendet",
  "orySettings.totpTitle": "Authentifikator-App",
  "orySettings.totpDescription":
    "Fügen Sie eine Authentifikator-App zu Ihrem Konto hinzu, um die Sicherheit Ihres Kontos zu verbessern. Beliebte Authentifikator-Apps sind LastPass und Google Authenticator.",
  "orySettings.totpSecretLabel":
    "Dies ist Ihr Authentifizierungs-App-Geheimnis. Verwenden Sie es, wenn Sie den QR-Code nicht scannen können.",
  "orySettings.totpCodeLabel": "Verifizierungscode",
  "orySettings.totpInfoNotLinked":
    "Um dies zu ermöglichen, scannen Sie den QR-Code mit Ihrer Authentifikator-App und geben Sie den Code ein.",
  "orySettings.save": "Speichern",
};

export default de;
