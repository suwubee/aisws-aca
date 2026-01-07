package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/ai-coding-assistant/model"
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
)

func setupCommentTestApp(t *testing.T) (*fiber.App, string) {
	t.Helper()

	dsn := fmt.Sprintf("file:comment_test_%d?mode=memory&cache=shared", time.Now().UnixNano())
	if err := model.InitDB(dsn); err != nil {
		t.Fatalf("InitDB failed: %v", err)
	}

	taskID := uuid.New().String()
	task := model.Task{
		ID:         taskID,
		Title:      "Test task",
		Status:     "todo",
		OrderIndex: float64(time.Now().UnixNano()),
	}
	if err := model.DB.Create(&task).Error; err != nil {
		t.Fatalf("create task failed: %v", err)
	}

	app := fiber.New()
	apiGroup := app.Group("/api", func(c *fiber.Ctx) error {
		c.Locals("username", "tester")
		return c.Next()
	})

	ctrl := NewCommentController()
	ctrl.RegisterRoutes(apiGroup)

	return app, taskID
}

func TestCommentController_CreateAndList(t *testing.T) {
	app, taskID := setupCommentTestApp(t)

	createReq := httptest.NewRequest("POST", fmt.Sprintf("/api/tasks/%s/comments", taskID), bytes.NewBufferString(`{"content":"hello"}`))
	createReq.Header.Set("Content-Type", "application/json")

	createResp, err := app.Test(createReq)
	if err != nil {
		t.Fatalf("POST request failed: %v", err)
	}
	defer createResp.Body.Close()

	if createResp.StatusCode != 201 {
		t.Fatalf("expected status 201, got %d", createResp.StatusCode)
	}

	var createBody struct {
		Item model.Comment `json:"item"`
	}
	if err := json.NewDecoder(createResp.Body).Decode(&createBody); err != nil {
		t.Fatalf("decode create response failed: %v", err)
	}
	if createBody.Item.ID == "" {
		t.Fatalf("expected non-empty comment id")
	}
	if createBody.Item.TaskID != taskID {
		t.Fatalf("expected task_id %q, got %q", taskID, createBody.Item.TaskID)
	}
	if createBody.Item.Content != "hello" {
		t.Fatalf("expected content %q, got %q", "hello", createBody.Item.Content)
	}
	if createBody.Item.Author != "tester" {
		t.Fatalf("expected author %q, got %q", "tester", createBody.Item.Author)
	}

	listReq := httptest.NewRequest("GET", fmt.Sprintf("/api/tasks/%s/comments", taskID), nil)
	listResp, err := app.Test(listReq)
	if err != nil {
		t.Fatalf("GET request failed: %v", err)
	}
	defer listResp.Body.Close()

	if listResp.StatusCode != 200 {
		t.Fatalf("expected status 200, got %d", listResp.StatusCode)
	}

	var listBody struct {
		Items []model.Comment `json:"items"`
	}
	if err := json.NewDecoder(listResp.Body).Decode(&listBody); err != nil {
		t.Fatalf("decode list response failed: %v", err)
	}
	if len(listBody.Items) != 1 {
		t.Fatalf("expected 1 comment, got %d", len(listBody.Items))
	}
	if listBody.Items[0].ID != createBody.Item.ID {
		t.Fatalf("expected comment id %q, got %q", createBody.Item.ID, listBody.Items[0].ID)
	}
}

func TestCommentController_CreateTaskComment_ValidationAndNotFound(t *testing.T) {
	app, taskID := setupCommentTestApp(t)

	emptyReq := httptest.NewRequest("POST", fmt.Sprintf("/api/tasks/%s/comments", taskID), bytes.NewBufferString(`{"content":"   "}`))
	emptyReq.Header.Set("Content-Type", "application/json")
	emptyResp, err := app.Test(emptyReq)
	if err != nil {
		t.Fatalf("POST request failed: %v", err)
	}
	defer emptyResp.Body.Close()
	if emptyResp.StatusCode != 400 {
		t.Fatalf("expected status 400, got %d", emptyResp.StatusCode)
	}

	invalidReq := httptest.NewRequest("POST", fmt.Sprintf("/api/tasks/%s/comments", taskID), bytes.NewBufferString(`{"content":`))
	invalidReq.Header.Set("Content-Type", "application/json")
	invalidResp, err := app.Test(invalidReq)
	if err != nil {
		t.Fatalf("POST request failed: %v", err)
	}
	defer invalidResp.Body.Close()
	if invalidResp.StatusCode != 400 {
		t.Fatalf("expected status 400, got %d", invalidResp.StatusCode)
	}

	missingTaskReq := httptest.NewRequest("POST", "/api/tasks/not-exist/comments", bytes.NewBufferString(`{"content":"hello"}`))
	missingTaskReq.Header.Set("Content-Type", "application/json")
	missingTaskResp, err := app.Test(missingTaskReq)
	if err != nil {
		t.Fatalf("POST request failed: %v", err)
	}
	defer missingTaskResp.Body.Close()
	if missingTaskResp.StatusCode != 404 {
		t.Fatalf("expected status 404, got %d", missingTaskResp.StatusCode)
	}
}

