package photos_server

import (
	"errors"
	"fmt"
	"log"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode"

	"golang.org/x/text/runes"
	"golang.org/x/text/transform"
	"golang.org/x/text/unicode/norm"
)

type people string
type keyword string
type year int
type nb int
type month int
type key string
type date string
type empty struct{}

var yearReg, _ = regexp.Compile("[0-9]{4}")
var nbReg, _ = regexp.Compile("[0-9]{1,2}")
var dateReg, _ = regexp.Compile("[0-9]{2}/[0-9]{2}/[0-9]{4}")
var months = map[string]int{
	"janvier":   1,
	"jan":       1,
	"fevrier":   2,
	"fev":       2,
	"mars":      3,
	"avril":     4,
	"avr":       4,
	"mai":       5,
	"juin":      6,
	"juillet":   7,
	"juil":      7,
	"aout":      8,
	"septembre": 9,
	"sept":      9,
	"octobre":   10,
	"oct":       10,
	"novembre":  11,
	"nov":       11,
	"decembre":  12,
	"dec":       12,
}

type tokens struct {
	list   []string
	offset int
}

func (t *tokens) hasNext() bool {
	return t.offset < len(t.list)
}

func (t *tokens) back() {
	if t.offset > 0 {
		t.offset--
	}
}

func (t *tokens) next() (string, error) {
	if !t.hasNext() {
		return "", errors.New("EOT")
	}
	token := t.list[t.offset]
	t.offset++
	return token, nil
}

type ParseRequest struct {
	Peoples   []string
	Keywords  []string
	StartDate time.Time
	EndDate   time.Time
}

func newRequest() ParseRequest {
	return ParseRequest{
		Peoples:   make([]string, 0),
		Keywords:  make([]string, 0),
		StartDate: time.Time{},
		EndDate:   time.Time{},
	}
}

func (pr *ParseRequest) addPeople(p people) {
	pr.Peoples = append(pr.Peoples, string(p))
}

func (pr *ParseRequest) addKeyword(k keyword) {
	pr.Keywords = append(pr.Keywords, string(k))
}

func (pr *ParseRequest) HasDates() bool {
	return pr.StartDate.Year() > 1000 && pr.EndDate.Year() > 1000
}

func (pr ParseRequest) String() string {
	return fmt.Sprintf("Peoples : %d, keywords : %d, start : %v, end : %v", len(pr.Peoples), len(pr.Keywords), pr.StartDate, pr.EndDate)
}

type ParserManager struct {
	Peoples map[string]int
}

func (p ParserManager) nextToken(t *tokens) interface{} {

	token, e := t.next()
	if e != nil {
		return empty{}
	}
	// search in people
	if _, exist := p.Peoples[token]; exist {
		return people(token)
	}
	switch {
	case token == "entre" || token == "le" || token == "a" || token == "et" || token == "en":
		return key(token)
	case dateReg.MatchString(token):
		return date(token)
	case yearReg.MatchString(token):
		d, _ := strconv.Atoi(token)
		return year(d)
	case nbReg.MatchString(token):
		d, _ := strconv.Atoi(token)
		return nb(d)
	case months[token] != 0:
		return month(months[token])
	default:
		return keyword(token)
	}
}

func (p ParserManager) getNextDate(tks *tokens) (time.Time, time.Time, error) {
	d, m, y := -1, -1, 0
	// If date is only a month or a year, return a start and end date with bounds : month 01-31
	// Format : 10/10/2025, 11 novembre 1996, novembre 1995, 1998. Stop when find year on
	noStop := true
	for noStop && tks.hasNext() {
		switch token := p.nextToken(tks); token.(type) {
		case year:
			y = int(token.(year))
			noStop = false
		case date:
			return getDate(token.(date)), time.Time{}, nil
		case month:
			m = int(token.(month))
		case nb:
			d = int(token.(nb))
		default:
			tks.back()
			noStop = false
		}
	}
	switch {
	case y == 0:
		return time.Time{}, time.Time{}, errors.New("impossible")
	case m == -1:
		return newDate(y, 1, 1), newDate(y, 12, 31), nil
	case d == -1:
		return newDate(y, m, 1), newDate(y, m+1, 0), nil
	default:
		return newDate(y, m, d), newDate(y, m, d), nil
	}
	// TODO : in no token anymore, rollback enough
}

func newDate(y, m, d int) time.Time {
	return time.Date(y, time.Month(m), d, 0, 0, 0, 0, time.Local)
}

func (p ParserManager) Parse(s string) ParseRequest {
	request := newRequest()
	rejects := make([]string, 0)
	tks := tokenize(s)
	for tks.hasNext() {
		switch token := p.nextToken(tks); token.(type) {
		case people:
			request.addPeople(token.(people))
		case keyword:
			request.addKeyword(token.(keyword))
		case month, year:
			tks.back()
			if dStart, dEnd, err := p.getNextDate(tks); err == nil {
				request.StartDate = dStart
				request.EndDate = dEnd
			}
		case key:
			switch string(token.(key)) {
			case "le", "en": // Wait a date

				if dStart, dEnd, err := p.getNextDate(tks); err == nil {
					request.StartDate = dStart
					request.EndDate = dEnd
				} else {
					rejects = append(rejects, fmt.Sprintf("%v", token))
				}
			case "entre": // Wait start date et end date
				if d, _, err := p.getNextDate(tks); err == nil {
					request.StartDate = d
				} else {
					continue
				}
				if !tks.hasNext() {
					continue
				}
				if token := p.nextToken(tks); fmt.Sprintf("%v", token) == "et" {
					if _, d, err := p.getNextDate(tks); err == nil {
						request.EndDate = d
					}
				}

			case "et": // Wait people
				if tks.hasNext() {
					nToken := p.nextToken(tks)
					if reflect.TypeOf(nToken).Name() == "people" {
						request.addPeople(nToken.(people))
					} else {
						log.Println("Error : impossible to take date", nToken)
						tks.back()
					}
				} else {
					rejects = append(rejects, fmt.Sprintf("%v", token))
				}
			}
		}
	}
	return request
}

func tokenize(s string) *tokens {
	values := strings.Split(s, " ")
	for i, value := range values {
		values[i] = normalize(value)
	}
	return &tokens{values, 0}
}

func normalize(s string) string {
	t := transform.Chain(norm.NFD, runes.Remove(runes.In(unicode.Mn)), norm.NFC)
	r, _, _ := transform.String(t, strings.ToLower(s))
	return r
}

func getDate(d date) time.Time {
	subs := strings.Split(string(d), "/")
	day, _ := strconv.Atoi(subs[0])
	month, _ := strconv.Atoi(subs[1])
	year, _ := strconv.Atoi(subs[2])
	return newDate(year, month, day)
}
