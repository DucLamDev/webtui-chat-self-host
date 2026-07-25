import { ChatWorkspace } from "@/features/chat/components/chat-workspace";

export function generateStaticParams() {
  return [{ target: [], workspace: "desktop" }];
}

export default function ChatRoutePage() {
  return <ChatWorkspace />;
}
