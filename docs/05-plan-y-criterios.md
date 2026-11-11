# Plan y criterios de aceptación — Netplus

> Versión final (4 de noviembre de 2026). Los requisitos (RF, RNF), las reglas
> (RN, VAL) y las decisiones (DEC, C) se definen en [01-analisis.md](01-analisis.md).

## 1. Etapas

Prioridad: **Alta** = sin esto no hay producto utilizable; **Media** = necesario
para la entrega completa.

| Etapa | Objetivo | Prioridad | Requisitos cubiertos | Depende de |
|---|---|---|---|---|
| E0 | Entorno base en Docker | Alta | RNF-10, RF-23 (parcial) | — |
| E1 | Autenticación, roles y usuarios | Alta | RF-01 a RF-05, RNF-01 a RNF-04 | E0 |
| E2 | Núcleo de importación y esquema de datos | Alta | RF-06, RF-07, RF-09, RF-13, RNF-05, RNF-07 | E1 |
| E3 | Importación del consolidado de horas con vista previa y confirmación | Alta | RF-08, RF-10 a RF-12, RF-14, RNF-17 | E2 |
| E4 | Importación de permisos | Alta | RF-07, RF-09, RF-14, RN-05 | E3 |
| E5 | Importación de marcaciones y horas trabajadas | Alta | RF-07, RF-09, RF-14, RN-14 | E3 |
| E6 | Dashboards y tablas de detalle | Alta | RF-16 a RF-19 | E3, E4, E5 |
| E7 | Exportación de reportes | Media | RF-20 | E6 |
| E8 | Modo servidor: worker, estados y HTTPS | Media | RF-15, RF-23, RNF-06 | E3 |
| E9 | Auditoría y respaldos | Media | RF-21, RF-22, RNF-13, RNF-15 | E1 |
| E10 | Pruebas y entrega | Alta | Todos | Todas |

El consolidado va primero porque tiene una fila por empleado y mes y permite
probar de punta a punta todo el flujo (carga, vista previa, confirmación y
reimportación). Las marcaciones van después porque son las más voluminosas y
requieren el cálculo de horas trabajadas.

### E0 — Entorno base en Docker

Estado: creada según el [README](../README.md) y el `compose.yaml`.

- Docker Compose con `api`, `frontend`, `postgres` y `redis`.
- Dockerfiles de Go y de Node con el código montado como volumen.
- Perfil `local` / `server` mediante archivo de entorno.
- Instrucciones de arranque, respaldo y subida a GitHub.

### E1 — Autenticación, roles y usuarios

- Incorporar Tailwind CSS y la estructura base del frontend.
- Migraciones de `users`, `sessions` y `audit_events`.
- Asistente de instalación para crear el primer `admin`.
- Inicio y cierre de sesión con cookies seguras (`COOKIE_SECURE`), hash argon2id
  y bloqueo por intentos fallidos.
- Middleware de autorización por rol en el backend (`AuthorizationPolicy`).
- Pantallas de gestión de usuarios `viewer`.

### E2 — Núcleo de importación y esquema de datos

- Migraciones de `uploads`, `import_issues`, `employees`, `departments`,
  `positions`, `attendance_marks`, `leave_requests` y `monthly_hours`.
- `ExcelReader` con Excelize sobre `io.Reader` y recorrido por iterador de filas.
- `CellNormalizer`: enteros, fechas, duraciones y texto (hallazgos H-01 a H-10).
- Interfaz `SheetParser` y reconocimiento del tipo por hoja.
- Validación de tamaño, extensión y contenido real; guardado del original en `./data`.
- Estados de una carga y registro de `import_issues`.

### E3 — Importación del consolidado con vista previa y confirmación

- `MonthlyHoursSheetParser` con las reglas VAL-30 a VAL-33.
- Vista previa: período, filas leídas, nuevos, actualizados y sin cambios.
- Selección de hojas, confirmación y descarte.
- Importación atómica por archivo y reimportación según RN-12.
- Pantalla de errores y advertencias; resumen de importación.

### E4 — Importación de permisos

- `LeavesSheetParser` con las reglas VAL-20 a VAL-23.
- Normalización de estados y actualización de permisos existentes.
- Texto libre visible solo para el `admin`.

### E5 — Importación de marcaciones y horas trabajadas

- `MarksSheetParser` con las reglas VAL-10 a VAL-13.
- `WorkedHoursCalculator`: pares entrada→salida y marcaciones incompletas.
- Pruebas con volumen (unas 10 000 filas).

### E6 — Dashboards y tablas de detalle

