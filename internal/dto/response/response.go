package response

import (
	"encoding/json"
	"time"

	"github.com/desi/crawl-conversations/internal/entity"
	"github.com/desi/crawl-conversations/internal/repository/read"
)

type PageResp struct {
	ID                int64  `json:"id"`
	PancakePageID     string `json:"pancake_page_id"`
	Platform          string `json:"platform"`
	Name              string `json:"name"`
	Status            string `json:"status"`
	ConversationCount int64  `json:"conversation_count"`
}

type ConversationResp struct {
	ID                    int64     `json:"id"`
	PancakeConversationID string    `json:"pancake_conversation_id"`
	Type                  string    `json:"type"`
	Snippet               string    `json:"snippet"`
	MessageCount          int       `json:"message_count"`
	LastMessageAt         time.Time `json:"last_message_at"`
	UpdatedAt             time.Time `json:"updated_at"`
}

type CustomerResp struct {
	ID     int64  `json:"id"`
	Name   string `json:"name"`
	Phone  string `json:"phone"`
	Email  string `json:"email"`
	FbID   string `json:"fb_id"`
	Gender string `json:"gender"`
}

type TagResp struct {
	ID    int64  `json:"id"`
	Name  string `json:"name"`
	Color string `json:"color"`
}

type ConversationDetailResp struct {
	ConversationResp
	Customer *CustomerResp `json:"customer"`
	Tags     []TagResp     `json:"tags"`
}

type MessageResp struct {
	ID               int64           `json:"id"`
	PancakeMessageID string          `json:"pancake_message_id"`
	SenderID         string          `json:"sender_id"`
	Direction        string          `json:"direction"`
	Type             string          `json:"type"`
	Content          string          `json:"content"`
	Attachments      json.RawMessage `json:"attachments"`
	InsertedAt       time.Time       `json:"inserted_at"`
}

type Page[T any] struct {
	Items      []T     `json:"items"`
	NextCursor *string `json:"next_cursor"`
}

func MapPages(in []read.PageCount) []PageResp {
	out := make([]PageResp, 0, len(in))
	for _, p := range in {
		out = append(out, PageResp{
			ID: p.ID, PancakePageID: p.PancakePageID, Platform: p.Platform,
			Name: p.Name, Status: p.Status, ConversationCount: p.ConversationCount,
		})
	}
	return out
}

func mapConversation(c entity.Conversation) ConversationResp {
	return ConversationResp{
		ID: c.ID, PancakeConversationID: c.PancakeConversationID, Type: c.Type,
		Snippet: c.Snippet, MessageCount: c.MessageCount,
		LastMessageAt: c.LastMessageAt, UpdatedAt: c.UpdatedAt,
	}
}

func MapConversations(in []entity.Conversation, limit int) Page[ConversationResp] {
	items := make([]ConversationResp, 0, len(in))
	for _, c := range in {
		items = append(items, mapConversation(c))
	}
	var next *string
	if len(in) >= limit && len(in) > 0 {
		last := in[len(in)-1]
		s := read.EncodeCursor(read.Cursor{Time: last.UpdatedAt, ID: last.ID})
		next = &s
	}
	return Page[ConversationResp]{Items: items, NextCursor: next}
}

func MapConversationDetail(d read.ConvDetail) ConversationDetailResp {
	resp := ConversationDetailResp{ConversationResp: mapConversation(d.Conversation)}
	if d.Customer != nil {
		resp.Customer = &CustomerResp{
			ID: d.Customer.ID, Name: d.Customer.Name, Phone: d.Customer.Phone,
			Email: d.Customer.Email, FbID: d.Customer.FbID, Gender: d.Customer.Gender,
		}
	}
	resp.Tags = make([]TagResp, 0, len(d.Tags))
	for _, t := range d.Tags {
		resp.Tags = append(resp.Tags, TagResp{ID: t.ID, Name: t.Name, Color: t.Color})
	}
	return resp
}

func MapMessages(in []entity.Message, limit int) Page[MessageResp] {
	items := make([]MessageResp, 0, len(in))
	for _, m := range in {
		items = append(items, MessageResp{
			ID: m.ID, PancakeMessageID: m.PancakeMessageID, SenderID: m.SenderID,
			Direction: m.Direction, Type: m.Type, Content: m.Content,
			Attachments: m.Attachments, InsertedAt: m.InsertedAt,
		})
	}
	var next *string
	if len(in) >= limit && len(in) > 0 {
		last := in[len(in)-1]
		s := read.EncodeCursor(read.Cursor{Time: last.InsertedAt, ID: last.ID})
		next = &s
	}
	return Page[MessageResp]{Items: items, NextCursor: next}
}

func MapCustomers(in []entity.Customer, limit int) Page[CustomerResp] {
	items := make([]CustomerResp, 0, len(in))
	for _, c := range in {
		items = append(items, CustomerResp{
			ID: c.ID, Name: c.Name, Phone: c.Phone, Email: c.Email, FbID: c.FbID, Gender: c.Gender,
		})
	}
	var next *string
	if len(in) >= limit && len(in) > 0 {
		last := in[len(in)-1]
		s := read.EncodeCursor(read.Cursor{ID: last.ID})
		next = &s
	}
	return Page[CustomerResp]{Items: items, NextCursor: next}
}
