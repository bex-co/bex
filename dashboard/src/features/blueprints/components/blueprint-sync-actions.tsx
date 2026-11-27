import {
  CircleArrowRight,
  CircleCheck,
  CircleHelp,
  CircleX,
  Clock,
  LoaderCircle,
} from "lucide-react";
import { useTranslations } from "@/common/hooks/use-translations";
import { cn } from "@/common/lib/utils/utils";
import { blueprintKindLabel } from "@/features/blueprints/lib/kind-labels";
import {
  PLACEHOLDER_SENTINEL,
  aroundPlaceholder,
} from "@/common/lib/i18n-placeholder";
import {
  SYNC_ACTION_SENTENCE,
  SYNC_STATE_LABEL,
  changingSyncActions,
} from "@/features/blueprints/lib/sync-state";
import type { BlueprintSyncAction } from "@/features/blueprints/types";

// Each action state's icon on Render's sync page (the row's leading mark):
// waiting an outlined arrow, pending a clock, running a spinner, success a
// check, error a cross, anything else an unknown mark.
const ACTION_STATE_ICON: Record<
  string,
  { Icon: typeof CircleCheck; tone: string }
> = {
  created: { Icon: CircleArrowRight, tone: "text-muted-foreground" },
  pending: { Icon: Clock, tone: "text-amber-600 dark:text-amber-500" },
  running: {
    Icon: LoaderCircle,
    tone: "animate-spin text-muted-foreground motion-reduce:animate-none",
  },
  success: { Icon: CircleCheck, tone: "text-primary" },
  error: { Icon: CircleX, tone: "text-destructive" },
};
const UNKNOWN_STATE = { Icon: CircleHelp, tone: "text-muted-foreground" };

function ActionRow({ action, state }: BlueprintSyncAction) {
  const { t } = useTranslations();
  const { Icon, tone } = ACTION_STATE_ICON[state] ?? UNKNOWN_STATE;
  const kindKey = blueprintKindLabel(action.kind);
  const stateKey = SYNC_STATE_LABEL[state];
  const [before, after] = aroundPlaceholder(
    t(SYNC_ACTION_SENTENCE[action.operation], {
      kind: kindKey ? t(kindKey) : action.kind,
      name: PLACEHOLDER_SENTINEL,
    }),
  );
  const fields = (action.changedFields ?? [])
    .map((field) => field.path)
    .filter(Boolean);
  return (
    <li className="flex items-start gap-3 text-sm leading-5">
      <Icon
        role="img"
        aria-label={
          stateKey ? t(stateKey) : t("blueprints.syncActionStateUnknown")
        }
        className={cn("mt-0.5 size-4 shrink-0", tone)}
      />
      <div className="min-w-0 space-y-0.5">
        <p className="break-words">
          {before}
          <strong className="font-semibold">{action.name}</strong>
          {after}
        </p>
        {action.operation === "update" && fields.length > 0 ? (
          <p className="break-words text-xs text-muted-foreground">
            {t("blueprints.previewChangedFields", {
              fields: fields.join(", "),
            })}
          </p>
        ) : null}
        {action.operation === "detach" ? (
          <p className="break-words text-xs text-muted-foreground">
            {t("blueprints.syncActionDetachNote")}
          </p>
        ) : null}
        {action.operation === "error" && action.message ? (
          <p className="break-words text-xs text-destructive">
            {action.message}
          </p>
        ) : null}
      </div>
    </li>
  );
}

/**
 * The actions a sync planned, one row each with its state, as Render's sync
 * page lists them (w3/m678, docs/render-artifacts/blueprint-pages.md § A
 * sync's page): **Resources already up to date** for a plan with none, Render's
 * critical line for a failed sync that has neither actions nor errors, and,
 * for a sync recorded before every sync kept its actions, a line saying so.
 */
export function BlueprintSyncActions({
  state,
  actions,
  actionsRecorded,
  hasErrors,
}: {
  state: string;
  actions: readonly BlueprintSyncAction[];
  actionsRecorded: boolean;
  hasErrors: boolean;
}) {
  const { t } = useTranslations();
  if (!actionsRecorded) {
    return (
      <p className="text-sm text-muted-foreground">
        {t("blueprints.syncActionsNotRecorded")}
      </p>
    );
  }
  const changing = changingSyncActions(actions);
  if (changing.length > 0) {
    return (
      <ul aria-label={t("blueprints.plannedActions")} className="space-y-3">
        {changing.map((a, i) => (
          <ActionRow key={`${a.action.kind}:${a.action.name}:${i}`} {...a} />
        ))}
      </ul>
    );
  }
  if (state === "error" && !hasErrors) {
    return (
      <div
        role="alert"
        className="rounded-md border border-destructive/50 p-3 text-sm"
      >
        <p className="font-medium text-destructive">
          {t("blueprints.syncNotFullySynced")}
        </p>
        <p className="text-muted-foreground">
          {t("blueprints.syncNotFullySyncedBody")}
        </p>
      </div>
    );
  }
  return <p className="text-sm">{t("blueprints.syncUpToDate")}</p>;
}
