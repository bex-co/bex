import { parseCurrentUser, type CurrentUser } from "./current-user";

/**
 * Reason a current-user read did not produce an identity. Kept distinct so the
 * footer can tell "offline/unavailable" apart from "no name" and offer retry
 * only where it makes sense.
 */
export type CurrentUserErrorCode = "network" | "unavailable" | "auth";

export class CurrentUserError extends Error {
  constructor(
    readonly code: CurrentUserErrorCode,
    message: string,
  ) {
    super(message);
    this.name = "CurrentUserError";
  }
}

/** Bearer-token access, mirroring the logs feature's credential injection. */
export type CurrentUserCredentials = {
  getAccessToken: () => Promise<string>;
  forceRefresh: () => Promise<unknown>;
};

type FetchLike = typeof fetch;

/**
 * Minimal, bounded REST client for Render's `GET /v1/users` "who am I"
 * endpoint. Injectable credentials/fetch keep it unit-testable; it refreshes
 * once on a 401 (like {@link ../logs/rest-transport}) and never persists or
 * logs the name/email it returns.
 */
export class CurrentUserClient {
  constructor(
    private readonly apiOrigin: string,
    private readonly credentials: CurrentUserCredentials,
    private readonly fetchImpl: FetchLike = fetch,
  ) {}

  async fetch(signal: AbortSignal): Promise<CurrentUser> {
    let response = await this.request(signal);
    if (response.status === 401) {
      await this.credentials.forceRefresh();
      response = await this.request(signal);
    }
    if (response.status === 401 || response.status === 403) {
      throw new CurrentUserError(
        "auth",
        `current user denied (${response.status})`,
      );
    }
    if (!response.ok) {
      throw new CurrentUserError(
        "unavailable",
        `current user unavailable (${response.status})`,
      );
    }
    let body: unknown;
    try {
      body = await response.json();
    } catch {
      throw new CurrentUserError(
        "unavailable",
        "current user response was not JSON",
      );
    }
    try {
      return parseCurrentUser(body);
    } catch {
      throw new CurrentUserError(
        "unavailable",
        "current user response was malformed",
      );
    }
  }

  private async request(signal: AbortSignal): Promise<Response> {
    const accessToken = await this.credentials.getAccessToken();
    try {
      return await this.fetchImpl(`${this.apiOrigin}/v1/users`, {
        method: "GET",
        headers: { Authorization: `Bearer ${accessToken}` },
        signal,
      });
    } catch (error) {
      if (signal.aborted) throw error;
      throw new CurrentUserError(
        "network",
        error instanceof Error ? error.message : "current user request failed",
      );
    }
  }
}
