import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { useWorkspace } from "@/features/workspaces/context/hooks";
import {
  AttachmentUploadError,
  deleteUnboundAttachment,
  uploadAttachment,
  type UploadFailure,
} from "@/features/agent-sessions/lib/attachments";
import type { AttachmentLimitsView } from "@/features/agent-sessions/types";

/** A client-side refusal before any byte is sent. */
export type LimitFailure = { kind: "tooLarge" } | { kind: "tooMany" };

/** One composer chip: a file on its way to, or already in, the store. */
export interface AttachmentChip {
  key: string;
  name: string;
  size: number;
  mediaType: string;
  status: "uploading" | "ready" | "error";
  /** Whole percent, 0–100. */
  progress: number;
  /** The `att-` id once the upload landed. */
  id?: string;
  error?: UploadFailure | LimitFailure;
}

export interface UseAttachmentUploadsResult {
  chips: AttachmentChip[];
  /** Ids to send with the message; only uploads that landed. */
  readyIds: string[];
  uploading: boolean;
  hasErrors: boolean;
  add: (files: File[]) => void;
  remove: (key: string) => void;
  /**
   * Drops the chips a send just bound. Chips added while the send was in
   * flight stay, so their uploads are neither lost nor orphaned.
   */
  clear: (sentIds: string[]) => void;
}

let nextKey = 0;

/**
 * Composer attachment state (ADR095 D10). Each file uploads as soon as it is
 * added so the message can be sent the moment typing ends; limits come from
 * the API's capabilities, never a hardcoded number. `ownerId` is the workspace
 * the files belong to; omitted, the selected one.
 */
export function useAttachmentUploads(
  limits: AttachmentLimitsView | null,
  ownerId?: string | null,
): UseAttachmentUploadsResult {
  const { currentWorkspaceId } = useWorkspace();
  const workspaceId = ownerId || currentWorkspaceId;
  const enabled = Boolean(limits?.enabled);
  const maxBytes = limits?.maxBytes ?? 0;
  const maxPerTurn = limits?.maxPerTurn ?? 0;
  const [chips, setChips] = useState<AttachmentChip[]>([]);
  const controllers = useRef(new Map<string, AbortController>());

  // The latest chips, read by every callback so side effects (starting an
  // upload, deleting an orphan) run once, outside any state updater.
  const chipsRef = useRef<AttachmentChip[]>([]);
  const commit = useCallback((next: AttachmentChip[]) => {
    chipsRef.current = next;
    setChips(next);
  }, []);
  const update = useCallback(
    (key: string, patch: Partial<AttachmentChip>) =>
      commit(
        chipsRef.current.map((chip) =>
          chip.key === key ? { ...chip, ...patch } : chip,
        ),
      ),
    [commit],
  );

  const start = useCallback(
    (key: string, file: File) => {
      const controller = new AbortController();
      controllers.current.set(key, controller);
      uploadAttachment(file, {
        ownerId: workspaceId,
        signal: controller.signal,
        onProgress: (fraction) => {
          // Re-render on whole-percent steps only, not every progress event.
          const progress = Math.floor(fraction * 100);
          const chip = chipsRef.current.find((c) => c.key === key);
          if (chip && chip.progress !== progress) update(key, { progress });
        },
      })
        .then((uploaded) => {
          if (!chipsRef.current.some((c) => c.key === key)) {
            // Removed (or cleared) while the last bytes were in flight: the
            // upload landed anyway, so delete it rather than orphan it.
            void deleteUnboundAttachment(uploaded.id).catch(() => {});
            return;
          }
          update(key, {
            status: "ready",
            progress: 100,
            id: uploaded.id,
            mediaType: uploaded.mediaType,
          });
        })
        .catch((err: unknown) => {
          if (err instanceof DOMException && err.name === "AbortError") return;
          update(key, {
            status: "error",
            error:
              err instanceof AttachmentUploadError
                ? err.failure
                : { kind: "network" },
          });
        })
        .finally(() => controllers.current.delete(key));
    },
    [workspaceId, update],
  );

  const add = useCallback(
    (files: File[]) => {
      if (!enabled || files.length === 0) return;
      const next = [...chipsRef.current];
      const started: [string, File][] = [];
      for (const file of files) {
        const chip: AttachmentChip = {
          key: `att-chip-${nextKey++}`,
          name: file.name || "attachment",
          size: file.size,
          mediaType: file.type,
          status: "uploading",
          progress: 0,
        };
        const live = next.filter((c) => c.status !== "error").length;
        if (file.size > maxBytes) {
          next.push({ ...chip, status: "error", error: { kind: "tooLarge" } });
        } else if (live >= maxPerTurn) {
          next.push({ ...chip, status: "error", error: { kind: "tooMany" } });
        } else {
          next.push(chip);
          started.push([chip.key, file]);
        }
      }
      commit(next);
      for (const [key, file] of started) start(key, file);
    },
    [enabled, maxBytes, maxPerTurn, start, commit],
  );

  const remove = useCallback(
    (key: string) => {
      controllers.current.get(key)?.abort();
      const chip = chipsRef.current.find((c) => c.key === key);
      // An upload that landed but was never sent: delete it now rather than
      // leave it to the 24h unbound sweep.
      if (chip?.status === "ready" && chip.id) {
        void deleteUnboundAttachment(chip.id).catch(() => {});
      }
      commit(chipsRef.current.filter((c) => c.key !== key));
    },
    [commit],
  );

  const clear = useCallback(
    (sentIds: string[]) => {
      const sent = new Set(sentIds);
      commit(chipsRef.current.filter((c) => !(c.id && sent.has(c.id))));
    },
    [commit],
  );

  // Leaving the composer abandons what was not sent: abort in-flight uploads
  // and delete the landed ones (sent ones were already cleared).
  useEffect(() => {
    const live = controllers.current;
    return () => {
      for (const controller of live.values()) controller.abort();
      for (const chip of chipsRef.current) {
        if (chip.status === "ready" && chip.id) {
          void deleteUnboundAttachment(chip.id).catch(() => {});
        }
      }
    };
  }, []);

  return useMemo(
    () => ({
      chips,
      readyIds: chips.flatMap((c) =>
        c.status === "ready" && c.id ? [c.id] : [],
      ),
      uploading: chips.some((c) => c.status === "uploading"),
      hasErrors: chips.some((c) => c.status === "error"),
      add,
      remove,
      clear,
    }),
    [chips, add, remove, clear],
  );
}
