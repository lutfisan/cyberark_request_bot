package bot

import (
	"testing"

	"cybarbot/internal/cyberark"
)

func TestBuildRequestSelectionKeyboard(t *testing.T) {
	requests := []cyberark.IncomingRequest{
		{
			RequestID:         "111_1",
			RequestorUserName: "alice",
			AccountDetails: cyberark.AccountDetails{
				Properties: cyberark.AccountProperties{
					Address: "10.0.0.1",
				},
			},
		},
		{
			RequestID:         "222_2",
			RequestorUserName: "bob",
			Operation:         "Connect with root on server-a",
		},
	}

	kb := buildRequestSelectionKeyboard(requests, "notif_detail_")

	if len(kb.InlineKeyboard) != 2 {
		t.Fatalf("expected 2 rows, got %d", len(kb.InlineKeyboard))
	}

	btn1 := kb.InlineKeyboard[0][0]
	if btn1.CallbackData != "notif_detail_111_1" {
		t.Errorf("expected callback data 'notif_detail_111_1', got %s", btn1.CallbackData)
	}
	if btn1.Text != "[111_1] alice -> 10.0.0.1" {
		t.Errorf("expected text '[111_1] alice -> 10.0.0.1', got %s", btn1.Text)
	}

	btn2 := kb.InlineKeyboard[1][0]
	if btn2.CallbackData != "notif_detail_222_2" {
		t.Errorf("expected callback data 'notif_detail_222_2', got %s", btn2.CallbackData)
	}
	if btn2.Text != "[222_2] bob -> server-a" {
		t.Errorf("expected text '[222_2] bob -> server-a', got %s", btn2.Text)
	}
}

func TestBuildConfirmReasonKeyboard(t *testing.T) {
	kb := buildConfirmReasonKeyboard("req_123")
	
	if len(kb.InlineKeyboard) != 1 {
		t.Fatalf("expected 1 row, got %d", len(kb.InlineKeyboard))
	}
	
	if len(kb.InlineKeyboard[0]) != 2 {
		t.Fatalf("expected 2 buttons, got %d", len(kb.InlineKeyboard[0]))
	}

	btn1 := kb.InlineKeyboard[0][0]
	if btn1.CallbackData != "confirm_skip_req_123" {
		t.Errorf("expected confirm_skip_req_123, got %s", btn1.CallbackData)
	}

	btn2 := kb.InlineKeyboard[0][1]
	if btn2.CallbackData != "confirm_reason_req_123" {
		t.Errorf("expected confirm_reason_req_123, got %s", btn2.CallbackData)
	}
}
