import type { JSONContent } from "@tiptap/core";

const REPO_PREFIX = "repo:";
const SESSION_PREFIX = "session:";
const MENTION_MARKER = "\u0000";

export interface ComposerDocument {
  task: string;
  repo: string | null;
  sessionIds: string[];
}

/**
 * Convert the rich composer document back into the existing create-session
 * contract. Mention nodes are context, not prompt prose: the repo goes to the
 * singular `repo` field and sessions become the existing context lines.
 */
export function readComposerDocument(doc: JSONContent): ComposerDocument {
  let repo: string | null = null;
  const sessionIds: string[] = [];

  function readText(node: JSONContent): string {
    if (node.type === "text") return node.text ?? "";
    if (node.type === "mention") {
      const id = typeof node.attrs?.id === "string" ? node.attrs.id : "";
      if (id.startsWith(REPO_PREFIX)) repo = id.slice(REPO_PREFIX.length);
      if (id.startsWith(SESSION_PREFIX)) {
        const sessionId = id.slice(SESSION_PREFIX.length);
        if (sessionId && !sessionIds.includes(sessionId)) {
          sessionIds.push(sessionId);
        }
      }
      return MENTION_MARKER;
    }
    return (node.content ?? []).map(readText).join("");
  }

  const task = (doc.content ?? [])
    .map(readText)
    .join("\n")
    // Mentions commonly have a typed space before them and the editor's
    // inserted space after them. Collapse only that mention-shaped gap, while
    // preserving deliberate whitespace elsewhere in the prompt.
    .replace(/[ \t]*\u0000[ \t]*/g, " ")
    .replace(/[ \t]+\n/g, "\n")
    .trim();

  return { task, repo, sessionIds };
}

export function repoMentionId(repo: string): string {
  return `${REPO_PREFIX}${repo}`;
}

export function sessionMentionId(sessionId: string): string {
  return `${SESSION_PREFIX}${sessionId}`;
}

export function isRepoMentionId(id: unknown): id is string {
  return typeof id === "string" && id.startsWith(REPO_PREFIX);
}
