import type { TranslationEntry } from "@/i18n";

const enSSHKeys: Record<string, TranslationEntry> = {
  "sshKeys.title": {
    message: "SSH Public Keys",
    description: "SSH keys settings card title",
  },
  "sshKeys.description": {
    message:
      "Public keys registered to your identity. Private keys never leave your device.",
    description: "SSH keys settings card description",
  },
  "sshKeys.add": {
    message: "Add SSH key",
    description: "Open add SSH key dialog",
  },
  "sshKeys.addTitle": {
    message: "Add SSH public key",
    description: "Add SSH key dialog title",
  },
  "sshKeys.addDescription": {
    message:
      "Paste one OpenSSH public key. Comments are removed when it is saved.",
    description: "Add SSH key dialog description",
  },
  "sshKeys.name": {
    message: "Name",
    description: "SSH key name field and table column",
  },
  "sshKeys.publicKey": {
    message: "Public key",
    description: "SSH public key field label",
  },
  "sshKeys.invalid": {
    message:
      "Enter one supported OpenSSH public key (RSA must be at least 2048 bits).",
    description: "Invalid SSH public key hint",
  },
  "sshKeys.cancel": { message: "Cancel", description: "Cancel SSH key action" },
  "sshKeys.save": { message: "Add key", description: "Submit SSH key button" },
  "sshKeys.fingerprint": {
    message: "Fingerprint",
    description: "SSH key fingerprint table column",
  },
  "sshKeys.created": {
    message: "Created",
    description: "SSH key creation table column",
  },
  "sshKeys.actions": {
    message: "Actions",
    description: "SSH key actions table column",
  },
  "sshKeys.emptyTitle": {
    message: "No SSH keys",
    description: "SSH key empty state title",
  },
  "sshKeys.emptyBody": {
    message: "Add a public key before connecting to a running service.",
    description: "SSH key empty state body",
  },
  "sshKeys.errorTitle": {
    message: "Couldn't load SSH keys",
    description: "SSH key load error title",
  },
  "sshKeys.errorBody": {
    message: "Try again, or ask a workspace administrator if access is denied.",
    description: "SSH key load error body",
  },
  "sshKeys.createSuccess": {
    message: "Added {name}",
    description: "SSH key creation success toast",
  },
  "sshKeys.createError": {
    message:
      "Couldn't add this SSH key. Check its format and use RSA 2048 bits or stronger.",
    description: "SSH key creation failure toast",
  },
  "sshKeys.duplicateError": {
    message: "This public key is already registered",
    description: "Duplicate SSH key toast",
  },
  "sshKeys.delete": { message: "Delete", description: "Delete SSH key action" },
  "sshKeys.deleteTitle": {
    message: "Delete {name}?",
    description: "Delete SSH key confirmation title",
  },
  "sshKeys.deleteBody": {
    message: "New SSH connections using this key will be rejected immediately.",
    description: "Delete SSH key confirmation body",
  },
  "sshKeys.deleteSuccess": {
    message: "Deleted {name}",
    description: "Delete SSH key success toast",
  },
  "sshKeys.deleteError": {
    message: "Couldn't delete this SSH key",
    description: "Delete SSH key failure toast",
  },
};

export default enSSHKeys;
