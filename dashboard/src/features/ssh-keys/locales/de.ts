import type { TranslationEntry } from "@/i18n";

const deSSHKeys: Record<string, TranslationEntry> = {
  "sshKeys.title": {
    message: "Öffentliche SSH-Schlüssel",
    description: "SSH keys settings card title",
  },
  "sshKeys.description": {
    message:
      "Öffentliche Schlüssel, die deiner Identität zugeordnet sind. Private Schlüssel verlassen nie dein Gerät.",
    description: "SSH keys settings card description",
  },
  "sshKeys.add": {
    message: "SSH-Schlüssel hinzufügen",
    description: "Open add SSH key dialog",
  },
  "sshKeys.addTitle": {
    message: "Öffentlichen SSH-Schlüssel hinzufügen",
    description: "Add SSH key dialog title",
  },
  "sshKeys.addDescription": {
    message:
      "Füge einen öffentlichen OpenSSH-Schlüssel ein. Kommentare werden beim Speichern entfernt.",
    description: "Add SSH key dialog description",
  },
  "sshKeys.name": {
    message: "Name",
    description: "SSH key name field and table column",
  },
  "sshKeys.publicKey": {
    message: "Öffentlicher Schlüssel",
    description: "SSH public key field label",
  },
  "sshKeys.invalid": {
    message:
      "Gib einen unterstützten öffentlichen OpenSSH-Schlüssel ein (RSA muss mindestens 2048 Bit haben).",
    description: "Invalid SSH public key hint",
  },
  "sshKeys.cancel": {
    message: "Abbrechen",
    description: "Cancel SSH key action",
  },
  "sshKeys.save": {
    message: "Schlüssel hinzufügen",
    description: "Submit SSH key button",
  },
  "sshKeys.fingerprint": {
    message: "Fingerprint",
    description: "SSH key fingerprint table column",
  },
  "sshKeys.created": {
    message: "Erstellt",
    description: "SSH key creation table column",
  },
  "sshKeys.actions": {
    message: "Aktionen",
    description: "SSH key actions table column",
  },
  "sshKeys.emptyTitle": {
    message: "Keine SSH-Schlüssel",
    description: "SSH key empty state title",
  },
  "sshKeys.emptyBody": {
    message:
      "Füge einen öffentlichen Schlüssel hinzu, bevor du dich mit einem laufenden Service verbindest.",
    description: "SSH key empty state body",
  },
  "sshKeys.errorTitle": {
    message: "SSH-Schlüssel konnten nicht geladen werden",
    description: "SSH key load error title",
  },
  "sshKeys.errorBody": {
    message:
      "Versuche es erneut oder frage einen Workspace-Admin, falls der Zugriff verweigert wird.",
    description: "SSH key load error body",
  },
  "sshKeys.createSuccess": {
    message: "{name} hinzugefügt",
    description: "SSH key creation success toast",
  },
  "sshKeys.createError": {
    message:
      "Dieser SSH-Schlüssel konnte nicht hinzugefügt werden. Prüfe das Format und verwende RSA mit 2048 Bit oder stärker.",
    description: "SSH key creation failure toast",
  },
  "sshKeys.duplicateError": {
    message: "Dieser öffentliche Schlüssel ist bereits registriert",
    description: "Duplicate SSH key toast",
  },
  "sshKeys.delete": {
    message: "Löschen",
    description: "Delete SSH key action",
  },
  "sshKeys.deleteNamed": {
    message: "SSH-Schlüssel {name} löschen",
    description: "Row delete button aria-label identifying the SSH key",
  },
  "sshKeys.deleteTitle": {
    message: "{name} löschen?",
    description: "Delete SSH key confirmation title",
  },
  "sshKeys.deleteBody": {
    message:
      "Neue SSH-Verbindungen mit diesem Schlüssel werden sofort abgelehnt.",
    description: "Delete SSH key confirmation body",
  },
  "sshKeys.deleteSuccess": {
    message: "{name} gelöscht",
    description: "Delete SSH key success toast",
  },
  "sshKeys.deleteError": {
    message: "Dieser SSH-Schlüssel konnte nicht gelöscht werden",
    description: "Delete SSH key failure toast",
  },
  "sshKeys.gateTitle": {
    message: "Füge einen SSH-Schlüssel hinzu, um dich zu verbinden",
    description: "RequiresSshKey gate — CTA title when no key is registered",
  },
  "sshKeys.gateBody": {
    message:
      "Die Verbindung läuft über SSH mit deinem eigenen Schlüssel. Füge deinen öffentlichen Schlüssel einmalig hinzu und komm dann zurück, um dich zu verbinden.",
    description: "RequiresSshKey gate — CTA body",
  },
  "sshKeys.gateAddKey": {
    message: "SSH-Schlüssel hinzufügen",
    description: "RequiresSshKey gate — primary action linking to settings",
  },
};

export default deSSHKeys;
