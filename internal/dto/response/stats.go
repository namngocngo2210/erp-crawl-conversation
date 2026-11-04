package response

import (
	"time"

	"github.com/desi/crawl-conversations/internal/entity"
	reporead "github.com/desi/crawl-conversations/internal/repository/read"
)

type DailyCountResp struct {
	Day   string `json:"day"`
	Count int64  `json:"count"`
}

type PageCountResp struct {
	PageID        int64  `json:"page_id"`
	PageName      string `json:"page_name"`
	Conversations int64  `json:"conversations"`
	Messages      int64  `json:"messages"`
}

type OverviewResp struct {
	Pages         int64            `json:"pages"`
	Conversations int64            `json:"conversations"`
	Messages      int64            `json:"messages"`
	ByDay         []DailyCountResp `json:"by_day"`
	ByPage        []PageCountResp  `json:"by_page"`
}

type CrawlRunResp struct {
	ID         int64      `json:"id"`
	StartedAt  time.Time  `json:"started_at"`
	FinishedAt *time.Time `json:"finished_at"`
	Trigger    string     `json:"trigger"`
	Status     string     `json:"status"`
	Error      string     `json:"error"`
}

// MapOverview maps repository/read types directly — does NOT import business/read to avoid import cycles.
// The handler calls: response.MapOverview(ov.Totals, ov.ByDay, ov.ByPage)
func MapOverview(t reporead.Totals, byDay []reporead.DailyCount, byPage []reporead.PageCountRow) OverviewResp {
	resp := OverviewResp{
		Pages:         t.Pages,
		Conversations: t.Conversations,
		Messages:      t.Messages,
	}
	for _, d := range byDay {
		resp.ByDay = append(resp.ByDay, DailyCountResp{
			Day:   d.Day.Format("2006-01-02"),
			Count: d.Count,
		})
	}
	for _, p := range byPage {
		resp.ByPage = append(resp.ByPage, PageCountResp{
			PageID:        p.PageID,
			PageName:      p.PageName,
			Conversations: p.Conversations,
			Messages:      p.Messages,
		})
	}
	return resp
}

func MapCrawlRuns(runs []entity.CrawlRun) []CrawlRunResp {
	out := make([]CrawlRunResp, 0, len(runs))
	for _, r := range runs {
		out = append(out, CrawlRunResp{
			ID:         r.ID,
			StartedAt:  r.StartedAt,
			FinishedAt: r.FinishedAt,
			Trigger:    r.Trigger,
			Status:     r.Status,
			Error:      r.Error,
		})
	}
	return out
}
