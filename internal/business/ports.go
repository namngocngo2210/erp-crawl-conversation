package business

import (
	"context"

	"github.com/desi/crawl-conversations/internal/dto"
	"github.com/desi/crawl-conversations/internal/repository"
)

type PancakeClient interface {
	ListPages(ctx context.Context) ([]dto.PageDTO, error)
	ListConversations(ctx context.Context, pageID, pageToken string, p dto.ListParams) (dto.ConversationsResponse, error)
	ListMessages(ctx context.Context, pageID, conversationID, pageToken string, p dto.ListParams) (dto.MessagesResponse, error)
	ListTags(ctx context.Context, pageID, pageToken string) ([]dto.TagDTO, error)
}

type TokenManager interface {
	Get(ctx context.Context, pageID string) (string, error)
	Invalidate(ctx context.Context, pageID string) error
}

type Stats struct {
	Conversations int
	Messages      int
	Customers     int
}

type Repos struct {
	Page         repository.PageRepository
	Customer     repository.CustomerRepository
	Tag          repository.TagRepository
	Conversation repository.ConversationRepository
	Message      repository.MessageRepository
	Checkpoint   repository.CheckpointRepository
}
