import { describe, expect, it, vi, beforeAll } from "vitest";
import { createElement } from "react";
import { renderToStaticMarkup } from "react-dom/server";
import Login from "../src/pages/Login";
import { api, serverAvailable } from "../src/services/api";
import {
  analyze,
  assertAdmin,
  defaultFilters,
  monthBounds,
} from "../src/domain/attendance";
import { createDemo, emptyData } from "../src/domain/demo";
import {
  detectColumns,
  mappingErrors,
  parseDate,
  parseTime,
  readExcel,
  validateSheet,
  type SheetData,
} from "../src/services/excel";
import { commitImport } from "../src/services/storage";
import {
  excelReport,
  exampleWorkbook,
  pdfReport,
  reportTable,
} from "../src/services/reports";

const sheet: SheetData = {
  name: "Asistencia",
  rows: [
    ["Código", "Nombre", "Departamento", "Fecha", "Entrada", "Salida"],
    ["0001", "Persona Ejemplo", "Pruebas", "2026-10-01", "08:20", "17:10"],
    ["0001", "Persona Ejemplo", "Pruebas", "2026-10-01", "08:20", "17:10"],
    ["0002", "Segunda Persona", "Pruebas", "2026-10-01", "08:50", null],
    ["0003", "Fecha Incorrecta", "Pruebas", "31/02/2026", "08:00", "17:00"],
    ["", "Sin Código", "Pruebas", "2026-10-01", "08:00", "17:00"],
  ],
};
const admin = emptyData().users[0];

describe("Fechas, horas y correspondencia", () => {
  it("valida fechas reales y formatos locales sin normalizar días imposibles", () => {
    expect(parseDate("31/02/2026")).toBeNull();
    expect(parseDate("29/02/2024")).toBe("2024-02-29");
    expect(parseDate("2026-10-01 08:30:00")).toBe("2026-10-01");
    expect(parseDate(46300)).toBe("2026-10-05");
    expect(parseTime("25:10")).toBeNull();
    expect(parseTime("08:61")).toBeNull();
    expect(parseTime(25)).toBeNull();
    expect(parseTime("5:15 PM")).toBe("17:15:00");
    expect(parseTime(0.5)).toBe("12:00:00");
    expect(parseTime("2026-10-01 08:30:00")).toBe("08:30:00");
  });
  it("reconoce encabezados y detecta columnas obligatorias", () => {
    const map = detectColumns(sheet.rows[0]);
    expect(map.code).toBe(0);
    expect(map.entry).toBe(4);
    expect(mappingErrors(map)).toEqual([]);
    expect(mappingErrors(detectColumns(["Otra columna"]))).toHaveLength(4);
  });
});

