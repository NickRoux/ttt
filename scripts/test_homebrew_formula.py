import unittest

from homebrew_formula import PLATFORMS, generate


class HomebrewFormulaTests(unittest.TestCase):
    def setUp(self):
        self.checksums = "\n".join(
            f"{str(index) * 64}  ttt_0.1.0_{system}_{arch}.tar.gz"
            for index, (system, arch) in enumerate(PLATFORMS, 1)
        )

    def test_every_platform_uses_its_own_archive_and_checksum(self):
        formula = generate("0.1.0", self.checksums)
        self.assertIn('version "0.1.0"', formula)
        for index, (system, arch) in enumerate(PLATFORMS, 1):
            self.assertIn(
                f'url "https://github.com/NickRoux/ttt/releases/download/v0.1.0/ttt_0.1.0_{system}_{arch}.tar.gz"\n'
                f'      sha256 "{str(index) * 64}"',
                formula,
            )

    def test_missing_platform_is_rejected(self):
        with self.assertRaisesRegex(ValueError, "missing checksum"):
            generate("0.1.0", self.checksums.splitlines()[0])

    def test_unsafe_or_mismatched_versions_are_rejected(self):
        for version in ("v0.1.0", "0.1", "00.1.0", "0.2.0", '0.1.0";system("bad")'):
            with self.subTest(version=version), self.assertRaises(ValueError):
                generate(version, self.checksums)

    def test_invalid_and_duplicate_checksums_are_rejected(self):
        for checksums in (
            "garbage",
            self.checksums.replace("1" * 64, "x" * 64),
            self.checksums + "\n" + self.checksums.splitlines()[0],
        ):
            with self.assertRaises(ValueError):
                generate("0.1.0", checksums)


if __name__ == "__main__":
    unittest.main()
