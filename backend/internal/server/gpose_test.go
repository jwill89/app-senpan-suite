package server_test

import (
	"fmt"
	"net/http"
	"slices"
	"testing"
	"time"
)

// -- Gpose booths ---------------------------------------------------------------

func gposeBoothPath(id int, tail string) string {
	return fmt.Sprintf("/api/v1/gpose-booths/%d", id) + tail
}

// createGposeType adds a type to the site's list and answers its id.
func (e *testEnv) createGposeType(t *testing.T, name string) int {
	t.Helper()
	var made map[string]any
	decodeInto(t, e.postJSON(t, "/api/v1/gpose-types", map[string]any{"name": name}), http.StatusCreated, &made)
	return int(made["id"].(float64))
}

// gposeBoothBody is a full booth as a save sends it; over replaces fields of it.
func gposeBoothBody(over map[string]any) map[string]any {
	body := map[string]any{
		"title": "Lantern Gpose", "description": "Pay at the booth.", "mode": "basic", "maxSignups": 2,
		"maxWaitlist": 1, "perPerson": 1, "slotStep": 15, "types": []map[string]any{}, "windows": []map[string]any{},
	}
	for k, v := range over {
		body[k] = v
	}
	return body
}

// createGposeBooth makes a booth (201) and answers its id.
func (e *testEnv) createGposeBooth(t *testing.T, over map[string]any) int {
	t.Helper()
	var made map[string]any
	decodeInto(t, e.postJSON(t, "/api/v1/gpose-booths", gposeBoothBody(over)), http.StatusCreated, &made)
	return int(made["id"].(float64))
}

func gposeSignUpBody(name, world string, typeID int, over map[string]any) map[string]any {
	body := map[string]any{"characterName": name, "world": world, "typeId": typeID}
	for k, v := range over {
		body[k] = v
	}
	return body
}

// gposeSignUp signs a visitor up and answers the response.
func (e *testEnv) gposeSignUp(t *testing.T, boothID int, body map[string]any) *http.Response {
	t.Helper()
	return e.postJSON(t, gposeBoothPath(boothID, "/sign-up"), body)
}

// gposeSignups reads a booth's sign-ups as staff, by character name.
func (e *testEnv) gposeSignups(t *testing.T, boothID int) map[string]map[string]any {
	t.Helper()
	var list struct {
		Signups []map[string]any `json:"signups"`
	}
	decodeInto(t, e.get(t, gposeBoothPath(boothID, "/signups")), http.StatusOK, &list)
	out := make(map[string]map[string]any, len(list.Signups))
	for _, s := range list.Signups {
		out[s["characterName"].(string)] = s
	}
	return out
}

func signupPath(boothID int, s map[string]any, tail string) string {
	return gposeBoothPath(boothID, fmt.Sprintf("/signups/%d", int(s["id"].(float64)))+tail)
}

func num(v any) int { return int(v.(float64)) }

