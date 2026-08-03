#!/usr/bin/env python3
"""Encrypted off-site backup and guarded restore tooling for self-host installs.

The application containers never receive the off-site credentials. This utility
is run through the opt-in Compose profiles in deploy/self-hosted/compose.yml.
Restic owns encryption, compression, repository integrity, multipart S3 upload,
and snapshot retention. This file owns the application-level bundle manifest,
PostgreSQL dump, primary object snapshot, and destructive restore guardrails.
"""

from __future__ import annotations

import argparse
import contextlib
import dataclasses
import datetime as dt
import hashlib
import hmac
import json
import os
import re
import shutil
import signal
import stat
import subprocess
import sys
import tempfile
import time
import urllib.parse
import uuid
from pathlib import Path, PurePosixPath
from typing import Iterable, Iterator, Mapping, Sequence

try:
    import fcntl  # type: ignore[import-not-found]
except ImportError:  # pragma: no cover - the production container is Linux.
    fcntl = None  # type: ignore[assignment]


TAG = "webtui-offsite-v1"
SCHEMA_VERSION = 1
SNAPSHOT_RE = re.compile(r"^[0-9a-fA-F]{8,64}$")
SAFE_NAME_RE = re.compile(r"^[A-Za-z0-9][A-Za-z0-9._-]{0,127}$")
BUCKET_RE = re.compile(r"^[a-z0-9][a-z0-9.-]{1,61}[a-z0-9]$")
RESERVED_STORAGE_PREFIX = ".webtui-restore-"
MAX_METADATA_BYTES = 128 * 1024 * 1024


class BackupError(RuntimeError):
    pass


def log(message: str) -> None:
    print(f"[offsite-backup] {message}", file=sys.stderr, flush=True)


def env_bool(name: str, default: bool = False) -> bool:
    raw = os.getenv(name)
    if raw is None or not raw.strip():
        return default
    value = raw.strip().lower()
    if value in {"1", "true", "yes", "on"}:
        return True
    if value in {"0", "false", "no", "off"}:
        return False
    raise BackupError(f"{name} must be true or false")


def env_int(name: str, default: int, minimum: int = 0, maximum: int | None = None) -> int:
    raw = os.getenv(name, str(default)).strip()
    try:
        value = int(raw)
    except ValueError as exc:
        raise BackupError(f"{name} must be an integer") from exc
    if value < minimum or (maximum is not None and value > maximum):
        upper = f" and <= {maximum}" if maximum is not None else ""
        raise BackupError(f"{name} must be >= {minimum}{upper}")
    return value


def require_text(name: str) -> str:
    value = os.getenv(name, "").strip()
    if not value:
        raise BackupError(f"{name} is required")
    if any(ord(char) < 32 for char in value):
        raise BackupError(f"{name} contains control characters")
    return value


def clean_prefix(raw: str, name: str) -> str:
    raw = raw.strip().strip("/")
    if not raw:
        return ""
    if "\\" in raw or any(ord(char) < 32 for char in raw):
        raise BackupError(f"{name} is not a safe object prefix")
    parts = raw.split("/")
    if any(part in {"", ".", ".."} for part in parts):
        raise BackupError(f"{name} is not a safe object prefix")
    return "/".join(parts)


def validate_endpoint(raw: str) -> str:
    endpoint = raw.strip().rstrip("/")
    if not endpoint or any(char.isspace() for char in endpoint):
        raise BackupError("OFFSITE_S3_ENDPOINT is invalid")
    candidate = endpoint if "://" in endpoint else "https://" + endpoint
    parsed = urllib.parse.urlsplit(candidate)
    if parsed.scheme not in {"http", "https"} or not parsed.hostname:
        raise BackupError("OFFSITE_S3_ENDPOINT must be an HTTP(S) endpoint or hostname")
    if parsed.username or parsed.password or parsed.query or parsed.fragment:
        raise BackupError("OFFSITE_S3_ENDPOINT must not contain credentials, query, or fragment")
    if parsed.path not in {"", "/"}:
        raise BackupError("OFFSITE_S3_ENDPOINT must not contain a path")
    return endpoint


def validate_bucket(value: str, name: str) -> str:
    bucket = value.strip().lower()
    if not BUCKET_RE.fullmatch(bucket) or ".." in bucket or ".-" in bucket or "-." in bucket:
        raise BackupError(f"{name} is not a valid S3 bucket name")
    return bucket


def safe_instance_name(raw: str) -> str:
    value = re.sub(r"[^A-Za-z0-9._-]+", "-", raw.strip()).strip("-.")
    return value[:120] or "webtui-self-host"


