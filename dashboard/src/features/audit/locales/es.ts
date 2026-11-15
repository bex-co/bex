import type { TranslationEntry } from "@/i18n";

const esAudit: Record<string, TranslationEntry> = {
  "audit.title": {
    message: "Registro de auditoría",
    description: "Settings Audit Log section card title",
  },
  "audit.description": {
    message:
      "Quién hizo qué en este espacio de trabajo, permitido o denegado, de más reciente a más antiguo. Visible solo para los administradores del espacio de trabajo.",
    description: "Settings Audit Log section card description",
  },
  "audit.columnTimestamp": {
    message: "Marca de tiempo",
    description: "Audit Log table column header",
  },
  "audit.columnActor": {
    message: "Actor",
    description: "Audit Log table column header — who performed the action",
  },
  "audit.columnAction": {
    message: "Acción",
    description: "Audit Log table column header — the verb performed",
  },
  "audit.columnStatus": {
    message: "Estado",
    description: "Audit Log table column header — allowed or denied",
  },
  "audit.columnResource": {
    message: "Recurso",
    description: "Audit Log table column header — the object acted on",
  },
  "audit.changeRename": {
    message: "Nombre {from} → {to}",
    description:
      "Audit log Resource cell: a service rename's label before and after (w5/183)",
  },
  "audit.changeDomain": {
    message: "Dominio {domain}",
    description:
      "Audit log Resource cell: the custom domain a domain verb added, removed or verified (w5/183)",
  },
  "audit.changeEnvGroup": {
    message: "Grupo de entorno {name}",
    description:
      "Audit log Resource cell: the environment group a link or unlink changed (w5/183)",
  },
  "audit.statusAllowed": {
    message: "Permitida",
    description: "Audit Log status badge for a successful action",
  },
  "audit.statusDenied": {
    message: "Denegada",
    description: "Audit Log status badge for a denied action",
  },
  "audit.actorUnknown": {
    message: "Desconocido",
    description:
      "Audit Log actor cell placeholder for an unauthenticated caller",
  },
  "audit.oauthDelegation": {
    message: "{client} · {scopes}",
    description:
      "Audit Log actor subtitle for a third-party OAuth grant (client id and canonical scopes)",
  },
  "audit.loadMore": {
    message: "Cargar más",
    description: "Button that fetches the next page of audit events",
  },
  "audit.emptyTitle": {
    message: "Aún no hay eventos de auditoría",
    description: "Audit Log empty-state title",
  },
  "audit.emptyBody": {
    message:
      "Las acciones de escritura en este espacio de trabajo aparecerán aquí.",
    description: "Audit Log empty-state body",
  },
  "audit.errorTitle": {
    message: "No se pudo cargar el registro de auditoría",
    description: "Audit Log generic error title",
  },
  "audit.errorBody": {
    message:
      "Algo salió mal al cargar el registro de auditoría de este espacio de trabajo. Inténtalo de nuevo.",
    description: "Audit Log generic error body",
  },
  "audit.unavailableTitle": {
    message: "Registro de auditoría no configurado",
    description:
      "Audit Log state when the control-plane store isn't wired (503)",
  },
  "audit.unavailableBody": {
    message:
      "Este despliegue de bex no tiene configurado un almacén de registro de auditoría.",
    description: "Audit Log unavailable-state body",
  },
  "audit.planTitle": {
    message: "Los registros de auditoría requieren el plan Pro",
    description:
      "Audit Log title on a Hobby workspace (PLAN_FEATURE_REQUIRED, w8/m138)",
  },
  "audit.planBody": {
    message:
      "Los registros de auditoría están disponibles en el plan Pro y superiores. Este espacio de trabajo está en Hobby.",
    description: "Audit Log plan notice body on a Hobby workspace (w8/m138)",
  },
  "audit.exportStart": {
    message: "Fecha de inicio (UTC)",
    description: "Audit Log CSV export start-date label",
  },
  "audit.exportEnd": {
    message: "Fecha de fin (UTC)",
    description: "Audit Log CSV export end-date label",
  },
  "audit.exportCSV": {
    message: "Exportar como CSV",
    description: "Audit Log CSV export button (Render's label)",
  },
  "audit.exportHint": {
    message:
      "Exporta hasta {days} días de eventos: marca de tiempo, actor, evento, estado y metadatos.",
    description: "Audit Log CSV export help text; {days} is the longest range",
  },
  "audit.exportRangeOrder": {
    message: "La fecha de fin debe ser igual o posterior a la fecha de inicio.",
    description: "Audit Log CSV export: end before start",
  },
  "audit.exportRangeTooLong": {
    message: "Una exportación puede abarcar como máximo {days} días.",
    description:
      "Audit Log CSV export: range over the limit; {days} is the limit",
  },
  "audit.exportTooLarge": {
    message:
      "Este rango tiene demasiados eventos para exportarlos de una vez. Elige un rango más corto.",
    description: "Audit Log CSV export: row budget exceeded",
  },
  "audit.exportFailed": {
    message: "La exportación falló. Inténtalo de nuevo.",
    description: "Audit Log CSV export: request failed",
  },
  "audit.exportRangeMissing": {
    message: "Elige una fecha de inicio y una de fin.",
    description: "Audit Log CSV export: a date is empty",
  },
};

export default esAudit;
