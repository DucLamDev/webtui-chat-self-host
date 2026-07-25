# Prompt Playbook Mobile App WebTui Chat

Tài liệu này tập hợp các prompt nên dùng để triển khai mobile app Flutter của WebTui Chat theo `docs/planning/mobile-flutter-roadmap.md`, tận dụng các agent skill nội bộ và giữ đúng Clean Architecture.

## Cách dùng

- Copy từng prompt vào Codex theo đúng phase đang làm.
- Trước khi chạy prompt UI, bảo đảm ảnh mẫu tồn tại tại `docs/design/mobile/references/webtui-mobile-zalo-reference.png`.
- Luôn yêu cầu agent đọc skill và tài liệu trước khi sửa code.
- Không chạy nhiều phase lớn trong một prompt nếu phase trước chưa qua test/acceptance.
- Sau mỗi phase, dùng prompt review/verification để khóa chất lượng trước khi qua phase tiếp theo.

## Skill cần tận dụng

### Skill thiết kế mobile được phép dùng

Riêng các prompt về thiết kế, visual planning, mockup, reference và audit UI mobile chỉ được dùng các skill dưới đây. Không dùng skill web/desktop/architecture để định hướng giao diện mobile nếu prompt không yêu cầu riêng.

| Skill | Khi dùng |
|---|---|
| `$imagegen-frontend-mobile` | Skill chính cho concept màn hình mobile, flow nhiều màn, design bible, phone mockup, safe area, navigation, readability và screen consistency |
| `$brandkit` | Khi cần chốt nhận diện trước UI: core metaphor, logo direction, palette, typography mood, icon language, texture và brand board |
| `$redesign-existing-projects` | Khi đã có UI Flutter/screenshot và cần audit/fix dấu hiệu generic, spacing yếu, text overflow, state thiếu, component quá web-like |
| `$gpt-taste` | Chỉ dùng như checklist phụ cho premium taste, hierarchy, spacing, bố cục bớt lặp và anti-generic; không áp dụng AIDA/GSAP/landing-page rules vào mobile |

### Skill kỹ thuật dự án

Các prompt triển khai code/architecture vẫn có thể đọc skill nội bộ của dự án nếu phase yêu cầu. Tuy nhiên, phần thiết kế/concept/audit UI mobile trong các prompt đó phải tuân thủ bảng skill thiết kế mobile ở trên.

| Skill | Khi dùng |
|---|---|
| `$webtui-chat-mobile` | Việc Flutter mobile, Android/iOS packaging, CH Play, Firebase App Distribution, APK download |
| `$webtui-chat-architecture` | Việc liên quan Clean Architecture, backend contract, OpenAPI, module backend, deploy, release metadata |
| `$webtui-chat-frontend` | Khi cần thêm web download page, API client web, landing/download page hoặc đối chiếu frontend hiện có |

## Ràng buộc cố định trong mọi prompt

```text
Luôn dùng tiếng Việt có dấu trong UI, log, empty state, toast, lỗi và tài liệu bàn giao.
Không hard-code flow/API order VPSTTT trong mobile; bot/AI phải lấy theo workspace config.
Không gọi trực tiếp LLM provider từ mobile; secret AI nằm ở backend/vault.
Không để domain layer import Flutter, Dio, Drift, Firebase hoặc generated DTO.
Không gọi Dio, Drift, Secure Storage, Firebase hoặc WebSocket trực tiếp trong widget.
Mọi dữ liệu, cache, route, websocket event và notification phải có tenant/workspace context rõ.
Không thêm mock/fallback data vào production flavor.
Khi làm UI mobile, bắt buộc đọc ảnh reference Zalo-like/WebTui trước khi dựng layout và chỉ dùng $brandkit, $gpt-taste, $imagegen-frontend-mobile, $redesign-existing-projects cho phần thiết kế/concept/audit UI mobile.
```

## Prompt 00: Khởi động toàn bộ chương trình mobile

```text
Use $webtui-chat-mobile and $webtui-chat-architecture.

Bạn hãy khởi động chương trình triển khai mobile app Flutter cho WebTui Chat.

Bắt buộc đọc trước:
- docs/planning/mobile-flutter-roadmap.md
- .agents/webtui-chat-mobile/SKILL.md
- docs/design/mobile/references/mobile-ui-reference.md
- .agents/webtui-chat-architecture/SKILL.md
- backend/api/openapi/openapi.yaml nếu cần đối chiếu contract

Mục tiêu:
1. Rà soát repo hiện tại để xác định đã có thư mục mobile/Flutter hay chưa.
2. Lập checklist trạng thái từng phase M0-M13: chưa làm, đang làm, đã xong, bị chặn.
3. Không viết code vội nếu thiếu contract hoặc thiếu ảnh reference UI.
4. Nếu ảnh docs/design/mobile/references/webtui-mobile-zalo-reference.png chưa tồn tại, báo rõ đây là blocker cho phần UI nhưng vẫn có thể làm phần architecture/contract.
5. Đề xuất thứ tự triển khai an toàn nhất trong 3 sprint đầu.

Yêu cầu đầu ra:
- Cập nhật hoặc tạo tài liệu trạng thái nếu cần.
- Nêu rõ file đã đọc, file đã chỉnh, test/verification đã chạy.
- Không sửa unrelated files.
```

