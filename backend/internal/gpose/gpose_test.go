package gpose

import (
	"slices"
	"testing"
	"time"
)

func at(s string) time.Time {
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		panic(err)
	}
	return t
}

func span(from, to string) Span { return Span{at(from), at(to)} }

func clock(ts []time.Time) []string {
	out := make([]string, len(ts))
	for i, t := range ts {
		out[i] = t.Format("15:04")
	}
	return out
}

// A shoot is offered every step from each window's start, only where it ends inside the
// window, never before from, and never over a booking held.
func TestStarts(t *testing.T) {
	windows := []Span{
		span("2026-08-01T18:00:00Z", "2026-08-01T19:00:00Z"),
		span("2026-08-01T20:00:00Z", "2026-08-01T20:40:00Z"),
	}
	held := []Span{span("2026-08-01T18:15:00Z", "2026-08-01T18:45:00Z")}
	got := clock(Starts(windows, held, 15*time.Minute, 15*time.Minute, at("2026-08-01T00:00:00Z")))
	want := []string{"18:00", "18:45", "20:00", "20:15"}
	if !slices.Equal(got, want) {
		t.Errorf("15-minute shoots: %v, want %v", got, want)
	}

	// A 30-minute shoot fits nowhere around the booking in the first window, and once in
	// the second (20:30 would end past its close).
	got = clock(Starts(windows, held, 15*time.Minute, 30*time.Minute, at("2026-08-01T00:00:00Z")))
	if want := []string{"20:00"}; !slices.Equal(got, want) {
		t.Errorf("30-minute shoots: %v, want %v", got, want)
	}

	// Nothing before from.
	got = clock(Starts(windows, nil, 15*time.Minute, 15*time.Minute, at("2026-08-01T18:40:00Z")))
	if want := []string{"18:45", "20:00", "20:15"}; !slices.Equal(got, want) {
		t.Errorf("after 18:40: %v, want %v", got, want)
	}

	if Starts(windows, nil, 0, time.Minute, time.Time{}) != nil || Starts(windows, nil, time.Minute, 0, time.Time{}) != nil {
		t.Error("a zero step or length offers times")
	}
}

func TestFits(t *testing.T) {
	windows := []Span{span("2026-08-01T18:00:00Z", "2026-08-01T19:00:00Z")}
	held := []Span{span("2026-08-01T18:30:00Z", "2026-08-01T18:45:00Z")}
	step, d := 15*time.Minute, 15*time.Minute
	cases := []struct {
		start  string
		d      time.Duration
		onGrid bool
		want   bool
	}{
		{"2026-08-01T18:00:00Z", d, true, true},
		{"2026-08-01T18:15:00Z", d, true, true},
		{"2026-08-01T18:30:00Z", d, true, false},                // held
		{"2026-08-01T18:20:00Z", d, true, false},                // off the grid
		{"2026-08-01T18:05:00Z", 10 * time.Minute, false, true}, // staff, off the grid
		{"2026-08-01T18:20:00Z", d, false, false},               // runs into the booking
		{"2026-08-01T18:50:00Z", d, false, false},               // ends after the close
		{"2026-08-01T17:50:00Z", d, false, false},               // starts before the open
		{"2026-08-01T18:00:00Z", 0, false, false},
	}
	for _, c := range cases {
		if got := Fits(windows, held, step, c.d, at(c.start), c.onGrid); got != c.want {
			t.Errorf("Fits(%s, %v, grid %v) = %v, want %v", c.start, c.d, c.onGrid, got, c.want)
		}
	}
	if Fits(windows, nil, 0, d, at("2026-08-01T18:00:00Z"), true) {
		t.Error("a zero step fits the grid")
	}
}

func TestPlace(t *testing.T) {
	cases := []struct {
		booked, waitlisted, maxSignups, maxWaitlist int64
		waitlist, ok                                bool
	}{
		{0, 0, 2, 0, false, true},
		{1, 0, 2, 0, false, true},
		{2, 0, 2, 1, true, true},
		{2, 1, 2, 1, false, false},
		{2, 0, 2, 0, false, false},
	}
	for _, c := range cases {
		waitlist, ok := Place(c.booked, c.waitlisted, c.maxSignups, c.maxWaitlist)
		if waitlist != c.waitlist || ok != c.ok {
			t.Errorf("Place(%d, %d, %d, %d) = %v, %v; want %v, %v", c.booked, c.waitlisted, c.maxSignups,
				c.maxWaitlist, waitlist, ok, c.waitlist, c.ok)
		}
	}
}

func TestHoldsAndValidStatus(t *testing.T) {
	for _, s := range []string{StatusWaiting, StatusCompleted} {
		if !Holds(s) || !ValidStatus(s) {
			t.Errorf("%s: holds %v, valid %v", s, Holds(s), ValidStatus(s))
		}
	}
	for _, s := range []string{StatusNoShow, StatusCanceled} {
		if Holds(s) || !ValidStatus(s) {
			t.Errorf("%s: holds %v, valid %v", s, Holds(s), ValidStatus(s))
		}
	}
	if ValidStatus("lost") {
		t.Error("an unknown status is valid")
	}
}

func basic() Booth {
	return Booth{Mode: ModeBasic, MaxSignups: 10, PerPerson: 1, SlotStep: 30, Types: []BoothType{{TypeID: 1, Price: 100, Minutes: 15}}}
}

func timeslot() Booth {
	return Booth{
		Mode: ModeTimeslot, PerPerson: 1, SlotStep: 15, MaxSignups: 9,
		Types: []BoothType{{TypeID: 1, Minutes: 15}, {TypeID: 2, Price: 5000, Minutes: 30}},
		Windows: []Window{
			{StartAt: "2026-08-02T18:00:00+02:00", EndAt: "2026-08-02T20:00:00+02:00"},
			{StartAt: "2026-08-01T18:00:00Z", EndAt: "2026-08-01T19:00:00Z"},
		},
	}
}

