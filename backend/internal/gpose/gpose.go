// Package gpose holds the rules of a gpose booth, pure: no database, clock or network.
// A basic booth takes sign-ups in order up to its limit, then a waitlist; a timeslot
// booth books start times on a step inside its open windows, with a waitlist beside
// them for when someone doesn't show.
package gpose

import (
	"fmt"
	"slices"
	"sort"
	"strings"
	"time"
)

// A booth's modes.
const (
	ModeBasic    = "basic"
	ModeTimeslot = "timeslot"
)

// A booth's statuses.
const (
	BoothOpen   = "open"
	BoothClosed = "closed"
)

// Where a sign-up stands. Waiting and completed hold a place (a basic booth) or a slot
// (a timeslot booth); a no-show and a cancellation give it up.
const (
	StatusWaiting   = "waiting"
	StatusCompleted = "completed"
	StatusNoShow    = "no_show"
	StatusCanceled  = "canceled"
)

// The limits a booth is held to, well past any real booth, so a typo of an extra zero
// is caught rather than stored.
const (
	MaxSignups   = 1000
	MaxWaitlist  = 1000
	MaxPerPerson = 20
	MinStep      = 5
	MaxStep      = 240
	MaxMinutes   = 600
	MaxPrice     = 999_999_999
	MaxTypes     = 20
	MaxWindows   = 60
)

// Holds reports whether a sign-up in status keeps its place or slot.
func Holds(status string) bool { return status == StatusWaiting || status == StatusCompleted }

// ValidStatus reports whether status is one a sign-up can have.
func ValidStatus(status string) bool {
	switch status {
	case StatusWaiting, StatusCompleted, StatusNoShow, StatusCanceled:
		return true
	}
	return false
}

// Span is a stretch of time, its start inclusive and its end exclusive.
type Span struct {
	Start, End time.Time
}

// Overlaps reports whether two spans share any moment.
func (s Span) Overlaps(o Span) bool { return s.Start.Before(o.End) && o.Start.Before(s.End) }

// Starts lists the times a shoot of length d can start: every step from each window's
// start whose shoot ends inside the window, starting no earlier than from and
// overlapping nothing held. In order, windows being sorted by start.
func Starts(windows, held []Span, step, d time.Duration, from time.Time) []time.Time {
	if step <= 0 || d <= 0 {
		return nil
	}
	var out []time.Time
	for _, w := range windows {
		for t := w.Start; !t.Add(d).After(w.End); t = t.Add(step) {
			if t.Before(from) {
				continue
			}
			if free(held, Span{t, t.Add(d)}) {
				out = append(out, t)
			}
		}
	}
	return out
}

// Fits reports whether a shoot of length d starting at start lies inside one window
// and overlaps nothing held. onGrid also asks that it start a whole number of steps
// after its window's start, as every time a visitor is offered does; staff placing
// someone from the waitlist into a freed slot are not held to it.
func Fits(windows, held []Span, step, d time.Duration, start time.Time, onGrid bool) bool {
	if d <= 0 {
		return false
	}
	shoot := Span{start, start.Add(d)}
	for _, w := range windows {
		if start.Before(w.Start) || shoot.End.After(w.End) {
			continue
		}
		if onGrid && (step <= 0 || start.Sub(w.Start)%step != 0) {
			return false
		}
		return free(held, shoot)
	}
	return false
}

func free(held []Span, s Span) bool {
	return !slices.ContainsFunc(held, s.Overlaps)
}

// Place decides where a new sign-up at a basic booth lands: a place while fewer than
// maxSignups hold one, else the waitlist while it has room. ok is false when both are
// full.
func Place(booked, waitlisted, maxSignups, maxWaitlist int64) (waitlist, ok bool) {
	switch {
	case booked < maxSignups:
		return false, true
	case waitlisted < maxWaitlist:
		return true, true
	}
	return false, false
}

// BoothType is a type a booth offers, as a save sends it.
type BoothType struct {
	TypeID  int64
	Price   int64
	Minutes int64
}

// Window is a time a timeslot booth is open, as a save sends it.
type Window struct {
	StartAt string
	EndAt   string
}

// Booth is a booth as a save sends it.
type Booth struct {
	Mode        string
	MaxSignups  int64
	MaxWaitlist int64
	PerPerson   int64
	SlotStep    int64
	Types       []BoothType
	Windows     []Window
}

// FieldError is the reason a booth can't be saved and the field to point at, written
// as a path within the booth ("windows[1].endAt").
type FieldError struct {
	Field   string
	Message string
}

func (e *FieldError) Error() string { return e.Message }

