import { useEffect, useState } from "react";
import { Link, useLocation } from "@tanstack/react-router";
import { LoaderCircle } from "lucide-react";
import {
  type PendingInvite,
  peekPendingInvite,
  stashInviteTokenFromURL,
  takePendingInvite,
} from "@/common/lib/invite-token";
import { useTranslations } from "@/common/hooks/use-translations";
import { Button } from "@/common/components/ui/button";
import { InviteRouteSkeleton } from "@/common/components/route-skeletons";
import { useInviteRedemption } from "@/features/team/hooks/use-invite-redemption";
import { useOrganizationInviteRedemption } from "@/features/organizations/hooks/use-organization-invite-redemption";
import {
  ORGANIZATION_ROLE_LABEL_KEYS,
  type OrganizationRole,
} from "@/features/organizations/types";
import { InvitationFrame } from "./invitation-frame";
import { takeInviteReturn } from "./invite-return";

type InviteFallbackPageProps = {
  authenticated: boolean;
  email?: string;
  continueTo: (destination: string) => void;
};

/** Browser-owned handoff and review; tokens never enter auth return URLs. A
 * link carries a workspace invite (`#invite=`) or an organization invite
 * (`#org-invite=`, w7/083); both are reviewed here and accepted explicitly. */
export function InviteFallbackPage({
  authenticated,
  email,
  continueTo,
}: InviteFallbackPageProps) {
  const { t } = useTranslations();
  const { href } = useLocation();
  const [capture, setCapture] = useState<{
    href: string;
    invite: PendingInvite | null;
    unavailable: boolean;
  } | null>(null);
  useEffect(() => {
    const result = stashInviteTokenFromURL({ scrubAll: true });
    // eslint-disable-next-line react-hooks/set-state-in-effect -- Capture and scrub the bearer on the client before rendering any action.
    setCapture({
      href,
      invite: peekPendingInvite(),
      unavailable: result === "unavailable",
    });
  }, [href]);

  function leave() {
    takePendingInvite();
    continueTo(takeInviteReturn());
  }
  if (!capture || capture.href !== href)
    return <InviteRouteSkeleton authenticated={authenticated} />;
  const invite = capture.invite;
  if (!invite)
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
  const organization = invite.kind === "organization";
  if (!authenticated)
    return (
      <InvitationFrame>
        <div className="space-y-2">
          <h1 className="text-2xl font-semibold">
            {t(organization ? "invites.orgTitle" : "invites.title")}
          </h1>
          <p className="text-sm text-muted-foreground">
            {t(
              organization ? "invites.orgAuthenticate" : "invites.authenticate",
            )}
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
  return organization ? (
    <OrganizationInviteReview
      key={`organization:${invite.token}`}
      token={invite.token}
      email={email}
      leave={leave}
      opened={(organizationId) => {
        takeInviteReturn();
        continueTo(`/org/${organizationId}/workspaces`);
      }}
    />
  ) : (
    <WorkspaceInviteReview
      key={`workspace:${invite.token}`}
      token={invite.token}
      email={email}
      leave={leave}
      opened={() => {
        takeInviteReturn();
        continueTo("/");
      }}
    />
  );
}

function WorkspaceInviteReview({
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
  const details = state.details;
  return (
    <InviteDecision
      state={state}
      email={email}
      leave={leave}
      kicker={t("invites.title")}
      heading={
        details &&
        t(state.joined ? "invites.memberTitle" : "invites.joinTitle", {
          workspace: details.workspaceName ?? "",
        })
      }
      roleLine={
        details &&
        t(state.joined ? "invites.memberRole" : "invites.role", {
          role: t(`team.role.${details.role}`),
        })
      }
      inviterEmail={details?.inviterEmail}
      actionLabel={t(state.joined ? "invites.open" : "invites.join")}
    />
  );
}

function OrganizationInviteReview({
  token,
  email,
  leave,
  opened,
}: {
  token: string;
  email?: string;
  leave: () => void;
  opened: (organizationId: string) => void;
}) {
  const { t } = useTranslations();
  const state = useOrganizationInviteRedemption(token, opened);
  const details = state.details;
  const roleKey =
    details && ORGANIZATION_ROLE_LABEL_KEYS[details.role as OrganizationRole];
  return (
    <InviteDecision
      state={state}
      email={email}
      leave={leave}
      kicker={t("invites.orgTitle")}
      heading={
        details &&
        t(state.joined ? "invites.orgMemberTitle" : "invites.orgJoinTitle", {
          organization: details.organizationName,
        })
      }
      roleLine={
        details &&
        t(state.joined ? "invites.memberRole" : "invites.orgRole", {
          role: roleKey ? t(roleKey) : details.role,
        })
      }
      inviterEmail={details?.inviterEmail}
      actionLabel={t(state.joined ? "invites.orgOpen" : "invites.orgJoin")}
    />
  );
}

/** One invitation decision: what it joins, as what, from whom, for which
 * signed-in account, and the explicit Join (or Open) action. */
function InviteDecision({
  state,
  email,
  leave,
  kicker,
  heading,
  roleLine,
  inviterEmail,
  actionLabel,
}: {
  state: {
    loading: boolean;
    errorKey: string | null;
    retryable: boolean;
    busy: boolean;
    joined: boolean;
    accept: () => Promise<void>;
    retry: () => void;
  };
  email?: string;
  leave: () => void;
  kicker: string;
  heading: string | null | undefined;
  roleLine: string | null | undefined;
  inviterEmail: string | null | undefined;
  actionLabel: string;
}) {
  const { t } = useTranslations();
  if (state.loading) return <InviteRouteSkeleton />;
  return (
    <InvitationFrame>
      <div className="space-y-3">
        <p className="text-sm text-muted-foreground">{kicker}</p>
        <h1 className="break-words text-2xl font-semibold">
          {heading ?? t("invites.unavailableTitle")}
        </h1>
        {roleLine && (
          <p className="text-sm text-muted-foreground">{roleLine}</p>
        )}
        {heading && inviterEmail && !state.joined && (
          <p className="break-words text-sm text-muted-foreground">
            {t("invites.inviter", { email: inviterEmail })}
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
        {heading ? (
          <Button disabled={state.busy} onClick={() => void state.accept()}>
            {state.busy && (
              <LoaderCircle
                className="size-4 animate-spin"
                aria-hidden="true"
              />
            )}
            {state.busy
              ? t(state.joined ? "invites.opening" : "invites.joining")
              : actionLabel}
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
