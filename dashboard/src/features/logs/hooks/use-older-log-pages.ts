import { useCallback, useMemo, useState } from "react";
import { useApolloClient } from "@apollo/client/react";
import { ServerError } from "@apollo/client/errors";
import { isUnauthenticatedError } from "@/common/apollo/auth-error-link";
import {
  hasGraphQLErrorCode,
  isForbiddenError,
} from "@/common/lib/graphql-error";
import {
  LogsDocument,
  type LogsQuery,
  type LogsQueryVariables,
} from "@/graphql/definitions";
import { dedupeLogLines, toLogLines } from "../lib/map";
import type { LogLine } from "../types";

type LogEnvelope = LogsQuery["logs"];

export function logReadAccessDenied(error: Error | undefined): boolean {
  return (
    isUnauthenticatedError(error) ||
    isForbiddenError(error) ||
    (ServerError.is(error) && [403, 404].includes(error.statusCode)) ||
    ["UNAUTHENTICATED", "FORBIDDEN", "NOT_FOUND"].some((code) =>
      hasGraphQLErrorCode(error, code),
    ) ||
    /not found|unauthenticated|unauthorized|session expired/i.test(
      error?.message ?? "",
    )
  );
}

export interface OlderLogPages {
  older: LogLine[];
  hasMore: boolean;
  loadingOlder: boolean;
  error: Error | undefined;
  loadOlder: () => void;
}

interface PagingState {
  key: string;
  generation: number;
  first: LogEnvelope | undefined;
  older: LogLine[];
  cursor: { startTime: string; endTime: string } | null;
  hasMore: boolean;
  loadingOlder: boolean;
  pagedBack: boolean;
  error: Error | undefined;
}

function initialPages(
  key: string,
  first: LogEnvelope | undefined,
  generation = 0,
): PagingState {
  return {
    key,
    generation,
    first,
    older: [],
    cursor: first
      ? { startTime: first.nextStartTime, endTime: first.nextEndTime }
      : null,
    hasMore: first?.hasMore ?? false,
    loadingOlder: false,
    pagedBack: false,
    error: undefined,
  };
}

/** Fixed callers reset on query changes; a sliding reader supplies its semantic
 * identity so clock bounds cannot invalidate a cursor or an in-flight page. */
export function useOlderLogPages(
  variables: LogsQueryVariables,
  first: LogEnvelope | undefined,
  options?: { readerKey?: string; blocked?: boolean },
): OlderLogPages {
  const client = useApolloClient();
  const key = JSON.stringify([
    options?.readerKey ?? variables,
    !!options?.blocked,
  ]);
  const head = options?.blocked ? undefined : first;
  const [state, setState] = useState(() => initialPages(key, head));
  if (state.key !== key) {
    setState(initialPages(key, head, state.generation + 1));
  } else if (head && state.first !== head) {
    setState({
      ...state,
      first: head,
      error: undefined,
      ...(!state.pagedBack
        ? {
            cursor: {
              startTime: head.nextStartTime,
              endTime: head.nextEndTime,
            },
            hasMore: head.hasMore,
          }
        : {}),
    });
  }

  // Prune persisted pages too, rather than only hiding expired lines. A slow
  // page completed with older bounds is pruned on its next render.
  const lower = options?.readerKey
    ? Date.parse(variables.startTime ?? "")
    : NaN;
  const retainedOlder = useMemo(() => {
    if (!Number.isFinite(lower)) return state.older;
    const retained = state.older.filter(
      (line) => !(Date.parse(line.timestamp) < lower),
    );
    return retained.length === state.older.length ? state.older : retained;
  }, [state.older, lower]);
  if (retainedOlder !== state.older) {
    setState((previous) =>
      previous.older === state.older
        ? { ...previous, older: retainedOlder }
        : previous,
    );
  }
  const expiredCursor =
    Number.isFinite(lower) &&
    !!state.cursor &&
    Date.parse(state.cursor.endTime) < lower;

  const loadOlder = useCallback(() => {
    if (
      !state.hasMore ||
      state.loadingOlder ||
      !state.cursor ||
      options?.blocked ||
      expiredCursor
    )
      return;
    // Lock the continuation before starting: a head response arriving while
    // this page is pending must not rewind it.
    setState((previous) =>
      previous.key === key && previous.generation === state.generation
        ? { ...previous, loadingOlder: true, pagedBack: true, error: undefined }
        : previous,
    );
    void client
      .query({
        query: LogsDocument,
        variables: {
          ...variables,
          startTime:
            Number.isFinite(lower) && lower > Date.parse(state.cursor.startTime)
              ? variables.startTime
              : state.cursor.startTime,
          endTime: state.cursor.endTime,
        },
        fetchPolicy: "network-only",
        errorPolicy: "all",
      })
      .then((result) => {
        const env = result.data?.logs;
        setState((previous) => {
          if (previous.key !== key || previous.generation !== state.generation)
            return previous;
          if (!env) {
            return {
              ...previous,
              loadingOlder: false,
              error: result.error,
              ...(logReadAccessDenied(result.error)
                ? { older: [], hasMore: false, cursor: null }
                : {}),
            };
          }
          return {
            ...previous,
            older: dedupeLogLines([...toLogLines(env.logs), ...previous.older]),
            loadingOlder: false,
            hasMore: env.hasMore,
            cursor: { startTime: env.nextStartTime, endTime: env.nextEndTime },
            error: result.error,
          };
        });
      })
      .catch((error: Error) => {
        setState((previous) =>
          previous.key === key && previous.generation === state.generation
            ? {
                ...previous,
                loadingOlder: false,
                error,
                ...(logReadAccessDenied(error)
                  ? { older: [], hasMore: false, cursor: null }
                  : {}),
              }
            : previous,
        );
      });
  }, [
    lower,
    expiredCursor,
    state.generation,
    state.hasMore,
    state.loadingOlder,
    state.cursor,
    options?.blocked,
    key,
    client,
    variables,
  ]);

  return {
    older: state.older,
    hasMore: state.hasMore && !expiredCursor,
    loadingOlder: state.loadingOlder,
    error: state.error,
    loadOlder,
  };
}
