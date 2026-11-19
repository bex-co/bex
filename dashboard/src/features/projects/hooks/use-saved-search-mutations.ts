import { useCallback } from "react";
import type { ApolloCache } from "@apollo/client";
import { useMutation } from "@apollo/client/react";
import {
  CreateSavedSearchDocument,
  DeleteSavedSearchDocument,
  DuplicateSavedSearchDocument,
  SavedSearchesDocument,
  UpdateSavedSearchDocument,
  type SavedSearchType,
} from "@/graphql/definitions";
import type { SavedSearch } from "./use-saved-searches";

export type SavedSearchBusy = "create" | "update" | "duplicate" | "delete";

export interface SavedSearchMutations {
  /** New View's Done: the created view. Throws the refusal. */
  create: (
    type: SavedSearchType,
    name: string,
    filters: Record<string, unknown>,
  ) => Promise<SavedSearch>;
  /** Edit view's Done: the saved view. Throws the refusal. */
  update: (
    id: string,
    name: string,
    filters: Record<string, unknown>,
  ) => Promise<SavedSearch>;
  /** The header's Duplicate: the copy. Throws the refusal. */
  duplicate: (id: string) => Promise<SavedSearch>;
  /**
   * The header's trash button: the views left. Throws the refusal.
   * `onDeleted` runs once bex-api has deleted the view, before it leaves the
   * list, so a page showing it can step aside first.
   */
  remove: (id: string, onDeleted?: () => void) => Promise<SavedSearch[]>;
  busy: SavedSearchBusy | null;
}

/**
 * Writes a project's views (w3/m507) and keeps the one list the sidebar and
 * the view page read (`SavedSearches(projectId)`) in step with each write: a
 * created or duplicated view joins its end, as bex-api lists views oldest
 * first, and a deleted one leaves it. An edit needs no list change; the cache
 * normalizes the returned view by id, so both readers see it at once.
 */
export function useSavedSearchMutations(
  projectId: string,
): SavedSearchMutations {
  const list = { query: SavedSearchesDocument, variables: { projectId } };
  const append = (cache: ApolloCache, view: SavedSearch | null | undefined) => {
    if (!view) return;
    cache.updateQuery(list, (data) =>
      data && !data.savedSearches.some((v) => v.id === view.id)
        ? { ...data, savedSearches: [...data.savedSearches, view] }
        : data,
    );
  };
  const [createMutation, created] = useMutation(CreateSavedSearchDocument, {
    update: (cache, { data }) => append(cache, data?.createSavedSearch),
  });
  const [updateMutation, updated] = useMutation(UpdateSavedSearchDocument);
  const [duplicateMutation, duplicated] = useMutation(
    DuplicateSavedSearchDocument,
    { update: (cache, { data }) => append(cache, data?.duplicateSavedSearch) },
  );
  const [deleteMutation, deleted] = useMutation(DeleteSavedSearchDocument);

  const create = useCallback(
    async (
      type: SavedSearchType,
      name: string,
      filters: Record<string, unknown>,
    ) => {
      const res = await createMutation({
        variables: { projectId, type, name, filters },
      });
      return returned(res.data?.createSavedSearch, "createSavedSearch");
    },
    [createMutation, projectId],
  );

  const update = useCallback(
    async (id: string, name: string, filters: Record<string, unknown>) => {
      const res = await updateMutation({ variables: { id, name, filters } });
      return returned(res.data?.updateSavedSearch, "updateSavedSearch");
    },
    [updateMutation],
  );

  const duplicate = useCallback(
    async (id: string) => {
      const res = await duplicateMutation({ variables: { id } });
      return returned(res.data?.duplicateSavedSearch, "duplicateSavedSearch");
    },
    [duplicateMutation],
  );

  const remove = useCallback(
    async (id: string, onDeleted?: () => void) => {
      let left: SavedSearch[] = [];
      await deleteMutation({
        variables: { id },
        update: (cache) => {
          onDeleted?.();
          cache.updateQuery(
            { query: SavedSearchesDocument, variables: { projectId } },
            (data) =>
              data && {
                ...data,
                savedSearches: (left = data.savedSearches.filter(
                  (view) => view.id !== id,
                )),
              },
          );
          cache.evict({
            id: cache.identify({ __typename: "SavedSearch", id }),
          });
        },
      });
      return left;
    },
    [deleteMutation, projectId],
  );

  const busy: SavedSearchBusy | null = created.loading
    ? "create"
    : updated.loading
      ? "update"
      : duplicated.loading
        ? "duplicate"
        : deleted.loading
          ? "delete"
          : null;

  return { create, update, duplicate, remove, busy };
}

/** A mutation's view, or a throw: a refusal already threw. */
function returned(
  view: SavedSearch | null | undefined,
  field: string,
): SavedSearch {
  if (!view) throw new Error(`${field} returned no view`);
  return view;
}
