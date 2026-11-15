import type { TranslationEntry } from "@/i18n";

const esSSHKeys: Record<string, TranslationEntry> = {
  "sshKeys.title": {
    message: "Claves públicas SSH",
    description: "SSH keys settings card title",
  },
  "sshKeys.description": {
    message:
      "Claves públicas registradas en tu identidad. Las claves privadas nunca salen de tu dispositivo.",
    description: "SSH keys settings card description",
  },
  "sshKeys.add": {
    message: "Añadir clave SSH",
    description: "Open add SSH key dialog",
  },
  "sshKeys.addTitle": {
    message: "Añadir clave pública SSH",
    description: "Add SSH key dialog title",
  },
  "sshKeys.addDescription": {
    message:
      "Pega una clave pública OpenSSH. Los comentarios se eliminan al guardarla.",
    description: "Add SSH key dialog description",
  },
  "sshKeys.name": {
    message: "Nombre",
    description: "SSH key name field and table column",
  },
  "sshKeys.publicKey": {
    message: "Clave pública",
    description: "SSH public key field label",
  },
  "sshKeys.invalid": {
    message:
      "Introduce una clave pública OpenSSH compatible (RSA debe tener al menos 2048 bits).",
    description: "Invalid SSH public key hint",
  },
  "sshKeys.cancel": {
    message: "Cancelar",
    description: "Cancel SSH key action",
  },
  "sshKeys.save": {
    message: "Añadir clave",
    description: "Submit SSH key button",
  },
  "sshKeys.fingerprint": {
    message: "Huella digital",
    description: "SSH key fingerprint table column",
  },
  "sshKeys.created": {
    message: "Creada",
    description: "SSH key creation table column",
  },
  "sshKeys.actions": {
    message: "Acciones",
    description: "SSH key actions table column",
  },
  "sshKeys.emptyTitle": {
    message: "No hay claves SSH",
    description: "SSH key empty state title",
  },
  "sshKeys.emptyBody": {
    message:
      "Añade una clave pública antes de conectarte a un servicio en ejecución.",
    description: "SSH key empty state body",
  },
  "sshKeys.errorTitle": {
    message: "No se pudieron cargar las claves SSH",
    description: "SSH key load error title",
  },
  "sshKeys.errorBody": {
    message:
      "Inténtalo de nuevo o consulta a un administrador del espacio de trabajo si se deniega el acceso.",
    description: "SSH key load error body",
  },
  "sshKeys.createSuccess": {
    message: "{name} añadida",
    description: "SSH key creation success toast",
  },
  "sshKeys.createError": {
    message:
      "No se pudo añadir esta clave SSH. Comprueba su formato y usa RSA de 2048 bits o más.",
    description: "SSH key creation failure toast",
  },
  "sshKeys.duplicateError": {
    message: "Esta clave pública ya está registrada",
    description: "Duplicate SSH key toast",
  },
  "sshKeys.delete": {
    message: "Eliminar",
    description: "Delete SSH key action",
  },
  "sshKeys.deleteNamed": {
    message: "Eliminar la clave SSH {name}",
    description: "Row delete button aria-label identifying the SSH key",
  },
  "sshKeys.deleteTitle": {
    message: "¿Eliminar {name}?",
    description: "Delete SSH key confirmation title",
  },
  "sshKeys.deleteBody": {
    message:
      "Las nuevas conexiones SSH que usen esta clave se rechazarán de inmediato.",
    description: "Delete SSH key confirmation body",
  },
  "sshKeys.deleteSuccess": {
    message: "{name} eliminada",
    description: "Delete SSH key success toast",
  },
  "sshKeys.deleteError": {
    message: "No se pudo eliminar esta clave SSH",
    description: "Delete SSH key failure toast",
  },
  "sshKeys.gateTitle": {
    message: "Añade una clave SSH para conectarte",
    description: "RequiresSshKey gate — CTA title when no key is registered",
  },
  "sshKeys.gateBody": {
    message:
      "Esto se conecta por SSH con tu propia clave. Añade tu clave pública una vez y vuelve para conectarte.",
    description: "RequiresSshKey gate — CTA body",
  },
  "sshKeys.gateAddKey": {
    message: "Añadir clave SSH",
    description: "RequiresSshKey gate — primary action linking to settings",
  },
};

export default esSSHKeys;
