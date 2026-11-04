import { useOverview } from "./use-stats";
import { useCrawlRuns } from "./use-crawl-runs";
import { Card } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { formatDateTime } from "@/lib/format";
import {
  LineChart, Line, XAxis, YAxis, Tooltip, ResponsiveContainer, CartesianGrid,
} from "recharts";

function Stat({ label, value }: { label: string; value: number }) {
  return (
    <Card className="p-4">
      <p className="text-sm text-muted-foreground">{label}</p>
      <p className="text-2xl font-semibold text-primary">{value.toLocaleString("vi-VN")}</p>
    </Card>
  );
}

const statusColor: Record<string, string> = {
  success: "bg-green-100 text-green-700",
  failed: "bg-red-100 text-red-700",
  running: "bg-blue-100 text-blue-700",
};

export function DashboardPage() {
  const ov = useOverview(30);
  const runs = useCrawlRuns(20);

  return (
    <div className="space-y-6 p-6">
      <div className="grid grid-cols-3 gap-4">
        <Stat label="Pages" value={ov.data?.pages ?? 0} />
        <Stat label="Conversations" value={ov.data?.conversations ?? 0} />
        <Stat label="Messages" value={ov.data?.messages ?? 0} />
      </div>

      <Card className="p-4">
        <h2 className="mb-3 text-sm font-medium">Tin nhắn theo ngày (30 ngày)</h2>
        <div className="h-64">
          {(ov.data?.by_day?.length ?? 0) > 0 && (
            <ResponsiveContainer width="100%" height="100%">
              <LineChart data={ov.data?.by_day ?? []}>
                <CartesianGrid strokeDasharray="3 3" />
                <XAxis dataKey="day" fontSize={11} />
                <YAxis fontSize={11} />
                <Tooltip />
                <Line type="monotone" dataKey="count" stroke="hsl(205 90% 48%)" dot={false} />
              </LineChart>
            </ResponsiveContainer>
          )}
        </div>
      </Card>

      <div className="grid grid-cols-2 gap-4">
        <Card className="p-4">
          <h2 className="mb-3 text-sm font-medium">Top page theo tin nhắn</h2>
          <table className="w-full text-sm">
            <thead>
              <tr className="text-left text-muted-foreground">
                <th>Page</th><th>Conv</th><th>Msg</th>
              </tr>
            </thead>
            <tbody>
              {(ov.data?.by_page ?? []).map((p) => (
                <tr key={p.page_id} className="border-t">
                  <td className="py-1">{p.page_name}</td>
                  <td>{p.conversations}</td>
                  <td>{p.messages}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </Card>

        <Card className="p-4">
          <h2 className="mb-3 text-sm font-medium">Lịch sử crawl gần đây</h2>
          <table className="w-full text-sm">
            <thead>
              <tr className="text-left text-muted-foreground">
                <th>Bắt đầu</th><th>Trigger</th><th>Trạng thái</th>
              </tr>
            </thead>
            <tbody>
              {(runs.data?.items ?? []).map((r) => (
                <tr key={r.id} className="border-t">
                  <td className="py-1">{formatDateTime(r.started_at)}</td>
                  <td>{r.trigger}</td>
                  <td>
                    <Badge className={statusColor[r.status] ?? ""}>{r.status}</Badge>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </Card>
      </div>
    </div>
  );
}
