package api

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"mime"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humago"

	"app-suite/internal/db"
	"app-suite/internal/festival"
	"app-suite/internal/gpose"
	"app-suite/internal/media"
	"app-suite/internal/permissions"
	"app-suite/internal/uploads"
)

// Gpose booths: a photographer's booth people sign up at, for a type of shoot from the
// site's list. A basic booth takes sign-ups in order up to its limit, then a waitlist;
// a timeslot booth books start times inside its open hours, with a waitlist for when
// someone doesn't show. The Gpose Booth page's staff set the booths up and work their
// sign-ups; visitors sign up on the booth's page or the festival dashboard. Once a
// shoot is done, staff may upload its gpose, which the visitor downloads from the same
// pages for a day (media.GposePhotos keeps the files).

var gposeTags = []string{"Gpose Booths"}

// GposeType is a type of shoot the site's booths can offer.
type GposeType struct {
	ID         int64  `json:"id" doc:"The type's id."`
	Name       string `json:"name" doc:"What it is called, such as Solo or Couple."`
	BoothCount int64  `json:"boothCount" doc:"How many booths offer it; one that any booth offers can't be deleted."`
	CreatedAt  string `json:"createdAt" format:"date-time" doc:"When it was made, RFC 3339 in UTC."`
}

// GposeTypeInput is a type as a save sends it.
type GposeTypeInput struct {
	Name string `json:"name" minLength:"1" maxLength:"60" pattern:"[\\p{L}\\p{N}\\p{P}\\p{S}]" patternDescription:"more than spaces" doc:"What it is called. Two types can't share a name, ignoring case."`
}

// GposeTypeList is the site's types.
type GposeTypeList struct {
	Types []GposeType `json:"types" nullable:"false" doc:"Every type, oldest first."`
}

// GposeBoothType is a type a booth offers.
type GposeBoothType struct {
	TypeID  int64  `json:"typeId" doc:"The type's id."`
	Name    string `json:"name" doc:"The type's name."`
	Price   int64  `json:"price" doc:"What it costs in gil; 0 for no price."`
	Minutes int64  `json:"minutes" doc:"How long one takes at a timeslot booth; 0 at a basic booth."`
}

// GposeBoothTypeInput is a type a booth offers, as a save sends it.
type GposeBoothTypeInput struct {
	TypeID  int64 `json:"typeId" minimum:"1" doc:"The type's id. One that doesn't exist is refused."`
	Price   int64 `json:"price,omitempty" minimum:"0" maximum:"999999999" doc:"What it costs in gil; 0 or left out for no price."`
	Minutes int64 `json:"minutes,omitempty" minimum:"0" maximum:"600" doc:"How long one takes, 1 to 600 minutes at a timeslot booth; ignored at a basic booth."`
}

// GposeWindow is a time a timeslot booth is open.
type GposeWindow struct {
	StartAt string `json:"startAt" format:"date-time" doc:"When it opens, RFC 3339 in UTC."`
	EndAt   string `json:"endAt" format:"date-time" doc:"When it closes, RFC 3339 in UTC; a shoot must end by then."`
}

// GposeWindowInput is an open time as a save sends it.
type GposeWindowInput struct {
	StartAt string `json:"startAt" format:"date-time" doc:"When it opens, RFC 3339."`
	EndAt   string `json:"endAt" format:"date-time" doc:"When it closes, RFC 3339; after it opens. Open times can't overlap."`
}

// GposeBooth is a booth as the staff see it.
type GposeBooth struct {
	ID              int64            `json:"id" doc:"The booth's id."`
	Title           string           `json:"title" doc:"The booth's title."`
	Description     string           `json:"description" doc:"What it is about and how to pay (Markdown)."`
	Mode            string           `json:"mode" enum:"basic,timeslot" doc:"basic takes sign-ups in order up to maxSignups, then a waitlist. timeslot books start times inside its open times, with a waitlist beside them."`
	MaxSignups      int64            `json:"maxSignups" doc:"The most sign-ups a basic booth takes; 0 at a timeslot booth."`
	MaxWaitlist     int64            `json:"maxWaitlist" doc:"The most people its waitlist holds; 0 for no waitlist."`
	PerPerson       int64            `json:"perPerson" doc:"The most sign-ups one character may have waiting at once."`
	SlotStep        int64            `json:"slotStep" doc:"Minutes between the start times a timeslot booth offers."`
	Status          string           `json:"status" enum:"open,closed" doc:"An open booth takes sign-ups and is on the public pages; a closed one is neither, and its sign-ups stay workable."`
	FestivalMapID   *int64           `json:"festivalMapId" nullable:"true" doc:"The festival it belongs to; null for none."`
	FestivalMapName string           `json:"festivalMapName" doc:"The festival's title; empty for none."`
	OccupantID      *int64           `json:"occupantId" nullable:"true" doc:"The stall on that festival's plan it is pinned to; null for none."`
	StallName       string           `json:"stallName" doc:"The stall's title; empty for none."`
	CreatedAt       string           `json:"createdAt" format:"date-time" doc:"When it was made, RFC 3339 in UTC."`
	Types           []GposeBoothType `json:"types" nullable:"false" doc:"The types it offers, in order. Empty in a list."`
	Windows         []GposeWindow    `json:"windows" nullable:"false" doc:"When a timeslot booth is open, in order. Empty in a list and at a basic booth."`
	BookedCount     int64            `json:"bookedCount" doc:"Sign-ups holding a place or slot: waiting or completed, off the waitlist."`
	WaitlistCount   int64            `json:"waitlistCount" doc:"People waiting on the waitlist."`
	WaitingCount    int64            `json:"waitingCount" doc:"Sign-ups not yet completed, a no-show or canceled, the waitlist included."`
}

// GposeBoothInput is a booth as a save sends it: all of its editable fields, since a save
// replaces them. Its status changes through close and reopen.
type GposeBoothInput struct {
	Title         string                `json:"title" minLength:"1" maxLength:"200" pattern:"[\\p{L}\\p{N}\\p{P}\\p{S}]" patternDescription:"more than spaces" doc:"The booth's title."`
	Description   string                `json:"description" maxLength:"20000" doc:"What it is about and how to pay (Markdown); empty for none."`
	Mode          string                `json:"mode" enum:"basic,timeslot" doc:"basic or timeslot. A booth that has sign-ups can't change it."`
	MaxSignups    int64                 `json:"maxSignups" minimum:"0" maximum:"1000" doc:"The most sign-ups a basic booth takes, at least 1; ignored at a timeslot booth."`
	MaxWaitlist   int64                 `json:"maxWaitlist" minimum:"0" maximum:"1000" doc:"The most people its waitlist holds; 0 for no waitlist."`
	PerPerson     int64                 `json:"perPerson" minimum:"1" maximum:"20" doc:"The most sign-ups one character may have waiting at once."`
	SlotStep      int64                 `json:"slotStep" minimum:"0" maximum:"240" doc:"Minutes between the start times a timeslot booth offers, 5 to 240; ignored at a basic booth."`
	Types         []GposeBoothTypeInput `json:"types" minItems:"1" maxItems:"20" nullable:"false" doc:"The types it offers, in order, each once."`
	Windows       []GposeWindowInput    `json:"windows" maxItems:"60" nullable:"false" doc:"When a timeslot booth is open, at least one; ignored at a basic booth. Sign-ups already booked keep their times when these change."`
	FestivalMapID *int64                `json:"festivalMapId,omitempty" nullable:"true" minimum:"1" doc:"The festival to file it under; null or left out for none. One that doesn't exist is refused."`
	OccupantID    *int64                `json:"occupantId,omitempty" nullable:"true" minimum:"1" doc:"The stall on that festival's plan to pin it to; null or left out for none. A stall that isn't on the festival is dropped."`
}

// GposeBoothList is every booth.
type GposeBoothList struct {
	Booths []GposeBooth `json:"booths" nullable:"false" doc:"Newest first, with their counts and without their types or open times."`
}

// GposeSignup is a sign-up as the staff see it.
type GposeSignup struct {
	ID               int64    `json:"id" doc:"The sign-up's id."`
	BoothID          int64    `json:"boothId" doc:"The booth it is at."`
	TypeID           *int64   `json:"typeId" nullable:"true" doc:"The type signed up for; null once that type is deleted."`
	TypeName         string   `json:"typeName" doc:"The type's name, as it was when they signed up."`
	Price            int64    `json:"price" doc:"Its price in gil, as it was when they signed up; 0 for none."`
	Minutes          int64    `json:"minutes" doc:"How long it takes, as it was when they signed up; 0 at a basic booth."`
	CharacterName    string   `json:"characterName" doc:"The character."`
	World            string   `json:"world" doc:"Their home world."`
	Waitlisted       bool     `json:"waitlisted" doc:"Whether they are on the waitlist rather than in a place or a slot."`
	Position         int64    `json:"position" doc:"Where they stand while waiting: their place in line at a basic booth, or on the waitlist; 0 for a booked slot or once they are no longer waiting."`
	SlotStart        *string  `json:"slotStart" nullable:"true" format:"date-time" doc:"When their slot starts, RFC 3339 in UTC; null at a basic booth and on the waitlist."`
	SlotEnd          *string  `json:"slotEnd" nullable:"true" format:"date-time" doc:"When their slot ends, RFC 3339 in UTC; null with no slot."`
	Status           string   `json:"status" enum:"waiting,completed,no_show,canceled" doc:"waiting holds their place or slot, and so does completed; no_show and canceled give it up."`
	Paid             bool     `json:"paid" doc:"Whether staff recorded the price as paid."`
	CalledAt         *string  `json:"calledAt" nullable:"true" format:"date-time" doc:"When staff last called them up, RFC 3339 in UTC; null when never."`
	CreatedAt        string   `json:"createdAt" format:"date-time" doc:"When they signed up, RFC 3339 in UTC."`
	AssignableStarts []string `json:"assignableStarts" nullable:"false" doc:"For someone waiting on a timeslot booth's waitlist, the start times their shoot fits now, one under way included; empty for anyone else."`
	PhotoURL         string   `json:"photoUrl" doc:"The gpose uploaded for them to download; empty when none was, or once its day is over."`
	PhotoExpiresAt   *string  `json:"photoExpiresAt" nullable:"true" format:"date-time" doc:"When that gpose is deleted, RFC 3339 in UTC: a day after it was uploaded; null with none."`
}