@dataclasses.dataclass(frozen=True)
class Settings:
    enabled: bool
    database_url: str
    source_provider: str
    source_storage_path: Path
    source_config_path: Path
    work_root: Path
    repository: str
    offsite_endpoint: str
    offsite_bucket: str
    offsite_prefix: str
    region: str
    bucket_lookup: str
    s3_connections: int
    storage_class: str
    ca_cert: str
    access_key: str
    secret_key: str
    session_token: str
    restic_password: str
    restic_password_file: str
    instance_name: str
    timeout_seconds: int
    interval_seconds: int
    run_on_start: bool
    min_free_bytes: int
    staging_headroom_percent: int
    keep_daily: int
    keep_weekly: int
    keep_monthly: int
    keep_yearly: int
    retention_enabled: bool
    verify_after_backup: bool
    verify_subset: str
    compression: str
    include_instance_env: bool
    restore_apply_allowed: bool

    @classmethod
    def from_env(cls, require_remote: bool = True) -> "Settings":
        enabled = env_bool("OFFSITE_BACKUP_ENABLED", False)
        source_provider = os.getenv("STORAGE_PROVIDER", "local").strip().lower()
        if source_provider not in {"local", "minio", "s3"}:
            raise BackupError("STORAGE_PROVIDER must be local, minio, or s3")

        endpoint = ""
        bucket = ""
        prefix = ""
        repository = ""
        access_key = os.getenv("OFFSITE_S3_ACCESS_KEY_ID", "").strip()
        secret_key = os.getenv("OFFSITE_S3_SECRET_ACCESS_KEY", "").strip()
        if require_remote:
            endpoint = validate_endpoint(require_text("OFFSITE_S3_ENDPOINT"))
            bucket = validate_bucket(require_text("OFFSITE_S3_BUCKET"), "OFFSITE_S3_BUCKET")
            prefix = clean_prefix(os.getenv("OFFSITE_S3_PREFIX", "webtui-chat"), "OFFSITE_S3_PREFIX")
            repository = f"s3:{endpoint}/{bucket}"
            if prefix:
                repository += "/" + prefix
            if not access_key or not secret_key:
                raise BackupError("OFFSITE_S3_ACCESS_KEY_ID and OFFSITE_S3_SECRET_ACCESS_KEY are required")

        region = os.getenv("OFFSITE_S3_REGION", "us-east-1").strip() or "us-east-1"
        bucket_lookup = os.getenv("OFFSITE_S3_BUCKET_LOOKUP", "auto").strip().lower()
        if bucket_lookup not in {"auto", "dns", "path"}:
            raise BackupError("OFFSITE_S3_BUCKET_LOOKUP must be auto, dns, or path")
        compression = os.getenv("OFFSITE_BACKUP_COMPRESSION", "auto").strip().lower()
        if compression not in {"auto", "max", "off"}:
            raise BackupError("OFFSITE_BACKUP_COMPRESSION must be auto, max, or off")
        verify_subset = os.getenv("OFFSITE_BACKUP_VERIFY_READ_DATA_SUBSET", "5%").strip()
        if verify_subset and not re.fullmatch(r"(?:100|[1-9]?[0-9])(?:\.[0-9]+)?%|[1-9][0-9]*/[1-9][0-9]*", verify_subset):
            raise BackupError("OFFSITE_BACKUP_VERIFY_READ_DATA_SUBSET must look like 5% or 1/7")

        settings = cls(
            enabled=enabled,
            database_url=os.getenv("DATABASE_URL", "").strip(),
            source_provider=source_provider,
            source_storage_path=Path(os.getenv("SOURCE_STORAGE_PATH", "/source/storage")),
            source_config_path=Path(os.getenv("SOURCE_CONFIG_PATH", "/source/config")),
            work_root=Path(os.getenv("OFFSITE_BACKUP_WORK_ROOT", "/var/lib/vpsttt-chat/backups/offsite")),
            repository=repository,
            offsite_endpoint=endpoint,
            offsite_bucket=bucket,
            offsite_prefix=prefix,
            region=region,
            bucket_lookup=bucket_lookup,
            s3_connections=env_int("OFFSITE_S3_CONNECTIONS", 8, 1, 64),
            storage_class=os.getenv("OFFSITE_S3_STORAGE_CLASS", "").strip(),
            ca_cert=os.getenv("OFFSITE_S3_CA_CERT", "").strip(),
            access_key=access_key,
            secret_key=secret_key,
            session_token=os.getenv("OFFSITE_S3_SESSION_TOKEN", "").strip(),
            restic_password=os.getenv("OFFSITE_RESTIC_PASSWORD", ""),
            restic_password_file=os.getenv("OFFSITE_RESTIC_PASSWORD_FILE", "").strip(),
            instance_name=safe_instance_name(os.getenv("INSTANCE_DOMAIN", os.getenv("APP_NAME", "webtui-self-host"))),
            timeout_seconds=env_int("OFFSITE_BACKUP_TIMEOUT_SECONDS", 7200, 60, 172800),
            interval_seconds=env_int("OFFSITE_BACKUP_INTERVAL_SECONDS", 86400, 300, 2678400),
            run_on_start=env_bool("OFFSITE_BACKUP_RUN_ON_START", False),
            min_free_bytes=env_int("OFFSITE_BACKUP_MIN_FREE_BYTES", 1073741824, 0),
            staging_headroom_percent=env_int("OFFSITE_BACKUP_STAGING_HEADROOM_PERCENT", 20, 0, 500),
            keep_daily=env_int("OFFSITE_BACKUP_KEEP_DAILY", 7, 0, 10000),
            keep_weekly=env_int("OFFSITE_BACKUP_KEEP_WEEKLY", 4, 0, 10000),
            keep_monthly=env_int("OFFSITE_BACKUP_KEEP_MONTHLY", 12, 0, 10000),
            keep_yearly=env_int("OFFSITE_BACKUP_KEEP_YEARLY", 3, 0, 10000),
            retention_enabled=env_bool("OFFSITE_BACKUP_RETENTION_ENABLED", True),
            verify_after_backup=env_bool("OFFSITE_BACKUP_VERIFY_AFTER_BACKUP", False),
            verify_subset=verify_subset,
            compression=compression,
            include_instance_env=env_bool("OFFSITE_BACKUP_INCLUDE_INSTANCE_ENV", False),
            restore_apply_allowed=env_bool("OFFSITE_RESTORE_ALLOW_APPLY", False),
        )
        settings.validate(require_remote=require_remote)
        return settings

    def validate(self, require_remote: bool = True) -> None:
        if not self.work_root.is_absolute() or str(self.work_root) == "/":
            raise BackupError("OFFSITE_BACKUP_WORK_ROOT must be a non-root absolute path")
        if not self.source_storage_path.is_absolute() or str(self.source_storage_path) == "/":
            raise BackupError("SOURCE_STORAGE_PATH must be a non-root absolute path")
        if require_remote:
            if not self.restic_password and not self.restic_password_file:
                raise BackupError("set OFFSITE_RESTIC_PASSWORD_FILE (preferred) or OFFSITE_RESTIC_PASSWORD")
            if self.restic_password_file:
                password_file = Path(self.restic_password_file)
                if not password_file.is_file() or password_file.is_symlink():
                    raise BackupError("OFFSITE_RESTIC_PASSWORD_FILE must be a regular file")
            if self.ca_cert and not Path(self.ca_cert).is_file():
                raise BackupError("OFFSITE_S3_CA_CERT does not exist")
        if self.source_provider in {"minio", "s3"} and require_remote:
            primary_bucket = os.getenv("MINIO_BUCKET", "").strip().lower()
            primary_endpoint = os.getenv("S3_ENDPOINT", "").strip().rstrip("/").lower()
            if primary_bucket and primary_endpoint:
                if primary_bucket == self.offsite_bucket and primary_endpoint == self.offsite_endpoint.lower():
                    raise BackupError("off-site backup must use a bucket separate from primary object storage")

    def restic_env(self) -> dict[str, str]:
        environment = os.environ.copy()
        environment.update(
            {
                "RESTIC_REPOSITORY": self.repository,
                "AWS_ACCESS_KEY_ID": self.access_key,
                "AWS_SECRET_ACCESS_KEY": self.secret_key,
                "AWS_DEFAULT_REGION": self.region,
            }
        )
        if self.session_token:
            environment["AWS_SESSION_TOKEN"] = self.session_token
        else:
            environment.pop("AWS_SESSION_TOKEN", None)
        if self.restic_password_file:
            environment["RESTIC_PASSWORD_FILE"] = self.restic_password_file
            environment.pop("RESTIC_PASSWORD", None)
        else:
            environment["RESTIC_PASSWORD"] = self.restic_password
            environment.pop("RESTIC_PASSWORD_FILE", None)
        return environment

    def restic_base(self) -> list[str]:
        command = ["restic"]
        if self.ca_cert:
            command.extend(["--cacert", self.ca_cert])
        command.extend(["-o", f"s3.region={self.region}"])
        command.extend(["-o", f"s3.bucket-lookup={self.bucket_lookup}"])
        command.extend(["-o", f"s3.connections={self.s3_connections}"])
        if self.storage_class:
            command.extend(["-o", f"s3.storage-class={self.storage_class}"])
        return command

    def redacted_plan(self) -> dict[str, object]:
        return {
            "enabled": self.enabled,
            "repository": self.repository,
            "region": self.region,
            "source_storage_provider": self.source_provider,
            "source_storage_path": str(self.source_storage_path) if self.source_provider == "local" else None,
            "instance": self.instance_name,
            "interval_seconds": self.interval_seconds,
            "retention": {
                "daily": self.keep_daily,
                "weekly": self.keep_weekly,
                "monthly": self.keep_monthly,
                "yearly": self.keep_yearly,
            },
            "compression": self.compression,
            "include_instance_env": self.include_instance_env,
        }


def ensure_enabled(settings: Settings) -> None:
    if not settings.enabled:
        raise BackupError("off-site backup is disabled; set OFFSITE_BACKUP_ENABLED=true explicitly")


def ensure_tools(settings: Settings, include_source: bool = True) -> None:
    tools = ["restic", "pg_dump", "pg_restore", "psql", "dropdb", "createdb"]
    if include_source and settings.source_provider in {"minio", "s3"}:
        tools.append("rclone")
    missing = [tool for tool in tools if shutil.which(tool) is None]
    if missing:
        raise BackupError("missing required commands: " + ", ".join(missing))