- Migración de `dashboard_configs`.
- Plantillas de dashboards de la sección 9 del análisis con Apache ECharts.
- Filtros por rango de fechas, departamento, código de pago y estado.
- Tablas de detalle con TanStack Table.
- Publicación, despublicación y vista del `viewer`.

### E7 — Exportación de reportes

- Exportación a CSV y XLSX con los filtros aplicados.
- Exclusión del texto libre en las exportaciones del `viewer`.

### E8 — Modo servidor: worker, estados y HTTPS

- Cola en Redis y worker Go para importaciones pesadas (más de 10 000 filas o 5 MB).
- Estados `queued`, `processing`, `imported` y `failed` visibles en la interfaz.
- Proxy HTTPS de producción en lugar del servidor de desarrollo de Vite.
- Firewall abierto solo en el puerto web; PostgreSQL y Redis privados.

### E9 — Auditoría y respaldos

- Registro de eventos en todas las acciones de RN-20.
- Pantalla de consulta de auditoría.
- Procedimiento de respaldo y restauración (PostgreSQL + `data`) y su prueba.
- Comando `reset-admin` y su documentación.

### E10 — Pruebas y entrega

Antes de entregar una instalación se prueba, como mínimo:

1. La importación de los tres archivos de ejemplo (anonimizados) y de uno inválido por tipo.
2. Un acceso como `viewer`, incluidos los intentos de usar funciones de `admin`.
3. Un respaldo y su restauración en una instalación limpia.
4. El perfil `local` y el perfil `server` desde otro equipo de la red.

## 2. Datos de prueba

Los tres archivos de ejemplo son la base de los criterios de aceptación. Los números
de la columna "Criterio" salen de ellos:

| Archivo de ejemplo | Contenido |
|---|---|
| Reporte de marcaciones | 22 filas, 1 empleado, del 12 al 17 de octubre de 2026; 11 entradas y 11 salidas. |
| Reporte de permisos | 59 filas, 42 empleados, 6 códigos de pago; 54 `Aprobado`, 4 `Rechazado` y 1 `Revocar`. |
| Consolidado de julio de 2026 | 99 filas, 13 departamentos, 35 cargos; 1 fila oculta, 2 celdas `h:mm` y 1 ID numérico. |

Los archivos reales traen datos personales y motivos de permisos relacionados con la
salud, así que **no deben subirse al repositorio**. Para las pruebas automáticas se
generan versiones anonimizadas con la misma estructura y las mismas rarezas, y se
guardan en `backend/testdata/`.

## 3. Criterios de aceptación

Cada criterio indica cuándo se considera terminada la función.

### 3.1 Acceso y usuarios

| ID | Req. | Criterio |
|---|---|---|
| CA-01 | RF-01, RN-02 | Al terminar el asistente de instalación existe un único usuario `admin` activo; si ya hay usuarios, el asistente no permite crear otro administrador. |
| CA-02 | RF-02 | Al iniciar sesión con credenciales válidas, el sistema crea una sesión con cookie `HttpOnly` y `SameSite=Lax` y muestra la vista que corresponde al rol. |
| CA-03 | RF-02 | Al iniciar sesión con credenciales inválidas, el sistema muestra un mensaje genérico (sin indicar si falló el usuario o la contraseña), no crea sesión y registra el intento en la auditoría. |
| CA-04 | RF-02, RN-17 | Tras 5 intentos fallidos en 15 minutos, el sistema rechaza nuevos intentos durante 15 minutos, incluso con credenciales correctas. |
| CA-05 | RF-03 | Al cerrar sesión, la cookie deja de ser válida y volver atrás en el navegador no muestra datos protegidos. |
| CA-06 | RF-04 | Al crear un `viewer` con un nombre de usuario que ya existe, el sistema muestra un error y no guarda el registro; con un nombre nuevo, el usuario queda activo con rol `viewer`. |
| CA-07 | RF-05 | Al desactivar un usuario, no puede iniciar sesión y sus sesiones abiertas dejan de ser válidas. |
| CA-08 | RN-03 | Si solo queda un `admin` activo, el sistema no permite desactivarlo; un `admin` tampoco puede desactivarse a sí mismo. |
| CA-09 | RN-18, RNF-03 | Al crear o cambiar una contraseña, en la base de datos solo queda el hash argon2id y la contraseña anterior deja de funcionar. |

### 3.2 Carga y validación (todos los tipos)

