# UGC moderation, user blocking, and legal acceptance

This module provides the server-side controls required for a production chat
client that distributes user-generated content.

## API contract

All safety endpoints require a bearer token and an active membership in the
workspace named by `{workspace_id}`.

| Endpoint | Purpose |
|---|---|
| `POST /api/v1/workspaces/{workspace_id}/moderation/reports` | Report a visible message or an active workspace user. |
| `GET /api/v1/workspaces/{workspace_id}/moderation/reports` | List the moderation queue; requires `moderation.manage`. |
| `PATCH /api/v1/workspaces/{workspace_id}/moderation/reports/{report_id}` | Move a report through `pending`, `reviewing`, `resolved`, or `dismissed`; requires `moderation.manage`. |
| `GET /api/v1/workspaces/{workspace_id}/blocks` | Return the current member's blocked users so clients can hide their content. |
| `POST /api/v1/workspaces/{workspace_id}/blocks` | Block another active member. Repeating the same request is idempotent. |
| `DELETE /api/v1/workspaces/{workspace_id}/blocks/{blocked_user_id}` | Unblock by user ID. Deleting an absent block is also idempotent. |

Valid report reasons are `spam`, `harassment`, `hate_speech`,
`sexual_content`, `violence`, `illegal_content`, `privacy`, `impersonation`,
and `other`. Details are optional and limited to 2,000 characters.

## Abuse and authorization rules

- A member cannot report or block themselves.
- A message report is accepted only when the reporter can access the message's
  channel. Visible `text`, `file`, `event` (including polls/integrations), and
  `bot` messages are reportable; deleted rows are not.
- Report creation stores an immutable `target_snapshot`. Message evidence is
  limited to optional sender/display IDs, producer kind, message kind,
  timestamp, a 2,000-character body
  excerpt, and a SHA-256 of the original body; attachment bytes and message
  metadata are intentionally excluded. User evidence contains only public
  username/display fields and account creation time.
- Senderless bot/integration reports retain a nullable `target_user_id`; they
  still enter the same queue and can be removed by `message.manage`. Blocking
  remains a user-to-user control, while operators disable an abusive bot,
  webhook, or API token at its source.
- Only one `pending` or `reviewing` report per reporter and target is allowed.
- A user can submit at most 50 reports in a rolling 24-hour window. The global
  authenticated HTTP rate limiter remains an additional boundary.
- Workspace owners and admins receive `moderation.manage` through migration
  `000039_ugc_moderation_and_legal_acceptance`.
- Closing a report as `resolved` or `dismissed` requires a resolution note.
- Report creation/update and block/unblock actions are written to `audit_logs`.

## Block enforcement

The relationship is enforced symmetrically for new interactions: neither side
can create a direct conversation, send/forward/schedule/edit a message, pin or
add a reaction, attach a file to an existing DM message, or start a call while
either side has blocked the other. A block created after a call invite also
hides and terminally rejects a ringing call; accept and WebRTC signaling are
denied, while reject/cancel/hangup remain available for safe cleanup. The
message repository repeats the send check inside its transaction so workers and
scheduled delivery cannot bypass the application-layer policy; a scheduled DM
blocked before delivery is cancelled. Every due scheduled message also
rechecks the sender account status, current `message.send` permission, and both
current legal-document acceptances inside the publishing transaction. A
suspension/deletion of the sender, suspension of the workspace/zone, role
revocation, policy rollover, or block therefore cancels the queued message
instead of publishing it. Transient database errors remain retryable and
state-update errors are surfaced to the worker. Deleting
one's own prior message, unpinning, and removing one's own reaction are
intentionally still allowed so either party can clean up content. Channel
history is retained for evidence and account export. Clients should load
`GET /blocks` and hide content from `blocked_user_id` locally while still
offering report and unblock actions.

## Moderation SLA and removal runbook

The launch operating target is initial triage within 24 hours for normal
reports and within 4 hours for credible threats, child-safety concerns, or
apparently illegal content. Every report should reach `resolved` or
`dismissed` within 72 hours unless an evidence-preservation or legal hold is
documented. These are operational SLOs; the API records timestamps but does
not itself schedule or guarantee the response time, so production monitoring
must alert when pending/reviewing reports approach the limits. The bundled
Prometheus snapshot exports `webtui_moderation_reports`, oldest-open age, and
4h/24h/72h overdue gauges. The optional observability profile includes
Alertmanager rules `WebTuiModerationUrgentTriageOverdue`,
`WebTuiModerationNormalTriageOverdue`, and
`WebTuiModerationClosureOverdue`; configure a real Alertmanager receiver and
complete an alert drill before store submission. These alerts are the minimum
queue consumer for headless self-host deployments; operators still review the
permissioned GET/PATCH API rather than receiving evidence in alert payloads.

1. An operator with `moderation.manage` moves the report to `reviewing` and
   checks the reported target plus necessary conversation context.
