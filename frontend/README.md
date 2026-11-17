# Nettplus Asistencia · frontend

Interfaz de control de asistencia y preparación de reportes para nómina. No
calcula salarios ni realiza pagos. Hace parte del repositorio Netplus: la API en
Go (carpeta [`../backend`](../backend)) guarda los datos en PostgreSQL. Cómo
levantar todo está en el [README principal](../README.md).

La interfaz usa el logo original y los colores del manual: azul `#233A84`, verde
`#73B542` y amarillo `#FDEA18`. El logo se sirve sin modificaciones y la fuente
es Inter (local); no se distribuyen las fuentes comerciales del manual.

## Comandos (desde esta carpeta, con Node de mise)

```powershell
npm install        # dependencias
npm run dev        # Vite en http://localhost:5173 (proxifica /api hacia :8080)
npm run build      # tsc + vite build
npm test           # Vitest: lógica de dominio, importación, reportes
npm run typecheck
```

Normalmente se usa `mise run dev` desde la raíz, que arranca también la API.

## Inicio de sesión

El acceso requiere correo y contraseña validados por la API en Go. La sesión
se guarda en una cookie HttpOnly/SameSite de 8 horas; los permisos y los datos
se validan en el servidor y se conservan en PostgreSQL. No se ofrecen cuentas
de prueba en pantalla ni acceso por demo. Los marcadores antiguos del navegador
se descartan; la sesión solo se restaura mediante `/api/session`.

Si la API o su base no están disponibles, el formulario muestra el estado y
bloquea el envío. **Comprobar conexión** permite reintentar. Las credenciales
incorrectas no habilitan ningún modo alternativo.

Las cuentas iniciales del servidor están en el README principal. El
administrador crea y edita usuarios, asigna roles, desactiva cuentas y cambia
contraseñas; cambiar la contraseña o el rol revoca las sesiones de esa cuenta, y
no puede desactivarse ni quitarse el rol a sí mismo. El viewer consulta y
exporta; la API responde 403 ante cualquier modificación, aunque se invoque
directamente.

## Funciones

- Dashboard con indicadores, filtros compartidos, comparaciones mensuales y
  gráficos interactivos (ECharts). Un clic en una barra diaria abre el reporte
  correspondiente.
- Importación real de `.xlsx`: reconocimiento de encabezados, hoja y fila
  configurables, vista previa, correspondencia de columnas, validación,
  duplicados e incidencias exportables. En modo servidor la API vuelve a leer y
  validar el archivo original antes de guardarlo.
- Directorio con búsqueda, estado, departamento, periodo e historial individual;
  se conservan archivo y fila de origen.
- Reportes diarios y consolidados mensuales exportables a Excel/PDF con los
  filtros activos.
- Novedades y horas: consulta y exportación de permisos registrados y horas
  informadas por el archivo.
- Usuarios, parámetros y auditoría para administradores.
- Los cambios se rechazan con 409 si se basan en una revisión obsoleta: la
  vista se actualiza y se pide revisar antes de reintentar.

La base inicial contiene personas y registros ficticios; un badge distingue la
base demo, los datos importados y las mezclas.

## Tres formatos de Excel

El importador identifica la fila de encabezados situada después del título. El
tipo de reporte puede cambiarse antes de confirmar.

| Tipo | Estructura admitida | Tratamiento |
|---|---|---|
| Marcaciones | ID del Empleado, Nombres, Departamento, Fecha, Hora y Tipo de Marcación; también fecha/hora juntas o entrada/salida separadas | Usa el tipo de marcación para distinguir entrada/salida. La columna adicional Tipo del biométrico no se interpreta como entrada/salida. |
| Permisos | ID del Empleado, Nombres, Apellidos, Departamento, Hora Entrada, Hora Salida, Código de pago y Estado de aprobación | Guarda fechas, categoría y aprobación. Omite motivos, comentarios y aprobadores. No crea marcaciones ni justifica ausencias. |
| Consolidado de horas | ID, Nombres, Apellidos, Departamento, Horas Suplementarias, Horas Extras y Horas Nocturnas | Detecta el mes en el título/hoja o permite elegirlo. Conserva duraciones de Excel, incluidas las mayores de 24 horas. Un campo vacío queda como No informado. No calcula ni aprueba horas. |

Para probarlos, descarga **Excel de ejemplo** desde Importaciones (también está
en `public/ejemplos/Nettplus_Ejemplo.xlsx`): cuatro hojas ficticias con
entrada/salida, eventos individuales, permisos y consolidado de octubre de 2026.
En la primera hoja hay tres filas válidas (una incompleta), una duplicada y dos
rechazadas; vuelve a cargar el mismo archivo para comprobar la deduplicación.
Cada confirmación importa una hoja.

Fechas admitidas: `AAAA-MM-DD`, `DD/MM/AAAA` y `DD-MM-AAAA`, además de fechas
nativas de Excel. Horas: `HH:MM` y `HH:MM:SS`. Los turnos nocturnos requieren
eventos con fecha individual. Los eventos sin tipo se conservan sin clasificar y
no se inventan salidas. Se conservan los códigos de texto y los rellenos
numéricos como `0000`.

Solo `.xlsx`, inicialmente hasta 20 MB (configurable entre 1 y 50 MB), máximo
100.000 filas y 150 columnas por hoja. No se ejecutan fórmulas. Los códigos con
nombres o departamentos contradictorios requieren revisión. Un consolidado ya
registrado para un ID/mes se conserva, sin sobrescribirlo.

## Indicadores y límites

- **Colaboradores:** personas del directorio que cumplen los filtros.
- **Asistencias:** pares únicos colaborador/fecha con al menos una marcación.
- **Puntualidad DEMO:** días dentro de tolerancia / días con entrada
  clasificada. Parámetros iniciales ficticios: 08:30 y 10 minutos.
- **Ausencias estimadas DEMO:** días configurados sin marcación, dentro de la
  vigencia laboral informada y de la cobertura importada. Requiere revisión
  humana.
- **Incompletos:** días sin entrada o sin salida clasificada.

Horarios, turnos, feriados, tolerancias, justificaciones y fórmulas oficiales
siguen pendientes de validación con Nettplus. Las fechas de asistencia son
fechas civiles; la auditoría se muestra en `America/Guayaquil`.

## Estructura

- `src/domain/`: modelo y cálculos de indicadores.
- `src/services/`: importadores, exportación a Excel/PDF y cliente de la API.
- `src/pages/`, `src/components/`, `src/context/`: pantallas y estado.
- La validación de Excel existe dos veces con el mismo comportamiento: aquí
  (`src/services/excel.ts`, `sourceDocuments.ts`, para la vista previa) y en la
  API (`backend/internal/excel`), que es la que decide al guardar. Si cambias
  una regla, cámbiala en ambos lados.
- El contrato JSON con la API está en `src/domain/types.ts` y
  `backend/internal/domain/types.go`.

## Vercel

Vercel sirve la interfaz (framework Vite, raíz `frontend/`, `npm run build`,
salida `dist`; `vercel.json` incluye el fallback de React Router). El acceso
requiere que `/api` llegue a la API Go con PostgreSQL; sin ese servicio no hay
inicio de sesión. Las credenciales de `.env` son exclusivas del servidor y no
deben copiarse a variables `VITE_` ni al bundle del navegador.
