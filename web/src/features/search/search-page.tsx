import { useState } from "react";
import { useSearchMessages } from "./use-search";
import { Input } from "@/components/ui/input";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { formatDateTime } from "@/lib/format";
import { Search } from "lucide-react";

export function SearchPage() {
  const [term, setTerm] = useState("");
  const [direction, setDirection] = useState("");
  const [query, setQuery] = useState({ q: "", direction: "" });
  const res = useSearchMessages(query);
  const items = res.data?.pages.flatMap((p) => p.items) ?? [];

  return (
    <div className="mx-auto max-w-3xl p-4">
      <form
        className="mb-4 flex gap-2"
        onSubmit={(e) => { e.preventDefault(); setQuery({ q: term, direction }); }}
      >
        <Input placeholder="Tìm nội dung tin nhắn…" value={term} onChange={(e) => setTerm(e.target.value)} />
        <select className="rounded-md border px-2 text-sm" value={direction} onChange={(e) => setDirection(e.target.value)}>
          <option value="">Tất cả</option>
          <option value="in">Khách gửi</option>
          <option value="out">Trang gửi</option>
        </select>
        <Button type="submit"><Search className="mr-1 h-4 w-4" />Tìm</Button>
      </form>

      {res.isLoading && query.q && <p className="text-sm text-muted-foreground">Đang tìm…</p>}
      {query.q && !res.isLoading && items.length === 0 && <p className="text-sm text-muted-foreground">Không có kết quả.</p>}

      <ul className="space-y-2">
        {items.map((m) => (
          <li key={m.id} className="rounded-md border p-3">
            <div className="mb-1 flex items-center justify-between">
              <Badge variant={m.direction === "out" ? "default" : "secondary"}>
                {m.direction === "out" ? "Trang gửi" : "Khách gửi"}
              </Badge>
              <span className="text-xs text-muted-foreground">{formatDateTime(m.inserted_at)}</span>
            </div>
            <p className="whitespace-pre-wrap break-words text-sm">{m.content}</p>
          </li>
        ))}
      </ul>

      {res.hasNextPage && (
        <Button variant="ghost" className="mt-3" onClick={() => res.fetchNextPage()} disabled={res.isFetchingNextPage}>
          {res.isFetchingNextPage ? "Đang tải…" : "Tải thêm"}
        </Button>
      )}
    </div>
  );
}
