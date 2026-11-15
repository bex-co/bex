import type { TranslationEntry } from "@/i18n";

const ptAudit: Record<string, TranslationEntry> = {
  "audit.title": {
    message: "Log de auditoria",
    description: "Settings Audit Log section card title",
  },
  "audit.description": {
    message:
      "Quem fez o quê neste workspace, permitido ou negado — do mais recente para o mais antigo. Visível apenas para administradores do workspace.",
    description: "Settings Audit Log section card description",
  },
  "audit.columnTimestamp": {
    message: "Data e hora",
    description: "Audit Log table column header",
  },
  "audit.columnActor": {
    message: "Autor",
    description: "Audit Log table column header — who performed the action",
  },
  "audit.columnAction": {
    message: "Ação",
    description: "Audit Log table column header — the verb performed",
  },
  "audit.columnStatus": {
    message: "Status",
    description: "Audit Log table column header — allowed or denied",
  },
  "audit.columnResource": {
    message: "Recurso",
    description: "Audit Log table column header — the object acted on",
  },
  "audit.changeRename": {
    message: "Nome {from} → {to}",
    description:
      "Audit log Resource cell: a service rename's label before and after (w5/183)",
  },
  "audit.changeDomain": {
    message: "Domínio {domain}",
    description:
      "Audit log Resource cell: the custom domain a domain verb added, removed or verified (w5/183)",
  },
  "audit.changeEnvGroup": {
    message: "Grupo de ambiente {name}",
    description:
      "Audit log Resource cell: the environment group a link or unlink changed (w5/183)",
  },
  "audit.statusAllowed": {
    message: "Permitida",
    description: "Audit Log status badge for a successful action",
  },
  "audit.statusDenied": {
    message: "Negada",
    description: "Audit Log status badge for a denied action",
  },
  "audit.actorUnknown": {
    message: "Desconhecido",
    description:
      "Audit Log actor cell placeholder for an unauthenticated caller",
  },
  "audit.oauthDelegation": {
    message: "{client} · {scopes}",
    description:
      "Audit Log actor subtitle for a third-party OAuth grant (client id and canonical scopes)",
  },
  "audit.loadMore": {
    message: "Carregar mais",
    description: "Button that fetches the next page of audit events",
  },
  "audit.emptyTitle": {
    message: "Nenhum evento de auditoria ainda",
    description: "Audit Log empty-state title",
  },
  "audit.emptyBody": {
    message: "As ações de gravação neste workspace aparecerão aqui.",
    description: "Audit Log empty-state body",
  },
  "audit.errorTitle": {
    message: "Não foi possível carregar o log de auditoria",
    description: "Audit Log generic error title",
  },
  "audit.errorBody": {
    message:
      "Algo deu errado ao carregar a trilha de auditoria deste workspace. Tente novamente.",
    description: "Audit Log generic error body",
  },
  "audit.unavailableTitle": {
    message: "Log de auditoria não configurado",
    description:
      "Audit Log state when the control-plane store isn't wired (503)",
  },
  "audit.unavailableBody": {
    message:
      "Esta instalação do bex não tem um armazenamento de log de auditoria configurado.",
    description: "Audit Log unavailable-state body",
  },
  "audit.planTitle": {
    message: "Os logs de auditoria exigem o plano Pro",
    description:
      "Audit Log title on a Hobby workspace (PLAN_FEATURE_REQUIRED, w8/m138)",
  },
  "audit.planBody": {
    message:
      "Os logs de auditoria estão disponíveis no plano Pro e superiores. Este workspace está no plano Hobby.",
    description: "Audit Log plan notice body on a Hobby workspace (w8/m138)",
  },
  "audit.exportStart": {
    message: "Data de início (UTC)",
    description: "Audit Log CSV export start-date label",
  },
  "audit.exportEnd": {
    message: "Data de término (UTC)",
    description: "Audit Log CSV export end-date label",
  },
  "audit.exportCSV": {
    message: "Exportar como CSV",
    description: "Audit Log CSV export button (Render's label)",
  },
  "audit.exportHint": {
    message:
      "Exporta até {days} dias de eventos: data e hora, autor, evento, status e metadados.",
    description: "Audit Log CSV export help text; {days} is the longest range",
  },
  "audit.exportRangeOrder": {
    message: "A data de término deve ser igual ou posterior à data de início.",
    description: "Audit Log CSV export: end before start",
  },
  "audit.exportRangeTooLong": {
    message: "Uma exportação pode abranger no máximo {days} dias.",
    description:
      "Audit Log CSV export: range over the limit; {days} is the limit",
  },
  "audit.exportTooLarge": {
    message:
      "Este intervalo tem eventos demais para exportar de uma vez. Escolha um intervalo menor.",
    description: "Audit Log CSV export: row budget exceeded",
  },
  "audit.exportFailed": {
    message: "A exportação falhou. Tente novamente.",
    description: "Audit Log CSV export: request failed",
  },
  "audit.exportRangeMissing": {
    message: "Escolha uma data de início e uma de término.",
    description: "Audit Log CSV export: a date is empty",
  },
};

export default ptAudit;
