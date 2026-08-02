import { useEffect, useRef } from "react";
import { LoaderCircle } from "lucide-react";
import { stashInviteTokenFromURL } from "@/common/lib/invite-token";
import { useTranslations } from "@/common/hooks/use-translations";

type InviteFallbackPageProps = {
  authenticated: boolean;
  continueTo: (destination: "/" | "/auth/sign-up") => void;
};

/** Browser fallback for the app-owned `/invite` HTTPS link. The bearer token
 * is captured and scrubbed before any navigation; authenticated visitors go
 * to the dashboard redemption hook, others enter the existing sign-up flow. */
export function InviteFallbackPage({
  authenticated,
  continueTo,
}: InviteFallbackPageProps) {
  const { t } = useTranslations();
  const started = useRef(false);

  useEffect(() => {
    if (started.current) return;
    started.current = true;
    stashInviteTokenFromURL({ scrubAll: true });
    continueTo(authenticated ? "/" : "/auth/sign-up");
  }, [authenticated, continueTo]);

  return (
    <main className="min-h-screen bg-background px-6 py-16 text-foreground">
      <div className="mx-auto flex max-w-sm flex-col items-center gap-4 text-center">
        <LoaderCircle className="size-6 animate-spin" aria-hidden="true" />
        <h1 className="text-lg font-semibold">{t("invites.openingTitle")}</h1>
        <p className="text-sm text-muted-foreground">
          {t("invites.openingDescription")}
        </p>
      </div>
    </main>
  );
}
