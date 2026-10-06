import {
  VerificationFlowState,
  type VerificationFlow,
} from "@ory/client-fetch";

/**
 * A verification flow whose "send code" form is pre-filled with `email`
 * (ADR075 D8 revision 2026-10-06, w2/m168). A signed-in unverified session
 * walled onto `/auth/verification` arrives with no flow id, so the page mints
 * a fresh flow in `choose_method` state, whose email field Kratos leaves
 * empty. Pre-filling the session's own address makes the wall a single
 * "send code" click away. The click is still the user's: auto-sending would
 * mail a new code on every bounce. After the send, Kratos's code step has its
 * own resend button.
 *
 * Ory Elements seeds its form defaults from each input node's `value`, so
 * setting it on a copy is enough. Returns the flow unchanged when it is not
 * in `choose_method`, has no empty `email` input, or `email` is missing.
 * Never mutates the input: the same flow object may be held in React state.
 */
export function withPrefilledEmail(
  flow: VerificationFlow,
  email: string | undefined,
): VerificationFlow {
  if (!email || flow.state !== VerificationFlowState.ChooseMethod) return flow;
  let changed = false;
  const nodes = flow.ui.nodes.map((node) => {
    const attrs = node.attributes;
    if (
      attrs.node_type !== "input" ||
      attrs.name !== "email" ||
      (attrs.value !== undefined && attrs.value !== null && attrs.value !== "")
    ) {
      return node;
    }
    changed = true;
    return { ...node, attributes: { ...attrs, value: email } };
  });
  if (!changed) return flow;
  return { ...flow, ui: { ...flow.ui, nodes } };
}