describe("Importación y permisos", () => {
  it("separa duplicados, errores e incompletos conservando la fila original", () => {
    const result = validateSheet(
      sheet,
      0,
      detectColumns(sheet.rows[0]),
      emptyData(),
    );
    expect(result).toMatchObject({
      total: 5,
      valid: 2,
      duplicates: 1,
      invalid: 2,
      incomplete: 1,
    });
    expect(result.punches).toHaveLength(3);
    expect(result.employees.map((e) => e.id)).toEqual(["0001", "0002"]);
    expect(result.issues.some((i) => i.row === 5 && i.column === "Fecha")).toBe(
      true,
    );
    expect(
      result.punches.some(
        (p) => p.employeeId === "0002" && p.kind === "salida",
      ),
    ).toBe(false);
  });
  it("evita duplicados al volver a cargar y conserva el historial", () => {
    const data = emptyData();
    const map = detectColumns(sheet.rows[0]);
    const next = commitImport(
      data,
      admin,
      "entrada.xlsx",
      sheet.name,
      validateSheet(sheet, 0, map, data),
    );
    expect(next.punches).toHaveLength(3);
    expect(next.imports[0].invalid).toBe(2);
    expect(next.audits[0].user).toBe(admin.name);
    const repeated = validateSheet(sheet, 0, map, next);
    expect(repeated.valid).toBe(0);
    expect(repeated.duplicates).toBe(3);
    expect(
      commitImport(next, admin, "entrada.xlsx", sheet.name, repeated).punches,
    ).toHaveLength(3);
  });
  it("bloquea la modificación por viewer en la capa de servicio", () => {
    expect(() => assertAdmin("viewer")).toThrow(/administrador/);
    const data = emptyData();
    const result = validateSheet(sheet, 0, detectColumns(sheet.rows[0]), data);
    expect(() =>
      commitImport(data, data.users[1], "archivo.xlsx", sheet.name, result),
    ).toThrow(/administrador/);
  });
  it("mantiene eventos individuales sin clasificar y rechaza colisiones de identidad", () => {
    const event: SheetData = {
      name: "Eventos",
      rows: [
        ["ID", "Nombre", "Fecha y hora", "Tipo"],
        ["1", "Prueba Uno", "2026-10-01 08:00", "Entrada"],
        ["1", "Prueba Uno", "2026-10-01 17:00", "Salida"],
        ["2", "Prueba Dos", "2026-10-01 09:00", ""],
        ["1", "Nombre Diferente", "2026-10-02 08:00", "Entrada"],
      ],
    };
    const result = validateSheet(
      event,
      0,
      detectColumns(event.rows[0]),
      emptyData(),
    );
    expect(result.valid).toBe(3);
    expect(result.invalid).toBe(1);
    expect(result.punches[2].kind).toBe("sin clasificar");
  });
  it("rechaza turnos con salida anterior y conserva códigos con ceros al leer Excel", async () => {
    const { default: ExcelJS } = await import("exceljs");
    const wb = new ExcelJS.Workbook();
    const s = wb.addWorksheet("Prueba");
    s.addRows([
      sheet.rows[0],
      [42, "Prueba Ceros", "Área", new Date("2026-10-01T00:00:00Z"), 0.35, 0.7],
    ]);
    s.getCell("A2").numFmt = "0000";
    s.getCell("D2").numFmt = "dd/mm/yyyy";
    const read = await readExcel(
      (await wb.xlsx.writeBuffer()) as unknown as ArrayBuffer,
    );
    expect(read[0].rows[1][0]).toBe("0042");
    const result = validateSheet(
      read[0],
      0,
      detectColumns(read[0].rows[0]),
      emptyData(),
    );
    expect(result.valid).toBe(1);
    const reversed = {
      ...sheet,
      rows: [
        sheet.rows[0],
        ["1", "Turno Noche", "Área", "2026-10-01", "22:00", "06:00"],
      ],
    };
    expect(
      validateSheet(reversed, 0, detectColumns(reversed.rows[0]), emptyData())
        .invalid,
    ).toBe(1);
  }, 30000);
});

