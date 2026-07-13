import { useEffect, useState } from "react";
import { getRouteApi } from "@tanstack/react-router";
import { AlertTriangle, KeyRound, ShieldCheck } from "lucide-react";
import { Alert, AlertDescription } from "@/common/components/ui/alert";
import { Button } from "@/common/components/ui/button";
import {
  Card,
  CardContent,
  CardDescription,
  CardFooter,
  CardHeader,
  CardTitle,
} from "@/common/components/ui/card";
import { useTranslations } from "@/common/hooks/use-translations";
import { cn } from "@/common/lib/utils/utils.ts";
import { AuthPageShell } from "@/features/auth/components/auth-page-shell";

const route = getRouteApi("/auth/consent");

/**
 * Scopes bex's own OAuth clients actually ask for get a sentence a human can
 * act on; anything else (an agent asking for something we've never seen) is
 * shown verbatim rather than dressed up — an unexplained scope should look
 * unexplained.
 */
const SCOPE_DESCRIPTIONS: Record<string, string> = {
  openid: "auth.consentScopeOpenid",
  offline_access: "auth.consentScopeOfflineAccess",
  profile: "auth.consentScopeProfile",
  email: "auth.consentScopeEmail",
};

/**
 * OAuth2 consent screen (docs/ADR012-auth.md §7, w4/m16). Reached only when the
 * route's GET handler decided a third-party client needs a human: it hands the
 * safe-to-render subset of Hydra's consent request down as loader data. The
 * decision goes back as a plain form POST to this same URL — no client-side
 * fetch, so an approval is always a real, same-origin, user-initiated
 * submission carrying the challenge-bound CSRF token.
 */
export default function ConsentPage() {
  const { consent } = route.useLoaderData();
  const { consent_challenge: challenge } = route.useSearch();
  const { t } = useTranslations();
  const [submitting, setSubmitting] = useState(false);

  // Only a document request runs the server handler that produces `consent`.
  // Landing here by client-side navigation — which is how the login-first bounce
  // returns (login-page navigates to `next`) — leaves the page with a challenge
  // and no view: reload it as a document so the handler can answer. Terminates
  // after one hop: the document response either renders the view or redirects.
  const needsDocumentLoad = !consent && !!challenge;
  useEffect(() => {
    if (needsDocumentLoad) window.location.replace(window.location.href);
  }, [needsDocumentLoad]);

  if (!consent) {
    return (
      <AuthPageShell
        title={t("auth.consentExpiredTitle")}
        subtitle={t("auth.consentExpiredSubtitle")}
      >
        {!needsDocumentLoad && (
          <Button asChild variant="outline">
            <a href="/">{t("common.goHome")}</a>
          </Button>
        )}
      </AuthPageShell>
    );
  }

  return (
    <AuthPageShell
      title={t("auth.consentTitle")}
      subtitle={t("auth.consentSubtitle", { client: consent.clientName })}
    >
      <Card>
        <CardHeader>
          <CardTitle className="flex items-center gap-2">
            <ShieldCheck className="size-5 text-primary" />
            {consent.clientName}
          </CardTitle>
          <CardDescription>
            {consent.clientUri ? (
              <a
                href={consent.clientUri}
                rel="noreferrer noopener"
                target="_blank"
                className="underline underline-offset-4"
              >
                {consent.clientUri}
              </a>
            ) : (
              <span className="font-mono text-xs">{consent.clientId}</span>
            )}
          </CardDescription>
        </CardHeader>

        <CardContent className="space-y-4">
          {consent.retryAfterFailure && (
            <Alert variant="destructive">
              <AlertTriangle />
              <AlertDescription>{t("auth.consentFailed")}</AlertDescription>
            </Alert>
          )}

          <div className="space-y-2">
            <p className="text-sm font-medium">
              {t("auth.consentScopesTitle")}
            </p>
            <ul className="space-y-2">
              {consent.scopes.map((scope) => (
                <li key={scope} className="flex items-start gap-2 text-sm">
                  <KeyRound className="size-4 mt-0.5 text-muted-foreground shrink-0" />
                  <span>
                    <span className="font-mono text-xs">{scope}</span>
                    {SCOPE_DESCRIPTIONS[scope] && (
                      <span className="block text-muted-foreground">
                        {t(SCOPE_DESCRIPTIONS[scope])}
                      </span>
                    )}
                  </span>
                </li>
              ))}
            </ul>
          </div>

          {consent.audiences.length > 0 && (
            <div className="space-y-1">
              <p className="text-sm font-medium">
                {t("auth.consentAudienceTitle")}
              </p>
              <p className="font-mono text-xs text-muted-foreground break-all">
                {consent.audiences.join(", ")}
              </p>
            </div>
          )}

          <p className="text-xs text-muted-foreground">
            {t("auth.consentRememberHint")}
          </p>
        </CardContent>

        <CardFooter>
          <form
            method="post"
            action="/auth/consent"
            // The decision rides on the submit button that was pressed, and the
            // browser builds this form's entry list *after* this handler returns
            // — so the buttons must NOT be `disabled` while submitting: React's
            // synchronous re-render would disable the submitter before the entry
            // list is built, a disabled control is excluded from it, and the POST
            // would arrive with no `decision` at all (a 400 for every real
            // browser, invisible to a scripted client that posts the fields
            // itself). Guard the double-submit without touching the controls.
            className={cn(
              "flex w-full gap-3",
              submitting && "pointer-events-none opacity-70",
            )}
            aria-busy={submitting}
            onSubmit={(event) => {
              if (submitting) event.preventDefault();
              setSubmitting(true);
            }}
          >
            <input
              type="hidden"
              name="consent_challenge"
              value={consent.challenge}
            />
            <input type="hidden" name="csrf_token" value={consent.csrfToken} />
            <Button
              type="submit"
              name="decision"
              value="deny"
              variant="outline"
              className="flex-1"
            >
              {t("auth.consentDeny")}
            </Button>
            <Button
              type="submit"
              name="decision"
              value="approve"
              className="flex-1"
            >
              {t("auth.consentApprove")}
            </Button>
          </form>
        </CardFooter>
      </Card>
    </AuthPageShell>
  );
}
