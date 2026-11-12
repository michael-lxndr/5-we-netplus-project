// Package excel lee libros .xlsx y valida su contenido. Es la réplica en Go de
// src/services/excel.ts y sourceDocuments.ts del frontend: ambos deben dar el
// mismo resultado para el mismo archivo.
package excel

import (
	"math"
	"regexp"
	"strconv"
	"strings"
	"time"

	"golang.org/x/text/unicode/norm"
)

// Cell es string, float64, time.Time o nil (celda vacía, fórmula o error).
type Cell = any

// Sheet es una hoja leída; las filas conservan su posición real (la fila 1 es Rows[0]).
type Sheet struct {
	Name string
	Rows [][]Cell
}

// Normalize quita tildes, pasa a minúsculas y deja solo a-z y 0-9.
func Normalize(s string) string {
	var b strings.Builder
	for _, r := range norm.NFD.String(strings.ToLower(s)) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// CellString equivale a String(value ?? "") de JavaScript.
func CellString(c Cell) string {
	switch v := c.(type) {
	case nil:
		return ""
	case string:
		return v
	case float64:
		return numberString(v)
	case time.Time:
		return v.UTC().Format("2006-01-02T15:04:05.000Z")
	}
	return ""
}

func numberString(f float64) string {
	if f == math.Trunc(f) && math.Abs(f) < 1e21 {
		return strconv.FormatFloat(f, 'f', 0, 64)
	}
	return strconv.FormatFloat(f, 'f', -1, 64)
}

// HasValue indica si la celda tiene contenido no vacío.
func HasValue(c Cell) bool {
	return strings.TrimSpace(CellString(c)) != ""
}

// Str devuelve el texto de la celda sin espacios en los extremos.
func Str(c Cell) string { return strings.TrimSpace(CellString(c)) }

var (
	reDateYMD = regexp.MustCompile(`^(\d{4})[-/](\d{1,2})[-/](\d{1,2})(?:[T\s]|$)`)
	reDateDMY = regexp.MustCompile(`^(\d{1,2})[-/](\d{1,2})[-/](\d{4})(?:[T\s]|$)`)
	reTime    = regexp.MustCompile(`(?i)(?:^|[T\s])(\d{1,2}):(\d{2})(?::(\d{2}))?\s*(AM|PM)?$`)
)

// ParseDate devuelve AAAA-MM-DD para fechas nativas de Excel, AAAA-MM-DD o DD/MM/AAAA.
func ParseDate(c Cell) (string, bool) {
	switch v := c.(type) {
	case nil:
		return "", false
	case time.Time:
		if v.IsZero() {
			return "", false
		}
		return v.UTC().Format("2006-01-02"), true
	case float64:
		if v >= 1 && v <= 2958465 {
			days := int64(math.Floor(v - 25569))
			return time.Unix(days*86400, 0).UTC().Format("2006-01-02"), true
		}
		return "", false
	}
	str := strings.TrimSpace(CellString(c))
	var year, month, day int
	if m := reDateYMD.FindStringSubmatch(str); m != nil {
		year, month, day = atoi(m[1]), atoi(m[2]), atoi(m[3])
	} else if m := reDateDMY.FindStringSubmatch(str); m != nil {
		day, month, year = atoi(m[1]), atoi(m[2]), atoi(m[3])
	} else {
		return "", false
	}
	if year < 1900 || year > 2100 {
		return "", false
	}
	d := time.Date(year, time.Month(month), day, 0, 0, 0, 0, time.UTC)
	if d.Year() != year || int(d.Month()) != month || d.Day() != day {
		return "", false
	}
	return d.Format("2006-01-02"), true
}

// ParseTime devuelve HH:MM:SS para horas nativas de Excel, HH:MM, HH:MM:SS o con AM/PM.
func ParseTime(c Cell) (string, bool) {
	switch v := c.(type) {
	case nil:
		return "", false
	case time.Time:
		if v.IsZero() {
			return "", false
		}
		return v.UTC().Format("15:04:05"), true
	case float64:
		if v >= 0 && v < 1 {
			seconds := int(math.Floor(math.Mod(v, 1)*86400+0.5)) % 86400
			return clock(seconds/3600, (seconds%3600)/60, seconds%60), true
		}
		return "", false
	}
	m := reTime.FindStringSubmatch(strings.TrimSpace(CellString(c)))
	if m == nil {
		return "", false
	}
	h, min, sec := atoi(m[1]), atoi(m[2]), 0
	if m[3] != "" {
		sec = atoi(m[3])
	}
	if m[4] != "" {
		if h < 1 || h > 12 {
			return "", false
		}
		h %= 12
		if strings.EqualFold(m[4], "PM") {
			h += 12
		}
	}
	if h > 23 || min > 59 || sec > 59 {
		return "", false
	}
	return clock(h, min, sec), true
}

func clock(h, m, s int) string {
	return twoDigits(h) + ":" + twoDigits(m) + ":" + twoDigits(s)
}

func twoDigits(n int) string {
	if n < 10 {
		return "0" + strconv.Itoa(n)
	}
	return strconv.Itoa(n)
}

func atoi(s string) int {
	n, _ := strconv.Atoi(s)
	return n
}
