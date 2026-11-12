-- Modelo de asistencia del frontend NettPlus. Reemplaza las tablas de acceso de la
-- iteración anterior (login por usuario) por cuentas con nombre y correo, y añade
-- los datos de asistencia. Aún no hay datos de producción que conservar.
DROP TABLE IF EXISTS audit_events;
DROP TABLE IF EXISTS sessions;
DROP TABLE IF EXISTS users;

CREATE TABLE users (
    seq           bigint GENERATED ALWAYS AS IDENTITY,
    id            text        PRIMARY KEY,
    name          text        NOT NULL,
    email         text        NOT NULL,
    role          varchar(16) NOT NULL CHECK (role IN ('admin', 'viewer')),
    active        boolean     NOT NULL DEFAULT true,
    password_hash text        NOT NULL
);
CREATE UNIQUE INDEX users_email_lower_key ON users (lower(email));

CREATE TABLE sessions (
    token_hash text        PRIMARY KEY, -- SHA-256 del token; el token solo vive en la cookie
    user_id    text        NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    expires_at timestamptz NOT NULL
);
CREATE INDEX sessions_user_id_idx ON sessions (user_id);
CREATE INDEX sessions_expires_at_idx ON sessions (expires_at);

-- Fila única: revisión (control de concurrencia optimista), origen de los datos y reglas.
CREATE TABLE app_state (
    id       smallint PRIMARY KEY CHECK (id = 1),
    revision bigint   NOT NULL,
    dataset  text     NOT NULL CHECK (dataset IN ('demo', 'imported', 'mixed', 'empty')),
    rules    jsonb    NOT NULL
);

CREATE TABLE employees (
    seq         bigint GENERATED ALWAYS AS IDENTITY,
    id          text PRIMARY KEY,
    name        text NOT NULL,
    department  text NOT NULL,
    status      text NOT NULL CHECK (status IN ('Activo', 'Inactivo')),
    active_from text,
    active_to   text
);

CREATE TABLE imports (
    seq        bigint GENERATED ALWAYS AS IDENTITY,
    id         text PRIMARY KEY,
    kind       text CHECK (kind IN ('permits', 'hours')), -- NULL = marcaciones de asistencia
    file       text        NOT NULL,
    sheet      text        NOT NULL,
    created_at timestamptz NOT NULL,
    user_name  text        NOT NULL,
    row_count  integer     NOT NULL,
    valid      integer     NOT NULL,
    duplicates integer     NOT NULL,
    invalid    integer     NOT NULL,
    warnings   integer     NOT NULL,
    issues     jsonb       NOT NULL DEFAULT '[]'
);
CREATE INDEX imports_created_at_idx ON imports (created_at DESC, seq DESC);

CREATE TABLE punches (
    seq         bigint GENERATED ALWAYS AS IDENTITY,
    id          text PRIMARY KEY,
    employee_id text    NOT NULL REFERENCES employees (id),
    import_id   text    NOT NULL REFERENCES imports (id),
    punch_date  text    NOT NULL, -- AAAA-MM-DD
    punch_time  text    NOT NULL, -- HH:MM:SS
    kind        text    NOT NULL CHECK (kind IN ('entrada', 'salida', 'sin clasificar')),
    source_row  integer NOT NULL,
    UNIQUE (employee_id, punch_date, punch_time, kind)
);
CREATE INDEX punches_date_idx ON punches (punch_date, employee_id);
CREATE INDEX punches_import_idx ON punches (import_id);

CREATE TABLE permits (
    seq         bigint GENERATED ALWAYS AS IDENTITY,
    id          text PRIMARY KEY,
    import_id   text    NOT NULL REFERENCES imports (id),
    employee_id text    NOT NULL, -- sin FK: puede no existir aún en el directorio
    name        text    NOT NULL,
    department  text    NOT NULL,
    starts_at   text    NOT NULL, -- AAAA-MM-DDTHH:MM:SS
    ends_at     text    NOT NULL,
    category    text    NOT NULL,
    approval    text    NOT NULL,
    source_row  integer NOT NULL
);
CREATE INDEX permits_import_idx ON permits (import_id);

CREATE TABLE source_hours (
    seq          bigint GENERATED ALWAYS AS IDENTITY,
    id           text PRIMARY KEY,
    import_id    text    NOT NULL REFERENCES imports (id),
    employee_id  text    NOT NULL,
    name         text    NOT NULL,
    department   text    NOT NULL,
    month        text    NOT NULL, -- AAAA-MM
    supplemental integer,          -- minutos; NULL = no informado
    extra        integer,
    night        integer,
    source_row   integer NOT NULL,
    UNIQUE (employee_id, month)
);
CREATE INDEX source_hours_import_idx ON source_hours (import_id);

CREATE TABLE audits (
    seq        bigint GENERATED ALWAYS AS IDENTITY,
    id         text PRIMARY KEY,
    created_at timestamptz NOT NULL,
    user_name  text        NOT NULL,
    action     text        NOT NULL,
    detail     text        NOT NULL
);
CREATE INDEX audits_created_at_idx ON audits (created_at DESC, seq DESC);