2. If a message violates policy, an operator who also has `message.manage`
   calls `DELETE /api/v1/workspaces/{workspace_id}/channels/{channel_id}/messages/{message_id}`.
   Workspace owner/admin system roles have both permissions. Deletion removes
   the message from normal reads/search and emits `MessageDeleted`. The soft-
   deleted message record follows the deployment's general message/database
   retention policy; `MODERATION_EVIDENCE_RETENTION_DAYS` governs only the
   report snapshot and reporter-supplied details, not the source message row.
3. The operator applies any appropriate account action through the existing
   administration workflow, records a concise `resolution_note`, and moves the
   report to `resolved`. Non-violations are moved to `dismissed` with a reason.
4. Operators monitor structured `moderation audit write failed after mutation`
   errors. The API does not make a client retry a committed mutation merely
   because its audit insert failed, avoiding duplicate reports/actions.

There is currently **no automated pre- or post-publication objectionable-content
filter** in this backend. The implemented control is user reporting, a human
moderation queue, permissioned removal, blocking, and interaction enforcement.
Store submissions and user-facing policy text must describe that truthfully;
if automated filtering is a launch requirement, it remains a separate blocker.

## Terms, AUP, and Privacy acknowledgement

`GET /api/v1/auth/legal-documents` returns the current document versions.
Password registration and Google registration accept these fields:

```json
{
  "terms_accepted": true,
  "terms_version": "2026-08-07",
  "privacy_accepted": true,
  "privacy_version": "2026-08-07"
}
```

The Terms version includes the Acceptable Use Policy. Accepted versions,
timestamp, IP address, user agent, workspace, and source are persisted in
`user_legal_acceptances`. `POST /auth/register` requires both acceptances and
the current versions. `POST /auth/google` permits an existing user to log in
without consent fields, but a Google identity that would create a new account
receives HTTP 409 with `LEGAL_ACCEPTANCE_REQUIRED`; the client must show both
documents and retry with the same credential plus all four fields.

`TERMS_VERSION` and `PRIVACY_POLICY_VERSION` configure the advertised and
accepted versions. Production startup fails when either is blank, unsafe, or a
placeholder. Keep these values identical to the versions published by the
portal and increment them whenever the corresponding document changes.

Existing or pre-provisioned users are never treated as having accepted a
document merely because the account predates this migration. After login, a
client calls authenticated
`GET /api/v1/auth/legal-acceptance?workspace_id={selected_workspace_id}`. The
query is optional only for backward compatibility and otherwise falls back to
the workspace in the token. A missing row for either current per-workspace
version returns `complete: false` and the response echoes `workspace_id`. The
client shows both documents and submits authenticated
`POST /api/v1/auth/legal-acceptance` with `workspace_id` plus all four legal
fields; omitting `workspace_id` uses the token workspace. Both endpoints require
an active membership and the requested workspace must belong to the token zone;
the server records the real timestamp, IP, User-Agent, workspace and source
transactionally. Until the status is complete,
UGC-producing profile/avatar, message/file/channel/call, collaboration, and bot
message routes return HTTP 409
`LEGAL_ACCEPTANCE_REQUIRED`. Login/refresh, reads, sync, report/block,
delete/cancel/reject/hangup, acceptance and logout remain available so an
account can recover safely without fabricating consent.

OIDC sign-in remains available for an already linked or pre-provisioned user,
but OIDC JIT creation is fail-closed in this release with
`OIDC_JIT_LEGAL_ACCEPTANCE_REQUIRED`. The current redirect flow does not carry
versioned legal consent transactionally; enable new-user JIT only after that
contract and persistence are implemented and tested.

Public-room guests are also UGC participants even though they do not create an
account. `POST /api/v1/public/conversations/{public_token}/join` therefore
requires the same four current acceptance fields. Migration
`000040_guest_legal_acceptance_evidence` stores the versions, server timestamp,
IP address, and User-Agent on the guest grant. It expires every outstanding
pre-migration grant instead of inferring or backfilling consent. Polling a
legacy/stale grant cannot reveal the private media room key.

## Deployment verification

1. Run migrations 39 and 40 before deploying the new API binary. Confirm
   migration 40 expires pre-existing waiting/approved public guest grants.
2. Confirm owners/admins have `moderation.manage` and ordinary members do not.
3. Exercise report, resolve, block, blocked DM, blocked call, and unblock using
   two non-administrator test accounts.
4. Verify audit events exist without copying message bodies into audit metadata.
5. Restrict snapshot access to `moderation.manage`. Keep snapshots only for the
   published moderation/evidence retention window; they are not general-purpose
   chat history. `MODERATION_EVIDENCE_RETENTION_DAYS` defaults to 365 and must
   match the portal's published security/moderation retention. The worker polls
   every minute and redacts `target_snapshot` plus user-supplied `details` in
   bounded, lock-safe batches once that limit expires. If law requires a longer
   period, change the value and the public policy together before collecting
   data; this release does not provide a per-report legal-hold override.
6. Open a public-room link in a signed-out browser. Confirm the join button is
   disabled until both current policies load and are checked, and confirm the
   resulting `channel_guest_requests` row contains real legal evidence.