def sanitize_output(text: str, settings: Settings) -> str:
    result = text
    for secret in (settings.secret_key, settings.access_key, settings.restic_password, settings.database_url):
        if secret:
            result = result.replace(secret, "[REDACTED]")
    return result


def run_command(
    command: Sequence[str],
    settings: Settings,
    *,
    cwd: Path | None = None,
    environment: Mapping[str, str] | None = None,
    capture: bool = False,
    timeout: int | None = None,
    quiet: bool = False,
) -> subprocess.CompletedProcess[str]:
    if not quiet:
        log(f"running {Path(command[0]).name} {command[1] if len(command) > 1 else ''}".rstrip())
    try:
        result = subprocess.run(
            list(command),
            cwd=str(cwd) if cwd else None,
            env=dict(environment) if environment else None,
            text=True,
            stdout=subprocess.PIPE if capture else None,
            stderr=subprocess.PIPE if capture else None,
            timeout=timeout or settings.timeout_seconds,
            check=False,
        )
    except subprocess.TimeoutExpired as exc:
        raise BackupError(f"{Path(command[0]).name} exceeded the configured timeout") from exc
    if result.returncode != 0:
        detail = sanitize_output((result.stderr or result.stdout or "").strip(), settings)
        if len(detail) > 4000:
            detail = detail[-4000:]
        raise BackupError(f"{Path(command[0]).name} failed ({result.returncode}): {detail or 'no diagnostic output'}")
    return result


class OperationLock:
    def __init__(self, root: Path) -> None:
        self.path = root / "locks" / "offsite-backup.lock"
        self.handle: object | None = None

    def __enter__(self) -> "OperationLock":
        if fcntl is None:
            raise BackupError("operation locking requires Linux; run this command in the backup container")
        self.path.parent.mkdir(parents=True, exist_ok=True)
        self.handle = self.path.open("a+")
        try:
            fcntl.flock(self.handle.fileno(), fcntl.LOCK_EX | fcntl.LOCK_NB)
        except BlockingIOError as exc:
            self.handle.close()
            raise BackupError("another off-site backup/restore operation is already running") from exc
        self.handle.seek(0)
        self.handle.truncate()
        self.handle.write(f"pid={os.getpid()} started={utc_now()}\n")
        self.handle.flush()
        return self

    def __exit__(self, exc_type: object, exc: object, traceback: object) -> None:
        if self.handle is not None:
            fcntl.flock(self.handle.fileno(), fcntl.LOCK_UN)
            self.handle.close()


def utc_now() -> str:
    return dt.datetime.now(dt.timezone.utc).replace(microsecond=0).isoformat().replace("+00:00", "Z")


def sha256_file(path: Path) -> str:
    digest = hashlib.sha256()
    with path.open("rb") as stream:
        while chunk := stream.read(1024 * 1024):
            digest.update(chunk)
    return digest.hexdigest()


def write_json(path: Path, payload: object, mode: int = 0o600) -> None:
    temporary = path.with_name(path.name + ".tmp")
    data = json.dumps(payload, ensure_ascii=False, indent=2, sort_keys=True).encode("utf-8") + b"\n"
    with temporary.open("wb") as stream:
        stream.write(data)
        stream.flush()
        os.fsync(stream.fileno())
    os.chmod(temporary, mode)
    os.replace(temporary, path)


def read_json(path: Path) -> object:
    if path.is_symlink() or not path.is_file():
        raise BackupError(f"required metadata is not a regular file: {path.name}")
    if path.stat().st_size > MAX_METADATA_BYTES:
        raise BackupError(f"metadata file is unexpectedly large: {path.name}")
    try:
        return json.loads(path.read_text(encoding="utf-8"))
    except (OSError, UnicodeDecodeError, json.JSONDecodeError) as exc:
        raise BackupError(f"invalid JSON metadata: {path.name}") from exc


def validate_relative_path(raw: str) -> PurePosixPath:
    if not isinstance(raw, str) or not raw or "\\" in raw or "\x00" in raw:
        raise BackupError("manifest contains an unsafe path")
    path = PurePosixPath(raw)
    if path.is_absolute() or any(part in {"", ".", ".."} for part in path.parts):
        raise BackupError(f"manifest contains path traversal: {raw!r}")
    if path.parts[0] not in {"database.dump", "storage", "config"}:
        raise BackupError(f"manifest path is outside the backup bundle: {raw!r}")
    if path.parts[0] == "database.dump" and len(path.parts) != 1:
        raise BackupError(f"manifest path is invalid: {raw!r}")
    return path


def iter_regular_files(root: Path, allowed_operational: bool = False) -> Iterator[tuple[str, Path]]:
    root = root.resolve(strict=True)
    for current, directories, files in os.walk(root, topdown=True, followlinks=False):
        current_path = Path(current)
        for name in list(directories):
            path = current_path / name
            metadata = path.lstat()
            if stat.S_ISLNK(metadata.st_mode) or not stat.S_ISDIR(metadata.st_mode):
                raise BackupError(f"backup tree contains a symlink or special directory: {path.relative_to(root)}")
        for name in files:
            path = current_path / name
            metadata = path.lstat()
            relative = path.relative_to(root).as_posix()
            if allowed_operational and relative == ".verified.json":
                continue
            if stat.S_ISLNK(metadata.st_mode) or not stat.S_ISREG(metadata.st_mode):
                raise BackupError(f"backup tree contains a symlink or special file: {relative}")
            yield relative, path


def copy_tree_safely(source: Path, destination: Path) -> None:
    source = source.resolve(strict=True)
    if not source.is_dir() or source == Path("/"):
        raise BackupError(f"unsafe source storage directory: {source}")
    destination.mkdir(parents=True, exist_ok=False)
    for current, directories, files in os.walk(source, topdown=True, followlinks=False):
        current_path = Path(current)
        relative_dir = current_path.relative_to(source)
        target_dir = destination / relative_dir
        for name in directories:
            src = current_path / name
            metadata = src.lstat()
            if stat.S_ISLNK(metadata.st_mode) or not stat.S_ISDIR(metadata.st_mode):
                raise BackupError(f"source storage contains an unsupported directory entry: {src}")
            if name.startswith(RESERVED_STORAGE_PREFIX):
                raise BackupError("an unfinished restore workspace exists in primary storage")
            target = target_dir / name
            target.mkdir(exist_ok=False)
            shutil.copystat(src, target, follow_symlinks=False)
        for name in files:
            src = current_path / name
            metadata = src.lstat()
            if stat.S_ISLNK(metadata.st_mode) or not stat.S_ISREG(metadata.st_mode):
                raise BackupError(f"source storage contains an unsupported file entry: {src}")
            shutil.copy2(src, target_dir / name, follow_symlinks=False)


def tree_size(path: Path) -> int:
    total = 0
    for _, file_path in iter_regular_files(path):
        total += file_path.stat().st_size
    return total