// Every staff operation needs the Gpose Booth page; the public ones need nothing.
func TestGposeGuard(t *testing.T) {
	env := newTestEnv(t)
	env.loginAdmin(t)
	typeID := env.createGposeType(t, "Solo")
	id := env.createGposeBooth(t, map[string]any{"types": []map[string]any{{"typeId": typeID, "price": 0}}})
	env.signOut(t)

	staff := []struct{ method, path string }{
		{http.MethodGet, "/api/v1/gpose-types"},
		{http.MethodPost, "/api/v1/gpose-types"},
		{http.MethodPut, fmt.Sprintf("/api/v1/gpose-types/%d", typeID)},
		{http.MethodDelete, fmt.Sprintf("/api/v1/gpose-types/%d", typeID)},
		{http.MethodGet, "/api/v1/gpose-booths"},
		{http.MethodPost, "/api/v1/gpose-booths"},
		{http.MethodGet, gposeBoothPath(id, "")},
		{http.MethodPut, gposeBoothPath(id, "")},
		{http.MethodDelete, gposeBoothPath(id, "")},
		{http.MethodPost, gposeBoothPath(id, "/close")},
		{http.MethodPost, gposeBoothPath(id, "/reopen")},
		{http.MethodGet, gposeBoothPath(id, "/signups")},
		{http.MethodPatch, gposeBoothPath(id, "/signups/1")},
		{http.MethodPost, gposeBoothPath(id, "/signups/1/call")},
		{http.MethodDelete, gposeBoothPath(id, "/signups/1")},
	}
	for _, c := range staff {
		if got := status(env.reqJSON(t, c.method, c.path, map[string]any{})); got != http.StatusUnauthorized {
			t.Errorf("%s %s signed out: %d, want 401", c.method, c.path, got)
		}
	}
	addAccount(t, env, "other", "correct horse battery", []string{"teahouse-raffles"})
	env.signIn(t, "other", "correct horse battery")
	for _, c := range staff {
		if got := status(env.reqJSON(t, c.method, c.path, map[string]any{})); got != http.StatusForbidden {
			t.Errorf("%s %s without the page: %d, want 403", c.method, c.path, got)
		}
	}

	addAccount(t, env, "gposer", "correct horse battery", []string{"festival-gpose"})
	env.signIn(t, "gposer", "correct horse battery")
	decodeInto(t, env.get(t, "/api/v1/gpose-booths"), http.StatusOK, nil)
	// The booth editor picks its festival and stall from the maps.
	decodeInto(t, env.get(t, "/api/v1/festival-maps"), http.StatusOK, nil)

	env.signOut(t)
	decodeInto(t, env.get(t, "/api/v1/gpose-booths/public"), http.StatusOK, nil)
	decodeInto(t, env.get(t, gposeBoothPath(id, "/public")), http.StatusOK, nil)
}

// A type's name is unique ignoring case, it can be renamed, and one a booth offers
// can't be deleted.
func TestGposeTypes(t *testing.T) {
	env := newTestEnv(t)
	env.loginAdmin(t)
	solo := env.createGposeType(t, " Solo ")
	couple := env.createGposeType(t, "Couple")
	if got := status(env.postJSON(t, "/api/v1/gpose-types", map[string]any{"name": "solo"})); got != http.StatusConflict {
		t.Errorf("a second Solo: %d, want 409", got)
	}
	if got := status(env.putJSON(t, fmt.Sprintf("/api/v1/gpose-types/%d", couple), map[string]any{"name": "SOLO"})); got != http.StatusConflict {
		t.Errorf("renaming onto Solo: %d, want 409", got)
	}
	var renamed map[string]any
	decodeInto(t, env.putJSON(t, fmt.Sprintf("/api/v1/gpose-types/%d", couple), map[string]any{"name": "Pair"}), http.StatusOK, &renamed)
	if renamed["name"] != "Pair" {
		t.Errorf("renamed to %v", renamed["name"])
	}
	if got := status(env.putJSON(t, "/api/v1/gpose-types/999", map[string]any{"name": "Group"})); got != http.StatusNotFound {
		t.Errorf("renaming no type: %d, want 404", got)
	}

	env.createGposeBooth(t, map[string]any{"types": []map[string]any{{"typeId": solo, "price": 1000}}})
	var list struct {
		Types []map[string]any `json:"types"`
	}
	decodeInto(t, env.get(t, "/api/v1/gpose-types"), http.StatusOK, &list)
	if len(list.Types) != 2 || list.Types[0]["name"] != "Solo" || num(list.Types[0]["boothCount"]) != 1 {
		t.Errorf("types %v", list.Types)
	}
	if got := status(env.del(t, fmt.Sprintf("/api/v1/gpose-types/%d", solo))); got != http.StatusConflict {
		t.Errorf("deleting an offered type: %d, want 409", got)
	}
	if got := status(env.del(t, fmt.Sprintf("/api/v1/gpose-types/%d", couple))); got != http.StatusNoContent {
		t.Errorf("deleting an unused type: %d, want 204", got)
	}
	if got := status(env.del(t, fmt.Sprintf("/api/v1/gpose-types/%d", couple))); got != http.StatusNotFound {
		t.Errorf("deleting it again: %d, want 404", got)
	}
}

