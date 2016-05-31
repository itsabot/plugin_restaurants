package restaurants

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/itsabot/abot/core"
	"github.com/itsabot/abot/shared/datatypes"
	"github.com/julienschmidt/httprouter"
	"github.com/labstack/gommon/log"
)

var r *httprouter.Router

func TestMain(m *testing.M) {
	err := os.Setenv("ABOT_ENV", "test")
	if err != nil {
		log.Fatal("failed to set ABOT_ENV.", err)
	}
	r, err = core.NewServer()
	if err != nil {
		log.Fatal("failed to start abot server.", err)
	}
	exitVal := m.Run()
	q := `DELETE FROM messages`
	_, err = p.DB.Exec(q)
	if err != nil {
		log.Info("failed to delete messages.", err)
	}
	q = `DELETE FROM states`
	_, err = p.DB.Exec(q)
	if err != nil {
		log.Info("failed to delete messages.", err)
	}
	os.Exit(exitVal)
}

func TestKWGetRestaurant(t *testing.T) {
	// Map test sentences with the important content that must be contained
	// in the reply.
	tests := map[string]string{
		"Find me a Thai restaurant in SF.": "How about",
		"Where's ramen in LA?":             "How about",
		"Where's a ramen restaurant?":      "Where are you now?",
		"Find me a restaurant":             "Where are you now?",
	}
	for test, expected := range tests {
		data := struct {
			FlexIDType int
			FlexID     string
			CMD        string
		}{
			FlexIDType: 3,
			FlexID:     "0",
			CMD:        test,
		}
		byt, err := json.Marshal(data)
		if err != nil {
			t.Fatal("failed to marshal req.", err)
		}
		c, b := request("POST", os.Getenv("ABOT_URL")+"/", byt)
		if c != http.StatusOK {
			t.Fatal("expected", http.StatusOK, "got", c, b)
		}
		if !strings.Contains(b, expected) {
			t.Fatalf("expected %q, got %q\n", expected, b)
		}
		in := core.NewMsg(&dt.User{ID: 1}, test)
		res := kwGetRestaurant(in)
		if len(expected) == 0 && len(res) > 0 {
			t.Fatalf("expected %q, got %q\n", expected, res)
		}
		if !strings.Contains(res, expected) {
			t.Fatalf("expected %q, got %q\n", expected, res)
		}
	}
}

func request(method, path string, data []byte) (int, string) {
	req, err := http.NewRequest(method, path, bytes.NewBuffer(data))
	if err != nil {
		return 0, "err completing request: " + err.Error()
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w.Code, string(w.Body.Bytes())
}
