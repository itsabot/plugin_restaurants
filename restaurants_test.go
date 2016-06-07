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
	"github.com/itsabot/abot/core/log"
	"github.com/julienschmidt/httprouter"
)

var r *httprouter.Router

func TestMain(m *testing.M) {
	cleanup()
	err := os.Setenv("ABOT_ENV", "test")
	if err != nil {
		log.Fatal("failed to set ABOT_ENV.", err)
	}
	r, err = core.NewServer()
	if err != nil {
		log.Fatal("failed to start abot server.", err)
	}
	exitVal := m.Run()
	cleanup()
	os.Exit(exitVal)
}

func TestKWRecommendRestaurants(t *testing.T) {
	// Map test sentences with the important content that must be contained
	// in the reply.
	seqTests := []string{
		"Find me a restaurant",
		"Where are you now?",
		"I'm in LA",
		"What kind of restaurant",
		"Thai",
		"How about",
		"No",
		"How about",
		"Yes",
		"Great! Let me know",
		"What's the address?",
		"The restaurant is at",
		"What's the phone number?",
		"The number is",
		"Show me some pics",
		"Their Yelp page probably has",
		"What are they rated?",
		"star rating on Yelp",
		"Can I see a menu?",
		"Yelp might have a menu for them",
		"Show me something else",
		"How about",
	}
	for i := 0; i+1 < len(seqTests); i += 2 {
		testReq(t, seqTests[i], seqTests[i+1])
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

func testReq(t *testing.T, in, exp string) {
	data := struct {
		FlexIDType int
		FlexID     string
		CMD        string
	}{
		FlexIDType: 3,
		FlexID:     "0",
		CMD:        in,
	}
	byt, err := json.Marshal(data)
	if err != nil {
		t.Fatal("failed to marshal req.", err)
	}
	c, b := request("POST", os.Getenv("ABOT_URL")+"/", byt)
	if c != http.StatusOK {
		t.Fatal("exp", http.StatusOK, "got", c, b)
	}
	if !strings.Contains(b, exp) {
		t.Fatalf("exp %q, got %q for %q\n", exp, b, in)
	}
}

func cleanup() {
	q := `DELETE FROM messages`
	_, err := p.DB.Exec(q)
	if err != nil {
		log.Info("failed to delete messages.", err)
	}
	q = `DELETE FROM states`
	_, err = p.DB.Exec(q)
	if err != nil {
		log.Info("failed to delete messages.", err)
	}
}
