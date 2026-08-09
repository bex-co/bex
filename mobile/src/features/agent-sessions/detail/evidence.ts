// Pure helpers for the agent-session detail (w11/m6 t004): validate the draft-PR
// link before it can open GitHub Mobile, and decide whether any bounded evidence
// exists to render. Kept render-free for the jest-lite runner.

export interface EvidenceLike {
  commandLog?: readonly string[] | null;
  testOutput?: readonly string[] | null;
  outputTail?: string | null;
  changedFiles?: readonly string[] | null;
  commits?: number | null;
  truncated?: boolean | null;
}

// A link may open externally ONLY when it is an https://github.com/ URL — the
// shared guard for every off-platform GitHub link (composer setup CTA + detail
// draft-PR). Any other host, http, or a javascript: scheme is rejected so a
// compromised/absent readiness/PR field can't redirect the phone off-platform.
export function isGitHubUrl(url: string | null | undefined): boolean {
  if (!url) return false;
  return /^https:\/\/github\.com\/[A-Za-z0-9._\-/]*$/.test(url);
}

// A draft PR link is additionally required to be a canonical pull URL, so the
// detail's "Open in GitHub" only ever targets the exact PR.
export function isGitHubPrUrl(url: string | null | undefined): boolean {
  if (!url) return false;
  return /^https:\/\/github\.com\/[A-Za-z0-9._-]+\/[A-Za-z0-9._-]+\/pull\/\d+$/.test(
    url,
  );
}

// True when the evidence carries at least one non-empty section — used to show
// an honest "no evidence yet" state rather than an empty card.
export function hasEvidence(
  evidence: EvidenceLike | null | undefined,
): boolean {
  if (!evidence) return false;
  return (
    (evidence.commandLog?.length ?? 0) > 0 ||
    (evidence.testOutput?.length ?? 0) > 0 ||
    (evidence.changedFiles?.length ?? 0) > 0 ||
    (evidence.outputTail ?? "").trim() !== ""
  );
}
