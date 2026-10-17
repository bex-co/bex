import type {
  DocumentNode,
  FragmentDefinitionNode,
  OperationDefinitionNode,
  SelectionSetNode,
} from "graphql";

function blank(
  set: SelectionSetNode,
  fragments: Map<string, FragmentDefinitionNode>,
): Record<string, unknown> {
  const out: Record<string, unknown> = {};
  for (const s of set.selections) {
    if (s.kind === "Field") out[s.alias?.value ?? s.name.value] = null;
    else if (s.kind === "FragmentSpread")
      Object.assign(
        out,
        blank(fragments.get(s.name.value)!.selectionSet, fragments),
      );
    else Object.assign(out, blank(s.selectionSet, fragments));
  }
  return out;
}

/**
 * Every field `doc` selects under its root field `field`, as null — a
 * complete object for a real Apollo cache write without copying a long
 * selection set into a test. Spread it, then set the fields under test.
 */
export function blankSelection(
  doc: DocumentNode,
  field: string,
): Record<string, unknown> {
  const fragments = new Map(
    doc.definitions
      .filter(
        (d): d is FragmentDefinitionNode => d.kind === "FragmentDefinition",
      )
      .map((d) => [d.name.value, d]),
  );
  const op = doc.definitions.find(
    (d): d is OperationDefinitionNode => d.kind === "OperationDefinition",
  )!;
  const root = op.selectionSet.selections.find(
    (s) => s.kind === "Field" && s.name.value === field,
  );
  if (!root || root.kind !== "Field" || !root.selectionSet)
    throw new Error(`no ${field} selection`);
  return blank(root.selectionSet, fragments);
}
