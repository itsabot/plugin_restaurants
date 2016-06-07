package restaurants

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"strings"

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

type business struct {
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

type yelpResp struct {
	Businesses []business
}

var c client
var p *dt.Plugin
var foodMap map[string]struct{} = map[string]struct{}{}

func init() {
	c.client.Credentials.Token = os.Getenv("YELP_CONSUMER_KEY")
	c.client.Credentials.Secret = os.Getenv("YELP_CONSUMER_SECRET")
	c.token.Token = os.Getenv("YELP_TOKEN")
	c.token.Secret = os.Getenv("YELP_TOKEN_SECRET")
	foods := language.Join(
		language.Foods(),
		language.Restaurants(),
		language.Desserts(),
	)
	for _, food := range foods {
		foodMap[food] = struct{}{}
	}
	var err error
	p, err = plugin.New("github.com/itsabot/plugin_restaurants")
	if err != nil {
		log.Fatal("building", err)
	}
	plugin.SetKeywords(p,
		dt.KeywordHandler{
			Fn: kwRecommendRestaurants,
			Trigger: &dt.StructuredInput{
				Commands: []string{
					"find",
					"what",
					"where",
					"show",
					"recommend",
					"recommendation",
				},
				Objects: foods,
			},
		},
		dt.KeywordHandler{
			Fn: kwGetPhone,
			Trigger: &dt.StructuredInput{
				Commands: []string{
					"find",
					"what",
					"show",
					"call",
					"have",
				},
				Objects: []string{"phone", "number"},
			},
		},
		dt.KeywordHandler{
			Fn: kwGetAddress,
			Trigger: &dt.StructuredInput{
				Commands: []string{
					"find",
					"what",
					"show",
					"navigate",
					"where",
					"have",
				},
				Objects: []string{"address", "directions", "located"},
			},
		},
		dt.KeywordHandler{
			Fn: kwGetRating,
			Trigger: &dt.StructuredInput{
				Commands: []string{
					"find",
					"what",
					"show",
					"have",
				},
				Objects: []string{"rating", "rated", "review"},
			},
		},
		dt.KeywordHandler{
			Fn: kwGetPictures,
			Trigger: &dt.StructuredInput{
				Commands: []string{
					"find",
					"show",
					"have",
					"see",
				},
				Objects: []string{"pictures", "pics"},
			},
		},
		dt.KeywordHandler{
			Fn: kwGetMenu,
			Trigger: &dt.StructuredInput{
				Commands: []string{
					"find",
					"what",
					"show",
					"have",
					"see",
				},
				Objects: []string{"menu"},
			},
		},
		dt.KeywordHandler{
			Fn: kwIterate,
			Trigger: &dt.StructuredInput{
				Commands: []string{
					"find",
					"what",
					"show",
					"have",
					"see",
				},
				Objects: []string{"something", "else", "different"},
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
					extractAndSaveCities(in)
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
					_ = kwRecommendRestaurants(in)
				},
				Complete: func(in *dt.Msg) (bool, string) {
					var complete bool
					if p.HasMemory(in, "restaurantType") {
						complete = true
					}
					return complete, ""
				},
				SkipIfComplete: true,
			},
		},
		task.Iterate(p, "iterate", task.OptsIterate{
			IterableMemKey:  "restaurantSearchResultsStrings",
			ResultMemKeyIdx: "selectedRestaurantIdx",
		}),
		{
			{
				OnEntry: func(in *dt.Msg) string {
					return "Great! Let me know if you'd like to get the address or phone number, or see reviews, pictures, or the menu."
				},
				OnInput: func(in *dt.Msg) {},
				Complete: func(in *dt.Msg) (bool, string) {
					// Notice that we keep the user in this
					// state. This allows the user to
					// continue asking questions without
					// memory being reset. We do this
					// because when the user selects a
					// restaurant, they usually aren't
					// "done," since they'll now need the
					// phone number, etc.
					return false, ""
				},
			},
		},
	})
	p.SM.SetOnReset(func(in *dt.Msg) {
		task.ResetIterate(p, in)
		p.DeleteMemory(in, "restaurantSearchResults")
		p.DeleteMemory(in, "restaurantSearchResultsStrings")
		p.DeleteMemory(in, "selectedRestaurantIdx")
		p.DeleteMemory(in, "restaurantType")
	})
	if err = plugin.Register(p); err != nil {
		p.Log.Fatal("failed to register restaurants plugin.", err)
	}
}