describe("Indicadores y reportes", () => {
  it("admite eventos con fecha y hora en columnas separadas", () => {
    const events: SheetData = {
      name: "Eventos",
      rows: [
        ["Código", "Nombre", "Fecha", "Hora", "Tipo"],
        ["E-01", "Persona Evento", "01/10/2026", "08:15", "Entrada"],
        ["E-01", "Persona Evento", "01/10/2026", 0.75, "Salida"],
      ],
    };
    const result = validateSheet(
      events,
      0,
      detectColumns(events.rows[0]),
      emptyData(),
    );
    expect(result.valid).toBe(2);
    expect(result.punches.map((p) => p.kind)).toEqual(["entrada", "salida"]);
    expect(result.punches[1].time).toBe("18:00:00");
  });
  it("cuenta días únicos, calcula puntualidad y no inventa ausencias sin vigencia", () => {
    const data = commitImport(
      emptyData(),
      admin,
      "datos.xlsx",
      sheet.name,
      validateSheet(sheet, 0, detectColumns(sheet.rows[0]), emptyData()),
    );
    const a = analyze(data, defaultFilters(data));
    expect(a.present).toBe(2);
    expect(a.punches.length).toBe(3);
    expect(a.late).toBe(1);
    expect(a.punctuality).toBe(50);
    expect(a.incomplete).toBe(1);
    expect(a.absent).toBeNull();
    expect(
      analyze(
        { ...data, rules: { ...data.rules, enabled: false } },
        defaultFilters(data),
      ).punctuality,
    ).toBeNull();
  });
  it("tiene datos demo coherentes, filtros reactivos y vigencia limitada para ausencias", () => {
    const data = createDemo();
    const f = defaultFilters(data);
    const a = analyze(data, f);
    expect(f.month).toBe("2026-10");
    expect(data.employees).toHaveLength(24);
    expect(a.present).toBeGreaterThan(450);
    expect(a.punctuality).toBeGreaterThan(80);
    expect(a.absent).not.toBeNull();
    const filtered = analyze(data, { ...f, department: "Tecnología" });
    expect(filtered.present).toBeLessThan(a.present);
    expect(filtered.employees.every((e) => e.department === "Tecnología")).toBe(
      true,
    );
    const oneDay = analyze(data, {
      ...f,
      from: "2026-10-01",
      to: "2026-10-01",
    });
    expect(oneDay.present).toBeLessThan(25);
    const sep = analyze(data, { ...f, ...monthBounds("2026-09") });
    expect(sep.present).toBeGreaterThan(450);
  });
  it("genera un ejemplo real y lo valida con incidencias esperadas", async () => {
    const sheets = await readExcel(
      (await exampleWorkbook()).buffer as ArrayBuffer,
    );
    expect(sheets).toHaveLength(4);
    const result = validateSheet(
      sheets[0],
      0,
      detectColumns(sheets[0].rows[0]),
      emptyData(),
    );
    expect(result).toMatchObject({
      total: 6,
      valid: 3,
      duplicates: 1,
      invalid: 2,
      incomplete: 1,
    });
    expect(result.punches).toHaveLength(5);
  });
  it("exporta Excel con registros filtrados y metadatos, y PDF válido con contenido", async () => {
    const data = createDemo();
    const f = { ...defaultFilters(data), department: "Finanzas" };
    const a = analyze(data, f);
    const table = reportTable("monthly", a.summaries, a.days);
    expect(table.rows.every((r) => r[2] === "Finanzas")).toBe(true);
    const bytes = await excelReport(
      "monthly",
      a.summaries,
      a.days,
      f,
      data.rules,
    );
    const sheets = await readExcel(bytes.buffer as ArrayBuffer);
    expect(sheets[0].rows.length).toBe(a.summaries.length + 1);
    expect(sheets[1].rows.some((row) => row.includes("Finanzas"))).toBe(true);
    const pdf = await pdfReport("monthly", a.summaries, a.days, f, data.rules);
    const text = new TextDecoder().decode(pdf);
    expect(text.startsWith("%PDF-")).toBe(true);
    expect(text).toContain("NETTPLUS");
    expect(text).toContain("Finanzas");
    expect(pdf.length).toBeGreaterThan(3000);
  });
});

