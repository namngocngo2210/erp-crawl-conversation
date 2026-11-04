package dto

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestMapTag_IntIDToString(t *testing.T) {
	tag := MapTag(7, TagDTO{ID: 46, Name: "VIP", Color: "#fff"})
	require.Equal(t, int64(7), tag.PageID)
	require.Equal(t, "46", tag.PancakeTagID)
	require.Equal(t, "VIP", tag.Name)
}

func TestMapMessage(t *testing.T) {
	ts := time.Date(2026, 6, 10, 0, 0, 0, 0, time.UTC)
	m := MapMessage(7, 9, MessageDTO{ID: "m1", SenderID: "s1", Direction: "out", Type: "INBOX", Content: "hi", InsertedAt: ts})
	require.Equal(t, int64(7), m.PageID)
	require.Equal(t, int64(9), m.ConversationID)
	require.Equal(t, "m1", m.PancakeMessageID)
	require.Equal(t, "out", m.Direction)
	require.Equal(t, ts, m.InsertedAt)
}

func TestMapConversation_UsesUpdatedAtForLastMessage(t *testing.T) {
	cid := int64(3)
	ts := time.Date(2026, 6, 11, 0, 0, 0, 0, time.UTC)
	c := MapConversation(7, &cid, ConversationDTO{ID: "c1", Type: "INBOX", MessageCount: 5, UpdatedAt: ts})
	require.Equal(t, "c1", c.PancakeConversationID)
	require.Equal(t, &cid, c.CustomerID)
	require.Equal(t, ts, c.UpdatedAt)
	require.Equal(t, ts, c.LastMessageAt) // proxy: updated_at
}