func kwRecommendRestaurants(in *dt.Msg) string {
	var foods string
	for _, t := range in.Tokens {
		_, ok := foodMap[strings.ToLower(t)]
		if ok {
			foods += t + " "
			continue
		}
	}
	p.SetMemory(in, "restaurantType", foods)
	extractAndSaveCities(in)
	if len(foods) == 0 || !p.HasMemory(in, prefs.Location) {
		return ""
	}
	form := url.Values{
		"term":     {foods + " restaurant"},
		"location": {p.GetMemory(in, prefs.Location).String()},
		"limit":    {fmt.Sprintf("%.0f", 10.0)},
	}
	var data yelpResp
	err := c.get("http://api.yelp.com/v2/search", form, &data)
	if err != nil {
		log.Info("failed to make yelp query.", err)
		return ""
	}
	var names []string
	for _, biz := range data.Businesses {
		names = append(names, biz.Name)
	}
	p.SetMemory(in, "restaurantSearchResults", data.Businesses)
	p.SetMemory(in, "restaurantSearchResultsStrings", names)
	return ""
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

func extractAndSaveCities(in *dt.Msg) {
	cities, _ := language.ExtractCities(p.DB, in)
	if len(cities) > 0 {
		p.SetMemory(in, prefs.Location, cities[0].Name)
	}
}

func kwGetPhone(in *dt.Msg) string {
	biz, err := getBusiness(in)
	if err != nil {
		return ""
	}
	return "The number is " + biz.DisplayPhone
}

func kwGetAddress(in *dt.Msg) string {
	biz, err := getBusiness(in)
	if err != nil {
		return ""
	}
	addr := strings.Join(biz.Location.DisplayAddress, "\n")
	return "The restaurant is at:\n\n" + addr
}

func kwGetPictures(in *dt.Msg) string {
	biz, err := getBusiness(in)
	if err != nil {
		return ""
	}
	return "Their Yelp page probably has some great pictures: " + biz.ImageURL
}

func kwGetRating(in *dt.Msg) string {
	biz, err := getBusiness(in)
	if err != nil {
		return ""
	}
	f := fmt.Sprintf("%.1f", biz.Rating)
	return "They have a " + f + " star rating on Yelp."
}

func kwGetMenu(in *dt.Msg) string {
	biz, err := getBusiness(in)
	if err != nil {
		return ""
	}
	return "Yelp might have a menu for them: " + biz.MobileURL
}

func kwIterate(in *dt.Msg) string {
	return p.SM.SetState(in, "iterate")
}

func getBusinesses(in *dt.Msg) []business {
	var businesses []business
	mem := p.GetMemory(in, "restaurantSearchResults")
	err := json.Unmarshal(mem.Val, &businesses)
	if err != nil {
		log.Info("failed to unmarshal businesses.", err)
		return nil
	}
	return businesses
}

func getBusiness(in *dt.Msg) (*business, error) {
	idx := int64(-1)
	if p.HasMemory(in, "selectedRestaurantIdx") {
		idx = p.GetMemory(in, "selectedRestaurantIdx").Int64()
	}
	if idx < 0 {
		return nil, errors.New("missing business")
	}
	businesses := getBusinesses(in)
	if businesses == nil {
		return nil, errors.New("missing businesses")
	}
	return &businesses[idx], nil
}
