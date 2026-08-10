import { Link } from "@tanstack/react-router";
import { KeyRound } from "lucide-react";
import type { ReactNode } from "react";
import { Button } from "@/common/components/ui/button";
import { trackEvent } from "@/common/lib/telemetry";
import { useTranslations } from "@/common/hooks/use-translations";

export interface AddSshKeyCtaProps {
  /**
   * Same-origin path to return to after a key is saved (e.g. `/agents/ags-…`).
   * Threaded through `/settings` as `?returnTo=` and validated there before any
   * navigation (safe-next.ts). Omit for no round-trip.
   */
  returnTo?: string;
  /** Bounded funnel label (e.g. "agent-session-zed", "service-ssh"). */
  surface?: string;
  /** Optional description override (default: the generic gate copy). */
  description?: string;
  /** Optional second door (e.g. "Open a browser terminal") rendered below. */
  secondaryAction?: ReactNode;
}

/**
 * The in-product replacement for a doomed SSH affordance when the caller has no
 * registered key (w2/m66). It keeps the value proposition visible and turns the
 * off-surface `Permission denied (publickey)` dead-end into a one-click,
 * round-tripping setup: add your key on `/settings`, come back here, connect.
 *
 * It never generates a keypair — private keys never touch the browser
 * (docs/ADR035-ssh.md); the user pastes their own public key.
 */
export function AddSshKeyCta({
  returnTo,
  surface,
  description,
  secondaryAction,
}: AddSshKeyCtaProps) {
  const { t } = useTranslations();
  return (
    <div className="space-y-2">
      <div className="flex items-center gap-2 text-sm font-medium">
        <KeyRound className="size-4" />
        {t("sshKeys.gateTitle")}
      </div>
      <p className="text-muted-foreground text-xs">
        {description ?? t("sshKeys.gateBody")}
      </p>
      <Button asChild size="sm" className="w-full">
        {/* `addKey` opens the form (SSR-safe query param); the hash scrolls the
            panel into view natively; `returnTo` round-trips back here. */}
        <Link
          to="/settings"
          hash="ssh-public-keys"
          search={returnTo ? { addKey: true, returnTo } : { addKey: true }}
          onClick={() =>
            trackEvent(
              "ssh_gate_cta_clicked",
              surface ? { surface } : undefined,
            )
          }
        >
          {t("sshKeys.gateAddKey")}
        </Link>
      </Button>
      {secondaryAction}
    </div>
  );
}
