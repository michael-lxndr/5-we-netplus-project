# Netplus

Control de asistencia: recibe reportes Excel (marcaciones, permisos y horas),
muestra dashboards e indicadores y genera reportes para preparar la nómina. API
en Go, frontend React + TypeScript (Vite), PostgreSQL y Redis. Cada carpeta tiene
su propio detalle: [frontend/README.md](frontend/README.md) (pantallas, formatos
de Excel, indicadores) y [docs/](docs/) (análisis y diseño).

## Cómo se desarrolla

- **Go y Node corren directamente en tu computadora**, con las versiones fijadas
  en [mise.toml](mise.toml) (Go 1.24, Node 22) para que todo el equipo use las
  mismas.
- **PostgreSQL y Redis** corren en Docker solo mientras trabajas (opción A), o
  se instala PostgreSQL de forma nativa si no puedes usar Docker (opción B).
  Nada queda corriendo ni ocupando puertos cuando terminas.

| Qué | Dónde | Puerto |
|---|---|---|
| Frontend (Vite) | host | 5173 |
| API (Go + Air) | host | 8080 (el frontend la proxifica en `/api`; lee y valida los Excel) |
| PostgreSQL | Docker (A) o nativo (B) | 5432 |
| Redis | Docker (A) | 6379 (el código aún no lo usa) |

## 1. Preparación (ambas opciones, una sola vez)

