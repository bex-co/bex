import { describe, expect, it } from "vitest";
import type { ReactNode } from "react";
import { act, renderHook } from "@testing-library/react";
import { ApolloClient, InMemoryCache } from "@apollo/client";
import { ApolloProvider } from "@apollo/client/react";
import { MockLink } from "@apollo/client/testing";
import {
  CreateSavedSearchDocument,
  DeleteSavedSearchDocument,
  DuplicateSavedSearchDocument,
  SavedSearchesDocument,
  SavedSearchType,
  UpdateSavedSearchDocument,
} from "@/graphql/definitions";
import { useSavedSearchMutations } from "../use-saved-search-mutations";

// Writing a project's views (w3/m507) keeps the one SavedSearches list the
// sidebar and the view page read in step: a new or duplicated view joins its
// end, an edit is seen through the normalized cache, a deleted view leaves.

const empty = { filters: [], version: 1 };
const view = (
  id: string,
  name: string,
  type = SavedSearchType.ApplicationLogs,
  filters: Record<string, unknown> = empty,
) => ({ __typename: "SavedSearch" as const, id, name, type, filters });

const LEVEL_ERROR = {
  filters: [{ field: "LEVEL", operator: "IS", values: ["error"] }],
  version: 1,
};

function setup(mocks: MockLink.MockedResponse[]) {
  const client = new ApolloClient({
    cache: new InMemoryCache(),
    link: new MockLink(mocks),
  });
  client.writeQuery({
    query: SavedSearchesDocument,
    variables: { projectId: "prj-1" },
    data: {
      savedSearches: [
        view("svs-app", "Application Logs"),
        view("svs-req", "Request Logs", SavedSearchType.RequestLogs),
      ],
    },
  });
  const wrapper = ({ children }: { children: ReactNode }) => (
    <ApolloProvider client={client}>{children}</ApolloProvider>
  );
  const hook = renderHook(() => useSavedSearchMutations("prj-1"), { wrapper });
  const names = () =>
    client
      .readQuery({
        query: SavedSearchesDocument,
        variables: { projectId: "prj-1" },
      })!
      .savedSearches.map((v) => v.name);
  return { hook, names };
}

describe("useSavedSearchMutations", () => {
  it("adds a created and a duplicated view to the project's list", async () => {
    const { hook, names } = setup([
      {
        request: {
          query: CreateSavedSearchDocument,
          variables: {
            projectId: "prj-1",
            type: SavedSearchType.ApplicationLogs,
            name: "Errors",
            filters: LEVEL_ERROR,
          },
        },
        result: {
          data: {
            createSavedSearch: view(
              "svs-new",
              "Errors",
              undefined,
              LEVEL_ERROR,
            ),
          },
        },
      },
      {
        request: {
          query: DuplicateSavedSearchDocument,
          variables: { id: "svs-new" },
        },
        result: {
          data: {
            duplicateSavedSearch: view(
              "svs-copy",
              "Errors (copy)",
              undefined,
              LEVEL_ERROR,
            ),
          },
        },
      },
    ]);
    let created;
    await act(async () => {
      created = await hook.result.current.create(
        SavedSearchType.ApplicationLogs,
        "Errors",
        LEVEL_ERROR,
      );
    });
    expect(created).toMatchObject({ id: "svs-new", name: "Errors" });
    expect(names()).toEqual(["Application Logs", "Request Logs", "Errors"]);

    await act(async () => {
      await hook.result.current.duplicate("svs-new");
    });
    expect(names()).toEqual([
      "Application Logs",
      "Request Logs",
      "Errors",
      "Errors (copy)",
    ]);
    expect(hook.result.current.busy).toBeNull();
  });

  it("shows an edit through the list, and drops a deleted view from it", async () => {
    const { hook, names } = setup([
      {
        request: {
          query: UpdateSavedSearchDocument,
          variables: { id: "svs-app", name: "All logs", filters: LEVEL_ERROR },
        },
        result: {
          data: {
            updateSavedSearch: view(
              "svs-app",
              "All logs",
              undefined,
              LEVEL_ERROR,
            ),
          },
        },
      },
      {
        request: {
          query: DeleteSavedSearchDocument,
          variables: { id: "svs-app" },
        },
        result: { data: { deleteSavedSearch: "svs-app" } },
      },
    ]);
    await act(async () => {
      await hook.result.current.update("svs-app", "All logs", LEVEL_ERROR);
    });
    expect(names()).toEqual(["All logs", "Request Logs"]);

    let left: { id: string }[] = [];
    await act(async () => {
      left = await hook.result.current.remove("svs-app");
    });
    expect(left.map((v) => v.id)).toEqual(["svs-req"]);
    expect(names()).toEqual(["Request Logs"]);
  });

  it("throws a refusal and leaves the list as it was", async () => {
    const { hook, names } = setup([
      {
        request: {
          query: CreateSavedSearchDocument,
          variables: {
            projectId: "prj-1",
            type: SavedSearchType.RequestLogs,
            name: "Request Logs",
            filters: empty,
          },
        },
        result: {
          errors: [
            {
              message:
                'a view named "Request Logs" already exists in this project',
              extensions: { code: "duplicate_saved_search_name" },
            },
          ],
        },
      },
    ]);
    await act(async () => {
      await expect(
        hook.result.current.create(
          SavedSearchType.RequestLogs,
          "Request Logs",
          empty,
        ),
      ).rejects.toThrow(/already exists/);
    });
    expect(names()).toEqual(["Application Logs", "Request Logs"]);
    expect(hook.result.current.busy).toBeNull();
  });
});
