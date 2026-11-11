# Análisis — Netplus

> Versión final (4 de noviembre de 2026). Se basa en [arquitectura.md](arquitectura.md),
> [flujo-usuario.puml](flujo-usuario.puml) y en tres archivos Excel de ejemplo
> entregados por la empresa: un reporte de marcaciones, un reporte de permisos y
> un consolidado mensual de horas. Las decisiones tomadas para cerrar las dudas
> anteriores están en la sección 10; lo que solo la empresa puede confirmar está
> en la sección 11.

## 1. Contexto y problema

El área de Recursos Humanos (RRHH) trabaja con tres archivos Excel que salen del
sistema de control de asistencia o se arman a mano:

- **Reporte de marcaciones:** cada entrada y salida registrada por un empleado.
- **Reporte de permisos:** solicitudes de permisos, vacaciones y licencias, con su
  estado de aprobación.
- **Consolidado mensual de horas:** horas suplementarias, extras y nocturnas de
  cada empleado en el mes.

Hoy esa información se revisa archivo por archivo. No hay una vista conjunta por
departamento, por empleado o por mes; los archivos tienen formatos distintos entre
sí (fechas, horas, tipos de celda) y algunos se editan a mano, por lo que pueden
traer errores. Además, no queda registro de quién cargó cada archivo ni quién puede
ver datos que incluyen información personal y de salud.

Afecta a:

- **RRHH (administrador):** pierde tiempo cruzando archivos y corrigiendo datos.
- **Jefaturas y personas que consultan (viewer):** dependen de que RRHH les envíe
  resúmenes y no pueden filtrar por su cuenta.

## 2. Objetivo

**General:** construir una aplicación web que reciba los archivos Excel de
asistencia de RRHH, los valide e importe, y los muestre en dashboards con filtros,
con acceso controlado por roles.

**Específicos:**

- Reconocer automáticamente el tipo de cada archivo (marcaciones, permisos o
  consolidado) y validarlo antes de importar, indicando hoja, fila y columna de
  cada error.
- Mostrar una vista previa de lo que se va a importar (registros nuevos,
  actualizados y sin cambios) y pedir confirmación.
- Permitir volver a cargar un archivo sin duplicar información.
- Publicar dashboards de horas, permisos y marcaciones para usuarios de solo
  consulta.
- Proteger los datos personales: el texto libre de los permisos (motivos y
  comentarios) solo lo ve el administrador.
- Instalarse en una sola computadora (sin Internet) o en un servidor de red
  local/VPN, con el mismo código y cambiando solo la configuración.

## 3. Alcance

### Incluido en la primera versión

- Asistente de instalación que crea el primer administrador.
- Inicio y cierre de sesión con roles `admin` y `viewer`; gestión de usuarios `viewer`.
- Carga de archivos `.xlsx` de los tres tipos descritos en la sección 5.
- Importación en dos pasos: validar y mostrar vista previa, luego confirmar.
- Reimportación sin duplicados (sección 8, RN-12).
- Dashboards con gráficos de barras, líneas y pie, filtros y rango de fechas.
- Tablas de detalle por tipo de dato.
- Publicación de dashboards, exportación a CSV y XLSX, auditoría.
- Respaldo y restauración (PostgreSQL + carpeta `data`).
- Dos perfiles de instalación (`local` y `server`) con el mismo código.

### Fuera de la primera versión

- Cálculo de puntualidad, tardanzas o horas extras a partir de horarios de trabajo:
  no hay un archivo de horarios (ver C-07).
- Unir automáticamente el código de cargo de las marcaciones con el nombre de cargo
  del consolidado: falta una tabla de equivalencias (ver C-05).
- Mantenimiento manual de empleados, departamentos o cargos: se crean y actualizan
  solo al importar.
- Revertir una importación ya confirmada (se corrige cargando de nuevo el archivo).
- Almacenamiento de objetos (MinIO); los originales se guardan en `./data`.
- Otros formatos (CSV, `.xls`, `.xlsm`) y exportación a PDF.
- Edición de datos dentro de la aplicación o escritura de vuelta a Excel.
- Registro de usuarios por cuenta propia, recuperación de contraseña por correo,
  SSO u OAuth.
