import type { TranslationEntry } from "@/i18n";

const ruAudit: Record<string, TranslationEntry> = {
  "audit.title": {
    message: "Журнал аудита",
    description: "Settings Audit Log section card title",
  },
  "audit.description": {
    message:
      "Кто что сделал в этом рабочем пространстве, с разрешением или отказом — сначала новые. Виден только администраторам рабочего пространства.",
    description: "Settings Audit Log section card description",
  },
  "audit.columnTimestamp": {
    message: "Метка времени",
    description: "Audit Log table column header",
  },
  "audit.columnActor": {
    message: "Исполнитель",
    description: "Audit Log table column header — who performed the action",
  },
  "audit.columnSource": {
    message: "Источник",
    description:
      "Audit Log table column header — the client address and user agent the actor acted from (w9/m450)",
  },
  "audit.columnAction": {
    message: "Действие",
    description: "Audit Log table column header — the verb performed",
  },
  "audit.columnStatus": {
    message: "Статус",
    description: "Audit Log table column header — allowed or denied",
  },
  "audit.columnResource": {
    message: "Ресурс",
    description: "Audit Log table column header — the object acted on",
  },
  "audit.changeRename": {
    message: "Название {from} → {to}",
    description:
      "Audit log Resource cell: a service rename's label before and after (w5/183)",
  },
  "audit.changeDomain": {
    message: "Домен {domain}",
    description:
      "Audit log Resource cell: the custom domain a domain verb added, removed or verified (w5/183)",
  },
  "audit.changeEnvGroup": {
    message: "Группа окружения {name}",
    description:
      "Audit log Resource cell: the environment group a link or unlink changed (w5/183)",
  },
  "audit.changeEnvVars": {
    message: "Переменные {keys}",
    description:
      "Audit log Resource cell: the environment variable names an env var row created, changed or deleted, never their values (w9/m449)",
  },
  "audit.changeGrantScopes": {
    message: "Права {scopes}",
    description:
      "Audit log Resource cell: the scopes a member granted an OAuth client (GrantOAuthAccessEvent, w4/m454)",
  },
  "audit.changeInstance": {
    message: "Экземпляр {instance}",
    description:
      "Audit log Resource cell: the service instance a shell session (StartShellEvent / EndShellEvent) ran on (w9/m451)",
  },
  "audit.statusAllowed": {
    message: "Разрешено",
    description: "Audit Log status badge for a successful action",
  },
  "audit.statusDenied": {
    message: "Отклонено",
    description: "Audit Log status badge for a denied action",
  },
  "audit.actorUnknown": {
    message: "Неизвестно",
    description:
      "Audit Log actor cell placeholder for an unauthenticated caller",
  },
  "audit.oauthDelegation": {
    message: "{client} · {scopes}",
    description:
      "Audit Log actor subtitle for a third-party OAuth grant (client id and canonical scopes)",
  },
  "audit.loadMore": {
    message: "Показать ещё",
    description: "Button that fetches the next page of audit events",
  },
  "audit.emptyTitle": {
    message: "Событий аудита пока нет",
    description: "Audit Log empty-state title",
  },
  "audit.emptyBody": {
    message: "Действия записи в этом рабочем пространстве появятся здесь.",
    description: "Audit Log empty-state body",
  },
  "audit.errorTitle": {
    message: "Не удалось загрузить журнал аудита",
    description: "Audit Log generic error title",
  },
  "audit.errorBody": {
    message:
      "При загрузке журнала аудита этого рабочего пространства что-то пошло не так. Повторите попытку.",
    description: "Audit Log generic error body",
  },
  "audit.unavailableTitle": {
    message: "Журнал аудита не настроен",
    description:
      "Audit Log state when the control-plane store isn't wired (503)",
  },
  "audit.unavailableBody": {
    message: "В этом развёртывании bex не настроено хранилище журнала аудита.",
    description: "Audit Log unavailable-state body",
  },
  "audit.planTitle": {
    message: "Журнал аудита требует тарифа Pro",
    description:
      "Audit Log title on a Hobby workspace (PLAN_FEATURE_REQUIRED, w8/m138)",
  },
  "audit.planBody": {
    message:
      "Журнал аудита доступен на тарифе Pro и выше. Это рабочее пространство находится на тарифе Hobby.",
    description: "Audit Log plan notice body on a Hobby workspace (w8/m138)",
  },
  "audit.exportStart": {
    message: "Дата начала (UTC)",
    description: "Audit Log CSV export start-date label",
  },
  "audit.exportEnd": {
    message: "Дата окончания (UTC)",
    description: "Audit Log CSV export end-date label",
  },
  "audit.exportCSV": {
    message: "Экспортировать в CSV",
    description: "Audit Log CSV export button (Render's label)",
  },
  "audit.exportHint": {
    message:
      "Экспортирует события за период до {days} дн.: метка времени, исполнитель, событие, статус и метаданные.",
    description: "Audit Log CSV export help text; {days} is the longest range",
  },
  "audit.exportRangeOrder": {
    message: "Дата окончания не должна быть раньше даты начала.",
    description: "Audit Log CSV export: end before start",
  },
  "audit.exportRangeTooLong": {
    message: "Экспорт может охватывать не более {days} дн.",
    description:
      "Audit Log CSV export: range over the limit; {days} is the limit",
  },
  "audit.exportTooLarge": {
    message:
      "В этом диапазоне слишком много событий для разового экспорта. Выберите период короче.",
    description: "Audit Log CSV export: row budget exceeded",
  },
  "audit.exportFailed": {
    message: "Не удалось выполнить экспорт. Повторите попытку.",
    description: "Audit Log CSV export: request failed",
  },
  "audit.exportRangeMissing": {
    message: "Выберите дату начала и дату окончания.",
    description: "Audit Log CSV export: a date is empty",
  },
};

export default ruAudit;
