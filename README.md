# Netplus

Entorno inicial para una aplicación que recibe archivos Excel y genera
dashboards. La primera iteración incluye una API mínima en Go, un frontend
React + TypeScript, PostgreSQL y Redis. Tailwind, autenticación e importación
Excelize se incorporarán en las siguientes iteraciones.

Todo el runtime de desarrollo se ejecuta en Docker. En la computadora solo se
necesita Docker Desktop; el código se edita en la carpeta clonada y se monta
como volumen dentro de los contenedores. Así, Go, Node, TypeScript, PostgreSQL
y Redis usan las mismas versiones para todo el equipo.

## Requisitos

- Docker Desktop con Docker Compose v2.
- Puerto libre `5173` para desarrollo local.

No es necesario instalar Go, Node, npm, TypeScript, PostgreSQL ni Redis en el
host.

## Arrancar en una computadora

```powershell
Copy-Item .env.example .env
docker compose up --build
```

- Frontend: <http://localhost:5173>
- API interna: <http://localhost:5173/api/health>

En este modo los puertos se publican en `127.0.0.1`; ningún otro equipo de la

El primer `--build` crea las imágenes de Go y Node. Las siguientes veces basta
con `docker compose up`, salvo que cambien `Dockerfile`, `go.mod` o
`package.json`.

Comandos útiles:

```powershell
docker compose ps
docker compose logs -f api
docker compose exec api go version
docker compose exec frontend node --version
docker compose down
```

Para trabajar dentro de VS Code, instala la extensión **Dev Containers** y usa
`Dev Containers: Reopen in Container`. La configuración de
[.devcontainer/devcontainer.json](.devcontainer/devcontainer.json) reutiliza
el mismo Compose.

## Arrancar como servidor para varios equipos

Copiar `.env.example` a `.env.server` y cambiar:

```dotenv
APP_MODE=server
WEB_BIND_IP=0.0.0.0
VITE_API_URL=/api
```

Luego ejecutar:

```powershell
docker compose --env-file .env.server up --build -d
```

Abrir únicamente `WEB_PORT` en el firewall del servidor y acceder desde los
clientes con `http://IP_DEL_SERVIDOR:5173`. La API Go se comunica mediante el
proxy interno de Vite; PostgreSQL y Redis tampoco se exponen a la red. En una
etapa posterior se puede reemplazar Vite por un proxy HTTPS de producción,
manteniendo un solo puerto público.

Para una instalación real, publicar la aplicación detrás de HTTPS y cambiar
`POSTGRES_PASSWORD` por un secreto fuerte. Los valores del archivo `.env` no
deben subirse al repositorio.

## Subir el proyecto a GitHub

El repositorio debe contener los fuentes, Dockerfiles, Compose y archivos de
configuración de ejemplo. No debe contener `.env`, `data/`, `node_modules/`,
`dist/` ni volúmenes de Docker.

Desde la carpeta del proyecto:

```powershell
git init
git add .
git commit -m "Create Docker development environment"
git branch -M main
git remote add origin https://github.com/USUARIO/REPOSITORIO.git
git push -u origin main
```

Después, otra persona puede comenzar así:

```powershell
git clone https://github.com/USUARIO/REPOSITORIO.git
Set-Location REPOSITORIO
Copy-Item .env.example .env
docker compose up --build
```

GitHub guarda el código y la configuración; las bases de datos y los archivos
subidos por usuarios permanecen en los volúmenes y en `data/` de cada
instalación, no en Git.

## Cambiar entre local y servidor

No se cambia el código de la aplicación. Se cambia el archivo de entorno:

- Local: bind `127.0.0.1`, una sola computadora.
- Servidor: bind web en `0.0.0.0`, servidor con IP fija o reserva DHCP.

La migración de datos se hace respaldando y restaurando PostgreSQL:

```powershell
docker compose exec postgres pg_dump -U netplus -d netplus > backup.sql
docker compose exec -T postgres psql -U netplus -d netplus < backup.sql
```

El cambio de red no crea usuarios automáticamente. La futura autenticación
deberá usar la misma base de datos, con roles `admin` y `viewer`, tanto en local
como en servidor.