- Varias empresas en una misma instalación.
- Aplicación móvil nativa y notificaciones o alertas.
- Proxy HTTPS de producción: es obligatorio para una instalación real en modo
  servidor, pero se implementa en una etapa posterior al entorno de desarrollo
  (ver [05-plan-y-criterios.md](05-plan-y-criterios.md)).

## 4. Usuarios y contexto

| Rol | Quién es | Qué necesita | Circunstancias de uso |
|---|---|---|---|
| `admin` | Persona de RRHH responsable de los datos y de la instalación. | Cargar y validar archivos, confirmar importaciones, configurar y publicar dashboards, ver el detalle completo (incluido el texto libre), administrar usuarios, revisar auditoría y respaldos. | Desde la PC principal o desde otro equipo con navegador. |
| `viewer` | Jefatura o persona que solo consulta. | Ver dashboards publicados, filtrar por fechas y departamento, y exportar reportes. | Desde cualquier equipo de la red con navegador y la URL del servidor. |

**Contexto de despliegue**

- **Local:** una sola computadora, sin Internet, accesible solo desde `127.0.0.1`.
- **Servidor:** una PC principal con IP fija (o reserva DHCP) que atiende a otros
  equipos de una red local o VPN. Los clientes solo necesitan un navegador.

**Volumen esperado (según los ejemplos):** unos 100 empleados y 13 departamentos;
un consolidado de 99 filas por mes; unos 60 permisos por mes; y unas 4 marcaciones
por empleado y día hábil, es decir, del orden de 10 000 marcaciones por mes.

## 5. Fuentes de datos

### 5.1 Los tres tipos de archivo

| Tipo (`FileType`) | Contenido | Cómo se reconoce | Una fila representa | Clave natural | Al reimportar |
|---|---|---|---|---|---|
| `marks` | Reporte de marcaciones | A1 = "Reporte de Marcaciones"; encabezados en la fila 2 | Una marcación (entrada o salida) | empleado + fecha y hora + tipo | Las existentes se omiten |
| `leaves` | Reporte de permisos | A1 = "Permiso"; encabezados en la fila 2 | Una solicitud de permiso, vacaciones o licencia | empleado + inicio + fin + código de pago | Las existentes se actualizan |
| `monthly_hours` | Consolidado mensual de horas | A1 empieza por "CONSOLIDADO MENSUAL"; encabezados en la fila 2; la hoja se llama "Mes Año" | Un empleado en un mes | empleado + año + mes | Las existentes se reemplazan |

Los tres ejemplos tienen una sola hoja. Un mismo libro puede traer varias hojas
(por ejemplo, un consolidado con varios meses); cada hoja se reconoce por separado.

### 5.2 Reporte de marcaciones (`marks`)

Ejemplo: 22 marcaciones de un empleado, del 12 al 17 de octubre de 2026 (11
entradas y 11 salidas). La hoja se llama como la fecha de exportación (`AAAAMMDD`).

| Columna del Excel | Celda | Campo destino | Regla |
|---|---|---|---|
| Id del Empleado | Texto numérico | `employees.code` | Entero positivo. |
| Nombres | Texto | `employees.first_name` | Obligatorio; sin espacios sobrantes. |
| ID de Departamento | Texto numérico | `departments.code` | Entero. |
| Departamento | Texto | `departments.name` | Obligatorio. |
| ID del Cargo | Texto numérico | `positions.code` | Entero. |
| Fecha | Texto `dd-mm-aaaa` | `attendance_marks.marked_at` (fecha) | Fecha válida. |
| Hora | Texto `hh:mm` | `attendance_marks.marked_at` (hora) | Entre 00:00 y 23:59. |
| Tipo de Marcación | Texto | `attendance_marks.kind` | `Entrada` → `entry`, `Salida` → `exit`; otro valor es error. |
| Tipo | Texto | `attendance_marks.method` | Texto libre (en el ejemplo: `Rostro&Huella`, `Cualquiera`). |
| Fuente de Datos | Texto | `attendance_marks.source` | Texto libre (en el ejemplo: `Dispositivo`, `Mobil`). |

