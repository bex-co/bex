// Copyright 2026 Tian Pan
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

import { useRef, useState } from "react";
import { useMutation } from "@apollo/client/react";
import { Github, Loader2 } from "lucide-react";
import {
  Alert,
  AlertDescription,
  AlertTitle,
} from "@/common/components/ui/alert";
import { Button } from "@/common/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/common/components/ui/dialog";
import { SudoCommandField } from "@/common/components/sudo-command-field";
import { graphQLErrorMessage } from "@/common/lib/graphql-error";
import { useTranslations } from "@/common/hooks/use-translations";
import {
  clearBrowserAccountState,
  endBrowserSession,
} from "@/common/lib/ory/logout";
import { DeleteAccountDocument } from "@/graphql/definitions";
import { accountDeletionConfirmation } from "@/features/auth/pages/settings-page/account-deletion-card";
import { SELF_HOST_URL } from "../../lib/payment-setup";

/**
 * The payment wall's self-host exit (ADR075 § Positioning).
 *
 * This used to be a bare link to GitHub, which cost nothing to click. It now
 * deletes the account — so it is deliberately TWO steps. Most people on this
 * screen are still evaluating bex, and a misclick that destroys the account is
 * a far worse failure than one extra confirmation. The typed phrase is the same
 * one the settings-page deletion demands (`accountDeletionConfirmation`), so
 * this exit is never looser than the deliberate path, and the wording stays
 * consistent wherever an account can be destroyed.
 *
 * On success the browser session is ended and the user is forwarded to the
 * self-hosting guide. A failed deletion keeps the dialog open and says so: the
 * one thing this must never do is forward as though the account were gone.
 */
export function SelfHostExit() {
  const { t } = useTranslations();
  const [open, setOpen] = useState(false);
  const [confirmation, setConfirmation] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [submitting, setSubmitting] = useState(false);
  const submitGuard = useRef(false);
  const [remove, { loading: deleting }] = useMutation(DeleteAccountDocument);

  const matches = confirmation === accountDeletionConfirmation;
  const busy = deleting || submitting;

  function close(next: boolean) {
    if (busy) return; // never drop a destructive request mid-flight
    setOpen(next);
    if (!next) {
      setConfirmation("");
      setError(null);
    }
  }

  async function handleDelete() {
    if (!matches || busy || submitGuard.current) return;
    submitGuard.current = true;
    setSubmitting(true);
    setError(null);
    try {
      await remove({ variables: { confirmation } });
    } catch (cause) {
      // The account still exists. Stay put and say so rather than forwarding to
      // the self-hosting guide as if the exit had completed.
      submitGuard.current = false;
      setSubmitting(false);
      setError(
        graphQLErrorMessage(cause) ?? t("onboarding.paymentSetupSelfHostError"),
      );
      return;
    }

    // Deletion is durable server-side, so local sign-out is best-effort — the
    // same reasoning as the settings-page card: a Kratos hiccup must not turn a
    // successful destructive request into a misleading retry prompt.
    try {
      await endBrowserSession();
    } catch {
      try {
        await clearBrowserAccountState();
      } catch {
        // The full-page transition below drops this page's state anyway.
      }
    }
    window.location.assign(SELF_HOST_URL);
  }

  return (
    <>
      <Button
        type="button"
        variant="link"
        className="h-auto p-0 font-medium text-primary"
        onClick={() => setOpen(true)}
      >
        <Github aria-hidden="true" className="size-4" />
        {t("onboarding.paymentSetupSelfHost")}
      </Button>

      <Dialog open={open} onOpenChange={close}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>
              {t("onboarding.paymentSetupSelfHostConfirmTitle")}
            </DialogTitle>
            <DialogDescription>
              {t("onboarding.paymentSetupSelfHostConfirmBody")}
            </DialogDescription>
          </DialogHeader>

          <SudoCommandField
            id="self-host-exit-confirm"
            promptKey="auth.deleteAccountConfirmLabel"
            phrase={accountDeletionConfirmation}
            value={confirmation}
            onValueChange={setConfirmation}
          />

          {error ? (
            <Alert variant="destructive">
              <AlertTitle>
                {t("onboarding.paymentSetupSelfHostConfirmTitle")}
              </AlertTitle>
              <AlertDescription>{error}</AlertDescription>
            </Alert>
          ) : null}

          <DialogFooter>
            <Button
              type="button"
              variant="outline"
              disabled={busy}
              onClick={() => close(false)}
            >
              {t("onboarding.paymentSetupSelfHostConfirmCancel")}
            </Button>
            <Button
              type="button"
              variant="destructive"
              disabled={!matches || busy}
              onClick={() => void handleDelete()}
            >
              {busy ? <Loader2 className="animate-spin" /> : null}
              {t("onboarding.paymentSetupSelfHostConfirmAction")}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </>
  );
}
