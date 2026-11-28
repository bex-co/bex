import type { ComponentProps, MouseEvent } from "react";
import { Link, useNavigate } from "@tanstack/react-router";
import {
  updatePlanSearch,
  upgradePlanTarget,
} from "@/features/workspaces/lib/update-plan";
import type { WorkspacePlanId } from "@/features/workspaces/types";

type UpgradePlanLinkProps = Omit<
  ComponentProps<typeof Link>,
  "to" | "search" | "params"
> & {
  /** The plan that allows what the link is beside. */
  plan: WorkspacePlanId;
};

/**
 * An upgrade link into Render's plan page (w3/m708): the plan that allows the
 * feature, and the page it was clicked on as `next`, as Render's
 * `/billing/update-plan?plan=professional&next=${pathname}` links write it.
 * The `next` is read when the link is followed, so the server-rendered href
 * (the plan alone) and the client's agree; a link opened in a new tab keeps
 * the plan and returns home, as Render's does without `next`.
 */
export function UpgradePlanLink({
  plan,
  onClick,
  ...props
}: UpgradePlanLinkProps) {
  const navigate = useNavigate();
  return (
    <Link
      {...props}
      to="/billing/update-plan"
      search={updatePlanSearch(plan)}
      onClick={(event: MouseEvent<HTMLAnchorElement>) => {
        onClick?.(event);
        if (
          event.defaultPrevented ||
          event.button !== 0 ||
          event.metaKey ||
          event.ctrlKey ||
          event.shiftKey ||
          event.altKey
        ) {
          return;
        }
        event.preventDefault();
        void navigate(upgradePlanTarget(plan));
      }}
    />
  );
}
