/**
 * Element-level non-null guard for the generated GraphQL result types, whose
 * lists carry nullable items per the schema. Use as `list.filter(nonNull)` to
 * drop the nulls once at the view boundary.
 */
export function nonNull<T>(x: T): x is NonNullable<T> {
  return x != null;
}
