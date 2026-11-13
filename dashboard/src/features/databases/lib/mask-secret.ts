/** What a hidden password reads as wherever it appears. */
export const SECRET_MASK = "••••••••";

/** Every occurrence of secret in value, masked; value unchanged without one. */
export function maskSecret(value: string, secret: string): string {
  return secret ? value.split(secret).join(SECRET_MASK) : value;
}

/**
 * The password in a connection URL as it is spelled there (percent-encoded,
 * so maskSecret finds it); "" for a URL with none or one that does not parse.
 */
export function connectionURLPassword(url: string): string {
  try {
    return new URL(url).password;
  } catch {
    return "";
  }
}