| ID | Req. | Criterio |
|---|---|---|
| CA-10 | VAL-01 | Al cargar un archivo que supera el tamaño máximo (25 MB por defecto), el sistema lo rechaza con un mensaje y no lo guarda. |
| CA-11 | VAL-01 | Al cargar un archivo `.xlsx` cuyo contenido no es un XLSX real (por ejemplo, un `.txt` renombrado), el sistema lo rechaza y no lo guarda. |
| CA-12 | VAL-02, RN-09 | Al cargar un libro sin ninguna hoja reconocible, el sistema lo rechaza, no conserva el original y explica qué título y encabezados espera para cada tipo. |
| CA-13 | VAL-02, RN-09 | Al cargar un libro con una hoja reconocible y otra que no, la hoja desconocida se omite con una advertencia y la reconocida se puede importar. |
| CA-14 | VAL-03, RF-10 | Cada error indica hoja, fila, columna, regla y motivo, y la pantalla permite corregir el archivo y volver a cargarlo. |
| CA-15 | RN-06 | Si una hoja tiene errores, no se puede elegir para importar; las hojas sin errores sí. |
| CA-16 | RN-06, RF-11 | Antes de confirmar, no se escribe ningún registro de datos: solo quedan el `upload` y sus `import_issues`. |
| CA-17 | RF-12, RN-07 | Al descartar una carga validada, el archivo original se elimina de `data/`, el estado pasa a `discarded` y no queda ningún registro importado. |
| CA-18 | RNF-17 | Si la importación falla a mitad de camino (fallo simulado), no queda ningún registro nuevo ni actualizado, el estado es `failed` y se muestra el motivo. |
| CA-19 | RN-21 | Un archivo con macros o con texto que parece HTML no ejecuta nada y el texto se muestra literal. |

### 3.3 Consolidado mensual (`monthly_hours`)

| ID | Req. | Criterio |
|---|---|---|
| CA-20 | RF-07, RF-11 | Con el consolidado de ejemplo, la vista previa muestra tipo `monthly_hours`, período julio de 2026, 99 filas leídas, 99 nuevos, 0 errores y 1 advertencia (fila oculta, ID 55). |
| CA-21 | H-05, H-06 | Las dos celdas con formato `h:mm` y el ID numérico se aceptan; una duración de 102:45 se guarda como 6165 minutos. |
| CA-22 | H-09 | Una celda vacía en "Horas Nocturnas" se guarda como 0 minutos y la fila es válida. |
| CA-23 | VAL-30 | Si el título dice un mes y la hoja otro, o no se puede leer el período, el sistema muestra un error. |
| CA-24 | VAL-31, VAL-32 | Una duración negativa, con texto inválido o mayor que 744 horas se rechaza con hoja, fila y columna. |
| CA-25 | VAL-33 | Un empleado repetido en la hoja se rechaza. |
| CA-26 | RN-12 | Al cargar de nuevo el mismo consolidado, la vista previa muestra 0 nuevos y 99 sin cambios; si cambia un valor, muestra 1 actualizado y, al confirmar, sigue habiendo un solo registro por empleado y mes. |
| CA-27 | RN-11, H-11 | Si un empleado figura con otro departamento en otro archivo, cada registro conserva el departamento de su archivo y no se informa error. |

### 3.4 Permisos (`leaves`)

| ID | Req. | Criterio |
|---|---|---|
| CA-28 | RF-07, RF-11 | Con el reporte de permisos de ejemplo, la vista previa muestra tipo `leaves`, 59 filas leídas, 42 empleados, 59 nuevos y 0 errores. |
| CA-29 | RN-12 | Tras importarlo hay 54 permisos `approved`, 4 `rejected` y 1 `revoked`, con 6 códigos de pago distintos. |
| CA-30 | VAL-21 | Un estado que no sea `Aprobado`, `Rechazado` o `Revocar` (por ejemplo, `Pendiente`) se rechaza con hoja, fila y columna. |
| CA-31 | VAL-20 | Una `Hora Salida` anterior a la `Hora Entrada` se rechaza. |
| CA-32 | VAL-23 | Un permiso repetido en el archivo se rechaza. |
| CA-33 | RN-12 | Al cargar de nuevo el mismo archivo, la vista previa muestra 0 nuevos y 59 sin cambios; si en un permiso cambia el estado, muestra 1 actualizado. |
| CA-34 | H-10 | Un apellido `..` se guarda como vacío. |
| CA-35 | RN-05 | El `viewer` no ve ni exporta `reason` ni `approval_comment`; el `admin` sí los ve en el detalle. |

### 3.5 Marcaciones (`marks`)