### 5.3 Reporte de permisos (`leaves`)

Ejemplo: 59 solicitudes de 42 empleados; inicios entre el 22 de junio y el 18 de
julio de 2026; 6 códigos de pago; 54 aprobadas, 4 rechazadas y 1 con estado `Revocar`.

| Columna del Excel | Celda | Campo destino | Regla |
|---|---|---|---|
| ID del Empleado | Texto numérico | `employees.code` | Entero positivo. |
| Nombres | Texto | `employees.first_name` | Obligatorio. |
| Apellidos | Texto | `employees.last_name` | El valor `..` o vacío se guarda como vacío. |
| nombre de empresa | Texto | `employees.company` | Obligatorio. |
| Departamento | Texto | `leave_requests.department_id` | Obligatorio. |
| Hora Entrada | Texto `aaaa-mm-dd hh:mm:ss` | `leave_requests.starts_at` | Fecha y hora válidas. |
| Hora Salida | Texto `aaaa-mm-dd hh:mm:ss` | `leave_requests.ends_at` | Fecha y hora válidas; no anterior a `starts_at`. |
| Motivo de Aprobación | Texto libre | `leave_requests.reason` | Opcional. Solo lo ve el `admin` (RN-05). |
| Hora | Texto `aaaa-mm-dd hh:mm:ss` | `leave_requests.recorded_at` | Fecha y hora válidas (ver C-01). |
| Código de pago | Texto | `leave_requests.leave_type` | Obligatorio. En el ejemplo: `Permisos Personales \| Hora`, `Permisos Personales \| día`, `Vacaciones`, `Permiso Médico`, `Licencia de Paternidad`, `Licencia por Fallecimiento`. |
| Estado de aprobación | Texto | `leave_requests.status` | `Aprobado` → `approved`, `Rechazado` → `rejected`, `Revocar` → `revoked`; otro valor es error. |
| Comentarios de aprobación | Texto libre | `leave_requests.approval_comment` | Opcional. Solo lo ve el `admin`. |
| Aprobador | Texto | `leave_requests.approver` | Opcional; es el usuario del sistema de origen. |

### 5.4 Consolidado mensual de horas (`monthly_hours`)

Ejemplo: hoja "Julio 2026" con 99 empleados, 13 departamentos y 35 cargos. La hoja
y el título de la celda A1 ("CONSOLIDADO MENSUAL - JULIO 2026") indican el período.

| Columna del Excel | Celda | Campo destino | Regla |
|---|---|---|---|
| ID | Texto numérico (una fila trae un número) | `employees.code` | Entero positivo; sin repetidos en la hoja. |
| Nombres | Texto | `employees.first_name` | Obligatorio. |
| Apellidos | Texto | `employees.last_name` | El valor `..` o vacío se guarda como vacío. |
| Empresa | Texto | `employees.company` | Obligatorio. |
| Departamento | Texto | `monthly_hours.department_id` | Obligatorio. |
| Cargo | Texto | `monthly_hours.position_id` (por nombre) | Obligatorio. |
| Horas Suplementarias | Duración `[h]:mm` | `monthly_hours.supplementary_minutes` | Duración válida, mayor o igual que 0. |
| Horas Extras | Duración `[h]:mm` | `monthly_hours.extra_minutes` | Igual. |
| Horas Nocturnas | Duración `[h]:mm` | `monthly_hours.night_minutes` | Igual; vacío equivale a 0. |
| Observaciones | Texto libre | `monthly_hours.notes` | Opcional. Solo lo ve el `admin`. |

Las columnas K a M de la hoja existen pero están vacías; se ignoran.

### 5.5 Hallazgos en los archivos de ejemplo y cómo se tratan

