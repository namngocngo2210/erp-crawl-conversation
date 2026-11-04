package business

import (
	"context"
	"strings"
	"time"

	"github.com/desi/crawl-conversations/internal/dto"
	"github.com/desi/crawl-conversations/internal/entity"
)

type PageProcessor interface {
	Process(ctx context.Context, page entity.Page) (Stats, error)
}

type pageProcessor struct {
	client  PancakeClient
	tokens  TokenManager
	repos   Repos
	overlap time.Duration
}

func NewPageProcessor(client PancakeClient, tokens TokenManager, repos Repos, overlap time.Duration) PageProcessor {
	return &pageProcessor{client: client, tokens: tokens, repos: repos, overlap: overlap}
}

const (
	conversationWindow      = 29 * 24 * time.Hour      // API requires since..until span < 1 month
	defaultBackfillLookback = 3 * 365 * 24 * time.Hour // used when page.InsertedAt is unknown
)

func (p *pageProcessor) Process(ctx context.Context, page entity.Page) (Stats, error) {
	tok, err := p.tokens.Get(ctx, page.PancakePageID)
	if err != nil {
		return Stats{}, err
	}
	st, err := p.processWithToken(ctx, page, tok)
	if isInvalidTokenErr(err) {
		// invalidate and retry once
		if iErr := p.tokens.Invalidate(ctx, page.PancakePageID); iErr != nil {
			return st, err // return original error if invalidate fails
		}
		tok2, tErr := p.tokens.Get(ctx, page.PancakePageID)
		if tErr != nil {
			return st, err
		}
		return p.processWithToken(ctx, page, tok2)
	}
	return st, err
}

func (p *pageProcessor) processWithToken(ctx context.Context, page entity.Page, tok string) (Stats, error) {
	var st Stats

	// Capture the page's tag catalog (idempotent).
	tags, err := p.client.ListTags(ctx, page.PancakePageID, tok)
	if err != nil {
		return st, err
	}
	for _, t := range tags {
		if _, err := p.repos.Tag.Upsert(ctx, dto.MapTag(page.ID, t)); err != nil {
			return st, err
		}
	}

	cp, found, err := p.repos.Checkpoint.Get(ctx, page.ID)
	if err != nil {
		return st, err
	}
	backfill := !found

	var cursor time.Time
	if found && cp.Cursor != nil {
		cursor = *cp.Cursor
	}
	msgThreshold := cursor.Add(-p.overlap) // only (re)fetch messages for convs updated after this
	maxUpdated := cursor

	// Conversations are filtered by inserted_at and each since..until window must
	// span < 1 month, so walk [start, now] in <=29-day windows for BOTH modes.
	// Incremental does NOT narrow the window by cursor (the cursor is updated_at,
	// the filter is inserted_at) — it relies on the per-conversation updated_at
	// gate below to skip the expensive message fetch for unchanged conversations.
	end := time.Now()
	start := page.InsertedAt
	if start.IsZero() {
		start = end.Add(-defaultBackfillLookback)
	}
	start = start.Add(-24 * time.Hour) // safety margin

	processConv := func(c dto.ConversationDTO) error {
		var custID *int64
		if c.Customer.ID != "" {
			id, err := p.repos.Customer.Upsert(ctx, dto.MapCustomer(page.ID, c.Customer))
			if err != nil {
				return err
			}
			custID = &id
			st.Customers++
		}
		convID, err := p.repos.Conversation.Upsert(ctx, dto.MapConversation(page.ID, custID, c))
		if err != nil {
			return err
		}
		st.Conversations++

		tagIDs := make([]int64, 0, len(c.Tags))
		for _, t := range c.Tags {
			id, err := p.repos.Tag.Upsert(ctx, dto.MapTag(page.ID, t))
			if err != nil {
				return err
			}
			tagIDs = append(tagIDs, id)
		}
		if err := p.repos.Conversation.SetTags(ctx, convID, tagIDs); err != nil {
			return err
		}

		// Expensive step: fetch messages only on backfill, or when the conversation
		// changed since the last run.
		if backfill || c.UpdatedAt.After(msgThreshold) {
			msgs, richCustomers, err := p.fetchMessages(ctx, page, convID, c.ID, tok)
			if err != nil {
				return err
			}
			for _, rc := range richCustomers {
				if c.Customer.ID != "" && rc.ID == c.Customer.ID {
					if _, err := p.repos.Customer.Upsert(ctx, dto.MapCustomer(page.ID, rc)); err != nil {
						return err
					}
				}
			}
			if len(msgs) > 0 {
				if backfill {
					if _, err := p.repos.Message.BulkLoad(ctx, msgs); err != nil {
						return err
					}
				} else {
					if err := p.repos.Message.UpsertBatch(ctx, msgs); err != nil {
						return err
					}
				}
				st.Messages += len(msgs)
			}
		}
		if c.UpdatedAt.After(maxUpdated) {
			maxUpdated = c.UpdatedAt
		}
		return nil
	}

	for winStart := start; winStart.Before(end); winStart = winStart.Add(conversationWindow) {
		winEnd := winStart.Add(conversationWindow)
		if winEnd.After(end) {
			winEnd = end
		}
		ws, we := winStart, winEnd
		pageNum := 1
		for {
			resp, err := p.client.ListConversations(ctx, page.PancakePageID, tok, dto.ListParams{PageNumber: pageNum, Since: &ws, Until: &we})
			if err != nil {
				return st, err
			}
			if len(resp.Conversations) == 0 {
				break
			}
			for _, c := range resp.Conversations {
				if err := processConv(c); err != nil {
					return st, err
				}
			}
			if !resp.HasNext {
				break
			}
			pageNum++
		}
	}

	if !maxUpdated.IsZero() {
		cur := maxUpdated
		if err := p.repos.Checkpoint.Save(ctx, entity.CrawlCheckpoint{
			PageID: page.ID, JobType: "conversations", Cursor: &cur, Status: "ok",
		}); err != nil {
			return st, err
		}
	}
	return st, nil
}

func isInvalidTokenErr(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "invalid access_token") || strings.Contains(msg, "status 401")
}

// fetchMessages walks the full message history via the current_count offset,
// stopping when an empty page is returned. Returns the mapped messages and the
// richer customer list from the first non-empty messages response.
func (p *pageProcessor) fetchMessages(ctx context.Context, page entity.Page, convID int64, pancakeConvID, tok string) ([]entity.Message, []dto.CustomerDTO, error) {
	var out []entity.Message
	var customers []dto.CustomerDTO
	collected := 0
	for {
		resp, err := p.client.ListMessages(ctx, page.PancakePageID, pancakeConvID, tok, dto.ListParams{CurrentCount: collected})
		if err != nil {
			return nil, nil, err
		}
		if len(resp.Messages) == 0 {
			break
		}
		if customers == nil {
			customers = resp.Customers
		}
		for _, m := range resp.Messages {
			out = append(out, dto.MapMessage(page.ID, convID, m))
		}
		collected += len(resp.Messages)
	}
	return out, customers, nil
}
