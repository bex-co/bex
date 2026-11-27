import { UndeployableImageFailureCode } from "@/graphql/schema-types";
import type { ImageCheck } from "@/features/services/hooks/use-image-check";

/**
 * Where the image field sits. The create form shows Render's helper line and
 * only the failures; Settings' Update source also says why a deployable image
 * is deployable, as Render's Update Image dialog does
 * (docs/render-artifacts/new-service-wizard.md § The image check).
 */
export type ImageCheckMessages = "create" | "update";

export type ImageFieldMessage = {
  text: string;
  tone: "muted" | "success" | "error";
};

const FAILURE_KEYS: Record<UndeployableImageFailureCode, string> = {
  [UndeployableImageFailureCode.DoesNotExist]:
    "services.imageCheckDoesNotExist",
  [UndeployableImageFailureCode.InvalidInputUrl]:
    "services.imageCheckInvalidUrl",
  [UndeployableImageFailureCode.InvalidRegistryCredential]:
    "services.imageCheckInvalidCredential",
  [UndeployableImageFailureCode.UnsupportedPlatform]:
    "services.imageCheckUnsupportedPlatform",
  [UndeployableImageFailureCode.ImageTooLarge]: "services.imageCheckTooLarge",
};

function failureKey(check: ImageCheck): string {
  if (check.failureCode === null) return "services.imageCheckError";
  if (
    check.failureCode === UndeployableImageFailureCode.DoesNotExist &&
    check.withCredential
  ) {
    return "services.imageCheckDoesNotExistWithCredential";
  }
  return FAILURE_KEYS[check.failureCode];
}

/** The check's sentence for the field's message line, or null for none. */
export function imageCheckMessage(
  t: (key: string) => string,
  check: ImageCheck,
  messages: ImageCheckMessages,
): ImageFieldMessage | null {
  if (check.status === "failed") {
    return { text: t(failureKey(check)), tone: "error" };
  }
  if (messages === "create") {
    return { text: t("services.imageCheckHelper"), tone: "muted" };
  }
  if (check.status === "deployable") {
    return {
      text: t(
        check.withCredential
          ? "services.imageCheckAuthorized"
          : "services.imageCheckPublic",
      ),
      tone: "success",
    };
  }
  return null;
}

/**
 * Every sentence the message line can hold. The line is a grid cell holding
 * them all, invisible, so it is as tall as the longest at the field's width
 * in the current language: checking, an answer, and a different answer never
 * move the rows below.
 */
export function imageMessageSizerKeys(messages: ImageCheckMessages): string[] {
  return [
    ...(messages === "create"
      ? ["services.imageCheckHelper"]
      : ["services.imageCheckPublic", "services.imageCheckAuthorized"]),
    ...Object.values(FAILURE_KEYS),
    "services.imageCheckDoesNotExistWithCredential",
    "services.imageCheckError",
  ];
}

export const MESSAGE_TONE_CLASS: Record<ImageFieldMessage["tone"], string> = {
  muted: "text-muted-foreground",
  success: "text-green-700 dark:text-green-500",
  error: "text-destructive",
};
