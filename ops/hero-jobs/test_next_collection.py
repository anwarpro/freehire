import importlib.util
from pathlib import Path
import tempfile
import unittest

spec = importlib.util.spec_from_file_location('next_collection', Path(__file__).with_name('next-collection.py'))
module = importlib.util.module_from_spec(spec)
spec.loader.exec_module(module)

class CollectionRotationTests(unittest.TestCase):
    def test_company_priority_is_bounded_and_preserves_board_scope(self):
        rows = [dict(provider='careerpage', board='https://company.test/jobs?x=1', region='', surface='company-career'),
                dict(provider='greenhouse', board='general', region='', surface='curator')]
        with tempfile.TemporaryDirectory() as directory:
            state = Path(directory)
            self.assertEqual(module.choose(rows, {'greenhouse'}, state, 10000000000), ('careerpage', 'https://company.test/jobs?x=1', ''))
            self.assertEqual(module.choose(rows, {'greenhouse'}, state, 0), ('greenhouse', '', ''))
    def test_invalid_provider_and_control_characters_do_not_become_arguments(self):
        rows = [dict(provider='bad;command', board='x', region='', surface='company-career'),
                dict(provider='careerpage', board='bad\nboard', region='', surface='company-career')]
        with tempfile.TemporaryDirectory() as directory:
            self.assertEqual(module.choose(rows, set(), Path(directory), 0), ('', '', ''))

if __name__ == '__main__': unittest.main()
