# Nextcloud Talk-inspired modernization

This document tracks the self-host friendly collaboration features added to
WebTui Chat and the remaining production work. The design principle is to keep
chat as the primary work context while avoiding mandatory SaaS dependencies.

## Delivered foundation

- Contextual Workspace panel beside the message timeline:
  - local/private conversation summary;
  - recent files and pinned decisions;
  - task extraction from `TODO:`, `Cần làm:` and Markdown checklists;
  - quick poll creation;
  - per-conversation tags, sensitive mode and important mode;
  - compact density and call-join preferences.
- Poll messages:
  - stored as a normal `event` message with validated metadata;
  - 2–10 options, single or multiple choice, anonymous label and closing time;
  - voting reuses realtime message reactions;
  - JSON import/export for reusable meeting templates;
  - web/mobile creation, rendering and voting.
- Talk collaboration rooms:
  - direct conversations remain private and cannot expose a guest link;
  - a direct conversation can be promoted to an internal group room;
  - internal, public and webinar room modes with password, lobby and guest
    microphone/camera/chat policies;
  - presenter, moderator, member and listener roles;
  - guest links do not expose the media-room key before password/lobby approval;
  - rotating or revoking a guest link rotates the media-room key;
  - breakout rooms only expose their room key to assigned users and moderators.
- Contextual collaboration:
  - versioned collaborative meeting notes and touch/pointer whiteboard;
  - room tasks, assignee/status and message-to-task conversion;
  - private reply from a group message;
  - `@group`, `@all` and `@everyone` are expanded on the backend;
  - web/mobile can bulk-add members from an internal department/user group.
- Privacy and notification controls:
  - sensitive conversations hide sidebar and native-notification previews;
  - important conversations can override web mute/quiet-hours rules;
  - all preferences are currently device-scoped.
- Calling:
  - default microphone/camera state before joining on web and mobile;
  - Android 1:1 screen sharing replaces the active WebRTC video track and runs
    with a media-projection foreground service;
  - zoom and pan for video/screen content;
  - draggable local-speaker camera bubble;
  - warning after 60 minutes;
  - existing WebRTC reconnect, screen sharing and call controls remain intact.

## Next production milestones

### P1 — shared collaboration state

- Persist tags, sensitive/important flags and compact preferences through a
  user-scoped backend API so they follow the account across devices.
- Add an audit event when sensitive or important modes change.
- Apply important-conversation override in the backend push decision, not only
  in the active web client.
- Add mobile poll-template import/export (mobile poll creation is delivered).

### P1 — contextual integrations

- Add a provider interface for Collabora/OnlyOffice document editing.
- Add CalDAV calendar and task adapters.
- Add a safe inline PDF/office viewer with signed, short-lived file URLs.
- Resolve people in file/task comments to the existing call/message action.

### P1 — private AI

- Add a server-side provider interface with Ollama as the default self-hosted
  implementation.
- Summarize only the unread message window and store user-approved summaries,
  not raw prompts.
- Add faster-whisper/Whisper.cpp transcription jobs and timestamped meeting
  transcripts.
- Add per-workspace AI policy, model allowlist, retention and audit controls.

### P2 — conferencing scale and security

- The Jitsi adapter is delivered for group calls while direct calls remain
  peer-to-peer WebRTC. Production deployments still need a hardened
  self-hosted Jitsi secure-domain/JWT configuration.
- Add call recording consent, recording retention and speaking-time metrics.
- Add secure-view watermarking for guest/file sharing.
- Add mobile biometric unlock and screenshot/background privacy policy.
- Design E2EE key management and multi-device recovery before advertising
  message or group-call E2EE.

## Open-source deployment profile

The default lightweight profile should remain PostgreSQL + Redis + object
storage + WebSocket + TURN. Optional collaboration/AI/SFU services must be
feature-gated so small customer installations are not forced to run them.
