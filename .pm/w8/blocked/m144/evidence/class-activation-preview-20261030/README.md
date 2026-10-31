# Reviewed activation preview — not applied

The prepared patch selects the bounded class for new production disks and guards its filesystem/mode, expansion, binding/reclaim policy, both Secret references and the reviewed CSI image pin. The legacy class remains for existing claims; local/default renders still exclude the production class. Three final simplify reviewers found no worthwhile changes. Full GitOps validation passed in a detached checkout, with the exact patch/log/file hashes retained here.

This is a preview. The production inventory confirms the bounded class already exists with the correct contract, but the actual Ready operator still predates the admitted-capacity writer. Main and production retain `hcloud-volumes-luks`. Apply only after the newer signed operator is actually Ready, then verify the effective bounded class and existing bindings again before the guarded owned fixture. No class or tenant resource changed during validation.
