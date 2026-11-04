import { useState } from "react";
import { PageSelector } from "./page-selector";
import { ConversationList } from "./conversation-list";
import { MessageThread } from "./message-thread";
import { CustomerPanel } from "./customer-panel";

export function InboxPage() {
  const [pageId, setPageId] = useState<number | null>(null);
  const [convId, setConvId] = useState<number | null>(null);
  const [direction, setDirection] = useState("");

  return (
    <div className="grid h-full grid-cols-[300px_1fr_280px]">
      <aside className="flex flex-col border-r">
        <div className="border-b p-2"><PageSelector value={pageId} onChange={(id) => { setPageId(id); setConvId(null); }} /></div>
        <ConversationList pageId={pageId} selectedId={convId} onSelect={setConvId} />
      </aside>
      <section className="overflow-hidden">
        <MessageThread convId={convId} direction={direction} onDirectionChange={setDirection} />
      </section>
      <aside className="border-l"><CustomerPanel convId={convId} /></aside>
    </div>
  );
}
