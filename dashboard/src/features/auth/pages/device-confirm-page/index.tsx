import { useEffect, useState } from "react";
import { getRouteApi } from "@tanstack/react-router";
import { KeyRound, ShieldCheck } from "lucide-react";
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
import type { DeviceErrorCode } from "@/common/server-fn/hydra-device";

const route = getRouteApi("/auth/device/");

/** Title/subtitle for a recoverable device-verification failure (w10/m8
 * t001). `missing_code`/`invalid_code` share the existing "expired" copy —
 * both remedy the same way: run `bex login` again. */
function deviceErrorCopy(
  code: DeviceErrorCode | null,
  t: ReturnType<typeof useTranslations>["t"],
): { title: string; subtitle: string } {
  switch (code) {
    case "unconfigured":
      return {
        title: t("auth.deviceUnavailableTitle"),
        subtitle: t("auth.deviceUnavailableSubtitle"),
      };
    case "unexpected_client":
      return {
        title: t("auth.deviceRefusedTitle"),
        subtitle: t("auth.deviceRefusedSubtitle"),
      };
    default:
      return {
        title: t("auth.deviceExpiredTitle"),
        subtitle: t("auth.deviceExpiredSubtitle"),
      };
  }
}

/**
 * CLI device-authorize confirmation (docs/ADR035-ssh.md's RFC 8628 bridge,
 * w4/m31/t002). Reached only when the route's GET handler decided a signed-in
 * human needs to confirm: it hands the user_code/device_challenge down as
 * loader data, the same shape the OAuth2 consent page uses for its own
 * challenge. The decision goes back as a plain form POST to this same URL —
 * no client-side fetch, so a confirmation is always a real, same-origin,
 * user-initiated submission (codex-security #9).
 */
export default function DeviceConfirmPage() {
  const { device, deviceErrorCode } = route.useLoaderData();
  const { user_code: userCode, device_challenge: challenge } =
    route.useSearch();
  const { t } = useTranslations();
  const [submitting, setSubmitting] = useState(false);

  // Only a document request runs the server handler that produces `device`.
  // Landing here by client-side navigation — which is how the login-first
  // bounce returns (login-page navigates to `next`) — leaves the page with a
  // code/challenge and no view: reload it as a document so the handler can
  // answer. Terminates after one hop: the document response either renders
  // the view or redirects. A recovered error code means the handler already
  // answered — never re-trigger the reload loop for it.
  const needsDocumentLoad =
    !device && !deviceErrorCode && !!userCode && !!challenge;
  useEffect(() => {
    if (needsDocumentLoad) window.location.replace(window.location.href);
  }, [needsDocumentLoad]);

  if (!device) {
    const { title, subtitle } = deviceErrorCopy(deviceErrorCode, t);
    return (
      <AuthPageShell title={title} subtitle={subtitle}>
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
      title={t("auth.deviceTitle")}
      subtitle={t("auth.deviceConfirmSubtitle")}
    >
      <Card>
        <CardHeader>
          <CardTitle className="flex items-center gap-2">
            <ShieldCheck className="size-5 text-primary" />
            {t("auth.deviceConfirmHeading")}
          </CardTitle>
          <CardDescription>
            {t("auth.deviceConfirmDescription")}
          </CardDescription>
        </CardHeader>

        <CardContent className="space-y-4">
          <div className="flex items-center justify-center gap-2 rounded-lg border bg-muted/40 py-4 font-mono text-lg tracking-[0.3em]">
            <KeyRound className="size-4 shrink-0 text-muted-foreground" />
            {device.userCode}
          </div>
          <p className="text-xs text-muted-foreground">
            {t("auth.deviceConfirmHint")}
          </p>
        </CardContent>

        <CardFooter>
          <form
            method="POST"
            action="/auth/device"
            // Mirrors the consent form: the decision rides on the submit
            // button pressed, and the browser builds the entry list *after*
            // this handler returns — so the buttons must NOT be `disabled`
            // while submitting, or a disabled control drops out of the entry
            // list and the POST arrives with fields missing.
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
            <input type="hidden" name="user_code" value={device.userCode} />
            <input
              type="hidden"
              name="device_challenge"
              value={device.challenge}
            />
            <Button asChild variant="outline" className="flex-1">
              <a href="/">{t("auth.deviceConfirmCancel")}</a>
            </Button>
            <Button type="submit" className="flex-1">
              {t("auth.deviceConfirmAuthorize")}
            </Button>
          </form>
        </CardFooter>
      </Card>
    </AuthPageShell>
  );
}
