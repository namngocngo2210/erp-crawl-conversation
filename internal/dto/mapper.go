package dto

import (
	"strconv"

	"github.com/desi/crawl-conversations/internal/entity"
)

func MapCustomer(pageID int64, c CustomerDTO) entity.Customer {
	return entity.Customer{
		PageID: pageID, PancakeCustomerID: c.ID, Name: c.Name, Phone: c.Phone,
		Email: c.Email, FbID: c.FbID, Gender: c.Gender, Raw: c.Raw,
	}
}

func MapTag(pageID int64, t TagDTO) entity.Tag {
	return entity.Tag{PageID: pageID, PancakeTagID: strconv.Itoa(t.ID), Name: t.Name, Color: t.Color}
}

func MapConversation(pageID int64, customerID *int64, c ConversationDTO) entity.Conversation {
	return entity.Conversation{
		PageID: pageID, PancakeConversationID: c.ID, CustomerID: customerID,
		Type: c.Type, Snippet: c.Snippet, MessageCount: c.MessageCount,
		LastMessageAt: c.UpdatedAt, UpdatedAt: c.UpdatedAt, Raw: c.Raw,
	}
}

func MapMessage(pageID, conversationID int64, m MessageDTO) entity.Message {
	return entity.Message{
		PageID: pageID, ConversationID: conversationID, PancakeMessageID: m.ID,
		SenderID: m.SenderID, Direction: m.Direction, Type: m.Type, Content: m.Content,
		Attachments: m.Attachments, InsertedAt: m.InsertedAt, Raw: m.Raw,
	}
}