| ID | Hallazgo | Tratamiento |
|---|---|---|
| H-01 | En marcaciones y permisos todas las celdas son texto (IDs, fechas y horas). | Se convierten a su tipo por columna; no se depende del formato de celda. |
| H-02 | Los formatos de fecha difieren: `dd-mm-aaaa` y `hh:mm` en marcaciones; `aaaa-mm-dd hh:mm:ss` en permisos. | Cada tipo de archivo define su formato; también se aceptan fechas nativas de Excel. |
| H-03 | En marcaciones y permisos la hoja se llama como la fecha de exportación y no como el período de los datos (permisos del 22-06 al 18-07 exportados el 26-10). | El período real sale de los datos. Solo en el consolidado la hoja indica el período. |
| H-04 | La fila 1 es un título combinado y los encabezados están en la fila 2. | El tipo se reconoce por el título de A1 y por los encabezados. |
| H-05 | Las duraciones del consolidado pasan de 24 horas (la mayor es 102:45) y 2 celdas de horas suplementarias usan el formato `h:mm`, que Excel entrega como hora del día. | Se aceptan ambos tipos y se convierten a minutos. |
| H-06 | El ID del consolidado es texto en 98 filas y número en una. | Se normaliza a entero. |
| H-07 | Hay espacios sobrantes en nombres (2 en el consolidado y 2 en permisos) y en el encabezado "Observaciones ". | Se recortan los espacios; los encabezados se comparan sin distinguir mayúsculas ni espacios. |
| H-08 | Hay una fila oculta con datos en el consolidado (ID 55). | Se importa y se informa como advertencia (ver C-02). |
| H-09 | "Horas Nocturnas" está vacía en 94 de 99 filas y 14 filas no tienen ninguna hora. | Vacío equivale a 0 y la fila es válida. |
| H-10 | Todos los apellidos vienen como `..`. | Se tratan como vacíos. |
| H-11 | Los IDs de empleado coinciden entre archivos (los 42 de permisos están en el consolidado), pero 8 de ellos tienen otro departamento, y cada archivo tiene departamentos que el otro no tiene. | El empleado se identifica por ID; el departamento se guarda en cada registro tal como viene en el archivo (ver C-06). |
| H-12 | Las marcaciones traen el código del cargo (19) y el consolidado solo el nombre. | Se guardan por separado, sin unirlos (ver C-05). |
| H-13 | Cada día hábil tiene 4 marcaciones (entrada, salida, entrada, salida) y el sábado 17-10 solo 2. Las 2 marcaciones hechas con `Cualquiera`/`Mobil` son salidas. | Entradas y salidas se emparejan en orden por empleado y día; una marcación sin pareja es advertencia. |
| H-14 | En permisos, 17 de 59 solicitudes abarcan varios días y la columna `Hora` es posterior al inicio del permiso en 35 de 59. | Se guarda como `recorded_at` sin interpretar (ver C-01). |
| H-15 | Los motivos y comentarios de permisos incluyen información de salud. | Solo el `admin` los ve (RN-05, RNF-16). |
| H-16 | El aprobador es un usuario del sistema de origen, no un nombre de persona. | Se guarda como texto. |

### 5.6 Reglas de validación

**Error:** el archivo no se importa hasta corregirlo. **Advertencia:** se informa en
la vista previa pero no bloquea.

