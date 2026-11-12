package api

import (
	"encoding/base64"
	"math"
	"net/http"
	"regexp"
	"unicode/utf8"

	"netplus/backend/internal/domain"
	"netplus/backend/internal/excel"
	"netplus/backend/internal/store"
)

var reXlsx = regexp.MustCompile(`(?i)\.xlsx$`)

type importRequest struct {
	Filename   string              `json:"filename"`
	Base64     *string             `json:"base64"`
	SheetIndex *float64            `json:"sheetIndex"`
	Header     *float64            `json:"header"`
	Map        map[string]*float64 `json:"map"`
	Kind       string              `json:"kind"`
	Month      string              `json:"month"`
	Revision   *int64              `json:"revision"`
}

func asInt(f *float64, min, max int) (int, bool) {
	if f == nil || math.IsNaN(*f) || *f != math.Trunc(*f) || *f < float64(min) || *f > float64(max) {
		return 0, false
	}
	return int(*f), true
}

// columnMap valida la correspondencia de columnas enviada por el cliente.
func columnMap(raw map[string]*float64) (excel.ColumnMap, error) {
	m := excel.ColumnMap{}
	for _, key := range excel.Fields {
		n, ok := asInt(raw[key], -1, 149)
		if !ok {
			return nil, fail(http.StatusBadRequest, "La correspondencia de columnas no es válida.")
		}
		m[key] = n
	}
	return m, nil
}

// handleImports vuelve a leer y validar el Excel en el servidor antes de guardarlo.
func (s *Server) handleImports(w http.ResponseWriter, r *http.Request, user *domain.User) {
	rules, err := s.st.Rules(r.Context())
	if err != nil {
		respondError(w, err)
		return
	}
	// El archivo viaja en base64 dentro del JSON (≈ 4/3 del tamaño).
	limit := int64(math.Ceil(float64(rules.MaxFileMB)*1024*1024*1.38)) + 4096
	var in importRequest
	if err := decodeBody(w, r, limit, &in); err != nil {
		respondError(w, err)
		return
	}
	if current, err := s.st.Revision(r.Context()); err != nil {
		respondError(w, err)
		return
	} else if in.Revision == nil || *in.Revision != current {
		respondError(w, store.ErrConflict)
		return
	}

	if !reXlsx.MatchString(in.Filename) || utf8.RuneCountInString(in.Filename) > 255 || in.Base64 == nil {
		respondError(w, fail(http.StatusBadRequest, "Selecciona un archivo Excel .xlsx válido."))
		return
	}
	bytes, err := base64.StdEncoding.DecodeString(*in.Base64)
	if err != nil || len(bytes) > rules.MaxFileMB*1024*1024 || len(bytes) < 2 || bytes[0] != 'P' || bytes[1] != 'K' {
		respondError(w, fail(http.StatusBadRequest, "El archivo no es un Excel válido o supera el tamaño permitido."))
		return
	}
	sheets, err := excel.ReadWorkbook(bytes)
	if err != nil {
		respondError(w, err)
		return
	}
	sheetIndex, okSheet := asInt(in.SheetIndex, 0, len(sheets)-1)
	header, okHeader := asInt(in.Header, 0, 99)
	if !okSheet || !okHeader || header >= len(sheets[sheetIndex].Rows) {
		respondError(w, fail(http.StatusBadRequest, "La hoja o la fila de encabezados no es válida."))
		return
	}
	sheet := sheets[sheetIndex]

	var cols excel.ColumnMap
	isDocument := in.Kind == excel.DocPermits || in.Kind == excel.DocHours
	if !isDocument {
		if cols, err = columnMap(in.Map); err != nil {
			respondError(w, err)
			return
		}
	}

	err = s.mutate(r, in.Revision, func(tx txn) error {
		ctx := r.Context()
		existing, err := store.LoadExisting(ctx, tx)
		if err != nil {
			return err
		}
		if isDocument {
			v, err := excel.ValidateDocument(sheet, header, in.Kind, existing, in.Month)
			if err != nil {
				return err
			}
			if v.Total == 0 {
				return fail(http.StatusBadRequest, "La hoja seleccionada no contiene datos.")
			}
			return store.CommitDocument(ctx, tx, user.Name, in.Filename, sheet.Name, v)
		}
		v, err := excel.ValidateSheet(sheet, header, cols, existing)
		if err != nil {
			return err
		}
		if v.Total == 0 {
			return fail(http.StatusBadRequest, "La hoja seleccionada no contiene datos.")
		}
		return store.CommitAttendance(ctx, tx, user.Name, in.Filename, sheet.Name, v)
	})
	if err != nil {
		respondError(w, err)
		return
	}
	s.sendData(w, r, user)
}
