package entity

import (
	"encoding/json"
	"time"
)

type Page struct {
	ID             int64 `gorm:"primaryKey"`
	PancakePageID  string
	Platform       string
	Name           string
	Status         string
	InsertedAt     time.Time `gorm:"-"` // transient: page creation time from API, used for backfill windowing; not persisted
	LastSyncedAt   *time.Time
	Raw            json.RawMessage `gorm:"type:jsonb"`
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

func (Page) TableName() string { return "pages" }

type Customer struct {
	ID                int64 `gorm:"primaryKey"`
	PageID            int64
	PancakeCustomerID string
	Name              string
	Phone             string
	Email             string
	FbID              string
	Gender            string
	Raw               json.RawMessage `gorm:"type:jsonb"`
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

func (Customer) TableName() string { return "customers" }

type Tag struct {
	ID           int64 `gorm:"primaryKey"`
	PageID       int64
	PancakeTagID string
	Name         string
	Color        string
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

func (Tag) TableName() string { return "tags" }

type Conversation struct {
	ID                    int64 `gorm:"primaryKey"`
	PageID                int64
	PancakeConversationID string
	CustomerID            *int64
	Type                  string
	Snippet               string
	MessageCount          int
	LastMessageAt         time.Time
	UpdatedAt             time.Time
	Raw                   json.RawMessage `gorm:"type:jsonb"`
	CreatedAt             time.Time
}

func (Conversation) TableName() string { return "conversations" }

type Message struct {
	ID               int64 `gorm:"primaryKey"`
	PageID           int64
	ConversationID   int64
	PancakeMessageID string
	SenderID         string
	Direction        string
	Type             string
	Content          string
	Attachments      json.RawMessage `gorm:"type:jsonb"`
	InsertedAt       time.Time
	Raw              json.RawMessage `gorm:"type:jsonb"`
	CreatedAt        time.Time
}

func (Message) TableName() string { return "messages" }

type CrawlRun struct {
	ID         int64 `gorm:"primaryKey"`
	StartedAt  time.Time
	FinishedAt *time.Time
	Trigger    string
	Status     string
	Stats      json.RawMessage `gorm:"type:jsonb"`
	Error      string
}

func (CrawlRun) TableName() string { return "crawl_runs" }

type CrawlCheckpoint struct {
	PageID    int64 `gorm:"primaryKey"`
	JobType   string
	Cursor    *time.Time
	Status    string
	LastRunAt *time.Time
}

func (CrawlCheckpoint) TableName() string { return "crawl_checkpoints" }
