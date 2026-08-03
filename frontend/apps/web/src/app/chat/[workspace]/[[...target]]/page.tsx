import { ChatEntry } from "@/features/chat/components/chat-entry";

export function generateStaticParams() {
  return [{ target: [], workspace: "desktop" }];
}

export default function ChatRoutePage() {
  return <ChatEntry />;
}