## Prompt 01: M0 Contract và mobile readiness

```text
Use $webtui-chat-mobile and $webtui-chat-architecture.

Hoàn thành Phase M0 trong docs/planning/mobile-flutter-roadmap.md cho mobile app Flutter.

Phạm vi:
- Lập ma trận parity web -> mobile.
- Đối chiếu OpenAPI với route Go.
- Liệt kê endpoint thiếu cho mobile: auth, workspace, RBAC, conversations, messages, files, notifications, push devices, sync/catch-up, call signaling, bot/AI config, release metadata.
- Thiết kế idempotency cho message/outbox.
- Thiết kế sync/catch-up cursor.
- Thiết kế device registration và notification preference.
- Chốt Android-first device matrix và privacy/data retention.

Bắt buộc:
- Không tạo endpoint giả không có backend owner.
- Nếu OpenAPI lệch route Go, route Go là nguồn sự thật tạm thời và phải ghi gap.
- Viết tài liệu tiếng Việt có dấu.

Deliverable mong muốn:
- docs/planning/mobile-contract-gap.md hoặc cập nhật roadmap nếu file phù hợp hơn.
- Danh sách API cần bổ sung theo mức P0/P1.
- Acceptance criteria cho M0.
- Không triển khai Flutter UI trong phase này.
```

## Prompt 02: Backend backlog P0 cho mobile

```text
Use $webtui-chat-architecture and $webtui-chat-mobile.

Từ backend backlog MB-1 đến MB-20 trong docs/planning/mobile-flutter-roadmap.md, hãy rà soát backend hiện tại và triển khai các phần P0 cần thiết cho mobile.

Ưu tiên:
1. push_devices register/update/delete
2. notification preference/mute/quiet hours
3. Idempotency-Key/client_message_id
4. sync/event cursor hoặc catch-up contract
5. call session/signaling API và WebSocket event
6. bot installation/config/flow API theo workspace
7. mobile release metadata/version API
8. public download/checksum manifest nếu liên quan APK/Desktop

Ràng buộc:
- Backend theo modular monolith Clean Architecture.
- Domain không phụ thuộc Gin, SQL driver, Redis, RabbitMQ, MinIO/S3.
- Handler chỉ gọi application service/use case.
- Cập nhật OpenAPI và test.
- Log tiếng Việt có dấu, không log secret/token/message content.

Deliverable:
- Code backend cần thiết.
- Migration nếu có.
- OpenAPI cập nhật.
- Unit/integration test.
- Tóm tắt endpoint mới và cách mobile dùng.
```

## Prompt 03: M1 Flutter foundation và Clean Architecture scaffold

```text
Use $webtui-chat-mobile and $webtui-chat-architecture.

Hoàn thành Phase M1: Flutter foundation và design system.

Bắt buộc đọc:
- docs/planning/mobile-flutter-roadmap.md
- docs/design/mobile/references/mobile-ui-reference.md
- .agents/webtui-chat-mobile/SKILL.md

Mục tiêu:
1. Scaffold Flutter app theo cấu trúc feature-first Clean Architecture.
2. Tạo flavors dev/staging/prod, base URL không hard-code trong widget.
3. Thiết lập Riverpod, go_router, Dio/OpenAPI client boundary, Drift foundation, secure storage abstraction.
4. Tạo core Result/Failure, logger redaction, request ID/error mapper.
5. Tạo design tokens từ reference UI: màu, typography, spacing, radius, shadow, list density, bottom tab, segmented control.
6. Tạo lint/review rule để chặn presentation import Dio/Drift/Firebase/generated DTO.
7. Tạo CI nền cho format/analyze/test/build APK debug.

Nếu ảnh reference chưa tồn tại:
- Không dựng UI chi tiết.
- Tạo TODO/blocker rõ trong docs và chỉ làm architecture/foundation.

Acceptance:
- flutter analyze pass.
- Unit test foundation pass.
- Không có widget gọi Dio/Drift trực tiếp.
- Có README kiến trúc mobile trong mobile app.
```

## Prompt 04: M1 Design system bám ảnh Zalo-like/WebTui

