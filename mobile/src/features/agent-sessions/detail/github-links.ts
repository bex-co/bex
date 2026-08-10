// Off-platform GitHub link guards for the agent-session screens (w11/m6 t004):
// validate a URL before it can leave the app for GitHub Mobile. Kept render-free
// for the jest-lite runner. (The evidence helpers this module also carried were
// removed with the evidence surface in w5/m65.)

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