// What the booth's rules can't take is refused, naming the field.
func TestGposeBoothValidation(t *testing.T) {
	env := newTestEnv(t)
	env.loginAdmin(t)
	solo := env.createGposeType(t, "Solo")
	cases := []map[string]any{
		{"types": []map[string]any{}},
		{"types": []map[string]any{{"typeId": 999, "price": 0}}},
		{"types": []map[string]any{{"typeId": solo, "price": 0}}, "maxSignups": 0},
		{"types": []map[string]any{{"typeId": solo, "price": 0, "minutes": 0}}, "mode": "timeslot",
			"windows": []map[string]any{{"startAt": "2026-08-01T18:00:00Z", "endAt": "2026-08-01T19:00:00Z"}}},
		{"types": []map[string]any{{"typeId": solo, "price": 0, "minutes": 15}}, "mode": "timeslot"},
		{"types": []map[string]any{{"typeId": solo, "price": 0}}, "festivalMapId": 999},
	}
	for i, c := range cases {
		if got := status(env.postJSON(t, "/api/v1/gpose-booths", gposeBoothBody(c))); got != http.StatusUnprocessableEntity {
			t.Errorf("case %d: %d, want 422", i, got)
		}
	}
	if got := status(env.get(t, gposeBoothPath(999, ""))); got != http.StatusNotFound {
		t.Errorf("no booth: %d, want 404", got)
	}
}

