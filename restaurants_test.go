package restaurants

import (
	"os"
	"testing"

	"github.com/itsabot/abot/shared/plugin"
	"github.com/julienschmidt/httprouter"
)

var r *httprouter.Router

func TestMain(m *testing.M) {
	r = plugin.TestPrepare()
	os.Exit(m.Run())
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
		err := plugin.TestReq(r, seqTests[i], seqTests[i+1])
		if err != nil {
			t.Fatal(err)
		}
	}
}
