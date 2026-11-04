package entity

import "testing"

func TestUserTableName(t *testing.T) {
	if (User{}).TableName() != "users" {
		t.Fatalf("want users, got %s", (User{}).TableName())
	}
}