// A basic booth takes sign-ups in order up to its limit, then its waitlist; a place
// given up goes to the longest-waiting person on the waitlist.
func TestGposeBasicSignupsAndWaitlist(t *testing.T) {
	env := newTestEnv(t)
	env.loginAdmin(t)
	solo := env.createGposeType(t, "Solo")
	couple := env.createGposeType(t, "Couple")
	id := env.createGposeBooth(t, map[string]any{
		"types": []map[string]any{{"typeId": solo, "price": 50000}, {"typeId": couple, "price": 80000, "minutes": 30}},
	})
	pub := newClient(t, env)

	var first map[string]any
	decodeInto(t, env.gposeSignUp(t, id, gposeSignUpBody(" Aya Lantern ", "Gilgamesh", solo, nil)), http.StatusCreated, &first)
	if first["waitlisted"] != false || num(first["position"]) != 1 || num(first["price"]) != 50000 || num(first["minutes"]) != 0 {
		t.Errorf("first sign-up %v", first)
	}
	decodeInto(t, env.gposeSignUp(t, id, gposeSignUpBody("Bo Koi", "Sargatanas", couple, nil)), http.StatusCreated, nil)
	var third map[string]any
	decodeInto(t, env.gposeSignUp(t, id, gposeSignUpBody("Cid Ember", "Gilgamesh", solo, nil)), http.StatusCreated, &third)
	if third["waitlisted"] != true || num(third["position"]) != 1 {
		t.Errorf("third sign-up should be first on the waitlist: %v", third)
	}
	if got := status(env.gposeSignUp(t, id, gposeSignUpBody("Dee Moon", "Gilgamesh", solo, nil))); got != http.StatusConflict {
		t.Errorf("past the waitlist: %d, want 409", got)
	}
	if got := status(env.gposeSignUp(t, id, gposeSignUpBody("aya lantern", "GILGAMESH", couple, nil))); got != http.StatusConflict {
		t.Errorf("a second sign-up for one character: %d, want 409", got)
	}
	if got := status(env.gposeSignUp(t, id, gposeSignUpBody("Eve", "Gilgamesh", 999, nil))); got != http.StatusUnprocessableEntity {
		t.Errorf("a type the booth doesn't offer: %d, want 422", got)
	}
	if got := status(env.gposeSignUp(t, id, gposeSignUpBody("", "Gilgamesh", solo, nil))); got != http.StatusUnprocessableEntity {
		t.Errorf("no name: %d, want 422", got)
	}

	var public map[string]any
	decodeInto(t, env.get(t, gposeBoothPath(id, "/public")), http.StatusOK, &public)
	if num(public["placesLeft"]) != 0 || num(public["waitlistLeft"]) != 0 || len(public["types"].([]any)) != 2 {
		t.Errorf("public booth %v", public)
	}

	s := env.gposeSignups(t, id)
	aya, bo, cid := s["Aya Lantern"], s["Bo Koi"], s["Cid Ember"]
	if num(aya["position"]) != 1 || num(bo["position"]) != 2 || cid["waitlisted"] != true {
		t.Errorf("staff list %v", s)
	}

	// Completing keeps the place; a no-show gives it to the waitlist.
	decodeInto(t, env.patchJSON(t, signupPath(id, aya, ""), map[string]any{"status": "completed", "paid": true}), http.StatusOK, nil)
	if s := env.gposeSignups(t, id); s["Cid Ember"]["waitlisted"] != true || s["Aya Lantern"]["paid"] != true {
		t.Errorf("after completing Aya %v", s)
	}
	decodeInto(t, env.patchJSON(t, signupPath(id, bo, ""), map[string]any{"status": "no_show"}), http.StatusOK, nil)
	s = env.gposeSignups(t, id)
	if s["Cid Ember"]["waitlisted"] != false || num(s["Cid Ember"]["position"]) != 1 {
		t.Errorf("Cid should hold Bo's place: %v", s["Cid Ember"])
	}

	// Calling someone up shows on their own look at the booth; someone not waiting
	// can't be called.
	var called map[string]any
	decodeInto(t, env.postJSON(t, signupPath(id, cid, "/call"), nil), http.StatusOK, &called)
	if called["calledAt"] == nil {
		t.Errorf("called %v", called)
	}
	if got := status(env.postJSON(t, signupPath(id, aya, "/call"), nil)); got != http.StatusConflict {
		t.Errorf("calling a completed sign-up: %d, want 409", got)
	}
	var mine struct {
		Signups []map[string]any `json:"signups"`
	}
	decodeInto(t, reqAs(t, pub, env, http.MethodPost, gposeBoothPath(id, "/lookup"), map[string]any{"characterName": "cid ember", "world": "gilgamesh"}), http.StatusOK, &mine)
	if len(mine.Signups) != 1 || mine.Signups[0]["calledAt"] == nil || mine.Signups[0]["typeName"] != "Solo" {
		t.Errorf("Cid's own look %v", mine.Signups)
	}
	decodeInto(t, reqAs(t, pub, env, http.MethodPost, gposeBoothPath(id, "/lookup"), map[string]any{"characterName": "Cid", "world": "Gilgamesh"}), http.StatusOK, &mine)
	if len(mine.Signups) != 0 {
		t.Errorf("part of a name found %v", mine.Signups)
	}

	// Undoing the no-show is staff's to do, even with the places taken.
	decodeInto(t, env.patchJSON(t, signupPath(id, bo, ""), map[string]any{"status": "waiting"}), http.StatusOK, nil)
	if got := status(env.patchJSON(t, signupPath(id, bo, ""), map[string]any{"slotStart": "2026-08-01T18:00:00Z"})); got != http.StatusUnprocessableEntity {
		t.Errorf("a slot at a basic booth: %d, want 422", got)
	}
	if got := status(env.patchJSON(t, gposeBoothPath(id, "/signups/999"), map[string]any{"paid": true})); got != http.StatusNotFound {
		t.Errorf("no sign-up: %d, want 404", got)
	}

	// Deleting a place-holder hands its place on, and a booth with sign-ups keeps its mode.
	if got := status(env.del(t, signupPath(id, bo, ""))); got != http.StatusNoContent {
		t.Errorf("delete a sign-up: %d, want 204", got)
	}
	if got := status(env.putJSON(t, gposeBoothPath(id, ""), gposeBoothBody(map[string]any{
		"mode": "timeslot", "types": []map[string]any{{"typeId": solo, "price": 0, "minutes": 15}},
		"windows": []map[string]any{{"startAt": "2026-08-01T18:00:00Z", "endAt": "2026-08-01T19:00:00Z"}},
	}))); got != http.StatusConflict {
		t.Errorf("changing the mode with sign-ups: %d, want 409", got)
	}

	// A closed booth is gone from the public side.
	decodeInto(t, env.postJSON(t, gposeBoothPath(id, "/close"), nil), http.StatusOK, nil)
	if got := status(env.get(t, gposeBoothPath(id, "/public"))); got != http.StatusNotFound {
		t.Errorf("closed booth, public: %d, want 404", got)
	}
	if got := status(env.gposeSignUp(t, id, gposeSignUpBody("Fay", "Gilgamesh", solo, nil))); got != http.StatusNotFound {
		t.Errorf("signing up at a closed booth: %d, want 404", got)
	}
	var open struct {
		Booths []map[string]any `json:"booths"`
	}
	decodeInto(t, env.get(t, "/api/v1/gpose-booths/public"), http.StatusOK, &open)
	if len(open.Booths) != 0 {
		t.Errorf("open booths %v", open.Booths)
	}
	decodeInto(t, env.postJSON(t, gposeBoothPath(id, "/reopen"), nil), http.StatusOK, nil)
	decodeInto(t, env.get(t, "/api/v1/gpose-booths/public"), http.StatusOK, &open)
	if len(open.Booths) != 1 {
		t.Errorf("open booths after reopening %v", open.Booths)
	}
	if got := status(env.del(t, gposeBoothPath(id, ""))); got != http.StatusNoContent {
		t.Errorf("delete the booth: %d, want 204", got)
	}
}

