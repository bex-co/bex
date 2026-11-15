import type { TranslationEntry } from "@/i18n";

const deAudit: Record<string, TranslationEntry> = {
  "audit.title": {
    message: "Audit-Log",
    description: "Settings Audit Log section card title",
  },
  "audit.description": {
    message:
      "Wer in diesem Workspace was getan hat, erlaubt oder abgelehnt — neueste zuerst. Nur für Workspace-Admins sichtbar.",
    description: "Settings Audit Log section card description",
  },
  "audit.columnTimestamp": {
    message: "Zeitstempel",
    description: "Audit Log table column header",
  },
  "audit.columnActor": {
    message: "Akteur",
    description: "Audit Log table column header — who performed the action",
  },
  "audit.columnAction": {
    message: "Aktion",
    description: "Audit Log table column header — the verb performed",
  },
  "audit.columnStatus": {
    message: "Status",
    description: "Audit Log table column header — allowed or denied",
  },
  "audit.columnResource": {
    message: "Ressource",
    description: "Audit Log table column header — the object acted on",
  },
  "audit.changeRename": {
    message: "Name {from} → {to}",
    description:
      "Audit log Resource cell: a service rename's label before and after (w5/183)",
  },
  "audit.changeDomain": {
    message: "Domain {domain}",
    description:
      "Audit log Resource cell: the custom domain a domain verb added, removed or verified (w5/183)",
  },
  "audit.changeEnvGroup": {
    message: "Umgebungsgruppe {name}",
    description:
      "Audit log Resource cell: the environment group a link or unlink changed (w5/183)",
  },
  "audit.statusAllowed": {
    message: "Erlaubt",
    description: "Audit Log status badge for a successful action",
  },
  "audit.statusDenied": {
    message: "Abgelehnt",
    description: "Audit Log status badge for a denied action",
  },
  "audit.actorUnknown": {
    message: "Unbekannt",
    description:
      "Audit Log actor cell placeholder for an unauthenticated caller",
  },
  "audit.oauthDelegation": {
    message: "{client} · {scopes}",
    description:
      "Audit Log actor subtitle for a third-party OAuth grant (client id and canonical scopes)",
  },
  "audit.loadMore": {
    message: "Mehr laden",
    description: "Button that fetches the next page of audit events",
  },
  "audit.emptyTitle": {
    message: "Noch keine Audit-Ereignisse",
    description: "Audit Log empty-state title",
  },
  "audit.emptyBody": {
    message: "Schreibende Aktionen in diesem Workspace erscheinen hier.",
    description: "Audit Log empty-state body",
  },
  "audit.errorTitle": {
    message: "Das Audit-Log konnte nicht geladen werden",
    description: "Audit Log generic error title",
  },
  "audit.errorBody": {
    message:
      "Beim Laden des Audit-Trails dieses Workspace ist etwas schiefgelaufen. Versuche es erneut.",
    description: "Audit Log generic error body",
  },
  "audit.unavailableTitle": {
    message: "Audit-Log nicht konfiguriert",
    description:
      "Audit Log state when the control-plane store isn't wired (503)",
  },
  "audit.unavailableBody": {
    message:
      "Für dieses bex-Deployment ist kein Audit-Log-Speicher konfiguriert.",
    description: "Audit Log unavailable-state body",
  },
  "audit.planTitle": {
    message: "Audit-Logs erfordern den Pro-Tarif",
    description:
      "Audit Log title on a Hobby workspace (PLAN_FEATURE_REQUIRED, w8/m138)",
  },
  "audit.planBody": {
    message:
      "Audit-Logs sind im Pro-Tarif und höher verfügbar. Dieser Workspace nutzt Hobby.",
    description: "Audit Log plan notice body on a Hobby workspace (w8/m138)",
  },
  "audit.exportStart": {
    message: "Startdatum (UTC)",
    description: "Audit Log CSV export start-date label",
  },
  "audit.exportEnd": {
    message: "Enddatum (UTC)",
    description: "Audit Log CSV export end-date label",
  },
  "audit.exportCSV": {
    message: "Als CSV exportieren",
    description: "Audit Log CSV export button (Render's label)",
  },
  "audit.exportHint": {
    message:
      "Exportiert Ereignisse von bis zu {days} Tagen: Zeitstempel, Akteur, Ereignis, Status und Metadaten.",
    description: "Audit Log CSV export help text; {days} is the longest range",
  },
  "audit.exportRangeOrder": {
    message: "Das Enddatum darf nicht vor dem Startdatum liegen.",
    description: "Audit Log CSV export: end before start",
  },
  "audit.exportRangeTooLong": {
    message: "Ein Export kann höchstens {days} Tage umfassen.",
    description:
      "Audit Log CSV export: range over the limit; {days} is the limit",
  },
  "audit.exportTooLarge": {
    message:
      "Dieser Zeitraum enthält zu viele Ereignisse für einen einzelnen Export. Wähle einen kürzeren Zeitraum.",
    description: "Audit Log CSV export: row budget exceeded",
  },
  "audit.exportFailed": {
    message: "Der Export ist fehlgeschlagen. Versuche es erneut.",
    description: "Audit Log CSV export: request failed",
  },
  "audit.exportRangeMissing": {
    message: "Wähle ein Start- und ein Enddatum.",
    description: "Audit Log CSV export: a date is empty",
  },
};

export default deAudit;