// GposeSignupList is a booth's sign-ups.
type GposeSignupList struct {
	Signups []GposeSignup `json:"signups" nullable:"false" doc:"Every sign-up in the order they were made."`
}

// GposeSignupChange is what staff change on a sign-up. Every field is optional.
type GposeSignupChange struct {
	Status    *string `json:"status,omitempty" enum:"waiting,completed,no_show,canceled" doc:"Where it stands now. Waiting again undoes a mistake; at a timeslot booth, a slot another booking has taken since is a 409."`
	Paid      *bool   `json:"paid,omitempty" doc:"Whether the price is paid."`
	SlotStart *string `json:"slotStart,omitempty" format:"date-time" doc:"Books it into the slot starting then, taking it off the waitlist: a timeslot booth's only. The shoot must fit inside an open time without overlapping another booking; it may start between the offered times, or have started already."`
}

// PublicGposeType is a type a booth offers, as a visitor sees it.
type PublicGposeType struct {
	TypeID  int64    `json:"typeId" doc:"The type's id."`
	Name    string   `json:"name" doc:"The type's name."`
	Price   int64    `json:"price" doc:"What it costs in gil; 0 for no price."`
	Minutes int64    `json:"minutes" doc:"How long one takes; 0 at a basic booth."`
	Starts  []string `json:"starts" nullable:"false" doc:"At a timeslot booth, the times it can be booked now, RFC 3339 in UTC, in order; empty at a basic booth and in a list."`
}

// PublicGposeBooth is an open booth as a visitor sees it: never who signed up.
type PublicGposeBooth struct {
	ID           int64             `json:"id" doc:"The booth's id."`
	Title        string            `json:"title" doc:"The booth's title."`
	Description  string            `json:"description" doc:"What it is about and how to pay (Markdown)."`
	Mode         string            `json:"mode" enum:"basic,timeslot" doc:"basic or timeslot."`
	PerPerson    int64             `json:"perPerson" doc:"The most sign-ups one character may have waiting at once."`
	MaxSignups   int64             `json:"maxSignups" doc:"The most sign-ups a basic booth takes; 0 at a timeslot booth."`
	MaxWaitlist  int64             `json:"maxWaitlist" doc:"The most people its waitlist holds; 0 for no waitlist."`
	PlacesLeft   int64             `json:"placesLeft" doc:"The places a basic booth has left; 0 when full and at a timeslot booth."`
	WaitlistLeft int64             `json:"waitlistLeft" doc:"The room left on the waitlist."`
	Types        []PublicGposeType `json:"types" nullable:"false" doc:"The types it offers, in order."`
	Windows      []GposeWindow     `json:"windows" nullable:"false" doc:"When a timeslot booth is open, in order; empty at a basic booth."`
	FestivalName string            `json:"festivalName" doc:"The festival it belongs to; empty for none."`
	FestivalPath string            `json:"festivalPath" doc:"The festival's shortcode or id for its page, when that festival is published; empty otherwise."`
	StallName    string            `json:"stallName" doc:"The stall it is at; empty for none."`
}

// PublicGposeBoothList is the open booths.
type PublicGposeBoothList struct {
	Booths []PublicGposeBooth `json:"booths" nullable:"false" doc:"The open booths, newest first, without their start times."`
}

// PublicGposeSignup is a sign-up as the person who made it sees it.
type PublicGposeSignup struct {
	ID             int64   `json:"id" doc:"The sign-up's id."`
	BoothID        int64   `json:"boothId" doc:"The booth it is at."`
	TypeName       string  `json:"typeName" doc:"The type signed up for."`
	Price          int64   `json:"price" doc:"Its price in gil; 0 for none."`
	Minutes        int64   `json:"minutes" doc:"How long it takes; 0 at a basic booth."`
	Waitlisted     bool    `json:"waitlisted" doc:"Whether they are on the waitlist."`
	Position       int64   `json:"position" doc:"Their place in line at a basic booth, or on the waitlist, while waiting; 0 otherwise."`
	SlotStart      *string `json:"slotStart" nullable:"true" format:"date-time" doc:"When their slot starts, RFC 3339 in UTC; null with no slot."`
	SlotEnd        *string `json:"slotEnd" nullable:"true" format:"date-time" doc:"When it ends; null with no slot."`
	Status         string  `json:"status" enum:"waiting,completed,no_show,canceled" doc:"Where it stands."`
	Paid           bool    `json:"paid" doc:"Whether the price is recorded as paid."`
	CalledAt       *string `json:"calledAt" nullable:"true" format:"date-time" doc:"When staff last called them up, RFC 3339 in UTC; null when never."`
	CreatedAt      string  `json:"createdAt" format:"date-time" doc:"When they signed up, RFC 3339 in UTC."`
	PhotoURL       string  `json:"photoUrl" doc:"Their gpose, which staff uploaded once the shoot was done, to view and download; empty when there is none, or once its day is over."`
	PhotoExpiresAt *string `json:"photoExpiresAt" nullable:"true" format:"date-time" doc:"When it is deleted, RFC 3339 in UTC: a day after it was uploaded; null with none."`
}

// GposePhotoForm is the gpose staff upload for a completed sign-up.
type GposePhotoForm struct {
	Image huma.FormFile `form:"image" contentType:"image/*" required:"true" doc:"The gpose: a PNG, JPEG or WebP of at most 32 MB, 40 megapixels and 16384 pixels a side, judged from its bytes. A PNG or JPEG is saved again without its metadata."`
}

// GposeSignUpInput is a visitor's sign-up.
type GposeSignUpInput struct {
	CharacterName  string  `json:"characterName" maxLength:"60" doc:"Their character."`
	World          string  `json:"world" maxLength:"60" doc:"Their home world."`
	TypeID         int64   `json:"typeId" minimum:"1" doc:"The type of gpose, one the booth offers."`
	SlotStart      *string `json:"slotStart,omitempty" format:"date-time" doc:"At a timeslot booth, the start time to book, one of the type's offered starts. Left out with waitlist set, it joins the waitlist."`
	Waitlist       bool    `json:"waitlist,omitempty" doc:"At a timeslot booth, join the waitlist instead of booking a time. A basic booth puts them on its waitlist by itself once its places are taken."`
	TurnstileToken string  `json:"turnstileToken,omitempty" maxLength:"4096" doc:"The Cloudflare Turnstile response; needed only when the bot check is on."`
}

// GposeLookupInput is who is asking after their own sign-ups.
type GposeLookupInput struct {
	CharacterName string `json:"characterName" maxLength:"60" doc:"Their character, the whole name they signed up with (case doesn't matter)."`
	World         string `json:"world" maxLength:"60" doc:"Their home world."`
}

// GposeLookup is a visitor's own sign-ups at a booth.
type GposeLookup struct {
	Signups []PublicGposeSignup `json:"signups" nullable:"false" doc:"Theirs, oldest first; empty when they have none."`
}

type gposeTypeIDInput struct {
	ID int64 `path:"id" doc:"The type's id."`
}

type gposeTypeCreateInput struct{ Body GposeTypeInput }

type gposeTypeSaveInput struct {
	ID   int64 `path:"id" doc:"The type's id."`
	Body GposeTypeInput
}

type gposeBoothIDInput struct {
	ID int64 `path:"id" doc:"The booth's id."`
}

type gposeBoothCreateInput struct{ Body GposeBoothInput }

type gposeBoothSaveInput struct {
	ID   int64 `path:"id" doc:"The booth's id."`
	Body GposeBoothInput
}

type gposeSignupIDInput struct {
	ID       int64 `path:"id" doc:"The booth's id."`
	SignupID int64 `path:"signupId" doc:"The sign-up's id."`
}

type gposeSignupChangeInput struct {
	ID       int64 `path:"id" doc:"The booth's id."`
	SignupID int64 `path:"signupId" doc:"The sign-up's id."`
	Body     GposeSignupChange
}

type gposePhotoUploadInput struct {
	ID       int64 `path:"id" doc:"The booth's id."`
	SignupID int64 `path:"signupId" doc:"The sign-up's id."`
	RawBody  huma.MultipartFormFiles[GposePhotoForm]
}

type gposePhotoInput struct {
	Token string `path:"token" maxLength:"64" doc:"The gpose's token, from its sign-up's photoUrl."`
}

type gposeSignUpRequest struct {
	ID   int64 `path:"id" doc:"The booth's id."`
	Body GposeSignUpInput
}

type gposeLookupRequest struct {
	ID   int64 `path:"id" doc:"The booth's id."`
	Body GposeLookupInput
}

type gposeTypeOutput struct{ Body GposeType }

type gposeTypeCreatedOutput struct {
	Status int
	Body   GposeType
}

type gposeTypeListOutput struct{ Body GposeTypeList }

type gposeBoothOutput struct{ Body GposeBooth }

type gposeBoothCreatedOutput struct {
	Status int
	Body   GposeBooth
}

type gposeBoothListOutput struct{ Body GposeBoothList }

type gposeSignupOutput struct{ Body GposeSignup }

type gposeSignupListOutput struct{ Body GposeSignupList }

type publicGposeBoothOutput struct{ Body PublicGposeBooth }

type publicGposeBoothListOutput struct{ Body PublicGposeBoothList }

type gposeSignedUpOutput struct {
	Status int
	Body   PublicGposeSignup
}

type gposeLookupOutput struct{ Body GposeLookup }