| ID | Aplica a | Severidad | Regla |
|---|---|---|---|
| VAL-01 | Todos | Error | La extensión es `.xlsx`, el contenido es un XLSX real y el tamaño no supera el máximo. |
| VAL-02 | Todos | Error | Al menos una hoja es reconocible: A1 identifica un tipo y están todos los encabezados esperados (en cualquier orden). Las hojas no reconocidas se omiten con una advertencia. |
| VAL-03 | Todos | Error | Una celda obligatoria vacía o con un valor que no corresponde a su tipo; el error indica hoja, fila y columna. |
| VAL-04 | Todos | Advertencia | Columnas adicionales con datos (se ignoran) y filas ocultas con datos (se importan). |
| VAL-05 | Todos | Advertencia | Un empleado ya existente llega con otro nombre o empresa; se actualiza. |
| VAL-10 | `marks` | Error | Fecha `dd-mm-aaaa` y hora `hh:mm` válidas. |
| VAL-11 | `marks` | Error | `Tipo de Marcación` distinto de `Entrada` y `Salida`. |
| VAL-12 | `marks` | Advertencia | Marcación repetida en el archivo (mismo empleado, fecha, hora y tipo); se ignora la repetida. |
| VAL-13 | `marks` | Advertencia | Entrada sin salida, o salida sin entrada, en el mismo día. |
| VAL-20 | `leaves` | Error | Fechas inválidas o `Hora Salida` anterior a `Hora Entrada`. |
| VAL-21 | `leaves` | Error | `Estado de aprobación` fuera de `Aprobado`, `Rechazado` y `Revocar`. |
| VAL-22 | `leaves` | Error | `Código de pago` vacío. |
| VAL-23 | `leaves` | Error | Permiso repetido en el archivo (mismo empleado, entrada, salida y código de pago). |
| VAL-30 | `monthly_hours` | Error | El mes y el año no se pueden leer, o el título y el nombre de la hoja no coinciden. |
| VAL-31 | `monthly_hours` | Error | Una duración no es una hora o duración válida, o es negativa. |
| VAL-32 | `monthly_hours` | Error | Una duración supera 744 horas (31 días), que no caben en un mes. |
| VAL-33 | `monthly_hours` | Error | Empleado repetido en la hoja. |

## 6. Requisitos funcionales

| ID | Requisito | Rol |
|---|---|---|
| RF-01 | Crear el primer usuario `admin` durante el asistente de instalación. | Instalador |
| RF-02 | Iniciar sesión con usuario y contraseña y recibir una sesión segura. | Todos |
| RF-03 | Cerrar sesión. | Todos |
| RF-04 | Crear usuarios `viewer`. | `admin` |
| RF-05 | Cambiar la contraseña, activar y desactivar usuarios. | `admin` |
| RF-06 | Cargar un archivo `.xlsx`. | `admin` |
| RF-07 | Reconocer el tipo de cada hoja (`marks`, `leaves`, `monthly_hours`) y, en el consolidado, su período. | Sistema |
| RF-08 | Elegir, entre las hojas reconocidas, las que se van a importar. | `admin` |
| RF-09 | Validar tamaño, extensión, contenido real, encabezados, tipos de dato y las reglas VAL de la sección 5.6. | Sistema |
| RF-10 | Mostrar errores y advertencias por hoja, fila y columna, y permitir corregir el archivo y volver a cargarlo. | `admin` |
| RF-11 | Mostrar una vista previa: tipo y período detectados, filas leídas, registros nuevos, actualizados y sin cambios, advertencias y primeras filas. | `admin` |
| RF-12 | Confirmar o descartar la importación. | `admin` |
| RF-13 | Guardar el archivo original en `./data` cuando supera la validación. | Sistema |
| RF-14 | Importar: normalizar y guardar empleados, departamentos, cargos y registros, y mostrar un resumen. | Sistema |
| RF-15 | En modo servidor, enviar las importaciones pesadas a un worker (Redis) y mostrar el estado de cada una. | Sistema / `admin` |
| RF-16 | Configurar dashboards: elegir fuente de datos, dimensión, medida, tipo de gráfico (barras, líneas, pie) y filtros. | `admin` |
| RF-17 | Consultar tablas de detalle de marcaciones, permisos y horas, con orden y filtros. | Todos |
| RF-18 | Publicar y despublicar dashboards. | `admin` |
| RF-19 | Consultar dashboards aplicando filtros y rango de fechas (`viewer`: solo publicados; `admin`: todos). | Todos |
| RF-20 | Exportar a CSV o XLSX un reporte permitido. | Todos |
| RF-21 | Consultar la auditoría de acciones. | `admin` |
| RF-22 | Respaldar y restaurar la base de datos y la carpeta `data`. | `admin` |
| RF-23 | Elegir el perfil `local` o `server` mediante configuración, sin duplicar código. | Instalador |

## 7. Requisitos no funcionales

