import { useState, useCallback } from "react";
import { useLazyQuery } from "@apollo/client/react";
import { ValidateBlueprintDocument } from "@/graphql/definitions";
import type { BlueprintValidationResult } from "@/features/blueprints/types";
import { toBlueprintValidationResult } from "@/features/blueprints/lib/views";

export interface UseValidateBlueprintResult {
  validate: (bexYaml: string) => Promise<BlueprintValidationResult | null>;
  result: BlueprintValidationResult | null;
  loading: boolean;
}

export function useValidateBlueprint(): UseValidateBlueprintResult {
  const [run, { loading }] = useLazyQuery(ValidateBlueprintDocument, {
    fetchPolicy: "no-cache",
  });
  const [result, setResult] = useState<BlueprintValidationResult | null>(null);

  const validate = useCallback(
    async (bexYaml: string): Promise<BlueprintValidationResult | null> => {
      setResult(null);
      const res = await run({ variables: { bexYaml } });
      const validation = res.data?.validateBlueprint
        ? toBlueprintValidationResult(res.data.validateBlueprint)
        : null;
      setResult(validation);
      return validation;
    },
    [run],
  );

  return { validate, result, loading };
}
