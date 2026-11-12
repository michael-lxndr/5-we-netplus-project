package excel

import (
	"bytes"
	"math"
	"regexp"
	"strconv"
	"time"

	"github.com/xuri/excelize/v2"

	"netplus/backend/internal/domain"
)

const (
	maxRows = 100001
	maxCols = 150
)

// Formatos numéricos integrados que ExcelJS reconoce como fecha/hora.
var builtinFormats = map[int]string{
	14: "mm-dd-yy", 15: "d-mmm-yy", 16: "d-mmm", 17: "mmm-yy",
	18: "h:mm AM/PM", 19: "h:mm:ss AM/PM", 20: "h:mm", 21: "h:mm:ss",
	22: "m/d/yy h:mm", 45: "mm:ss", 46: "[h]:mm:ss", 47: "mmss.0",
}

var (
	reBrackets = regexp.MustCompile(`\[[^\]]*\]`)
	reQuoted   = regexp.MustCompile(`"[^"]*"`)
	reDateCode = regexp.MustCompile(`[ymdhMsb]+`)
	reTimeOnly = regexp.MustCompile(`(?i)^(?:\[h{1,2}\]|h{1,2}):mm`)
	reZeroPad  = regexp.MustCompile(`^0{2,}$`)
)

type numFmt struct {
	date     bool // el número es una fecha/hora
	timeOnly bool // hora o duración: se conserva como fracción de día
	pad      int  // relleno con ceros (formato 000), 0 si no aplica
}

func classifyFormat(code string) numFmt {
	if code == "" {
		return numFmt{}
	}
	stripped := reQuoted.ReplaceAllString(reBrackets.ReplaceAllString(code, ""), "")
	return numFmt{
		date:     reDateCode.MatchString(stripped),
		timeOnly: reTimeOnly.MatchString(code),
		pad: func() int {
			if reZeroPad.MatchString(code) {
				return len(code)
			}
			return 0
		}(),
	}
}

// ReadWorkbook lee todas las hojas de un .xlsx. No ejecuta fórmulas: sus celdas quedan vacías.
func ReadWorkbook(data []byte) ([]Sheet, error) {
	f, err := excelize.OpenReader(bytes.NewReader(data), excelize.Options{
		UnzipSizeLimit:    1 << 30,
		UnzipXMLSizeLimit: 512 << 20,
	})
	if err != nil {
		return nil, domain.InputError("El archivo no es un Excel válido o está dañado.")
	}
	defer f.Close()

	names := f.GetSheetList()
	if len(names) == 0 {
		return nil, domain.InputError("El archivo no contiene hojas de cálculo.")
	}
	date1904 := false
	if props, err := f.GetWorkbookProps(); err == nil && props.Date1904 != nil {
		date1904 = *props.Date1904
	}
	formats := map[int]numFmt{}
	formatOf := func(styleID int) numFmt {
		if nf, ok := formats[styleID]; ok {
			return nf
		}
		var nf numFmt
		if style, err := f.GetStyle(styleID); err == nil && style != nil {
			code := ""
			if style.CustomNumFmt != nil {
				code = *style.CustomNumFmt
			} else {
				code = builtinFormats[style.NumFmt]
			}
			nf = classifyFormat(code)
		}
		formats[styleID] = nf
		return nf
	}

	sheets := make([]Sheet, 0, len(names))
	for _, name := range names {
		raw, err := f.GetRows(name, excelize.Options{RawCellValue: true})
		if err != nil {
			return nil, domain.InputError("No se pudo leer la hoja «" + name + "» del archivo.")
		}
		width := 0
		for _, r := range raw {
			if len(r) > width {
				width = len(r)
			}
		}
		if len(raw) > maxRows || width > maxCols {
			return nil, domain.InputError("El archivo supera el límite de 100.000 filas o 150 columnas por hoja.")
		}
		rows := make([][]Cell, len(raw))
		for i, r := range raw {
			row := make([]Cell, width)
			for j, text := range r {
				if text == "" {
					continue
				}
				coord, _ := excelize.CoordinatesToCellName(j+1, i+1)
				row[j] = convertCell(f, name, coord, text, date1904, formatOf)
			}
			rows[i] = row
		}
		sheets = append(sheets, Sheet{Name: name, Rows: rows})
	}
	return sheets, nil
}

func convertCell(f *excelize.File, sheet, coord, text string, date1904 bool, formatOf func(int) numFmt) Cell {
	kind, _ := f.GetCellType(sheet, coord)
	switch kind {
	case excelize.CellTypeBool, excelize.CellTypeError, excelize.CellTypeFormula:
		return nil
	case excelize.CellTypeSharedString, excelize.CellTypeInlineString:
		return text
	case excelize.CellTypeDate:
		for _, layout := range []string{time.RFC3339, "2006-01-02T15:04:05", "2006-01-02"} {
			if t, err := time.Parse(layout, text); err == nil {
				return t.UTC()
			}
		}
		return text
	}
	// Celda numérica (el atributo t puede faltar). Las fórmulas se descartan.
	if formula, _ := f.GetCellFormula(sheet, coord); formula != "" {
		return nil
	}
	n, err := strconv.ParseFloat(text, 64)
	if err != nil {
		return text
	}
	styleID, _ := f.GetCellStyle(sheet, coord)
	nf := formatOf(styleID)
	if nf.date && !nf.timeOnly {
		offset := 0.0
		if date1904 {
			offset = 1462
		}
		return time.UnixMilli(int64(math.Round((n - 25569 + offset) * 86400000))).UTC()
	}
	if nf.pad > 0 {
		s := numberString(n)
		for len(s) < nf.pad {
			s = "0" + s
		}
		return s
	}
	return n
}
