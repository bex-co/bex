import { describe, expect, it } from "vitest";
import {
  keepLastReplay,
  partSignature,
} from "@/features/agent-sessions/lib/collapse-doubled-parts";

describe("partSignature", () => {
  it("matches two replays of one part that differ only in per-delivery fields", () => {
    // The SDK re-mints the id and re-stamps arrival/provider timestamps + the
    // streaming state on every replay; identity is the visible content only.
    const replayA = {
      type: "text",
      text: "A network namespace isolates network resources.",
      id: "msg-1-part-1",
      at: "2026-08-22T06:00:00.000Z",
      state: "done",
      providerMetadata: { bex: { at: "2026-08-22T06:00:00.000Z" } },
    };
    const replayB = {
      type: "text",
      text: "A network namespace isolates network resources.",
      id: "msg-9-part-3", // fresh id
      at: "2026-08-22T06:05:12.500Z", // re-stamped arrival time
      state: "streaming",
      providerMetadata: { bex: { at: "2026-08-22T06:05:12.500Z" } },
    };
    expect(partSignature(replayA)).toBe(partSignature(replayB));
  });

  it("still separates genuinely different content", () => {
    const a = { type: "text", text: "PID namespace", id: "1" };
    const b = { type: "text", text: "network namespace", id: "2" };
    expect(partSignature(a)).not.toBe(partSignature(b));
  });

  it("keeps tool input/output as identity (distinct calls stay distinct)", () => {
    const readFoo = { type: "tool", toolName: "read", input: { path: "foo" }, id: "1" };
    const readBar = { type: "tool", toolName: "read", input: { path: "bar" }, id: "2" };
    expect(partSignature(readFoo)).not.toBe(partSignature(readBar));
  });

  it("matches a user prompt across its dispatched → settled status flip", () => {
    // The provisioning replay carries the prompt not-yet-settled; the settle
    // replay carries it settled. Identity is text + turn, not delivery status.
    const dispatched = {
      type: "data-user-prompt",
      data: { text: "hi", turn: 1, settled: false, complete: false, truncated: false },
    };
    const settled = {
      type: "data-user-prompt",
      data: { text: "hi", turn: 1, settled: true, complete: true, truncated: false },
    };
    expect(partSignature(dispatched)).toBe(partSignature(settled));
  });
});

describe("keepLastReplay", () => {
  // Messages carry fresh ids per replay; dedup is by an id-agnostic signature.
  const sig = (m: { turn: string }) => m.turn;

  it("returns a single (un-stacked) replay unchanged", () => {
    const msgs = [{ turn: "a" }, { turn: "b" }, { turn: "c" }];
    expect(keepLastReplay(msgs, sig)).toEqual(msgs);
  });

  it("keeps only the newest of several equal-length stacked replays", () => {
    const msgs = [
      { turn: "a", n: 1 },
      { turn: "b", n: 1 },
      { turn: "a", n: 2 },
      { turn: "b", n: 2 },
      { turn: "a", n: 3 },
      { turn: "b", n: 3 },
    ];
    expect(keepLastReplay(msgs, (m) => m.turn)).toEqual([
      { turn: "a", n: 3 },
      { turn: "b", n: 3 },
    ]);
  });

  it("keeps the newest replay when the last one grew a turn (prefix growth)", () => {
    // replay1=[a,b,c,d], replay2=[a,b,c,d], replay3=[a,b,c,d,e]
    const msgs = ["a,b,c,d", "a,b,c,d", "a,b,c,d,e"].flatMap((r) =>
      r.split(",").map((turn) => ({ turn })),
    );
    expect(keepLastReplay(msgs, (m) => m.turn).map((m) => m.turn)).toEqual([
      "a",
      "b",
      "c",
      "d",
      "e",
    ]);
  });

  it("keeps the live-streaming tail of the newest, still-partial replay", () => {
    // replay1=[a,b,c,d] complete, replay2=[a,b,e-partial] mid-stream.
    const msgs = [
      { turn: "a" },
      { turn: "b" },
      { turn: "c" },
      { turn: "d" },
      { turn: "a" },
      { turn: "b" },
      { turn: "e-partial" },
    ];
    expect(keepLastReplay(msgs, (m) => m.turn).map((m) => m.turn)).toEqual([
      "a",
      "b",
      "e-partial",
    ]);
  });

  it("handles 0 and 1 message", () => {
    expect(keepLastReplay([], sig)).toEqual([]);
    expect(keepLastReplay([{ turn: "a" }], sig)).toEqual([{ turn: "a" }]);
  });

  it("collapses a transcript re-appended within one message's parts (the real shape)", () => {
    // The gateway packs the whole conversation into one message; each in-place
    // re-attach re-appends the transcript to its PARTS with prefix growth (the
    // newest copy has one more turn). keepLastReplay(parts, partSignature) must
    // keep only that newest copy. Parts mirror the live shape: a `user-prompt`
    // data part + an assistant `text` part per turn, with per-delivery
    // timestamps/state that partSignature ignores.
    type TestPart = { type: string } & Record<string, unknown>;
    const uPrompt = (turn: number, text: string): TestPart => ({
      type: "data-user-prompt",
      data: { text, turn, complete: true, settled: true },
    });
    const answer = (text: string, at: string): TestPart => ({
      type: "text",
      text,
      state: "done",
      providerMetadata: { bex: { at, endAt: at } },
    });
    const visible = (p: TestPart): string =>
      typeof p.text === "string"
        ? p.text
        : ((p.data as { text?: string })?.text ?? "");
    const t1 = [uPrompt(1, "Q1"), answer("A1", "2026-08-22T05:00:00.000Z")];
    const t2 = [uPrompt(2, "Q2"), answer("A2", "2026-08-22T05:01:00.000Z")];
    const t3 = [uPrompt(3, "Q3"), answer("A3", "2026-08-22T05:02:00.000Z")];
    // Three replays: [t1,t2] then [t1,t2] then [t1,t2,t3] (newest grew a turn).
    const parts = [...t1, ...t2, ...t1, ...t2, ...t1, ...t2, ...t3];
    const kept = keepLastReplay(parts, partSignature);
    expect(kept.map(visible)).toEqual(["Q1", "A1", "Q2", "A2", "Q3", "A3"]);
  });

  it("folds the first-turn provisioning replay into the settled one (status flip)", () => {
    // The live first turn: an early replay carries the prompt not-yet-settled,
    // then the settle replay carries it settled plus the answer. Without ignoring
    // the status flip these wouldn't match and the prompt would render twice.
    const prompt = (settled: boolean) => ({
      type: "data-user-prompt",
      data: { text: "capital of France?", turn: 1, settled, complete: settled },
    });
    const parts = [
      prompt(false),
      prompt(true),
      { type: "text", text: "Paris.", state: "done" },
    ];
    const kept = keepLastReplay(parts, partSignature);
    expect(kept).toHaveLength(2);
    expect(
      kept.map((p) =>
        (p as { text?: string }).text ??
        ((p as { data?: { text?: string } }).data?.text ?? ""),
      ),
    ).toEqual(["capital of France?", "Paris."]);
  });
});
