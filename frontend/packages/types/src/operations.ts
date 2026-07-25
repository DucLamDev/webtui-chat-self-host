import type { ISODateTime, Id, JsonObject } from "./api";

export type CronJobStatus = "active" | "disabled" | "paused";

export type CronJobRunner = "builtin_cleanup" | "http" | "script" | "worker";

export type CronJobRunStatus = "cancelled" | "failed" | "running" | "success";

export type CronJob = {
  id: Id;
  name: string;
  description?: string | null;
  schedule: string;
  runner: CronJobRunner | string;
  status: CronJobStatus | string;
  payload: JsonObject;
  last_run_at?: ISODateTime | null;
  next_run_at?: ISODateTime | null;
  locked_at?: ISODateTime | null;
  locked_by?: string | null;
  created_at: ISODateTime;
  updated_at: ISODateTime;
};

export type SaveCronJobInput = {
  name: string;
  description?: string;
  schedule: string;
  runner: CronJobRunner | string;
  status?: CronJobStatus | string;
  payload: JsonObject;
};

export type CronJobRun = {
  id: Id;
  cron_job_id: Id;
  status: CronJobRunStatus | string;
  started_at: ISODateTime;
  finished_at?: ISODateTime | null;
  log?: string | null;
  error?: string | null;
  duration_ms?: number | null;
};

export type BackupJobStatus = "active" | "disabled" | "paused";

export type BackupTarget = "local" | "minio" | "s3";

export type BackupType = "database";

export type BackupRunStatus = "cancelled" | "failed" | "running" | "success";

export type BackupJob = {
  id: Id;
  workspace_id?: Id | null;
  name: string;
  target: BackupTarget | string;
  backup_type: BackupType | string;
  schedule?: string | null;
  status: BackupJobStatus | string;
  config: JsonObject;
  last_run_at?: ISODateTime | null;
  next_run_at?: ISODateTime | null;
  locked_at?: ISODateTime | null;
  locked_by?: string | null;
  created_at?: ISODateTime;
  updated_at?: ISODateTime;
};

export type CreateBackupJobInput = {
  name: string;
  target?: BackupTarget | string;
  backup_type?: BackupType | string;
  schedule?: string;
  status?: BackupJobStatus | string;
  config?: JsonObject;
};

export type BackupRun = {
  id: Id;
  backup_job_id?: Id | null;
  status: BackupRunStatus | string;
  backup_type: BackupType | string;
  object_key?: string | null;
  byte_size?: number | null;
  checksum_sha256?: string | null;
  started_at: ISODateTime;
  finished_at?: ISODateTime | null;
  error?: string | null;
  duration_ms?: number | null;
};