func (h handlers) registerGpose(api huma.API) {
	page := huma.Middlewares{h.permitted(api, permissions.FestivalGpose)}
	guarded := []int{http.StatusUnauthorized, http.StatusForbidden}
	notFound := slices.Concat(guarded, []int{http.StatusNotFound})
	invalid := slices.Concat(notFound, []int{http.StatusConflict, http.StatusUnprocessableEntity})

	huma.Register(api, huma.Operation{
		OperationID: "list-gpose-types", Method: http.MethodGet, Path: "/gpose-types",
		Summary: "The site's gpose types", Tags: gposeTags, Security: secured, Middlewares: page, Errors: guarded,
	}, h.listGposeTypes)
	huma.Register(api, huma.Operation{
		OperationID: "create-gpose-type", Method: http.MethodPost, Path: "/gpose-types",
		Summary: "Add a gpose type", Description: "A name another type has, ignoring case, is a 409.",
		Tags: gposeTags, DefaultStatus: http.StatusCreated, Security: secured, Middlewares: page,
		Errors: slices.Concat(guarded, []int{http.StatusConflict, http.StatusUnprocessableEntity}),
	}, h.createGposeType)
	huma.Register(api, huma.Operation{
		OperationID: "update-gpose-type", Method: http.MethodPut, Path: "/gpose-types/{id}",
		Summary: "Rename a gpose type", Description: "Sign-ups already made keep the name they were made under.",
		Tags: gposeTags, Security: secured, Middlewares: page, Errors: invalid,
	}, h.updateGposeType)
	huma.Register(api, huma.Operation{
		OperationID: "delete-gpose-type", Method: http.MethodDelete, Path: "/gpose-types/{id}",
		Summary: "Delete a gpose type", Description: "One a booth still offers is a 409 naming how many.",
		Tags: gposeTags, DefaultStatus: http.StatusNoContent, Security: secured, Middlewares: page,
		Errors: slices.Concat(notFound, []int{http.StatusConflict}),
	}, h.deleteGposeType)

	huma.Register(api, huma.Operation{
		OperationID: "list-gpose-booths", Method: http.MethodGet, Path: "/gpose-booths",
		Summary: "Every gpose booth", Tags: gposeTags, Security: secured, Middlewares: page, Errors: guarded,
	}, h.listGposeBooths)
	huma.Register(api, huma.Operation{
		OperationID: "create-gpose-booth", Method: http.MethodPost, Path: "/gpose-booths",
		Summary: "Make a gpose booth", Description: "It starts open.", Tags: gposeTags,
		DefaultStatus: http.StatusCreated, Security: secured, Middlewares: page,
		Errors: slices.Concat(guarded, []int{http.StatusUnprocessableEntity}),
	}, h.createGposeBooth)
	huma.Register(api, huma.Operation{
		OperationID: "list-public-gpose-booths", Method: http.MethodGet, Path: "/gpose-booths/public",
		Summary: "The open gpose booths", Tags: gposeTags, Middlewares: huma.Middlewares{public},
	}, h.listPublicGposeBooths)
	huma.Register(api, huma.Operation{
		OperationID: "get-public-gpose-booth", Method: http.MethodGet, Path: "/gpose-booths/{id}/public",
		Summary: "One open gpose booth", Description: "With the times each type can be booked now. A closed booth is not found.",
		Tags: gposeTags, Middlewares: huma.Middlewares{public}, Errors: []int{http.StatusNotFound},
	}, h.getPublicGposeBooth)
	huma.Register(api, huma.Operation{
		OperationID: "get-gpose-booth", Method: http.MethodGet, Path: "/gpose-booths/{id}",
		Summary: "One gpose booth with its types and open times", Tags: gposeTags, Security: secured, Middlewares: page,
		Errors: notFound,
	}, h.getGposeBooth)
	huma.Register(api, huma.Operation{
		OperationID: "update-gpose-booth", Method: http.MethodPut, Path: "/gpose-booths/{id}",
		Summary: "Replace a gpose booth's editable fields",
		Description: "Its status is kept. A booth that has sign-ups can't change between basic and timeslot (409). " +
			"Sign-ups keep the type, price and time they were made with.",
		Tags: gposeTags, Security: secured, Middlewares: page, Errors: invalid,
	}, h.updateGposeBooth)
	huma.Register(api, huma.Operation{
		OperationID: "close-gpose-booth", Method: http.MethodPost, Path: "/gpose-booths/{id}/close",
		Summary:     "Close a gpose booth",
		Description: "It takes no sign-ups and leaves the public pages; its sign-ups stay workable.",
		Tags:        gposeTags, Security: secured, Middlewares: page, Errors: notFound,
	}, h.closeGposeBooth)
	huma.Register(api, huma.Operation{
		OperationID: "reopen-gpose-booth", Method: http.MethodPost, Path: "/gpose-booths/{id}/reopen",
		Summary: "Open a closed gpose booth again", Tags: gposeTags, Security: secured, Middlewares: page, Errors: notFound,
	}, h.reopenGposeBooth)
	huma.Register(api, huma.Operation{
		OperationID: "delete-gpose-booth", Method: http.MethodDelete, Path: "/gpose-booths/{id}",
		Summary: "Delete a gpose booth", Description: "Its sign-ups go with it.",
		Tags: gposeTags, DefaultStatus: http.StatusNoContent, Security: secured, Middlewares: page, Errors: notFound,
	}, h.deleteGposeBooth)
	huma.Register(api, huma.Operation{
		OperationID: "list-gpose-signups", Method: http.MethodGet, Path: "/gpose-booths/{id}/signups",
		Summary: "A gpose booth's sign-ups",
		Description: "In the order they were made, with each one's place in line and, for the waitlist of a timeslot " +
			"booth, the times their shoot could be fitted in now.",
		Tags: gposeTags, Security: secured, Middlewares: page, Errors: notFound,
	}, h.listGposeSignups)
	huma.Register(api, huma.Operation{
		OperationID: "change-gpose-signup", Method: http.MethodPatch, Path: "/gpose-booths/{id}/signups/{signupId}",
		Summary: "Mark a sign-up completed, a no-show or canceled, record it paid, or give it a slot",
		Description: "At a basic booth, a sign-up that gives up its place (a no-show or canceled) moves the longest-waiting " +
			"person on the waitlist into it. At a timeslot booth, slotStart books someone into a slot, the waitlist's " +
			"way into one a no-show freed.",
		Tags: gposeTags, Security: secured, Middlewares: page, Errors: invalid,
	}, h.changeGposeSignup)
	huma.Register(api, huma.Operation{
		OperationID: "call-gpose-signup", Method: http.MethodPost, Path: "/gpose-booths/{id}/signups/{signupId}/call",
		Summary: "Call a person up",
		Description: "Their festival dashboard and booth page alert them (a sound, a vibration, the tab's title) until " +
			"they dismiss it; calling again alerts them again. Only someone waiting can be called (409).",
		Tags: gposeTags, Security: secured, Middlewares: page, Errors: slices.Concat(notFound, []int{http.StatusConflict}),
	}, h.callGposeSignup)
	huma.Register(api, huma.Operation{
		OperationID: "delete-gpose-signup", Method: http.MethodDelete, Path: "/gpose-booths/{id}/signups/{signupId}",
		Summary:     "Delete a sign-up",
		Description: "For one made in error. At a basic booth, the place it held goes to the waitlist, as a cancellation does.",
		Tags:        gposeTags, DefaultStatus: http.StatusNoContent, Security: secured, Middlewares: page, Errors: notFound,
	}, h.deleteGposeSignup)
	huma.Register(api, huma.Operation{
		OperationID: "upload-gpose-photo", Method: http.MethodPost, Path: "/gpose-booths/{id}/signups/{signupId}/photo",
		Summary: "Upload the gpose for a completed sign-up",
		Description: "For the visitor to view and download on their festival dashboard and the booth's page. Only a " +
			"completed sign-up takes one (409), and an upload replaces the one before. It is kept a day from its " +
			"upload, then deleted.",
		Tags: gposeTags, Security: secured, Middlewares: page, MaxBodyBytes: gpose.MaxPhotoBytes + multipartOverhead,
		Errors: slices.Concat(notFound, []int{
			http.StatusConflict, http.StatusRequestEntityTooLarge, http.StatusUnprocessableEntity,
		}),
	}, h.uploadGposePhoto)
	huma.Register(api, huma.Operation{
		OperationID: "delete-gpose-photo", Method: http.MethodDelete, Path: "/gpose-booths/{id}/signups/{signupId}/photo",
		Summary: "Delete a sign-up's gpose", Description: "Before its day is over, for one uploaded in error.",
		Tags: gposeTags, DefaultStatus: http.StatusNoContent, Security: secured, Middlewares: page, Errors: notFound,
	}, h.deleteGposePhoto)
	huma.Register(api, huma.Operation{
		OperationID: "get-gpose-photo", Method: http.MethodGet, Path: "/gpose-photos/{token}",
		Summary: "A gpose",
		Description: "Its bytes, for the token in its sign-up's photoUrl, named for download after the character. " +
			"Only for a day from its upload: after that, like a token that isn't one, a 404. Kept private, and never " +
			"past that day. Ranges and conditional requests are answered.",
		Tags: gposeTags, Middlewares: huma.Middlewares{public}, Errors: []int{http.StatusNotFound},
		Responses: map[string]*huma.Response{
			"200": {Description: "The gpose.", Content: gposePhotoContent()},
			"206": {Description: "The part of the gpose a Range header asked for.", Content: gposePhotoContent()},
			"304": {Description: "The gpose is unchanged since the If-None-Match tag or If-Modified-Since date."},
		},
	}, h.getGposePhoto)
	huma.Register(api, huma.Operation{
		OperationID: "sign-up-gpose-booth", Method: http.MethodPost, Path: "/gpose-booths/{id}/sign-up",
		Summary: "Sign up at a gpose booth",
		Description: "Anyone may, at an open booth. A basic booth puts them in the next place, or on its waitlist once the " +
			"places are taken; a timeslot booth books the start time sent, or the waitlist. A booth that is full, a time " +
			"just taken, or a character already holding the most sign-ups the booth allows is a 409. Fifteen tries from " +
			"one address in ten minutes, made or not (429, with Retry-After).",
		Tags: gposeTags, DefaultStatus: http.StatusCreated,
		Errors: []int{http.StatusForbidden, http.StatusNotFound, http.StatusConflict, http.StatusUnprocessableEntity,
			http.StatusTooManyRequests},
	}, h.signUpGposeBooth)
	huma.Register(api, huma.Operation{
		OperationID: "look-up-gpose-signups", Method: http.MethodPost, Path: "/gpose-booths/{id}/lookup",
		Summary: "A visitor's own sign-ups at a booth",
		Description: "On the whole name and world, ignoring case. A POST so the name stays out of the access log. The " +
			"pages read it every half minute while a sign-up waits, so a staff call reaches them. 150 reads from one " +
			"address in ten minutes (429, with Retry-After). Only for an open booth.",
		Tags: gposeTags, Errors: []int{http.StatusNotFound, http.StatusUnprocessableEntity, http.StatusTooManyRequests},
	}, h.lookUpGposeSignups)
}

