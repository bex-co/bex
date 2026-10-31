"""Synthetic transports only: no credential imports or HTTP calls."""
import ast
import copy
import hashlib
import json
from pathlib import Path
import unittest
from urllib.parse import parse_qs, urlsplit
from unittest.mock import Mock

import yaml

ROOT = Path('/Users/tianpan/projects/bex7')
HARNESS = Path('/private/tmp/bex7-m144-disk-acceptance-n6d16nzf')
V2, V3 = (HARNESS / f'hosted-disk-observed-v{x}.py' for x in (2, 3))
SPEC = ROOT / 'lego/backend/internal/api/openapi/render-public-api-1.json'
DOCUMENT = json.loads(SPEC.read_text())


def resolve(value):
    while '$ref' in value:
        node = DOCUMENT
        for part in value['$ref'][2:].split('/'):
            part = part.replace('~1', '/').replace('~0', '~')
            node = node[int(part)] if isinstance(node, list) else node[part]
        value = node
    return value


def allowed_query(path):
    item = DOCUMENT['paths'][path]
    return {p['name'] for raw in item.get('parameters', []) + item['get'].get('parameters', [])
            if (p := resolve(raw))['in'] == 'query'}


class SurfaceTests(unittest.TestCase):
    def prepare(self, source=V3, size=1, legacy_transport=False, wrong_disk=False, capacity=None):
        disk = {'id': 'dsk-synthetic', 'serviceId': 'srv-synthetic', 'name': 'synthetic-disk',
                'mountPath': '/var/data', 'sizeGB': size}
        capacity = size * 1024**3 if capacity is None else capacity
        used = [{'values': [{'time': 'synthetic', 'value': 491520}]}]
        cap = [{'values': [{'time': 'synthetic', 'value': capacity}]}]
        calls, ledger = [], {}

        def request(method, target, body=None):
            calls.append((method, target, copy.deepcopy(body)))
            url = urlsplit(target)
            if method == 'GET' and url.path.startswith('/v1/metrics/'):
                params = parse_qs(url.query)
                allowed = allowed_query(url.path.removeprefix('/v1'))
                if not legacy_transport:
                    self.assertLessEqual(set(params), allowed)
                self.assertEqual(params['resource'], ['srv-synthetic'])
                response = cap if url.path.endswith('disk-capacity') else used
            elif (method, target) == ('GET', '/v1/disks/dsk-synthetic'):
                response = {**disk, 'id': 'dsk-foreign'} if wrong_disk else disk
            elif (method, target) == ('POST', '/graphql'):
                if 'd' in body['variables']:
                    response = {'data': {'disk': disk, 'service': {'disk': disk}}}
                else:
                    query = body['variables']['q']
                    self.assertEqual((query['kind'], query['resource']), ('service', 'srv-synthetic'))
                    response = {'data': {'datastoreMetrics': cap if query['name'] == 'DISK_CAPACITY' else used}}
            elif (method, target) == ('POST', '/mcp'):
                tool = body['params']['name']
                if tool == 'get_disk':
                    self.assertEqual(body['params']['arguments'], {'diskId': 'dsk-synthetic'})
                    value = disk
                else:
                    self.assertEqual(tool, 'get_datastore_metrics')
                    value = {'series': [
                        {'labels': {'metric': 'disk'}, 'points': [{'value': 491520}]},
                        {'labels': {'metric': 'disk_capacity'}, 'points': [{'value': capacity}]}]}
                response = {'result': {'structuredContent': value}}
            elif (method, target) == ('POST', '/v1/blueprints/generate'):
                self.assertEqual(body, {'ownerId': 'tea-synthetic', 'serviceIds': ['srv-synthetic']})
                response = {'manifest': yaml.safe_dump({'services': [{'disk': disk}]})}
            elif (method, target) == ('POST', '/v1/blueprints/validate'):
                response = {'valid': True}
            else:
                self.fail(f'unexpected synthetic transport: {method} {target}')
            return {'HTTP': 200, 'response': json.dumps(response),
                    'request': {'method': method, 'path': target}}

        names = {'surfaces', 'rpc', 'mcp_value', 'series_values', 'value'}
        tree = ast.parse(source.read_text())
        functions = [n for n in tree.body if isinstance(n, ast.FunctionDef) and n.name in names]
        namespace = {'disk': lambda: copy.deepcopy(disk), 'request': request, 'yaml': yaml,
                     'json': json, 'WS': 'tea-synthetic', 'S': {'mount': '/var/data'},
                     'L': ledger, 'save': Mock(), 'now': lambda: 'synthetic'}
        exec(compile(ast.Module(body=functions, type_ignores=[]), str(source), 'exec'), namespace)
        return namespace, calls, ledger

    def test_resource_and_usage_receipts_coexist_at_both_requested_sizes(self):
        for size in (1, 5):
            with self.subTest(size=size):
                ns, calls, ledger = self.prepare(size=size)
                result = ns['surfaces']()
                receipts = ledger['surface_readbacks'][0]['receipts']
                self.assertEqual(receipts['REST_disk_resource']['request']['path'], '/v1/disks/dsk-synthetic')
                self.assertEqual(receipts['REST_disk']['request']['path'], '/v1/metrics/disk-usage?resource=srv-synthetic')
                self.assertIsNot(receipts['REST_disk_resource'], receipts['REST_disk'])
                self.assertTrue(ledger['surface_readbacks'][0]['verified'])
                self.assertEqual(result['size_gib'], size)
                self.assertTrue(all('kind' not in parse_qs(urlsplit(path).query)
                                    for method, path, _ in calls if path.startswith('/v1/metrics/')))

    def test_actual_pinned_public_query_schema_excludes_kind(self):
        self.assertEqual(hashlib.sha256(SPEC.read_bytes()).hexdigest(),
                         'c6851bc830fa0f01fa58face22c8cf46d4762054c6fe879d724fae5ade72a520')
        for path in ('/metrics/disk-usage', '/metrics/disk-capacity'):
            self.assertEqual(allowed_query(path), {'startTime', 'endTime', 'resolutionSeconds', 'resource', 'service'})

    def test_original_v2_query_is_rejected_by_the_same_public_schema(self):
        ns, _, _ = self.prepare(source=V2)
        with self.assertRaises(AssertionError):
            ns['surfaces']()

    def test_original_collision_is_independently_reproduced(self):
        # A synthetic permissive transport isolates the lost-resource defect;
        # it grants no approval for the original undeclared live query.
        ns, _, ledger = self.prepare(source=V2, legacy_transport=True)
        with self.assertRaises(AssertionError):
            ns['surfaces']()
        receipts = ledger['surface_readbacks'][0]['receipts']
        self.assertNotIn('REST_disk_resource', receipts)
        self.assertIn('/metrics/disk-usage?', receipts['REST_disk']['request']['path'])
        self.assertNotIn('verified', ledger['surface_readbacks'][0])

    def test_foreign_disk_response_still_refuses_acceptance(self):
        ns, _, ledger = self.prepare(wrong_disk=True)
        with self.assertRaises(AssertionError):
            ns['surfaces']()
        self.assertNotIn('verified', ledger['surface_readbacks'][0])

    def test_physical_ten_gib_capacity_cannot_pass_requested_one_gib(self):
        ns, _, ledger = self.prepare(capacity=10 * 1024**3)
        with self.assertRaises(AssertionError):
            ns['surfaces']()
        self.assertNotIn('verified', ledger['surface_readbacks'][0])

    def test_only_surfaces_changed_and_all_other_functions_are_byte_identical(self):
        def functions(path):
            text = path.read_text()
            return {n.name: ast.get_source_segment(text, n) for n in ast.parse(text).body
                    if isinstance(n, ast.FunctionDef)}
        before, after = functions(V2), functions(V3)
        self.assertEqual(before.keys(), after.keys())
        self.assertEqual([name for name in before if before[name] != after[name]], ['surfaces'])


if __name__ == '__main__':
    unittest.main(verbosity=2)
