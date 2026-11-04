package pancake

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/desi/crawl-conversations/internal/dto"
)

type Client interface {
	ListPages(ctx context.Context) ([]dto.PageDTO, error)
	GeneratePageToken(ctx context.Context, pageID string) (string, error)
	ListConversations(ctx context.Context, pageID, pageToken string, p dto.ListParams) (dto.ConversationsResponse, error)
	ListMessages(ctx context.Context, pageID, conversationID, pageToken string, p dto.ListParams) (dto.MessagesResponse, error)
	ListTags(ctx context.Context, pageID, pageToken string) ([]dto.TagDTO, error)
}

const conversationsPageSize = 200

type client struct {
	pageBaseURL    string
	accountBaseURL string
	accessToken    string
	rl             *RateLimiter
	hc             *http.Client
}

func NewClient(pageBaseURL, accountBaseURL, accessToken string, rl *RateLimiter, hc *http.Client) Client {
	if hc == nil {
		hc = &http.Client{Timeout: 30 * time.Second}
	}
	return &client{pageBaseURL: pageBaseURL, accountBaseURL: accountBaseURL, accessToken: accessToken, rl: rl, hc: hc}
}

// doJSON issues an HTTP request with retry on 429/5xx and decodes JSON into out.
// pageID (when non-empty) drives the per-page rate limiter. body/contentType are optional.
func (c *client) doJSON(ctx context.Context, pageID, method, fullURL string, body []byte, contentType string, out any) error {
	const maxAttempts = 4
	var lastErr error
	for attempt := 0; attempt < maxAttempts; attempt++ {
		if pageID != "" {
			if err := c.rl.Wait(ctx, pageID); err != nil {
				return err
			}
		}
		var reader io.Reader
		if body != nil {
			reader = bytes.NewReader(body)
		}
		req, err := http.NewRequestWithContext(ctx, method, fullURL, reader)
		if err != nil {
			return err
		}
		if contentType != "" {
			req.Header.Set("Content-Type", contentType)
		}
		resp, err := c.hc.Do(req)
		if err != nil {
			lastErr = err
			backoff(ctx, attempt)
			continue
		}
		respBody, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		if resp.StatusCode == 429 || resp.StatusCode >= 500 {
			lastErr = fmt.Errorf("pancake status %d: %s", resp.StatusCode, string(respBody))
			backoff(ctx, attempt)
			continue
		}
		if resp.StatusCode >= 400 {
			return fmt.Errorf("pancake status %d: %s", resp.StatusCode, string(respBody))
		}
		return json.Unmarshal(respBody, out)
	}
	return lastErr
}

func backoff(ctx context.Context, attempt int) {
	d := time.Duration(1<<attempt) * 200 * time.Millisecond
	select {
	case <-ctx.Done():
	case <-time.After(d):
	}
}

func (c *client) ListPages(ctx context.Context) ([]dto.PageDTO, error) {
	u := fmt.Sprintf("%s/pages?access_token=%s", c.accountBaseURL, url.QueryEscape(c.accessToken))
	var raw struct {
		Success     *bool  `json:"success"`
		Message     string `json:"message"`
		Categorized struct {
			Activated []struct {
				ID         string          `json:"id"`
				Name       string          `json:"name"`
				Platform   string          `json:"platform"`
				InsertedAt dto.PancakeTime `json:"inserted_at"`
			} `json:"activated"`
		} `json:"categorized"`
	}
	if err := c.doJSON(ctx, "", http.MethodGet, u, nil, "", &raw); err != nil {
		return nil, err
	}
	if raw.Success != nil && !*raw.Success {
		return nil, fmt.Errorf("pancake pages error: %s", raw.Message)
	}
	out := make([]dto.PageDTO, 0, len(raw.Categorized.Activated))
	for _, p := range raw.Categorized.Activated {
		out = append(out, dto.PageDTO{ID: p.ID, Name: p.Name, Platform: p.Platform, InsertedAt: p.InsertedAt.Time})
	}
	return out, nil
}

func (c *client) GeneratePageToken(ctx context.Context, pageID string) (string, error) {
	u := fmt.Sprintf("%s/pages/%s/generate_page_access_token?access_token=%s",
		c.accountBaseURL, url.PathEscape(pageID), url.QueryEscape(c.accessToken))
	var raw struct {
		PageAccessToken string `json:"page_access_token"`
		Success         bool   `json:"success"`
		Message         string `json:"message"`
	}
	if err := c.doJSON(ctx, "", http.MethodPost, u, []byte(`{}`), "application/json", &raw); err != nil {
		return "", err
	}
	if raw.PageAccessToken == "" {
		return "", fmt.Errorf("generate page token failed for %s: %s", pageID, raw.Message)
	}
	return raw.PageAccessToken, nil
}