```text
Use $webtui-chat-mobile.

Thiết kế design system Flutter cho mobile app dựa trên ảnh:
docs/design/mobile/references/webtui-mobile-zalo-reference.png

Bắt buộc:
- Mở ảnh reference trước khi code.
- Đọc docs/design/mobile/references/mobile-ui-reference.md.
- Không làm landing page.
- Không dùng UI card phình to kiểu marketing.
- Tất cả copy tiếng Việt có dấu.

Component cần có:
- App scaffold mobile.
- Bottom navigation: Tin nhắn, Danh bạ, Khám phá/Kênh, Thêm/Cài đặt.
- Segmented tabs.
- Search bar.
- Conversation list item.
- Channel/bot list item.
- Avatar/status badge.
- Unread badge.
- Toggle, slider, setting row.
- Empty/loading/error states.
- Message bubble base.

Acceptance:
- Screenshot iPhone-like và Android-like.
- So sánh spacing, density, màu, navigation với ảnh reference.
- Không text overflow.
- Không phụ thuộc backend/mock production.
```

## Prompt 04A: Planning thiết kế mobile bằng skill chuyên biệt

```text
Use $brandkit, $gpt-taste, $imagegen-frontend-mobile and $redesign-existing-projects.

Hãy lập kế hoạch thiết kế mobile cho flow/màn: <TÊN_FLOW_HOẶC_MÀN>.

Phạm vi:
- Chỉ planning thiết kế, visual direction, mockup strategy và acceptance UI.
- Không viết code Flutter/React/HTML trong prompt này.
- Không dùng skill ngoài $brandkit, $gpt-taste, $imagegen-frontend-mobile, $redesign-existing-projects cho phần thiết kế/concept/audit UI mobile.

Bắt buộc đọc trước:
- docs/planning/mobile-flutter-roadmap.md, đặc biệt mục 6.1, 6.2, 6.3.
- docs/design/mobile/references/mobile-ui-reference.md.
- Mở ảnh docs/design/mobile/references/webtui-mobile-zalo-reference.png nếu có.
- .agents/skills/imagegen-frontend-mobile/SKILL.md.
- .agents/skills/brandkit/SKILL.md nếu cần chốt nhận diện/palette/icon mood.
- .agents/skills/redesign-existing-projects/SKILL.md nếu đã có UI hoặc screenshot cần audit.
- .agents/skills/gpt-taste/SKILL.md chỉ để lấy checklist anti-generic/hierarchy, không dùng rule AIDA/GSAP/web landing.

Skill routing:
1. $brandkit: dùng trước nếu brand direction, palette, type mood hoặc icon language chưa rõ; nếu đã chốt thì ghi "không cần dùng sâu".
2. $imagegen-frontend-mobile: skill chính, phải khóa platform mode, số màn, design bible, navigation model, safe area, phone mockup, readability và screen consistency.
3. $redesign-existing-projects: chỉ dùng sau khi có UI/screenshot để scan, diagnose và lập danh sách fix; không rewrite từ đầu.
4. $gpt-taste: dùng như checklist phụ để tránh layout generic, text bé, spacing bí, nhịp màn lặp; bỏ qua web-only như hero page, AIDA, GSAP ScrollTrigger, bento desktop.

Đầu ra cần có:
- Bảng skill routing đã dùng và lý do.
- Design bible: platform mode, palette, typography mood, spacing, radius, icon style, texture, navigation, state language.
- Screen flow: thứ tự màn, hành động chuyển màn, state chính, bottom sheet/action sheet cần có.
- Danh sách mockup/image cần generate bằng $imagegen-frontend-mobile, mỗi mockup nêu rõ số màn và mục tiêu.
- Handoff cho Flutter: design tokens cần tạo, component cần dựng, state loading/empty/error, screenshot cần chụp để verify.
- Acceptance checklist: safe area, bottom nav, touch target, keyboard region, text tiếng Việt có dấu, không text overflow, không giống web thu nhỏ, không landing page.
```

## Prompt 05: M2 Auth và secure session

```text
Use $webtui-chat-mobile and $webtui-chat-architecture.

Hoàn thành Phase M2: Auth, secure session và app lock.

Phạm vi:
- Secure token repository: refresh token trong Keychain/Keystore, access token trong memory.
- Login email/username.
- Refresh queue: nhiều request 401 chỉ refresh một lần.
- Logout và clear local state theo policy.
- Session list/revoke.
- Device identity không dùng hardware identifier nhạy cảm.
- Biometric/PIN app lock nếu backend/UX đã sẵn.
- Background screenshot privacy.

Ràng buộc:
- Domain không biết secure storage implementation.
- Data layer implement token source.
- Presentation chỉ gọi use case/controller.
- Không log token, Authorization header hoặc URL chứa token.

Acceptance:
- Unit test auth use case.
- Test refresh race/queue.
- Widget test login loading/error/success.
- Logout xóa token và reset workspace scope.
```

## Prompt 06: M3 Workspace, RBAC, profile

