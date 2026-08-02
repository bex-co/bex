// Hand-written TypedDocumentNodes for the Blueprints dashboard surface (w7/m27).
// Mirrors the GraphQL operations bex-api registers in apps/graphql.go (w2/m15 +
// w7/m27 + w2/m62). When codegen can run against a live bex-api, migrate imports to
// @/graphql/definitions and delete this file.

import gql from "graphql-tag";
import type { TypedDocumentNode } from "@graphql-typed-document-node/core";
import type {
  BlueprintView,
  BlueprintSyncView,
  BlueprintValidationResult,
  BlueprintPreviewResult,
  SyncBlueprintResult,
} from "@/features/blueprints/types";

// --- queries ---

export interface BlueprintsQueryVars {
  ownerId?: string | null;
}
export interface BlueprintsQuery {
  blueprints: Array<BlueprintView | null> | null;
}
export const BlueprintsDocument = gql`
  query Blueprints($ownerId: String) {
    blueprints(ownerId: $ownerId) {
      id
      name
      repo
      branch
      path
      autoSync
      status
      lastSync
      createdAt
      updatedAt
    }
  }
` as unknown as TypedDocumentNode<BlueprintsQuery, BlueprintsQueryVars>;

export interface BlueprintQueryVars {
  id: string;
  ownerId?: string | null;
}
export interface BlueprintQuery {
  blueprint: BlueprintView | null;
}
export const BlueprintDocument = gql`
  query Blueprint($id: String!, $ownerId: String) {
    blueprint(id: $id, ownerId: $ownerId) {
      id
      name
      repo
      branch
      path
      autoSync
      manifest
      status
      lastSync
      resources {
        id
        name
        type
      }
      createdAt
      updatedAt
    }
  }
` as unknown as TypedDocumentNode<BlueprintQuery, BlueprintQueryVars>;

export interface BlueprintSyncsQueryVars {
  id: string;
  ownerId?: string | null;
  cursor?: string | null;
  limit?: number | null;
}
export interface BlueprintSyncsQuery {
  blueprintSyncs: Array<BlueprintSyncView | null> | null;
}
export const BlueprintSyncsDocument = gql`
  query BlueprintSyncs($id: String!, $ownerId: String, $cursor: String, $limit: Int) {
    blueprintSyncs(id: $id, ownerId: $ownerId, cursor: $cursor, limit: $limit) {
      id
      commitId
      state
      startedAt
      completedAt
    }
  }
` as unknown as TypedDocumentNode<BlueprintSyncsQuery, BlueprintSyncsQueryVars>;

export interface ValidateBlueprintQueryVars {
  bexYaml: string;
}
export interface ValidateBlueprintQuery {
  validateBlueprint: BlueprintValidationResult | null;
}
export const ValidateBlueprintDocument = gql`
  query ValidateBlueprint($bexYaml: String!) {
    validateBlueprint(bexYaml: $bexYaml) {
      valid
      errors
    }
  }
` as unknown as TypedDocumentNode<
  ValidateBlueprintQuery,
  ValidateBlueprintQueryVars
>;

export interface BlueprintPreviewQueryVars {
  repo: string;
  branch: string;
  path?: string | null;
  ownerId?: string | null;
}
export interface BlueprintPreviewQuery {
  blueprintPreview: BlueprintPreviewResult | null;
}
export const BlueprintPreviewDocument = gql`
  query BlueprintPreview(
    $repo: String!
    $branch: String!
    $path: String
    $ownerId: String
  ) {
    blueprintPreview(
      repo: $repo
      branch: $branch
      path: $path
      ownerId: $ownerId
    ) {
      found
      commitId
      error
      validation {
        valid
        errors
        plan {
          services
          databases
          keyValue
          envGroups
          totalActions
        }
      }
    }
  }
` as unknown as TypedDocumentNode<
  BlueprintPreviewQuery,
  BlueprintPreviewQueryVars
>;

// --- mutations ---

export interface CreateBlueprintVars {
  repo: string;
  branch: string;
  path?: string | null;
  name?: string | null;
  confirm?: string | null;
  ownerId?: string | null;
}
export interface CreateBlueprintMutation {
  createBlueprint: BlueprintView | null;
}
export const CreateBlueprintDocument = gql`
  mutation CreateBlueprint(
    $repo: String!
    $branch: String!
    $path: String
    $name: String
    $confirm: String
    $ownerId: String
  ) {
    createBlueprint(
      repo: $repo
      branch: $branch
      path: $path
      name: $name
      confirm: $confirm
      ownerId: $ownerId
    ) {
      id
      name
      repo
      branch
      path
      autoSync
      status
      lastSync
      createdAt
      updatedAt
    }
  }
` as unknown as TypedDocumentNode<CreateBlueprintMutation, CreateBlueprintVars>;

export interface UpdateBlueprintVars {
  id: string;
  name?: string | null;
  autoSync?: boolean | null;
  path?: string | null;
  ownerId?: string | null;
}
export interface UpdateBlueprintMutation {
  updateBlueprint: BlueprintView | null;
}
export const UpdateBlueprintDocument = gql`
  mutation UpdateBlueprint(
    $id: String!
    $name: String
    $autoSync: Boolean
    $path: String
    $ownerId: String
  ) {
    updateBlueprint(
      id: $id
      name: $name
      autoSync: $autoSync
      path: $path
      ownerId: $ownerId
    ) {
      id
      name
      repo
      branch
      path
      autoSync
      status
      lastSync
      updatedAt
    }
  }
` as unknown as TypedDocumentNode<UpdateBlueprintMutation, UpdateBlueprintVars>;

export interface DisconnectBlueprintVars {
  id: string;
  ownerId?: string | null;
}
export interface DisconnectBlueprintMutation {
  disconnectBlueprint: boolean | null;
}
export const DisconnectBlueprintDocument = gql`
  mutation DisconnectBlueprint($id: String!, $ownerId: String) {
    disconnectBlueprint(id: $id, ownerId: $ownerId)
  }
` as unknown as TypedDocumentNode<
  DisconnectBlueprintMutation,
  DisconnectBlueprintVars
>;

export interface SyncBlueprintVars {
  id: string;
  ownerId?: string | null;
  bexYaml?: string | null;
  confirm?: string | null;
}
export interface SyncBlueprintMutation {
  syncBlueprint: SyncBlueprintResult | null;
}
export const SyncBlueprintDocument = gql`
  mutation SyncBlueprint(
    $id: String!
    $ownerId: String
    $bexYaml: String
    $confirm: String
  ) {
    syncBlueprint(
      id: $id
      ownerId: $ownerId
      bexYaml: $bexYaml
      confirm: $confirm
    ) {
      blueprint {
        id
        name
        status
        lastSync
        updatedAt
      }
      services {
        id
        name
      }
      databases
    }
  }
` as unknown as TypedDocumentNode<SyncBlueprintMutation, SyncBlueprintVars>;
