/*
 * Copyright 2026.
 * Licensed under the Apache License, Version 2.0.
 */

import { createPublicKey, verify } from "node:crypto";
import type { IncomingMessage } from "node:http";

const grantHeader = "x-bex-driver-grant";
const clockSkewSeconds = 5;

interface GrantClaims {
  ses: string;
  act: string;
  iat: number;
  exp: number;
  jti: string;
}

function decodeBase64URL(value: string): Buffer {
  return Buffer.from(value, "base64url");
}

// DriverGrantVerifier consumes gateway-signed turn grants exactly once. The
// sandbox receives only an Ed25519 public key, so tenant code can inspect the
// verifier and call localhost but cannot authorize a credential-bearing turn.
export class DriverGrantVerifier {
  private readonly publicKey;
  private readonly sessionID: string;
  private readonly consumed = new Map<string, number>();

  constructor(publicKey: string, sessionID: string) {
    if (!publicKey || !sessionID) {
      throw new Error("driver turn grants require a public key and session id");
    }
    this.publicKey = createPublicKey({
      key: { kty: "OKP", crv: "Ed25519", x: publicKey },
      format: "jwk",
    });
    this.sessionID = sessionID;
  }

  consume(
    request: IncomingMessage,
    action: "turn" | "snapshot",
    now = Math.floor(Date.now() / 1000),
  ): boolean {
    const token = request.headers[grantHeader];
    if (typeof token !== "string") return false;
    const [body, signature, extra] = token.split(".");
    if (!body || !signature || extra !== undefined) return false;
    if (
      !verify(
        null,
        Buffer.from(body),
        this.publicKey,
        decodeBase64URL(signature),
      )
    ) {
      return false;
    }
    let claims: GrantClaims;
    try {
      claims = JSON.parse(
        decodeBase64URL(body).toString("utf8"),
      ) as GrantClaims;
    } catch {
      return false;
    }
    if (
      claims.ses !== this.sessionID ||
      claims.act !== action ||
      typeof claims.iat !== "number" ||
      typeof claims.exp !== "number" ||
      typeof claims.jti !== "string" ||
      !claims.jti ||
      claims.exp < now - clockSkewSeconds ||
      claims.iat > now + clockSkewSeconds
    ) {
      return false;
    }
    for (const [nonce, expiry] of this.consumed) {
      if (expiry < now - clockSkewSeconds) this.consumed.delete(nonce);
    }
    if (this.consumed.has(claims.jti)) return false;
    this.consumed.set(claims.jti, claims.exp);
    return true;
  }
}
