import {
  CircleCheck,
  CircleDashed,
  CircleSlash,
  CircleX,
  LoaderCircle,
} from "lucide-react";
import { cn } from "@/common/lib/utils/utils";
import {
  deployStatusKind,
  type DeployStatusKind,
} from "@/features/deploys/lib/deploy-status";

const KIND_ICON: Record<
  DeployStatusKind,
  { Icon: typeof CircleCheck; tone: string }
> = {
  live: { Icon: CircleCheck, tone: "fill-primary text-primary-foreground" },
  succeeded: { Icon: CircleCheck, tone: "text-primary" },
  failed: { Icon: CircleX, tone: "text-destructive" },
  canceled: { Icon: CircleSlash, tone: "text-muted-foreground" },
  in_progress: {
    Icon: LoaderCircle,
    tone: "animate-spin text-foreground motion-reduce:animate-none",
  },
  pending: { Icon: CircleDashed, tone: "text-muted-foreground" },
};

/**
 * A deploy's status as Render draws it: one icon, filled for the deploy that
 * is live now. The status's own name is the icon's accessible name. Shared by
 * the Deploys list rows and the deploy page's Status fact so the two cannot
 * draw one status two ways.
 */
export function DeployStatusIcon({
  status,
  ...rest
}: {
  status: string;
  /** The status's name; omitted where the status is already spelled beside
   *  the icon, which then stays decorative. */
  label?: string;
  className?: string;
}) {
  return <StatusKindIcon kind={deployStatusKind(status)} {...rest} />;
}

/**
 * One of Render's status icons by kind. A cron run draws its status with the
 * same set (w3/m556): Render's run list and deploy list share their icons,
 * the filled check marking the newest success.
 */
export function StatusKindIcon({
  kind,
  label,
  className,
}: {
  kind: DeployStatusKind;
  label?: string;
  className?: string;
}) {
  const { Icon, tone } = KIND_ICON[kind];
  return (
    <Icon
      role={label ? "img" : undefined}
      aria-label={label}
      aria-hidden={label ? undefined : true}
      className={cn("size-[18px] shrink-0", tone, className)}
    />
  );
}
