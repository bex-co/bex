// A replay GET re-streams the whole durable transcript into the agent message.
// It happens more than once per session — a React dev double-mount does it twice
// (w3/m44), and in prod every in-place re-attach (`attachSignal` change) fires
// another `resumeStream`. Each replay re-mints FRESH ids, so `useChat` appends
// the copy rather than merging it: the message's `parts` become the transcript
// stacked K times, with prefix growth (the newest copy is the most complete).
// `keepLastReplay` folds that back to the newest copy, comparing content
// id-agnostically via `partSignature`.

type Part = { type: string } & Record<string, unknown>;

// Fields a replay does NOT copy verbatim, so a content signature must ignore
// them or two replays of one part would differ: the fresh `id`, the
// arrival/provider timestamps + streaming `state` the SDK stamps per delivery,
// and a user-prompt's delivery-status flags (`settled`/`complete`/`truncated`/
// `reason`), which flip as a turn goes dispatched → settled (so the provisioning
// replay and the settled replay of the SAME prompt otherwise wouldn't match).
// Everything left — type, text, turn, tool name, tool input/output — is verbatim
// from the durable transcript, so genuinely different parts still differ while
// exact re-replays match.
const NON_IDENTITY_FIELDS = new Set([
  "id",
  "at",
  "endAt",
  "createdAt",
  "timestamp",
  "state",
  "providerMetadata",
  "callProviderMetadata",
  "resultProviderMetadata",
  "settled",
  "complete",
  "truncated",
  "reason",
]);

// An ISO-8601 timestamp value (e.g. a replay arrival time) — dropped wherever it
// appears, keyed or not, so an unnamed timestamp field can't desync a signature.
const ISO_TIMESTAMP = /^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(\.\d+)?(Z|[+-]\d{2}:\d{2})?$/;

// Strip the per-delivery fields recursively (they can nest inside `metadata`/
// `data`) — none of that is identity, at any depth.
function stripNonIdentity(value: unknown): unknown {
  if (Array.isArray(value)) return value.map(stripNonIdentity);
  if (value && typeof value === "object") {
    const out: Record<string, unknown> = {};
    for (const [key, v] of Object.entries(value as Record<string, unknown>)) {
      if (NON_IDENTITY_FIELDS.has(key)) continue;
      if (typeof v === "string" && ISO_TIMESTAMP.test(v)) continue;
      out[key] = stripNonIdentity(v);
    }
    return out;
  }
  return value;
}

// Memoized by part identity. Settled parts keep their object across streamed
// tokens (only the growing tail is a new object), so the recursive stringify is
// paid once per part rather than over the whole transcript on every token —
// without this, the streaming render is O(transcript²) per turn.
const signatureCache = new WeakMap<object, string>();

/**
 * A part's content signature: its identity-bearing content with the per-delivery
 * fields removed at every depth. Two replays of one part share a signature; two
 * genuinely different parts (distinct same-name tool calls, or a line the agent
 * really did repeat) do not, so real content is never folded away.
 */
export function partSignature(part: Part): string {
  const cached = signatureCache.get(part);
  if (cached !== undefined) return cached;
  const sig = JSON.stringify(stripNonIdentity(part));
  signatureCache.set(part, sig);
  return sig;
}

/**
 * Drop stacked replays, keeping only the newest copy. Each replay restarts from
 * the transcript's first item, so the LAST item whose signature matches the
 * first marks the newest replay's start; slice from there. Replays only grow, so
 * the newest is the most complete and carries the live-streaming tail. Returned
 * as-is when nothing is stacked (the common single-replay case). `signatureOf`
 * is injected so the caller can supply the memoized `partSignature`.
 */
export function keepLastReplay<M>(
  items: readonly M[],
  signatureOf: (item: M) => string,
): readonly M[] {
  if (items.length < 2) return items;
  const first = signatureOf(items[0]);
  let start = 0;
  for (let i = 1; i < items.length; i++) {
    if (signatureOf(items[i]) === first) start = i;
  }
  return start === 0 ? items : items.slice(start);
}