```text
Use $webtui-chat-mobile and $webtui-chat-architecture.

Hoàn thành Phase M3: Workspace, RBAC, profile và settings.

Phạm vi:
- List/select workspace.
- Workspace switch isolation.
- Permission repository từ /rbac/me hoặc contract backend hiện có.
- Profile view/update.
- Avatar camera/gallery/upload.
- Theme/language/notification settings.
- Privacy/session screen.
- Permission denied UX.

Ràng buộc:
- Mọi local cache/query/event phải có workspace_id hoặc session scope rõ.
- Khi đổi workspace phải reset provider scope, websocket subscription, unread counter, cursor và navigation stack.
- UI gate bằng permission code, không suy từ tên role.

Acceptance:
- Không lẫn dữ liệu giữa workspace.
- 403 hiển thị tiếng Việt có dấu và không thành lỗi hệ thống chung.
- Unit test permission logic.
- Widget test workspace switch.
```

## Prompt 07: M4 Conversation, channel và mobile navigation

```text
Use $webtui-chat-mobile.

Hoàn thành Phase M4: Hội thoại, channel và mobile navigation theo ảnh reference Zalo-like/WebTui.

Bắt buộc:
- Mở docs/design/mobile/references/webtui-mobile-zalo-reference.png.
- Bám màn Tin nhắn, Bạn bè, Kênh & Bot, Kênh trong reference.

Phạm vi:
- Conversation list: DM/channel preview, time, unread, avatar/status.
- Tabs Tất cả/Chưa đọc/Yêu thích.
- Search conversation/user/channel.
- Direct conversation create/open, không tạo duplicate.
- Channel public/private/group list.
- Create/join/invite/request channel.
- Channel details/member/pin/media/file/settings.
- Read state/unread badge.
- Back gesture/system back không mất draft.
- Tablet adaptive list-detail.

Acceptance:
- Screenshot phone/tablet.
- Không text overflow.
- Navigation giống app native, không giống web thu nhỏ.
- Dữ liệu lấy từ API/use case, không hard-code mock production.
```

## Prompt 08: M5 Message timeline và realtime

```text
Use $webtui-chat-mobile and $webtui-chat-architecture.

Hoàn thành Phase M5: Tin nhắn và realtime.

Phạm vi:
- Cursor timeline và reverse list không nhảy scroll.
- Composer text/markdown/emoji/mention.
- Draft per conversation.
- Edit/delete/recall theo permission.
- Reaction picker và summary.
- Reply/thread.
- Pin/unpin.
- Forward.
- Message search/filter và jump-to-message.
- WebSocket manager: auth, join/leave, backoff, lifecycle.
- Realtime event reducer idempotent.
- Typing/presence.
- Foreground catch-up.

Ràng buộc:
- Socket callback không update widget trực tiếp.
- Event phải đi qua reducer/application service.
- Message retry dùng client_message_id/idempotency.
- Không log nội dung message.

Acceptance:
- Unit test reducer.
- Multi-device manual/test script: create/update/delete/reaction/read/presence.
- Offline/foreground catch-up không mất hoặc trùng message.
```

## Prompt 09: M6 Media, voice note và audio/video call

```text
Use $webtui-chat-mobile and $webtui-chat-architecture.

Hoàn thành Phase M6: Ảnh, file, camera, voice, video và call.

Phạm vi media:
- Camera/gallery picker.
- File picker MIME/size validation.
- Image resize/compress/EXIF policy.
- Upload queue/progress/cancel/retry.
- Attach file vào message.
- Image viewer, file download/open/share.
- Voice recorder/player.
- Video player.
- Share intent vào WebTui.

Phạm vi call:
- Call domain model: CallSession, CallParticipant, CallState, CallDirection, CallEndReason.
- Call signaling repository.
- Incoming call UX có chuông/rung.
- Outgoing call UX.
- Active audio/video call.
- Camera on/off, switch camera, mic mute, speaker, timer.
- Missed call/incoming answered/outgoing answered card giống luồng chat.
- Call notification bridge với push/background.

Ràng buộc:
- WebRTC/signaling không nằm trong widget.
- Permission camera/mic/photo/file có rationale tiếng Việt.
- File chat nội bộ đi qua backend auth.

Acceptance:
- Test camera/mic permission denied/permanently denied.
- Hai thiết bị gọi được audio/video.
- Bên nhận thấy incoming call.
- Missed call hiển thị trong timeline.
```

## Prompt 10: M7 Push, background và deep link