def primary_rclone(settings: Settings) -> tuple[dict[str, str], str]:
    endpoint = os.getenv("S3_ENDPOINT", "").strip().rstrip("/")
    bucket = validate_bucket(require_text("MINIO_BUCKET"), "MINIO_BUCKET")
    access_key = require_text("S3_ACCESS_KEY_ID")
    secret_key = require_text("S3_SECRET_ACCESS_KEY")
    prefix = clean_prefix(os.getenv("OFFSITE_SOURCE_S3_PREFIX", ""), "OFFSITE_SOURCE_S3_PREFIX")
    provider = "AWS" if settings.source_provider == "s3" and "amazonaws.com" in endpoint else "Other"
    environment = os.environ.copy()
    environment.update(
        {
            "RCLONE_CONFIG_PRIMARY_TYPE": "s3",
            "RCLONE_CONFIG_PRIMARY_PROVIDER": provider,
            "RCLONE_CONFIG_PRIMARY_ENV_AUTH": "false",
            "RCLONE_CONFIG_PRIMARY_ACCESS_KEY_ID": access_key,
            "RCLONE_CONFIG_PRIMARY_SECRET_ACCESS_KEY": secret_key,
            "RCLONE_CONFIG_PRIMARY_REGION": os.getenv("S3_REGION", "us-east-1").strip() or "us-east-1",
        }
    )
    if endpoint:
        environment["RCLONE_CONFIG_PRIMARY_ENDPOINT"] = endpoint
    remote = f"primary:{bucket}"
    if prefix:
        remote += "/" + prefix
    return environment, remote


def source_storage_size(settings: Settings) -> int:
    if settings.source_provider == "local":
        if not settings.source_storage_path.is_dir():
            raise BackupError(f"source storage does not exist: {settings.source_storage_path}")
        return tree_size(settings.source_storage_path)
    environment, remote = primary_rclone(settings)
    result = run_command(
        ["rclone", "size", remote, "--json"],
        settings,
        environment=environment,
        capture=True,
        quiet=True,
    )
    try:
        payload = json.loads(result.stdout or "{}")
        return int(payload["bytes"])
    except (KeyError, TypeError, ValueError, json.JSONDecodeError) as exc:
        raise BackupError("rclone did not return a usable source size") from exc


def database_environment(settings: Settings) -> tuple[dict[str, str], str]:
    if not settings.database_url:
        raise BackupError("DATABASE_URL is required")
    parsed = urllib.parse.urlsplit(settings.database_url)
    if parsed.scheme not in {"postgres", "postgresql"} or not parsed.hostname:
        raise BackupError("DATABASE_URL must be a PostgreSQL URL")
    database_name = urllib.parse.unquote(parsed.path.lstrip("/"))
    if not re.fullmatch(r"[A-Za-z0-9_-]{1,63}", database_name):
        raise BackupError("DATABASE_URL contains an unsupported database name")
    if database_name in {"postgres", "template0", "template1"}:
        raise BackupError("refusing to operate on a PostgreSQL maintenance database")
    environment = os.environ.copy()
    environment["PGHOST"] = parsed.hostname
    environment["PGPORT"] = str(parsed.port or 5432)
    if parsed.username:
        environment["PGUSER"] = urllib.parse.unquote(parsed.username)
    if parsed.password:
        environment["PGPASSWORD"] = urllib.parse.unquote(parsed.password)
    query = urllib.parse.parse_qs(parsed.query, keep_blank_values=False)
    supported = {
        "sslmode": "PGSSLMODE",
        "sslrootcert": "PGSSLROOTCERT",
        "sslcert": "PGSSLCERT",
        "sslkey": "PGSSLKEY",
    }
    for key, target in supported.items():
        values = query.get(key)
        if values:
            environment[target] = values[-1]
    return environment, database_name


def database_size(settings: Settings) -> int:
    environment, database_name = database_environment(settings)
    result = run_command(
        ["psql", "--no-psqlrc", "--tuples-only", "--no-align", "--dbname", database_name, "--command", "SELECT pg_database_size(current_database())"],
        settings,
        environment=environment,
        capture=True,
        quiet=True,
    )
    try:
        return int((result.stdout or "").strip())
    except ValueError as exc:
        raise BackupError("PostgreSQL did not return a usable database size") from exc


def postgres_version(settings: Settings) -> str:
    environment, database_name = database_environment(settings)
    result = run_command(
        ["psql", "--no-psqlrc", "--tuples-only", "--no-align", "--dbname", database_name, "--command", "SHOW server_version"],
        settings,
        environment=environment,
        capture=True,
        quiet=True,
    )
    return (result.stdout or "unknown").strip()


def record_backup_run_started(settings: Settings) -> str:
    """Create observability state without ever becoming a backup dependency."""
    try:
        environment, database_name = database_environment(settings)
        result = run_command(
            [
                "psql",
                "--no-psqlrc",
                "--set",
                "ON_ERROR_STOP=1",
                "--tuples-only",
                "--no-align",
                "--dbname",
                database_name,
                "--command",
                "INSERT INTO backup_runs (backup_job_id, status, backup_type) "
                "VALUES (NULL, 'running', 'full') RETURNING id::text",
            ],
            settings,
            environment=environment,
            capture=True,
            quiet=True,
        )
        run_id = (result.stdout or "").strip().splitlines()[0].strip()
        uuid.UUID(run_id)
        return run_id
    except Exception as exc:
        log(f"WARNING: could not record backup run start: {sanitize_output(str(exc), settings)}")
        return ""


def finish_backup_run(
    settings: Settings,
    run_id: str,
    *,
    status: str,
    object_key: str = "",
    byte_size: int | None = None,
    checksum: str = "",
    error: str = "",
) -> None:
    """Finish observability state best-effort; preserve the primary outcome."""
    if not run_id:
        return
    if status not in {"success", "failed"}:
        log(f"WARNING: ignored invalid backup telemetry status {status!r}")
        return
    clean_error = error.replace("\x00", " ").strip()[:4000]
    try:
        environment, database_name = database_environment(settings)
        run_command(
            [
                "psql",
                "--no-psqlrc",
                "--set",
                "ON_ERROR_STOP=1",
                "--set",
                f"run_id={run_id}",
                "--set",
                f"status={status}",
                "--set",
                f"object_key={object_key}",
                "--set",
                f"byte_size={'' if byte_size is None else byte_size}",
                "--set",
                f"checksum={checksum}",
                "--set",
                f"error={clean_error}",
                "--dbname",
                database_name,
                "--command",
                "UPDATE backup_runs SET status = :'status', "
                "object_key = NULLIF(:'object_key', ''), "
                "byte_size = NULLIF(:'byte_size', '')::bigint, "
                "checksum_sha256 = NULLIF(:'checksum', ''), "
                "finished_at = now(), error = NULLIF(:'error', '') "
                "WHERE id = :'run_id'::uuid",
            ],
            settings,
            environment=environment,
            capture=True,
            quiet=True,
        )
    except Exception as exc:
        log(f"WARNING: could not finish backup run telemetry: {sanitize_output(str(exc), settings)}")


def check_staging_capacity(settings: Settings, source_bytes: int, database_bytes: int) -> None:
    settings.work_root.mkdir(parents=True, exist_ok=True)
    estimated = source_bytes + database_bytes
    estimated += estimated * settings.staging_headroom_percent // 100
    required = estimated + settings.min_free_bytes
    free = shutil.disk_usage(settings.work_root).free
    if free < required:
        raise BackupError(
            f"not enough staging space: need about {required} bytes, have {free} bytes; "
            "increase the backup_data volume or adjust the documented headroom"
        )


def create_database_dump(settings: Settings, destination: Path) -> None:
    environment, database_name = database_environment(settings)
    command = [
        "pg_dump",
        "--format=custom",
        "--compress=6",
        "--no-owner",
        "--no-privileges",
        "--file",
        str(destination),
        database_name,
    ]
    if env_bool("OFFSITE_BACKUP_SERIALIZABLE_DEFERRABLE", False):
        command.insert(1, "--serializable-deferrable")
    run_command(command, settings, environment=environment, quiet=True)
    if not destination.is_file() or destination.stat().st_size == 0:
        raise BackupError("pg_dump produced an empty archive")
    with destination.open("rb") as stream:
        if stream.read(5) != b"PGDMP":
            raise BackupError("pg_dump output is not a PostgreSQL custom archive")