func noSuchGposeType() error { return huma.Error404NotFound("There is no such gpose type.") }

func noSuchGposeBooth() error { return huma.Error404NotFound("There is no such gpose booth.") }

func noSuchGposeSignup() error {
	return huma.Error404NotFound("There is no such sign-up at this booth.")
}

func boolOf(v int64) bool { return v != 0 }

// -- Types ---------------------------------------------------------------------------

func (h handlers) listGposeTypes(ctx context.Context, _ *struct{}) (*gposeTypeListOutput, error) {
	rows, err := h.read().ListGposeTypes(ctx)
	if err != nil {
		return nil, h.fail("list gpose types", err)
	}
	out := GposeTypeList{Types: make([]GposeType, 0, len(rows))}
	for _, r := range rows {
		out.Types = append(out.Types, GposeType{ID: r.ID, Name: r.Name, BoothCount: r.BoothCount, CreatedAt: r.CreatedAt})
	}
	return &gposeTypeListOutput{Body: out}, nil
}

// loadGposeType answers a type with how many booths offer it.
func (h handlers) loadGposeType(ctx context.Context, id int64) (GposeType, error) {
	q := h.read()
	t, err := q.GetGposeType(ctx, id)
	if errors.Is(err, sql.ErrNoRows) {
		return GposeType{}, noSuchGposeType()
	}
	if err != nil {
		return GposeType{}, h.fail("read a gpose type", err)
	}
	n, err := q.CountGposeTypeBooths(ctx, id)
	if err != nil {
		return GposeType{}, h.fail("count a gpose type's booths", err)
	}
	return GposeType{ID: t.ID, Name: t.Name, BoothCount: n, CreatedAt: t.CreatedAt}, nil
}

func takenTypeName() error {
	return huma.Error409Conflict("There is already a type by that name.")
}

func (h handlers) createGposeType(ctx context.Context, in *gposeTypeCreateInput) (*gposeTypeCreatedOutput, error) {
	ctx = context.WithoutCancel(ctx)
	id, err := h.write().CreateGposeType(ctx, strings.TrimSpace(in.Body.Name))
	if uniqueViolation(err) {
		return nil, takenTypeName()
	}
	if err != nil {
		return nil, h.fail("create a gpose type", err)
	}
	t, err := h.loadGposeType(ctx, id)
	if err != nil {
		return nil, err
	}
	return &gposeTypeCreatedOutput{Status: http.StatusCreated, Body: t}, nil
}

func (h handlers) updateGposeType(ctx context.Context, in *gposeTypeSaveInput) (*gposeTypeOutput, error) {
	ctx = context.WithoutCancel(ctx)
	n, err := h.write().UpdateGposeType(ctx, db.UpdateGposeTypeParams{Name: strings.TrimSpace(in.Body.Name), ID: in.ID})
	switch {
	case uniqueViolation(err):
		return nil, takenTypeName()
	case err != nil:
		return nil, h.fail("rename a gpose type", err)
	case n == 0:
		return nil, noSuchGposeType()
	}
	t, err := h.loadGposeType(ctx, in.ID)
	if err != nil {
		return nil, err
	}
	return &gposeTypeOutput{Body: t}, nil
}

func (h handlers) deleteGposeType(ctx context.Context, in *gposeTypeIDInput) (*struct{}, error) {
	t, err := h.loadGposeType(ctx, in.ID)
	if err != nil {
		return nil, err
	}
	if t.BoothCount > 0 {
		return nil, huma.Error409Conflict(fmt.Sprintf("%s is offered by %d %s. Take it off them first.", t.Name,
			t.BoothCount, plural(t.BoothCount, "booth", "booths")))
	}
	n, err := h.write().DeleteGposeType(context.WithoutCancel(ctx), in.ID)
	// A booth that took the type up since the count is refused by its foreign key.
	if err != nil && strings.Contains(err.Error(), "FOREIGN KEY") {
		return nil, huma.Error409Conflict(t.Name + " is offered by a booth. Take it off it first.")
	}
	if err != nil {
		return nil, h.fail("delete a gpose type", err)
	}
	if n == 0 {
		return nil, noSuchGposeType()
	}
	return nil, nil
}

func plural(n int64, one, many string) string {
	if n == 1 {
		return one
	}
	return many
}

// -- Booths --------------------------------------------------------------------------

func toGposeBooth(r *db.GetGposeBoothRow) GposeBooth {
	return GposeBooth{
		ID: r.ID, Title: r.Title, Description: r.Description, Mode: r.Mode, MaxSignups: r.MaxSignups,
		MaxWaitlist: r.MaxWaitlist, PerPerson: r.PerPerson, SlotStep: r.SlotStep, Status: r.Status,
		FestivalMapID: intPtr(r.FestivalMapID), FestivalMapName: r.FestivalMapName, OccupantID: intPtr(r.OccupantID),
		StallName: r.StallName, CreatedAt: r.CreatedAt, Types: []GposeBoothType{}, Windows: []GposeWindow{},
		BookedCount: r.BookedCount, WaitlistCount: r.WaitlistCount, WaitingCount: r.WaitingCount,
	}
}

// gposeBooth is a booth with what a visitor's view of it needs beyond the staff's.
type gposeBooth struct {
	GposeBooth
	festivalSlug, festivalStatus string
}

// withDetail reads a booth's types and open times.
func (h handlers) withDetail(ctx context.Context, q *db.Queries, r *db.GetGposeBoothRow) (gposeBooth, error) {
	b := gposeBooth{GposeBooth: toGposeBooth(r), festivalSlug: r.FestivalMapSlug, festivalStatus: r.FestivalMapStatus}
	types, err := q.ListGposeBoothTypes(ctx, r.ID)
	if err != nil {
		return b, h.fail("read a gpose booth's types", err)
	}
	for _, t := range types {
		b.Types = append(b.Types, GposeBoothType{TypeID: t.TypeID, Name: t.Name, Price: t.Price, Minutes: t.Minutes})
	}
	windows, err := q.ListGposeBoothWindows(ctx, r.ID)
	if err != nil {
		return b, h.fail("read a gpose booth's open times", err)
	}
	for _, w := range windows {
		b.Windows = append(b.Windows, GposeWindow{StartAt: w.StartAt, EndAt: w.EndAt})
	}
	return b, nil
}

// loadGposeBooth reads a booth with its types and open times, or the 404.
func (h handlers) loadGposeBooth(ctx context.Context, q *db.Queries, id int64) (gposeBooth, error) {
	r, err := q.GetGposeBooth(ctx, id)
	if errors.Is(err, sql.ErrNoRows) {
		return gposeBooth{}, noSuchGposeBooth()
	}
	if err != nil {
		return gposeBooth{}, h.fail("read a gpose booth", err)
	}
	return h.withDetail(ctx, q, r)
}

func (h handlers) listGposeBooths(ctx context.Context, _ *struct{}) (*gposeBoothListOutput, error) {
	rows, err := h.read().ListGposeBooths(ctx)
	if err != nil {
		return nil, h.fail("list gpose booths", err)
	}
	out := GposeBoothList{Booths: make([]GposeBooth, 0, len(rows))}
	for _, r := range rows {
		out.Booths = append(out.Booths, toGposeBooth((*db.GetGposeBoothRow)(r)))
	}
	return &gposeBoothListOutput{Body: out}, nil
}

func (h handlers) getGposeBooth(ctx context.Context, in *gposeBoothIDInput) (*gposeBoothOutput, error) {
	b, err := h.loadGposeBooth(ctx, h.read(), in.ID)
	if err != nil {
		return nil, err
	}
	return &gposeBoothOutput{Body: b.GposeBooth}, nil
}

// gposeBoothSave is a booth checked and put as it is stored.
type gposeBoothSave struct {
	booth                 gpose.Booth
	title, description    string
	festivalMapID, occupy sql.NullInt64
}

// gposeBoothColumns checks a booth the schema can't: its rules, that the types it
// offers exist, and the festival and stall it is filed under.
func (h handlers) gposeBoothColumns(ctx context.Context, in GposeBoothInput) (gposeBoothSave, error) {
	s := gposeBoothSave{
		booth: gpose.Booth{
			Mode: in.Mode, MaxSignups: in.MaxSignups, MaxWaitlist: in.MaxWaitlist, PerPerson: in.PerPerson,
			SlotStep: in.SlotStep, Types: make([]gpose.BoothType, 0, len(in.Types)),
			Windows: make([]gpose.Window, 0, len(in.Windows)),
		},
		title: strings.TrimSpace(in.Title), description: in.Description,
		festivalMapID: nullInt(in.FestivalMapID), occupy: nullInt(in.OccupantID),
	}
	for _, t := range in.Types {
		s.booth.Types = append(s.booth.Types, gpose.BoothType{TypeID: t.TypeID, Price: t.Price, Minutes: t.Minutes})
	}
	for _, w := range in.Windows {
		s.booth.Windows = append(s.booth.Windows, gpose.Window{StartAt: w.StartAt, EndAt: w.EndAt})
	}
	if bad := gpose.CheckBooth(&s.booth); bad != nil {
		return s, fieldProblem("body."+bad.Field, nil, bad.Message)
	}
	q := h.read()
	for i, t := range s.booth.Types {
		if _, err := q.GetGposeType(ctx, t.TypeID); errors.Is(err, sql.ErrNoRows) {
			return s, unknown(fmt.Sprintf("body.types[%d].typeId", i), t.TypeID, "gpose type")
		} else if err != nil {
			return s, h.fail("read a gpose type", err)
		}
	}
	if err := h.settleFestivalStall(ctx, &s.festivalMapID, &s.occupy, "gpose booth"); err != nil {
		return s, err
	}
	return s, nil
}