```text
Use $webtui-chat-mobile and $webtui-chat-architecture.

Hoàn thành Phase M7: Native push, background và deep link.

Phạm vi:
- Firebase project/flavor config dev/staging/prod.
- Register/update/unregister push token.
- Foreground local notification.
- Background/terminated notification.
- Badge count.
- Deep link/universal link/app link mở đúng workspace/channel/message.
- Notification preference/mute/quiet hours/sensitive preview.
- Duplicate suppression.

Backend nếu thiếu:
- Bổ sung worker FCM/APNs, retry/DLQ/delivery log.
- Bổ sung notification target chuẩn.

Acceptance:
- Push foreground/background/terminated pass trên thiết bị thật.
- Không notification trùng khi đang mở đúng channel.
- Deep link không mở nhầm workspace.
- Lock screen preview tôn trọng setting.
```

## Prompt 11: M8 Offline, sync và reliability

```text
Use $webtui-chat-mobile and $webtui-chat-architecture.

Hoàn thành Phase M8: Offline, sync và reliability.

Phạm vi:
- Cache workspace/conversation/message.
- Drift migration/versioning.
- Draft per conversation.
- Message outbox.
- Idempotent retry.
- Attachment outbox.
- Reconnect sync/catch-up.
- Conflict policy server-wins.
- Cache eviction/storage settings.
- Network quality UX.

Ràng buộc:
- Clear cache không xóa outbox/draft nếu chưa gửi.
- Retry không tạo message/attachment trùng.
- App mở offline vẫn xem dữ liệu gần nhất.

Acceptance:
- Test airplane mode.
- Test process death/restart.
- Test token expiry khi offline/online lại.
- Outbox gửi lại thành một message server duy nhất.
```

## Prompt 12: M9 Module nghiệp vụ: phòng ban, bot/AI, automation, ticket, admin

```text
Use $webtui-chat-mobile and $webtui-chat-architecture.

Hoàn thành Phase M9: Module nghiệp vụ đầy đủ.

Phạm vi:
- Phòng ban: cây, chi tiết, member, channel liên kết theo permission.
- Bot catalog theo workspace.
- AI provider/model config qua backend.
- Bot flow mobile-friendly: prompt, trigger, fallback, handoff, approval.
- Tool binding và knowledge source theo workspace.
- Test bot flow.
- Bot audit/version/publish/rollback.
- Automation list/detail/CRUD/run/pause.
- Webhook/API token screens theo quyền, không lộ secret.
- Ticket lifecycle: tạo, phân công, trạng thái, comment, attachment, notification.
- Announcement/system message.
- Workspace admin mobile.
- Super admin mobile.

Ràng buộc:
- Mobile không gọi trực tiếp LLM provider.
- Secret không lưu cache/log/plaintext.
- VPSTTT order chỉ là một tool tùy chọn, không hard-code.
- Permission gate bằng code backend.

Acceptance:
- Không còn placeholder thuộc scope P0.
- Bot/AI config tách theo workspace.
- Ticket module toàn tiếng Việt có dấu.
- Admin/super admin không trộn tenant cache.
```

## Prompt 13: M10 Native UX, accessibility và performance

```text
Use $webtui-chat-mobile.

Hoàn thành Phase M10: Native UX, accessibility và performance.

Phạm vi:
- Permission UX Android/iOS.
- Keyboard/safe area/rotation.
- Accessibility semantics TalkBack/VoiceOver.
- Dynamic font/touch target/contrast.
- Reduced motion.
- Timeline virtualization/performance.
- Image/memory optimization.
- Battery/background optimization.
- Localization.
- App update/version gate.

Acceptance:
- Font lớn không vỡ layout.
- Touch target đạt chuẩn mobile.
- Cuộn timeline lớn mượt trên thiết bị tầm trung.
- Không OOM khi mở gallery nhiều ảnh.
- Không giữ WebSocket/heartbeat trái lifecycle.
- Toàn bộ UI tiếng Việt có dấu.
```

## Prompt 14: M11 Android packaging và internal distribution

```text
Use $webtui-chat-mobile and $webtui-chat-architecture.

Hoàn thành Phase M11: Test, security và Android packaging.

Phạm vi:
- Unit/widget/integration/realtime/offline/security test.
- Device matrix Android.
- Workflow mobile.yml: analyze/test/build/sign/upload artifact.
- Chốt applicationId/package name.
- Android signing setup: upload keystore, CI secret, Play App Signing decision.
- Build signed AAB/APK, versionCode tăng tự động.
- Debug symbols/mapping.
- Firebase App Distribution.
- Direct APK download fallback trên chat.vpsttt.com/download/.
- Mobile update metadata.
- Tài liệu cài Android nội bộ.

Ràng buộc:
- Không commit .jks/password.
- AAB dùng cho CH Play, APK signed dùng cho Firebase/direct download.
- APK public phải có SHA-256 checksum và release notes.

Acceptance:
- Tester nội bộ cài được APK/Firebase.
- CI build pass.
- Có hướng dẫn cài đặt rõ ràng.
```

## Prompt 15: M12 CH Play và kênh tải Android

