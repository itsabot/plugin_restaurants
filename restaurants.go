package restaurants

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"

	"github.com/garyburd/go-oauth/oauth"
	"github.com/itsabot/abot/core/log"
	"github.com/itsabot/abot/shared/datatypes"
	"github.com/itsabot/abot/shared/language"
	"github.com/itsabot/abot/shared/plugin"
	"github.com/itsabot/abot/shared/prefs"
	"github.com/itsabot/abot/shared/task"
)

type client struct {
	client oauth.Client
	token  oauth.Credentials
}

type yelpResp struct {
	Businesses []struct {
		Name         string
		ImageURL     string `json:"image_url"`
		MobileURL    string `json:"mobile_url"`
		DisplayPhone string `json:"display_phone"`
		Distance     int
		Rating       float64
		Location     struct {
			City           string
			DisplayAddress []string `json:"display_address"`
		}
	}
}

var ErrNoBusinesses = errors.New("no businesses")

var c client
var p *dt.Plugin
var foodMap map[string]struct{} = map[string]struct{}{}

func init() {
	c.client.Credentials.Token = os.Getenv("YELP_CONSUMER_KEY")
	c.client.Credentials.Secret = os.Getenv("YELP_CONSUMER_SECRET")
	c.token.Token = os.Getenv("YELP_TOKEN")
	c.token.Secret = os.Getenv("YELP_TOKEN_SECRET")
	for _, food := range language.Foods() {
		foodMap[food] = struct{}{}
	}
	var err error
	p, err = plugin.New("github.com/itsabot/plugin_restaurants")
	if err != nil {
		log.Fatal("building", err)
	}
	plugin.SetKeywords(p,
		dt.KeywordHandler{
			Fn: kwGetRestaurant,
			Trigger: &dt.StructuredInput{
				Commands: []string{
					"find",
					"what",
					"where",
					"show",
					"recommend",
					"recommendation",
				},
				Objects: language.Foods(),
			},
		},
	)
	plugin.SetStates(p, [][]dt.State{
		[]dt.State{
			{
				OnEntry: func(in *dt.Msg) string {
					return "Where are you now?"
				},
				OnInput: func(in *dt.Msg) {
					p.SetMemory(in, prefs.Location, in.Sentence)
				},
				Complete: func(in *dt.Msg) (bool, string) {
					return p.HasMemory(in, prefs.Location), ""
				},
				SkipIfComplete: true,
			},
			{
				OnEntry: func(in *dt.Msg) string {
					return "Ok. What kind of restaurant are you looking for?"
				},
				OnInput: func(in *dt.Msg) {
					restaurants := recommendRestaurants(in)
					p.SetMemory(in, "restaurantSearchResults",
						restaurants)
					p.SetMemory(in, "restaurantSearchResultsStrings",
						restaurants)
				},
				Complete: func(in *dt.Msg) (bool, string) {
					return p.HasMemory(in, "restaurantSearchResults"), ""
				},
			},
		},
		task.Iterate(p, "", task.OptsIterate{
			IterableMemKey: "restaurantSearchResults",
			ResultMemKey:   "selectedRestaurant",
		}),
		{
			{
				OnEntry: func(in *dt.Msg) string {
					mem := p.GetMemory(in, "selectedRestaurant")
					return "You selected " + mem.String()
				},
				OnInput: func(in *dt.Msg) {

				},
				Complete: func(in *dt.Msg) (bool, string) {
					return true, ""
				},
			},
		},
	})
	p.SM.SetOnReset(func(in *dt.Msg) {
		p.DeleteMemory(in, "restaurantSearchResults")
		p.DeleteMemory(in, "selectedRestaurant")
		task.ResetIterate(p, in)
	})
	if err = plugin.Register(p); err != nil {
		p.Log.Fatal("failed to register restaurants plugin.", err)
	}
}

func kwGetRestaurant(in *dt.Msg) string {
	var foods string
	for _, t := range in.Tokens {
		_, ok := foodMap[t]
		if ok {
			foods += t + " "
			continue
		}
	}
	cities, _ := language.ExtractCities(p.DB, in)
	if len(cities) > 0 {
		p.SetMemory(in, prefs.Location, cities[0].Name)
	}
	return ""
}

