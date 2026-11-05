package api

import (
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/awlx/bmwtools/pkg/database"
	"github.com/gin-gonic/gin"
)

const testDemoFile = "../../FINAL_DEMO_CHARGING_DATA_SMOOTH_CURVES.JSON"

type testServer struct {
	t      *testing.T
	router *gin.Engine
	db     *database.Manager
	cookie *http.Cookie
}

func newTestServer(t *testing.T) *testServer {
	t.Helper()
	gin.SetMode(gin.TestMode)
	demoDataFile = testDemoFile
	db, err := database.New(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	h := NewHandler(nil, db)
	r := gin.New()
	r.POST("/api/upload", h.UploadJSON)
	r.GET("/api/demo", h.LoadDemoData)
	r.POST("/api/contribute", h.Contribute)
	r.GET("/api/contribute", h.GetContributeStatus)
	return &testServer{t: t, router: r, db: db}
}

func (s *testServer) do(req *http.Request) (int, map[string]interface{}) {
	s.t.Helper()
	if s.cookie != nil {
		req.AddCookie(s.cookie)
	}
	w := httptest.NewRecorder()
	s.router.ServeHTTP(w, req)
	for _, c := range w.Result().Cookies() {
		if c.Name == "session_id" {
			s.cookie = c
		}
	}
	var body map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		s.t.Fatalf("bad JSON %q: %v", w.Body.String(), err)
	}
	return w.Code, body
}

func (s *testServer) upload(consent, model string) (int, map[string]interface{}) {
	s.t.Helper()
	content, err := os.ReadFile(testDemoFile)
	if err != nil {
		s.t.Fatal(err)
	}
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	fw, _ := mw.CreateFormFile("file", "BMW-CarData-Ladehistorie_WBA12345678901234_2025.json")
	fw.Write(content)
	mw.WriteField("consent", consent)
	mw.WriteField("model", model)
	mw.Close()
	req := httptest.NewRequest(http.MethodPost, "/api/upload", &buf)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	return s.do(req)
}

