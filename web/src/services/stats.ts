import api from "./client";
import type { Overview, CrawlRun } from "@/types/api";

export async function getOverview(days?: number): Promise<Overview> {
  const { data } = await api.get<Overview>("/stats/overview", { params: { days } });
  return data;
}

export async function getCrawlRuns(limit?: number): Promise<{ items: CrawlRun[] }> {
  const { data } = await api.get<{ items: CrawlRun[] }>("/stats/crawl-runs", { params: { limit } });
  return data;
}