```text
Use $webtui-chat-mobile and $webtui-chat-architecture.

Hoàn thành Phase M12: CH Play và kênh tải Android.

Phạm vi:
- Tạo Play Console app.
- Play App Signing.
- Target SDK/API compliance theo yêu cầu hiện hành.
- Store listing assets: icon, screenshots, feature graphic, short/full description tiếng Việt có dấu.
- Privacy policy, support email, account deletion/export nếu áp dụng.
- Data safety/content rating.
- Permission declarations.
- Internal testing track.
- Closed/open testing track.
- Pre-launch report và Android vitals.
- Production staged rollout.
- Managed Google Play/private app nếu bán B2B.
- APK download public fallback.
- Landing/download page tự nhận Android/iOS/Desktop.

Ràng buộc:
- Kiểm tra lại yêu cầu Google Play mới nhất trước khi release.
- Không public bản chưa qua security/privacy checklist.

Acceptance:
- Người dùng tải được qua CH Play hoặc link APK có kiểm soát.
- Có plan pause/rollback.
- Crash/ANR monitoring active.
```

## Prompt 16: M13 iOS hardening và release

```text
Use $webtui-chat-mobile and $webtui-chat-architecture.

Hoàn thành Phase M13: iOS hardening và release.

Phạm vi:
- Bundle/signing/provisioning.
- APNs/Firebase push.
- Universal Links.
- Keychain/biometric/privacy screen.
- Camera/mic/photo/file UX.
- Background/lifecycle test.
- Privacy manifest/Store metadata.
- TestFlight internal/external.
- App Store submission/rollout.

Acceptance:
- Build signed trên macOS runner.
- Push foreground/background/terminated pass trên thiết bị thật.
- Universal Link mở đúng workspace/channel/message.
- TestFlight nhóm test xác nhận parity.
- App Store rollout có monitoring và rollback version plan.
```

## Prompt 17: Web download page cho Mobile/Desktop

```text
Use $webtui-chat-frontend, $webtui-chat-mobile and $webtui-chat-architecture.

Tạo hoặc hoàn thiện trang download public cho WebTui Chat.

Mục tiêu:
- Trang tự nhận nền tảng Android/iOS/Desktop nếu có thể.
- Android: ưu tiên CH Play, fallback APK signed tại chat.vpsttt.com/download/.
- iOS: TestFlight/App Store khi có.
- Desktop: Windows/macOS/Linux installer.
- Hiển thị version, release notes, checksum SHA-256, ngày phát hành.
- Link lấy từ backend release metadata/version API, không hard-code trong component nếu backend đã có.

Ràng buộc:
- UI tiếng Việt có dấu.
- Không lộ secret/storage key.
- APK/Desktop installer public qua bucket/download domain riêng.

Acceptance:
- Link tải hoạt động.
- Checksum hiển thị rõ.
- Trạng thái chưa có bản cho nền tảng nào đó hiển thị lịch sự.
```

## Prompt 18: Review Clean Architecture mobile

```text
Use $webtui-chat-mobile and $webtui-chat-architecture.

Hãy review code mobile hiện tại với tư thế code review nghiêm ngặt.

Ưu tiên tìm lỗi:
- Domain import Flutter/Dio/Drift/Firebase/generated DTO.
- Widget gọi trực tiếp API/database/socket/storage.
- DTO lọt lên presentation/use case.
- Thiếu workspace_id trong cache/query/event/notification.
- Secret/token/message content bị log.
- Bot/AI hard-code provider/tool/flow VPSTTT.
- Production flavor có mock/fallback data.
- UI tiếng Việt không dấu.
- UI không bám reference Zalo-like/WebTui.
- Test thiếu cho reducer/outbox/auth/permission.

Yêu cầu trả lời:
- Findings trước, theo severity.
- Gắn file/line cụ thể.
- Nêu test đã chạy hoặc chưa chạy.
- Không tự refactor lớn nếu user chỉ yêu cầu review.
```

## Prompt 19: Fix CI/test fail mobile

```text
Use $webtui-chat-mobile.

CI/test mobile đang fail. Hãy điều tra và sửa đến khi pass.

Quy trình:
1. Đọc log lỗi đầy đủ.
2. Xác định lỗi thuộc build, analyze, unit, widget, integration, signing hay dependency.
3. Sửa nguyên nhân gốc, không skip test tùy tiện.
4. Không chạy lệnh phá hủy hoặc reset worktree.
5. Nếu lỗi do network/dependency registry, báo rõ và xin quyền/escalation nếu cần.

Acceptance:
- flutter analyze pass.
- Test liên quan pass.
- Build target tương ứng pass nếu khả thi.
- Tóm tắt nguyên nhân và file đã sửa.
```

## Prompt 20: Audit UI theo ảnh mẫu