func (s *testServer) contribute(consent, model string) (int, map[string]interface{}) {
	s.t.Helper()
	form := url.Values{"consent": {consent}, "model": {model}}
	req := httptest.NewRequest(http.MethodPost, "/api/contribute", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	return s.do(req)
}

func (s *testServer) status() map[string]interface{} {
	s.t.Helper()
	_, body := s.do(httptest.NewRequest(http.MethodGet, "/api/contribute", nil))
	return body
}

func (s *testServer) storedSessions() int {
	s.t.Helper()
	n, err := s.db.GetSessionCount()
	if err != nil {
		s.t.Fatal(err)
	}
	return n
}

func TestContributeAfterUploadStoresData(t *testing.T) {
	s := newTestServer(t)
	if code, body := s.upload("false", ""); code != http.StatusOK || body["stored"] != false {
		t.Fatalf("upload = %d %v", code, body)
	}
	if n := s.storedSessions(); n != 0 {
		t.Fatalf("stored before consent = %d", n)
	}
	if st := s.status(); st["can_contribute"] != true || st["shared"] != false {
		t.Fatalf("status = %v", st)
	}

	code, body := s.contribute("true", "BMW i4")
	if code != http.StatusOK || body["new"] != true || body["stored"] != true {
		t.Fatalf("contribute = %d %v", code, body)
	}
	stored := s.storedSessions()
	if stored == 0 || float64(stored) != body["count"] {
		t.Fatalf("stored = %d, body = %v", stored, body)
	}
	if st := s.status(); st["can_contribute"] != false || st["shared"] != true {
		t.Fatalf("status after = %v", st)
	}

	// A later upload of the same file with consent stores nothing new.
	if code, body := s.upload("true", "BMW i4"); code != http.StatusOK || body["new"] != false {
		t.Errorf("re-upload = %d %v", code, body)
	}
	if n := s.storedSessions(); n != stored {
		t.Errorf("stored after re-upload = %d, want %d", n, stored)
	}
}

func TestContributeTwiceStoresNothingNew(t *testing.T) {
	s := newTestServer(t)
	s.upload("false", "")
	s.contribute("true", "BMW i4")
	stored := s.storedSessions()

	code, body := s.contribute("true", "BMW i4")
	if code != http.StatusOK || body["new"] != false {
		t.Fatalf("second contribute = %d %v", code, body)
	}
	if n := s.storedSessions(); n != stored {
		t.Errorf("stored = %d, want %d", n, stored)
	}
}

func TestContributeAfterConsentAtUploadStoresNothingNew(t *testing.T) {
	s := newTestServer(t)
	if code, body := s.upload("true", "BMW i4"); code != http.StatusOK || body["stored"] != true {
		t.Fatalf("upload = %d %v", code, body)
	}
	stored := s.storedSessions()
	if st := s.status(); st["shared"] != true || st["can_contribute"] != false {
		t.Fatalf("status = %v", st)
	}
	if code, body := s.contribute("true", "BMW i4"); code != http.StatusOK || body["new"] != false {
		t.Fatalf("contribute = %d %v", code, body)
	}
	if n := s.storedSessions(); n != stored {
		t.Errorf("stored = %d, want %d", n, stored)
	}
}

func TestContributeRejectsDemoData(t *testing.T) {
	s := newTestServer(t)
	if code, body := s.do(httptest.NewRequest(http.MethodGet, "/api/demo", nil)); code != http.StatusOK {
		t.Fatalf("demo = %d %v", code, body)
	}
	if st := s.status(); st["can_contribute"] != false || st["demo"] != true {
		t.Fatalf("status = %v", st)
	}
	if code, _ := s.contribute("true", "BMW i4"); code != http.StatusBadRequest {
		t.Errorf("contribute demo = %d, want 400", code)
	}

	// Upload first, then demo: the demo data replaces the upload.
	s.upload("false", "")
	s.do(httptest.NewRequest(http.MethodGet, "/api/demo", nil))
	if code, _ := s.contribute("true", "BMW i4"); code != http.StatusBadRequest {
		t.Errorf("contribute after demo = %d, want 400", code)
	}
	if n := s.storedSessions(); n != 0 {
		t.Errorf("stored = %d, want 0", n)
	}
}

func TestContributeWithoutDataFails(t *testing.T) {
	s := newTestServer(t)
	if code, _ := s.contribute("true", "BMW i4"); code != http.StatusBadRequest {
		t.Errorf("contribute without session = %d, want 400", code)
	}
	if st := s.status(); st["can_contribute"] != false {
		t.Errorf("status = %v", st)
	}
}

func TestContributeNeedsModelAndConsent(t *testing.T) {
	s := newTestServer(t)
	s.upload("false", "")
	for _, tc := range []struct{ consent, model string }{
		{"true", ""},
		{"true", "Tesla Model 3"},
		{"false", "BMW i4"},
	} {
		if code, _ := s.contribute(tc.consent, tc.model); code != http.StatusBadRequest {
			t.Errorf("contribute(%q, %q) = %d, want 400", tc.consent, tc.model, code)
		}
	}
	if n := s.storedSessions(); n != 0 {
		t.Errorf("stored = %d, want 0", n)
	}
}

func TestContributeUsesOnlyOwnSession(t *testing.T) {
	s := newTestServer(t)
	s.upload("false", "")
	// Another user without data must not share the first user's data.
	s.cookie = nil
	if code, _ := s.contribute("true", "BMW i4"); code != http.StatusBadRequest {
		t.Errorf("contribute from other user = %d, want 400", code)
	}
	s.cookie = &http.Cookie{Name: "session_id", Value: "guess"}
	if code, _ := s.contribute("true", "BMW i4"); code != http.StatusBadRequest {
		t.Errorf("contribute with unknown session = %d, want 400", code)
	}
	if n := s.storedSessions(); n != 0 {
		t.Errorf("stored = %d, want 0", n)
	}
}
