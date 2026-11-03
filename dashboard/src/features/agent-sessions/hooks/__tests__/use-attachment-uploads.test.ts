import { beforeEach, describe, expect, it, vi } from "vitest";
import { act, renderHook, waitFor } from "@testing-library/react";
import { useAttachmentUploads } from "@/features/agent-sessions/hooks/use-attachment-uploads";
import { AttachmentUploadError } from "@/features/agent-sessions/lib/attachments";

vi.mock("@/features/workspaces/context/hooks", () => ({
  useWorkspace: () => ({ currentWorkspaceId: "tea-1" }),
}));

type Pending = {
  resolve: (v: unknown) => void;
  reject: (e: unknown) => void;
  ownerId: string | null;
  signal?: AbortSignal;
  onProgress?: (n: number) => void;
};
const pending: Pending[] = [];
const deleteUnbound = vi.fn();
// A real XHR that already received its 201 ignores abort(); this models it.
let abortRejects = true;
vi.mock("@/features/agent-sessions/lib/attachments", async (orig) => ({
  ...(await orig<typeof import("@/features/agent-sessions/lib/attachments")>()),
  uploadAttachment: (
    _file: File,
    opts: {
      ownerId: string | null;
      signal?: AbortSignal;
      onProgress?: (n: number) => void;
    },
  ) =>
    new Promise((resolve, reject) => {
      pending.push({ resolve, reject, ...opts });
      opts.signal?.addEventListener("abort", () => {
        if (abortRejects) {
          reject(new DOMException("upload canceled", "AbortError"));
        }
      });
    }),
  deleteUnboundAttachment: (id: string) => {
    deleteUnbound(id);
    return Promise.resolve();
  },
}));

const limits = { enabled: true, maxBytes: 10, maxPerTurn: 2 };
const file = (name: string, size = 4) =>
  new File(["x".repeat(size)], name, { type: "text/plain" });
const landed = (id: string) => ({
  id,
  name: id,
  mediaType: "text/plain",
  size: 4,
});

beforeEach(() => {
  abortRejects = true;
  pending.length = 0;
  deleteUnbound.mockReset();
});

describe("useAttachmentUploads", () => {
  it("uploads immediately and exposes only landed ids", async () => {
    const { result } = renderHook(() => useAttachmentUploads(limits));
    act(() => result.current.add([file("a.log")]));
    expect(result.current.uploading).toBe(true);
    expect(result.current.readyIds).toEqual([]);
    expect(pending[0].ownerId).toBe("tea-1");

    act(() => pending[0].onProgress?.(0.504));
    expect(result.current.chips[0].progress).toBe(50);

    await act(async () => pending[0].resolve(landed("att-1")));
    expect(result.current.uploading).toBe(false);
    expect(result.current.readyIds).toEqual(["att-1"]);
  });

  it("re-renders on whole-percent progress steps only", () => {
    const { result } = renderHook(() => useAttachmentUploads(limits));
    act(() => result.current.add([file("a.log")]));
    act(() => pending[0].onProgress?.(0.501));
    const before = result.current;
    act(() => pending[0].onProgress?.(0.509));
    expect(result.current).toBe(before);
  });

  it("uploads into the named workspace when one is given", () => {
    const { result } = renderHook(() => useAttachmentUploads(limits, "tea-9"));
    act(() => result.current.add([file("a.log")]));
    expect(pending[0].ownerId).toBe("tea-9");
  });

  it("refuses files over the API's limits without uploading them", () => {
    const { result } = renderHook(() => useAttachmentUploads(limits));
    act(() =>
      result.current.add([
        file("big.bin", 11),
        file("a"),
        file("b"),
        file("c"),
      ]),
    );
    expect(pending).toHaveLength(2);
    expect(result.current.chips.map((c) => c.error?.kind)).toEqual([
      "tooLarge",
      undefined,
      undefined,
      "tooMany",
    ]);
    expect(result.current.hasErrors).toBe(true);
  });

  it("keeps a typed failure, the server's words verbatim", async () => {
    const { result } = renderHook(() => useAttachmentUploads(limits));
    act(() => result.current.add([file("a.log"), file("b.log")]));
    await act(async () => {
      pending[0].reject(
        new AttachmentUploadError({
          kind: "server",
          message: "workspace attachment quota reached",
        }),
      );
      pending[1].reject(new TypeError("boom"));
    });
    expect(result.current.chips.map((c) => c.error)).toEqual([
      { kind: "server", message: "workspace attachment quota reached" },
      { kind: "network" },
    ]);
  });

  it("removing deletes a landed upload and aborts an in-flight one", async () => {
    const { result } = renderHook(() => useAttachmentUploads(limits));
    act(() => result.current.add([file("a"), file("b")]));
    await act(async () => pending[0].resolve(landed("att-1")));
    const [ready, inFlight] = result.current.chips;

    act(() => result.current.remove(ready.key));
    expect(deleteUnbound).toHaveBeenCalledWith("att-1");

    act(() => result.current.remove(inFlight.key));
    expect(pending[1].signal?.aborted).toBe(true);
    await waitFor(() => expect(result.current.chips).toHaveLength(0));
    expect(deleteUnbound).toHaveBeenCalledTimes(1);
  });

  it("deletes an upload that lands after its chip was removed", async () => {
    abortRejects = false;
    const { result } = renderHook(() => useAttachmentUploads(limits));
    act(() => result.current.add([file("a")]));
    act(() => result.current.remove(result.current.chips[0].key));
    // The 201 was already on its way: the abort could not stop it.
    await act(async () => pending[0].resolve(landed("att-late")));
    expect(deleteUnbound).toHaveBeenCalledWith("att-late");
    expect(result.current.chips).toEqual([]);
  });

  it("clear(sent) keeps chips added while the send was in flight", async () => {
    const { result } = renderHook(() => useAttachmentUploads(limits));
    act(() => result.current.add([file("a")]));
    await act(async () => pending[0].resolve(landed("att-1")));
    const sent = result.current.readyIds;
    act(() => result.current.add([file("b")]));
    act(() => result.current.clear(sent));
    expect(result.current.chips.map((c) => c.name)).toEqual(["b"]);
    expect(pending[1].signal?.aborted).toBe(false);
    expect(deleteUnbound).not.toHaveBeenCalled();
  });

  it("unmounting deletes landed-but-unsent uploads and aborts the rest", async () => {
    const { result, unmount } = renderHook(() => useAttachmentUploads(limits));
    act(() => result.current.add([file("a"), file("b")]));
    await act(async () => pending[0].resolve(landed("att-1")));
    unmount();
    expect(deleteUnbound).toHaveBeenCalledWith("att-1");
    expect(pending[1].signal?.aborted).toBe(true);
  });

  it("does nothing while attachments are disabled", () => {
    const { result } = renderHook(() =>
      useAttachmentUploads({ ...limits, enabled: false }),
    );
    act(() => result.current.add([file("a")]));
    expect(result.current.chips).toEqual([]);
    expect(pending).toHaveLength(0);
  });
});