// saveGposeBoothParts replaces a booth's types and open times.
func saveGposeBoothParts(ctx context.Context, q *db.Queries, id int64, b gpose.Booth) error {
	if err := q.DeleteGposeBoothTypes(ctx, id); err != nil {
		return err
	}
	for i, t := range b.Types {
		if err := q.InsertGposeBoothType(ctx, db.InsertGposeBoothTypeParams{
			BoothID: id, TypeID: t.TypeID, Price: t.Price, Minutes: t.Minutes, SortOrder: int64(i),
		}); err != nil {
			return err
		}
	}
	if err := q.DeleteGposeBoothWindows(ctx, id); err != nil {
		return err
	}
	for i, w := range b.Windows {
		if err := q.InsertGposeBoothWindow(ctx, db.InsertGposeBoothWindowParams{
			BoothID: id, StartAt: w.StartAt, EndAt: w.EndAt, SortOrder: int64(i),
		}); err != nil {
			return err
		}
	}
	return nil
}

func (h handlers) createGposeBooth(ctx context.Context, in *gposeBoothCreateInput) (*gposeBoothCreatedOutput, error) {
	s, err := h.gposeBoothColumns(ctx, in.Body)
	if err != nil {
		return nil, err
	}
	ctx = context.WithoutCancel(ctx)
	var id int64
	err = h.transact(ctx, func(q *db.Queries) error {
		var err error
		id, err = q.CreateGposeBooth(ctx, db.CreateGposeBoothParams{
			Title: s.title, Description: s.description, Mode: s.booth.Mode, MaxSignups: s.booth.MaxSignups,
			MaxWaitlist: s.booth.MaxWaitlist, PerPerson: s.booth.PerPerson, SlotStep: s.booth.SlotStep,
			FestivalMapID: s.festivalMapID, OccupantID: s.occupy,
		})
		if err != nil {
			return err
		}
		return saveGposeBoothParts(ctx, q, id, s.booth)
	})
	if err != nil {
		return nil, h.fail("create a gpose booth", err)
	}
	b, err := h.loadGposeBooth(ctx, h.read(), id)
	if err != nil {
		return nil, err
	}
	return &gposeBoothCreatedOutput{Status: http.StatusCreated, Body: b.GposeBooth}, nil
}

func (h handlers) updateGposeBooth(ctx context.Context, in *gposeBoothSaveInput) (*gposeBoothOutput, error) {
	s, err := h.gposeBoothColumns(ctx, in.Body)
	if err != nil {
		return nil, err
	}
	ctx = context.WithoutCancel(ctx)
	err = h.transact(ctx, func(q *db.Queries) error {
		before, err := q.GetGposeBooth(ctx, in.ID)
		if errors.Is(err, sql.ErrNoRows) {
			return noSuchGposeBooth()
		}
		if err != nil {
			return err
		}
		if before.Mode != s.booth.Mode {
			n, err := q.CountGposeSignups(ctx, in.ID)
			if err != nil {
				return err
			}
			if n > 0 {
				return huma.Error409Conflict("This booth has sign-ups, so it can't change between basic and timeslot sign-up.")
			}
		}
		if _, err := q.UpdateGposeBooth(ctx, db.UpdateGposeBoothParams{
			Title: s.title, Description: s.description, Mode: s.booth.Mode, MaxSignups: s.booth.MaxSignups,
			MaxWaitlist: s.booth.MaxWaitlist, PerPerson: s.booth.PerPerson, SlotStep: s.booth.SlotStep,
			FestivalMapID: s.festivalMapID, OccupantID: s.occupy, ID: in.ID,
		}); err != nil {
			return err
		}
		return saveGposeBoothParts(ctx, q, in.ID, s.booth)
	})
	if err != nil {
		return nil, h.problemOr(err, "update a gpose booth")
	}
	b, err := h.loadGposeBooth(ctx, h.read(), in.ID)
	if err != nil {
		return nil, err
	}
	return &gposeBoothOutput{Body: b.GposeBooth}, nil
}

func (h handlers) setGposeBoothStatus(ctx context.Context, id int64, status string) (*gposeBoothOutput, error) {
	ctx = context.WithoutCancel(ctx)
	n, err := h.write().SetGposeBoothStatus(ctx, db.SetGposeBoothStatusParams{Status: status, ID: id})
	if err != nil {
		return nil, h.fail("set a gpose booth's status", err)
	}
	if n == 0 {
		return nil, noSuchGposeBooth()
	}
	b, err := h.loadGposeBooth(ctx, h.read(), id)
	if err != nil {
		return nil, err
	}
	return &gposeBoothOutput{Body: b.GposeBooth}, nil
}

func (h handlers) closeGposeBooth(ctx context.Context, in *gposeBoothIDInput) (*gposeBoothOutput, error) {
	return h.setGposeBoothStatus(ctx, in.ID, gpose.BoothClosed)
}

func (h handlers) reopenGposeBooth(ctx context.Context, in *gposeBoothIDInput) (*gposeBoothOutput, error) {
	return h.setGposeBoothStatus(ctx, in.ID, gpose.BoothOpen)
}

func (h handlers) deleteGposeBooth(ctx context.Context, in *gposeBoothIDInput) (*struct{}, error) {
	ctx = context.WithoutCancel(ctx)
	var photos []string
	err := h.transact(ctx, func(q *db.Queries) error {
		var err error
		if photos, err = q.ListGposeBoothPhotoTokens(ctx, in.ID); err != nil {
			return err
		}
		n, err := q.DeleteGposeBooth(ctx, in.ID)
		if err == nil && n == 0 {
			return noSuchGposeBooth()
		}
		return err
	})
	if err != nil {
		return nil, h.problemOr(err, "delete a gpose booth")
	}
	// Its sign-ups went with it, so their gposes go too.
	for _, token := range photos {
		h.GposePhotos.Remove(token)
	}
	return nil, nil
}

// -- Slots ---------------------------------------------------------------------------

// windowSpans is a booth's open times as spans.
func (b GposeBooth) windowSpans() []gpose.Span {
	pairs := make([][2]string, len(b.Windows))
	for i, w := range b.Windows {
		pairs[i] = [2]string{w.StartAt, w.EndAt}
	}
	return gpose.Spans(pairs)
}

func (b GposeBooth) step() time.Duration { return time.Duration(b.SlotStep) * time.Minute }

// heldSpans is the slots a booth's bookings hold, leaving out the sign-up skip (0 for
// none), on the queries given (a transaction's, when the answer decides a write).
func heldSpans(ctx context.Context, q *db.Queries, boothID, skip int64) ([]gpose.Span, error) {
	rows, err := q.ListGposeHeldSlots(ctx, boothID)
	if err != nil {
		return nil, err
	}
	out := make([]gpose.Span, 0, len(rows))
	for _, r := range rows {
		if r.ID == skip {
			continue
		}
		start, err1 := time.Parse(time.RFC3339, r.SlotStart)
		end, err2 := time.Parse(time.RFC3339, r.SlotEnd)
		if err1 == nil && err2 == nil {
			out = append(out, gpose.Span{Start: start, End: end})
		}
	}
	return out, nil
}

func formatTimes(ts []time.Time) []string {
	out := make([]string, len(ts))
	for i, t := range ts {
		out[i] = t.UTC().Format(time.RFC3339)
	}
	return out
}

// -- Sign-ups, as staff see them -----------------------------------------------------

// gposePositions is where each waiting sign-up stands: its place in line at a basic
// booth, or on the waitlist. Sign-ups are in the order they were made.
func gposePositions(signups []*db.GposeSignup, mode string) map[int64]int64 {
	out := make(map[int64]int64, len(signups))
	var line, waitlist int64
	for _, s := range signups {
		if s.Status != gpose.StatusWaiting {
			continue
		}
		switch {
		case s.Waitlisted != 0:
			waitlist++
			out[s.ID] = waitlist
		case mode == gpose.ModeBasic:
			line++
			out[s.ID] = line
		}
	}
	return out
}

// gposePhotoView is a sign-up's gpose as the API shows it: its address, on the site the
// request was made to, and when it goes; nothing when it has none or its day is over
// at now.
func gposePhotoView(site string, s *db.GposeSignup, now time.Time) (string, *string) {
	if s.PhotoToken == "" || s.PhotoExpiresAt == nil {
		return "", nil
	}
	until, err := time.Parse(time.RFC3339, *s.PhotoExpiresAt)
	if err != nil || !now.Before(until) {
		return "", nil
	}
	return site + Prefix + "/gpose-photos/" + s.PhotoToken, s.PhotoExpiresAt
}

func toGposeSignup(s *db.GposeSignup, position int64, site string, now time.Time) GposeSignup {
	out := GposeSignup{
		ID: s.ID, BoothID: s.BoothID, TypeID: intPtr(s.TypeID), TypeName: s.TypeName, Price: s.Price, Minutes: s.Minutes,
		CharacterName: s.CharacterName, World: s.World, Waitlisted: boolOf(s.Waitlisted), Position: position,
		SlotStart: s.SlotStart, SlotEnd: s.SlotEnd, Status: s.Status, Paid: boolOf(s.Paid), CalledAt: s.CalledAt,
		CreatedAt: s.CreatedAt, AssignableStarts: []string{},
	}
	out.PhotoURL, out.PhotoExpiresAt = gposePhotoView(site, s, now)
	return out
}

func toPublicGposeSignup(s *db.GposeSignup, position int64, site string, now time.Time) PublicGposeSignup {
	out := PublicGposeSignup{
		ID: s.ID, BoothID: s.BoothID, TypeName: s.TypeName, Price: s.Price, Minutes: s.Minutes,
		Waitlisted: boolOf(s.Waitlisted), Position: position, SlotStart: s.SlotStart, SlotEnd: s.SlotEnd,
		Status: s.Status, Paid: boolOf(s.Paid), CalledAt: s.CalledAt, CreatedAt: s.CreatedAt,
	}
	out.PhotoURL, out.PhotoExpiresAt = gposePhotoView(site, s, now)
	return out
}