// A timeslot booth offers start times on its step that fit each type, books them so no
// two overlap, keeps a waitlist, and lets staff put someone from the waitlist into a
// slot a no-show freed.
func TestGposeTimeslot(t *testing.T) {
	env := newTestEnv(t)
	env.loginAdmin(t)
	solo := env.createGposeType(t, "Solo")
	group := env.createGposeType(t, "Group")
	open := time.Now().UTC().Add(3 * time.Hour).Truncate(time.Hour)
	at := func(minutes int) string { return open.Add(time.Duration(minutes) * time.Minute).Format(time.RFC3339) }
	id := env.createGposeBooth(t, map[string]any{
		"mode": "timeslot", "slotStep": 15, "maxWaitlist": 2,
		"types":   []map[string]any{{"typeId": solo, "price": 0, "minutes": 15}, {"typeId": group, "price": 100000, "minutes": 30}},
		"windows": []map[string]any{{"startAt": at(0), "endAt": at(60)}},
	})
	starts := func() map[string][]string {
		var b struct {
			Types []struct {
				Name   string   `json:"name"`
				Starts []string `json:"starts"`
			} `json:"types"`
		}
		decodeInto(t, env.get(t, gposeBoothPath(id, "/public")), http.StatusOK, &b)
		out := map[string][]string{}
		for _, ty := range b.Types {
			out[ty.Name] = ty.Starts
		}
		return out
	}
	if s := starts(); !slices.Equal(s["Solo"], []string{at(0), at(15), at(30), at(45)}) || !slices.Equal(s["Group"], []string{at(0), at(15), at(30)}) {
		t.Errorf("starts %v", s)
	}

	decodeInto(t, env.gposeSignUp(t, id, gposeSignUpBody("Aya", "Gilgamesh", solo, map[string]any{"slotStart": at(0)})), http.StatusCreated, nil)
	if got := status(env.gposeSignUp(t, id, gposeSignUpBody("Bo", "Gilgamesh", group, map[string]any{"slotStart": at(0)}))); got != http.StatusConflict {
		t.Errorf("a taken time: %d, want 409", got)
	}
	if got := status(env.gposeSignUp(t, id, gposeSignUpBody("Bo", "Gilgamesh", group, map[string]any{"slotStart": at(5)}))); got != http.StatusConflict {
		t.Errorf("a time off the step: %d, want 409", got)
	}
	if got := status(env.gposeSignUp(t, id, gposeSignUpBody("Bo", "Gilgamesh", group, nil))); got != http.StatusUnprocessableEntity {
		t.Errorf("no time and no waitlist: %d, want 422", got)
	}
	var bo map[string]any
	decodeInto(t, env.gposeSignUp(t, id, gposeSignUpBody("Bo", "Gilgamesh", group, map[string]any{"slotStart": at(15)})), http.StatusCreated, &bo)
	if bo["slotStart"] != at(15) || bo["slotEnd"] != at(45) || num(bo["position"]) != 0 {
		t.Errorf("Bo's booking %v", bo)
	}
	decodeInto(t, env.gposeSignUp(t, id, gposeSignUpBody("Cid", "Gilgamesh", solo, map[string]any{"slotStart": at(45)})), http.StatusCreated, nil)
	if s := starts(); len(s["Solo"]) != 0 || len(s["Group"]) != 0 {
		t.Errorf("starts when full %v", s)
	}
	var wait map[string]any
	decodeInto(t, env.gposeSignUp(t, id, gposeSignUpBody("Dee", "Gilgamesh", solo, map[string]any{"waitlist": true})), http.StatusCreated, &wait)
	if wait["waitlisted"] != true || num(wait["position"]) != 1 || wait["slotStart"] != nil {
		t.Errorf("Dee's waitlist sign-up %v", wait)
	}

	// Bo doesn't show: their half hour is free again, and Dee's shoot fits it.
	s := env.gposeSignups(t, id)
	decodeInto(t, env.patchJSON(t, signupPath(id, s["Bo"], ""), map[string]any{"status": "no_show"}), http.StatusOK, nil)
	s = env.gposeSignups(t, id)
	if got := toStrings(s["Dee"]["assignableStarts"]); !slices.Equal(got, []string{at(15), at(30)}) {
		t.Errorf("Dee could go at %v", got)
	}
	if got := status(env.patchJSON(t, signupPath(id, s["Dee"], ""), map[string]any{"slotStart": at(0)})); got != http.StatusConflict {
		t.Errorf("into Aya's slot: %d, want 409", got)
	}
	var dee map[string]any
	decodeInto(t, env.patchJSON(t, signupPath(id, s["Dee"], ""), map[string]any{"slotStart": at(20)}), http.StatusOK, &dee)
	if dee["waitlisted"] != false || dee["slotStart"] != at(20) || dee["slotEnd"] != at(35) {
		t.Errorf("Dee placed %v", dee)
	}
	if got := status(env.patchJSON(t, signupPath(id, s["Bo"], ""), map[string]any{"status": "waiting"})); got != http.StatusConflict {
		t.Errorf("Bo's slot taken since: %d, want 409", got)
	}
	if got := status(env.patchJSON(t, signupPath(id, s["Bo"], ""), map[string]any{"slotStart": at(30)})); got != http.StatusConflict {
		t.Errorf("a slot for someone not waiting: %d, want 409", got)
	}

	// The waitlist has its limit.
	decodeInto(t, env.gposeSignUp(t, id, gposeSignUpBody("Eve", "Gilgamesh", solo, map[string]any{"waitlist": true})), http.StatusCreated, nil)
	decodeInto(t, env.gposeSignUp(t, id, gposeSignUpBody("Fay", "Gilgamesh", solo, map[string]any{"waitlist": true})), http.StatusCreated, nil)
	if got := status(env.gposeSignUp(t, id, gposeSignUpBody("Gil", "Gilgamesh", solo, map[string]any{"waitlist": true}))); got != http.StatusConflict {
		t.Errorf("past the waitlist: %d, want 409", got)
	}
}

