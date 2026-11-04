package entity

import "testing"

func TestTableNames(t *testing.T) {
	cases := map[string]string{
		Page{}.TableName():            "pages",
		Message{}.TableName():         "messages",
		Conversation{}.TableName():    "conversations",
		Customer{}.TableName():        "customers",
		Tag{}.TableName():             "tags",
		CrawlRun{}.TableName():        "crawl_runs",
		CrawlCheckpoint{}.TableName(): "crawl_checkpoints",
	}
	for got, want := range cases {
		if got != want {
			t.Fatalf("table name mismatch: got %q want %q", got, want)
		}
	}
}
