package schedule

import (
	"errors"
	"sort"
	"time"

	"github.com/NikishGum/JudeOSLite/backend/internal/training/groups"
)

var ErrInput = errors.New("invalid schedule input")
var ErrConflict = errors.New("schedule version conflict")
var ErrPreview = errors.New("schedule preview changed")
var ErrDuplicate = errors.New("duplicate group time")

type Input struct {
	GroupID      string   `json:"groupId"`
	TrainerID    string   `json:"trainerId"`
	Mode         string   `json:"mode"`
	Weekdays     []int    `json:"weekdays,omitempty"`
	Parity       string   `json:"dayParity,omitempty"`
	Dates        []string `json:"selectedDates,omitempty"`
	Time         string   `json:"startsTime"`
	Duration     int      `json:"durationMinutes"`
	From         string   `json:"from"`
	Until        *string  `json:"until"`
	Version      int      `json:"version"`
	PreviewToken string   `json:"previewToken,omitempty"`
}
type Rule struct {
	Input
	ID       string `json:"id"`
	ClubID   string `json:"clubId"`
	SeriesID string `json:"seriesId"`
	Timezone string `json:"timezone"`
}
type Session struct {
	ID        string    `json:"id"`
	ClubID    string    `json:"clubId"`
	GroupID   string    `json:"groupId"`
	RuleID    *string   `json:"ruleId"`
	TrainerID string    `json:"trainerId"`
	Date      string    `json:"date"`
	Timezone  string    `json:"timezone"`
	StartsAt  time.Time `json:"startsAt"`
	EndsAt    time.Time `json:"endsAt"`
	Status    string    `json:"status"`
	Manual    bool      `json:"manualOverride"`
	Frozen    bool      `json:"rosterFrozen"`
	Version   int       `json:"version"`
}
type Occurrence struct {
	Date     string    `json:"date"`
	StartsAt time.Time `json:"startsAt"`
	EndsAt   time.Time `json:"endsAt"`
}
type Preview struct {
	Token     string       `json:"token"`
	Timezone  string       `json:"timezone"`
	Dates     []Occurrence `json:"dates"`
	Affected  []Session    `json:"affected"`
	Preserved []Session    `json:"preserved"`
}

func validate(in *Input) error {
	if in.GroupID == "" || in.TrainerID == "" || !groups.ValidDate(in.From) || in.Duration < 1 || in.Duration > 1440 {
		return ErrInput
	}
	if in.Until != nil && (!groups.ValidDate(*in.Until) || *in.Until <= in.From) {
		return ErrInput
	}
	t, e := time.Parse("15:04", in.Time)
	if e != nil || t.Format("15:04") != in.Time {
		return ErrInput
	}
	switch in.Mode {
	case "weekly":
		if len(in.Weekdays) < 1 || len(in.Weekdays) > 7 || in.Parity != "" || len(in.Dates) > 0 {
			return ErrInput
		}
		sort.Ints(in.Weekdays)
		for i, d := range in.Weekdays {
			if d < 1 || d > 7 || (i > 0 && in.Weekdays[i-1] == d) {
				return ErrInput
			}
		}
	case "day_parity":
		if (in.Parity != "even" && in.Parity != "odd") || len(in.Weekdays) > 0 || len(in.Dates) > 0 {
			return ErrInput
		}
	case "date_list", "once":
		if len(in.Dates) < 1 || len(in.Dates) > 1000 || (in.Mode == "once" && len(in.Dates) != 1) || len(in.Weekdays) > 0 || in.Parity != "" || in.Until != nil {
			return ErrInput
		}
		sort.Strings(in.Dates)
		for i, d := range in.Dates {
			if !groups.ValidDate(d) || d < in.From || (i > 0 && in.Dates[i-1] == d) {
				return ErrInput
			}
		}
		if in.Mode == "once" && in.From != in.Dates[0] {
			return ErrInput
		}
	default:
		return ErrInput
	}
	return nil
}

// Dates are civil dates in the version's timezone. Duration is elapsed minutes.
// A nonexistent/ambiguous DST wall time is rejected instead of silently shifted.
func occurrence(date, clock, zone string, minutes int) (Occurrence, error) {
	loc, e := time.LoadLocation(zone)
	if e != nil {
		return Occurrence{}, ErrInput
	}
	wall := date + " " + clock
	start, e := time.ParseInLocation("2006-01-02 15:04", wall, loc)
	if e != nil || start.Format("2006-01-02 15:04") != wall {
		return Occurrence{}, ErrInput
	}
	for _, delta := range []time.Duration{-2 * time.Hour, -time.Hour, -30 * time.Minute, 30 * time.Minute, time.Hour, 2 * time.Hour} {
		if start.Add(delta).In(loc).Format("2006-01-02 15:04") == wall {
			return Occurrence{}, ErrInput
		}
	}
	return Occurrence{date, start, start.Add(time.Duration(minutes) * time.Minute)}, nil
}

// FutureOccurrence shares the strict local calendar/DST rules with manual
// session creation and moves. The source date of a rule occurrence stays intact.
func FutureOccurrence(date, clock, zone string, minutes int, now time.Time) (Occurrence, error) {
	t, err := time.Parse("15:04", clock)
	if !groups.ValidDate(date) || err != nil || t.Format("15:04") != clock || minutes < 1 || minutes > 1440 {
		return Occurrence{}, ErrInput
	}
	o, err := occurrence(date, clock, zone, minutes)
	if err != nil || !o.StartsAt.After(now) {
		return Occurrence{}, ErrInput
	}
	return o, nil
}

func expand(in Input, zone string, now time.Time) ([]Occurrence, error) {
	loc, e := time.LoadLocation(zone)
	if e != nil {
		return nil, ErrInput
	}
	local := now.In(loc)
	today := time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, time.UTC)
	end := time.Date(local.Year(), local.Month()+2, 1, 0, 0, 0, 0, time.UTC)
	dates := in.Dates
	if in.Mode == "weekly" || in.Mode == "day_parity" {
		dates = []string{}
		from, _ := time.Parse("2006-01-02", in.From)
		if from.Before(today) {
			from = today
		}
		for d := from; d.Before(end); d = d.AddDate(0, 0, 1) {
			date := d.Format("2006-01-02")
			if in.Until != nil && date >= *in.Until {
				break
			}
			matches := false
			if in.Mode == "day_parity" {
				matches = (d.Day()%2 == 0) == (in.Parity == "even")
			} else {
				iso := (int(d.Weekday())+6)%7 + 1
				for _, w := range in.Weekdays {
					if iso == w {
						matches = true
					}
				}
			}
			if matches {
				dates = append(dates, date)
			}
		}
	}
	out := []Occurrence{}
	for _, date := range dates {
		if date < in.From || (in.Until != nil && date >= *in.Until) {
			continue
		}
		o, e := occurrence(date, in.Time, zone, in.Duration)
		if e != nil {
			return nil, e
		}
		if !o.StartsAt.After(now) {
			continue
		}
		out = append(out, o)
	}
	return out, nil
}
