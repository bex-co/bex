import { isValidGitUrl } from "@/common/lib/utils/git-url";

export interface DeployLink {
  repo: string;
  /** Set only when the link ends in `/tree/<branch>`. */
  branch?: string;
}

const TREE_RE = /^(https:\/\/[^/]+\/[^/]+\/[^/]+?)\/tree\/(.+?)\/?$/;

/**
 * Parse the `repo` of a Deploy-to-Render-style link (`/deploy?repo=<url>`). As
 * on Render, the branch is the repository's default unless the URL ends in
 * `/tree/<branch>`. Returns null when `repo` is not a clone URL bex accepts
 * (w8/163).
 */
export function parseDeployRepo(raw: unknown): DeployLink | null {
  if (typeof raw !== "string") return null;
  const value = raw.trim();
  if (!isValidGitUrl(value)) return null;
  const tree = TREE_RE.exec(value);
  if (tree) return { repo: tree[1], branch: tree[2] };
  return { repo: value.replace(/\/+$/, "") };
}