1. Instalar [Git](https://git-scm.com/) y [mise](https://mise.jdx.dev):

   ```powershell
   winget install Git.Git
   winget install jdx.mise
   ```

2. Activar mise en PowerShell, para que `go` y `node` queden en el PATH, y
   **reabrir la terminal**:

   ```powershell
   Add-Content $PROFILE 'mise activate pwsh | Out-String | Invoke-Expression'
   ```

3. Clonar e instalar las herramientas y dependencias:

   ```powershell
   git clone https://github.com/michael-lxndr/<repositorio>.git
   cd <repositorio>
   Copy-Item .env.example .env
   mise trust
   mise install      # Go 1.24 y Node 22
   mise run setup    # go mod download + npm install
   ```

Comprueba las versiones: `mise exec -- go version` debe decir `go1.24.x` y
`mise exec -- node --version`, `v22.x`.

## 2A. Levantar CON Docker

Requisito extra: Docker Desktop abierto y en verde.

```powershell
mise run dev
```

Esto ejecuta `docker compose up -d --wait postgres redis` (espera a que estén
sanos), arranca la API con recarga automática y el frontend. Abre
<http://localhost:5173>.

Al terminar, `Ctrl+C` y libera los puertos:

```powershell
mise run db:stop     # detiene los contenedores; los datos se conservan
```

## 2B. Levantar SIN Docker

1. Instalar PostgreSQL 17 nativo:

   ```powershell
   winget install PostgreSQL.PostgreSQL.17
   ```

   Anota la contraseña del usuario `postgres` que elijas en el instalador.

2. Crear el usuario y la base de datos que indica `.env` (te pedirá esa
   contraseña; `psql` está en `C:\Program Files\PostgreSQL\17\bin`):

   ```powershell
   & "C:\Program Files\PostgreSQL\17\bin\psql.exe" -U postgres -c "CREATE USER netplus WITH PASSWORD 'netplus_dev';"
   & "C:\Program Files\PostgreSQL\17\bin\psql.exe" -U postgres -c "CREATE DATABASE netplus OWNER netplus;"
   ```

3. Arrancar API y frontend (sin tocar Docker):

   ```powershell
   mise run dev:nodocker
   ```

Redis no hace falta todavía: el código no lo usa. Si ya tienes Postgres en el
puerto 5432 con otras credenciales, cambia `DATABASE_URL` en `.env`.

## 3. Verificar que todo funciona

Con `mise run dev` (o `dev:nodocker`) corriendo, en **otra terminal**:

```powershell
mise run check
```

Debe terminar en `Todo en orden.` con cuatro líneas `OK`:

```
OK    API responde (http://localhost:8080/api/health)
OK    Frontend responde (http://localhost:5173)
OK    Proxy del frontend llega a la API (/api/health en :5173)
OK    API conecta con PostgreSQL (database en /api/health)
```

Comprobación visual: abre <http://localhost:5173>; debe verse la pantalla de
inicio de sesión con "Servidor local disponible". En una base nueva la API crea
estas cuentas y datos de demostración (ver "Cuentas iniciales"):

| Rol | Correo | Contraseña |
|---|---|---|
| Administrador | `admin@nettplus.local` | `NettplusDemo2026!` |
| Consulta (viewer) | `viewer@nettplus.local` | `ConsultaDemo2026!` |

También puedes entrar a la **demo del navegador** (sin servidor, datos en
IndexedDB) desde la misma pantalla.

Comprobaciones adicionales opcionales:

```powershell
mise run test                                   # pruebas de Go y del frontend
cd backend;  mise exec -- go vet ./...          # análisis estático de Go
cd frontend; mise exec -- npx tsc --noEmit      # tipos de TypeScript
docker compose ps                               # (A) postgres y redis "healthy"
```

Para comprobar que la recarga automática funciona, guarda cualquier cambio en un
`.go` y mira en la terminal cómo Air vuelve a compilar y reinicia la API.

### Si algo falla

| Síntoma | Causa y solución |
|---|---|
| `mise: command not found` / `go` o `node` no se encuentran | No se activó mise. Repite el paso 1.2 y **reabre la terminal**. |
| `mise ERROR ... is not trusted` | Ejecuta `mise trust` en la carpeta del proyecto. |
| `check`: las 4 líneas dan `ECONNREFUSED` | `dev` no está corriendo. Lánzalo en otra terminal y espera a ver `Netplus API escuchando en :8080`. |
| `check`: solo falla "conecta con PostgreSQL" | La base no está accesible o `DATABASE_URL` en `.env` no coincide con tus credenciales. (A) `docker compose ps`; (B) comprueba que el servicio PostgreSQL de Windows esté iniciado. |
| `DATABASE_URL es obligatoria` | Falta `.env`: `Copy-Item .env.example .env`. |
| `password authentication failed` con Docker | El volumen se creó con otra contraseña. Iguala `POSTGRES_PASSWORD` y `DATABASE_URL` en `.env` a la original, o reinicia los datos con `docker compose down -v` (**borra la base**). |
| `port is already allocated` o `address already in use` en 5432 / 6379 / 5173 / 8080 | Otro programa usa el puerto. Cambia `DB_PORT` o `REDIS_PORT` en `.env` (y `DATABASE_URL`), o cierra el otro programa. |
| `docker: error during connect` | Docker Desktop no está abierto. Ábrelo y espera a que esté en verde. |
| El frontend carga pero el login da error de red | La API no está arriba; mira la terminal de `dev` o ejecuta `mise run check`. |
| `Netplus Project is not recognized` al arrancar la API | Air necesita el binario `.exe`; ya está configurado en `backend/.air.toml`. Si lo cambias, mantén `main.exe`. |

## Variables de entorno

`.env` (no se sube a Git) lo leen `docker compose` y mise. El ejemplo está en
[.env.example](.env.example). Si cambias el usuario, la contraseña o el puerto de
Postgres, actualiza también `DATABASE_URL`. Las migraciones SQL se aplican solas
al arrancar la API.

### Cuentas iniciales

Al arrancar con la tabla de usuarios vacía, la API crea un administrador y un
viewer con las credenciales de la tabla de arriba (o las de `ADMIN_EMAIL`,
`ADMIN_PASSWORD`, `VIEWER_EMAIL` y `VIEWER_PASSWORD` de `.env`). Son
credenciales públicas del prototipo: antes de usar información laboral real
define las tuyas en `.env` **antes del primer arranque**, o cámbialas desde
**Usuarios y permisos**. Con `SEED_DEMO=false` la base nueva arranca sin
personas ni marcaciones ficticias.

> Actualizando desde la versión anterior (login por usuario, sin datos de
> asistencia): la migración `002_attendance.sql` reemplaza las tablas de acceso,
> así que la cuenta creada en la pantalla de configuración inicial deja de
> existir y se recrean las cuentas iniciales.

### Pruebas de integración de la API

Necesitan una base PostgreSQL **vacía y desechable** (la prueba borra su
esquema). Con Docker levantado:

```powershell
docker compose exec -T postgres psql -U netplus -d postgres -c "CREATE DATABASE netplus_test;"
$env:TEST_DATABASE_URL = "postgres://netplus:<contraseña>@localhost:5432/netplus_test?sslmode=disable"
mise run test:api
```

Sin `TEST_DATABASE_URL` esas pruebas se omiten y solo corren las del lector de
Excel.

## Tareas disponibles

`mise tasks` las lista todas:

| Tarea | Qué hace |
|---|---|
| `mise run setup` | Instala dependencias de Go y de Node. |
| `mise run dev` | Docker (Postgres + Redis) + API + frontend. |
| `mise run dev:nodocker` | API + frontend, sin Docker. |
| `mise run check` | Verifica API, frontend, proxy y base de datos. |
| `mise run test` | Pruebas de Go (`test:api`) y del frontend (`test:web`). |
| `mise run db` / `db:stop` | Levanta / detiene Postgres y Redis en Docker. |
| `mise run api` / `web` | Solo la API (con `db`) o solo el frontend. |

VS Code detecta Go y TypeScript directamente; instala las extensiones **Go** y
**ESLint/TypeScript**. No hace falta ningún dev container.

## Respaldo de la base de datos (con Docker)

```powershell
docker compose exec postgres pg_dump -U netplus -d netplus > backup.sql
docker compose exec -T postgres psql -U netplus -d netplus < backup.sql
```

## Despliegue como servidor

Pendiente. El despliegue (binario Go + frontend compilado detrás de HTTPS) se
definirá en una iteración posterior. Vercel (`frontend/vercel.json`) sirve solo
la interfaz y la **demo del navegador**: la API en Go y PostgreSQL no se
publican allí.

## Contribuir

Una rama por cambio (`feature/...`), push y Pull Request a `main`. No subir
`.env`, `data/`, `node_modules/` ni `dist/`.
