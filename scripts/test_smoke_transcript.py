"""Check smoke transcript ownership without reading any real Messages data."""

import json
import os
from pathlib import Path
import subprocess
import tempfile
import unittest

SCRIPT = Path(__file__).with_name("agent-smoke-transcript.sh")


class SmokeTranscriptTests(unittest.TestCase):
    def setUp(self):
        self.temporary = tempfile.TemporaryDirectory(prefix="imsgcrawl-transcript-test-")
        self.addCleanup(self.temporary.cleanup)
        self.root = Path(self.temporary.name)
        binaries = self.root / "bin"
        binaries.mkdir()
        stub = binaries / "imsgcrawl"
        stub.write_text("#!/bin/sh\nprintf '%s\\n' '{\"items\":[]}'\n")
        stub.chmod(0o700)
        self.environment = {**os.environ, "PATH": str(binaries) + os.pathsep + os.environ["PATH"]}

    def run_smoke(self, output):
        return subprocess.run(
            ["bash", str(SCRIPT), "--out-dir", str(output), "--inline-raw"],
            text=True, capture_output=True, env=self.environment, timeout=60,
            preexec_fn=lambda: os.umask(0o022),
        )

    def test_artifacts_are_private_under_permissive_umask(self):
        output = self.root / "nested" / "output"
        result = self.run_smoke(output)
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertTrue((output / "review.txt").is_file())
        self.assertTrue((output / "transcript.txt").is_file())
        self.assertTrue(list((output / "raw").glob("*.stdout")))
        for path in [output, *output.rglob("*")]:
            self.assertEqual(path.stat().st_mode & 0o077, 0, str(path))

    def test_step_headers_match_manifest(self):
        output = self.root / "output with spaces"
        (self.root / "bin" / "imsgcrawl").write_text(
            "#!/bin/sh\n"
            "if [ \"$1\" = --version ]; then printf 'synthetic failure\\n' >&2; exit 7; fi\n"
            "printf '%s\\n' '{\"items\":[]}'\n"
        )
        result = self.run_smoke(output)
        self.assertEqual(result.returncode, 1, result.stderr)
        entries = [json.loads(line) for line in (output / "manifest.jsonl").read_text().splitlines()]
        self.assertGreater(len(entries), 1)
        self.assertEqual(entries[0]["exit_code"], 7)
        review = (output / "review.txt").read_text()
        transcript = (output / "transcript.txt").read_text()
        for entry in entries:
            header = (
                "\n" + "=" * 80 + "\n"
                f"[{entry['id']}] {entry['name']}\n"
                + "=" * 80 + "\n"
                f"command: {entry['command']}\n"
                f"exit_code: {entry['exit_code']}\n"
                f"stdout_file: {entry['stdout_file']}\n"
                f"stderr_file: {entry['stderr_file']}\n"
                f"stdout_bytes: {entry['stdout_bytes']}\n"
                f"stderr_bytes: {entry['stderr_bytes']}\n"
            )
            with self.subTest(step=entry["id"]):
                self.assertEqual(review.count(header), 1)
                self.assertEqual(transcript.count(header), 1)

    def test_existing_directory_is_untouched(self):
        output = self.root / "existing"
        output.mkdir()
        marker = output / "review.txt"
        marker.write_text("keep existing synthetic data")
        before = marker.read_bytes()
        result = self.run_smoke(output)
        self.assertNotEqual(result.returncode, 0)
        self.assertEqual(marker.read_bytes(), before)
        self.assertEqual(list(output.iterdir()), [marker])

    def test_symlink_output_is_rejected(self):
        target = self.root / "target"
        target.mkdir()
        output = self.root / "alias"
        output.symlink_to(target, target_is_directory=True)
        result = self.run_smoke(output)
        self.assertNotEqual(result.returncode, 0)
        self.assertEqual(list(target.iterdir()), [])


if __name__ == "__main__":
    unittest.main()