describe("Acceso autenticado", () => {
  const props = {
    available: true,
    onLogin: async () => {},
    onCheck: async () => {},
  };
  it("ofrece únicamente acceso con credenciales, sin accesos ni contraseñas de prueba", () => {
    const html = renderToStaticMarkup(createElement(Login, props));
    expect(html).toMatch(/autocomplete="username"/i);
    expect(html).toMatch(/autocomplete="current-password"/i);
    expect(html).toContain('type="email"');
    expect(html).toContain('type="password"');
    expect(html).not.toMatch(
      /Demo administrador|Demo consulta|Accesos iniciales de prueba|NettplusDemo2026|ConsultaDemo2026/,
    );
    expect(html).not.toMatch(/<button[^>]*class="login-submit"[^>]*disabled/);
  });
  it("bloquea el envío cuando no hay servidor y permite comprobar la conexión", () => {
    const html = renderToStaticMarkup(
      createElement(Login, { ...props, available: false }),
    );
    expect(html).toMatch(/<button[^>]*class="login-submit"[^>]*disabled/);
    expect(html).toContain("Servicio no disponible");
    expect(html).toContain("Comprobar conexión");
  });
  it("solo considera disponible una API conectada a PostgreSQL", async () => {
    const fetchMock = vi.fn();
    vi.stubGlobal("fetch", fetchMock);
    try {
      for (const [body, expected] of [
        [{ service: "nettplus-api", database: "ok" }, true],
        [{ service: "nettplus-api", database: "down" }, false],
        [{ service: "otro", database: "ok" }, false],
      ] as const) {
        fetchMock.mockResolvedValueOnce(
          new Response(JSON.stringify(body), {
            headers: { "Content-Type": "application/json" },
          }),
        );
        expect(await serverAvailable()).toBe(expected);
      }
      fetchMock.mockResolvedValueOnce(new Response("<html>Vercel</html>"));
      expect(await serverAvailable()).toBe(false);
      fetchMock.mockRejectedValueOnce(new TypeError("Failed to fetch"));
      expect(await serverAvailable()).toBe(false);
    } finally {
      vi.unstubAllGlobals();
    }
  });
  it("no convierte errores de red o credenciales rechazadas en una sesión", async () => {
    const fetchMock = vi.fn();
    vi.stubGlobal("fetch", fetchMock);
    try {
      fetchMock.mockRejectedValueOnce(new TypeError("Failed to fetch"));
      await expect(api("/auth/login", "POST", {})).rejects.toMatchObject({
        status: 503,
      });
      fetchMock.mockResolvedValueOnce(
        new Response(
          JSON.stringify({
            error: "El correo o la contraseña no son correctos.",
          }),
          { status: 401, headers: { "Content-Type": "application/json" } },
        ),
      );
      await expect(api("/auth/login", "POST", {})).rejects.toMatchObject({
        status: 401,
      });
      expect(fetchMock.mock.calls[1][1]).toMatchObject({
        credentials: "same-origin",
        method: "POST",
      });
    } finally {
      vi.unstubAllGlobals();
    }
  });
});

describe("Filtro de fechas de auditoría con zona horaria Ecuador", () => {
  let toEcuadorDate: (timestamp: string) => string;

  beforeAll(async () => {
    // Importar la función real de Admin.tsx para probarla directamente
    const adminModule = await import("../src/pages/Admin");
    toEcuadorDate = adminModule.toEcuadorDate;
  });

  it("convierte correctamente un timestamp cerca de medianoche UTC a fecha Ecuador", () => {
    // Timestamp: 2026-10-15 23:30:00 UTC
    // En Ecuador (UTC-5): 2026-10-15 18:30:00
    const timestamp = "2026-10-15T23:30:00Z";
    const ecuadorDate = toEcuadorDate(timestamp);
    expect(ecuadorDate).toBe("2026-10-15");
  });

  it("convierte correctamente un timestamp justo después de medianoche UTC a fecha Ecuador del día anterior", () => {
    // Timestamp: 2026-10-16 00:30:00 UTC
    // En Ecuador (UTC-5): 2026-10-15 19:30:00
    const timestamp = "2026-10-16T00:30:00Z";
    const ecuadorDate = toEcuadorDate(timestamp);
    expect(ecuadorDate).toBe("2026-10-15");
  });

  it("convierte correctamente un timestamp al final del día en Ecuador", () => {
    // Timestamp: 2026-10-15 04:59:59 UTC
    // En Ecuador (UTC-5): 2026-10-14 23:59:59
    const timestamp = "2026-10-15T04:59:59Z";
    const ecuadorDate = toEcuadorDate(timestamp);
    expect(ecuadorDate).toBe("2026-10-14");
  });

  it("filtra auditoría por rango de fechas usando conversión Ecuador", () => {
    const audits = [
      { id: "1", at: "2026-10-15T10:00:00Z", user: "admin", action: "Inicio de sesión", detail: "OK" },
      { id: "2", at: "2026-10-16T02:00:00Z", user: "admin", action: "Importación", detail: "OK" }, // 21:00 Ecuador el 15
      { id: "3", at: "2026-10-17T05:00:00Z", user: "admin", action: "Cierre", detail: "OK" }, // 00:00 Ecuador el 17
    ];

    const dateFrom = "2026-10-15";
    const dateTo = "2026-10-16";

    const filtered = audits.filter((a) => {
      const eventDate = toEcuadorDate(a.at);
      if (dateFrom && eventDate < dateFrom) return false;
      if (dateTo && eventDate > dateTo) return false;
      return true;
    });

    // Debe incluir los eventos del 15 y 16 en Ecuador
    expect(filtered.length).toBe(2);
    expect(filtered[0].id).toBe("1");
    expect(filtered[1].id).toBe("2");
  });
});

