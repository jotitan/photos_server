package photos_server

import (
	"testing"
	"time"
)

var parser = ParserManager{Peoples: map[string]int{"roger": 1, "bob": 2, "elsa": 3}}

func TestParse(t *testing.T) {
	r := parser.Parse("roger paris")
	if len(r.Peoples) != 1 || len(r.Keywords) != 1 {
		t.Error("Must return one people and one keyword", r)
	}

	r = parser.Parse("roger à paris")
	if len(r.Peoples) != 1 || len(r.Keywords) != 1 || r.Peoples[0] != "roger" {
		t.Error("Must return one people and one keyword", r)
	}

	r = parser.Parse("le 12/12/2025 roger à paris")
	if len(r.Peoples) != 1 || len(r.Keywords) != 1 || r.Peoples[0] != "roger" || r.StartDate.Year() != 2025 {
		t.Error("Must return one people and one keyword and start date with 12/12/2025", r)
	}

	r = parser.Parse("le 12 decembre 2025 roger à paris")
	if len(r.Peoples) != 1 || len(r.Keywords) != 1 || r.Peoples[0] != "roger" || r.StartDate.Year() != 2025 {
		t.Error("Must return one people and one keyword and start date with 12/12/2025", r)
	}

	r = parser.Parse("bob et roger")
	if len(r.Peoples) != 2 {
		t.Error("Must return two people", r)
	}

	r = parser.Parse("mars 2024")
	if r.StartDate.Year() != 2024 || r.StartDate.Day() != 1 || r.StartDate.Month() != time.March || r.EndDate.Month() != time.March || r.EndDate.Day() != 31 {
		t.Error("Must return year 2024 from 1 to 31 mars", r)
	}

	r = parser.Parse("en mars 2024")
	if r.StartDate.Year() != 2024 || r.StartDate.Day() != 1 || r.StartDate.Month() != time.March || r.EndDate.Month() != time.March || r.EndDate.Day() != 31 || len(r.Keywords) > 0 {
		t.Error("Must return year 2024 from 1 to 31 mars", r)
	}

	r = parser.Parse("entre novembre 1995 et juin 1997")
	if r.StartDate.Year() != 1995 || r.EndDate.Year() != 1997 || r.EndDate.Month() != time.June {
		t.Error("Must return start and end date", r)
	}

	r = parser.Parse("bob et elsa à Paris entre 10/12/1992 et juin 1997")
	if r.StartDate.Year() != 1992 || r.EndDate.Year() != 1997 || len(r.Peoples) != 2 || len(r.Keywords) != 1 {
		t.Error("Must return start and end date, 2 peoples and 1 keyword", r)
	}

	r = parser.Parse("entre 10/12/1992 et juin 1997")
	if r.StartDate.Year() != 1992 || r.EndDate.Year() != 1997 || r.EndDate.Month() != time.June || r.EndDate.Day() != 30 {
		t.Error("Must return start and end date", r)
	}

	r = parser.Parse("entre 10 dec 1992 et juil 1997")
	if r.StartDate.Year() != 1992 || r.EndDate.Year() != 1997 || r.StartDate.Month() != time.December || r.EndDate.Month() != time.July || r.EndDate.Day() != 31 {
		t.Error("Must return start and end date", r)
	}
}
