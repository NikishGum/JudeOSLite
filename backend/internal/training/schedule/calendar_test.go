package schedule

import (
	"fmt"
	"testing"
	"time"
)

func TestCivilCalendar(t *testing.T) {
	for _, tc := range []struct {
		year  int
		month time.Month
		days  int
	}{{2027, 2, 28}, {2028, 2, 29}, {2028, 4, 30}, {2028, 1, 31}, {2028, 12, 31}} {
		for _, parity := range []string{"even", "odd"} {
			t.Run(fmt.Sprintf("%d-%02d-%s", tc.year, tc.month, parity), func(t *testing.T) {
				now := time.Date(tc.year, tc.month, 1, 0, 0, 0, 0, time.UTC)
				until := now.AddDate(0, 1, 0).Format("2006-01-02")
				in := Input{Mode: "day_parity", Parity: parity, From: now.Format("2006-01-02"), Until: &until, Time: "18:30", Duration: 90}
				dates, e := expand(in, "Europe/Minsk", now)
				if e != nil {
					t.Fatal(e)
				}
				want := tc.days / 2
				if parity == "odd" {
					want = (tc.days + 1) / 2
				}
				if len(dates) != want {
					t.Fatalf("%d dates want %d", len(dates), want)
				}
				for _, d := range dates {
					date, _ := time.Parse("2006-01-02", d.Date)
					if (date.Day()%2 == 0) != (parity == "even") {
						t.Fatal("wrong parity")
					}
					if d.StartsAt.UTC().Hour() != 15 || d.EndsAt.Sub(d.StartsAt) != 90*time.Minute {
						t.Fatal("timezone/duration")
					}
				}
			})
		}
	}
	now := time.Date(2028, 12, 30, 22, 0, 0, 0, time.UTC) // Minsk already Dec 31.
	in := Input{Mode: "weekly", Weekdays: []int{1, 3, 5, 7}, From: "2028-01-01", Time: "18:00", Duration: 60}
	dates, e := expand(in, "Europe/Minsk", now)
	if e != nil {
		t.Fatal(e)
	}
	for _, d := range dates {
		if d.Date < "2028-12-31" || d.Date >= "2029-02-01" {
			t.Fatal("local horizon/year")
		}
		v, _ := time.Parse("2006-01-02", d.Date)
		iso := (int(v.Weekday())+6)%7 + 1
		if iso != 1 && iso != 3 && iso != 5 && iso != 7 {
			t.Fatal("weekday")
		}
	}
	in = Input{Mode: "date_list", Dates: []string{"2029-03-31", "2031-02-28"}, From: "2029-01-01", Time: "23:30", Duration: 90}
	dates, e = expand(in, "Europe/Minsk", now)
	if e != nil || len(dates) != 2 || dates[1].EndsAt.In(time.FixedZone("Minsk", 10800)).Day() != 1 {
		t.Fatalf("explicit far dates/cross-midnight: %v %v", dates, e)
	}
}
func TestInputAndDST(t *testing.T) {
	good := Input{GroupID: "g", TrainerID: "t", Mode: "weekly", Weekdays: []int{1, 7}, From: "2028-02-01", Time: "18:00", Duration: 60}
	if e := validate(&good); e != nil {
		t.Fatal(e)
	}
	for _, patch := range []func(*Input){func(i *Input) { i.Weekdays = []int{1, 1} }, func(i *Input) { i.Weekdays = []int{0} }, func(i *Input) { i.From = "2027-02-29" }, func(i *Input) { i.Time = "24:00" }, func(i *Input) { i.Duration = 0 }, func(i *Input) { i.Dates = []string{"2028-01-01"} }, func(i *Input) { i.Mode = "combined" }, func(i *Input) { i.Mode = "date_list"; i.Weekdays = nil; i.Dates = []string{"2028-02-02", "2028-02-02"} }} {
		bad := good
		patch(&bad)
		if validate(&bad) == nil {
			t.Fatalf("invalid input accepted: %+v", bad)
		}
	}
	for _, date := range []string{"2028-03-26", "2028-10-29"} {
		if _, e := occurrence(date, "02:30", "Europe/Berlin", 60); e == nil {
			t.Fatal("DST ambiguous/nonexistent accepted")
		}
	}
	if _, e := occurrence("2028-03-26", "18:00", "Europe/Berlin", 60); e != nil {
		t.Fatal(e)
	}
}