type rawTag struct {
	ID    int    `json:"id"`
	Text  string `json:"text"`
	Color string `json:"color"`
}

// flexContact tolerates a JSON value that is EITHER a plain string
// (e.g. "0900...") OR an object (e.g. {"phone_number":"0900...","type":"..."}).
// The Pancake messages endpoint returns recent_phone_numbers / recent_emails in
// both shapes depending on the page, so a plain []string fails to unmarshal.
type flexContact struct{ Value string }

func (f *flexContact) UnmarshalJSON(b []byte) error {
	b = bytes.TrimSpace(b)
	if len(b) == 0 || string(b) == "null" {
		return nil
	}
	if b[0] == '"' {
		return json.Unmarshal(b, &f.Value)
	}
	if b[0] == '{' {
		var obj struct {
			PhoneNumber string `json:"phone_number"`
			Number      string `json:"number"`
			Phone       string `json:"phone"`
			Email       string `json:"email"`
			Address     string `json:"address"`
			Value       string `json:"value"`
		}
		if err := json.Unmarshal(b, &obj); err != nil {
			return nil // unknown object shape: skip rather than fail the whole page
		}
		for _, v := range []string{obj.PhoneNumber, obj.Number, obj.Phone, obj.Email, obj.Address, obj.Value} {
			if v != "" {
				f.Value = v
				break
			}
		}
		return nil
	}
	return nil // numbers/arrays/other: ignore, don't fail
}

func firstContact(list []flexContact) string {
	for _, c := range list {
		if c.Value != "" {
			return c.Value
		}
	}
	return ""
}

func (c *client) ListConversations(ctx context.Context, pageID, pageToken string, p dto.ListParams) (dto.ConversationsResponse, error) {
	since := int64(0)
	if p.Since != nil {
		since = p.Since.Unix()
	}
	until := time.Now().Unix()
	if p.Until != nil {
		until = p.Until.Unix()
	}
	pageNum := p.PageNumber
	if pageNum < 1 {
		pageNum = 1
	}
	q := url.Values{}
	q.Set("page_access_token", pageToken)
	q.Set("page_number", strconv.Itoa(pageNum))
	q.Set("since", strconv.FormatInt(since, 10))
	q.Set("until", strconv.FormatInt(until, 10))
	q.Set("sort_by", "updated_at")
	u := fmt.Sprintf("%s/pages/%s/conversations?%s", c.pageBaseURL, url.PathEscape(pageID), q.Encode())

	var raw struct {
		Success       *bool  `json:"success"`
		Message       string `json:"message"`
		Total         int    `json:"total"`
		Conversations []struct {
			ID           string          `json:"id"`
			Type         string          `json:"type"`
			Snippet      string          `json:"snippet"`
			MessageCount int             `json:"message_count"`
			UpdatedAt    dto.PancakeTime `json:"updated_at"`
			InsertedAt   dto.PancakeTime `json:"inserted_at"`
			Tags         []rawTag        `json:"tags"`
			Customers    []struct {
				ID   string `json:"id"`
				FbID string `json:"fb_id"`
				Name string `json:"name"`
			} `json:"customers"`
			PageCustomer struct {
				Gender string `json:"gender"`
			} `json:"page_customer"`
		} `json:"conversations"`
	}
	if err := c.doJSON(ctx, pageID, http.MethodGet, u, nil, "", &raw); err != nil {
		return dto.ConversationsResponse{}, err
	}
	// Pancake returns HTTP 200 with success:false for logical errors
	// (e.g. "date range must be less than 1 month!"). Surface it as an error
	// instead of silently returning an empty list.
	if raw.Success != nil && !*raw.Success {
		return dto.ConversationsResponse{}, fmt.Errorf("pancake conversations error (page %s): %s", pageID, raw.Message)
	}
	out := dto.ConversationsResponse{Total: raw.Total, HasNext: pageNum*conversationsPageSize < raw.Total}
	for _, cv := range raw.Conversations {
		cust := dto.CustomerDTO{Gender: cv.PageCustomer.Gender}
		if len(cv.Customers) > 0 {
			cust.ID = cv.Customers[0].ID
			cust.FbID = cv.Customers[0].FbID
			cust.Name = cv.Customers[0].Name
		}
		tags := make([]dto.TagDTO, 0, len(cv.Tags))
		for _, t := range cv.Tags {
			tags = append(tags, dto.TagDTO{ID: t.ID, Name: t.Text, Color: t.Color})
		}
		out.Conversations = append(out.Conversations, dto.ConversationDTO{
			ID: cv.ID, Type: cv.Type, Snippet: cv.Snippet, MessageCount: cv.MessageCount,
			UpdatedAt: cv.UpdatedAt.Time, InsertedAt: cv.InsertedAt.Time, Customer: cust, Tags: tags,
		})
	}
	return out, nil
}

