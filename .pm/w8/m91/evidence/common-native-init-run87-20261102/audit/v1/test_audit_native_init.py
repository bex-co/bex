"""Pure offline original-predicate controls; never target execution or CI proof."""
import copy
import importlib.util
from pathlib import Path
import unittest

ROOT = Path(__file__).resolve().parent
spec = importlib.util.spec_from_file_location("native_init_readonly_audit", ROOT / "audit_native_init.py")
audit = importlib.util.module_from_spec(spec)
spec.loader.exec_module(audit)


class NativeInitPredicateControls(unittest.TestCase):
    def setUp(self):
        self.generated = next((audit.ARTIFACT / "generated").iterdir())
        self.expected = audit.load(self.generated / "source-custody.json")["expected"]

    def test_each_actual_retained_proof_preserves_original_positive_predicates(self):
        for launch in audit.LAUNCHES:
            with self.subTest(launch=launch):
                receipt = audit.load(self.generated / launch / "receipt.json")
                audit.validate_proof(self.expected, receipt["pre_stop"], launch, False)
                audit.validate_proof(self.expected, receipt["proof"], launch, True)
                self.assertEqual(receipt["proof"], audit.go_proof(audit.load(self.generated / launch / "qa-output/runtime.json")))

    def test_original_identity_drain_and_mapping_negative_controls(self):
        proof = audit.load(self.generated / "npm/receipt.json")["proof"]
        def altered(path, value):
            actual = copy.deepcopy(proof)
            parent = actual
            for key in path[:-1]:parent = parent[key]
            parent[path[-1]] = value
            return actual
        cases = [(('Identity', 'Node'), '24.0.0'), (('Identity', 'BaseEnv'), {}),
                 (('Identity', 'Init', 'Argv'), ['/native-init']),
                 (('Identity', 'Init', 'Mappings'), []), (('Identity', 'Init', 'Inode'), '999999'),
                 (('Identity', 'Init', 'SHA256'), '0' * 64), (('Identity', 'Init', 'PID1Machine'), 62),
                 (('Events',), ['listening', 'hold_begin', 'hold_end', 'closed']),
                 (('Signals',), 0), (('Signals',), True), (('Completed',), 0), (('Pass',), False), (('Run',), 'other-case')]
        for path, value in cases:
            with self.subTest(path=path), self.assertRaises(AssertionError):
                audit.validate_proof(self.expected, altered(path, value), 'npm', True)

    def test_original_wrapper_exit_matrix_and_direct_path_rule(self):
        for launch in audit.LAUNCHES:
            self.assertTrue(audit.accepted_exit(launch, 0))
            self.assertEqual(audit.accepted_exit(launch, 143), launch != 'direct')
            self.assertEqual(audit.accepted_exit(launch, 1), launch == 'yarn')
            self.assertFalse(audit.accepted_exit(launch, 137))
            self.assertFalse(audit.accepted_exit(launch, True))
        proof = audit.load(self.generated / 'direct/receipt.json')['proof']
        proof['Identity']['Env']['PATH'] = '/wrong'
        with self.assertRaises(AssertionError):audit.validate_proof(self.expected, proof, 'direct', True)


if __name__ == '__main__':
    unittest.main()
