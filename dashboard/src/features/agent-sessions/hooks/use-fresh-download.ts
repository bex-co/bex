import { useState } from "react";
import { useApolloClient } from "@apollo/client/react";
import { AgentAttachmentDocument } from "@/graphql/definitions";

/**
 * Downloads an attachment through a freshly signed URL (ADR095 D7): a listed
 * or rendered URL may have expired since, so every click asks for a new one.
 * `busy` guards against a second query while one is in flight; `failed` says
 * the file could not be reached, and clears on the next attempt.
 */
export function useFreshDownload(id: string, initiallyFailed = false) {
  const client = useApolloClient();
  const [busy, setBusy] = useState(false);
  const [failed, setFailed] = useState(initiallyFailed);

  async function download() {
    if (busy) return;
    setBusy(true);
    setFailed(false);
    try {
      const res = await client.query({
        query: AgentAttachmentDocument,
        variables: { id },
        fetchPolicy: "network-only",
      });
      const url = res.data?.agentAttachment?.downloadUrl;
      if (!url) throw new Error("no download url");
      // The storage origin answers with Content-Disposition: attachment, so
      // this downloads without leaving the page.
      window.location.assign(url);
    } catch {
      setFailed(true);
    } finally {
      setBusy(false);
    }
  }

  return { download, busy, failed };
}
