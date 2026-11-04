import { useConversations } from "./use-conversations";
import { formatRelative } from "@/lib/format";
import { cn } from "@/lib/utils";
import { Button } from "@/components/ui/button";

export function ConversationList({
  pageId, selectedId, onSelect,
}: { pageId: number | null; selectedId: number | null; onSelect: (id: number) => void }) {
  const q = useConversations(pageId);
  if (pageId == null) return <div className="p-3 text-sm text-muted-foreground">Chọn page để xem hội thoại.</div>;
  if (q.isLoading) return <div className="p-3 text-sm text-muted-foreground">Đang tải…</div>;
  const items = q.data?.pages.flatMap((p) => p.items) ?? [];
  return (
    <div className="flex h-full flex-col overflow-y-auto">
      {items.map((c) => (
        <button
          key={c.id}
          onClick={() => onSelect(c.id)}
          className={cn("border-b px-3 py-2 text-left hover:bg-muted", selectedId === c.id && "bg-muted")}
        >
          <div className="flex justify-between">
            <span className="truncate text-sm font-medium">#{c.pancake_conversation_id}</span>
            <span className="text-xs text-muted-foreground">{formatRelative(c.last_message_at)}</span>
          </div>
          <p className="truncate text-xs text-muted-foreground">{c.snippet}</p>
        </button>
      ))}
      {q.hasNextPage && (
        <Button variant="ghost" size="sm" onClick={() => q.fetchNextPage()} disabled={q.isFetchingNextPage}>
          {q.isFetchingNextPage ? "Đang tải…" : "Tải thêm"}
        </Button>
      )}
    </div>
  );
}
