import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { act, renderHook } from "@testing-library/react";
import { memoryStorage } from "@/test/mocks/storage";
import {
  REMEMBERED_SESSIONS,
  SESSION_PANES_KEY,
  readSessionPane,
  useSessionPane,
  writeSessionPane,
} from "@/features/agent-sessions/hooks/use-session-pane";

// w10/060: the session page remembers per session, in this browser, which
// sheet was open, as Devin's workspace pane does.

const A = "ags-d4ae2ab0f8cs73a9ejo0";
const B = "ags-d4ae2ab0f8cs73a9ejo1";

let storage: Storage;
beforeEach(() => {
  storage = memoryStorage();
  vi.stubGlobal("localStorage", storage);
});
afterEach(() => {
  vi.unstubAllGlobals();
});

describe("useSessionPane", () => {
  it("starts a session never opened with both sheets closed", () => {
    const { result } = renderHook(() => useSessionPane(A));
    expect(result.current.pane).toBeNull();
    expect(result.current.restored).toBe(false);
  });

  it("reopens the sheet a session had open on the next visit, as restored", () => {
    const first = renderHook(() => useSessionPane(A));
    act(() => first.result.current.setPane("terminal"));
    // Opened on this visit: not restored.
    expect(first.result.current).toMatchObject({
      pane: "terminal",
      restored: false,
    });
    first.unmount();

    const again = renderHook(() => useSessionPane(A));
    expect(again.result.current).toMatchObject({
      pane: "terminal",
      restored: true,
    });
    // Another session keeps its own.
    expect(renderHook(() => useSessionPane(B)).result.current.pane).toBeNull();
  });

  it("forgets a sheet the person closed", () => {
    writeSessionPane(A, "files");
    const { result, unmount } = renderHook(() => useSessionPane(A));
    expect(result.current).toMatchObject({ pane: "files", restored: true });
    act(() => result.current.setPane(null));
    expect(result.current).toMatchObject({ pane: null, restored: false });
    unmount();
    expect(renderHook(() => useSessionPane(A)).result.current.pane).toBeNull();
    expect(storage.getItem(SESSION_PANES_KEY)).toBeNull();
  });

  it("follows the session when the page moves to another one", () => {
    writeSessionPane(B, "terminal");
    const { result, rerender } = renderHook(({ id }) => useSessionPane(id), {
      initialProps: { id: A },
    });
    expect(result.current.pane).toBeNull();
    rerender({ id: B });
    expect(result.current).toMatchObject({ pane: "terminal", restored: true });
  });

  it("does not open or close with another tab's choice", () => {
    const { result, rerender } = renderHook(() => useSessionPane(A));
    // Another tab opens this session's Terminal.
    writeSessionPane(A, "terminal");
    rerender();
    expect(result.current.pane).toBeNull();
  });
});

describe("the remembered sessions", () => {
  it("keeps the newest sessions and forgets the one left longest ago", () => {
    const ids = Array.from(
      { length: REMEMBERED_SESSIONS + 1 },
      (_, i) => `ags-${String(i).padStart(20, "0")}`,
    );
    for (const id of ids) writeSessionPane(id, "files");
    expect(readSessionPane(ids[0])).toBeNull();
    expect(readSessionPane(ids[1])).toBe("files");
    expect(readSessionPane(ids[REMEMBERED_SESSIONS])).toBe("files");
    // Writing a session again makes it the newest.
    writeSessionPane(ids[1], "terminal");
    writeSessionPane("ags-newest0000000000000", "files");
    expect(readSessionPane(ids[1])).toBe("terminal");
    expect(readSessionPane(ids[2])).toBeNull();
  });

  it.each([
    ["not JSON", "{"],
    ["a list", '["files"]'],
    ["an unknown sheet", JSON.stringify({ [A]: "desktop" })],
  ])("reads storage holding %s as nothing remembered", (_, raw) => {
    storage.setItem(SESSION_PANES_KEY, raw);
    expect(readSessionPane(A)).toBeNull();
    expect(renderHook(() => useSessionPane(A)).result.current.pane).toBeNull();
  });

  it("still opens and closes the sheet when storage refuses the write", () => {
    vi.stubGlobal("localStorage", {
      ...memoryStorage(),
      setItem: () => {
        throw new DOMException("quota", "QuotaExceededError");
      },
    });
    const { result, unmount } = renderHook(() => useSessionPane(A));
    act(() => result.current.setPane("terminal"));
    expect(result.current.pane).toBe("terminal");
    unmount();
    expect(renderHook(() => useSessionPane(A)).result.current.pane).toBeNull();
  });
});
