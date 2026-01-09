package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/ai-coding-assistant/config"
	"github.com/ai-coding-assistant/middleware"
	"github.com/ai-coding-assistant/model"
	"github.com/ai-coding-assistant/service/keybinding"
	promptsvc "github.com/ai-coding-assistant/service/prompt"
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
)

func TestResetData_ClearsBusinessTablesButKeepsUsersAndBuiltinTemplates(t *testing.T) {
	dsn := fmt.Sprintf("file:api_test_%d?mode=memory&cache=shared", time.Now().UnixNano())
	if err := model.InitDB(dsn); err != nil {
		t.Fatalf("InitDB failed: %v", err)
	}

	authCfg := &config.AuthConfig{
		JWTSecret:     "test-secret",
		JWTExpiration: time.Hour,
		Username:      "admin",
		Password:      "admin123",
	}

	app := fiber.New()
	apiGroup := app.Group("/api")

	authCtrl := NewAuthController(authCfg)
	app.Post("/api/auth/login", authCtrl.Login)

	apiGroup.Use(middleware.AuthMiddleware(authCfg))
	apiGroup.Post("/auth/reset-data", middleware.RequireRole("admin"), authCtrl.ResetData)

	token := loginForToken(t, app, "admin", "admin123")

	var admin model.User
	if err := model.DB.First(&admin, "username = ?", "admin").Error; err != nil {
		t.Fatalf("expected admin user after login, got error: %v", err)
	}

	now := time.Now()

	taskID := uuid.NewString()
	task := model.Task{
		ID:         taskID,
		UserID:     admin.ID,
		Title:      "Test Task",
		Status:     "todo",
		Priority:   1,
		OrderIndex: 1,
		WorkDir:    "/tmp/test",
		CLIType:    "claude",
		CreatedAt:  now,
		UpdatedAt:  now,
	}
	if err := model.DB.Create(&task).Error; err != nil {
		t.Fatalf("create task failed: %v", err)
	}

	comment := model.Comment{
		ID:        uuid.NewString(),
		TaskID:    taskID,
		Content:   "hello",
		Author:    "tester",
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := model.DB.Create(&comment).Error; err != nil {
		t.Fatalf("create comment failed: %v", err)
	}

	terminalID := uuid.NewString()
	terminal := model.TerminalSession{
		ID:        terminalID,
		UserID:    admin.ID,
		Title:     "Test Terminal",
		TaskID:    &taskID,
		Shell:     "bash",
		Status:    "running",
		PID:       123,
		CreatedAt: now,
	}
	if err := model.DB.Create(&terminal).Error; err != nil {
		t.Fatalf("create terminal session failed: %v", err)
	}

	aiSessionID := uuid.NewString()
	aiSession := model.AISession{
		ID:         aiSessionID,
		TerminalID: terminalID,
		TaskID:     &taskID,
		AIType:     "claude-code",
		State:      "waiting_approval",
		CreatedAt:  now,
		UpdatedAt:  now,
	}
	if err := model.DB.Create(&aiSession).Error; err != nil {
		t.Fatalf("create ai session failed: %v", err)
	}

	approvalRecord := model.ApprovalRecord{
		ID:            uuid.NewString(),
		TerminalID:    terminalID,
		AISessionID:   &aiSessionID,
		PromptType:    "permission",
		PromptContent: "Allow create file?",
		Response:      "yes",
		AutoApproved:  false,
		CreatedAt:     now,
	}
	if err := model.DB.Create(&approvalRecord).Error; err != nil {
		t.Fatalf("create approval record failed: %v", err)
	}

	logRecord := model.Log{
		ID:         uuid.NewString(),
		TerminalID: &terminalID,
		TaskID:     &taskID,
		LogType:    "output",
		Content:    "hi",
		CreatedAt:  now,
	}
	if err := model.DB.Create(&logRecord).Error; err != nil {
		t.Fatalf("create log failed: %v", err)
	}

	message := model.Message{
		ID:         uuid.NewString(),
		TerminalID: &terminalID,
		TaskID:     &taskID,
		Type:       "info",
		Title:      "test",
		Content:    "hello",
		Status:     "unread",
		CreatedAt:  now,
		UpdatedAt:  now,
	}
	if err := model.DB.Create(&message).Error; err != nil {
		t.Fatalf("create message failed: %v", err)
	}

	project := model.Project{
		ID:        uuid.NewString(),
		Name:      "Test Project",
		Type:      model.ProjectTypeLocal,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := model.DB.Create(&project).Error; err != nil {
		t.Fatalf("create project failed: %v", err)
	}

	cliProfile := model.CLIProfile{
		ID:        uuid.NewString(),
		Name:      "Default Claude",
		Type:      model.CLIProfileTypeClaude,
		Command:   "claude",
		CreatedAt: now,
		UpdatedAt: now,
		DefaultArgs: model.StringArray{
			"--help",
		},
	}
	if err := model.DB.Create(&cliProfile).Error; err != nil {
		t.Fatalf("create cli profile failed: %v", err)
	}

	workflowID := uuid.NewString()
	workflow := model.Workflow{
		ID:          workflowID,
		Name:        "Test Workflow",
		Description: "desc",
		Nodes:       "[]",
		Edges:       "[]",
		Status:      "draft",
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if err := model.DB.Create(&workflow).Error; err != nil {
		t.Fatalf("create workflow failed: %v", err)
	}

	workflowNode := model.WorkflowNode{
		ID:         uuid.NewString(),
		WorkflowID: workflowID,
		Type:       model.NodeTypeTask,
		Name:       "node",
	}
	if err := model.DB.Create(&workflowNode).Error; err != nil {
		t.Fatalf("create workflow node failed: %v", err)
	}

	workflowRun := model.WorkflowRun{
		ID:         uuid.NewString(),
		WorkflowID: workflowID,
		Status:     "pending",
		Logs:       "[]",
	}
	if err := model.DB.Create(&workflowRun).Error; err != nil {
		t.Fatalf("create workflow run failed: %v", err)
	}

	aiWorkflowSession := model.AIWorkflowSession{
		ID:         uuid.NewString(),
		WorkflowID: workflowID,
		UserGoal:   "goal",
		Status:     "running",
		Messages:   "[]",
		Steps:      "[]",
		Context:    "{}",
		StartedAt:  now,
	}
	if err := model.DB.Create(&aiWorkflowSession).Error; err != nil {
		t.Fatalf("create ai workflow session failed: %v", err)
	}

	secret := model.Secret{
		ID:         uuid.NewString(),
		Name:       "Test Secret",
		Type:       "api_key",
		Ciphertext: "cipher",
		Meta:       "{}",
		CreatedAt:  now,
		UpdatedAt:  now,
	}
	if err := model.DB.Create(&secret).Error; err != nil {
		t.Fatalf("create secret failed: %v", err)
	}

	groupID := uuid.NewString()
	group := model.ServerGroup{
		ID:          groupID,
		Name:        "Group",
		Description: "",
		ParentID:    nil,
	}
	if err := model.DB.Create(&group).Error; err != nil {
		t.Fatalf("create server group failed: %v", err)
	}

	server := model.SSHServer{
		ID:       uuid.NewString(),
		Name:     "Server",
		Host:     "127.0.0.1",
		Port:     22,
		Username: "root",
		AuthType: "password",
		GroupID:  &groupID,
		Tags:     "[]",
	}
	if err := model.DB.Create(&server).Error; err != nil {
		t.Fatalf("create ssh server failed: %v", err)
	}

	aiProvider := model.AIProviderConfig{
		ID:        uuid.NewString(),
		Name:      "default-provider",
		Provider:  "openai",
		Model:     "gpt-4o-mini",
		Enabled:   true,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := model.DB.Create(&aiProvider).Error; err != nil {
		t.Fatalf("create ai provider config failed: %v", err)
	}

	agentConfig := model.AgentConfig{
		AgentType:   "claude-code",
		DisplayName: "Claude Code",
		Enabled:     true,
		Priority:    100,
		DetectModes: model.StringArray{"claude"},
	}
	if err := model.DB.Create(&agentConfig).Error; err != nil {
		t.Fatalf("create agent config failed: %v", err)
	}

	ruleSet := model.RuleSet{
		ID:                uuid.NewString(),
		Name:              "Test Rule",
		Type:              "task",
		ApprovalMode:      "manual",
		AutoInputType:     "yes",
		WhitelistPatterns: "[]",
		BlacklistPatterns: "[]",
		ContextLines:      50,
		DetectClaudeCode:  true,
		DetectCodex:       true,
		DetectGemini:      true,
		NotifyOnBlock:     true,
		NotifyOnApprove:   false,
		CreatedAt:         now,
		UpdatedAt:         now,
	}
	if err := model.DB.Create(&ruleSet).Error; err != nil {
		t.Fatalf("create rule set failed: %v", err)
	}

	customTemplate := model.WorkflowTemplate{
		ID:          uuid.NewString(),
		Name:        "Custom Template",
		Description: "custom",
		Category:    model.WorkflowTemplateCategoryDevelopment,
		Nodes:       "[]",
		Edges:       "[]",
		IsBuiltin:   false,
		CreatedAt:   now,
	}
	if err := model.DB.Create(&customTemplate).Error; err != nil {
		t.Fatalf("create workflow template failed: %v", err)
	}

	resetReq := httptest.NewRequest("POST", "/api/auth/reset-data", nil)
	resetReq.Header.Set("Authorization", "Bearer "+token)
	resetResp, err := app.Test(resetReq)
	if err != nil {
		t.Fatalf("reset request failed: %v", err)
	}
	defer resetResp.Body.Close()

	if resetResp.StatusCode != 200 {
		t.Fatalf("expected reset status 200, got %d", resetResp.StatusCode)
	}

	var resetBody map[string]any
	if err := json.NewDecoder(resetResp.Body).Decode(&resetBody); err != nil {
		t.Fatalf("decode reset response failed: %v", err)
	}
	if resetBody["message"] == "" {
		t.Fatalf("expected reset response message")
	}

	assertTableCount(t, &model.User{}, 1)
	assertTableCount(t, &model.Task{}, 0)
	assertTableCount(t, &model.Comment{}, 0)
	assertTableCount(t, &model.TerminalSession{}, 0)
	assertTableCount(t, &model.AISession{}, 0)
	assertTableCount(t, &model.ApprovalRecord{}, 0)
	assertTableCount(t, &model.Log{}, 0)
	assertTableCount(t, &model.Message{}, 0)
	assertTableCount(t, &model.Project{}, 0)
	assertTableCount(t, &model.CLIProfile{}, 0)
	assertTableCount(t, &model.Workflow{}, 0)
	assertTableCount(t, &model.WorkflowNode{}, 0)
	assertTableCount(t, &model.WorkflowRun{}, 0)
	assertTableCount(t, &model.AIWorkflowSession{}, 0)
	assertTableCount(t, &model.Secret{}, 0)
	assertTableCount(t, &model.SSHServer{}, 0)
	assertTableCount(t, &model.ServerGroup{}, 0)
	assertTableCount(t, &model.AIProviderConfig{}, 0)
	assertTableCount(t, &model.AgentConfig{}, 0)
	assertTableCount(t, &model.RuleSet{}, 0)
	assertTableCount(t, &model.ScheduledJob{}, 0)
	assertTableCount(t, &model.ScheduledJobRun{}, 0)

	// Builtin templates should remain, while custom templates should be deleted.
	var tplCount int64
	if err := model.DB.Model(&model.WorkflowTemplate{}).Count(&tplCount).Error; err != nil {
		t.Fatalf("count workflow templates failed: %v", err)
	}
	if tplCount == 0 {
		t.Fatalf("expected builtin workflow templates to remain")
	}

	var customCount int64
	if err := model.DB.Model(&model.WorkflowTemplate{}).Where("is_builtin = ?", false).Count(&customCount).Error; err != nil {
		t.Fatalf("count custom workflow templates failed: %v", err)
	}
	if customCount != 0 {
		t.Fatalf("expected custom workflow templates to be deleted, got %d", customCount)
	}

	// Builtin prompt templates should be restored (used by AI approval/task automation/workflows).
	var promptCount int64
	if err := model.DB.Model(&model.PromptTemplate{}).Count(&promptCount).Error; err != nil {
		t.Fatalf("count prompt templates failed: %v", err)
	}
	expectedPromptCount := int64(len(promptsvc.SupportedTemplateKeys()))
	if promptCount != expectedPromptCount {
		t.Fatalf("expected %d prompt templates after reset, got %d", expectedPromptCount, promptCount)
	}

	var presetCount int64
	if err := model.DB.Model(&model.PromptTemplatePreset{}).Count(&presetCount).Error; err != nil {
		t.Fatalf("count prompt template presets failed: %v", err)
	}
	if presetCount < expectedPromptCount {
		t.Fatalf("expected at least %d prompt template presets after reset, got %d", expectedPromptCount, presetCount)
	}

	// Builtin key bindings should be restored (used by terminal shortcuts/automation).
	var keyBindingCount int64
	if err := model.DB.Model(&model.KeyBinding{}).Count(&keyBindingCount).Error; err != nil {
		t.Fatalf("count key bindings failed: %v", err)
	}
	expectedKeyBindingCount := int64(len(keybinding.SupportedIDs()))
	if keyBindingCount != expectedKeyBindingCount {
		t.Fatalf("expected %d key bindings after reset, got %d", expectedKeyBindingCount, keyBindingCount)
	}
}

func assertTableCount(t *testing.T, modelPtr any, expected int64) {
	t.Helper()
	var count int64
	if err := model.DB.Model(modelPtr).Count(&count).Error; err != nil {
		t.Fatalf("count table failed: %v", err)
	}
	if count != expected {
		buf := &bytes.Buffer{}
		_, _ = fmt.Fprintf(buf, "expected %T count %d, got %d", modelPtr, expected, count)
		t.Fatalf("%s", buf.String())
	}
}
