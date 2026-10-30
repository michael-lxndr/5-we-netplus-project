# Arquitectura propuesta para el sistema de dashboards

El sistema tendrá dos formas de instalación, pero conservará el mismo código de
negocio y la misma interfaz web.

## Roles

| Rol | Permisos |
|---|---|
| `admin` | Iniciar sesión, cargar Excel, seleccionar hojas, validar/importar datos, administrar dashboards y consultar todos los reportes. |
| `viewer` | Iniciar sesión y consultar dashboards y reportes publicados. No puede cargar archivos ni modificar la configuración. |

El primer usuario administrador se crea durante la instalación. El administrador
puede crear usuarios `viewer` y cambiar sus contraseñas o desactivarlos.

## Versión local

Está pensada para una sola computadora y no requiere Internet:

```text
Navegador
    |
    v
Aplicación Go (API + frontend servido)
    |
    +-- Excelize: lectura y validación de XLSX
    +-- PostgreSQL: usuarios, archivos procesados y métricas
    +-- ./data: archivos originales y copias de seguridad
```

- Se instala como un ejecutable o como un paquete Docker local.
- La aplicación escucha únicamente en `127.0.0.1`.
- El procesamiento ocurre en la misma computadora.
- PostgreSQL se usa desde el inicio para que la migración a servidor no cambie
  el motor de base de datos.
- Redis es opcional en esta modalidad y se activa cuando se procesen tareas
  pesadas en segundo plano.
- Los respaldos deben incluir la base de datos y la carpeta `data`.

## Versión multiusuario/servidor

Está pensada para varias computadoras conectadas a una red local o VPN:

```text
PC del admin       PC de un viewer       PC de otro viewer
      \                  |                    /
       +----------- navegador --------------+
                           |
                    HTTP/HTTPS
                           |
                 PC principal / servidor
                           |
                 Go API + worker de Excel
                    |             |
              PostgreSQL        ./data
                    |
                  Redis
```

- La PC principal es el servidor y el procesador central.
- Los demás equipos solo necesitan un navegador y la URL del servidor.
- PostgreSQL soporta los accesos concurrentes y se mantiene igual que en la
  versión local.
- Los Excel originales se conservan en un volumen local `./data`; MinIO queda
  fuera de la primera versión.
- Redis y un worker Go procesan archivos grandes sin bloquear la API.
- La aplicación debe publicarse mediante HTTPS, incluso dentro de una VPN.
- Solo el servidor debe tener acceso de escritura a PostgreSQL, Redis y `./data`.

## Stack común

### Backend

- Go 1.24+.
- `chi` para routing HTTP.
- Excelize para abrir el archivo subido mediante `io.Reader` y recorrer hojas
  grandes con un iterador de filas.
- SQLC para consultas tipadas.
- `golang-migrate` para migraciones.
- Sesiones con cookies `HttpOnly`, `Secure` y `SameSite=Lax`.

### Frontend

- React + TypeScript + Vite.
- Tailwind CSS y componentes accesibles.
- TanStack Query para consultar la API.
- TanStack Table para previsualizar filas y errores.
- Apache ECharts para gráficos de barras, líneas, pie y filtros interactivos.

### Seguridad y validación

- Límite de tamaño de subida configurable.
- Validar extensión, contenido real del archivo, hojas y columnas esperadas.
- Nunca ejecutar macros ni tratar el contenido del Excel como HTML.
- Registrar quién subió, validó o publicó cada archivo.
- Aplicar autorización en el backend; ocultar botones en el frontend no es
  suficiente.
- Limitar intentos de inicio de sesión y registrar auditoría.

## Modelo de datos mínimo

```text
users
  id, username, password_hash, role, active, created_at

uploads
  id, original_name, storage_key, status, uploaded_by, created_at

import_errors
  id, upload_id, row_number, column_name, message

datasets
  id, upload_id, name, imported_at

dashboard_configs
  id, name, config_json, published, created_by, updated_at

audit_events
  id, user_id, action, resource_type, resource_id, created_at
```

Estas tablas viven en PostgreSQL en ambas versiones. Las entidades y las reglas
de autorización deben ser iguales en local y en servidor.

## Flujo de despliegue recomendado

1. Construir un binario Go y el frontend estático.
2. Elegir el perfil `local` o `server` mediante configuración, no mediante
   código duplicado.
3. En `local`, publicar los puertos en `127.0.0.1` y usar PostgreSQL con
   almacenamiento en disco.
4. En `server`, publicar únicamente el puerto web en la IP de la PC principal;
   la API Go se comunica por la red interna de Compose y PostgreSQL, Redis y
   los archivos permanecen privados.
5. Crear el primer `admin` durante el asistente de instalación.
6. Probar una importación de Excel, un acceso `viewer` y un respaldo antes de
   entregar la instalación.

## Cambio entre local y servidor

No existe una aplicación local y otra aplicación diferente. Es el mismo
Compose y el mismo código; cambia la configuración de red:

- **Local:** `127.0.0.1`, una sola computadora y acceso desde ese equipo.
- **Servidor:** `0.0.0.0` solo para el frontend, IP fija o reserva DHCP y
  acceso desde otros equipos. El frontend hace proxy interno hacia la API.

Para pasar datos de una instalación a otra se respalda y restaura PostgreSQL.
Los archivos de Excel se copian junto con la carpeta `data`. Nunca se migran
contraseñas en texto plano.