describe("Exportación de auditoría a Excel", () => {
  it("exporta todas las filas filtradas, no solo la página visible", async () => {
    const { tableExport } = await import("../src/services/reports");
    const ExcelJS = await import("exceljs");

    const headers = ["Fecha", "Usuario", "Operación", "Resultado"];
    const rows = [
      ["2026-10-15 10:00", "admin", "Inicio de sesión", "OK"],
      ["2026-10-15 11:00", "admin", "Importación", "OK"],
      ["2026-10-15 12:00", "admin", "Cierre", "OK"],
      ["2026-10-16 09:00", "viewer", "Consulta", "OK"],
      ["2026-10-16 10:00", "viewer", "Exportación", "OK"],
    ];

    const bytes = await tableExport(
      "xlsx",
      headers,
      rows,
      "Auditoría de prueba",
      "Filtro: todos",
    );

    // Verificar que se generó el archivo
    expect(bytes).toBeInstanceOf(Uint8Array);
    expect(bytes.length).toBeGreaterThan(0);

    // Leer el Excel para verificar el contenido
    const workbook = new ExcelJS.Workbook();
    await workbook.xlsx.load(bytes as any);

    const sheet = workbook.getWorksheet("Detalle");
    expect(sheet).toBeDefined();

    if (!sheet) throw new Error("Hoja no encontrada");

    // Verificar que tiene el header correcto
    const headerRow = sheet.getRow(1);
    expect(headerRow.getCell(1).value).toBe("Fecha");
    expect(headerRow.getCell(2).value).toBe("Usuario");
    expect(headerRow.getCell(3).value).toBe("Operación");
    expect(headerRow.getCell(4).value).toBe("Resultado");

    // Verificar que tiene todas las filas de datos (5 filas)
    expect(sheet.rowCount).toBe(6); // header + 5 filas de datos

    // Verificar el contenido de algunas filas
    expect(sheet.getRow(2).getCell(1).value).toBe("2026-10-15 10:00");
    expect(sheet.getRow(2).getCell(2).value).toBe("admin");
    expect(sheet.getRow(6).getCell(1).value).toBe("2026-10-16 10:00");
    expect(sheet.getRow(6).getCell(2).value).toBe("viewer");
  });

  it("exporta solo las filas filtradas cuando se aplican filtros", async () => {
    const { tableExport } = await import("../src/services/reports");
    const ExcelJS = await import("exceljs");

    const headers = ["Fecha", "Usuario", "Operación", "Resultado"];
    const allRows = [
      ["2026-10-15 10:00", "admin", "Inicio de sesión", "OK"],
      ["2026-10-15 11:00", "admin", "Importación", "OK"],
      ["2026-10-16 09:00", "viewer", "Consulta", "OK"],
    ];

    // Filtrar solo filas de admin
    const filteredRows = allRows.filter((row) => row[1] === "admin");

    const bytes = await tableExport(
      "xlsx",
      headers,
      filteredRows,
      "Auditoría filtrada",
      "Filtro: admin",
    );

    // Verificar que se generó el archivo
    expect(bytes).toBeInstanceOf(Uint8Array);
    expect(bytes.length).toBeGreaterThan(0);

    // Leer el Excel para verificar el contenido
    const workbook = new ExcelJS.Workbook();
    await workbook.xlsx.load(bytes as any);

    const sheet = workbook.getWorksheet("Detalle");
    expect(sheet).toBeDefined();

    if (!sheet) throw new Error("Hoja no encontrada");

    // Verificar que solo tiene las filas filtradas (2 filas de admin + header)
    expect(sheet.rowCount).toBe(3); // header + 2 filas de datos

    // Verificar que ambas filas son de admin
    expect(sheet.getRow(2).getCell(2).value).toBe("admin");
    expect(sheet.getRow(3).getCell(2).value).toBe("admin");
  });
});