// staffSignups is a booth's sign-ups as staff see them, each waitlisted one at a
// timeslot booth with the times its shoot fits now.
func (h handlers) staffSignups(ctx context.Context, q *db.Queries, b GposeBooth) ([]GposeSignup, error) {
	rows, err := q.ListGposeSignups(ctx, b.ID)
	if err != nil {
		return nil, err
	}
	positions := gposePositions(rows, b.Mode)
	var windows, held []gpose.Span
	if b.Mode == gpose.ModeTimeslot {
		windows = b.windowSpans()
		if held, err = heldSpans(ctx, q, b.ID, 0); err != nil {
			return nil, err
		}
	}
	now := time.Now().UTC()
	site := requestOf(ctx).Site
	byLength := map[int64][]string{}
	out := make([]GposeSignup, 0, len(rows))
	for _, s := range rows {
		v := toGposeSignup(s, positions[s.ID], site, now)
		if b.Mode == gpose.ModeTimeslot && s.Waitlisted != 0 && s.Status == gpose.StatusWaiting && s.Minutes > 0 {
			starts, ok := byLength[s.Minutes]
			if !ok {
				d := time.Duration(s.Minutes) * time.Minute
				// From one shoot's length ago, so a slot under way (a no-show's, say) is offered.
				starts = formatTimes(gpose.Starts(windows, held, b.step(), d, now.Add(-d)))
				byLength[s.Minutes] = starts
			}
			v.AssignableStarts = starts
		}
		out = append(out, v)
	}
	return out, nil
}

func (h handlers) listGposeSignups(ctx context.Context, in *gposeBoothIDInput) (*gposeSignupListOutput, error) {
	q := h.read()
	b, err := h.loadGposeBooth(ctx, q, in.ID)
	if err != nil {
		return nil, err
	}
	list, err := h.staffSignups(ctx, q, b.GposeBooth)
	if err != nil {
		return nil, h.fail("list a gpose booth's sign-ups", err)
	}
	return &gposeSignupListOutput{Body: GposeSignupList{Signups: list}}, nil
}

// staffSignup answers one sign-up as staff see it, read afresh.
func (h handlers) staffSignup(ctx context.Context, boothID, signupID int64) (*gposeSignupOutput, error) {
	q := h.read()
	b, err := h.loadGposeBooth(ctx, q, boothID)
	if err != nil {
		return nil, err
	}
	list, err := h.staffSignups(ctx, q, b.GposeBooth)
	if err != nil {
		return nil, h.fail("read a gpose sign-up", err)
	}
	for _, s := range list {
		if s.ID == signupID {
			return &gposeSignupOutput{Body: s}, nil
		}
	}
	return nil, noSuchGposeSignup()
}

// loadGposeSignup reads a sign-up that belongs to the booth, or the 404.
func loadGposeSignup(ctx context.Context, q *db.Queries, boothID, signupID int64) (*db.GposeSignup, error) {
	s, err := q.GetGposeSignup(ctx, signupID)
	if errors.Is(err, sql.ErrNoRows) || (err == nil && s.BoothID != boothID) {
		return nil, noSuchGposeSignup()
	}
	return s, err
}

// promoteWaitlist moves the longest-waiting person on a basic booth's waitlist into a
// place, when a place is free.
func promoteWaitlist(ctx context.Context, q *db.Queries, b GposeBooth) error {
	if b.Mode != gpose.ModeBasic {
		return nil
	}
	booked, err := q.CountGposeBooked(ctx, b.ID)
	if err != nil || booked >= b.MaxSignups {
		return err
	}
	next, err := q.FirstGposeWaitlisted(ctx, b.ID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	_, err = q.SetGposeSignupWaitlisted(ctx, db.SetGposeSignupWaitlistedParams{Waitlisted: 0, ID: next})
	return err
}

func slotTaken() error {
	return huma.Error409Conflict("That time is taken by another booking - pick another.")
}

func (h handlers) changeGposeSignup(ctx context.Context, in *gposeSignupChangeInput) (*gposeSignupOutput, error) {
	body := in.Body
	var slotStart time.Time
	if body.SlotStart != nil {
		t, err := time.Parse(time.RFC3339, *body.SlotStart)
		if err != nil {
			return nil, fieldProblem("body.slotStart", *body.SlotStart, "Send the slot's start as an RFC 3339 time.")
		}
		slotStart = t.UTC().Truncate(time.Second)
	}
	ctx = context.WithoutCancel(ctx)
	err := h.transact(ctx, func(q *db.Queries) error {
		b, err := h.loadGposeBooth(ctx, q, in.ID)
		if err != nil {
			return err
		}
		s, err := loadGposeSignup(ctx, q, in.ID, in.SignupID)
		if err != nil {
			return err
		}
		if body.SlotStart != nil {
			if err := bookSlot(ctx, q, b.GposeBooth, s, slotStart); err != nil {
				return err
			}
			if s, err = q.GetGposeSignup(ctx, s.ID); err != nil {
				return err
			}
		}
		if body.Status != nil && *body.Status != s.Status {
			if err := changeStatus(ctx, q, b.GposeBooth, s, *body.Status); err != nil {
				return err
			}
		}
		if body.Paid != nil {
			if _, err := q.SetGposeSignupPaid(ctx, db.SetGposeSignupPaidParams{Paid: boolInt(*body.Paid), ID: s.ID}); err != nil {
				return err
			}
		}
		return nil
	})
	if uniqueViolation(err) {
		return nil, slotTaken()
	}
	if err != nil {
		return nil, h.problemOr(err, "change a gpose sign-up")
	}
	return h.staffSignup(ctx, in.ID, in.SignupID)
}

// bookSlot puts a waiting sign-up at a timeslot booth into the slot starting at start,
// off the waitlist. Staff may place it between the offered times, or in one under way.
func bookSlot(ctx context.Context, q *db.Queries, b GposeBooth, s *db.GposeSignup, start time.Time) error {
	switch {
	case b.Mode != gpose.ModeTimeslot:
		return fieldProblem("body.slotStart", nil, "Only a timeslot booth books times.")
	case s.Status != gpose.StatusWaiting:
		return huma.Error409Conflict("Only someone still waiting can be given a slot.")
	case s.Minutes <= 0:
		return huma.Error409Conflict("This sign-up has no length to book.")
	}
	d := time.Duration(s.Minutes) * time.Minute
	held, err := heldSpans(ctx, q, b.ID, s.ID)
	if err != nil {
		return err
	}
	if !gpose.Fits(b.windowSpans(), held, b.step(), d, start, false) {
		return huma.Error409Conflict("That shoot doesn't fit there: it must be inside an open time without overlapping another booking.")
	}
	from, to := start.Format(time.RFC3339), start.Add(d).Format(time.RFC3339)
	_, err = q.SetGposeSignupSlot(ctx, db.SetGposeSignupSlotParams{SlotStart: &from, SlotEnd: &to, ID: s.ID})
	return err
}

// changeStatus moves a sign-up to status. One taking a place or slot back (a mistake
// undone) needs its slot still free; one giving its place up at a basic booth hands it
// to the waitlist.
func changeStatus(ctx context.Context, q *db.Queries, b GposeBooth, s *db.GposeSignup, status string) error {
	if !gpose.ValidStatus(status) {
		return fieldProblem("body.status", status, "That isn't a sign-up status.")
	}
	wasHeld, nowHeld := gpose.Holds(s.Status), gpose.Holds(status)
	if !wasHeld && nowHeld && s.SlotStart != nil && s.Waitlisted == 0 {
		start, err1 := time.Parse(time.RFC3339, *s.SlotStart)
		end, err2 := time.Parse(time.RFC3339, deref(s.SlotEnd))
		if err1 == nil && err2 == nil {
			held, err := heldSpans(ctx, q, b.ID, s.ID)
			if err != nil {
				return err
			}
			mine := gpose.Span{Start: start, End: end}
			if slices.ContainsFunc(held, mine.Overlaps) {
				return huma.Error409Conflict("Their slot has been booked by someone else since. Give them another time instead.")
			}
		}
	}
	if _, err := q.SetGposeSignupStatus(ctx, db.SetGposeSignupStatusParams{Status: status, ID: s.ID}); err != nil {
		return err
	}
	if wasHeld && !nowHeld && s.Waitlisted == 0 {
		return promoteWaitlist(ctx, q, b)
	}
	return nil
}

func (h handlers) callGposeSignup(ctx context.Context, in *gposeSignupIDInput) (*gposeSignupOutput, error) {
	q := h.read()
	if _, err := h.loadGposeBooth(ctx, q, in.ID); err != nil {
		return nil, err
	}
	s, err := loadGposeSignup(ctx, q, in.ID, in.SignupID)
	if err != nil {
		return nil, h.problemOr(err, "read a gpose sign-up")
	}
	ctx = context.WithoutCancel(ctx)
	n, err := h.write().CallGposeSignup(ctx, s.ID)
	if err != nil {
		return nil, h.fail("call a gpose sign-up", err)
	}
	if n == 0 {
		return nil, huma.Error409Conflict("Only someone still waiting can be called.")
	}
	by := ""
	if u := Who(ctx); u != nil {
		by = u.Username
	}
	h.Logger.Info("gpose sign-up called", "boothId", in.ID, "signupId", s.ID, "by", by)
	return h.staffSignup(ctx, in.ID, in.SignupID)
}

func (h handlers) deleteGposeSignup(ctx context.Context, in *gposeSignupIDInput) (*struct{}, error) {
	ctx = context.WithoutCancel(ctx)
	var photo string
	err := h.transact(ctx, func(q *db.Queries) error {
		b, err := h.loadGposeBooth(ctx, q, in.ID)
		if err != nil {
			return err
		}
		s, err := loadGposeSignup(ctx, q, in.ID, in.SignupID)
		if err != nil {
			return err
		}
		if _, err := q.DeleteGposeSignup(ctx, s.ID); err != nil {
			return err
		}
		photo = s.PhotoToken
		if gpose.Holds(s.Status) && s.Waitlisted == 0 {
			return promoteWaitlist(ctx, q, b.GposeBooth)
		}
		return nil
	})
	if err != nil {
		return nil, h.problemOr(err, "delete a gpose sign-up")
	}
	if photo != "" {
		h.GposePhotos.Remove(photo)
	}
	return nil, nil
}

// -- Gposes --------------------------------------------------------------------------

// gposePhotoOutput is a gpose the handler writes itself, so it can set its own cache
// headers and answer ranges and conditional requests.
type gposePhotoOutput struct {
	Body func(ctx huma.Context)
}

// gposePhotoContent declares the content types a gpose is served as.
func gposePhotoContent() map[string]*huma.MediaType {
	content := map[string]*huma.MediaType{}
	for _, ct := range gpose.PhotoTypes {
		content[ct] = &huma.MediaType{Schema: &huma.Schema{Type: "string", Format: "binary"}}
	}
	return content
}

func notCompleted() error {
	return huma.Error409Conflict("Mark the sign-up completed before uploading its gpose.")
}

func (h handlers) uploadGposePhoto(ctx context.Context, in *gposePhotoUploadInput) (*gposeSignupOutput, error) {
	form := in.RawBody.Data()
	defer func() { _ = form.Image.Close() }() // read in full below; nothing to report on close

	q := h.read()
	if _, err := h.loadGposeBooth(ctx, q, in.ID); err != nil {
		return nil, err
	}
	s, err := loadGposeSignup(ctx, q, in.ID, in.SignupID)
	if err != nil {
		return nil, h.problemOr(err, "read a gpose sign-up")
	}
	if s.Status != gpose.StatusCompleted {
		return nil, notCompleted()
	}
	data, err := io.ReadAll(io.LimitReader(form.Image, gpose.MaxPhotoBytes+1))
	if err != nil {
		return nil, h.fail("read a gpose", err)
	}
	if len(data) > gpose.MaxPhotoBytes {
		return nil, huma.Error413RequestEntityTooLarge("That gpose is over 32 MB.")
	}
	// Judged from its bytes, never the type the browser sent, and saved again without
	// its metadata when it is a PNG or JPEG.
	checked, err := uploads.CheckImage(data, uploads.ImageRule{Types: gpose.PhotoTypes})
	switch {
	case errors.Is(err, uploads.ErrTooLarge):
		return nil, fieldProblem("body.image", nil, "That gpose is too large: over 40 megapixels or 16384 pixels on a side.")
	case errors.Is(err, uploads.ErrNotImage):
		return nil, fieldProblem("body.image", nil, "The gpose must be a PNG, JPEG or WebP image.")
	case err != nil:
		return nil, h.fail("check a gpose", err)
	}
	token, err := h.GposePhotos.Save(checked.Data)
	if err != nil {
		return nil, h.fail("save a gpose", err)
	}
	// Detached: the file is on disk now, and its sign-up must name it or the file go.
	ctx = context.WithoutCancel(ctx)
	until := time.Now().UTC().Add(gpose.PhotoLifetime).Format(time.RFC3339)
	var replaced string
	err = h.transact(ctx, func(q *db.Queries) error {
		cur, err := loadGposeSignup(ctx, q, in.ID, in.SignupID)
		if err != nil {
			return err
		}
		n, err := q.SetGposeSignupPhoto(ctx, db.SetGposeSignupPhotoParams{
			PhotoToken: token, PhotoType: checked.Type, PhotoExpiresAt: &until, ID: cur.ID,
		})
		if err != nil {
			return err
		}
		// Undone by other staff since it was read.
		if n == 0 {
			return notCompleted()
		}
		replaced = cur.PhotoToken
		return nil
	})
	if err != nil {
		h.GposePhotos.Remove(token)
		return nil, h.problemOr(err, "record a gpose")
	}
	if replaced != "" {
		h.GposePhotos.Remove(replaced)
	}
	by := ""
	if u := Who(ctx); u != nil {
		by = u.Username
	}
	h.Logger.Info("gpose photo uploaded", "boothId", in.ID, "signupId", in.SignupID, "replaced", replaced != "",
		"bytes", len(checked.Data), "by", by)
	return h.staffSignup(ctx, in.ID, in.SignupID)
}

func (h handlers) deleteGposePhoto(ctx context.Context, in *gposeSignupIDInput) (*struct{}, error) {
	q := h.read()
	if _, err := h.loadGposeBooth(ctx, q, in.ID); err != nil {
		return nil, err
	}
	s, err := loadGposeSignup(ctx, q, in.ID, in.SignupID)
	if err != nil {
		return nil, h.problemOr(err, "read a gpose sign-up")
	}
	if s.PhotoToken == "" {
		return nil, nil
	}
	// Only the one read: a replacement uploaded since stays.
	n, err := h.write().ClearGposeSignupPhoto(context.WithoutCancel(ctx), db.ClearGposeSignupPhotoParams{
		ID: s.ID, PhotoToken: s.PhotoToken,
	})
	if err != nil {
		return nil, h.fail("delete a gpose", err)
	}
	if n > 0 {
		h.GposePhotos.Remove(s.PhotoToken)
	}
	return nil, nil
}

func (h handlers) getGposePhoto(ctx context.Context, in *gposePhotoInput) (*gposePhotoOutput, error) {
	missing := huma.Error404NotFound("There is no such gpose, or its day is over.")
	if !media.ValidGposePhotoToken(in.Token) {
		return nil, missing
	}
	p, err := h.read().GetGposePhoto(ctx, in.Token)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, missing
	}
	if err != nil {
		return nil, h.fail("read a gpose", err)
	}
	// Judged to the moment, whether or not the sweep has deleted it yet.
	until, err := time.Parse(time.RFC3339, p.PhotoExpiresAt)
	left := time.Until(until)
	if err != nil || left <= 0 {
		return nil, missing
	}
	f, info, err := h.GposePhotos.Open(in.Token)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, missing
	}
	if err != nil {
		return nil, h.fail("open a gpose", err)
	}
	disposition := mime.FormatMediaType("inline", map[string]string{"filename": gpose.PhotoName(p.CharacterName, p.PhotoType)})
	return &gposePhotoOutput{Body: func(c huma.Context) {
		defer func() { _ = f.Close() }() // read only
		r, w := humago.Unwrap(c)
		hdr := w.Header()
		hdr.Set("Content-Type", p.PhotoType)
		hdr.Set("X-Content-Type-Options", "nosniff")
		hdr.Set("Content-Disposition", disposition)
		// Private, and never kept past the gpose's day. A token names one upload, so
		// its bytes don't change while it lasts.
		hdr.Set("Cache-Control", fmt.Sprintf("private, max-age=%d", int64(left/time.Second)))
		hdr.Set("ETag", fileTag(info))
		http.ServeContent(w, r, "", info.ModTime(), f)
	}}, nil
}

