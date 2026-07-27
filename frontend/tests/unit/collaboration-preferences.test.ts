import { describe, expect, it } from "vitest";
import {
  normalizeConversationTags,
  parseCollaborationDisplay,
  parseConversationPreferences
} from "../../apps/web/src/features/chat/model/collaboration-preferences";

describe("collaboration preferences", () => {
  it("normalizes tags and removes duplicates", () => {
    expect(
      normalizeConversationTags([
        "#KháchHàng",
        " kháchhàng ",
        "Dự Án A",
        "",
        42
      ])
    ).toEqual(["KháchHàng", "Dự Án A"]);
  });

  it("parses conversation privacy without trusting malformed entries", () => {
    expect(
      parseConversationPreferences(
        JSON.stringify({
          "channel-1": {
            important: true,
            sensitive: true,
            tags: ["NộiBộ"]
          },
          "channel-2": "invalid"
        })
      )
    ).toEqual({
      "channel-1": {
        important: true,
        sensitive: true,
        tags: ["NộiBộ"]
      }
    });
  });

  it("uses safe call defaults for missing display preferences", () => {
    expect(parseCollaborationDisplay(null)).toEqual({
      callJoin: {
        cameraEnabled: true,
        microphoneEnabled: true
      },
      compactMode: false
    });
  });
});
