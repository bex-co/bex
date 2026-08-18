import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { useTranslations } from "@/common/hooks/use-translations";
import {
  WebhookMutationError,
  webhookErrorMessageKey,
  type WebhookFormField,
} from "@/features/webhooks/lib/errors";
import {
  clearWebhookFormError,
  validateWebhookForm,
  webhookFieldOrder,
  type WebhookFormErrors,
  type WebhookValidationInput,
} from "@/features/webhooks/lib/validation";

export function useWebhookFormFeedback({
  mutationError,
  fallbackMessage,
  clearMutationError,
}: {
  mutationError: Error | null;
  fallbackMessage: string;
  clearMutationError: () => void;
}) {
  const { t } = useTranslations();
  const [errors, setErrors] = useState<WebhookFormErrors>({});
  const nameRef = useRef<HTMLInputElement>(null);
  const urlRef = useRef<HTMLInputElement>(null);
  const eventsRef = useRef<HTMLDivElement>(null);

  const focusField = useCallback((field: WebhookFormField) => {
    if (field === "name") nameRef.current?.focus();
    else if (field === "url") urlRef.current?.focus();
    else eventsRef.current?.focus();
  }, []);

  const mutationFeedback = useMemo(() => {
    if (!mutationError) return null;
    const field =
      mutationError instanceof WebhookMutationError
        ? mutationError.field
        : "form";
    const key = webhookErrorMessageKey(mutationError);
    return { field, message: key ? t(key) : fallbackMessage };
  }, [fallbackMessage, mutationError, t]);

  useEffect(() => {
    if (mutationFeedback) focusField(mutationFeedback.field);
  }, [focusField, mutationFeedback]);

  const displayedErrors = useMemo(
    () =>
      mutationFeedback
        ? { ...errors, [mutationFeedback.field]: mutationFeedback.message }
        : errors,
    [errors, mutationFeedback],
  );

  const validationMessages = useMemo(
    () => ({
      nameRequired: t("webhooks.validation.nameRequired"),
      nameDuplicate: t("webhooks.validation.nameDuplicate"),
      urlRequired: t("webhooks.validation.urlRequired"),
      urlHTTPS: t("webhooks.validation.urlHTTPS"),
      eventsRequired: t("webhooks.validation.eventsRequired"),
    }),
    [t],
  );

  const validate = useCallback(
    (input: WebhookValidationInput) => {
      clearMutationError();
      const nextErrors = validateWebhookForm(input, validationMessages);
      setErrors(nextErrors);
      const first = webhookFieldOrder.find((field) => nextErrors[field]);
      if (first) focusField(first);
      return first == null;
    },
    [clearMutationError, focusField, validationMessages],
  );

  const clearField = useCallback(
    (field: WebhookFormField) => {
      setErrors((current) => clearWebhookFormError(current, field));
      clearMutationError();
    },
    [clearMutationError],
  );

  return {
    errors: displayedErrors,
    nameRef,
    urlRef,
    eventsRef,
    validate,
    clearField,
  };
}
