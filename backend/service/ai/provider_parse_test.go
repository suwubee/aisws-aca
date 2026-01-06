package ai

import "testing"

func TestParseDecisionFromResponse_JSON(t *testing.T) {
	resp := `{"action":"approve","input":"yes\n","confidence":0.92,"reasoning":"safe"}`
	decision := parseDecisionFromResponse(resp)

	if decision.Action != "approve" {
		t.Fatalf("expected action approve, got %q", decision.Action)
	}
	if decision.Input != "yes\r" {
		t.Fatalf("expected input yes\\r, got %q", decision.Input)
	}
	if decision.Confidence != 0.92 {
		t.Fatalf("expected confidence 0.92, got %v", decision.Confidence)
	}
	if decision.Reasoning == "" {
		t.Fatalf("expected non-empty reasoning")
	}
}

func TestParseDecisionFromResponse_CodeBlockJSON(t *testing.T) {
	resp := "Here is the decision:\n```json\n{\"action\":\"reject\",\"input\":\"\",\"confidence\":0.8,\"reasoning\":\"danger\"}\n```\n"
	decision := parseDecisionFromResponse(resp)

	if decision.Action != "reject" {
		t.Fatalf("expected action reject, got %q", decision.Action)
	}
	if decision.Confidence != 0.8 {
		t.Fatalf("expected confidence 0.8, got %v", decision.Confidence)
	}
}

func TestParseDecisionFromResponse_KeyValue(t *testing.T) {
	resp := "ACTION: wait\nINPUT: \nCONFIDENCE: 92%\nREASONING: need human\n"
	decision := parseDecisionFromResponse(resp)

	if decision.Action != "wait" {
		t.Fatalf("expected action wait, got %q", decision.Action)
	}
	if decision.Confidence != 0.92 {
		t.Fatalf("expected confidence 0.92, got %v", decision.Confidence)
	}
	if decision.Reasoning != "need human" {
		t.Fatalf("expected reasoning %q, got %q", "need human", decision.Reasoning)
	}
}

func TestParseDecisionFromResponse_ChineseKeyValue(t *testing.T) {
	resp := "动作：通过\n输入：y\\n\n置信度：0.7\n原因：安全\n"
	decision := parseDecisionFromResponse(resp)

	if decision.Action != "approve" {
		t.Fatalf("expected action approve, got %q", decision.Action)
	}
	if decision.Confidence != 0.7 {
		t.Fatalf("expected confidence 0.7, got %v", decision.Confidence)
	}
}