func recommendRestaurants(in *dt.Msg) []string {
	form := url.Values{
		"term":     {in.Sentence + " restaurant"},
		"location": {p.GetMemory(in, prefs.Location).String()},
		"limit":    {fmt.Sprintf("%.0f", 10)},
	}
	var data yelpResp
	err := c.get("http://api.yelp.com/v2/search", form, &data)
	if err != nil {
		/*
			m.Sentence = "I can't find that for you now. " +
				"Let's try again later."
			return err
		*/
		// return for confused response, given Yelp errors are rare, but
		// unintentional runs of Yelp queries are much more common
		return nil
	}
	var names []string
	for _, biz := range data.Businesses {
		names = append(names, biz.Name)
	}
	return names
}

func (c *client) get(urlStr string, params url.Values, v interface{}) error {
	resp, err := c.client.Get(nil, &c.token, urlStr, params)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return fmt.Errorf("yelp status %d", resp.StatusCode)
	}
	return json.NewDecoder(resp.Body).Decode(v)
}

/*
	// Responses were returned, and the user has asked this plugin an
	// additional query. Handle the query by keyword
	words := strings.Fields(*resp)
	offI := int(m.State["offset"].(float64))
	var s string
	for _, w := range words {
		w = strings.TrimRight(w, ").,;?!:")
		switch strings.ToLower(w) {
		case "rated", "rating", "review", "recommend", "recommended":
			s = fmt.Sprintf("It has a %s star review on Yelp",
				getRating(m, offI))
			*resp = s
		case "number", "phone":
			s = getPhone(m, offI)
			*resp = s
		case "call":
			s = fmt.Sprintf("You can reach them here: %s",
				getPhone(m, offI))
			*resp = s
		case "information", "info":
			s = fmt.Sprintf("Here's some more info: %s",
				getURL(m, offI))
			*resp = s
		case "where", "location", "address", "direction", "directions",
			"addr":
			s = fmt.Sprintf("It's at %s", getAddress(m, offI))
			*resp = s
		case "pictures", "pic", "pics":
			s = fmt.Sprintf("I found some pics here: %s",
				getURL(m, offI))
			*resp = s
		case "menu", "have":
			s = fmt.Sprintf("Yelp might have a menu... %s",
				getURL(m, offI))
			*resp = s
		case "not", "else", "no", "anything", "something":
			m.State["offset"] = float64(offI + 1)
			if err := t.searchYelp(m, resp); err != nil {
				return err
			}
		// TODO perhaps handle this case and "thanks" at the Abot level?
		// with bayesian classification
		case "good", "great", "yes", "perfect":
			// TODO feed into learning engine
			*resp = language.Positive()
		case "thanks", "thank":
			*resp = language.Welcome()
		}
		if len(*resp) > 0 {
			return nil
		}
	}
	/*

func getRating(r *dt.Msg, offset int) string {
	businesses := r.State["businesses"].([]interface{})
	firstBusiness := businesses[offset].(map[string]interface{})
	return fmt.Sprintf("%.1f", firstBusiness["Rating"].(float64))
}

func getURL(r *dt.Msg, offset int) string {
	businesses := r.State["businesses"].([]interface{})
	firstBusiness := businesses[offset].(map[string]interface{})
	return firstBusiness["mobile_url"].(string)
}

func getPhone(r *dt.Msg, offset int) string {
	businesses := r.State["businesses"].([]interface{})
	firstBusiness := businesses[offset].(map[string]interface{})
	return firstBusiness["display_phone"].(string)
}

func getAddress(r *dt.Msg, offset int) string {
	businesses := r.State["businesses"].([]interface{})
	firstBusiness := businesses[offset].(map[string]interface{})
	location := firstBusiness["Location"].(map[string]interface{})
	dispAddr := location["display_address"].([]interface{})
	if len(dispAddr) > 1 {
		str1 := dispAddr[0].(string)
		str2 := dispAddr[1].(string)
		return fmt.Sprintf("%s in %s", str1, str2)
	}
	return dispAddr[0].(string)
}

*/
