package domain

import (
	"fmt"
	"time"
)

// Seed agrupa los registros que se cargan al iniciar o al reiniciar la demostración.
// Imports está en el orden de presentación del frontend (más reciente primero).
type Seed struct {
	Dataset     string
	Employees   []Employee
	Punches     []Punch
	Imports     []ImportLog
	Permits     []Permit
	SourceHours []SourceHours
}

// EmptySeed es la base sin datos de asistencia.
func EmptySeed() Seed {
	return Seed{
		Dataset:     "empty",
		Employees:   []Employee{},
		Punches:     []Punch{},
		Imports:     []ImportLog{},
		Permits:     []Permit{},
		SourceHours: []SourceHours{},
	}
}

// DemoSeed genera personas y registros ficticios (réplica de src/domain/demo.ts).
func DemoSeed() Seed {
	s := EmptySeed()
	s.Dataset = "demo"
	names := []string{
		"Elena Solís", "Mateo Valverde", "Lucía Campos", "Nicolás Paredes", "Isabel Cedeño",
		"Samuel Vega", "Camila Rosales", "Daniel Lema", "Valentina Paz", "Gabriel Molina",
		"Emilia Bravo", "Julián Acosta", "Renata Arias", "Tomás Villalba", "Paula Salcedo",
		"Sebastián Luna", "Clara Benítez", "Adrián Cobo", "Sofía Naranjo", "Diego Cabrera",
		"Martina Lucero", "Felipe Dávila", "Alma Rivera", "Bruno Tapia",
	}
	departments := []string{"Operaciones", "Comercial", "Tecnología", "Finanzas", "Recursos Humanos"}

	for i, name := range names {
		e := Employee{
			ID:         fmt.Sprintf("NP-%04d", i+1),
			Name:       name,
			Department: departments[i%len(departments)],
			Status:     "Activo",
			ActiveFrom: "2026-09-01",
		}
		if i == 23 {
			e.Status = "Inactivo"
			e.ActiveTo = "2026-10-15"
		}
		s.Employees = append(s.Employees, e)
	}

	var imports []ImportLog // octubre, septiembre, permisos, horas
	for _, month := range []int{9, 10} {
		importID := fmt.Sprintf("demo-%d", month)
		count := 0
		for day := 1; day <= 30; day++ {
			date := fmt.Sprintf("2026-%02d-%02d", month, day)
			wd := time.Date(2026, time.Month(month), day, 12, 0, 0, 0, time.UTC).Weekday()
			if wd == time.Sunday || wd == time.Saturday {
				continue
			}
			for i, e := range s.Employees {
				if (e.ActiveTo != "" && date > e.ActiveTo) || (day+i*3+month)%31 == 0 {
					continue
				}
				var entry string
				if (day+i*7+month)%13 == 0 {
					entry = fmt.Sprintf("08:%d:00", 45+(i%12))
				} else {
					entry = fmt.Sprintf("08:%02d:00", 10+((i+day)%25))
				}
				s.Punches = append(s.Punches, Punch{
					ID: fmt.Sprintf("%s-%d-%d-e", importID, i, day), EmployeeID: e.ID,
					Date: date, Time: entry, Kind: KindEntry, ImportID: importID, SourceRow: day*24 + i,
				})
				count++
				if (day*3+i+month)%47 != 0 {
					s.Punches = append(s.Punches, Punch{
						ID: fmt.Sprintf("%s-%d-%d-s", importID, i, day), EmployeeID: e.ID,
						Date: date, Time: fmt.Sprintf("17:%02d:00", (i+day)%25), Kind: KindExit,
						ImportID: importID, SourceRow: day*24 + i,
					})
					count++
				}
			}
		}
		name := "Septiembre"
		if month == 10 {
			name = "Octubre"
		}
		log := ImportLog{
			ID: importID, File: fmt.Sprintf("Asistencia_DEMO_%s_2026.xlsx", name), Sheet: "Marcaciones",
			At:   time.Date(2026, time.Month(month), 30, 22, 30, 0, 0, time.UTC),
			User: "Administración demo", Rows: count, Valid: count, Issues: []ImportIssue{},
		}
		imports = append([]ImportLog{log}, imports...)
	}

	at := time.Date(2026, 10, 29, 20, 0, 0, 0, time.UTC)
	s.Permits = []Permit{
		{ID: "demo-permiso-1", EmployeeID: "NP-0001", Name: names[0], Department: departments[0],
			From: "2026-10-15T08:00:00", To: "2026-10-15T11:00:00", Category: "Permiso personal",
			Approval: "Aprobado", ImportID: "demo-permisos", SourceRow: 3},
		{ID: "demo-permiso-2", EmployeeID: "NP-0003", Name: names[2], Department: departments[2],
			From: "2026-10-19T08:00:00", To: "2026-10-19T18:00:00", Category: "Permiso registrado",
			Approval: "Aprobado", ImportID: "demo-permisos", SourceRow: 4},
		{ID: "demo-permiso-3", EmployeeID: "NP-0005", Name: names[4], Department: departments[4],
			From: "2026-10-22T12:00:00", To: "2026-10-22T15:00:00", Category: "Permiso personal",
			Approval: "Pendiente", ImportID: "demo-permisos", SourceRow: 5},
	}
	for i, e := range s.Employees[:8] {
		sup, extra, night := (i+1)*45, i*30, 0
		if i%2 == 1 {
			night = 90
		}
		s.SourceHours = append(s.SourceHours, SourceHours{
			ID: fmt.Sprintf("demo-horas-%d", i), EmployeeID: e.ID, Name: e.Name, Department: e.Department,
			Month: "2026-10", Supplemental: &sup, Extra: &extra, Night: &night,
			ImportID: "demo-horas", SourceRow: i + 3,
		})
	}
	imports = append(imports,
		ImportLog{Kind: "permits", ID: "demo-permisos", File: "Permisos_DEMO_Octubre.xlsx", Sheet: "Octubre 2026",
			At: at, User: "Administración demo", Rows: 3, Valid: 3, Issues: []ImportIssue{}},
		ImportLog{Kind: "hours", ID: "demo-horas", File: "Horas_DEMO_Octubre.xlsx", Sheet: "Octubre 2026",
			At: at, User: "Administración demo", Rows: 8, Valid: 8, Issues: []ImportIssue{}},
	)
	s.Imports = imports
	return s
}