func toStrings(v any) []string {
	var out []string
	for _, s := range v.([]any) {
		out = append(out, s.(string))
	}
	return out
}

// A booth filed under a festival shows on its dashboard with the visitor's own
// sign-ups, and on the stall it is pinned to; deleting the festival unlinks it.
func TestGposeFestivalLink(t *testing.T) {
	env := newTestEnv(t)
	env.loginAdmin(t)
	f := env.createFestival(t)
	solo := env.createGposeType(t, "Solo")
	id := env.createGposeBooth(t, map[string]any{
		"festivalMapId": f.mapID, "occupantId": f.occupantID,
		"types": []map[string]any{{"typeId": solo, "price": 0}},
	})
	var detail map[string]any
	decodeInto(t, env.get(t, gposeBoothPath(id, "")), http.StatusOK, &detail)
	if num(detail["festivalMapId"]) != f.mapID || detail["stallName"] == "" {
		t.Errorf("booth %v", detail)
	}
	decodeInto(t, env.gposeSignUp(t, id, gposeSignUpBody("Aya Lantern", "Gilgamesh", solo, nil)), http.StatusCreated, nil)

	d := env.dashboard(t, f.mapID, map[string]any{"characterName": "Aya Lantern", "world": "Gilgamesh"})
	booths := d["gposeBooths"].([]any)
	if len(booths) != 1 {
		t.Fatalf("dashboard booths %v", booths)
	}
	booth := booths[0].(map[string]any)
	if booth["booth"].(map[string]any)["festivalPath"] == "" || len(booth["signups"].([]any)) != 1 {
		t.Errorf("dashboard booth %v", booth)
	}

	var m map[string]any
	decodeInto(t, env.get(t, fmt.Sprintf("/api/v1/festival-maps/public/%d", f.mapID)), http.StatusOK, &m)
	pinned := firstOccupant(stallsOf(m)[0])["gposeBooth"]
	if pinned == nil || num(pinned.(map[string]any)["id"]) != id {
		t.Errorf("stall's gpose booth %v", pinned)
	}

	if got := status(env.del(t, mapPathOf(f.mapID))); got != http.StatusNoContent {
		t.Fatalf("delete the festival: %d", got)
	}
	decodeInto(t, env.get(t, gposeBoothPath(id, "")), http.StatusOK, &detail)
	if detail["festivalMapId"] != nil || detail["occupantId"] != nil {
		t.Errorf("booth still linked %v", detail)
	}
}

