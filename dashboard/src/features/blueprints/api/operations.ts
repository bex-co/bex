// Hand-written TypedDocumentNodes for the Blueprints dashboard surface (w7/m27).
// Mirrors the GraphQL operations bex-api registers in apps/graphql.go (w2/m15 +
// w7/m27). When codegen can run against a live bex-api, migrate imports to
// @/graphql/definitions and delete this file.

import gql from "graphql-tag";
import type { TypedDocumentNode } from "@graphql-typed-document-node/core";
import type {
  BlueprintView,
  BlueprintValidationResult,
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
      manifest
      status
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
      manifest
      status
      createdAt
      updatedAt
    }
  }
` as unknown as TypedDocumentNode<BlueprintQuery, BlueprintQueryVars>;

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
` as unknown as TypedDocumentNode<ValidateBlueprintQuery, ValidateBlueprintQueryVars>;

// --- mutations ---

export interface SyncBlueprintVars {
  id: string;
  ownerId?: string | null;
  bexYaml?: string | null;
}
export interface SyncBlueprintMutation {
  syncBlueprint: SyncBlueprintResult | null;
}
export const SyncBlueprintDocument = gql`
  mutation SyncBlueprint($id: String!, $ownerId: String, $bexYaml: String) {
    syncBlueprint(id: $id, ownerId: $ownerId, bexYaml: $bexYaml) {
      blueprint {
        id
        name
        status
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