def snapshot_storage(settings: Settings, destination: Path) -> None:
    if settings.source_provider == "local":
        copy_tree_safely(settings.source_storage_path, destination)
        return
    destination.mkdir(parents=True, exist_ok=False)
    environment, remote = primary_rclone(settings)
    run_command(
        [
            "rclone",
            "copy",
            remote,
            str(destination),
            "--fast-list",
            "--metadata",
            "--transfers",
            str(min(settings.s3_connections, 32)),
        ],
        settings,
        environment=environment,
    )
    list(iter_regular_files(destination))


def snapshot_config(settings: Settings, destination: Path) -> None:
    destination.mkdir(parents=True, exist_ok=False)
    for name in ("compose.yml", "Caddyfile"):
        source = settings.source_config_path / name
        if source.is_file() and not source.is_symlink():
            shutil.copy2(source, destination / name, follow_symlinks=False)
    if settings.include_instance_env:
        source = settings.source_config_path / ".env"
        if not source.is_file() or source.is_symlink():
            raise BackupError("OFFSITE_BACKUP_INCLUDE_INSTANCE_ENV=true but /source/config/.env is unavailable")
        shutil.copy2(source, destination / "instance.env", follow_symlinks=False)


def build_manifest(settings: Settings, stage: Path, pg_version: str) -> dict[str, object]:
    entries: list[dict[str, object]] = []
    for relative, path in iter_regular_files(stage):
        if relative in {"manifest.json", "checksums.json"}:
            continue
        validate_relative_path(relative)
        entries.append({"path": relative, "size": path.stat().st_size, "sha256": sha256_file(path)})
    entries.sort(key=lambda item: str(item["path"]).encode("utf-8"))
    checksums = {"schema_version": SCHEMA_VERSION, "algorithm": "sha256", "files": entries}
    checksums_path = stage / "checksums.json"
    write_json(checksums_path, checksums)
    bundle_id = str(uuid.uuid4())
    manifest: dict[str, object] = {
        "schema_version": SCHEMA_VERSION,
        "bundle_id": bundle_id,
        "created_at": utc_now(),
        "application": "webtui-chat-self-host",
        "application_version": os.getenv("APP_VERSION", "self-hosted").strip() or "self-hosted",
        "instance": settings.instance_name,
        "database": {
            "engine": "postgresql",
            "server_version": pg_version,
            "format": "pg_dump-custom",
            "archive": "database.dump",
            "cluster_globals_included": False,
        },
        "storage": {"provider": settings.source_provider, "root": "storage"},
        "config": {
            "root": "config",
            "instance_env_included": settings.include_instance_env,
            "automatically_applied_on_restore": False,
        },
        "checksums": {
            "algorithm": "sha256",
            "file": "checksums.json",
            "sha256": sha256_file(checksums_path),
        },
        "envelope": {
            "format": "restic-v2",
            "client_side_encryption": True,
            "compression": settings.compression,
        },
    }
    write_json(stage / "manifest.json", manifest)
    return manifest


def verify_bundle(bundle: Path, settings: Settings | None = None, validate_archive: bool = False) -> dict[str, object]:
    bundle = bundle.resolve(strict=True)
    manifest_path = bundle / "manifest.json"
    checksums_path = bundle / "checksums.json"
    manifest_raw = read_json(manifest_path)
    checksums_raw = read_json(checksums_path)
    if not isinstance(manifest_raw, dict) or manifest_raw.get("schema_version") != SCHEMA_VERSION:
        raise BackupError("unsupported backup manifest schema")
    if not isinstance(checksums_raw, dict) or checksums_raw.get("schema_version") != SCHEMA_VERSION:
        raise BackupError("unsupported checksum manifest schema")
    checksum_meta = manifest_raw.get("checksums")
    if not isinstance(checksum_meta, dict) or checksum_meta.get("sha256") != sha256_file(checksums_path):
        raise BackupError("checksum manifest digest mismatch")
    bundle_id = manifest_raw.get("bundle_id")
    try:
        uuid.UUID(str(bundle_id))
    except (ValueError, TypeError) as exc:
        raise BackupError("backup manifest has an invalid bundle_id") from exc

    files_raw = checksums_raw.get("files")
    if not isinstance(files_raw, list):
        raise BackupError("checksum manifest does not contain a file list")
    expected: dict[str, tuple[int, str]] = {}
    for item in files_raw:
        if not isinstance(item, dict):
            raise BackupError("checksum manifest contains an invalid entry")
        relative = validate_relative_path(item.get("path"))
        normalized = relative.as_posix()
        if normalized in expected:
            raise BackupError(f"checksum manifest contains duplicate path: {normalized}")
        size = item.get("size")
        digest = item.get("sha256")
        if not isinstance(size, int) or size < 0 or not isinstance(digest, str) or not re.fullmatch(r"[0-9a-f]{64}", digest):
            raise BackupError(f"checksum metadata is invalid for {normalized}")
        expected[normalized] = (size, digest)
    if "database.dump" not in expected:
        raise BackupError("backup bundle is missing database.dump")

    actual: dict[str, Path] = {}
    for relative, path in iter_regular_files(bundle, allowed_operational=True):
        if relative in {"manifest.json", "checksums.json"}:
            continue
        validate_relative_path(relative)
        actual[relative] = path
    if set(actual) != set(expected):
        missing = sorted(set(expected) - set(actual))[:5]
        extra = sorted(set(actual) - set(expected))[:5]
        raise BackupError(f"backup file inventory mismatch; missing={missing}, extra={extra}")
    for relative, path in actual.items():
        size, digest = expected[relative]
        if path.stat().st_size != size or not hmac.compare_digest(sha256_file(path), digest):
            raise BackupError(f"checksum mismatch: {relative}")
    with (bundle / "database.dump").open("rb") as stream:
        if stream.read(5) != b"PGDMP":
            raise BackupError("database.dump is not a PostgreSQL custom archive")
    if validate_archive:
        if settings is None:
            raise BackupError("settings are required to validate the PostgreSQL archive")
        run_command(["pg_restore", "--list", str(bundle / "database.dump")], settings, capture=True, quiet=True)
    return manifest_raw


def restic_ready(settings: Settings) -> None:
    run_command(settings.restic_base() + ["cat", "config"], settings, environment=settings.restic_env(), capture=True, quiet=True)


def init_repository(settings: Settings) -> None:
    ensure_enabled(settings)
    ensure_tools(settings, include_source=False)
    run_command(settings.restic_base() + ["init"], settings, environment=settings.restic_env())
    restic_ready(settings)
    log("repository initialized; store the Restic password in a second secure location")


def parse_snapshot_id(output: str) -> str:
    snapshot_id = ""
    for line in output.splitlines():
        try:
            payload = json.loads(line)
        except json.JSONDecodeError:
            continue
        if isinstance(payload, dict) and payload.get("message_type") == "summary":
            candidate = payload.get("snapshot_id")
            if isinstance(candidate, str):
                snapshot_id = candidate
    if not SNAPSHOT_RE.fullmatch(snapshot_id):
        raise BackupError("restic did not return a valid snapshot ID")
    return snapshot_id.lower()


def run_retention(settings: Settings, dry_run: bool = False) -> None:
    command = settings.restic_base() + [
        "forget",
        "--tag",
        TAG,
        "--group-by",
        "host,tags",
        "--keep-daily",
        str(settings.keep_daily),
        "--keep-weekly",
        str(settings.keep_weekly),
        "--keep-monthly",
        str(settings.keep_monthly),
        "--keep-yearly",
        str(settings.keep_yearly),
        "--prune",
    ]
    if dry_run:
        command.append("--dry-run")
    run_command(command, settings, environment=settings.restic_env())


