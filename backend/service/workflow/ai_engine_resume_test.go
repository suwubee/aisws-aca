package workflow

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/ai-coding-assistant/model"
	"github.com/ai-coding-assistant/service/ai"
)

func TestAIWorkflowEngine_ResumeWorkflow_ClosingRuntimeFallsBackToPersistedPath(t *testing.T) {
	initWorkflowEngineTestDB(t)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/chat/completions" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id":      "test",
			"object":  "chat.completion",
			"created": time.Now().Unix(),
			"model":   "test-model",
			"choices": []map[string]any{
				{
					"index": 0,
					"message": map[string]any{
						"role":    "assistant",
						"content": "<complete>ok</complete>",
					},
				},
			},
		})
	}))
	defer srv.Close()

	now := time.Now()
	aiCfg := model.AIProviderConfig{
		ID:        "ai-1",
		UserID:    "u-1",
		Name:      "default",
		Provider:  "openai",
		BaseURL:   srv.URL,
		APIKey:    "test",
		Model:     "test-model",
		IsDefault: true,
		Enabled:   true,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := model.DB.Create(&aiCfg).Error; err != nil {
		t.Fatalf("create AI provider config: %v", err)
	}

	messages, _ := json.Marshal([]ai.ChatMessage{
		{Role: "system", Content: "sys"},
		{Role: "assistant", Content: "<complete>done</complete>"},
	})
	completedAt := now
	record := model.AIWorkflowSession{
		ID:          "sess-1",
		WorkflowID:  "task-1",
		UserGoal:    "goal",
		Status:      "completed",
		Messages:    string(messages),
		Steps:       "[]",
		Context:     "{}",
		Summary:     "done",
		StartedAt:   now,
		CompletedAt: &completedAt,
	}
	if err := model.DB.Create(&record).Error; err != nil {
		t.Fatalf("create workflow session: %v", err)
	}

	engine := NewAIWorkflowEngine(nil)
	engine.maxIterations = 1

	rt := &runtimeSession{
		session:  &AIWorkflowSession{ID: record.ID},
		aiConfig: &aiCfg,
		closing:  true,
		done:     make(chan struct{}),
	}
	engine.inflight.Store(record.ID, rt)

	go func() {
		time.Sleep(10 * time.Millisecond)
		engine.inflight.Delete(record.ID)
		close(rt.done)
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if _, err := engine.ResumeWorkflow(ctx, record.ID, "follow-up"); err != nil {
		t.Fatalf("ResumeWorkflow: %v", err)
	}

	s, err := engine.GetSession(record.ID)
	if err != nil {
		t.Fatalf("GetSession: %v", err)
	}

	found := false
	for _, m := range s.Messages {
		if m.Role == "user" && m.Content == "follow-up" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected follow-up user message to be persisted, got messages=%#v", s.Messages)
	}
}
