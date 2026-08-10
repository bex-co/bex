import { describe, expect, it } from "vitest";
import {
  readComposerDocument,
  repoMentionId,
  sessionMentionId,
} from "@/features/agent-sessions/lib/composer-document";

describe("readComposerDocument", () => {
  it("extracts inline context while preserving prompt paragraphs", () => {
    expect(
      readComposerDocument({
        type: "doc",
        content: [
          {
            type: "paragraph",
            content: [
              { type: "text", text: "Fix this in " },
              {
                type: "mention",
                attrs: { id: repoMentionId("acme/widgets") },
              },
              { type: "text", text: " using " },
              {
                type: "mention",
                attrs: { id: sessionMentionId("as-prior") },
              },
              { type: "text", text: " as context." },
            ],
          },
          {
            type: "paragraph",
            content: [{ type: "text", text: "Keep this paragraph." }],
          },
        ],
      }),
    ).toEqual({
      task: "Fix this in using as context.\nKeep this paragraph.",
      repo: "acme/widgets",
      sessionIds: ["as-prior"],
    });
  });

  it("deduplicates session ids and ignores malformed mention attrs", () => {
    expect(
      readComposerDocument({
        type: "doc",
        content: [
          {
            type: "paragraph",
            content: [
              { type: "mention", attrs: { id: sessionMentionId("as-1") } },
              { type: "mention", attrs: { id: sessionMentionId("as-1") } },
              { type: "mention", attrs: { id: 42 } },
            ],
          },
        ],
      }),
    ).toEqual({ task: "", repo: null, sessionIds: ["as-1"] });
  });
});