def backup_once(settings: Settings, dry_run: bool = False) -> str:
    ensure_enabled(settings)
    ensure_tools(settings)
    settings.work_root.mkdir(parents=True, exist_ok=True)
    if dry_run:
        source_bytes = source_storage_size(settings)
        db_bytes = database_size(settings)
        check_staging_capacity(settings, source_bytes, db_bytes)
        plan = settings.redacted_plan()
        plan["estimated_source_storage_bytes"] = source_bytes
        plan["estimated_database_bytes"] = db_bytes
        plan["remote_mutation"] = False
        print(json.dumps(plan, ensure_ascii=False, indent=2, sort_keys=True))
        return ""

    run_id = record_backup_run_started(settings)
    snapshot_id = ""
    object_key = ""
    bundle_size: int | None = None
    bundle_checksum = ""
    try:
        source_bytes = source_storage_size(settings)
        db_bytes = database_size(settings)
        check_staging_capacity(settings, source_bytes, db_bytes)
        restic_ready(settings)
        staging_root = settings.work_root / "staging"
        staging_root.mkdir(parents=True, exist_ok=True)
        with OperationLock(settings.work_root):
            stage = Path(tempfile.mkdtemp(prefix="backup-", dir=staging_root))
            try:
                log("creating a snapshot-consistent PostgreSQL custom archive")
                create_database_dump(settings, stage / "database.dump")
                log("copying primary file/object storage into immutable staging")
                snapshot_storage(settings, stage / "storage")
                snapshot_config(settings, stage / "config")
                manifest = build_manifest(settings, stage, postgres_version(settings))
                verify_bundle(stage, settings, validate_archive=True)
                bundle_size = tree_size(stage)
                bundle_checksum = sha256_file(stage / "manifest.json")
                result = run_command(
                    settings.restic_base()
                    + [
                        "backup",
                        "--json",
                        "--tag",
                        TAG,
                        "--host",
                        settings.instance_name,
                        "--compression",
                        settings.compression,
                        ".",
                    ],
                    settings,
                    cwd=stage,
                    environment=settings.restic_env(),
                    capture=True,
                )
                if result.stderr:
                    log(sanitize_output(result.stderr.strip(), settings))
                snapshot_id = parse_snapshot_id(result.stdout or "")
                object_key = f"{settings.repository}#{snapshot_id}"
                log(f"snapshot {snapshot_id} uploaded; bundle {manifest['bundle_id']}")
                if settings.retention_enabled:
                    run_retention(settings)
                if settings.verify_after_backup:
                    verify_repository(settings, snapshot_id=None, stage_name=None)
            finally:
                shutil.rmtree(stage, ignore_errors=True)
        finish_backup_run(
            settings,
            run_id,
            status="success",
            object_key=object_key,
            byte_size=bundle_size,
            checksum=bundle_checksum,
        )
        print(snapshot_id)
        return snapshot_id
    except Exception as exc:
        finish_backup_run(
            settings,
            run_id,
            status="failed",
            object_key=object_key,
            byte_size=bundle_size,
            checksum=bundle_checksum,
            error=sanitize_output(str(exc), settings),
        )
        raise


def list_snapshots(settings: Settings, as_json: bool) -> None:
    ensure_enabled(settings)
    ensure_tools(settings, include_source=False)
    command = settings.restic_base() + ["snapshots", "--tag", TAG]
    if as_json:
        command.append("--json")
    run_command(command, settings, environment=settings.restic_env())


def validate_snapshot_id(snapshot_id: str) -> str:
    value = snapshot_id.strip().lower()
    if not SNAPSHOT_RE.fullmatch(value):
        raise BackupError("snapshot ID must contain 8 to 64 hexadecimal characters; 'latest' is intentionally rejected")
    return value


def safe_stage_path(settings: Settings, name: str) -> Path:
    if not SAFE_NAME_RE.fullmatch(name):
        raise BackupError("stage name contains unsafe characters")
    root = settings.work_root.resolve() / "restore-staging"
    root.mkdir(parents=True, exist_ok=True)
    candidate = root / name
    if candidate.parent != root:
        raise BackupError("stage path escapes the restore staging root")
    return candidate


def stage_snapshot(settings: Settings, snapshot_id: str, stage_name: str, replace: bool = False) -> Path:
    ensure_enabled(settings)
    ensure_tools(settings, include_source=False)
    snapshot_id = validate_snapshot_id(snapshot_id)
    target = safe_stage_path(settings, stage_name)
    if target.exists():
        if not replace:
            raise BackupError(f"restore stage already exists: {stage_name}; use --replace-stage explicitly")
        shutil.rmtree(target)
    partial = target.with_name(".partial-" + target.name + "-" + uuid.uuid4().hex)
    restore_root = partial / "restic-restore"
    partial.mkdir(mode=0o700)
    try:
        with OperationLock(settings.work_root):
            run_command(
                settings.restic_base() + ["restore", snapshot_id, "--target", str(restore_root)],
                settings,
                environment=settings.restic_env(),
            )
        candidates = list(restore_root.rglob("manifest.json"))
        if len(candidates) != 1:
            raise BackupError(f"snapshot must contain exactly one manifest.json, found {len(candidates)}")
        bundle = candidates[0].parent
        manifest = verify_bundle(bundle, settings, validate_archive=True)
        prepared = partial / "bundle"
        os.replace(bundle, prepared)
        shutil.rmtree(restore_root, ignore_errors=True)
        marker = {
            "schema_version": SCHEMA_VERSION,
            "snapshot_id": snapshot_id,
            "bundle_id": manifest["bundle_id"],
            "manifest_sha256": sha256_file(prepared / "manifest.json"),
            "verified_at": utc_now(),
        }
        write_json(prepared / ".verified.json", marker)
        os.replace(prepared, target)
        shutil.rmtree(partial, ignore_errors=True)
        log(f"snapshot {snapshot_id} verified and staged at {target}")
        print(str(target))
        return target
    except Exception:
        shutil.rmtree(partial, ignore_errors=True)
        raise


def load_verified_stage(settings: Settings, stage_name: str) -> tuple[Path, dict[str, object], dict[str, object]]:
    stage = safe_stage_path(settings, stage_name)
    if not stage.is_dir() or stage.is_symlink():
        raise BackupError("restore stage does not exist or is unsafe")
    marker_raw = read_json(stage / ".verified.json")
    if not isinstance(marker_raw, dict) or marker_raw.get("schema_version") != SCHEMA_VERSION:
        raise BackupError("restore stage has no valid verification marker")
    manifest = verify_bundle(stage, settings, validate_archive=True)
    if marker_raw.get("bundle_id") != manifest.get("bundle_id"):
        raise BackupError("restore stage bundle ID changed after verification")
    if marker_raw.get("manifest_sha256") != sha256_file(stage / "manifest.json"):
        raise BackupError("restore stage manifest changed after verification")
    if not SNAPSHOT_RE.fullmatch(str(marker_raw.get("snapshot_id", ""))):
        raise BackupError("restore stage snapshot ID is invalid")
    return stage, manifest, marker_raw