```text
Use $webtui-chat-mobile.

Hãy audit UI mobile hiện tại theo ảnh reference:
docs/design/mobile/references/webtui-mobile-zalo-reference.png

Bắt buộc:
- Mở ảnh reference.
- Chạy app Flutter nếu có thể.
- Chụp screenshot phone nhỏ, phone lớn và tablet nếu có responsive.
- So sánh từng màn với reference: Splash/Login, Tin nhắn, Bạn bè, Kênh & Bot, Kênh, Cài đặt.

Kiểm tra:
- Bottom navigation có đúng tinh thần không.
- Segmented tabs có gọn không.
- List density có quá thưa/quá card-heavy không.
- Màu xanh-trắng có cân bằng không.
- Text có tràn không.
- Icon có mềm, quen thuộc không.
- UI có đang giống web thu nhỏ không.
- Tiếng Việt có dấu đầy đủ không.

Deliverable:
- Danh sách lệch so với reference.
- Fix các lỗi UI rõ ràng nếu user yêu cầu sửa.
- Screenshot/verification sau sửa.
```

## Prompt 21: Prompt triển khai một feature mobile bất kỳ

```text
Use $webtui-chat-mobile and $webtui-chat-architecture.

Triển khai feature: <TÊN_FEATURE>.

Context:
- Phase liên quan: <M0-M13>.
- API liên quan: <endpoint hoặc OpenAPI section>.
- Permission liên quan: <permission code>.
- Workspace scope: <có/không, field cụ thể>.
- UI reference: docs/design/mobile/references/webtui-mobile-zalo-reference.png nếu có UI.

Yêu cầu kiến trúc:
- Domain: entity/value object/failure/repository contract.
- Application: use case và reducer/service nếu có realtime/offline.
- Data: remote/local data source, DTO mapper, repository impl.
- Presentation: controller/state/widget/screen.
- Test: domain/use case/reducer/widget tùy rủi ro.

Ràng buộc:
- Không gọi API/database/socket trực tiếp trong widget.
- Không để DTO lọt ra UI.
- Không hard-code dữ liệu production.
- UI tiếng Việt có dấu.

Acceptance:
- Feature hoạt động theo API thật hoặc empty/error state trung thực nếu API thiếu.
- flutter analyze/test pass.
- Tóm tắt file đã sửa và test đã chạy.
```

## Prompt 22: Prompt hoàn thành một phase và khóa acceptance

```text
Use $webtui-chat-mobile and $webtui-chat-architecture.

Hãy hoàn thành và khóa acceptance cho phase <MÃ_PHASE> trong docs/planning/mobile-flutter-roadmap.md.

Yêu cầu:
1. Đọc toàn bộ task của phase.
2. Rà repo xem task nào đã xong, task nào thiếu.
3. Implement phần thiếu trong phạm vi phase.
4. Chạy verification phù hợp.
5. Cập nhật tài liệu trạng thái nếu cần.
6. Không chuyển sang phase sau nếu acceptance của phase hiện tại chưa đạt.

Đầu ra:
- Checklist task phase với trạng thái.
- File đã sửa.
- Test/command đã chạy.
- Rủi ro còn lại.
- Đề xuất phase tiếp theo.
```

## Prompt 23: Prompt tạo issue/backlog từ roadmap

```text
Use $webtui-chat-mobile.

Từ docs/planning/mobile-flutter-roadmap.md, hãy chuyển phase <MÃ_PHASE> thành backlog issue chi tiết.

Mỗi issue cần có:
- Title ngắn.
- Mục tiêu.
- Scope.
- Out of scope.
- Acceptance criteria.
- Test/verification.
- Dependency.
- Priority P0/P1.
- Skill/tài liệu cần đọc trước.

Không viết code trong prompt này.
```

## Prompt 24: Prompt release readiness tổng thể

```text
Use $webtui-chat-mobile, $webtui-chat-architecture and $webtui-chat-frontend.

Hãy audit release readiness cho mobile app WebTui.

Kiểm tra:
- M0-M13 status.
- Backend MB-1 đến MB-20.
- Security/privacy.
- Push/deep link.
- Offline/outbox.
- Call signaling.
- Bot/AI workspace config.
- CH Play readiness.
- APK direct download readiness.
- iOS TestFlight/App Store readiness.
- Download page/release metadata.
- Monitoring/crash/ANR.

Đầu ra:
- Go/No-go.
- Blocker P0.
- P1 nên xử lý trước public beta.
- Checklist command/test cần chạy.
- Release plan theo từng ngày.
```

## Prompt 25: Prompt handoff cho team dev

