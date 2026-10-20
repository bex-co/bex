import { describe, expect, it } from "vitest";
import type { Session } from "@ory/client-fetch";
import { Route as SetupPaymentRoute } from "@/routes/setup.payment";

const session = (verified: boolean) =>
  ({
    id: "ses-1",
    identity: {
      traits: { email: "dev@example.com" },
      verifiable_addresses: [{ value: "dev@example.com", verified }],
    },
  }) as unknown as Session;

/** What the route's beforeLoad throws for a session, or undefined. */
function beforeLoadThrows(current: Session | null) {
  const beforeLoad = SetupPaymentRoute.options.beforeLoad as (ctx: {
    context: { session: Session | null; aal2Required?: boolean };
    location: { href: string };
  }) => void;
  try {
    beforeLoad({
      context: { session: current },
      location: { href: "/setup/payment?next=%2Fservices" },
    });
  } catch (thrown) {
    return (thrown as { options?: { href?: string; to?: string } }).options;
  }
  return undefined;
}

// No workspace query runs for an unverified session (w6/079), so the wall
// would load forever: it goes to verification first, as sign-up orders it.
describe("/setup/payment and email verification", () => {
  it("sends an unverified session to the verification wall, keeping the destination", () => {
    expect(beforeLoadThrows(session(false))?.href).toBe(
      "/auth/verification?next=%2Fsetup%2Fpayment%3Fnext%3D%252Fservices",
    );
  });

  it("lets a verified session in", () => {
    expect(beforeLoadThrows(session(true))).toBeUndefined();
  });

  it("still sends a session-less visitor to sign in", () => {
    expect(beforeLoadThrows(null)?.to).toBe("/auth/login");
  });
});