// A save replaces a booth's fields, types and open times, keeping its status; a booth
// that isn't there is a 404 to every staff operation.
func TestGposeBoothUpdate(t *testing.T) {
	env := newTestEnv(t)
	env.loginAdmin(t)
	solo := env.createGposeType(t, "Solo")
	group := env.createGposeType(t, "Group")
	id := env.createGposeBooth(t, map[string]any{"types": []map[string]any{{"typeId": solo, "price": 100}}})
	decodeInto(t, env.postJSON(t, gposeBoothPath(id, "/close"), nil), http.StatusOK, nil)

	var saved map[string]any
	decodeInto(t, env.putJSON(t, gposeBoothPath(id, ""), gposeBoothBody(map[string]any{
		"title": " Portrait Studio ", "mode": "timeslot", "slotStep": 30, "maxWaitlist": 0,
		"types": []map[string]any{{"typeId": group, "price": 9000, "minutes": 60}, {"typeId": solo, "minutes": 30}},
		"windows": []map[string]any{
			{"startAt": "2026-08-02T18:00:00+02:00", "endAt": "2026-08-02T20:00:00+02:00"},
			{"startAt": "2026-08-01T18:00:00Z", "endAt": "2026-08-01T19:00:00Z"},
		},
	})), http.StatusOK, &saved)
	types := saved["types"].([]any)
	windows := saved["windows"].([]any)
	if saved["title"] != "Portrait Studio" || saved["status"] != "closed" || num(saved["maxSignups"]) != 0 ||
		len(types) != 2 || types[0].(map[string]any)["name"] != "Group" || num(types[1].(map[string]any)["price"]) != 0 ||
		len(windows) != 2 || windows[0].(map[string]any)["startAt"] != "2026-08-01T18:00:00Z" ||
		windows[1].(map[string]any)["startAt"] != "2026-08-02T16:00:00Z" {
		t.Errorf("saved booth %v", saved)
	}

	var list struct {
		Booths []map[string]any `json:"booths"`
	}
	decodeInto(t, env.get(t, "/api/v1/gpose-booths"), http.StatusOK, &list)
	if len(list.Booths) != 1 || len(list.Booths[0]["types"].([]any)) != 0 {
		t.Errorf("list %v", list.Booths)
	}

	missing := gposeBoothPath(999, "")
	for _, c := range []struct {
		method, path string
		body         any
	}{
		{http.MethodPut, missing, gposeBoothBody(map[string]any{"types": []map[string]any{{"typeId": solo}}})},
		{http.MethodPost, gposeBoothPath(999, "/close"), nil},
		{http.MethodPost, gposeBoothPath(999, "/reopen"), nil},
		{http.MethodDelete, missing, nil},
		{http.MethodGet, gposeBoothPath(999, "/signups"), nil},
		{http.MethodPost, gposeBoothPath(id, "/signups/999/call"), nil},
		{http.MethodDelete, gposeBoothPath(id, "/signups/999"), nil},
		{http.MethodGet, gposeBoothPath(999, "/public"), nil},
	} {
		if got := status(env.reqJSON(t, c.method, c.path, c.body)); got != http.StatusNotFound {
			t.Errorf("%s %s: %d, want 404", c.method, c.path, got)
		}
	}
	// A closed booth takes no visitor's lookup either.
	body := map[string]any{"characterName": "Aya", "world": "Gilgamesh"}
	if got := status(env.postJSON(t, gposeBoothPath(id, "/lookup"), body)); got != http.StatusNotFound {
		t.Errorf("lookup at a closed booth: %d, want 404", got)
	}
}
