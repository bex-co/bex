import { useCallback, useEffect, useRef, useState } from "react";
import { useApolloClient } from "@apollo/client/react";
import {
  IsDeployableImageDocument,
  type IsDeployableImageQuery,
} from "@/graphql/definitions";
import { UndeployableImageFailureCode } from "@/graphql/schema-types";
import { dockerImageProblem } from "@/common/lib/utils/docker-image";

/** How long typing must pause before the image is checked (Render's 800 ms). */
export const IMAGE_CHECK_DEBOUNCE_MS = 800;

/**
 * - `idle`: nothing to check (an empty field, or a reference the field's own
 *   syntax check refuses).
 * - `checking`: a check is waiting for typing to pause, or is in flight.
 * - `deployable`, `failed`: the registry's answer for the field's value.
 */
export type ImageCheckStatus = "idle" | "checking" | "deployable" | "failed";

export interface ImageCheck {
  status: ImageCheckStatus;
  /** Render's code for a failure; null for a failure none names. */
  failureCode: UndeployableImageFailureCode | null;
  /** Whether a credential is chosen: Render words some answers for it. */
  withCredential: boolean;
  /** Call from the field's paste handler: the pasted value checks at once. */
  notePaste: () => void;
}

type Key = { image: string; credentialId: string };

type Answer = {
  key: Key;
  deployable: boolean;
  failureCode: UndeployableImageFailureCode | null;
};

const sameKey = (a: Key | null, b: Key | null) =>
  a !== null &&
  b !== null &&
  a.image === b.image &&
  a.credentialId === b.credentialId;

const KNOWN_CODES = new Set<string>(
  Object.values(UndeployableImageFailureCode),
);

function failureCodeOf(
  raw: IsDeployableImageQuery["isDeployableImage"]["failureCode"],
): UndeployableImageFailureCode | null {
  return raw && KNOWN_CODES.has(raw) ? raw : null;
}

/**
 * Render's isDeployableImage before an image-backed service is created or
 * switched to (w1/m673; docs/render-artifacts/new-service-wizard.md § The
 * image check). Typing checks the image 800 ms after it pauses; a paste, a
 * credential change, and the value the field opens with check at once. An
 * answer for a value or credential that has since changed is dropped.
 */
export function useImageCheck({
  image,
  credentialId,
  enabled,
}: {
  image: string;
  credentialId: string;
  enabled: boolean;
}): ImageCheck {
  const client = useApolloClient();
  const value = image.trim();
  const checkable =
    enabled && value !== "" && dockerImageProblem(value) === null;
  const currentImage = checkable ? value : null;

  const [answer, setAnswer] = useState<Answer | null>(null);
  // The key of the last check sent: an answer for any other is stale.
  const lastRun = useRef<Key | null>(null);
  const pasted = useRef(false);
  // Whether the value the field opened with has been seen. That value checks
  // at once, as Render's Update Image dialog checks the current image; every
  // later edit waits for typing to pause.
  const opened = useRef(false);

  const run = useCallback(
    (key: Key) => {
      lastRun.current = key;
      void client
        .query({
          query: IsDeployableImageDocument,
          variables: {
            externalImageReference: {
              imageUrl: key.image,
              registryCredentialId: key.credentialId || null,
            },
          },
          fetchPolicy: "network-only",
        })
        .then(
          ({ data }) => {
            if (!sameKey(lastRun.current, key)) return;
            const result = data?.isDeployableImage;
            setAnswer({
              key,
              deployable: result?.deployable === true,
              failureCode: failureCodeOf(result?.failureCode ?? null),
            });
          },
          () => {
            if (!sameKey(lastRun.current, key)) return;
            setAnswer({ key, deployable: false, failureCode: null });
          },
        );
    },
    [client],
  );

  useEffect(() => {
    if (!enabled) {
      opened.current = false;
      return;
    }
    const opening = !opened.current;
    opened.current = true;
    const wasPasted = pasted.current;
    pasted.current = false;
    if (currentImage === null) return;
    const key = { image: currentImage, credentialId };
    // The opening value, a paste, and a credential chosen for the image
    // already checked go at once; a typed value waits for the pause.
    if (opening || wasPasted || lastRun.current?.image === currentImage) {
      run(key);
      return;
    }
    const timer = setTimeout(() => run(key), IMAGE_CHECK_DEBOUNCE_MS);
    return () => clearTimeout(timer);
  }, [currentImage, credentialId, enabled, run]);

  const notePaste = useCallback(() => {
    pasted.current = true;
  }, []);

  const current: Key | null =
    currentImage === null ? null : { image: currentImage, credentialId };
  const status: ImageCheckStatus =
    current === null
      ? "idle"
      : answer !== null && sameKey(answer.key, current)
        ? answer.deployable
          ? "deployable"
          : "failed"
        : "checking";
  return {
    status,
    failureCode:
      status === "failed" && answer !== null ? answer.failureCode : null,
    withCredential: credentialId !== "",
    notePaste,
  };
}
