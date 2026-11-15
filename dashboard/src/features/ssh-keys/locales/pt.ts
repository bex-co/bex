import type { TranslationEntry } from "@/i18n";

const ptSSHKeys: Record<string, TranslationEntry> = {
  "sshKeys.title": {
    message: "Chaves públicas SSH",
    description: "SSH keys settings card title",
  },
  "sshKeys.description": {
    message:
      "Chaves públicas registradas na sua identidade. As chaves privadas nunca saem do seu dispositivo.",
    description: "SSH keys settings card description",
  },
  "sshKeys.add": {
    message: "Adicionar chave SSH",
    description: "Open add SSH key dialog",
  },
  "sshKeys.addTitle": {
    message: "Adicionar chave pública SSH",
    description: "Add SSH key dialog title",
  },
  "sshKeys.addDescription": {
    message:
      "Cole uma chave pública OpenSSH. Os comentários são removidos ao salvar.",
    description: "Add SSH key dialog description",
  },
  "sshKeys.name": {
    message: "Nome",
    description: "SSH key name field and table column",
  },
  "sshKeys.publicKey": {
    message: "Chave pública",
    description: "SSH public key field label",
  },
  "sshKeys.invalid": {
    message:
      "Informe uma chave pública OpenSSH compatível (RSA deve ter pelo menos 2048 bits).",
    description: "Invalid SSH public key hint",
  },
  "sshKeys.cancel": {
    message: "Cancelar",
    description: "Cancel SSH key action",
  },
  "sshKeys.save": {
    message: "Adicionar chave",
    description: "Submit SSH key button",
  },
  "sshKeys.fingerprint": {
    message: "Fingerprint",
    description: "SSH key fingerprint table column",
  },
  "sshKeys.created": {
    message: "Criada em",
    description: "SSH key creation table column",
  },
  "sshKeys.actions": {
    message: "Ações",
    description: "SSH key actions table column",
  },
  "sshKeys.emptyTitle": {
    message: "Nenhuma chave SSH",
    description: "SSH key empty state title",
  },
  "sshKeys.emptyBody": {
    message:
      "Adicione uma chave pública antes de se conectar a um serviço em execução.",
    description: "SSH key empty state body",
  },
  "sshKeys.errorTitle": {
    message: "Não foi possível carregar as chaves SSH",
    description: "SSH key load error title",
  },
  "sshKeys.errorBody": {
    message:
      "Tente novamente ou peça a um administrador do workspace se o acesso for negado.",
    description: "SSH key load error body",
  },
  "sshKeys.createSuccess": {
    message: "{name} adicionada",
    description: "SSH key creation success toast",
  },
  "sshKeys.createError": {
    message:
      "Não foi possível adicionar esta chave SSH. Verifique o formato e use RSA de 2048 bits ou mais forte.",
    description: "SSH key creation failure toast",
  },
  "sshKeys.duplicateError": {
    message: "Esta chave pública já está registrada",
    description: "Duplicate SSH key toast",
  },
  "sshKeys.delete": {
    message: "Excluir",
    description: "Delete SSH key action",
  },
  "sshKeys.deleteNamed": {
    message: "Excluir chave SSH {name}",
    description: "Row delete button aria-label identifying the SSH key",
  },
  "sshKeys.deleteTitle": {
    message: "Excluir {name}?",
    description: "Delete SSH key confirmation title",
  },
  "sshKeys.deleteBody": {
    message:
      "Novas conexões SSH que usarem esta chave serão rejeitadas imediatamente.",
    description: "Delete SSH key confirmation body",
  },
  "sshKeys.deleteSuccess": {
    message: "{name} excluída",
    description: "Delete SSH key success toast",
  },
  "sshKeys.deleteError": {
    message: "Não foi possível excluir esta chave SSH",
    description: "Delete SSH key failure toast",
  },
  "sshKeys.gateTitle": {
    message: "Adicione uma chave SSH para se conectar",
    description: "RequiresSshKey gate — CTA title when no key is registered",
  },
  "sshKeys.gateBody": {
    message:
      "A conexão é feita por SSH com a sua própria chave. Adicione sua chave pública uma vez e depois volte para se conectar.",
    description: "RequiresSshKey gate — CTA body",
  },
  "sshKeys.gateAddKey": {
    message: "Adicionar chave SSH",
    description: "RequiresSshKey gate — primary action linking to settings",
  },
};

export default ptSSHKeys;