| ID | Req. | Criterio |
|---|---|---|
| CA-36 | RF-07, RF-11 | Con el reporte de marcaciones de ejemplo, la vista previa muestra tipo `marks`, 22 filas, 1 empleado, 22 nuevos, 0 errores y 0 advertencias. |
| CA-37 | VAL-10 | Una hora inválida (por ejemplo, `25:61`) o una fecha que no sea `dd-mm-aaaa` se rechaza con hoja, fila y columna. |
| CA-38 | VAL-11 | Un `Tipo de Marcación` distinto de `Entrada` y `Salida` se rechaza. |
| CA-39 | VAL-12 | Una marcación repetida se ignora y se informa como advertencia. |
| CA-40 | VAL-13, RN-14 | Una entrada sin salida el mismo día se importa con una advertencia y aparece en la lista de marcaciones incompletas. |
| CA-41 | RN-14 | Las horas trabajadas del 12-10-2026 son 486 minutos (8 h 06 min) y las del 17-10-2026 son 276 minutos (4 h 36 min). |
| CA-42 | RN-12 | Al cargar de nuevo el mismo archivo, la vista previa muestra 0 nuevos y 22 sin cambios. |

### 3.6 Datos cruzados

| ID | Req. | Criterio |
|---|---|---|
| CA-43 | RN-10 | Tras importar los tres archivos de ejemplo hay 99 empleados: los 42 de permisos y el de marcaciones ya existían en el consolidado. |
| CA-44 | VAL-05 | Si un empleado ya existente llega con otro nombre, se actualiza y se informa como advertencia. |

### 3.7 Dashboards, exportación y administración

| ID | Req. | Criterio |
|---|---|---|
| CA-45 | RF-16 | Al configurar un gráfico de barras, líneas o pie con una fuente, una dimensión y una medida válidas, la vista previa muestra los datos importados; si la configuración no es válida, no permite guardarla. |
| CA-46 | RF-18 | Al publicar un dashboard, un `viewer` lo ve en su lista en la siguiente consulta; al despublicarlo, deja de verlo. |
| CA-47 | RF-19 | Al aplicar un filtro de departamento o un rango de fechas, los gráficos y las tablas muestran solo los datos que cumplen el criterio; si no hay datos, se muestra "sin datos". |
| CA-48 | RN-15 | Un permiso del 6 al 9 de julio aparece al filtrar del 8 al 10 de julio, porque se solapa con el rango. |
| CA-49 | RN-04 | Si un `viewer` intenta abrir un dashboard no publicado (por la interfaz o llamando a la API), el backend no devuelve datos. |
| CA-50 | RN-01, RN-22 | Si un `viewer` llama directamente a un endpoint de administración (carga, usuarios, configuración o auditoría), el backend responde `403` y registra el evento. |
| CA-51 | RF-20, RN-24 | Al exportar un reporte permitido a CSV o XLSX, el archivo contiene los mismos datos filtrados que muestra la pantalla; si el reporte no está permitido, el backend rechaza la exportación. |
| CA-52 | RF-21, RN-20 | Al subir, confirmar o descartar una importación, publicar un dashboard, crear un usuario, exportar o fallar un inicio de sesión, aparece un evento de auditoría con usuario, acción, recurso y fecha. |
| CA-53 | RF-22, RNF-13 | Al restaurar un respaldo en una instalación limpia, están disponibles los usuarios, los dashboards y los datos importados, y los Excel originales están en `data/`. |

### 3.8 Despliegue y rendimiento

| ID | Req. | Criterio |
|---|---|---|
| CA-54 | RF-23, RNF-06 | En perfil `local`, el puerto web solo responde desde `127.0.0.1`; en perfil `server`, responde desde otro equipo de la red y PostgreSQL y Redis no son accesibles desde él. |
| CA-55 | RF-15, RN-19 | En modo servidor, una importación de más de 10 000 filas responde `202`, y el estado pasa por `queued`, `processing` e `imported` (o `failed` con el motivo); en modo local la misma importación la procesa la API. |
| CA-56 | RNF-07, RNF-08 | Con un archivo de marcaciones sintético de unas 10 000 filas, la importación tarda menos de 30 segundos y la memoria de la API no crece en proporción al tamaño del archivo. |
| CA-57 | RN-23 | Las pruebas de autorización dan el mismo resultado en el perfil `local` y en el perfil `server`. |

## 4. Definición de terminado

Una función se considera terminada cuando:

- Cumple sus criterios de aceptación de la sección 3.
- La autorización se verifica en el backend, no solo en la interfaz.
- Las migraciones se aplican desde cero y sobre una base existente.
- Funciona con `docker compose up` en el perfil `local`; si afecta a la red, también
  en el perfil `server`.
- Las acciones relevantes quedan registradas en la auditoría.
- Las pruebas automáticas usan archivos anonimizados, nunca los reales.
- La documentación afectada (README, análisis o diagramas) está actualizada.
