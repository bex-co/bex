import { describe, expect, it } from "vitest";
import {
  VerificationFlowState,
  type VerificationFlow,
} from "@ory/client-fetch";
import { withPrefilledEmail } from "../verification-prefill";

function flow(
  state: string,
  emailValue: string | undefined = undefined,
): VerificationFlow {
  return {
    id: "flow-1",
    state,
    ui: {
      action: "https://auth.example/self-service/verification?flow=flow-1",
      method: "POST",
      nodes: [
        {
          type: "input",
          group: "default",
          attributes: {
            node_type: "input",
            name: "csrf_token",
            type: "hidden",
            value: "csrf",
          },
          messages: [],
          meta: {},
        },
        {
          type: "input",
          group: "code",
          attributes: {
            node_type: "input",
            name: "email",
            type: "email",
            ...(emailValue === undefined ? {} : { value: emailValue }),
          },
          messages: [],
          meta: {},
        },
        {
          type: "input",
          group: "code",
          attributes: {
            node_type: "input",
            name: "method",
            type: "submit",
            value: "code",
          },
          messages: [],
          meta: {},
        },
      ],
    },
  } as unknown as VerificationFlow;
}

const emailValue = (f: VerificationFlow) =>
  (
    f.ui.nodes.find(
      (n) =>
        n.attributes.node_type === "input" && n.attributes.name === "email",
    )?.attributes as { value?: unknown }
  ).value;

describe("withPrefilledEmail (ADR075 D8 wall)", () => {
  it("pre-fills the empty email input of a choose_method flow", () => {
    const original = flow(VerificationFlowState.ChooseMethod);
    const filled = withPrefilledEmail(original, "dev@example.com");
    expect(emailValue(filled)).toBe("dev@example.com");
    // Never mutates the flow held in React state.
    expect(emailValue(original)).toBeUndefined();
    // Other nodes are untouched.
    expect(filled.ui.nodes[0]).toBe(original.ui.nodes[0]);
    expect(filled.ui.nodes[2]).toBe(original.ui.nodes[2]);
  });

  it("leaves a sent_email flow alone (the code step owns resend)", () => {
    const sent = flow(VerificationFlowState.SentEmail);
    expect(withPrefilledEmail(sent, "dev@example.com")).toBe(sent);
  });

  it("never overwrites an address Kratos already filled in", () => {
    const typed = flow(VerificationFlowState.ChooseMethod, "other@example.com");
    expect(withPrefilledEmail(typed, "dev@example.com")).toBe(typed);
  });

  it("is a no-op without an email", () => {
    const original = flow(VerificationFlowState.ChooseMethod);
    expect(withPrefilledEmail(original, undefined)).toBe(original);
  });
});
