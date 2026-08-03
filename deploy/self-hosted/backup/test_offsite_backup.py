import importlib.util
import json
import os
import subprocess
import sys
import tempfile
import unittest
from pathlib import Path
from unittest import mock


MODULE_PATH = Path(__file__).with_name("offsite_backup.py")
SPEC = importlib.util.spec_from_file_location("offsite_backup", MODULE_PATH)
assert SPEC and SPEC.loader
backup = importlib.util.module_from_spec(SPEC)
sys.modules[SPEC.name] = backup
SPEC.loader.exec_module(backup)


class OffsiteBackupTests(unittest.TestCase):
    def environment(self, root: Path) -> dict[str, str]:
        return {
            "OFFSITE_BACKUP_ENABLED": "true",
            "DATABASE_URL": "postgres://app:secret@postgres:5432/app?sslmode=disable",
            "STORAGE_PROVIDER": "local",
            "SOURCE_STORAGE_PATH": str(root / "source"),
            "SOURCE_CONFIG_PATH": str(root / "config"),
            "OFFSITE_BACKUP_WORK_ROOT": str(root / "work"),
            "OFFSITE_S3_ENDPOINT": "https://s3.us-east-1.amazonaws.com",
            "OFFSITE_S3_BUCKET": "example-backups",
            "OFFSITE_S3_PREFIX": "instances/chat.example.com",
            "OFFSITE_S3_REGION": "us-east-1",
            "OFFSITE_S3_ACCESS_KEY_ID": "test-access",
            "OFFSITE_S3_SECRET_ACCESS_KEY": "test-secret",
            "OFFSITE_RESTIC_PASSWORD": "not-a-real-password",
            "INSTANCE_DOMAIN": "chat.example.com",
        }

    def settings(self, root: Path):
        (root / "source").mkdir()
        (root / "config").mkdir()
        with mock.patch.dict(os.environ, self.environment(root), clear=True):
            return backup.Settings.from_env()

    def test_builds_aws_and_minio_repository_urls(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            settings = self.settings(root)
            self.assertEqual(
                settings.repository,
                "s3:https://s3.us-east-1.amazonaws.com/example-backups/instances/chat.example.com",
            )

            environment = self.environment(root)
            environment["OFFSITE_S3_ENDPOINT"] = "http://minio:9000"
            environment["OFFSITE_S3_BUCKET_LOOKUP"] = "path"
            with mock.patch.dict(os.environ, environment, clear=True):
                minio = backup.Settings.from_env()
            self.assertEqual(
                minio.repository,
                "s3:http://minio:9000/example-backups/instances/chat.example.com",
            )
            self.assertEqual(minio.bucket_lookup, "path")

    def test_rejects_traversal_in_object_prefix(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            (root / "source").mkdir()
            (root / "config").mkdir()
            environment = self.environment(root)
            environment["OFFSITE_S3_PREFIX"] = "tenant/../other"
            with mock.patch.dict(os.environ, environment, clear=True):
                with self.assertRaisesRegex(backup.BackupError, "safe object prefix"):
                    backup.Settings.from_env()

    def test_rejects_same_bucket_as_primary_object_storage(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            (root / "source").mkdir()
            (root / "config").mkdir()
            environment = self.environment(root)
            environment.update(
                {
                    "STORAGE_PROVIDER": "minio",
                    "S3_ENDPOINT": environment["OFFSITE_S3_ENDPOINT"],
                    "MINIO_BUCKET": environment["OFFSITE_S3_BUCKET"],
                }
            )
            with mock.patch.dict(os.environ, environment, clear=True):
                with self.assertRaisesRegex(backup.BackupError, "bucket separate"):
                    backup.Settings.from_env()

    def create_bundle(self, root: Path):
        settings = self.settings(root)
        bundle = root / "bundle"
        (bundle / "storage" / "workspace").mkdir(parents=True)
        (bundle / "config").mkdir()
        (bundle / "database.dump").write_bytes(b"PGDMP\x01test-database")
        (bundle / "storage" / "workspace" / "avatar.png").write_bytes(b"image")
        (bundle / "config" / "compose.yml").write_text("services: {}\n", encoding="utf-8")
        backup.build_manifest(settings, bundle, "16.10")
        return settings, bundle

    def test_manifest_round_trip_verifies_all_files(self):
        with tempfile.TemporaryDirectory() as directory:
            settings, bundle = self.create_bundle(Path(directory))
            manifest = backup.verify_bundle(bundle, settings, validate_archive=False)
            self.assertEqual(manifest["schema_version"], 1)
            checksums = json.loads((bundle / "checksums.json").read_text(encoding="utf-8"))
            self.assertEqual(
                {entry["path"] for entry in checksums["files"]},
                {"database.dump", "storage/workspace/avatar.png", "config/compose.yml"},
            )

    def test_manifest_rejects_checksum_path_traversal_even_with_updated_digest(self):
        with tempfile.TemporaryDirectory() as directory:
            settings, bundle = self.create_bundle(Path(directory))
            checksums_path = bundle / "checksums.json"
            checksums = json.loads(checksums_path.read_text(encoding="utf-8"))
            checksums["files"][0]["path"] = "storage/../../outside"
            backup.write_json(checksums_path, checksums)
            manifest_path = bundle / "manifest.json"
            manifest = json.loads(manifest_path.read_text(encoding="utf-8"))
            manifest["checksums"]["sha256"] = backup.sha256_file(checksums_path)
            backup.write_json(manifest_path, manifest)
            with self.assertRaisesRegex(backup.BackupError, "path traversal"):
                backup.verify_bundle(bundle, settings, validate_archive=False)

    def test_bundle_rejects_unlisted_and_symlink_files(self):
        with tempfile.TemporaryDirectory() as directory:
            settings, bundle = self.create_bundle(Path(directory))
            (bundle / "storage" / "unexpected").write_bytes(b"extra")
            with self.assertRaisesRegex(backup.BackupError, "inventory mismatch"):
                backup.verify_bundle(bundle, settings, validate_archive=False)
            (bundle / "storage" / "unexpected").unlink()
            link = bundle / "storage" / "link"
            try:
                link.symlink_to(bundle / "database.dump")
            except (OSError, NotImplementedError):
                self.skipTest("symlinks are unavailable on this platform")
            with self.assertRaisesRegex(backup.BackupError, "symlink or special file"):
                backup.verify_bundle(bundle, settings, validate_archive=False)

    def test_snapshot_and_stage_names_are_strict(self):
        self.assertEqual(backup.validate_snapshot_id("a1b2c3d4"), "a1b2c3d4")
        for value in ("latest", "../abc12345", "abc", "abc1234g"):
            with self.assertRaises(backup.BackupError):
                backup.validate_snapshot_id(value)
        with tempfile.TemporaryDirectory() as directory:
            settings = self.settings(Path(directory))
            with self.assertRaisesRegex(backup.BackupError, "unsafe"):
                backup.safe_stage_path(settings, "../restore")

    def test_default_disabled_does_not_require_remote_when_inspecting_defaults(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            environment = {
                "DATABASE_URL": "postgres://app:secret@postgres/app",
                "SOURCE_STORAGE_PATH": str(root / "source"),
                "OFFSITE_BACKUP_WORK_ROOT": str(root / "work"),
            }
            with mock.patch.dict(os.environ, environment, clear=True):
                settings = backup.Settings.from_env(require_remote=False)
            self.assertFalse(settings.enabled)
            self.assertFalse(settings.include_instance_env)
            self.assertEqual(settings.repository, "")

    def test_backup_run_telemetry_uses_full_null_job_and_is_best_effort(self):
        with tempfile.TemporaryDirectory() as directory:
            settings = self.settings(Path(directory))
            run_id = "12345678-1234-4234-8234-123456789abc"
            completed = subprocess.CompletedProcess([], 0, stdout=run_id + "\nINSERT 0 1\n", stderr="")
            with mock.patch.object(backup, "run_command", return_value=completed) as command:
                self.assertEqual(backup.record_backup_run_started(settings), run_id)
            sql = command.call_args.args[0]
            self.assertIn("backup_job_id, status, backup_type", sql[-1])
            self.assertIn("NULL, 'running', 'full'", sql[-1])

            with mock.patch.object(backup, "run_command", return_value=completed) as finish_command:
                backup.finish_backup_run(
                    settings,
                    run_id,
                    status="success",
                    object_key="s3:example/snapshot#abc12345",
                    byte_size=42,
                    checksum="a" * 64,
                )
            finish_args = finish_command.call_args.args[0]
            self.assertIn("status=success", finish_args)
            self.assertIn("byte_size=42", finish_args)
            self.assertIn("checksum_sha256", finish_args[-1])

            with mock.patch.object(backup, "run_command", side_effect=backup.BackupError("telemetry unavailable")):
                self.assertEqual(backup.record_backup_run_started(settings), "")
                # Finishing telemetry must never raise over the real backup result.
                backup.finish_backup_run(
                    settings,
                    run_id,
                    status="success",
                    object_key="s3:example/snapshot#abc12345",
                    byte_size=42,
                    checksum="a" * 64,
                )


if __name__ == "__main__":
    unittest.main()