| ID | Categoría | Requisito |
|---|---|---|
| RNF-01 | Seguridad | Sesiones con cookies `HttpOnly`, `Secure` y `SameSite=Lax`. |
| RNF-02 | Seguridad | La autorización se aplica en el backend en cada endpoint; ocultar botones en el frontend no cuenta como control. |
| RNF-03 | Seguridad | Las contraseñas se guardan solo como hash (argon2id); nunca en texto plano ni se migran así. |
| RNF-04 | Seguridad | Límite de intentos de inicio de sesión y registro de auditoría. |
| RNF-05 | Seguridad | Límite de tamaño de subida configurable; se valida extensión y contenido real; no se ejecutan macros ni se interpreta el contenido del Excel como HTML. |
| RNF-06 | Seguridad de red | En modo servidor se usa HTTPS (también dentro de una VPN), solo se publica el puerto web y PostgreSQL, Redis y `./data` permanecen privados; solo el servidor escribe en ellos. |
| RNF-07 | Rendimiento | Las hojas grandes se recorren con un iterador de filas, sin cargar el archivo completo en memoria; en modo servidor las importaciones pesadas corren en un worker para no bloquear la API. |
| RNF-08 | Rendimiento | Metas propuestas: importar un consolidado de 100 filas en menos de 5 s; un mes de marcaciones (unas 10 000 filas) en menos de 30 s; abrir un dashboard con un año de datos en menos de 2 s. |
| RNF-09 | Accesibilidad | Componentes accesibles: navegación por teclado, foco visible, etiquetas en formularios y contraste suficiente. |
| RNF-10 | Portabilidad | Mismo código y mismo Compose en ambos perfiles; el desarrollo corre en Docker Desktop sin instalar Go, Node, PostgreSQL ni Redis en el equipo. |
| RNF-11 | Operación | El perfil `local` funciona sin conexión a Internet. |
| RNF-12 | Mantenibilidad | Migraciones versionadas (`golang-migrate`) y consultas tipadas (SQLC); mismas entidades y reglas de autorización en ambos perfiles. |
| RNF-13 | Recuperación | El respaldo incluye PostgreSQL y `./data`; la restauración se prueba antes de entregar una instalación. |
| RNF-14 | Usabilidad | Interfaz en español; cada error de validación indica hoja, fila, columna y motivo. |
| RNF-15 | Trazabilidad | Las acciones relevantes quedan registradas en `audit_events`. |
| RNF-16 | Protección de datos | Los archivos contienen datos personales y, en permisos, información de salud, que en Ecuador la Ley Orgánica de Protección de Datos Personales trata como categoría especial. El texto libre solo lo ve el `admin`, el disco y los respaldos deben protegerse, y la empresa define quién accede y por cuánto tiempo se conserva (C-08). Este documento no es asesoría legal. |
| RNF-17 | Fiabilidad | Una importación confirmada es atómica por archivo: o se guardan todos los registros o ninguno. Si falla, el estado queda `failed` y los datos anteriores no cambian. |

## 8. Reglas de negocio

