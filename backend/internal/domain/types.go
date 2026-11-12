// Package domain define el modelo de datos de Netplus. Los nombres JSON son el
// contrato con el frontend (src/domain/types.ts): no se deben cambiar sin
// cambiar también esa interfaz.
package domain

import "time"

const (
	RoleAdmin  = "admin"
	RoleViewer = "viewer"
)

const (
	KindEntry      = "entrada"
	KindExit       = "salida"
	KindUnassigned = "sin clasificar"
)

const (
	IssueError     = "error"
	IssueDuplicate = "duplicate"
	IssueWarning   = "warning"
)

// InputError es un error causado por datos del usuario; la API lo responde con 400.
type InputError string

func (e InputError) Error() string { return string(e) }

type Employee struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Department string `json:"department"`
	Status     string `json:"status"` // "Activo" | "Inactivo"
	ActiveFrom string `json:"activeFrom,omitempty"`
	ActiveTo   string `json:"activeTo,omitempty"`
}

type Punch struct {
	ID         string `json:"id"`
	EmployeeID string `json:"employeeId"`
	Date       string `json:"date"` // AAAA-MM-DD
	Time       string `json:"time"` // HH:MM:SS
	Kind       string `json:"kind"`
	ImportID   string `json:"importId"`
	SourceRow  int    `json:"sourceRow"`
}

// PunchKey identifica una marcación para detectar duplicados.
func PunchKey(employeeID, date, time, kind string) string {
	return employeeID + "|" + date + "|" + time + "|" + kind
}

type ImportIssue struct {
	Row     int    `json:"row"`
	Column  string `json:"column"`
	Message string `json:"message"`
	Type    string `json:"type"`
}

type ImportLog struct {
	Kind       string        `json:"kind,omitempty"` // "permits" | "hours"; vacío = asistencia
	ID         string        `json:"id"`
	File       string        `json:"file"`
	Sheet      string        `json:"sheet"`
	At         time.Time     `json:"at"`
	User       string        `json:"user"`
	Rows       int           `json:"rows"`
	Valid      int           `json:"valid"`
	Duplicates int           `json:"duplicates"`
	Invalid    int           `json:"invalid"`
	Warnings   int           `json:"warnings"`
	Issues     []ImportIssue `json:"issues"`
}

type User struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Email  string `json:"email"`
	Role   string `json:"role"`
	Active bool   `json:"active"`
}

type AuditLog struct {
	ID     string    `json:"id"`
	At     time.Time `json:"at"`
	User   string    `json:"user"`
	Action string    `json:"action"`
	Detail string    `json:"detail"`
}

type Rules struct {
	Enabled   bool   `json:"enabled"`
	Start     string `json:"start"` // HH:MM
	Tolerance int    `json:"tolerance"`
	Weekdays  []int  `json:"weekdays"`
	MaxFileMB int    `json:"maxFileMB"`
}

// DefaultRules son los parámetros iniciales (ficticios, pendientes de validar con Nettplus).
func DefaultRules() Rules {
	return Rules{Enabled: true, Start: "08:30", Tolerance: 10, Weekdays: []int{1, 2, 3, 4, 5}, MaxFileMB: 20}
}

type Permit struct {
	ID         string `json:"id"`
	EmployeeID string `json:"employeeId"`
	Name       string `json:"name"`
	Department string `json:"department"`
	From       string `json:"from"` // AAAA-MM-DDTHH:MM:SS
	To         string `json:"to"`
	Category   string `json:"category"`
	Approval   string `json:"approval"`
	ImportID   string `json:"importId"`
	SourceRow  int    `json:"sourceRow"`
}

// SourceHours guarda duraciones en minutos; nil significa "No informado".
type SourceHours struct {
	ID           string `json:"id"`
	EmployeeID   string `json:"employeeId"`
	Name         string `json:"name"`
	Department   string `json:"department"`
	Month        string `json:"month"` // AAAA-MM
	Supplemental *int   `json:"supplemental"`
	Extra        *int   `json:"extra"`
	Night        *int   `json:"night"`
	ImportID     string `json:"importId"`
	SourceRow    int    `json:"sourceRow"`
}

// AppData es el documento completo que consume el frontend.
type AppData struct {
	Version     int           `json:"version"`
	Revision    int64         `json:"revision"`
	Dataset     string        `json:"dataset"` // demo | imported | mixed | empty
	Employees   []Employee    `json:"employees"`
	Punches     []Punch       `json:"punches"`
	Imports     []ImportLog   `json:"imports"`
	Audits      []AuditLog    `json:"audits"`
	Users       []User        `json:"users"`
	Rules       Rules         `json:"rules"`
	Permits     []Permit      `json:"permits"`
	SourceHours []SourceHours `json:"sourceHours"`
}
