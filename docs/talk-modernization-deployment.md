# Talk modernization deployment

The migration `000030_talk_productivity_and_resilience` adds resumable uploads,
scheduled messages, reminders, Thread V2 metadata, Talk Home, meetings, voice
rooms, Breakout V2, recording consent/retention, federation records and
workspace integration flags.

## Upgrade the self-hosted stack

Back up PostgreSQL and object storage first, then rebuild and run migrations:

```sh
cd deploy/self-hosted
docker compose build api worker web admin migrate
docker compose run --rm migrate migrate up
docker compose up -d api worker web admin
```

The worker must be running. It sends scheduled messages/reminders and maintains
expired pins, uploads, meetings, breakout rooms, voice rooms and recordings.

## Local AI with Ollama

Ollama is optional and remains inside the self-hosted network:

```sh
docker compose --profile ai up -d ollama
docker compose exec ollama ollama pull qwen2.5:7b-instruct
```

Enable it for a workspace as an owner:

```sh
curl -X PUT "https://chat.example.com/api/v1/workspaces/WORKSPACE_ID/talk/integrations" \
  -H "Authorization: Bearer ACCESS_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "ai_enabled": true,
    "ai_provider": "ollama",
    "transcription_provider": "faster_whisper",
    "federation_enabled": false,
    "e2ee_calls_enabled": false,
    "sip_enabled": false,
    "bridge_enabled": false,
    "config": {
      "ollama_url": "http://ollama:11434",
      "ollama_model": "qwen2.5:7b-instruct"
    }
  }'
```

Only loopback, private IPs and known local service names are accepted for AI
requests. Add custom internal DNS names to `TALK_AI_ALLOWED_HOSTS`, separated by
commas. Public arbitrary URLs are rejected to limit SSRF risk.

`local_ai` is also supported through an OpenAI-compatible endpoint:

```json
{
  "ai_provider": "local_ai",
  "config": {
    "local_ai_url": "http://local-ai:8080",
    "local_ai_model": "qwen2.5-7b-instruct"
  }
}
```

## Media and advanced providers

- WebRTC 1-1 audio/video and screen sharing use the configured STUN/TURN list.
  DTLS-SRTP transport encryption is active by WebRTC design.
- Group meetings use the configured Jitsi base URL. Recording requires a Jibri
  or external recording worker; the app now provides policy, consent,
  lifecycle, callback and retention control.
- `e2ee_calls_enabled`, SIP, bridge and federation are capability gates. Do not
  advertise them as available until the corresponding audited client/provider
  adapter is deployed. In particular, the E2EE flag alone is not Insertable
  Streams end-to-end media encryption.
- Resumable uploads use 5 MiB chunks, SHA-256 per chunk, a 24-hour resumable
  session and a 2 GiB client limit.

## Verification

```sh
curl -fsS https://chat.example.com/ready
docker compose logs --tail=100 migrate api worker
docker compose exec postgres psql -U "$POSTGRES_USER" -d "$POSTGRES_DB" \
  -c "select version, dirty from schema_migrations;"
```

Test calls with two real devices on different networks. An Android emulator is
useful for UI and messaging, but it is not sufficient for final camera,
microphone, push notification, Bluetooth routing or background incoming-call
validation.