// CheckBooth validates a booth and puts it as it is stored: the fields its mode doesn't
// use set to their defaults, the windows sorted and in UTC to the second. It returns the
// first thing wrong.
func CheckBooth(b *Booth) *FieldError {
	if b.Mode != ModeBasic && b.Mode != ModeTimeslot {
		return &FieldError{"mode", "Pick the kind of sign-up"}
	}
	if len(b.Types) == 0 {
		return &FieldError{"types", "Offer at least one type of gpose"}
	}
	if len(b.Types) > MaxTypes {
		return &FieldError{"types", fmt.Sprintf("Offer at most %d types", MaxTypes)}
	}
	if b.PerPerson < 1 || b.PerPerson > MaxPerPerson {
		return &FieldError{"perPerson", fmt.Sprintf("Sign-ups per person must be 1 to %d", MaxPerPerson)}
	}
	if b.MaxWaitlist < 0 || b.MaxWaitlist > MaxWaitlist {
		return &FieldError{"maxWaitlist", fmt.Sprintf("The waitlist must be 0 (none) to %d", MaxWaitlist)}
	}
	seen := make(map[int64]bool, len(b.Types))
	for i := range b.Types {
		t := &b.Types[i]
		at := fmt.Sprintf("types[%d]", i)
		if seen[t.TypeID] {
			return &FieldError{at + ".typeId", "Each type can be offered once"}
		}
		seen[t.TypeID] = true
		if t.Price < 0 || t.Price > MaxPrice {
			return &FieldError{at + ".price", "A price must be 0 (free) or more, in gil"}
		}
		if b.Mode == ModeTimeslot {
			if t.Minutes < 1 || t.Minutes > MaxMinutes {
				return &FieldError{at + ".minutes", fmt.Sprintf("Each type needs its length: 1 to %d minutes", MaxMinutes)}
			}
		} else {
			t.Minutes = 0
		}
	}
	if b.Mode == ModeBasic {
		if b.MaxSignups < 1 || b.MaxSignups > MaxSignups {
			return &FieldError{"maxSignups", fmt.Sprintf("The most sign-ups must be 1 to %d", MaxSignups)}
		}
		b.SlotStep, b.Windows = 15, nil
		return nil
	}
	b.MaxSignups = 0
	if b.SlotStep < MinStep || b.SlotStep > MaxStep {
		return &FieldError{"slotStep", fmt.Sprintf("Start times must be every %d to %d minutes", MinStep, MaxStep)}
	}
	return checkWindows(b)
}

func checkWindows(b *Booth) *FieldError {
	if len(b.Windows) == 0 {
		return &FieldError{"windows", "Add at least one time the booth is open"}
	}
	if len(b.Windows) > MaxWindows {
		return &FieldError{"windows", fmt.Sprintf("Add at most %d open times", MaxWindows)}
	}
	spans := make([]Span, len(b.Windows))
	for i := range b.Windows {
		w := &b.Windows[i]
		at := func(field string) string { return fmt.Sprintf("windows[%d].%s", i, field) }
		start, err1 := time.Parse(time.RFC3339, strings.TrimSpace(w.StartAt))
		end, err2 := time.Parse(time.RFC3339, strings.TrimSpace(w.EndAt))
		if err1 != nil || err2 != nil {
			return &FieldError{at("startAt"), fmt.Sprintf("Open time %d needs a start and an end", i+1)}
		}
		start, end = start.UTC().Truncate(time.Second), end.UTC().Truncate(time.Second)
		if !end.After(start) {
			return &FieldError{at("endAt"), fmt.Sprintf("Open time %d ends before it starts", i+1)}
		}
		spans[i] = Span{start, end}
	}
	sort.Slice(spans, func(i, j int) bool { return spans[i].Start.Before(spans[j].Start) })
	for i := 1; i < len(spans); i++ {
		if spans[i].Start.Before(spans[i-1].End) {
			return &FieldError{"windows", "Two of the open times overlap"}
		}
	}
	for i, s := range spans {
		b.Windows[i] = Window{StartAt: s.Start.Format(time.RFC3339), EndAt: s.End.Format(time.RFC3339)}
	}
	return nil
}

// Spans reads stored windows (start and end pairs, RFC 3339) as spans, sorted, leaving
// out any that don't read.
func Spans(pairs [][2]string) []Span {
	out := make([]Span, 0, len(pairs))
	for _, p := range pairs {
		start, err1 := time.Parse(time.RFC3339, p[0])
		end, err2 := time.Parse(time.RFC3339, p[1])
		if err1 == nil && err2 == nil && end.After(start) {
			out = append(out, Span{start, end})
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Start.Before(out[j].Start) })
	return out
}