func (c *client) ListMessages(ctx context.Context, pageID, conversationID, pageToken string, p dto.ListParams) (dto.MessagesResponse, error) {
	q := url.Values{}
	q.Set("page_access_token", pageToken)
	q.Set("current_count", strconv.Itoa(p.CurrentCount))
	u := fmt.Sprintf("%s/pages/%s/conversations/%s/messages?%s",
		c.pageBaseURL, url.PathEscape(pageID), url.PathEscape(conversationID), q.Encode())

	var raw struct {
		Success      *bool    `json:"success"`
		Message      string   `json:"message"`
		RecentEmails []flexContact `json:"recent_emails"`
		Messages     []struct {
			ID         string          `json:"id"`
			Type       string          `json:"type"`
			Message    string          `json:"message"`
			Original   string          `json:"original_message"`
			InsertedAt dto.PancakeTime `json:"inserted_at"`
			From       struct {
				ID string `json:"id"`
			} `json:"from"`
			Attachments json.RawMessage `json:"attachments"`
		} `json:"messages"`
		Customers []struct {
			ID           string   `json:"id"`
			Name         string   `json:"name"`
			FbID         string   `json:"fb_id"`
			Phones       []flexContact `json:"recent_phone_numbers"`
			PersonalInfo struct {
				Gender string `json:"gender"`
			} `json:"personal_info"`
		} `json:"customers"`
	}
	if err := c.doJSON(ctx, pageID, http.MethodGet, u, nil, "", &raw); err != nil {
		return dto.MessagesResponse{}, err
	}
	if raw.Success != nil && !*raw.Success {
		return dto.MessagesResponse{}, fmt.Errorf("pancake messages error (page %s, conv %s): %s", pageID, conversationID, raw.Message)
	}
	out := dto.MessagesResponse{}
	for _, m := range raw.Messages {
		content := m.Original
		if content == "" {
			content = m.Message
		}
		direction := "in"
		if m.From.ID == pageID {
			direction = "out"
		}
		out.Messages = append(out.Messages, dto.MessageDTO{
			ID: m.ID, SenderID: m.From.ID, Direction: direction, Type: m.Type,
			Content: content, Attachments: m.Attachments, InsertedAt: m.InsertedAt.Time,
		})
	}
	email := firstContact(raw.RecentEmails)
	for _, cu := range raw.Customers {
		phone := firstContact(cu.Phones)
		out.Customers = append(out.Customers, dto.CustomerDTO{
			ID: cu.ID, Name: cu.Name, FbID: cu.FbID, Phone: phone, Email: email, Gender: cu.PersonalInfo.Gender,
		})
	}
	return out, nil
}

func (c *client) ListTags(ctx context.Context, pageID, pageToken string) ([]dto.TagDTO, error) {
	u := fmt.Sprintf("%s/pages/%s/tags?page_access_token=%s",
		c.pageBaseURL, url.PathEscape(pageID), url.QueryEscape(pageToken))
	var raw struct {
		Success *bool    `json:"success"`
		Message string   `json:"message"`
		Tags    []rawTag `json:"tags"`
	}
	if err := c.doJSON(ctx, pageID, http.MethodGet, u, nil, "", &raw); err != nil {
		return nil, err
	}
	if raw.Success != nil && !*raw.Success {
		return nil, fmt.Errorf("pancake tags error (page %s): %s", pageID, raw.Message)
	}
	out := make([]dto.TagDTO, 0, len(raw.Tags))
	for _, t := range raw.Tags {
		out = append(out, dto.TagDTO{ID: t.ID, Name: t.Text, Color: t.Color})
	}
	return out, nil
}
