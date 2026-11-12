// Verifica que el entorno de desarrollo esté sano. Uso: mise run check
// Requiere que la API y el frontend estén corriendo (mise run dev).
const checks = [
  {
    name: "API responde (http://localhost:8080/api/health)",
    url: "http://localhost:8080/api/health",
    ok: (r, b) => r.ok && b.status === "ok",
  },
  {
    name: "Frontend responde (http://localhost:5173)",
    url: "http://localhost:5173/",
    text: true,
    ok: (r) => r.ok,
  },
  {
    name: "Proxy del frontend llega a la API (/api/health en :5173)",
    url: "http://localhost:5173/api/health",
    ok: (r, b) => r.ok && b.status === "ok",
  },
  {
    name: "API conecta con PostgreSQL (database en /api/health)",
    url: "http://localhost:8080/api/health",
    ok: (r, b) => r.ok && b.database === "ok",
  },
];

let failed = 0;
for (const c of checks) {
  try {
    const res = await fetch(c.url, { signal: AbortSignal.timeout(5000) });
    const body = c.text ? await res.text() : await res.json();
    const pass = c.ok(res, body);
    console.log(`${pass ? "OK   " : "FALLA"} ${c.name}`);
    if (!pass) failed++;
  } catch (e) {
    console.log(`FALLA ${c.name} -> ${e.cause?.code ?? e.message}`);
    failed++;
  }
}

if (failed) {
  console.log(`\n${failed} comprobación(es) fallaron. Mira la tabla "Si algo falla" del README.`);
  process.exit(1);
}
console.log("\nTodo en orden.");
