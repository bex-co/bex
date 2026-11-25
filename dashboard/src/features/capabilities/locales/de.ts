import type { TranslationEntry } from "@/i18n";

const deCapabilities: Record<string, TranslationEntry> = {
  "capabilities.reasonCanCreate": {
    message:
      "Deine Rolle darf diese Änderung nicht vornehmen. Frag einen Workspace-Admin.",
    description:
      "Tooltip/hint on a create-gated service, environment, or datastore control for a member without can_create",
  },
  "capabilities.reasonCanOperate": {
    message:
      "Deine Rolle darf diesen Service nur ansehen. Bitte einen Workspace-Admin, Änderungen vorzunehmen.",
    description:
      "Tooltip/hint on an operate-gated lifecycle control (e.g. a service restart) for a viewer without can_operate",
  },
  "capabilities.reasonCanViewSensitive": {
    message:
      "Deine Rolle darf Secret-Werte nicht anzeigen. Frag einen Workspace-Admin.",
    description:
      "Tooltip on a reveal control (connection string, env value) for a member without can_view_sensitive",
  },
  "capabilities.reasonCanManageBilling": {
    message:
      "Deine Rolle darf die Abrechnung nicht verwalten. Frag einen Workspace-Admin oder ein Abrechnungsmitglied.",
    description:
      "Tooltip on a billing control for a member without can_manage_billing",
  },
  "capabilities.reasonCanViewBilling": {
    message:
      "Deine Rolle darf die Abrechnungsdetails nicht sehen. Frag einen Workspace-Admin.",
    description:
      "Why a member without can_view_billing (a Contributor or a Viewer, as on Render) sees no billing page, nav entry or figures",
  },
  "capabilities.reasonCanViewActivity": {
    message:
      "Deine Rolle darf Service-Ereignisse, Deployments und Metriken nicht sehen. Frag einen Workspace-Admin.",
    description:
      "Why a member without can_view_activity (a Billing member, as on Render) sees no service events, deploys or metrics: the reason on those pages and panels, whose nav entries are hidden",
  },
  "capabilities.reasonCanViewLogs": {
    message: "Deine Rolle darf keine Logs sehen. Frag einen Workspace-Admin.",
    description:
      "Why a member without can_view_logs (a Viewer or a Billing member, as on Render) sees no logs: the reason on the Logs pages and panels, whose nav entries are hidden",
  },
  "capabilities.roleDeniedTitle": {
    message: "Für deine Rolle nicht verfügbar",
    description:
      "Title above the role's reason (capabilities.reasonCanViewActivity or capabilities.reasonCanViewLogs) where a page or panel of service events, deploys, metrics or logs would be",
  },
  "capabilities.reasonCanManage": {
    message:
      "Deine Rolle darf Mitglieder nicht verwalten. Frag einen Workspace-Admin.",
    description:
      "Tooltip on a member-management control for a member without can_manage",
  },
  "capabilities.actionDenied": {
    message:
      "Deine Rolle darf diese Aktion nicht ausführen. Frag einen Workspace-Admin.",
    description:
      "Tooltip on a service/deploy action refused by the server action decision",
  },
  "capabilities.actionAdminOnly": {
    message: "Diese Aktion ist Admins vorbehalten.",
    description:
      "Tooltip on an action refused to a non-admin in a protected environment: Suspend or Resume, per resource and in bulk (Render's copy, w4/m578); a datastore's connection-info reveal and IP rules, and an environment group's variables, secret files, links and Delete (w4/m594)",
  },
  "capabilities.actionUnavailable": {
    message:
      "Der Server konnte diese Aktion gerade nicht bestätigen. Versuche es erneut.",
    description:
      "Tooltip when an action projection is unavailable or returned an unknown outcome",
  },
  "capabilities.actionChecking": {
    message: "Es wird geprüft, ob du diese Aktion ausführen darfst…",
    description:
      "Tooltip while a service/deploy action decision is still loading",
  },
  "capabilities.blockedProtectedConfirmation": {
    message:
      "Diese geschützte Ressource braucht eine zusätzliche Bestätigung, bevor diese Aktion ausgeführt werden kann.",
    description: "Blocked reason for protected_confirmation_required",
  },
  "capabilities.blockedSuspended": {
    message:
      "Nicht verfügbar, solange der Service pausiert ist. Setze ihn zuerst fort.",
    description: "Blocked reason for suspended precondition",
  },
  "capabilities.blockedNotSuspended": {
    message: "Diese Ressource läuft bereits, es gibt also nichts fortzusetzen.",
    description: "Blocked reason for not_suspended precondition",
  },
  "capabilities.blockedNoActiveDeploy": {
    message:
      "Es gibt gerade kein aktives Deployment, das abgebrochen werden könnte.",
    description: "Blocked reason for no_active_deploy",
  },
  "capabilities.blockedNoActiveRun": {
    message:
      "Es gibt gerade keinen aktiven Lauf, der abgebrochen werden könnte.",
    description: "Blocked reason for no_active_run",
  },
  "capabilities.blockedNoEligibleRollbackTarget": {
    message: "Kein früheres Deployment kommt als Rollback-Ziel infrage.",
    description: "Blocked reason for no_eligible_rollback_target",
  },
  "capabilities.blockedBillingBlocked": {
    message:
      "Die Abrechnung braucht Aufmerksamkeit, bevor diese Aktion ausgeführt werden kann. Kläre das unter Abrechnung.",
    description: "Blocked reason for billing_blocked",
  },
  "capabilities.blockedExpired": {
    message:
      "Der kostenlose Plan ist abgelaufen, daher bleibt diese Ressource pausiert. Wechsle zu einem kostenpflichtigen Plan, um sie fortzusetzen.",
    description: "Blocked reason for expired precondition",
  },
  "capabilities.blockedFreeHoursExhausted": {
    message:
      "Dieser Workspace hat seine monatlichen kostenlosen Instanzstunden aufgebraucht, daher bleibt dieser Service pausiert. Kostenlose Services werden nächsten Monat fortgesetzt, oder wechsle zu einem kostenpflichtigen Plan, um ihn jetzt auszuführen.",
    description: "Blocked reason for free_hours_exhausted precondition",
  },
  "capabilities.blockedBandwidthExhausted": {
    message:
      "Dieser Workspace hat seine monatlich enthaltene Bandbreite aufgebraucht, daher bleibt dieser Service pausiert. Services werden nächsten Monat fortgesetzt, oder hinterlege eine Zahlungsmethode, um sie jetzt auszuführen.",
    description: "Blocked reason for bandwidth_exhausted precondition",
  },
  "capabilities.blockedUnavailable": {
    message:
      "Der Server konnte diese Aktion gerade nicht bestätigen. Versuche es erneut.",
    description: "Blocked reason for unavailable precondition",
  },
  "capabilities.blockedGeneric": {
    message:
      "Der Server erlaubt diese Aktion gerade nicht. Versuche es erneut.",
    description: "Fallback blocked reason",
  },
  "capabilities.grantUnavailable": {
    message:
      "Die Berechtigungen konnten nicht aktualisiert werden. Versuche es erneut — das ist keine Rollenänderung.",
    description:
      "Recovery copy when a capability grant check fails or is stale (m144)",
  },
  "capabilities.grantStale": {
    message:
      "Die Berechtigungen müssen aktualisiert werden, bevor diese Aktion ausgeführt werden kann.",
    description:
      "Copy when a capability answer is older than the freshness bound",
  },
};

export default deCapabilities;
