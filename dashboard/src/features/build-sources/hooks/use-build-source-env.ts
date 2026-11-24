import { useCallback } from "react";
import { useApolloClient } from "@apollo/client/react";
import {
  BuildSourceEnvVarValueDocument,
  BuildSourceSecretFileContentDocument,
} from "@/graphql/definitions";

/**
 * Fetches a single build-time env var's value on demand ("Show secret") via
 * `buildSource(id) { envVar(key) { value } }`. Network-only so revealed values
 * are always fresh and never cached across views.
 */
export function useRevealBuildSourceEnvVar(buildSourceId: string) {
  const client = useApolloClient();
  return useCallback(
    async (key: string): Promise<string> => {
      const res = await client.query({
        query: BuildSourceEnvVarValueDocument,
        variables: { id: buildSourceId, key },
        fetchPolicy: "network-only",
        errorPolicy: "none",
      });
      return res.data?.buildSource?.envVar?.value ?? "";
    },
    [client, buildSourceId],
  );
}

/**
 * Fetches a single build-time secret file's content on demand ("Show") via
 * `buildSource(id) { secretFile(name) { content } }`.
 */
export function useRevealBuildSourceSecretFile(buildSourceId: string) {
  const client = useApolloClient();
  return useCallback(
    async (name: string): Promise<string> => {
      const res = await client.query({
        query: BuildSourceSecretFileContentDocument,
        variables: { id: buildSourceId, name },
        fetchPolicy: "network-only",
        errorPolicy: "none",
      });
      return res.data?.buildSource?.secretFile?.content ?? "";
    },
    [client, buildSourceId],
  );
}