def verify_repository(settings: Settings, snapshot_id: str | None, stage_name: str | None) -> None:
    ensure_enabled(settings)
    ensure_tools(settings, include_source=False)
    command = settings.restic_base() + ["check"]
    if settings.verify_subset:
        command.extend(["--read-data-subset", settings.verify_subset])
    run_command(command, settings, environment=settings.restic_env())
    if snapshot_id:
        name = stage_name or f"verify-{snapshot_id}-{uuid.uuid4().hex[:8]}"
        target = stage_snapshot(settings, snapshot_id, name)
        shutil.rmtree(target)
        log(f"snapshot {snapshot_id} passed repository and application checksum verification")


def restore_confirmation(action: str, marker: Mapping[str, object]) -> str:
    return f"{action}:{marker['snapshot_id']}:{marker['bundle_id']}"


def require_apply_permission(settings: Settings) -> None:
    if not settings.restore_apply_allowed:
        raise BackupError("destructive restore is disabled in this container (OFFSITE_RESTORE_ALLOW_APPLY=false)")


def restore_database(settings: Settings, stage_name: str, confirmation: str) -> None:
    require_apply_permission(settings)
    stage, _, marker = load_verified_stage(settings, stage_name)
    expected = restore_confirmation("RESTORE_DB", marker)
    if not hmac.compare_digest(confirmation, expected):
        raise BackupError(f"database restore confirmation mismatch; expected exactly {expected}")
    environment, database_name = database_environment(settings)
    run_command(["pg_restore", "--list", str(stage / "database.dump")], settings, capture=True, quiet=True)
    with OperationLock(settings.work_root):
        log(f"dropping and recreating PostgreSQL database {database_name!r}")
        run_command(
            ["dropdb", "--force", "--if-exists", "--maintenance-db", "postgres", database_name],
            settings,
            environment=environment,
            quiet=True,
        )
        run_command(
            ["createdb", "--maintenance-db", "postgres", database_name],
            settings,
            environment=environment,
            quiet=True,
        )
        run_command(
            [
                "pg_restore",
                "--exit-on-error",
                "--no-owner",
                "--no-privileges",
                "--dbname",
                database_name,
                str(stage / "database.dump"),
            ],
            settings,
            environment=environment,
            quiet=True,
        )
    log(f"database restored from snapshot {marker['snapshot_id']}")


def restore_state_path(settings: Settings, bundle_id: str) -> Path:
    root = settings.work_root / "restore-state"
    root.mkdir(parents=True, exist_ok=True)
    return root / f"{bundle_id}.json"


def activate_local_storage(settings: Settings, stage: Path, marker: Mapping[str, object]) -> None:
    destination = settings.source_storage_path.resolve(strict=True)
    if destination == Path("/") or not destination.is_dir():
        raise BackupError("refusing to restore into an unsafe storage destination")
    for child in destination.iterdir():
        if child.name.startswith(RESERVED_STORAGE_PREFIX):
            raise BackupError("primary storage contains an unfinished restore workspace")
    bundle_id = str(marker["bundle_id"])
    new_dir = destination / f"{RESERVED_STORAGE_PREFIX}new-{bundle_id}"
    old_dir = destination / f"{RESERVED_STORAGE_PREFIX}old-{bundle_id}"
    state_path = restore_state_path(settings, bundle_id)
    copy_tree_safely(stage / "storage", new_dir)
    old_dir.mkdir(mode=0o700)
    moved_old: list[Path] = []
    moved_new: list[Path] = []
    try:
        write_json(
            state_path,
            {
                "schema_version": SCHEMA_VERSION,
                "provider": "local",
                "status": "preparing",
                "bundle_id": bundle_id,
                "snapshot_id": marker["snapshot_id"],
                "old_directory": old_dir.name,
                "activated_at": None,
            },
        )
        for child in list(destination.iterdir()):
            if child in {new_dir, old_dir}:
                continue
            target = old_dir / child.name
            os.replace(child, target)
            moved_old.append(target)
        for child in list(new_dir.iterdir()):
            target = destination / child.name
            os.replace(child, target)
            moved_new.append(target)
        new_dir.rmdir()
        write_json(
            state_path,
            {
                "schema_version": SCHEMA_VERSION,
                "provider": "local",
                "status": "activated",
                "bundle_id": bundle_id,
                "snapshot_id": marker["snapshot_id"],
                "old_directory": old_dir.name,
                "activated_at": utc_now(),
            },
        )
    except Exception:
        new_dir.mkdir(mode=0o700, exist_ok=True)
        for path in reversed(moved_new):
            if path.exists():
                os.replace(path, new_dir / path.name)
        for path in reversed(moved_old):
            if path.exists():
                os.replace(path, destination / path.name)
        shutil.rmtree(new_dir, ignore_errors=True)
        shutil.rmtree(old_dir, ignore_errors=True)
        state_path.unlink(missing_ok=True)
        raise


def sync_primary_s3(settings: Settings, source: Path) -> None:
    environment, remote = primary_rclone(settings)
    run_command(
        [
            "rclone",
            "sync",
            str(source),
            remote,
            "--fast-list",
            "--metadata",
            "--delete-after",
            "--transfers",
            str(min(settings.s3_connections, 32)),
        ],
        settings,
        environment=environment,
    )


def apply_storage(settings: Settings, stage_name: str, confirmation: str) -> None:
    require_apply_permission(settings)
    stage, _, marker = load_verified_stage(settings, stage_name)
    expected = restore_confirmation("RESTORE_STORAGE", marker)
    if not hmac.compare_digest(confirmation, expected):
        raise BackupError(f"storage restore confirmation mismatch; expected exactly {expected}")
    with OperationLock(settings.work_root):
        if settings.source_provider == "local":
            activate_local_storage(settings, stage, marker)
        else:
            sync_primary_s3(settings, stage / "storage")
    log(f"primary storage restored from snapshot {marker['snapshot_id']}")


def rollback_local_storage(settings: Settings, stage_name: str, confirmation: str) -> None:
    require_apply_permission(settings)
    _, _, marker = load_verified_stage(settings, stage_name)
    expected = restore_confirmation("ROLLBACK_STORAGE", marker)
    if not hmac.compare_digest(confirmation, expected):
        raise BackupError(f"storage rollback confirmation mismatch; expected exactly {expected}")
    bundle_id = str(marker["bundle_id"])
    state_path = restore_state_path(settings, bundle_id)
    state = read_json(state_path)
    if (
        not isinstance(state, dict)
        or state.get("bundle_id") != bundle_id
        or state.get("provider") != "local"
        or state.get("status") != "activated"
    ):
        raise BackupError("no valid local storage rollback state exists")
    destination = settings.source_storage_path.resolve(strict=True)
    old_dir = destination / str(state.get("old_directory", ""))
    if old_dir.parent != destination or not old_dir.is_dir() or not old_dir.name.startswith(RESERVED_STORAGE_PREFIX + "old-"):
        raise BackupError("local storage rollback directory is unsafe or missing")
    failed_dir = destination / f"{RESERVED_STORAGE_PREFIX}failed-{bundle_id}"
    failed_dir.mkdir(mode=0o700)
    with OperationLock(settings.work_root):
        for child in list(destination.iterdir()):
            if child in {old_dir, failed_dir}:
                continue
            os.replace(child, failed_dir / child.name)
        for child in list(old_dir.iterdir()):
            os.replace(child, destination / child.name)
        old_dir.rmdir()
    state_path.unlink(missing_ok=True)
    log(f"local storage rolled back; failed restored data retained at {failed_dir}")


