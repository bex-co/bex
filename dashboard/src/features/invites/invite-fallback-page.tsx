import { useEffect, useState } from "react";
import { Link, useLocation } from "@tanstack/react-router";
import { LoaderCircle } from "lucide-react";
import {
  peekPendingInviteToken,
  stashInviteTokenFromURL,
  takePendingInviteToken,
} from "@/common/lib/invite-token";
import { useTranslations } from "@/common/hooks/use-translations";
import { Button } from "@/common/components/ui/button";
import { InviteRouteSkeleton } from "@/common/components/route-skeletons";
import { useInviteRedemption } from "@/features/team/hooks/use-invite-redemption";
import { InvitationFrame } from "./invitation-frame";
import { takeInviteReturn } from "./invite-return";

type InviteFallbackPageProps = {
  authenticated: boolean;
  email?: string;
  continueTo: (destination: string) => void;
};

/** Browser-owned handoff and review; tokens never enter auth return URLs. */
export function InviteFallbackPage({
  authenticated,
  email,
  continueTo,
}: InviteFallbackPageProps) {
  const { t } = useTranslations();
  const { href } = useLocation();
  const [capture, setCapture] = useState<{
    href: string;
    token: string | null;
    unavailable: boolean;
  } | null>(null);
  useEffect(() => {
    const result = stashInviteTokenFromURL({ scrubAll: true });
    // eslint-disable-next-line react-hooks/set-state-in-effect -- Capture and scrub the bearer on the client before rendering any action.
    setCapture({
      href,
      token: peekPendingInviteToken(),
      unavailable: result === "unavailable",
    });
  }, [href]);

  function leave() {
    takePendingInviteToken();
    continueTo(takeInviteReturn());
  }
  if (!capture || capture.href !== href)
    return <InviteRouteSkeleton authenticated={authenticated} />;
  if (!capture.token)
    return (
      <InvitationFrame>
        <h1 className="text-2xl font-semibold">{t("invites.title")}</h1>
        <p role="alert" className="text-sm text-muted-foreground">
          {t(
            capture.unavailable
              ? "invites.storageUnavailable"
              : "invites.invalid",
          )}
        </p>
        <Button className="mt-auto" onClick={leave}>
          {t("invites.continue")}
        </Button>
      </InvitationFrame>
    );
  if (!authenticated)
    return (
      <InvitationFrame>
        <div className="space-y-2">
          <h1 className="text-2xl font-semibold">{t("invites.title")}</h1>
          <p className="text-sm text-muted-foreground">
            {t("invites.authenticate")}
          </p>
        </div>
        <div className="mt-auto flex flex-col gap-3">
          <Button asChild>
            <Link
              to="/auth/sign-up"
              search={{
                next: "/invite",
                flow: undefined,
                login_challenge: undefined,
              }}
            >
              {t("invites.signUp")}
            </Link>
          </Button>
          <Button variant="outline" asChild>
            <Link
              to="/auth/login"
              search={{
                next: "/invite",
                flow: undefined,
                login_challenge: undefined,
                aal: undefined,
              }}
            >
              {t("invites.signIn")}
            </Link>
          </Button>
          <Button variant="ghost" onClick={leave}>
            {t("invites.notNow")}
          </Button>
        </div>
      </InvitationFrame>
    );
  return (
    <InviteReview
      key={capture.token}
      token={capture.token}
      email={email}
      leave={leave}
      opened={() => {
        takeInviteReturn();
        continueTo("/");
      }}
    />
  );
}

function InviteReview({
  token,
  email,
  leave,
  opened,
}: {
  token: string;
  email?: string;
  leave: () => void;
  opened: () => void;
}) {
  const { t } = useTranslations();
  const state = useInviteRedemption(token, opened);
  if (state.loading) return <InviteRouteSkeleton />;
  const details = state.details;
  return (
    <InvitationFrame>
      <div className="space-y-3">
        <p className="text-sm text-muted-foreground">{t("invites.title")}</p>
        <h1 className="break-words text-2xl font-semibold">
          {details
            ? t(state.joined ? "invites.memberTitle" : "invites.joinTitle", {
                workspace: details.workspaceName ?? "",
              })
            : t("invites.unavailableTitle")}
        </h1>
        {details && (
          <p className="text-sm text-muted-foreground">
            {t(state.joined ? "invites.memberRole" : "invites.role", {
              role: t(`team.role.${details.role}`),
            })}
          </p>
        )}
        {details?.inviterEmail && !state.joined && (
          <p className="break-words text-sm text-muted-foreground">
            {t("invites.inviter", { email: details.inviterEmail })}
          </p>
        )}
        {email && (
          <p className="break-words text-sm">
            {t("invites.account", { email })}
          </p>
        )}
      </div>
      {state.errorKey && (
        <p role="alert" className="text-sm text-destructive">
          {t(state.errorKey)}
        </p>
      )}
      <div className="mt-auto flex flex-col gap-3">
        {details ? (
          <Button disabled={state.busy} onClick={() => void state.accept()}>
            {state.busy && (
              <LoaderCircle
                className="size-4 animate-spin"
                aria-hidden="true"
              />
            )}
            {t(
              state.busy
                ? state.joined
                  ? "invites.opening"
                  : "invites.joining"
                : state.joined
                  ? "invites.open"
                  : "invites.join",
            )}
          </Button>
        ) : (
          state.retryable && (
            <Button onClick={state.retry}>{t("invites.retry")}</Button>
          )
        )}
        <Button variant="ghost" disabled={state.busy} onClick={leave}>
          {t("invites.notNow")}
        </Button>
      </div>
    </InvitationFrame>
  );
}
