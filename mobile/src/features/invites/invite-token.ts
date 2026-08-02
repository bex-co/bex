export const INVITE_TOKEN_LENGTH = 32;

const inviteTokenPattern = /^[0-9a-f]{32}$/;

/** Accept only the current server-minted 128-bit lowercase hex capability. */
export function parseInviteToken(value: unknown): string | null {
  return typeof value === "string" && inviteTokenPattern.test(value)
    ? value
    : null;
}

export type StoredInvite = {
  version: 1;
  token: string;
  subject: string | null;
};

export function parseStoredInvite(value: unknown): StoredInvite | null {
  if (!value || typeof value !== "object" || Array.isArray(value)) return null;
  const record = value as Record<string, unknown>;
  if (Object.keys(record).sort().join(",") !== "subject,token,version") {
    return null;
  }
  const token = parseInviteToken(record.token);
  const subject = record.subject;
  if (
    record.version !== 1 ||
    !token ||
    !(
      subject === null ||
      (typeof subject === "string" &&
        subject.length > 0 &&
        subject.length <= 256 &&
        !/[\u0000-\u001f\u007f]/.test(subject))
    )
  ) {
    return null;
  }
  return { version: 1, token, subject };
}
