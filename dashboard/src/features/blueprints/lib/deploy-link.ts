import { decodePathPart, isValidGitUrl } from "@/common/lib/utils/git-url";

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

/**
 * The repository a Deploy button's page names, for a link with no `repo`. As on
 * Render, the button deploys "the referrer of the page containing the Deploy
 * to Render button" (w33/003). That page must be a repository's own,
 * `https://<host>/<owner>/<repo>`, or one of its branches, `…/tree/<branch>`;
 * its query (GitHub's `?tab=readme-ov-file`) is dropped. Null for no referrer,
 * an origin alone (what browsers send another site by default, so github.com
 * gives no more than that), a page of this dashboard (`ownOrigin`), and any
 * other page.
 */
export function parseDeployReferrer(
  referrer: string,
  ownOrigin: string | null,
): DeployLink | null {
  let url: URL;
  try {
    url = new URL(referrer);
  } catch {
    return null;
  }
  if (ownOrigin && url.host === hostOf(ownOrigin)) return null;
  const [owner, repo, tree, ...branch] = url.pathname
    .split("/")
    .filter(Boolean);
  if (!owner || !repo) return null;
  const base = `${url.origin}/${owner}/${repo}`;
  if (tree === undefined) return parseDeployRepo(base);
  if (tree !== "tree" || branch.length === 0) return null;
  return parseDeployRepo(
    `${base}/tree/${branch.map(decodePathPart).join("/")}`,
  );
}

function hostOf(origin: string): string | null {
  try {
    return new URL(origin).host;
  } catch {
    return null;
  }
}
