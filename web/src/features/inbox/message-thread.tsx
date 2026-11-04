import { useMessages } from "./use-messages";
import { formatDateTime } from "@/lib/format";
import { cn } from "@/lib/utils";
import { Button } from "@/components/ui/button";
import type { Message } from "@/types/api";

const FILTERS = [
  { key: "", label: "Tất cả" },
  { key: "in", label: "Khách gửi" },
  { key: "out", label: "Trang gửi" },
];

export function MessageThread({
  convId, direction, onDirectionChange,
}: { convId: number | null; direction: string; onDirectionChange: (d: string) => void }) {
  const q = useMessages(convId, direction);
  if (convId == null) return <div className="flex h-full items-center justify-center text-sm text-muted-foreground">Chọn một hội thoại.</div>;
  const items: Message[] = q.data?.pages.flatMap((p) => p.items) ?? [];
  return (
    <div className="flex h-full flex-col">
      <div className="flex gap-1 border-b p-2">
        {FILTERS.map((f) => (
          <Button key={f.key} size="sm" variant={direction === f.key ? "default" : "ghost"} onClick={() => onDirectionChange(f.key)}>
            {f.label}
          </Button>
        ))}
      </div>
      <div className="flex flex-1 flex-col-reverse overflow-y-auto p-3">
        {items.map((m) => (
          <div key={m.id} className={cn("mb-2 flex", m.direction === "out" ? "justify-end" : "justify-start")}>
            <div className={cn("max-w-[70%] rounded-lg px-3 py-2 text-sm",
              m.direction === "out" ? "bg-primary text-primary-foreground" : "bg-muted")}>
              <p className="whitespace-pre-wrap break-words">{m.content}</p>
              <span className="mt-1 block text-[10px] opacity-70">{formatDateTime(m.inserted_at)}</span>
            </div>
          </div>
        ))}
        {q.hasNextPage && (
          <Button variant="ghost" size="sm" onClick={() => q.fetchNextPage()} disabled={q.isFetchingNextPage}>
            {q.isFetchingNextPage ? "Đang tải…" : "Tải tin cũ hơn"}
          </Button>
        )}
      </div>
    </div>
  );
}
