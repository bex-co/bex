import { useId } from "react";
import { Mail } from "lucide-react";
import { Badge } from "@/common/components/ui/badge";
import { Button } from "@/common/components/ui/button";
import { useTranslations } from "@/common/hooks/use-translations";
import { useCardRadioGroup } from "@/common/hooks/use-card-radio-group";
import { cn } from "@/common/lib/utils/utils.ts";
import {
  WORKSPACE_PLAN_CATALOG,
  type WorkspacePlanCatalogEntry,
  type WorkspacePlanId,
} from "@/features/workspaces/types";

/** Where Enterprise's Get in touch writes: bex's contact address, the one
 *  bex.co/pricing's "contact us" opens (Render's is its /contact page). */
const ENTERPRISE_CONTACT_HREF = "mailto:hello@bex.co";

export interface PlanPickerProps {
  selected: WorkspacePlanId;
  onSelect: (plan: WorkspacePlanId) => void;
  disabled?: boolean;
  /**
   * The workspace's plan, on Change plan (w3/m708): Render's TeamPlanPicker
   * for an existing workspace. Its card says Current plan and cannot be
   * chosen again, a chosen other card says New plan, and Enterprise is Get in
   * touch rather than a plan to select. Unset on New Workspace.
   */
  current?: string;
}

/**
 * bex's workspace capability cards mirror Render's plan ids and limits, with
 * monthly fees 30% below Render's (ADR030 / pricing.yaml): Hobby $0, Pro
 * $17.50, Scale $349.30, Enterprise custom. Scale and Enterprise are charged
 * once per organization, on its primary workspace (w7/m464, ADR115 D2), so
 * their labels say "per organization". Resource-tier usage is billed
 * separately (ADR046) and stays in the footnote under the cards.
 */
export function PlanPicker({
  selected,
  onSelect,
  disabled = false,
  current,
}: PlanPickerProps) {
  const { t } = useTranslations();
  const usageNoteId = useId();
  const updating = current !== undefined;
  const contactOnly = (plan: WorkspacePlanCatalogEntry) =>
    updating && plan.id === "enterprise";
  const radios = WORKSPACE_PLAN_CATALOG.filter((plan) => !contactOnly(plan));
  const selectable = radios
    .map((plan) => plan.id)
    .filter((id) => id !== current);
  const radio = useCardRadioGroup(selectable, selected, onSelect, disabled);

  const cards = radios.map((plan) => {
    const checked = selected === plan.id;
    const isCurrent = plan.id === current;
    const isNew = updating && checked && !isCurrent;
    const chosen = isNew || (!updating && checked);
    return (
      <button
        key={plan.id}
        {...(isCurrent ? { tabIndex: -1 } : radio(plan.id))}
        type="button"
        role="radio"
        aria-checked={checked}
        disabled={disabled || isCurrent}
        onClick={() => onSelect(plan.id)}
        className={cn(
          CARD_CLASS,
          "hover:border-foreground/50 disabled:cursor-not-allowed",
          // The current plan is shown at full strength, only not
          // selectable; a disabled picker greys every card.
          disabled ? "disabled:opacity-60" : null,
          checked ? "border-primary ring-2 ring-primary" : "border-border",
        )}
      >
        <PlanCardBody
          plan={plan}
          badge={
            isCurrent
              ? t("workspaces.planCurrentBadge")
              : isNew
                ? t("workspaces.planNewBadge")
                : null
          }
        />
        <span
          className={cn(
            "mt-auto text-sm font-medium",
            chosen ? "text-primary" : "text-muted-foreground",
          )}
        >
          {chosen ? t("workspaces.planSelected") : t("workspaces.planSelect")}
        </span>
      </button>
    );
  });
  const group = {
    role: "radiogroup",
    "aria-label": t("workspaces.planPickerLabel"),
    "aria-describedby": usageNoteId,
  };

  return (
    <div className="space-y-3">
      {updating ? (
        // Enterprise sits in the cards' grid but outside the radio group: it
        // is a contact link, not a plan to select.
        <div className="grid grid-cols-1 gap-4 @xl:grid-cols-2 @4xl:grid-cols-4">
          <div {...group} className="contents">
            {cards}
          </div>
          {WORKSPACE_PLAN_CATALOG.filter(contactOnly).map((plan) => (
            <div
              key={plan.id}
              className={cn(CARD_CLASS, "border-border")}
              data-testid={`plan-card-${plan.id}`}
            >
              <PlanCardBody
                plan={plan}
                badge={
                  plan.id === current ? t("workspaces.planCurrentBadge") : null
                }
              />
              <Button asChild variant="outline" size="sm" className="mt-auto">
                <a href={ENTERPRISE_CONTACT_HREF}>
                  {t("workspaces.planContact")}
                  <Mail aria-hidden="true" />
                </a>
              </Button>
            </div>
          ))}
        </div>
      ) : (
        <div {...group} className="grid grid-cols-1 gap-4 sm:grid-cols-2">
          {cards}
        </div>
      )}
      <p id={usageNoteId} className="text-muted-foreground text-xs">
        {t("workspaces.planUsageBillingNote")}
      </p>
    </div>
  );
}

const CARD_CLASS =
  "flex min-h-56 flex-col rounded-xl border p-5 text-left transition-colors";

/** A plan card's name, badge, price, pitch and limits. */
function PlanCardBody({
  plan,
  badge,
}: {
  plan: WorkspacePlanCatalogEntry;
  badge: string | null;
}) {
  const { t } = useTranslations();
  return (
    <>
      {/* w7/088: the name never breaks inside a word. A price too long to
          sit beside it ("$349.30/mo per organization" in a ~220 px card)
          wraps under the name instead. */}
      <div className="mb-2 flex flex-wrap items-baseline justify-between gap-x-2 gap-y-0.5">
        <span className="flex shrink-0 items-center gap-2 whitespace-nowrap text-lg font-semibold">
          {t(plan.nameKey)}
          {badge ? (
            <Badge variant="secondary" className="font-medium">
              {badge}
            </Badge>
          ) : null}
        </span>
        <span className="text-muted-foreground text-sm font-medium">
          {t(plan.billingKey)}
        </span>
      </div>
      <p className="text-muted-foreground mb-3 text-sm">
        {t(plan.descriptionKey)}
      </p>
      <ul className="text-muted-foreground mb-4 list-disc space-y-1 pl-4 text-sm">
        {plan.bulletKeys.map((key) => (
          <li key={key}>{t(key)}</li>
        ))}
      </ul>
    </>
  );
}
