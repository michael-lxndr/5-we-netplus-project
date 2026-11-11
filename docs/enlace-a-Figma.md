# Prototipo de interfaz (Figma) — Netplus

## Enlace

- **Archivo de Figma:** _pendiente de crear._ Pegar aquí la URL cuando exista
  (formato: `https://www.figma.com/design/<ID>/Netplus`).
- **Prototipo navegable:** _pendiente._
- **Responsable del archivo:** _pendiente._
- **Permisos:** _pendiente_ (quién puede editar y quién solo ver).

## Qué cubre el prototipo

El prototipo muestra las pantallas y cómo navega una persona entre ellas. El
modelo de datos y la arquitectura se documentan aparte (ver `../docs/`). Los
requisitos (RF), las reglas (RN, VAL) y los datos de cada tipo de archivo están en
[`01-analisis.md`](../docs/01-analisis.md).

| # | Pantalla | Rol | Requisito |
|---|---|---|---|
| P01 | Asistente de instalación (crear primer administrador) | Instalador | RF-01 |
| P02 | Inicio de sesión (incluye el bloqueo temporal) | Todos | RF-02 |
| P03 | Panel del administrador: dashboards y estado de importaciones | `admin` | RF-15, RF-19 |
| P04 | Cargar archivo Excel | `admin` | RF-06 |
| P05 | Vista previa y confirmación: tipo y período detectados, hojas para elegir, nuevos / actualizados / sin cambios, advertencias | `admin` | RF-07, RF-08, RF-11, RF-12 |
| P06 | Errores y advertencias de validación (hoja, fila, columna, regla y motivo) | `admin` | RF-09, RF-10 |
| P07 | Resumen y estado de la importación (en cola, procesando, importado, fallido) | `admin` | RF-14, RF-15 |
| P08 | Detalle de datos con pestañas Marcaciones, Permisos y Horas; el texto libre solo se ve como `admin` | Todos | RF-17 |
| P09 | Configurar gráfico: fuente, dimensión, medida, tipo y filtros | `admin` | RF-16 |
| P10 | Publicar dashboard | `admin` | RF-18 |
| P11 | Gestión de usuarios `viewer` | `admin` | RF-04, RF-05 |
| P12 | Auditoría | `admin` | RF-21 |
| P13 | Dashboard de horas (consolidado) | Todos | RF-19 |
| P14 | Dashboard de permisos | Todos | RF-19 |
| P15 | Dashboard de marcaciones y horas trabajadas | Todos | RF-19 |
| P16 | Exportar reporte (CSV o XLSX) | Todos | RF-20 |

## Recorridos a prototipar

1. **Primer uso:** P01 → P02 → P03.
2. **Importar un archivo con éxito:** P03 → P04 → P05 → P07 → P08.
3. **Importar un archivo con errores:** P03 → P04 → P06 → P04 (volver a cargar).
4. **Descartar una carga:** P04 → P05 → P03.
5. **Reimportar un archivo:** P04 → P05 (con "sin cambios" y "actualizados") → P07.
6. **Publicar un dashboard:** P08 → P09 → P10.
7. **Administrar usuarios:** P03 → P11.
8. **Consultar como `viewer`:** P02 → P13, P14 o P15 → P16.

## Estados que conviene diseñar en cada pantalla

Vacío (sin datos todavía), cargando, error y éxito. Además:

- **P02:** bloqueo temporal por demasiados intentos.
- **P05:** una hoja válida, una hoja con errores (no se puede elegir) y una hoja
  omitida por no ser reconocible.
- **P13 a P15:** "sin datos" para el rango de fechas elegido.

## Qué ve cada rol

- El `admin` ve todas las pantallas y el texto libre de los permisos (motivos y
  comentarios) y de las observaciones del consolidado.
- El `viewer` ve P02, P08 (sin texto libre), P13 a P15 y P16, solo de los dashboards
  publicados.

## Lineamientos visuales (propuesta)

- Reutilizar la paleta de [`flujo-usuario.puml`](../docs/flujo-usuario.puml):
  azul oscuro `#183153`, verde azulado `#0D9488`, fondo claro `#F7F9FC` y
  `#E6FFFB`.
- Interfaz en español, pensada para Tailwind CSS y componentes accesibles:
  contraste suficiente, foco visible y navegación por teclado.
- Gráficos de barras, líneas y pie (Apache ECharts); las tablas, con TanStack Table.
- Usar nombres y datos inventados en los frames; no pegar datos reales de empleados.
- Nombrar los frames con el número de pantalla (por ejemplo, `P06 - Errores de
  validación`).

## Pendiente

- Crear el archivo de Figma y completar la sección "Enlace".
- Confirmar con RRHH qué dashboards necesitan primero (sección 9 del análisis).
