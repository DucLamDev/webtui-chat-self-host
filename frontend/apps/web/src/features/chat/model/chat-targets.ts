import type { ChatChannel, DirectConversation } from "./types";

export type ChatTarget = { id: string; name: string };

export function buildChatTargets(
  channels: Array<Pick<ChatChannel, "id" | "isMember" | "name" | "type">>,
  directConversations: Array<Pick<DirectConversation, "id" | "user">>
): ChatTarget[] {
  const targets = new Map<string, ChatTarget>();

  for (const channel of channels) {
    if (channel.isMember && channel.type !== "direct") {
      targets.set(channel.id, { id: channel.id, name: channel.name });
    }
  }

  for (const conversation of directConversations) {
    targets.set(conversation.id, { id: conversation.id, name: `Chat: ${conversation.user.name}` });
  }

  return [...targets.values()];
}