// A basic booth is stored without the fields only a timeslot booth uses, and a
// timeslot booth without its sign-up limit, its windows sorted and in UTC.
func TestCheckBoothPutsItAsStored(t *testing.T) {
	b := basic()
	b.Windows = []Window{{StartAt: "2026-08-01T18:00:00Z", EndAt: "2026-08-01T19:00:00Z"}}
	if err := CheckBooth(&b); err != nil {
		t.Fatal(err)
	}
	if b.Types[0].Minutes != 0 || b.Windows != nil || b.SlotStep != 15 {
		t.Errorf("basic booth kept %+v", b)
	}

	ts := timeslot()
	if err := CheckBooth(&ts); err != nil {
		t.Fatal(err)
	}
	if ts.MaxSignups != 0 {
		t.Errorf("timeslot booth kept a sign-up limit of %d", ts.MaxSignups)
	}
	want := []Window{
		{StartAt: "2026-08-01T18:00:00Z", EndAt: "2026-08-01T19:00:00Z"},
		{StartAt: "2026-08-02T16:00:00Z", EndAt: "2026-08-02T18:00:00Z"},
	}
	if !slices.Equal(ts.Windows, want) {
		t.Errorf("windows %v, want %v", ts.Windows, want)
	}
}

func TestCheckBoothRefuses(t *testing.T) {
	cases := []struct {
		field string
		edit  func(*Booth)
	}{
		{"mode", func(b *Booth) { b.Mode = "queue" }},
		{"types", func(b *Booth) { b.Types = nil }},
		{"types", func(b *Booth) { b.Types = make([]BoothType, MaxTypes+1) }},
		{"perPerson", func(b *Booth) { b.PerPerson = 0 }},
		{"perPerson", func(b *Booth) { b.PerPerson = MaxPerPerson + 1 }},
		{"maxWaitlist", func(b *Booth) { b.MaxWaitlist = -1 }},
		{"types[1].typeId", func(b *Booth) { b.Types = append(b.Types, BoothType{TypeID: 1}) }},
		{"types[0].price", func(b *Booth) { b.Types[0].Price = -1 }},
		{"maxSignups", func(b *Booth) { b.MaxSignups = 0 }},
		{"maxSignups", func(b *Booth) { b.MaxSignups = MaxSignups + 1 }},
	}
	for _, c := range cases {
		b := basic()
		c.edit(&b)
		if err := CheckBooth(&b); err == nil || err.Field != c.field {
			t.Errorf("basic, want %s refused: got %v", c.field, err)
		}
	}

	slots := []struct {
		field string
		edit  func(*Booth)
	}{
		{"types[0].minutes", func(b *Booth) { b.Types[0].Minutes = 0 }},
		{"types[1].minutes", func(b *Booth) { b.Types[1].Minutes = MaxMinutes + 1 }},
		{"slotStep", func(b *Booth) { b.SlotStep = MinStep - 1 }},
		{"slotStep", func(b *Booth) { b.SlotStep = MaxStep + 1 }},
		{"windows", func(b *Booth) { b.Windows = nil }},
		{"windows", func(b *Booth) { b.Windows = make([]Window, MaxWindows+1) }},
		{"windows[0].startAt", func(b *Booth) { b.Windows[0].StartAt = "tomorrow" }},
		{"windows[1].endAt", func(b *Booth) { b.Windows[1].EndAt = b.Windows[1].StartAt }},
		{"windows", func(b *Booth) {
			b.Windows = append(b.Windows, Window{StartAt: "2026-08-01T18:30:00Z", EndAt: "2026-08-01T21:00:00Z"})
		}},
	}
	for _, c := range slots {
		b := timeslot()
		c.edit(&b)
		if err := CheckBooth(&b); err == nil || err.Field != c.field {
			t.Errorf("timeslot, want %s refused: got %v", c.field, err)
		} else if err.Error() == "" {
			t.Errorf("%s: no message", c.field)
		}
	}
}

func TestSpans(t *testing.T) {
	got := Spans([][2]string{
		{"2026-08-02T18:00:00Z", "2026-08-02T19:00:00Z"},
		{"bad", "2026-08-02T19:00:00Z"},
		{"2026-08-01T18:00:00Z", "2026-08-01T18:00:00Z"},
		{"2026-08-01T18:00:00Z", "2026-08-01T19:00:00Z"},
	})
	want := []Span{span("2026-08-01T18:00:00Z", "2026-08-01T19:00:00Z"), span("2026-08-02T18:00:00Z", "2026-08-02T19:00:00Z")}
	if !slices.Equal(got, want) {
		t.Errorf("Spans = %v, want %v", got, want)
	}
}

// A gpose downloads as gpose- and the character's name, made safe on every OS, with the
// extension of its type.
func TestPhotoName(t *testing.T) {
	cases := []struct{ character, typ, want string }{
		{"Aya Lantern", "image/png", "gpose-Aya-Lantern.png"},
		{" Y'shtola  Rhul ", "image/jpeg", "gpose-Y-shtola-Rhul.jpg"},
		{"Ésoterique", "image/webp", "gpose-Ésoterique.webp"},
		{"../..", "image/png", "gpose.png"},
		{"", "", "gpose.png"},
	}
	for _, c := range cases {
		if got := PhotoName(c.character, c.typ); got != c.want {
			t.Errorf("PhotoName(%q, %q) = %q, want %q", c.character, c.typ, got, c.want)
		}
	}
}
