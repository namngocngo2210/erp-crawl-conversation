import { useConversation } from "./use-conversation";
import { Badge } from "@/components/ui/badge";
import { User, Phone, Mail } from "lucide-react";

export function CustomerPanel({ convId }: { convId: number | null }) {
  const { data } = useConversation(convId);
  if (convId == null || !data) return <div className="p-3 text-sm text-muted-foreground">—</div>;
  const c = data.customer;
  return (
    <div className="space-y-3 p-3 text-sm">
      <div className="flex items-center gap-2 font-medium"><User className="h-4 w-4" />{c?.name || "Khách ẩn danh"}</div>
      {c?.phone && <div className="flex items-center gap-2 text-muted-foreground"><Phone className="h-4 w-4" />{c.phone}</div>}
      {c?.email && <div className="flex items-center gap-2 text-muted-foreground"><Mail className="h-4 w-4" />{c.email}</div>}
      <div className="flex flex-wrap gap-1">
        {data.tags.map((t) => (
          <Badge key={t.id} style={{ backgroundColor: t.color || undefined }}>{t.name}</Badge>
        ))}
      </div>
    </div>
  );
}
