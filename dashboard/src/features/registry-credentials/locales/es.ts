import type { TranslationEntry } from "@/i18n";

const esRegistryCredentials: Record<string, TranslationEntry> = {
  "registryCredentials.title": {
    message: "Credenciales de registro",
    description:
      "Settings Integrations → Registry Credentials section card title",
  },
  "registryCredentials.description": {
    message:
      "Credenciales para registros de imágenes externos privados (Docker Hub, GHCR, GitLab Container Registry, ECR, etc.), para que un servicio con imagen existente pueda descargar de ellos.",
    description: "Settings Registry Credentials section card description",
  },
  "registryCredentials.colName": {
    message: "Nombre",
    description: "Registry Credentials table column header",
  },
  "registryCredentials.colUsername": {
    message: "Nombre de usuario",
    description: "Registry Credentials table column header",
  },
  "registryCredentials.colStatus": {
    message: "Caducidad",
    description: "Registry Credentials table column header",
  },
  "registryCredentials.colCreated": {
    message: "Creada",
    description: "Registry Credentials table column header",
  },
  "registryCredentials.emptyTitle": {
    message: "Sin credenciales de registro",
    description: "Registry Credentials empty-state title",
  },
  "registryCredentials.emptyBody": {
    message:
      "Añade una credencial para que un servicio pueda descargar una imagen de un registro privado.",
    description: "Registry Credentials empty-state body",
  },
  "registryCredentials.forbiddenTitle": {
    message: "No autorizado",
    description:
      "Registry Credentials state when the caller lacks permission (403)",
  },
  "registryCredentials.forbiddenBody": {
    message:
      "No tienes permiso para gestionar las credenciales de registro de este espacio de trabajo.",
    description: "Registry Credentials forbidden-state body",
  },
  "registryCredentials.errorTitle": {
    message: "No se pudieron cargar las credenciales de registro",
    description: "Registry Credentials generic error title",
  },
  "registryCredentials.errorBody": {
    message: "Algo salió mal. Inténtalo de nuevo.",
    description: "Registry Credentials generic error body",
  },
  "registryCredentials.create": {
    message: "Añadir credencial",
    description: "Button that opens the create dialog",
  },
  "registryCredentials.createTitle": {
    message: "Añadir credencial de registro",
    description: "Create dialog title",
  },
  "registryCredentials.createDescription": {
    message:
      "Se almacena de forma segura. La contraseña o el token no se volverá a mostrar después de esto.",
    description: "Create dialog description",
  },
  "registryCredentials.fieldHost": {
    message: "Host del registro",
    description: "Create dialog host field label",
  },
  "registryCredentials.fieldHostPlaceholder": {
    message: "p. ej. ghcr.io, docker.io, registry.gitlab.com",
    description: "Create dialog host field placeholder",
  },
  "registryCredentials.fieldUsername": {
    message: "Nombre de usuario",
    description: "Create dialog username field label",
  },
  "registryCredentials.fieldAuthToken": {
    message: "Contraseña o token de acceso",
    description: "Create dialog secret field label",
  },
  "registryCredentials.hostInvalid": {
    message:
      "Introduce un host de registro como ghcr.io o registry.example.com:5000, sin ruta.",
    description:
      "Inline error under the Registry host when it is not a bare host[:port] (w21/053)",
  },
  "registryCredentials.hostTooLong": {
    message: "Máximo 253 bytes",
    description: "Inline error under an over-long Registry host (w21/053)",
  },
  "registryCredentials.usernameControl": {
    message: "No debe contener caracteres de control",
    description:
      "Inline error under a registry username with a control character (w21/053)",
  },
  "registryCredentials.usernameTooLong": {
    message: "Máximo 253 bytes",
    description: "Inline error under an over-long registry username (w21/053)",
  },
  "registryCredentials.authTokenTooLong": {
    message: "Máximo 64 KiB",
    description:
      "Inline error under a registry password or token over the 64 KiB limit (w21/053)",
  },
  "registryCredentials.fieldName": {
    message: "Nombre para mostrar",
    description: "Create dialog optional display-name field label",
  },
  "registryCredentials.fieldNameOptional": {
    message: "(opcional; por defecto, el host)",
    description: "Create dialog display-name field's optional hint",
  },
  "registryCredentials.createCancel": {
    message: "Cancelar",
    description: "Create dialog cancel button",
  },
  "registryCredentials.createSubmit": {
    message: "Añadir",
    description: "Create dialog submit button",
  },
  "registryCredentials.createSuccess": {
    message: "Se añadió una credencial para {host}",
    description: "Toast on a successful create",
  },
  "registryCredentials.createError": {
    message: "No se pudo añadir la credencial para {host}",
    description: "Toast on a failed create",
  },
  "registryCredentials.delete": {
    message: "Eliminar",
    description: "Row action / confirmation button to delete a credential",
  },
  "registryCredentials.deleteNamed": {
    message: "Eliminar la credencial de registro {name}",
    description:
      "Row delete button aria-label identifying the registry credential",
  },
  "registryCredentials.edit": {
    message: "Editar",
    description: "Row action to open the edit-credential dialog",
  },
  "registryCredentials.editTitle": {
    message: "Editar credencial de registro",
    description: "Edit-credential dialog title",
  },
  "registryCredentials.editDescription": {
    message:
      "Renombra la credencial, cambia su nombre de usuario o rota el token. El host del registro no se puede cambiar.",
    description: "Edit-credential dialog description",
  },
  "registryCredentials.editSubmit": {
    message: "Guardar cambios",
    description: "Edit-credential dialog submit button",
  },
  "registryCredentials.fieldHostImmutable": {
    message: "El host del registro no se puede cambiar después de la creación.",
    description: "Hint under the read-only host field in the edit dialog",
  },
  "registryCredentials.fieldAuthTokenKeep": {
    message: "Déjalo en blanco para conservar el token actual",
    description: "Placeholder for the token field in the edit dialog",
  },
  "registryCredentials.fieldAuthTokenKeepHint": {
    message:
      "El token almacenado nunca se muestra. Introduce un valor nuevo solo para rotarlo.",
    description: "Hint under the token field in the edit dialog",
  },
  "registryCredentials.updateSuccess": {
    message: "Credencial de registro actualizada.",
    description: "Toast after an edit succeeds",
  },
  "registryCredentials.updateError": {
    message: "No se pudo actualizar la credencial de registro.",
    description: "Toast after an edit fails",
  },
  "registryCredentials.deleteConfirmTitle": {
    message: "¿Eliminar {name}?",
    description: "Delete-confirmation dialog title",
  },
  "registryCredentials.deleteConfirmBody": {
    message:
      "Solo se puede eliminar una credencial que ningún servicio use: quítala primero de todos los servicios, o se rechazará la eliminación. Esta acción no se puede deshacer.",
    description: "Delete-confirmation dialog body",
  },
  "registryCredentials.deleteCancel": {
    message: "Cancelar",
    description: "Delete-confirmation dialog cancel button",
  },
  "registryCredentials.deleteSuccess": {
    message: "{name} eliminada",
    description: "Toast on a successful delete",
  },
  "registryCredentials.deleteError": {
    message: "No se pudo eliminar {name}",
    description: "Toast on a failed delete",
  },
  "registryCredentials.expired": {
    message: "Caducada",
    description:
      "Expiry-status badge for a past-expiry credential (w2/m14/t007)",
  },
  "registryCredentials.expiringSoon": {
    message: "Caduca pronto",
    description: "Expiry-status badge for a credential nearing expiry",
  },
  "registryCredentials.expiresOn": {
    message: "Caduca el {date}",
    description:
      "Expiry-status text for an active credential with a future expiry",
  },
  "registryCredentials.neverExpires": {
    message: "No caduca nunca",
    description: "Expiry-status text for a credential with no expiry set",
  },
};

export default esRegistryCredentials;
