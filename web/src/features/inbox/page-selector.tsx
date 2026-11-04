import { usePages } from "./use-pages";

export function PageSelector({ value, onChange }: { value: number | null; onChange: (id: number) => void }) {
  const { data, isLoading } = usePages();
  if (isLoading) return <div className="p-2 text-sm text-muted-foreground">Đang tải pages...</div>;
  return (
    <select
      className="w-full rounded-md border px-2 py-1.5 text-sm"
      value={value ?? ""}
      onChange={(e) => onChange(Number(e.target.value))}
    >
      <option value="" disabled>Chọn page…</option>
      {data?.items.map((p) => (
        <option key={p.id} value={p.id}>{p.name} ({p.conversation_count})</option>
      ))}
    </select>
  );
}