| ID | Regla |
|---|---|
| RN-01 | Solo existen los roles `admin` y `viewer`. El `viewer` no puede cargar archivos ni modificar la configuración. |
| RN-02 | El primer `admin` se crea únicamente durante la instalación. |
| RN-03 | Solo el `admin` crea usuarios `viewer`, cambia contraseñas o desactiva usuarios. Un usuario inactivo no puede iniciar sesión. Siempre debe quedar al menos un `admin` activo y un `admin` no puede desactivarse a sí mismo. |
| RN-04 | El `viewer` solo ve dashboards publicados; el `admin` ve todos. |
| RN-05 | El texto libre (`reason`, `approval_comment` y `notes`) solo lo ven los `admin`. No aparece en las pantallas del `viewer` ni en sus exportaciones. |
| RN-06 | Una importación tiene dos pasos: validar y confirmar. Una hoja con errores no se puede importar; las hojas sin errores se pueden elegir en la vista previa. Si ninguna hoja es válida, el archivo se rechaza y no se conserva el original. Las advertencias no bloquean. |
| RN-07 | El original válido se conserva sin modificarse en `./data`. Si el `admin` descarta la carga, se elimina. |
| RN-08 | Estados de una carga: `rejected`, `validated`, `queued`, `processing`, `imported`, `failed` y `discarded`. |
| RN-09 | El tipo de archivo se reconoce por hoja (título de A1 y encabezados). Las hojas no reconocidas se omiten y se informan; si ninguna es reconocible, el archivo se rechaza. |
| RN-10 | El empleado se identifica por su ID del sistema de origen. El nombre y la empresa se actualizan con el último archivo importado. |
| RN-11 | El departamento y el cargo son atributos de cada registro, como figuran en el archivo. Se comparan sin distinguir mayúsculas, tildes ni espacios sobrantes. |
| RN-12 | Al reimportar nunca se borran registros. `marks`: las marcaciones existentes se omiten. `leaves`: los permisos existentes se actualizan (estado, aprobador, comentarios, motivo y registro). `monthly_hours`: los registros existentes del mismo empleado y mes se reemplazan. |
| RN-13 | Las duraciones se guardan en minutos enteros, redondeando los segundos. |
| RN-14 | Las horas trabajadas son un dato derivado: suma de salida menos entrada de cada par consecutivo entrada→salida, por empleado y día. Las marcaciones sin pareja no suman y se listan como incompletas. |
| RN-15 | Un permiso entra en un rango de fechas si se solapa con él. Su duración es la diferencia entre fin e inicio, en horas de calendario. |
| RN-16 | Las fechas y horas se guardan en hora local de Ecuador continental, sin conversión de zona horaria. |
| RN-17 | Tras 5 intentos fallidos en 15 minutos, el inicio de sesión se bloquea 15 minutos (valores configurables). |
| RN-18 | Las contraseñas nunca se almacenan ni se migran en texto plano. |
| RN-19 | El tamaño máximo de archivo es 25 MB (configurable). Una importación es pesada si supera 10 000 filas o 5 MB; solo en modo servidor se envía al worker, y en modo local la procesa la API (Redis es opcional). |
| RN-20 | Subir, validar, confirmar o descartar una importación, publicar dashboards, crear o desactivar usuarios, cambiar contraseñas, exportar y los accesos denegados o fallidos generan un evento de auditoría. |
| RN-21 | El contenido del Excel se trata siempre como dato: no se ejecutan macros ni se interpreta como HTML. |
| RN-22 | Toda acción restringida se autoriza en el backend, sin importar lo que muestre el frontend. |
| RN-23 | Las entidades y las reglas de autorización son las mismas en `local` y en `server`. |
| RN-24 | Solo se puede exportar un reporte al que el usuario tiene acceso según RN-04 y RN-05. |

## 9. Dashboards y reportes previstos

El `admin` los configura (RF-16) a partir de estas plantillas. Todos admiten
filtros por rango de fechas y departamento. Los de permisos admiten además filtro
por código de pago y estado.

| Fuente | Dashboard | Gráfico |
|---|---|---|
| Horas (`monthly_hours`) | Horas por departamento y tipo (suplementarias, extras, nocturnas) | Barras apiladas |
| Horas | Evolución mensual de horas (cuando hay más de un mes cargado) | Líneas |
| Horas | Los 10 empleados con más horas extras | Barras |
| Horas | Distribución por tipo de hora | Pie |
| Permisos (`leaves`) | Permisos por código de pago | Pie |
| Permisos | Permisos por estado de aprobación | Barras |
| Permisos | Horas de permiso por departamento | Barras |
| Permisos | Evolución semanal o mensual de permisos | Líneas |
| Permisos | Solicitudes por aprobador | Barras |
| Marcaciones (`marks`) | Horas trabajadas por empleado y día | Barras / tabla |
| Marcaciones | Marcaciones por método y fuente | Pie |
| Marcaciones | Hora promedio de entrada y de salida | Líneas |
| Marcaciones | Marcaciones incompletas (calidad de datos) | Tabla |

Cada fuente tiene además una tabla de detalle con orden y filtros (RF-17).

## 10. Decisiones adoptadas

Cierran las dudas D-01 a D-14 de la versión anterior de este documento.

