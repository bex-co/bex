import { useEffect, useState } from "react";
import {
  Elements,
  PaymentElement,
  useElements,
  useStripe,
} from "@stripe/react-stripe-js";
import { loadStripe, type Stripe } from "@stripe/stripe-js";
import { CheckCircle2, CreditCard } from "lucide-react";
import { Alert, AlertDescription } from "@/common/components/ui/alert";
import { Button } from "@/common/components/ui/button";
import { useTranslations } from "@/common/hooks/use-translations";
import { getSearchParamOnClient } from "@/common/lib/search-params/client";
import type { WorkspaceCreationAttempt } from "@/features/workspaces/hooks/use-workspace-creation-billing";

const stripeClients = new Map<string, Promise<Stripe | null>>();

function stripeClient(key: string) {
  let client = stripeClients.get(key);
  if (!client) {
    client = loadStripe(key);
    stripeClients.set(key, client);
  }
  return client;
}

interface Props {
  attempt: WorkspaceCreationAttempt | null;
  required: boolean;
  providerAvailable: boolean;
  disabled: boolean;
  confirmed: boolean;
  onAdd: () => void;
  onConfirmed: () => void;
  onError: (message: string) => void;
}

export function CreateWorkspacePaymentPanel(props: Props) {
  const { t } = useTranslations();
  const setupReady = Boolean(
    props.attempt?.clientSecret && props.attempt.publishableKey,
  );
  const stripePromise = props.attempt?.publishableKey
    ? stripeClient(props.attempt.publishableKey)
    : null;

  return (
    <section className="space-y-3" aria-labelledby="workspace-payment-heading">
      <div className="space-y-1">
        <h2 id="workspace-payment-heading" className="text-lg font-semibold">
          {t("workspaces.paymentTitle")}
        </h2>
        <p className="text-muted-foreground text-sm">
          {t("workspaces.paymentDescription")}
        </p>
      </div>

      <div className="rounded-lg border bg-card p-4 sm:p-5">
        {props.confirmed ? (
          <div className="flex items-center gap-3 text-sm" role="status">
            <CheckCircle2 className="size-5 text-emerald-600" />
            <span>{t("workspaces.paymentAdded")}</span>
          </div>
        ) : setupReady && stripePromise && props.attempt ? (
          <Elements
            stripe={stripePromise}
            options={{
              clientSecret: props.attempt.clientSecret,
              appearance: { theme: "stripe" },
            }}
          >
            <WorkspacePaymentElement
              attemptId={props.attempt.id}
              onConfirmed={props.onConfirmed}
              onError={props.onError}
            />
          </Elements>
        ) : props.providerAvailable ? (
          <div className="flex flex-col items-start gap-3 sm:flex-row sm:items-center sm:justify-between">
            <div className="flex items-center gap-3">
              <CreditCard className="text-muted-foreground size-5" />
              <p className="text-sm">
                {props.required
                  ? t("workspaces.paymentRequired")
                  : t("workspaces.paymentOptional")}
              </p>
            </div>
            <Button
              type="button"
              variant="outline"
              onClick={props.onAdd}
              disabled={props.disabled}
            >
              {t("workspaces.paymentAdd")}
            </Button>
          </div>
        ) : (
          <Alert>
            <AlertDescription>
              {t("workspaces.paymentSelfHosted")}
            </AlertDescription>
          </Alert>
        )}
      </div>
    </section>
  );
}

function WorkspacePaymentElement({
  attemptId,
  onConfirmed,
  onError,
}: {
  attemptId: string;
  onConfirmed: () => void;
  onError: (message: string) => void;
}) {
  const { t } = useTranslations();
  const stripe = useStripe();
  const elements = useElements();
  const [complete, setComplete] = useState(false);
  const [busy, setBusy] = useState(false);

  useEffect(() => {
    if (!stripe) return;
    const secret = getSearchParamOnClient("setup_intent_client_secret");
    if (!secret) return;
    void stripe.retrieveSetupIntent(secret).then(({ setupIntent, error }) => {
      if (error) onError(error.message ?? t("workspaces.paymentError"));
      else if (setupIntent?.status === "succeeded") onConfirmed();
    });
  }, [onConfirmed, onError, stripe, t]);

  async function savePaymentMethod() {
    if (!stripe || !elements || !complete) return;
    setBusy(true);
    const returnUrl = new URL(window.location.href);
    returnUrl.search = "";
    returnUrl.searchParams.set("attempt", attemptId);
    const { error } = await stripe.confirmSetup({
      elements,
      confirmParams: { return_url: returnUrl.toString() },
      redirect: "if_required",
    });
    setBusy(false);
    if (error) {
      onError(error.message ?? t("workspaces.paymentError"));
      return;
    }
    onConfirmed();
  }

  return (
    <div className="space-y-4">
      <PaymentElement
        options={{ layout: "tabs" }}
        onChange={(event) => setComplete(event.complete)}
      />
      <div className="flex justify-end">
        <Button
          type="button"
          onClick={() => void savePaymentMethod()}
          disabled={!stripe || !elements || !complete || busy}
          loading={busy}
        >
          {t("workspaces.paymentSave")}
        </Button>
      </div>
    </div>
  );
}
