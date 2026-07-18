import gql from "graphql-tag";
import type { TypedDocumentNode } from "@graphql-typed-document-node/core";
import type { ProjectView } from "@/features/projects/hooks/use-projects";

export interface ProjectQueryVariables {
  id: string;
}

export interface ProjectQuery {
  project: ProjectView | null;
}

/** Single-project read used by the route loader and its two child pages. */
export const ProjectDocument = gql`
  query Project($id: String!) {
    project(id: $id) {
      id
      name
      ownerId
      serviceIds
      databaseIds
      keyValueIds
    }
  }
` as unknown as TypedDocumentNode<ProjectQuery, ProjectQueryVariables>;