```text
Use $webtui-chat-mobile.

Hãy tạo tài liệu handoff cho team dev mobile từ trạng thái hiện tại.

Nội dung:
- Kiến trúc tổng quan.
- Cấu trúc thư mục.
- Cách chạy local.
- Cách chọn environment/flavor.
- Cách đọc API/OpenAPI.
- Cách thêm feature mới theo Clean Architecture.
- Cách làm UI bám ảnh reference.
- Cách chạy test.
- Cách build APK/AAB.
- Cách phát hành Firebase/CH Play.
- Những điều cấm làm.

Tài liệu phải tiếng Việt có dấu, ngắn gọn nhưng đủ để dev mới bắt đầu.
```

## Prompt 26: Prompt yêu cầu Codex tự tiếp tục đến khi xong trong một phase

```text
Use $webtui-chat-mobile and $webtui-chat-architecture.

Bạn hãy tự chủ hoàn thành phase <MÃ_PHASE> end-to-end trong phạm vi hiện tại.

Cách làm:
- Đọc roadmap, skill, reference liên quan.
- Tự khám phá repo trước khi sửa.
- Implement các phần thiếu.
- Chạy test/verification.
- Nếu gặp blocker khách quan, ghi rõ blocker và phần đã làm.
- Không dừng ở đề xuất nếu có thể tự làm.
- Không revert thay đổi không do bạn tạo.
- Không sửa unrelated files.

Kết thúc bằng:
- Tóm tắt thay đổi.
- Test đã chạy.
- File quan trọng.
- Việc còn lại nếu có.
```

## Prompt 27: Prompt kiểm tra ảnh reference trước khi làm UI

```text
Use $webtui-chat-mobile.

Trước khi làm UI mobile, hãy kiểm tra reference UI.

Yêu cầu:
1. Kiểm tra file docs/design/mobile/references/webtui-mobile-zalo-reference.png có tồn tại không.
2. Nếu tồn tại, mở ảnh và mô tả ngắn các pattern cần bám cho màn sắp làm: <TÊN_MÀN>.
3. Nếu không tồn tại, dừng phần UI và yêu cầu đặt ảnh gốc vào đúng path.
4. Không tự dựng UI theo trí nhớ nếu thiếu ảnh.

Sau khi xác nhận reference, mới bắt đầu sửa code UI.
```

## Prompt 28: Prompt kiểm tra không tiếng Việt không dấu

```text
Use $webtui-chat-mobile.

Hãy rà soát mobile app và tài liệu liên quan để tìm tiếng Việt không dấu trong UI/log/docs mới.

Phạm vi:
- mobile app Flutter nếu đã có.
- docs/planning/mobile-flutter-roadmap.md
- docs/design/mobile/references/**
- .agents/webtui-chat-mobile/**
- release/download docs nếu có.

Yêu cầu:
- Sửa các chuỗi tiếng Việt không dấu thành tiếng Việt có dấu.
- Không sửa tên biến/code identifier nếu không cần.
- Không đổi nội dung tiếng Anh kỹ thuật hợp lệ.

Acceptance:
- Báo danh sách file đã sửa.
- Không còn label/empty state/toast/lỗi tiếng Việt không dấu trong phạm vi rà.
```

## Prompt 29: Prompt bảo mật secret và release key

```text
Use $webtui-chat-mobile and $webtui-chat-architecture.

Hãy audit bảo mật secret cho mobile release.

Kiểm tra:
- Android upload keystore có bị commit không.
- Keystore password/key alias/store password có nằm trong repo/log không.
- Firebase service account/Google Play API key có nằm trong repo không.
- S3/MinIO key có bị commit không.
- AI provider key/bot secret có bị lộ ở mobile/web không.
- CI secret dùng đúng GitHub Environment/Secrets chưa.
- Log/crash report có redact token/secret/message content không.

Nếu phát hiện secret thật:
- Không in lại secret trong câu trả lời.
- Yêu cầu rotate secret.
- Sửa repo để dùng placeholder/env example.

Acceptance:
- Danh sách risk.
- Patch loại bỏ secret khỏi file nếu có.
- Hướng dẫn rotate cụ thể.
```

## Prompt 30: Prompt sau mỗi lần cập nhật skill/roadmap

```text
Use $webtui-chat-mobile and $webtui-chat-architecture.

Sau thay đổi quan trọng về mobile app, hãy cập nhật tri thức dự án.

Yêu cầu:
- Nếu thay đổi roadmap, cập nhật docs/planning/mobile-flutter-roadmap.md.
- Nếu thay đổi rule UI hoặc workflow mobile, cập nhật .agents/webtui-chat-mobile/SKILL.md.
- Nếu thay đổi kiến trúc chung, cập nhật .agents/webtui-chat-architecture/SKILL.md hoặc references tương ứng.
- Nếu thay đổi reference UI, cập nhật docs/design/mobile/references/mobile-ui-reference.md.

Không cập nhật skill bằng thông tin tạm thời hoặc chưa chốt.
```
