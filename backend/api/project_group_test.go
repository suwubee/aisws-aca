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
)

func setupProjectGroupTestApp(t *testing.T) *fiber.App {
	t.Helper()

	dsn := fmt.Sprintf("file:project_group_test_%d?mode=memory&cache=shared", time.Now().UnixNano())
	if err := model.InitDB(dsn); err != nil {
		t.Fatalf("InitDB failed: %v", err)
	}

	app := fiber.New()
	apiGroup := app.Group("/api", func(c *fiber.Ctx) error {
		c.Locals("username", "tester")
		c.Locals("userID", "u-1")
		c.Locals("role", "admin")
		return c.Next()
	})

	ctrl := NewProjectGroupController()
	ctrl.RegisterRoutes(apiGroup)

	return app
}

func TestProjectGroupController_CRUD_AndUnbindProjects(t *testing.T) {
	app := setupProjectGroupTestApp(t)

	createReq := httptest.NewRequest("POST", "/api/project-groups", bytes.NewBufferString(`{"name":"group-1","description":"desc"}`))
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
		Item model.ProjectGroup `json:"item"`
	}
	if err := json.NewDecoder(createResp.Body).Decode(&createBody); err != nil {
		t.Fatalf("decode create response failed: %v", err)
	}
	if createBody.Item.ID == "" {
		t.Fatalf("expected non-empty group id")
	}
	if createBody.Item.Name != "group-1" {
		t.Fatalf("expected group name %q, got %q", "group-1", createBody.Item.Name)
	}

	// Bind a project to ensure delete unbinds it.
	project := model.Project{
		ID:      "proj-1",
		Name:    "proj-1",
		Type:    model.ProjectTypeLocal,
		GroupID: &createBody.Item.ID,
	}
	if err := model.DB.Create(&project).Error; err != nil {
		t.Fatalf("create project failed: %v", err)
	}

	listReq := httptest.NewRequest("GET", "/api/project-groups", nil)
	listResp, err := app.Test(listReq)
	if err != nil {
		t.Fatalf("GET request failed: %v", err)
	}
	defer listResp.Body.Close()
	if listResp.StatusCode != 200 {
		t.Fatalf("expected status 200, got %d", listResp.StatusCode)
	}

	var listBody struct {
		Items []model.ProjectGroup `json:"items"`
	}
	if err := json.NewDecoder(listResp.Body).Decode(&listBody); err != nil {
		t.Fatalf("decode list response failed: %v", err)
	}
	if len(listBody.Items) != 1 {
		t.Fatalf("expected 1 group, got %d", len(listBody.Items))
	}

	updateReq := httptest.NewRequest("PUT", "/api/project-groups/"+createBody.Item.ID, bytes.NewBufferString(`{"name":"group-1-updated"}`))
	updateReq.Header.Set("Content-Type", "application/json")
	updateResp, err := app.Test(updateReq)
	if err != nil {
		t.Fatalf("PUT request failed: %v", err)
	}
	defer updateResp.Body.Close()
	if updateResp.StatusCode != 200 {
		t.Fatalf("expected status 200, got %d", updateResp.StatusCode)
	}

	deleteReq := httptest.NewRequest("DELETE", "/api/project-groups/"+createBody.Item.ID, nil)
	deleteResp, err := app.Test(deleteReq)
	if err != nil {
		t.Fatalf("DELETE request failed: %v", err)
	}
	defer deleteResp.Body.Close()
	if deleteResp.StatusCode != 200 {
		t.Fatalf("expected status 200, got %d", deleteResp.StatusCode)
	}

	var updatedProject model.Project
	if err := model.DB.First(&updatedProject, "id = ?", project.ID).Error; err != nil {
		t.Fatalf("query project failed: %v", err)
	}
	if updatedProject.GroupID != nil {
		t.Fatalf("expected project group_id to be nil after group delete, got %v", updatedProject.GroupID)
	}
}
