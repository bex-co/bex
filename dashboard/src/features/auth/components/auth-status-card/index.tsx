import type { ReactNode } from "react";

/**
 * The single status card of `/auth/account-deleted` and `/auth/error`: an icon
 * beside a message, then one action. `AuthStatusRouteSkeleton`
 * (common/components/route-skeletons.tsx) is its pending preview.
 */
export function AuthStatusCard({
  icon,
  message,
  action,
}: {
  icon: ReactNode;
  message: ReactNode;
  action: ReactNode;
}) {
  return (
    <div className="space-y-6 rounded-xl border bg-card p-6 shadow-sm sm:p-8">
      <div className="flex items-start gap-3">
        {icon}
        {/* A div, not a p: the message may be a loading Skeleton (a div). */}
        <div className="text-sm text-muted-foreground">{message}</div>
      </div>
      {action}
    </div>
  );
}
