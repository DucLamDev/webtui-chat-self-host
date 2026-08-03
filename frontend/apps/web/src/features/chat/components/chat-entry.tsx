"use client";

import dynamic from "next/dynamic";

const ChatWorkspace = dynamic(
  () => import("./chat-workspace").then((module) => module.ChatWorkspace),
  { loading: ChatWorkspaceLoading, ssr: false }
);

export function ChatEntry() {
  return <ChatWorkspace />;
}

function ChatWorkspaceLoading() {
  return (
    <main aria-busy="true" aria-label="Đang mở cuộc trò chuyện" className="chat-entry-loading">
      <aside aria-hidden="true" className="chat-entry-loading__rail" />
      <section aria-hidden="true" className="chat-entry-loading__list">
        <i /><i /><i /><i />
      </section>
      <section className="chat-entry-loading__main" role="status">
        <span>Đang mở cuộc trò chuyện…</span>
        <div aria-hidden="true"><i /><i /><i /></div>
      </section>
    </main>
  );
}