func TestCommentController_UpdateAndDelete(t *testing.T) {
	app, taskID := setupCommentTestApp(t)

	comment := model.Comment{
		ID:      uuid.New().String(),
		TaskID:  taskID,
		Content: "old",
		Author:  "tester",
	}
	if err := model.DB.Create(&comment).Error; err != nil {
		t.Fatalf("create comment failed: %v", err)
	}

	updateReq := httptest.NewRequest("PUT", fmt.Sprintf("/api/comments/%s", comment.ID), bytes.NewBufferString(`{"content":"new"}`))
	updateReq.Header.Set("Content-Type", "application/json")
	updateResp, err := app.Test(updateReq)
	if err != nil {
		t.Fatalf("PUT request failed: %v", err)
	}
	defer updateResp.Body.Close()

	if updateResp.StatusCode != 200 {
		t.Fatalf("expected status 200, got %d", updateResp.StatusCode)
	}

	var updateBody struct {
		Item model.Comment `json:"item"`
	}
	if err := json.NewDecoder(updateResp.Body).Decode(&updateBody); err != nil {
		t.Fatalf("decode update response failed: %v", err)
	}
	if updateBody.Item.Content != "new" {
		t.Fatalf("expected updated content %q, got %q", "new", updateBody.Item.Content)
	}

	updateEmptyReq := httptest.NewRequest("PUT", fmt.Sprintf("/api/comments/%s", comment.ID), bytes.NewBufferString(`{"content":" "}`))
	updateEmptyReq.Header.Set("Content-Type", "application/json")
	updateEmptyResp, err := app.Test(updateEmptyReq)
	if err != nil {
		t.Fatalf("PUT request failed: %v", err)
	}
	defer updateEmptyResp.Body.Close()
	if updateEmptyResp.StatusCode != 400 {
		t.Fatalf("expected status 400, got %d", updateEmptyResp.StatusCode)
	}

	updateInvalidReq := httptest.NewRequest("PUT", fmt.Sprintf("/api/comments/%s", comment.ID), bytes.NewBufferString(`{"content":`))
	updateInvalidReq.Header.Set("Content-Type", "application/json")
	updateInvalidResp, err := app.Test(updateInvalidReq)
	if err != nil {
		t.Fatalf("PUT request failed: %v", err)
	}
	defer updateInvalidResp.Body.Close()
	if updateInvalidResp.StatusCode != 400 {
		t.Fatalf("expected status 400, got %d", updateInvalidResp.StatusCode)
	}

	updateMissingReq := httptest.NewRequest("PUT", "/api/comments/not-exist", bytes.NewBufferString(`{"content":"new"}`))
	updateMissingReq.Header.Set("Content-Type", "application/json")
	updateMissingResp, err := app.Test(updateMissingReq)
	if err != nil {
		t.Fatalf("PUT request failed: %v", err)
	}
	defer updateMissingResp.Body.Close()
	if updateMissingResp.StatusCode != 404 {
		t.Fatalf("expected status 404, got %d", updateMissingResp.StatusCode)
	}

	deleteReq := httptest.NewRequest("DELETE", fmt.Sprintf("/api/comments/%s", comment.ID), nil)
	deleteResp, err := app.Test(deleteReq)
	if err != nil {
		t.Fatalf("DELETE request failed: %v", err)
	}
	defer deleteResp.Body.Close()
	if deleteResp.StatusCode != 200 {
		t.Fatalf("expected status 200, got %d", deleteResp.StatusCode)
	}

	deleteMissingReq := httptest.NewRequest("DELETE", fmt.Sprintf("/api/comments/%s", comment.ID), nil)
	deleteMissingResp, err := app.Test(deleteMissingReq)
	if err != nil {
		t.Fatalf("DELETE request failed: %v", err)
	}
	defer deleteMissingResp.Body.Close()
	if deleteMissingResp.StatusCode != 404 {
		t.Fatalf("expected status 404, got %d", deleteMissingResp.StatusCode)
	}
}