def finalize_local_storage(settings: Settings, stage_name: str, confirmation: str) -> None:
    require_apply_permission(settings)
    _, _, marker = load_verified_stage(settings, stage_name)
    expected = restore_confirmation("FINALIZE_STORAGE", marker)
    if not hmac.compare_digest(confirmation, expected):
        raise BackupError(f"storage finalize confirmation mismatch; expected exactly {expected}")
    bundle_id = str(marker["bundle_id"])
    state_path = restore_state_path(settings, bundle_id)
    state = read_json(state_path)
    if (
        not isinstance(state, dict)
        or state.get("bundle_id") != bundle_id
        or state.get("provider") != "local"
        or state.get("status") != "activated"
    ):
        raise BackupError("no valid local storage finalize state exists")
    destination = settings.source_storage_path.resolve(strict=True)
    old_dir = destination / str(state.get("old_directory", ""))
    if old_dir.parent != destination or not old_dir.name.startswith(RESERVED_STORAGE_PREFIX + "old-"):
        raise BackupError("local storage finalize directory is unsafe")
    with OperationLock(settings.work_root):
        shutil.rmtree(old_dir)
        state_path.unlink(missing_ok=True)
    log("previous local storage snapshot removed after successful restore")


def remove_stage(settings: Settings, stage_name: str, confirmation: str) -> None:
    stage, _, marker = load_verified_stage(settings, stage_name)
    expected = restore_confirmation("REMOVE_STAGE", marker)
    if not hmac.compare_digest(confirmation, expected):
        raise BackupError(f"stage removal confirmation mismatch; expected exactly {expected}")
    shutil.rmtree(stage)


def print_stage_confirmation(settings: Settings, stage_name: str, action: str) -> None:
    _, _, marker = load_verified_stage(settings, stage_name)
    allowed = {"RESTORE_DB", "RESTORE_STORAGE", "ROLLBACK_STORAGE", "FINALIZE_STORAGE", "REMOVE_STAGE"}
    normalized = action.strip().upper().replace("-", "_")
    if normalized not in allowed:
        raise BackupError("unsupported confirmation action")
    print(restore_confirmation(normalized, marker))


def schedule(settings: Settings) -> None:
    if not settings.enabled:
        log("OFFSITE_BACKUP_ENABLED=false; scheduler exits without changing the repository")
        return
    ensure_tools(settings)
    stopped = False

    def stop(_signum: int, _frame: object) -> None:
        nonlocal stopped
        stopped = True

    signal.signal(signal.SIGTERM, stop)
    signal.signal(signal.SIGINT, stop)
    if settings.run_on_start:
        try:
            backup_once(settings)
        except BackupError as exc:
            log(f"scheduled backup failed: {exc}")
    while not stopped:
        deadline = time.monotonic() + settings.interval_seconds
        while not stopped and time.monotonic() < deadline:
            time.sleep(min(5, max(0, deadline - time.monotonic())))
        if stopped:
            break
        try:
            backup_once(settings)
        except BackupError as exc:
            log(f"scheduled backup failed: {exc}")


def build_parser() -> argparse.ArgumentParser:
    parser = argparse.ArgumentParser(description="Encrypted S3 off-site backup and guarded restore")
    subparsers = parser.add_subparsers(dest="command", required=True)
    subparsers.add_parser("plan", help="validate and print a redacted plan without remote mutation")
    subparsers.add_parser("storage-provider", help="print the configured primary storage provider")
    subparsers.add_parser("init", help="initialize the configured Restic repository (never automatic)")
    backup_parser = subparsers.add_parser("backup", help="create and upload a complete application backup")
    backup_parser.add_argument("--dry-run", action="store_true")
    list_parser = subparsers.add_parser("list", help="list off-site snapshots")
    list_parser.add_argument("--json", action="store_true")
    prune_parser = subparsers.add_parser("prune", help="apply configured retention and prune unreachable data")
    prune_parser.add_argument("--dry-run", action="store_true")
    verify_parser = subparsers.add_parser("verify", help="check repository data and optionally restore/verify one snapshot")
    verify_parser.add_argument("snapshot", nargs="?")
    verify_parser.add_argument("--stage-name")
    stage_parser = subparsers.add_parser("stage", help="download and verify a snapshot without applying it")
    stage_parser.add_argument("snapshot")
    stage_parser.add_argument("--stage-name", required=True)
    stage_parser.add_argument("--replace-stage", action="store_true")
    confirmation_parser = subparsers.add_parser("confirmation", help="print an internal exact confirmation for a staged operation")
    confirmation_parser.add_argument("stage_name")
    confirmation_parser.add_argument("action")
    restore_db_parser = subparsers.add_parser("restore-db", help="destructively replace PostgreSQL from a verified stage")
    restore_db_parser.add_argument("stage_name")
    restore_db_parser.add_argument("--confirmation", required=True)
    apply_parser = subparsers.add_parser("apply-storage", help="destructively replace primary storage from a verified stage")
    apply_parser.add_argument("stage_name")
    apply_parser.add_argument("--confirmation", required=True)
    rollback_parser = subparsers.add_parser("rollback-storage", help="roll back a local-storage activation")
    rollback_parser.add_argument("stage_name")
    rollback_parser.add_argument("--confirmation", required=True)
    finalize_parser = subparsers.add_parser("finalize-storage", help="remove retained old local storage after health checks")
    finalize_parser.add_argument("stage_name")
    finalize_parser.add_argument("--confirmation", required=True)
    remove_parser = subparsers.add_parser("remove-stage", help="remove a verified restore staging directory")
    remove_parser.add_argument("stage_name")
    remove_parser.add_argument("--confirmation", required=True)
    subparsers.add_parser("schedule", help="run the opt-in interval scheduler")
    return parser


def main(argv: Sequence[str] | None = None) -> int:
    args = build_parser().parse_args(argv)
    try:
        disabled_scheduler = args.command == "schedule" and not env_bool("OFFSITE_BACKUP_ENABLED", False)
        settings = Settings.from_env(require_remote=not disabled_scheduler)
        settings.work_root.mkdir(parents=True, exist_ok=True)
        if args.command == "plan":
            ensure_enabled(settings)
            ensure_tools(settings)
            print(json.dumps(settings.redacted_plan(), ensure_ascii=False, indent=2, sort_keys=True))
        elif args.command == "storage-provider":
            print(settings.source_provider)
        elif args.command == "init":
            init_repository(settings)
        elif args.command == "backup":
            backup_once(settings, dry_run=args.dry_run)
        elif args.command == "list":
            list_snapshots(settings, args.json)
        elif args.command == "prune":
            ensure_enabled(settings)
            ensure_tools(settings, include_source=False)
            run_retention(settings, dry_run=args.dry_run)
        elif args.command == "verify":
            snapshot = validate_snapshot_id(args.snapshot) if args.snapshot else None
            verify_repository(settings, snapshot, args.stage_name)
        elif args.command == "stage":
            stage_snapshot(settings, args.snapshot, args.stage_name, args.replace_stage)
        elif args.command == "confirmation":
            print_stage_confirmation(settings, args.stage_name, args.action)
        elif args.command == "restore-db":
            restore_database(settings, args.stage_name, args.confirmation)
        elif args.command == "apply-storage":
            apply_storage(settings, args.stage_name, args.confirmation)
        elif args.command == "rollback-storage":
            rollback_local_storage(settings, args.stage_name, args.confirmation)
        elif args.command == "finalize-storage":
            finalize_local_storage(settings, args.stage_name, args.confirmation)
        elif args.command == "remove-stage":
            remove_stage(settings, args.stage_name, args.confirmation)
        elif args.command == "schedule":
            schedule(settings)
        else:
            raise BackupError(f"unsupported command: {args.command}")
        return 0
    except BackupError as exc:
        log(f"ERROR: {exc}")
        return 2


if __name__ == "__main__":
    raise SystemExit(main())
