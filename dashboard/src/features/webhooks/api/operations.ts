import gql from "graphql-tag";
import type { TypedDocumentNode } from "@graphql-typed-document-node/core";
import type {
  CreateWebhookEndpointMutation,
  CreateWebhookEndpointMutationVariables,
  WebhookDeliveriesQuery,
  WebhookDeliveriesQueryVariables,
} from "@/graphql/definitions";

// Kept beside webhooks.graphql while offline codegen is blocked by the current
// GraphQL 17/codegen plugin crash. These typed documents carry the corrected
// runtime selections; definitions.ts retains the generated schema types.
export const CreateWebhookEndpointDocument = gql`
  mutation CreateWebhookEndpoint(
    $ownerId: String
    $name: String
    $url: String!
    $eventTypes: [String]!
    $enabled: Boolean
  ) {
    createWebhookEndpoint(
      ownerId: $ownerId
      name: $name
      url: $url
      eventTypes: $eventTypes
      enabled: $enabled
    ) {
      id
      name
      url
      eventTypes
      enabled
      secret
      createdAt
    }
  }
` as unknown as TypedDocumentNode<
  CreateWebhookEndpointMutation,
  CreateWebhookEndpointMutationVariables
>;

export const WebhookDeliveriesDocument = gql`
  query WebhookDeliveries(
    $endpointId: String!
    $ownerId: String
    $cursor: String
    $limit: Int
    $sentAfter: String
    $sentBefore: String
    $status: String
  ) {
    webhookDeliveries(
      endpointId: $endpointId
      ownerId: $ownerId
      cursor: $cursor
      limit: $limit
      sentAfter: $sentAfter
      sentBefore: $sentBefore
      status: $status
    ) {
      id
      eventType
      serviceId
      status
      attemptCount
      lastStatusCode
      lastError
      responseBody
      sentAt
      nextAttemptAt
      deliveredAt
      createdAt
      cursor
    }
  }
` as unknown as TypedDocumentNode<
  WebhookDeliveriesQuery,
  WebhookDeliveriesQueryVariables
>;