// -- The public side -----------------------------------------------------------------

// publicGposeBooth is an open booth as a visitor sees it, with the times each type can
// be booked when withStarts is set.
func (h handlers) publicGposeBooth(ctx context.Context, q *db.Queries, b gposeBooth, withStarts bool, now time.Time) (PublicGposeBooth, error) {
	out := PublicGposeBooth{
		ID: b.ID, Title: b.Title, Description: b.Description, Mode: b.Mode, PerPerson: b.PerPerson,
		MaxSignups: b.MaxSignups, MaxWaitlist: b.MaxWaitlist, WaitlistLeft: max(0, b.MaxWaitlist-b.WaitlistCount),
		Types: make([]PublicGposeType, 0, len(b.Types)), Windows: b.Windows, FestivalName: b.FestivalMapName,
		StallName: b.StallName,
	}
	if b.Mode == gpose.ModeBasic {
		out.PlacesLeft = max(0, b.MaxSignups-b.BookedCount)
	}
	if b.FestivalMapID != nil && festival.NormalizeMapStatus(b.festivalStatus) == festival.MapPublished {
		out.FestivalPath = mapPath(*b.FestivalMapID, b.festivalSlug)
	}
	var windows, held []gpose.Span
	if withStarts && b.Mode == gpose.ModeTimeslot {
		windows = b.windowSpans()
		var err error
		if held, err = heldSpans(ctx, q, b.ID, 0); err != nil {
			return out, h.fail("read a gpose booth's bookings", err)
		}
	}
	for _, t := range b.Types {
		pt := PublicGposeType{TypeID: t.TypeID, Name: t.Name, Price: t.Price, Minutes: t.Minutes, Starts: []string{}}
		if windows != nil && t.Minutes > 0 {
			pt.Starts = formatTimes(gpose.Starts(windows, held, b.step(), time.Duration(t.Minutes)*time.Minute, now))
		}
		out.Types = append(out.Types, pt)
	}
	return out, nil
}

// openGposeBooth reads an open booth, or the 404 a closed or missing one is to a visitor.
func (h handlers) openGposeBooth(ctx context.Context, q *db.Queries, id int64) (gposeBooth, error) {
	b, err := h.loadGposeBooth(ctx, q, id)
	if err != nil {
		return b, err
	}
	if b.Status != gpose.BoothOpen {
		return b, noSuchGposeBooth()
	}
	return b, nil
}

func (h handlers) listPublicGposeBooths(ctx context.Context, _ *struct{}) (*publicGposeBoothListOutput, error) {
	q := h.read()
	rows, err := q.ListGposeBooths(ctx)
	if err != nil {
		return nil, h.fail("list gpose booths", err)
	}
	now := time.Now().UTC()
	out := PublicGposeBoothList{Booths: []PublicGposeBooth{}}
	for _, r := range rows {
		if r.Status != gpose.BoothOpen {
			continue
		}
		b, err := h.withDetail(ctx, q, (*db.GetGposeBoothRow)(r))
		if err != nil {
			return nil, err
		}
		pb, err := h.publicGposeBooth(ctx, q, b, false, now)
		if err != nil {
			return nil, err
		}
		out.Booths = append(out.Booths, pb)
	}
	return &publicGposeBoothListOutput{Body: out}, nil
}

