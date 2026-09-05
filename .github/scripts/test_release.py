#!/usr/bin/env python3
"""Unit tests for the release notes engine in release.py.

Run with:
    mise exec -- python .github/scripts/test_release.py
"""

import sys
import tempfile
import unittest
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parent))

import release


class YamlDialectTests(unittest.TestCase):
    def test_parses_scalars_lists_and_blocks(self) -> None:
        text = "\n".join(
            [
                "# comment ignored",
                "tag: v1.2.3",
                'title: "Quoted: title"',
                "highlights: |",
                "  first line",
                "  second line",
                "",
                "features:",
                "  - plain item",
                '  - "item with: colon"',
                "fixes:",
                "  - only fix",
            ]
        )

        data = release.parse_release_notes_yaml(text)

        self.assertEqual(data["tag"], "v1.2.3")
        self.assertEqual(data["title"], "Quoted: title")
        self.assertEqual(data["highlights"], "first line\nsecond line")
        self.assertEqual(data["features"], ["plain item", "item with: colon"])
        self.assertEqual(data["fixes"], ["only fix"])

    def test_folded_block_joins_lines(self) -> None:
        data = release.parse_release_notes_yaml("highlights: >\n  one\n  two\n")
        self.assertEqual(data["highlights"], "one two")


class BodyFromNotesTests(unittest.TestCase):
    def test_renders_every_section_and_compare_link(self) -> None:
        body = release.build_release_body_from_notes(
            {
                "title": "Hardening",
                "highlights": "Summary line.",
                "features": ["new flag"],
                "fixes": ["fixed crash"],
                "changes": ["bumped Go"],
                "breaking": ["changed default"],
            },
            "v0.1.4",
            "v0.1.5",
            "corebunker/go-overlay",
        )

        self.assertIn("## Hardening", body)
        self.assertIn("Summary line.", body)
        self.assertIn("### Features\n- new flag", body)
        self.assertIn("### Fixes\n- fixed crash", body)
        self.assertIn("### Changes\n- bumped Go", body)
        self.assertIn("### Breaking changes\n- changed default", body)
        self.assertIn(
            "compare/v0.1.4...v0.1.5",
            body,
        )

    def test_explicit_previous_overrides_detection(self) -> None:
        body = release.build_release_body_from_notes(
            {"title": "T", "fixes": ["f"], "previous": "v0.0.9"},
            "v0.1.4",
            "v0.1.5",
            "owner/repo",
        )
        self.assertIn("compare/v0.0.9...v0.1.5", body)

    def test_tag_mismatch_is_rejected(self) -> None:
        with self.assertRaises(ValueError):
            release.build_release_body_from_notes(
                {"tag": "v9.9.9", "fixes": ["f"]}, None, "v0.1.5", "owner/repo"
            )

    def test_empty_notes_are_rejected(self) -> None:
        with self.assertRaises(ValueError):
            release.build_release_body_from_notes(
                {"title": "Nothing here"}, None, "v0.1.5", "owner/repo"
            )


class BodyFromCommitsTests(unittest.TestCase):
    def test_groups_conventional_commits(self) -> None:
        body = release.build_release_body(
            [
                "feat(cli): add --config flag",
                "fix: stop double wait",
                "chore: tidy imports",
            ],
            "v0.1.4",
            "v0.1.5",
            "owner/repo",
        )

        self.assertIn("### Features\n- add --config flag", body)
        self.assertIn("### Fixes\n- stop double wait", body)
        self.assertIn("### Other changes\n- tidy imports", body)

    def test_no_commits_returns_none(self) -> None:
        self.assertIsNone(
            release.build_release_body([], "v0.1.4", "v0.1.5", "owner/repo")
        )


class NotesLookupTests(unittest.TestCase):
    def test_notes_file_is_used_when_present(self) -> None:
        with tempfile.TemporaryDirectory() as raw_root:
            root = Path(raw_root)
            notes_dir = root / release.NOTES_DIR
            notes_dir.mkdir()
            (notes_dir / "v0.1.9.yaml").write_text(
                'title: From file\nfixes:\n  - "handled"\n', encoding="utf-8"
            )

            body = release.release_notes_from_file(
                "v0.1.9", "owner/repo", root=root
            )

            self.assertIsNotNone(body)
            self.assertIn("## From file", body)
            self.assertIn("- handled", body)

    def test_missing_notes_file_returns_none(self) -> None:
        with tempfile.TemporaryDirectory() as raw_root:
            self.assertIsNone(
                release.release_notes_from_file(
                    "v9.9.9", "owner/repo", root=Path(raw_root)
                )
            )


class ShippedNotesTests(unittest.TestCase):
    def test_every_shipped_notes_file_renders(self) -> None:
        notes_dir = release.REPO_ROOT / release.NOTES_DIR
        files = sorted(notes_dir.glob("v*.yaml"))
        self.assertTrue(files, "no release notes files found")

        for path in files:
            tag = path.stem
            with self.subTest(tag=tag):
                data = release.load_release_notes_file(path)
                body = release.build_release_body_from_notes(
                    data, None, tag, "corebunker/go-overlay"
                )
                self.assertTrue(body.startswith("## "))


if __name__ == "__main__":
    unittest.main(verbosity=2)
