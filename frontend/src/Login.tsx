import { useEffect, useState, type FormEvent } from "react";
import { api, ApiError, type User } from "./api";
import { ThemeSwitch } from "./ThemeSwitch";
import logo from "../media/Logo Nettplus Horizontal 4.png";
import icon from "../media/cropped-cropped-1-2-1-192x192.png";

type Props = {
  setupRequired: boolean;
  onAuthenticated: (user: User) => void;
};

const MIN_PASSWORD = 10;

function useServiceOnline() {
  const [online, setOnline] = useState<boolean | null>(null);
  useEffect(() => {
    let alive = true;
    api
      .health()
      .then(() => alive && setOnline(true))
      .catch(() => alive && setOnline(false));
    return () => {
      alive = false;
    };
  }, []);
  return online;
}

export function Login({ setupRequired, onAuthenticated }: Props) {
  const [username, setUsername] = useState("");
  const [password, setPassword] = useState("");
  const [confirm, setConfirm] = useState("");
  const [showPassword, setShowPassword] = useState(false);
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);
  const online = useServiceOnline();

  async function handleSubmit(e: FormEvent) {
    e.preventDefault();
    if (busy) return;

    if (!username.trim() || !password) {
      setError("Ingresa tu usuario y contraseña para continuar.");
      return;
    }
    if (setupRequired) {
      if (password.length < MIN_PASSWORD) {
        setError(`La contraseña debe tener al menos ${MIN_PASSWORD} caracteres.`);
        return;
      }
      if (password !== confirm) {
        setError("Las contraseñas no coinciden.");
        return;
      }
    }

    setError("");
    setBusy(true);
    try {
      const user = setupRequired
        ? await api.setup(username.trim(), password)
        : await api.login(username.trim(), password);
      onAuthenticated(user);
    } catch (err) {
      if (err instanceof ApiError && err.status === 429) {
        const mins = Math.max(1, Math.ceil((err.retryAfter ?? 60) / 60));
        setError(`Demasiados intentos fallidos. Intenta de nuevo en ${mins} min.`);
      } else {
        setError(err instanceof Error ? err.message : "Ocurrió un error inesperado.");
      }
      setBusy(false);
    }
  }

  return (
    <div className="stage">
      <ThemeSwitch />
      <div className="islands">
        {/* Isla de marca */}
        <section className="island island--brand">
          <div className="logo">
            <img className="logo__icon" src={icon} alt="" width={48} height={48} />
            <div className="logo__text">
              <strong>NETPLUS</strong>
              <small>ASISTENCIA</small>
            </div>
          </div>
          <div className="brand__copy">
            <h1>Tu asistencia, en un solo panorama.</h1>
            <p>Importa los archivos de Z-BioTime y revisa marcaciones, permisos y horas en dashboards claros.</p>
          </div>
        </section>

        {/* Isla del formulario */}
        <section className="island island--form" aria-labelledby="login-title">
          <img className="form__logo" src={logo} alt="Nettplus — The power of fiber" />
          <header>
            <p className="eyebrow">{setupRequired ? "Configuración inicial" : "Bienvenido"}</p>
            <h2 id="login-title">{setupRequired ? "Crear administrador" : "Iniciar sesión"}</h2>
          </header>

          <form onSubmit={handleSubmit} noValidate>
            <label className="field">
              <span>Usuario</span>
              <input
                type="text"
                autoComplete="username"
                autoCapitalize="none"
                spellCheck={false}
                placeholder="nombre.apellido"
                value={username}
                onChange={(e) => setUsername(e.target.value)}
                disabled={busy}
              />
            </label>

            <label className="field">
              <span>Contraseña</span>
              <div className="field__password">
                <input
                  type={showPassword ? "text" : "password"}
                  autoComplete={setupRequired ? "new-password" : "current-password"}
                  placeholder="••••••••••"
                  value={password}
                  onChange={(e) => setPassword(e.target.value)}
                  disabled={busy}
                />
                <button
                  type="button"
                  className="link"
                  onClick={() => setShowPassword((v) => !v)}
                  aria-pressed={showPassword}
                >
                  {showPassword ? "Ocultar" : "Mostrar"}
                </button>
              </div>
            </label>

            {setupRequired && (
              <label className="field">
                <span>Confirmar contraseña</span>
                <input
                  type={showPassword ? "text" : "password"}
                  autoComplete="new-password"
                  placeholder="••••••••••"
                  value={confirm}
                  onChange={(e) => setConfirm(e.target.value)}
                  disabled={busy}
                />
                <small className="hint">Mínimo {MIN_PASSWORD} caracteres.</small>
              </label>
            )}

            {error && (
              <p className="error" role="alert">
                {error}
              </p>
            )}

            <button type="submit" className="btn" disabled={busy}>
              {busy ? "Procesando…" : setupRequired ? "Crear y continuar" : "Ingresar"}
            </button>

            {!setupRequired && (
              <p className="hint">¿Olvidaste tu contraseña? Solicítala al administrador.</p>
            )}
          </form>
        </section>

        {/* Islas pequeñas */}
        <aside className="island island--stat">
          <span className="stat__icon" aria-hidden="true">▤</span>
          <div>
            <strong>Excel → Dashboard</strong>
            <small>Importa, valida y visualiza</small>
          </div>
        </aside>

        <aside className="island island--status" aria-live="polite">
          <span className={`dot ${online === false ? "dot--off" : online === null ? "dot--wait" : ""}`} aria-hidden="true" />
          <span>{online === null ? "Verificando servicio…" : online ? "Servicio en línea" : "Servicio sin conexión"}</span>
        </aside>
      </div>

      <p className="footer">© {new Date().getFullYear()} Netplus</p>
    </div>
  );
}
