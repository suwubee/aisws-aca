package api

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/ai-coding-assistant/middleware"
	"github.com/ai-coding-assistant/model"
	secretservice "github.com/ai-coding-assistant/service/secret"
	sshservice "github.com/ai-coding-assistant/service/ssh"
	"github.com/ai-coding-assistant/service/terminal"
	"github.com/gofiber/fiber/v2"
	cryptossh "golang.org/x/crypto/ssh"
)

func setupSSHServerTestApp(t *testing.T) (*fiber.App, *SSHServerController) {
	t.Helper()

	dsn := fmt.Sprintf("file:ssh_server_test_%d?mode=memory&cache=shared", time.Now().UnixNano())
	if err := model.InitDB(dsn); err != nil {
		t.Fatalf("InitDB failed: %v", err)
	}

	app := fiber.New()
	apiGroup := app.Group("/api", func(c *fiber.Ctx) error {
		c.Locals("username", "tester")
		c.Locals("role", "admin")
		return c.Next()
	})

	adminGroup := apiGroup.Group("", middleware.RequireRole("admin"))

	ctrl := NewSSHServerController("test-master-key", nil)
	ctrl.RegisterRoutes(adminGroup)

	return app, ctrl
}

type stubTerminalCreator struct {
	session *terminal.Session
	err     error
	called  string
}

func (s *stubTerminalCreator) CreateSSHSession(serverID string) (*terminal.Session, error) {
	s.called = serverID
	if s.err != nil {
		return nil, s.err
	}
	return s.session, nil
}

