"""Regression checks for broker selection and persisted credential protection."""
import importlib.util
import os
from pathlib import Path
import sys
import tempfile
import types
import unittest

# The tests below exercise storage/selection, not an MSAL or broker emulator.
try:
    import msal
except ImportError:
    module = types.ModuleType("msal")
    module.SerializableTokenCache = object
    module.PublicClientApplication = object
    sys.modules["msal"] = module
spec = importlib.util.spec_from_file_location("wam_auth", Path(__file__).with_name("wam_auth.py"))
wam = importlib.util.module_from_spec(spec)
spec.loader.exec_module(wam)

class BrokerBoundaries(unittest.TestCase):
    def test_no_username_fallback_for_explicit_account_id(self):
        other = {"local_account_id": "wrong-id", "username": "same@example.invalid"}
        self.assertIsNone(wam.find_account([other], "expected-id", "same@example.invalid"))

    @unittest.skipUnless(os.name == "nt", "Windows DPAPI")
    def test_cache_encrypted_on_disk(self):
        cache = types.SimpleNamespace(has_state_changed=True, serialize=lambda: '{"refresh_token":"distinct-test-secret"}')
        with tempfile.TemporaryDirectory() as directory:
            path = str(Path(directory)/"cache")
            wam.save_token_cache(cache, path)
            self.assertNotIn(b"distinct-test-secret", Path(path).read_bytes())
            loaded = []
            wam.load_token_cache(types.SimpleNamespace(deserialize=loaded.append), path)
            self.assertEqual(loaded, [cache.serialize()])

if __name__ == "__main__":
    unittest.main()
