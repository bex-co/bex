import { Loader2 } from "lucide-react";
import { Badge } from "@/common/components/ui/badge";
import { useTranslations } from "@/common/hooks/use-translations";
import { ownValue } from "@/common/lib/own-value";
import { buildStatusKey } from "@/features/agent-environment/lib/repo-environment-builds";

type ChipVariant = "success" | "destructive" | "secondary" | "outline";

/** The session phase chip's live-work blue (agent-sessions/session-list). */
const ACTIVE_CHIP_CLASS =
  "border-transparent bg-blue-600/15 text-blue-700 dark:text-blue-400";

/**
 * A build's status in the session phase chip's colours: a running build is
 * live work (blue), a successful one is ready (green), a failed one red, a
 * cancelled one grey like a canceled session.
 */
const STATUS_CHIP: Record<
  string,
  { variant: ChipVariant; className?: string }
> = {
  queued: { variant: "outline" },
  building: { variant: "outline", className: ACTIVE_CHIP_CLASS },
  success: { variant: "success" },
  failed: { variant: "destructive" },
  cancelled: { variant: "secondary" },
};

/** Queued, Building, Ready, Failed or Cancelled (ADR111 D2). */
export function RepoEnvironmentBuildBadge({ status }: { status: string }) {
  const { t } = useTranslations();
  const chip = ownValue(STATUS_CHIP, status) ?? { variant: "outline" };
  const key = buildStatusKey(status);
  return (
    <Badge
      variant={chip.variant}
      className={chip.className}
      data-testid="repo-environment-build-status"
      data-status={status}
    >
      {status === "building" ? <Loader2 className="animate-spin" /> : null}
      {key ? t(key) : status}
    </Badge>
  );
}