func TestSSHServerController_CreateServerTerminal_ReturnsSessionID(t *testing.T) {
	app := fiber.New()
	apiGroup := app.Group("/api", func(c *fiber.Ctx) error {
		c.Locals("username", "tester")
		c.Locals("role", "admin")
		return c.Next()
	})
	adminGroup := apiGroup.Group("", middleware.RequireRole("admin"))

	creator := &stubTerminalCreator{session: terminal.NewSession("sess-1", "/bin/bash", 1024)}
	ctrl := NewSSHServerController("test-master-key", creator)
	ctrl.RegisterRoutes(adminGroup)

	req := httptest.NewRequest("POST", "/api/servers/srv-1/terminal", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("POST request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 201 {
		t.Fatalf("expected status 201, got %d", resp.StatusCode)
	}

	var body struct {
		SessionID string `json:"session_id"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode response failed: %v", err)
	}
	if body.SessionID != "sess-1" {
		t.Fatalf("expected session_id %q, got %q", "sess-1", body.SessionID)
	}
	if creator.called != "srv-1" {
		t.Fatalf("expected CreateSSHSession called with %q, got %q", "srv-1", creator.called)
	}
}

func TestSSHServerController_GroupsCRUD(t *testing.T) {
	app, _ := setupSSHServerTestApp(t)

	createReq := httptest.NewRequest("POST", "/api/server-groups", bytes.NewBufferString(`{"name":"prod","description":"Production"}`))
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
		Item model.ServerGroup `json:"item"`
	}
	if err := json.NewDecoder(createResp.Body).Decode(&createBody); err != nil {
		t.Fatalf("decode create response failed: %v", err)
	}
	if createBody.Item.ID == "" {
		t.Fatalf("expected non-empty group id")
	}

	listReq := httptest.NewRequest("GET", "/api/server-groups", nil)
	listResp, err := app.Test(listReq)
	if err != nil {
		t.Fatalf("GET request failed: %v", err)
	}
	defer listResp.Body.Close()
	if listResp.StatusCode != 200 {
		t.Fatalf("expected status 200, got %d", listResp.StatusCode)
	}

	var listBody struct {
		Items []model.ServerGroup `json:"items"`
	}
	if err := json.NewDecoder(listResp.Body).Decode(&listBody); err != nil {
		t.Fatalf("decode list response failed: %v", err)
	}
	if len(listBody.Items) != 1 {
		t.Fatalf("expected 1 group, got %d", len(listBody.Items))
	}
	if listBody.Items[0].ID != createBody.Item.ID {
		t.Fatalf("expected group id %q, got %q", createBody.Item.ID, listBody.Items[0].ID)
	}
}

func TestSSHServerController_ServersCRUD_PasswordAuth(t *testing.T) {
	app, ctrl := setupSSHServerTestApp(t)

	group := model.ServerGroup{
		ID:          "group-1",
		Name:        "prod",
		Description: "Production",
	}
	if err := model.DB.Create(&group).Error; err != nil {
		t.Fatalf("create group failed: %v", err)
	}

	createReq := httptest.NewRequest("POST", "/api/servers", bytes.NewBufferString(fmt.Sprintf(`{
		"name":"srv1",
		"host":"127.0.0.1",
		"port":22,
		"username":"root",
		"auth_type":"password",
		"password":"p@ss",
		"group_id":"%s",
		"tags":"[\"prod\"]"
	}`, group.ID)))
	createReq.Header.Set("Content-Type", "application/json")
	createResp, err := app.Test(createReq)
	if err != nil {
		t.Fatalf("POST request failed: %v", err)
	}
	defer createResp.Body.Close()
	if createResp.StatusCode != 201 {
		t.Fatalf("expected status 201, got %d", createResp.StatusCode)
	}

	var createBody map[string]any
	if err := json.NewDecoder(createResp.Body).Decode(&createBody); err != nil {
		t.Fatalf("decode create response failed: %v", err)
	}
	item, ok := createBody["item"].(map[string]any)
	if !ok {
		t.Fatalf("expected item in response")
	}
	if _, ok := item["password"]; ok {
		t.Fatalf("expected password to be omitted from response")
	}
	if _, ok := item["private_key"]; ok {
		t.Fatalf("expected private_key to be omitted from response")
	}
	if _, ok := item["passphrase"]; ok {
		t.Fatalf("expected passphrase to be omitted from response")
	}

	id, _ := item["id"].(string)
	if id == "" {
		t.Fatalf("expected non-empty server id")
	}

	var stored model.SSHServer
	if err := model.DB.First(&stored, "id = ?", id).Error; err != nil {
		t.Fatalf("query stored server failed: %v", err)
	}
	if stored.Password == "" {
		t.Fatalf("expected encrypted password to be stored")
	}
	if stored.Password == "p@ss" {
		t.Fatalf("expected stored password not equal plaintext")
	}
	plaintext, err := secretservice.DecryptAESGCMBase64(ctrl.encryptionKey, stored.Password)
	if err != nil {
		t.Fatalf("decrypt stored password failed: %v", err)
	}
	if plaintext != "p@ss" {
		t.Fatalf("expected password %q, got %q", "p@ss", plaintext)
	}

	getReq := httptest.NewRequest("GET", "/api/servers/"+id, nil)
	getResp, err := app.Test(getReq)
	if err != nil {
		t.Fatalf("GET request failed: %v", err)
	}
	defer getResp.Body.Close()
	if getResp.StatusCode != 200 {
		t.Fatalf("expected status 200, got %d", getResp.StatusCode)
	}

	updateReq := httptest.NewRequest("PUT", "/api/servers/"+id, bytes.NewBufferString(`{"name":"srv1-updated","password":"newpass"}`))
	updateReq.Header.Set("Content-Type", "application/json")
	updateResp, err := app.Test(updateReq)
	if err != nil {
		t.Fatalf("PUT request failed: %v", err)
	}
	defer updateResp.Body.Close()
	if updateResp.StatusCode != 200 {
		t.Fatalf("expected status 200, got %d", updateResp.StatusCode)
	}

	var storedAfterUpdate model.SSHServer
	if err := model.DB.First(&storedAfterUpdate, "id = ?", id).Error; err != nil {
		t.Fatalf("query stored server after update failed: %v", err)
	}
	updatedPlaintext, err := secretservice.DecryptAESGCMBase64(ctrl.encryptionKey, storedAfterUpdate.Password)
	if err != nil {
		t.Fatalf("decrypt updated password failed: %v", err)
	}
	if updatedPlaintext != "newpass" {
		t.Fatalf("expected updated password %q, got %q", "newpass", updatedPlaintext)
	}

	deleteReq := httptest.NewRequest("DELETE", "/api/servers/"+id, nil)
	deleteResp, err := app.Test(deleteReq)
	if err != nil {
		t.Fatalf("DELETE request failed: %v", err)
	}
	defer deleteResp.Body.Close()
	if deleteResp.StatusCode != 200 {
		t.Fatalf("expected status 200, got %d", deleteResp.StatusCode)
	}
}

func TestSSHServerController_UpdateAuthType_ClearsOldSecrets(t *testing.T) {
	app, ctrl := setupSSHServerTestApp(t)

	createReq := httptest.NewRequest("POST", "/api/servers", bytes.NewBufferString(`{
		"name":"srv1",
		"host":"127.0.0.1",
		"username":"root",
		"auth_type":"password",
		"password":"p@ss"
	}`))
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
		Item struct {
			ID string `json:"id"`
		} `json:"item"`
	}
	if err := json.NewDecoder(createResp.Body).Decode(&createBody); err != nil {
		t.Fatalf("decode create response failed: %v", err)
	}
	if createBody.Item.ID == "" {
		t.Fatalf("expected non-empty server id")
	}

	switchReq := httptest.NewRequest("PUT", "/api/servers/"+createBody.Item.ID, bytes.NewBufferString(`{"auth_type":"key","private_key":"KEYDATA","passphrase":"pp"}`))
	switchReq.Header.Set("Content-Type", "application/json")
	switchResp, err := app.Test(switchReq)
	if err != nil {
		t.Fatalf("PUT request failed: %v", err)
	}
	defer switchResp.Body.Close()
	if switchResp.StatusCode != 200 {
		t.Fatalf("expected status 200, got %d", switchResp.StatusCode)
	}

	var stored model.SSHServer
	if err := model.DB.First(&stored, "id = ?", createBody.Item.ID).Error; err != nil {
		t.Fatalf("query stored server failed: %v", err)
	}
	if stored.Password != "" {
		t.Fatalf("expected password to be cleared when switching auth_type to key")
	}
	if stored.PrivateKey == "" {
		t.Fatalf("expected private key to be stored")
	}

	keyPlaintext, err := secretservice.DecryptAESGCMBase64(ctrl.encryptionKey, stored.PrivateKey)
	if err != nil {
		t.Fatalf("decrypt private key failed: %v", err)
	}
	if keyPlaintext != "KEYDATA" {
		t.Fatalf("expected private key %q, got %q", "KEYDATA", keyPlaintext)
	}
}

func TestSSHServerController_TestServerConnection(t *testing.T) {
	app, ctrl := setupSSHServerTestApp(t)

	server := model.SSHServer{
		ID:         "srv-test",
		Name:       "srv",
		Host:       "example",
		Port:       22,
		Username:   "tester",
		AuthType:   "password",
		Password:   "ciphertext",
		LastStatus: "unknown",
	}
	if err := model.DB.Create(&server).Error; err != nil {
		t.Fatalf("create server failed: %v", err)
	}

	ctrl.sshManager = &fakeSSHTester{err: nil}

	testReq := httptest.NewRequest("POST", "/api/servers/"+server.ID+"/test", nil)
	testResp, err := app.Test(testReq)
	if err != nil {
		t.Fatalf("POST test request failed: %v", err)
	}
	defer testResp.Body.Close()
	if testResp.StatusCode != 200 {
		t.Fatalf("expected status 200, got %d", testResp.StatusCode)
	}

	var stored model.SSHServer
	if err := model.DB.First(&stored, "id = ?", server.ID).Error; err != nil {
		t.Fatalf("query stored server failed: %v", err)
	}
	if stored.LastStatus != "online" {
		t.Fatalf("expected last_status %q, got %q", "online", stored.LastStatus)
	}

	ctrl.sshManager = &fakeSSHTester{err: errors.New("connection failed")}

	testReq = httptest.NewRequest("POST", "/api/servers/"+server.ID+"/test", nil)
	testResp, err = app.Test(testReq)
	if err != nil {
		t.Fatalf("POST test request failed: %v", err)
	}
	defer testResp.Body.Close()
	if testResp.StatusCode != 400 {
		t.Fatalf("expected status 400, got %d", testResp.StatusCode)
	}

	if err := model.DB.First(&stored, "id = ?", server.ID).Error; err != nil {
		t.Fatalf("query stored server failed: %v", err)
	}
	if stored.LastStatus != "offline" {
		t.Fatalf("expected last_status %q, got %q", "offline", stored.LastStatus)
	}

	testReq = httptest.NewRequest("POST", "/api/servers/missing/test", nil)
	testResp, err = app.Test(testReq)
	if err != nil {
		t.Fatalf("POST test request failed: %v", err)
	}
	defer testResp.Body.Close()
	if testResp.StatusCode != 404 {
		t.Fatalf("expected status 404, got %d", testResp.StatusCode)
	}
}

type fakeSSHTester struct {
	err          error
	batchResults map[string]sshservice.ExecuteResult
	lastBatchIDs []string
	lastCommand  string
}

func (t *fakeSSHTester) TestConnection(server *model.SSHServer) error {
	return t.err
}

func (t *fakeSSHTester) ExecuteCommand(serverID, command string) (string, error) {
	return "", nil
}

func (t *fakeSSHTester) BatchExecute(serverIDs []string, command string) map[string]sshservice.ExecuteResult {
	t.lastBatchIDs = append([]string(nil), serverIDs...)
	t.lastCommand = command
	if t.batchResults != nil {
		return t.batchResults
	}
	return map[string]sshservice.ExecuteResult{}
}

func TestSSHServerController_BatchExecute(t *testing.T) {
	app, ctrl := setupSSHServerTestApp(t)

	fake := &fakeSSHTester{
		batchResults: map[string]sshservice.ExecuteResult{
			"srv-1": {Output: "out1"},
			"srv-2": {Output: "", Error: "boom"},
		},
	}
	ctrl.sshManager = fake

	execReq := httptest.NewRequest("POST", "/api/servers/batch-execute", bytes.NewBufferString(`{
		"server_ids":[" srv-1 ", "srv-1", "srv-2"],
		"command":"  ls -la  "
	}`))
	execReq.Header.Set("Content-Type", "application/json")
	execResp, err := app.Test(execReq)
	if err != nil {
		t.Fatalf("POST request failed: %v", err)
	}
	defer execResp.Body.Close()
	if execResp.StatusCode != 200 {
		t.Fatalf("expected status 200, got %d", execResp.StatusCode)
	}

	var body struct {
		Results map[string]sshservice.ExecuteResult `json:"results"`
	}
	if err := json.NewDecoder(execResp.Body).Decode(&body); err != nil {
		t.Fatalf("decode response failed: %v", err)
	}
	if body.Results["srv-1"].Output != "out1" {
		t.Fatalf("expected srv-1 output %q, got %q", "out1", body.Results["srv-1"].Output)
	}
	if body.Results["srv-2"].Error != "boom" {
		t.Fatalf("expected srv-2 error %q, got %q", "boom", body.Results["srv-2"].Error)
	}

	if fake.lastCommand != "ls -la" {
		t.Fatalf("expected command %q, got %q", "ls -la", fake.lastCommand)
	}

	seen := make(map[string]struct{})
	for _, id := range fake.lastBatchIDs {
		seen[id] = struct{}{}
	}
	if len(seen) != 2 {
		t.Fatalf("expected 2 unique server ids, got %d", len(seen))
	}
	if _, ok := seen["srv-1"]; !ok {
		t.Fatalf("expected srv-1 to be executed")
	}
	if _, ok := seen["srv-2"]; !ok {
		t.Fatalf("expected srv-2 to be executed")
	}

	invalidReq := httptest.NewRequest("POST", "/api/servers/batch-execute", bytes.NewBufferString(`{"server_ids":["srv-1"],"command":" "}`))
	invalidReq.Header.Set("Content-Type", "application/json")
	invalidResp, err := app.Test(invalidReq)
	if err != nil {
		t.Fatalf("POST request failed: %v", err)
	}
	defer invalidResp.Body.Close()
	if invalidResp.StatusCode != 400 {
		t.Fatalf("expected status 400, got %d", invalidResp.StatusCode)
	}
}

func TestSSHServerController_UploadKey_SupportedFormats(t *testing.T) {
	app, ctrl := setupSSHServerTestApp(t)

	passwordCiphertext, err := secretservice.EncryptAESGCMBase64(ctrl.encryptionKey, "p@ss")
	if err != nil {
		t.Fatalf("encrypt password failed: %v", err)
	}

	rsaKey, err := rsa.GenerateKey(rand.Reader, 1024)
	if err != nil {
		t.Fatalf("generate rsa key failed: %v", err)
	}
	rsaPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(rsaKey),
	})

	ecdsaKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate ecdsa key failed: %v", err)
	}
	ecdsaDER, err := x509.MarshalECPrivateKey(ecdsaKey)
	if err != nil {
		t.Fatalf("marshal ecdsa key failed: %v", err)
	}
	ecdsaPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "EC PRIVATE KEY",
		Bytes: ecdsaDER,
	})

	_, ed25519Key, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate ed25519 key failed: %v", err)
	}
	ed25519DER, err := x509.MarshalPKCS8PrivateKey(ed25519Key)
	if err != nil {
		t.Fatalf("marshal ed25519 key failed: %v", err)
	}
	ed25519PEM := pem.EncodeToMemory(&pem.Block{
		Type:  "PRIVATE KEY",
		Bytes: ed25519DER,
	})

	tests := []struct {
		name        string
		privateKey  []byte
		expectType  string
		serverID    string
		keyFilename string
	}{
		{name: "rsa", privateKey: rsaPEM, expectType: "ssh-rsa", serverID: "srv-upload-rsa", keyFilename: "id_rsa"},
		{name: "ecdsa", privateKey: ecdsaPEM, expectType: "ecdsa-sha2-nistp256", serverID: "srv-upload-ecdsa", keyFilename: "id_ecdsa"},
		{name: "ed25519", privateKey: ed25519PEM, expectType: "ssh-ed25519", serverID: "srv-upload-ed25519", keyFilename: "id_ed25519"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			server := model.SSHServer{
				ID:         tc.serverID,
				Name:       "srv",
				Host:       "127.0.0.1",
				Port:       22,
				Username:   "root",
				AuthType:   "password",
				Password:   passwordCiphertext,
				LastStatus: "unknown",
			}
			if err := model.DB.Create(&server).Error; err != nil {
				t.Fatalf("create server failed: %v", err)
			}

			req := newUploadKeyRequest(t, "/api/servers/"+tc.serverID+"/upload-key", "key", tc.keyFilename, tc.privateKey, "")
			resp, err := app.Test(req)
			if err != nil {
				t.Fatalf("POST upload-key failed: %v", err)
			}
			defer resp.Body.Close()
			if resp.StatusCode != 200 {
				t.Fatalf("expected status 200, got %d", resp.StatusCode)
			}

			var responseBody map[string]any
			if err := json.NewDecoder(resp.Body).Decode(&responseBody); err != nil {
				t.Fatalf("decode response failed: %v", err)
			}
			item, ok := responseBody["item"].(map[string]any)
			if !ok {
				t.Fatalf("expected item in response")
			}
			if _, ok := item["private_key"]; ok {
				t.Fatalf("expected private_key to be omitted from response")
			}
			if _, ok := item["passphrase"]; ok {
				t.Fatalf("expected passphrase to be omitted from response")
			}
			if _, ok := item["password"]; ok {
				t.Fatalf("expected password to be omitted from response")
			}

			var stored model.SSHServer
			if err := model.DB.First(&stored, "id = ?", tc.serverID).Error; err != nil {
				t.Fatalf("query stored server failed: %v", err)
			}

			if stored.AuthType != "key" {
				t.Fatalf("expected auth_type %q, got %q", "key", stored.AuthType)
			}
			if stored.Password != "" {
				t.Fatalf("expected password to be cleared")
			}
			if stored.PrivateKey == "" {
				t.Fatalf("expected private key to be stored")
			}

			plaintextKey, err := secretservice.DecryptAESGCMBase64(ctrl.encryptionKey, stored.PrivateKey)
			if err != nil {
				t.Fatalf("decrypt stored private key failed: %v", err)
			}
			signer, err := cryptossh.ParsePrivateKey([]byte(plaintextKey))
			if err != nil {
				t.Fatalf("parse decrypted private key failed: %v", err)
			}
			if signer.PublicKey().Type() != tc.expectType {
				t.Fatalf("expected key type %q, got %q", tc.expectType, signer.PublicKey().Type())
			}
		})
	}
}

func TestSSHServerController_UploadKey_WithPassphrase(t *testing.T) {
	app, ctrl := setupSSHServerTestApp(t)

	passwordCiphertext, err := secretservice.EncryptAESGCMBase64(ctrl.encryptionKey, "p@ss")
	if err != nil {
		t.Fatalf("encrypt password failed: %v", err)
	}

	server := model.SSHServer{
		ID:         "srv-upload-passphrase",
		Name:       "srv",
		Host:       "127.0.0.1",
		Port:       22,
		Username:   "root",
		AuthType:   "password",
		Password:   passwordCiphertext,
		LastStatus: "unknown",
	}
	if err := model.DB.Create(&server).Error; err != nil {
		t.Fatalf("create server failed: %v", err)
	}

	rsaKey, err := rsa.GenerateKey(rand.Reader, 1024)
	if err != nil {
		t.Fatalf("generate rsa key failed: %v", err)
	}

	block, err := x509.EncryptPEMBlock(rand.Reader, "RSA PRIVATE KEY", x509.MarshalPKCS1PrivateKey(rsaKey), []byte("p@ssphrase"), x509.PEMCipherAES256)
	if err != nil {
		t.Fatalf("encrypt pem block failed: %v", err)
	}
	encryptedPEM := pem.EncodeToMemory(block)

	req := newUploadKeyRequest(t, "/api/servers/"+server.ID+"/upload-key", "key", "id_rsa", encryptedPEM, "p@ssphrase")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("POST upload-key failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("expected status 200, got %d", resp.StatusCode)
	}

	var stored model.SSHServer
	if err := model.DB.First(&stored, "id = ?", server.ID).Error; err != nil {
		t.Fatalf("query stored server failed: %v", err)
	}
	if stored.Password != "" {
		t.Fatalf("expected password to be cleared")
	}
	if stored.PrivateKey == "" {
		t.Fatalf("expected private key to be stored")
	}
	if stored.Passphrase == "" {
		t.Fatalf("expected passphrase to be stored")
	}

	passphrasePlaintext, err := secretservice.DecryptAESGCMBase64(ctrl.encryptionKey, stored.Passphrase)
	if err != nil {
		t.Fatalf("decrypt stored passphrase failed: %v", err)
	}
	if passphrasePlaintext != "p@ssphrase" {
		t.Fatalf("expected passphrase %q, got %q", "p@ssphrase", passphrasePlaintext)
	}
}

func TestSSHServerController_UploadKey_PassphraseMissing(t *testing.T) {
	app, _ := setupSSHServerTestApp(t)

	server := model.SSHServer{
		ID:         "srv-upload-passphrase-missing",
		Name:       "srv",
		Host:       "127.0.0.1",
		Port:       22,
		Username:   "root",
		AuthType:   "password",
		Password:   "ciphertext",
		LastStatus: "unknown",
	}
	if err := model.DB.Create(&server).Error; err != nil {
		t.Fatalf("create server failed: %v", err)
	}

	rsaKey, err := rsa.GenerateKey(rand.Reader, 1024)
	if err != nil {
		t.Fatalf("generate rsa key failed: %v", err)
	}

	block, err := x509.EncryptPEMBlock(rand.Reader, "RSA PRIVATE KEY", x509.MarshalPKCS1PrivateKey(rsaKey), []byte("p@ssphrase"), x509.PEMCipherAES256)
	if err != nil {
		t.Fatalf("encrypt pem block failed: %v", err)
	}
	encryptedPEM := pem.EncodeToMemory(block)

	req := newUploadKeyRequest(t, "/api/servers/"+server.ID+"/upload-key", "key", "id_rsa", encryptedPEM, "")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("POST upload-key failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 400 {
		t.Fatalf("expected status 400, got %d", resp.StatusCode)
	}

	var body map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode response failed: %v", err)
	}
	if body["error"] != "Passphrase is required" {
		t.Fatalf("expected error %q, got %q", "Passphrase is required", body["error"])
	}

	var stored model.SSHServer
	if err := model.DB.First(&stored, "id = ?", server.ID).Error; err != nil {
		t.Fatalf("query stored server failed: %v", err)
	}
	if stored.PrivateKey != "" {
		t.Fatalf("expected private key to not be stored on error")
	}
}

func TestSSHServerController_UploadKey_InvalidKey(t *testing.T) {
	app, _ := setupSSHServerTestApp(t)

	server := model.SSHServer{
		ID:         "srv-upload-invalid",
		Name:       "srv",
		Host:       "127.0.0.1",
		Port:       22,
		Username:   "root",
		AuthType:   "password",
		Password:   "ciphertext",
		LastStatus: "unknown",
	}
	if err := model.DB.Create(&server).Error; err != nil {
		t.Fatalf("create server failed: %v", err)
	}

	req := newUploadKeyRequest(t, "/api/servers/"+server.ID+"/upload-key", "key", "id_invalid", []byte("not a key"), "")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("POST upload-key failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 400 {
		t.Fatalf("expected status 400, got %d", resp.StatusCode)
	}

	var body map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode response failed: %v", err)
	}
	if body["error"] != "Invalid private key" {
		t.Fatalf("expected error %q, got %q", "Invalid private key", body["error"])
	}

	var stored model.SSHServer
	if err := model.DB.First(&stored, "id = ?", server.ID).Error; err != nil {
		t.Fatalf("query stored server failed: %v", err)
	}
	if stored.PrivateKey != "" {
		t.Fatalf("expected private key to not be stored on error")
	}
}

func TestSSHServerController_UploadKey_ServerNotFound(t *testing.T) {
	app, _ := setupSSHServerTestApp(t)

	req := newUploadKeyRequest(t, "/api/servers/missing/upload-key", "key", "id_rsa", []byte("not a key"), "")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("POST upload-key failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 404 {
		t.Fatalf("expected status 404, got %d", resp.StatusCode)
	}
}

func newUploadKeyRequest(t *testing.T, url, fieldName, filename string, keyData []byte, passphrase string) *http.Request {
	t.Helper()

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)

	part, err := writer.CreateFormFile(fieldName, filename)
	if err != nil {
		t.Fatalf("create form file failed: %v", err)
	}
	if _, err := part.Write(keyData); err != nil {
		t.Fatalf("write key data failed: %v", err)
	}

	if passphrase != "" {
		if err := writer.WriteField("passphrase", passphrase); err != nil {
			t.Fatalf("write passphrase failed: %v", err)
		}
	}

	if err := writer.Close(); err != nil {
		t.Fatalf("close multipart writer failed: %v", err)
	}

	req := httptest.NewRequest("POST", url, &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("Content-Length", fmt.Sprintf("%d", body.Len()))
	return req
}

func TestSSHServerController_UploadKey_MissingKeyFile(t *testing.T) {
	app, _ := setupSSHServerTestApp(t)

	server := model.SSHServer{
		ID:         "srv-upload-missing-file",
		Name:       "srv",
		Host:       "127.0.0.1",
		Port:       22,
		Username:   "root",
		AuthType:   "password",
		Password:   "ciphertext",
		LastStatus: "unknown",
	}
	if err := model.DB.Create(&server).Error; err != nil {
		t.Fatalf("create server failed: %v", err)
	}

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	if err := writer.WriteField("passphrase", "any"); err != nil {
		t.Fatalf("write passphrase failed: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close multipart writer failed: %v", err)
	}

	req := httptest.NewRequest("POST", "/api/servers/"+server.ID+"/upload-key", &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("POST upload-key failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 400 {
		t.Fatalf("expected status 400, got %d", resp.StatusCode)
	}

	var responseBody map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&responseBody); err != nil {
		t.Fatalf("decode response failed: %v", err)
	}
	if responseBody["error"] != "Key file is required" {
		t.Fatalf("expected error %q, got %q", "Key file is required", responseBody["error"])
	}
}
