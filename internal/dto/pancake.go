package dto

import (
	"encoding/json"
	"time"
)

type ListParams struct {
	PageNumber   int        // conversations: 1-based page
	Since        *time.Time // conversations: filter by inserted_at (unix seconds)
	Until        *time.Time // conversations
	CurrentCount int        // messages: offset = number of messages already fetched
}

type PageDTO struct {
	ID         string
	Name       string
	Platform   string
	InsertedAt time.Time
	Raw        json.RawMessage
}

type CustomerDTO struct {
	ID     string
	Name   string
	Phone  string
	Email  string
	FbID   string
	Gender string
	Raw    json.RawMessage
}

type TagDTO struct {
	ID    int
	Name  string // from API field "text"
	Color string
}

type ConversationDTO struct {
	ID           string
	Type         string
	Snippet      string
	MessageCount int
	UpdatedAt    time.Time
	InsertedAt   time.Time
	Customer     CustomerDTO
	Tags         []TagDTO
	Raw          json.RawMessage
}

type MessageDTO struct {
	ID          string
	SenderID    string // from.id
	Direction   string // "in" | "out" (computed: from.id == page_id => out)
	Type        string
	Content     string // original_message (fallback: message)
	Attachments json.RawMessage
	InsertedAt  time.Time
	Raw         json.RawMessage
}

type ConversationsResponse struct {
	Conversations []ConversationDTO
	Total         int
	HasNext       bool
}

type MessagesResponse struct {
	Messages  []MessageDTO
	Customers []CustomerDTO // richer customer data (incl. phones) from the messages envelope
}
