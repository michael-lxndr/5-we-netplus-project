package api

import (
	"net/http"
	"regexp"
	"strings"
	"unicode/utf8"

	"netplus/backend/internal/auth"
	"netplus/backend/internal/domain"
	"netplus/backend/internal/ids"
	"netplus/backend/internal/store"
)

const maxJSONBody = 1 << 20

var (
	reClock = regexp.MustCompile(`^([01]\d|2[0-3]):[0-5]\d$`)
	reEmail = regexp.MustCompile(`^[^\s@]+@[^\s@]+\.[^\s@]+$`)
)

type rulesRequest struct {
	Rules *struct {
		Enabled   *bool  `json:"enabled"`
		Start     string `json:"start"`
		Tolerance *int   `json:"tolerance"`
		Weekdays  []int  `json:"weekdays"`
		MaxFileMB *int   `json:"maxFileMB"`
	} `json:"rules"`
	Revision *int64 `json:"revision"`
}

func parseRules(in *rulesRequest) (domain.Rules, error) {
	bad := fail(http.StatusBadRequest, "Los parámetros de asistencia no son válidos.")
	r := in.Rules
	if r == nil || r.Enabled == nil || !reClock.MatchString(r.Start) || r.Tolerance == nil ||
		*r.Tolerance < 0 || *r.Tolerance > 120 || r.MaxFileMB == nil || *r.MaxFileMB < 1 || *r.MaxFileMB > 50 ||
		len(r.Weekdays) == 0 {
		return domain.Rules{}, bad
	}
	seen := map[int]bool{}
	for _, d := range r.Weekdays {
		if d < 0 || d > 6 || seen[d] {
			return domain.Rules{}, bad
		}
		seen[d] = true
	}
	return domain.Rules{Enabled: *r.Enabled, Start: r.Start, Tolerance: *r.Tolerance,
		Weekdays: r.Weekdays, MaxFileMB: *r.MaxFileMB}, nil
}

func (s *Server) handleRules(w http.ResponseWriter, r *http.Request, user *domain.User) {
	var in rulesRequest
	if err := decodeBody(w, r, maxJSONBody, &in); err != nil {
		respondError(w, err)
		return
	}
	rules, err := parseRules(&in)
	if err == nil {
		err = s.mutate(r, in.Revision, func(tx txn) error { return store.SetRules(r.Context(), tx, user.Name, rules) })
	}
	if err != nil {
		respondError(w, err)
		return
	}
	s.sendData(w, r, user)
}

func (s *Server) handleReset(w http.ResponseWriter, r *http.Request, user *domain.User) {
	var in struct {
		Mode     string `json:"mode"`
		Revision *int64 `json:"revision"`
	}
	if err := decodeBody(w, r, maxJSONBody, &in); err != nil {
		respondError(w, err)
		return
	}
	var err error
	if in.Mode != "seed" && in.Mode != "clear" {
		err = fail(http.StatusBadRequest, "Operación de datos no válida.")
	} else {
		err = s.mutate(r, in.Revision, func(tx txn) error { return store.ResetData(r.Context(), tx, user.Name, in.Mode) })
	}
	if err != nil {
		respondError(w, err)
		return
	}
	s.sendData(w, r, user)
}

type userRequest struct {
	User *struct {
		ID     string  `json:"id"`
		Name   *string `json:"name"`
		Email  *string `json:"email"`
		Role   *string `json:"role"`
		Active *bool   `json:"active"`
	} `json:"user"`
	Password string `json:"password"`
	Revision *int64 `json:"revision"`
}

func (s *Server) handleUsers(w http.ResponseWriter, r *http.Request, user *domain.User) {
	var in userRequest
	if err := decodeBody(w, r, maxJSONBody, &in); err != nil {
		respondError(w, err)
		return
	}
	if err := s.saveUser(r, in, user); err != nil {
		respondError(w, err)
		return
	}
	s.sendData(w, r, user)
}

func (s *Server) saveUser(r *http.Request, in userRequest, actor *domain.User) error {
	u := in.User
	if u == nil || u.Name == nil || strings.TrimSpace(*u.Name) == "" || utf8.RuneCountInString(*u.Name) > 80 ||
		u.Email == nil || !reEmail.MatchString(*u.Email) || len(*u.Email) > 120 ||
		u.Role == nil || (*u.Role != domain.RoleAdmin && *u.Role != domain.RoleViewer) || u.Active == nil {
		return fail(http.StatusBadRequest, "Revisa el nombre, correo y rol del usuario.")
	}
	passwordLen := utf8.RuneCountInString(in.Password)
	if in.Password != "" && (passwordLen < auth.MinPasswordLength || passwordLen > 128) {
		return fail(http.StatusBadRequest, "La contraseña debe tener entre 10 y 128 caracteres.")
	}
	var hash string
	if in.Password != "" {
		var err error
		if hash, err = auth.Hash(in.Password); err != nil {
			return err
		}
	}
	input := domain.User{
		ID: u.ID, Name: strings.TrimSpace(*u.Name), Email: strings.ToLower(strings.TrimSpace(*u.Email)),
		Role: *u.Role, Active: *u.Active,
	}

	return s.mutate(r, in.Revision, func(tx txn) error {
		ctx := r.Context()
		var existing *domain.User
		if input.ID != "" {
			var err error
			if existing, err = store.UserByID(ctx, tx, input.ID); err != nil {
				return err
			}
			if existing == nil {
				return fail(http.StatusNotFound, "El usuario ya no está disponible.")
			}
		}
		if existing == nil && in.Password == "" {
			return fail(http.StatusBadRequest, "La contraseña debe tener entre 10 y 128 caracteres.")
		}
		if existing != nil && existing.ID == actor.ID && (!input.Active || input.Role != domain.RoleAdmin) {
			return fail(http.StatusBadRequest, "No puedes desactivar ni quitar el rol de tu sesión actual.")
		}
		exceptID := ""
		if existing != nil {
			exceptID = existing.ID
		}
		if taken, err := store.EmailTaken(ctx, tx, input.Email, exceptID); err != nil {
			return err
		} else if taken {
			return store.ErrEmailTaken
		}
		if existing == nil {
			input.ID = ids.New()
		}
		if err := store.SaveUser(ctx, tx, input, hash, existing); err != nil {
			return err
		}
		action, state := "Alta de usuario", "activo"
		if existing != nil {
			action = "Edición de usuario"
		}
		if !input.Active {
			state = "inactivo"
		}
		return store.AddAudit(ctx, tx, actor.Name, action, input.Name+" · "+input.Role+" · "+state)
	})
}
