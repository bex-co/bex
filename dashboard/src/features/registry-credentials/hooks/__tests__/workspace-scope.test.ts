import { act, renderHook } from "@testing-library/react";
import { beforeEach, expect, it, vi } from "vitest";
import { print } from "graphql";
import {
  RegistryCredentialDocument,
  RegistryCredentialsDocument,
  CreateRegistryCredentialDocument,
  UpdateRegistryCredentialDocument,
  DeleteRegistryCredentialDocument,
} from "@/graphql/definitions";
import { useRegistryCredential } from "../use-registry-credential";
import { useRegistryCredentials } from "../use-registry-credentials";
import { useCreateRegistryCredential } from "../use-create-registry-credential";
import { useUpdateRegistryCredential } from "../use-update-registry-credential";
import { useDeleteRegistryCredential } from "../use-delete-registry-credential";

const query = vi.fn();
const create = vi.fn();
const update = vi.fn();
const remove = vi.fn();
let workspaceId: string | null = "tea-b";
vi.mock("@/features/workspaces/context/hooks", () => ({
  useWorkspace: () => ({ currentWorkspaceId: workspaceId }),
}));
vi.mock("@apollo/client/react", () => ({
  useQuery: (...args: unknown[]) => query(...args),
  useMutation: (document: unknown) => [
    document === CreateRegistryCredentialDocument
      ? create
      : document === UpdateRegistryCredentialDocument
        ? update
        : remove,
  ],
}));

beforeEach(() => {
  workspaceId = "tea-b";
  query.mockReset();
  query.mockReturnValue({
    data: undefined,
    loading: false,
    error: undefined,
    refetch: vi.fn(),
  });
  for (const mutation of [create, update, remove]) {
    mutation.mockReset();
    mutation.mockResolvedValue({ data: {} });
  }
});

it("uses the selected workspace for credential reads and writes, including after a switch", async () => {
  const { result, rerender } = renderHook(() => ({
    detail: useRegistryCredential("rgc-b"),
    list: useRegistryCredentials(),
    create: useCreateRegistryCredential(),
    update: useUpdateRegistryCredential(),
    remove: useDeleteRegistryCredential(),
  }));
  for (const ownerId of ["tea-b", "tea-c"]) {
    workspaceId = ownerId;
    rerender();
    expect(query).toHaveBeenCalledWith(
      RegistryCredentialDocument,
      expect.objectContaining({ variables: { id: "rgc-b", ownerId } }),
    );
    expect(query).toHaveBeenCalledWith(
      RegistryCredentialsDocument,
      expect.objectContaining({ variables: { ownerId } }),
    );
    await act(async () => {
      await result.current.create.create({
        host: "ghcr.io",
        username: "tester",
        authToken: "write-only",
      });
      await result.current.update.update({ id: "rgc-b", name: "renamed" });
      await result.current.remove.remove("rgc-b", "renamed");
    });
    for (const mutation of [create, update, remove]) {
      expect(mutation).toHaveBeenLastCalledWith({
        variables: expect.objectContaining({ ownerId }),
      });
    }
  }
});

it("carries ownerId from operation variables into every registry field on the wire", () => {
  for (const document of [
    RegistryCredentialDocument,
    RegistryCredentialsDocument,
    CreateRegistryCredentialDocument,
    UpdateRegistryCredentialDocument,
    DeleteRegistryCredentialDocument,
  ]) {
    const operation = print(document);
    expect(operation).toContain("$ownerId: String");
    expect(operation).toContain("ownerId: $ownerId");
  }
});

it("waits for workspace selection before reading credentials", () => {
  workspaceId = null;
  const { result, rerender } = renderHook(() => ({
    detail: useRegistryCredential("rgc-b"),
    list: useRegistryCredentials(),
  }));
  expect(result.current.detail.loading).toBe(true);
  expect(result.current.list.loading).toBe(true);
  for (const document of [
    RegistryCredentialDocument,
    RegistryCredentialsDocument,
  ]) {
    expect(query).toHaveBeenCalledWith(
      document,
      expect.objectContaining({ skip: true }),
    );
  }
  workspaceId = "tea-b";
  rerender();
  expect(result.current.detail.loading).toBe(false);
  expect(result.current.list.loading).toBe(false);
  for (const document of [
    RegistryCredentialDocument,
    RegistryCredentialsDocument,
  ]) {
    expect(query).toHaveBeenCalledWith(
      document,
      expect.objectContaining({
        skip: false,
        variables: expect.objectContaining({ ownerId: "tea-b" }),
      }),
    );
  }
});