func (h handlers) getPublicGposeBooth(ctx context.Context, in *gposeBoothIDInput) (*publicGposeBoothOutput, error) {
	q := h.read()
	b, err := h.openGposeBooth(ctx, q, in.ID)
	if err != nil {
		return nil, err
	}
	pb, err := h.publicGposeBooth(ctx, q, b, true, time.Now().UTC())
	if err != nil {
		return nil, err
	}
	return &publicGposeBoothOutput{Body: pb}, nil
}

// visitorGposeSignups is a character's own sign-ups at a booth, each with where it stands
// and its gpose while that lasts at now.
func visitorGposeSignups(ctx context.Context, q *db.Queries, b GposeBooth, name, world string, now time.Time) ([]PublicGposeSignup, error) {
	mine, err := q.ListGposeSignupsFor(ctx, db.ListGposeSignupsForParams{BoothID: b.ID, Name: name, World: world})
	if err != nil || len(mine) == 0 {
		return []PublicGposeSignup{}, err
	}
	all, err := q.ListGposeSignups(ctx, b.ID)
	if err != nil {
		return nil, err
	}
	positions := gposePositions(all, b.Mode)
	site := requestOf(ctx).Site
	out := make([]PublicGposeSignup, 0, len(mine))
	for _, s := range mine {
		out = append(out, toPublicGposeSignup(s, positions[s.ID], site, now))
	}
	return out, nil
}

var errGposeSignupDone = errors.New("signed up")

func (h handlers) signUpGposeBooth(ctx context.Context, in *gposeSignUpRequest) (*gposeSignedUpOutput, error) {
	ip := requestOf(ctx).IP
	if wait := retryAfter(h.GposeSignupLimit, ip); wait > 0 {
		h.Logger.Warn("gpose sign-up held back", "ip", ip)
		return nil, tooMany(wait, "Too many sign-ups from here. Try again later.")
	}
	// Every try counts, made or not, so nobody can flood a booth.
	if h.GposeSignupLimit != nil {
		h.GposeSignupLimit.Fail(ip)
	}
	body := in.Body
	if err := h.botCheck(ctx, body.TurnstileToken, "gpose sign-up"); err != nil {
		return nil, err
	}
	name, world, err := emaVisitor(body.CharacterName, body.World)
	if err != nil {
		return nil, err
	}
	var slotStart time.Time
	if body.SlotStart != nil {
		t, err := time.Parse(time.RFC3339, *body.SlotStart)
		if err != nil {
			return nil, fieldProblem("body.slotStart", *body.SlotStart, "Send the start time as an RFC 3339 time.")
		}
		slotStart = t.UTC().Truncate(time.Second)
	}
	b, err := h.openGposeBooth(ctx, h.read(), in.ID)
	if err != nil {
		return nil, err
	}
	i := slices.IndexFunc(b.Types, func(t GposeBoothType) bool { return t.TypeID == body.TypeID })
	if i < 0 {
		return nil, unknown("body.typeId", body.TypeID, "type at this booth")
	}
	typ := b.Types[i]
	if b.Mode == gpose.ModeTimeslot && !body.Waitlist && body.SlotStart == nil {
		return nil, fieldProblem("body.slotStart", nil, "Pick a time, or join the waitlist.")
	}

	ctx = context.WithoutCancel(ctx)
	var id int64
	err = h.transact(ctx, func(q *db.Queries) error {
		// The counts are read inside the write lock, so two people taking the last place
		// or the same time can't both pass them.
		mine, err := q.CountGposePersonWaiting(ctx, db.CountGposePersonWaitingParams{BoothID: b.ID, Name: name, World: world})
		if err != nil {
			return err
		}
		if mine >= b.PerPerson {
			if b.PerPerson == 1 {
				return huma.Error409Conflict("You already have a sign-up waiting at this booth.")
			}
			return huma.Error409Conflict(fmt.Sprintf("You already have %d sign-ups waiting at this booth, the most it allows.", mine))
		}
		p := db.InsertGposeSignupParams{
			BoothID: b.ID, TypeID: sql.NullInt64{Int64: typ.TypeID, Valid: true}, TypeName: typ.Name, Price: typ.Price,
			Minutes: typ.Minutes, CharacterName: name, World: world,
		}
		if p.Waitlisted, err = h.gposePlace(ctx, q, b.GposeBooth, typ, body.Waitlist, slotStart, &p); err != nil {
			return err
		}
		id, err = q.InsertGposeSignup(ctx, p)
		return err
	})
	if uniqueViolation(err) {
		return nil, slotTaken()
	}
	if err != nil {
		return nil, h.problemOr(err, "record a gpose sign-up")
	}
	signups, err := visitorGposeSignups(ctx, h.read(), b.GposeBooth, name, world, time.Now().UTC())
	if err != nil {
		return nil, h.fail("reload a gpose sign-up", err)
	}
	i = slices.IndexFunc(signups, func(s PublicGposeSignup) bool { return s.ID == id })
	if i < 0 {
		return nil, h.fail("reload a gpose sign-up", errGposeSignupDone)
	}
	// The write middleware leaves the public sign-up out, so tell the page.
	ResourceChanged(h.Live, "gpose-booths")
	return &gposeSignedUpOutput{Status: http.StatusCreated, Body: signups[i]}, nil
}

// gposePlace decides where a new sign-up goes, inside the transaction q is bound to:
// whether it is waitlisted, and at a timeslot booth the slot it books, set on p.
func (h handlers) gposePlace(ctx context.Context, q *db.Queries, b GposeBooth, typ GposeBoothType, waitlist bool, start time.Time, p *db.InsertGposeSignupParams) (int64, error) {
	full := huma.Error409Conflict("This booth is full.")
	if b.Mode == gpose.ModeBasic {
		booked, err := q.CountGposeBooked(ctx, b.ID)
		if err != nil {
			return 0, err
		}
		waiting, err := q.CountGposeWaitlist(ctx, b.ID)
		if err != nil {
			return 0, err
		}
		onWaitlist, ok := gpose.Place(booked, waiting, b.MaxSignups, b.MaxWaitlist)
		if !ok {
			return 0, full
		}
		return boolInt(onWaitlist), nil
	}
	if waitlist {
		if b.MaxWaitlist == 0 {
			return 0, huma.Error409Conflict("This booth has no waitlist.")
		}
		waiting, err := q.CountGposeWaitlist(ctx, b.ID)
		if err != nil {
			return 0, err
		}
		if waiting >= b.MaxWaitlist {
			return 0, huma.Error409Conflict("The waitlist is full.")
		}
		return 1, nil
	}
	held, err := heldSpans(ctx, q, b.ID, 0)
	if err != nil {
		return 0, err
	}
	d := time.Duration(typ.Minutes) * time.Minute
	if start.Before(time.Now().UTC()) || !gpose.Fits(b.windowSpans(), held, b.step(), d, start, true) {
		return 0, huma.Error409Conflict("That time isn't free any more - pick another.")
	}
	from, to := start.Format(time.RFC3339), start.Add(d).Format(time.RFC3339)
	p.SlotStart, p.SlotEnd = &from, &to
	return 0, nil
}

func (h handlers) lookUpGposeSignups(ctx context.Context, in *gposeLookupRequest) (*gposeLookupOutput, error) {
	ip := requestOf(ctx).IP
	if wait := retryAfter(h.GposeLookupLimit, ip); wait > 0 {
		h.Logger.Warn("gpose lookup held back", "ip", ip)
		return nil, tooMany(wait, "Too many requests. Please try again in a few minutes.")
	}
	if h.GposeLookupLimit != nil {
		h.GposeLookupLimit.Fail(ip)
	}
	name, world, err := emaVisitor(in.Body.CharacterName, in.Body.World)
	if err != nil {
		return nil, err
	}
	q := h.read()
	b, err := h.openGposeBooth(ctx, q, in.ID)
	if err != nil {
		return nil, err
	}
	mine, err := visitorGposeSignups(ctx, q, b.GposeBooth, name, world, time.Now().UTC())
	if err != nil {
		return nil, h.fail("look up gpose sign-ups", err)
	}
	return &gposeLookupOutput{Body: GposeLookup{Signups: mine}}, nil
}

// -- The festival --------------------------------------------------------------------

// FestivalGposeBooth is a booth filed under the festival, with the visitor's own
// sign-ups at it.
type FestivalGposeBooth struct {
	Booth   PublicGposeBooth    `json:"booth" doc:"The booth."`
	Open    bool                `json:"open" doc:"Whether it takes sign-ups. A closed booth is listed only while the visitor has a gpose there to download, and without its start times."`
	Signups []PublicGposeSignup `json:"signups" nullable:"false" doc:"The visitor's own sign-ups, on their whole name and world; empty when they have none or sent no name."`
}

// dashboardGpose is the festival's open booths, and the closed ones where the visitor
// still has a gpose to download, with the visitor's sign-ups at each.
func (h handlers) dashboardGpose(ctx context.Context, q *db.Queries, mapID int64, name, world string, now time.Time) ([]FestivalGposeBooth, error) {
	rows, err := q.ListFestivalGposeBooths(ctx, sql.NullInt64{Int64: mapID, Valid: true})
	if err != nil {
		return nil, h.fail("list a festival's gpose booths", err)
	}
	known := name != "" && world != ""
	out := make([]FestivalGposeBooth, 0, len(rows))
	for _, r := range rows {
		open := r.Status == gpose.BoothOpen
		if !open && !known {
			continue
		}
		signups := []PublicGposeSignup{}
		if known {
			if signups, err = visitorGposeSignups(ctx, q, toGposeBooth((*db.GetGposeBoothRow)(r)), name, world, now); err != nil {
				return nil, h.fail("read a visitor's gpose sign-ups", err)
			}
		}
		if !open && !slices.ContainsFunc(signups, func(s PublicGposeSignup) bool { return s.PhotoURL != "" }) {
			continue
		}
		b, err := h.withDetail(ctx, q, (*db.GetGposeBoothRow)(r))
		if err != nil {
			return nil, err
		}
		pb, err := h.publicGposeBooth(ctx, q, b, open, now)
		if err != nil {
			return nil, err
		}
		out = append(out, FestivalGposeBooth{Booth: pb, Open: open, Signups: signups})
	}
	return out, nil
}
