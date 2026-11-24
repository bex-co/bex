import { useCallback, useState } from "react";
import { useIsHydrated } from "@/common/hooks/use-is-hydrated";

/** The sheet a session's page holds open beside the conversation. */
export type SessionPane = "files" | "terminal";

/** Where this browser keeps each session's open sheet. */
export const SESSION_PANES_KEY = "bex-agent-session-panes";
/** Sessions remembered at most; the one left longest ago is forgotten first. */
export const REMEMBERED_SESSIONS = 50;

type Remembered = Record<string, SessionPane>;

function isPane(value: unknown): value is SessionPane {
  return value === "files" || value === "terminal";
}

/** Every remembered session, oldest first. Unreadable storage holds none. */
function readAll(): Remembered {
  try {
    const raw: unknown = JSON.parse(
      window.localStorage.getItem(SESSION_PANES_KEY) ?? "{}",
    );
    if (!raw || typeof raw !== "object" || Array.isArray(raw)) return {};
    const out: Remembered = {};
    for (const [id, pane] of Object.entries(raw)) {
      if (isPane(pane)) out[id] = pane;
    }
    return out;
  } catch {
    return {};
  }
}

export function readSessionPane(sessionId: string): SessionPane | null {
  return readAll()[sessionId] ?? null;
}

/**
 * Remembers the sheet `sessionId`'s page has open, or that it has none. A
 * session moves to the newest end on every write (`ags-…` ids are not array
 * indices, so the object keeps insertion order), and the oldest past
 * `REMEMBERED_SESSIONS` are dropped. Storage that refuses a write keeps
 * nothing: the sheet still opens and closes, and the next visit starts closed.
 */
export function writeSessionPane(
  sessionId: string,
  pane: SessionPane | null,
): void {
  const all = readAll();
  delete all[sessionId];
  if (pane) all[sessionId] = pane;
  const kept = Object.entries(all).slice(-REMEMBERED_SESSIONS);
  try {
    if (kept.length === 0) {
      window.localStorage.removeItem(SESSION_PANES_KEY);
    } else {
      window.localStorage.setItem(
        SESSION_PANES_KEY,
        JSON.stringify(Object.fromEntries(kept)),
      );
    }
  } catch {
    // Storage refused: nothing is remembered.
  }
}

interface PaneState {
  sessionId: string;
  pane: SessionPane | null;
  restored: boolean;
}

/**
 * Which sheet a session's page has open, Files or Terminal, remembered per
 * session in this browser (w10/060), as Devin's workspace pane "remembers per
 * session whether it was open, and a session starts with it closed"
 * (docs.devin.ai release notes, 2026-11-13).
 *
 * `restored` is true while the open sheet is the one this browser remembered
 * from an earlier visit, not one the person opened on this one. The Terminal
 * reads it so that a remembered Terminal never starts a closed sandbox on its
 * own (ADR054 D10).
 *
 * Storage is read once per session per mount, never on the server or the
 * hydrating render, which both have the sheets closed, so the HTML and the
 * first client render agree and a remembered sheet opens straight after. It
 * is never re-read after that: another tab opening or closing this session's
 * Terminal does not open or close this tab's.
 */
export function useSessionPane(sessionId: string) {
  const hydrated = useIsHydrated();
  const [state, setState] = useState<PaneState | null>(null);
  let current = state?.sessionId === sessionId ? state : null;
  if (current === null && hydrated) {
    const pane = readSessionPane(sessionId);
    current = { sessionId, pane, restored: pane !== null };
    setState(current);
  }
  const setPane = useCallback(
    (pane: SessionPane | null) => {
      writeSessionPane(sessionId, pane);
      setState({ sessionId, pane, restored: false });
    },
    [sessionId],
  );
  return {
    pane: current?.pane ?? null,
    restored: current?.restored ?? false,
    setPane,
  };
}