| ID | Tema | Decisión |
|---|---|---|
| DEC-01 | Estructura del Excel | Tres tipos de archivo con esquema fijo (sección 5). |
| DEC-02 | Selección de hojas | Importación en dos pasos: el sistema valida y lista las hojas reconocidas; el `admin` elige y confirma en la vista previa. |
| DEC-03 | Tamaño y tarea pesada | Máximo 25 MB; pesada si supera 10 000 filas o 5 MB (RN-19). |
| DEC-04 | Reimportación | Idempotente por clave natural, sin borrar registros (RN-12). |
| DEC-05 | Almacenamiento de filas | Tablas propias por tipo de dato; no se usa JSONB. |
| DEC-06 | Sesiones | Tabla `sessions` en PostgreSQL, para que Redis siga siendo opcional en modo local. |
| DEC-07 | Contraseñas | Hash argon2id, mínimo 10 caracteres; bloqueo según RN-17. |
| DEC-08 | Administradores | Puede haber varios; siempre queda al menos uno activo (RN-03). |
| DEC-09 | Exportación | CSV y XLSX de lo que muestra el dashboard o la tabla, con los filtros aplicados (RN-24). |
| DEC-10 | Respaldos | Manuales, por línea de comandos como en el README (`pg_dump` y copia de `data`); sin pantalla en la primera versión. Los originales y la auditoría se conservan sin límite de tiempo. |
| DEC-11 | Recuperación de contraseña | El `admin` restablece la de un `viewer`. Para el único `admin` se documenta un procedimiento por línea de comandos (comando `reset-admin`, por implementar). |
| DEC-12 | Cookie `Secure` | Se controla con la variable `COOKIE_SECURE`: activa con HTTPS y desactivable solo en desarrollo sobre `http://IP`. En `localhost` los navegadores aceptan la cookie `Secure`. |
| DEC-13 | Rendimiento | Metas de RNF-08; volumen de la sección 4. |
| DEC-14 | Zona horaria | Hora local sin conversión (RN-16). |

## 11. Puntos a confirmar con RRHH

Los archivos de ejemplo no permiten resolverlos. Mientras tanto, el sistema aplica
el tratamiento indicado.

| ID | Pregunta | Mientras tanto |
|---|---|---|
| C-01 | ¿La columna `Hora` de permisos es el momento de la solicitud o de la aprobación? | Se guarda como `recorded_at`, sin interpretarla. |
| C-02 | ¿La fila oculta del consolidado (ID 55) se ocultó para excluirla? | Se importa con advertencia. |
| C-03 | ¿Qué diferencia hay entre "Horas Suplementarias" y "Horas Extras" y cómo se calculan? ¿Salen de las marcaciones? | Se guardan tal como vienen, sin recalcular ni cruzar con marcaciones. |
| C-04 | ¿El estado `Revocar` equivale a un permiso revocado? | Se guarda como `revoked`. |
| C-05 | ¿Existe una tabla de equivalencias entre el código y el nombre del cargo? | Se guardan por separado. |
| C-06 | ¿El departamento distinto de 8 empleados entre archivos es un cambio real o una diferencia de catálogos? | Se guarda el de cada archivo. |
| C-07 | ¿Hay horarios de trabajo para calcular puntualidad y horas extras? | Fuera de la primera versión. |
| C-08 | ¿Quién puede ver el detalle de cada empleado y cuánto tiempo se conservan los datos personales? | `viewer` sin texto libre; conservación sin límite (DEC-10). |

## 12. Documentos relacionados

- [arquitectura.md](arquitectura.md): arquitectura y stack.
- [flujo-usuario.puml](flujo-usuario.puml): flujo de actividad general.
- [02-casos-de-uso.puml](02-casos-de-uso.puml): casos de uso y secuencias.
- [03-diagrama-de-clases.puml](03-diagrama-de-clases.puml): modelo del dominio.
- [04-modelo-de-datos.puml](04-modelo-de-datos.puml): tablas de PostgreSQL.
- [05-plan-y-criterios.md](05-plan-y-criterios.md): etapas y criterios de aceptación.
- [../diseño/enlace-a-Figma.md](../diseño/enlace-a-Figma.md): prototipo de interfaz.
