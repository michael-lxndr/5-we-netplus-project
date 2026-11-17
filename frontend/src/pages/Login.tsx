import { useState } from "react";
import {
  ArrowRight,
  Check,
  ChevronRight,
  Eye,
  EyeOff,
  FileSpreadsheet,
  Fingerprint,
  LockKeyhole,
  Mail,
  RefreshCw,
  ShieldCheck,
  UsersRound,
} from "lucide-react";

export default function Login({
  available,
  onLogin,
  onCheck,
}: {
  available: boolean;
  onLogin: (email: string, password: string) => Promise<void>;
  onCheck: () => Promise<void>;
}) {
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [show, setShow] = useState(false);
  const [capsLock, setCapsLock] = useState(false);
  const [busy, setBusy] = useState<"login" | "connection" | null>(null);
  const [error, setError] = useState("");

  async function run(
    kind: "login" | "connection",
    action: () => Promise<void>,
  ) {
    if (busy) return;
    setBusy(kind);
    setError("");
    try {
      await action();
    } catch (e) {
      setError(e instanceof Error ? e.message : "No se pudo iniciar sesión.");
    } finally {
      setBusy(null);
    }
  }

  return (
    <div className="login-page">
      <div className="login-ambient" aria-hidden="true">
        <i />
        <i />
        <i />
      </div>
      <header className="login-header">
        <a href="/" className="login-logo" aria-label="Nettplus, inicio">
          <img
            src="/nettplus-logo.png"
            alt="Nettplus · The Power of Fiber"
            width="220"
            height="75"
          />
        </a>
        <span className="login-header-label">
          <span /> Portal del equipo
        </span>
      </header>

      <main className="login-layout" id="main-content">
        <section className="login-story" aria-labelledby="login-story-title">
          <span className="login-eyebrow">
            <span /> PERSONAS. CONEXIONES. POSIBILIDADES.
          </span>
          <h1 id="login-story-title">
            Todo tu equipo.
            <br />
            <span>En sintonía.</span>
          </h1>
          <p className="login-description">
            Cada jornada cuenta. Conecta la asistencia, las novedades y los
            reportes en un solo lugar.
          </p>
          <div className="login-fiber-scene" aria-hidden="true">
            <div className="login-fiber-orb">
              {[0, 1, 2, 3, 4, 5].map((i) => (
                <i key={i} style={{ rotate: `${i * 28 - 70}deg` }} />
              ))}
              <span className="login-orb-core">
                <Fingerprint size={66} strokeWidth={1} />
              </span>
            </div>
            <div className="login-float login-float-team">
              <span className="login-float-icon">
                <UsersRound size={20} />
              </span>
              <div>
                <strong>Tu gente, conectada</strong>
                <small>Una visión compartida</small>
              </div>
              <span className="login-float-check">
                <Check size={13} />
              </span>
            </div>
            <div className="login-float login-float-data">
              <span className="login-float-icon">
                <FileSpreadsheet size={19} />
              </span>
              <div>
                <strong>De los datos a la claridad</strong>
                <small>Importa · Revisa · Consulta</small>
              </div>
            </div>
            <span className="login-scene-caption">THE POWER OF FIBER</span>
          </div>
          <div className="login-story-note">
            <span className="login-note-line" /> Menos archivos dispersos. Más
            tiempo para tu equipo.
          </div>
        </section>

        <section className="login-panel" aria-labelledby="login-title">
          <div className="login-card-top">
            <span className="login-lock">
              <LockKeyhole size={23} strokeWidth={1.7} />
            </span>
            <span className="login-card-label">TU ESPACIO NETTPLUS</span>
          </div>
          <h2 id="login-title">Qué bueno tenerte aquí.</h2>
          <p className="login-intro">Ingresa con tu cuenta para continuar.</p>
          <div
            className={`login-connection ${available ? "is-connected" : "is-offline"}`}
            role="status"
            aria-live="polite"
          >
            <span className="login-status-dot" />
            {available ? "Conexión disponible" : "Servicio no disponible"}
            <ShieldCheck size={14} aria-hidden="true" />
          </div>
          {!available && (
            <p className="login-offline-note" id="login-offline">
              No podemos conectar en este momento. Comprueba la conexión para
              volver a intentar.
            </p>
          )}
          <form
            onSubmit={(e) => {
              e.preventDefault();
              if (available)
                void run("login", () => onLogin(email.trim(), password));
            }}
            aria-busy={busy === "login"}
          >
            <label htmlFor="login-email">Correo electrónico</label>
            <div className="login-input-wrap">
              <Mail size={18} aria-hidden="true" />
              <input
                id="login-email"
                name="email"
                autoComplete="username"
                type="email"
                inputMode="email"
                autoCapitalize="none"
                spellCheck={false}
                required
                maxLength={120}
                placeholder="nombre@nettplus.com"
                value={email}
                onChange={(e) => {
                  setEmail(e.target.value);
                  setError("");
                }}
                disabled={Boolean(busy)}
                aria-describedby={error ? "login-error" : undefined}
              />
            </div>
            <label htmlFor="login-password">Contraseña</label>
            <div className="login-input-wrap">
              <LockKeyhole size={18} aria-hidden="true" />
              <input
                id="login-password"
                name="password"
                autoComplete="current-password"
                type={show ? "text" : "password"}
                required
                maxLength={128}
                placeholder="Tu contraseña"
                value={password}
                onChange={(e) => {
                  setPassword(e.target.value);
                  setError("");
                }}
                onKeyUp={(e) => setCapsLock(e.getModifierState("CapsLock"))}
                onKeyDown={(e) => setCapsLock(e.getModifierState("CapsLock"))}
                onBlur={() => setCapsLock(false)}
                disabled={Boolean(busy)}
                aria-describedby={
                  [capsLock && "login-caps", error && "login-error"]
                    .filter(Boolean)
                    .join(" ") || undefined
                }
              />
              <button
                className="login-password-toggle"
                type="button"
                aria-label={show ? "Ocultar contraseña" : "Mostrar contraseña"}
                aria-pressed={show}
                onClick={() => setShow((s) => !s)}
                disabled={Boolean(busy)}
              >
                {show ? <EyeOff size={18} /> : <Eye size={18} />}
              </button>
            </div>
            {capsLock && (
              <p className="login-caps" id="login-caps" role="status">
                Bloq Mayús está activado.
              </p>
            )}
            {error && (
              <p className="login-error" id="login-error" role="alert">
                {error}
              </p>
            )}
            <button
              className="login-submit"
              type="submit"
              disabled={Boolean(busy) || !available}
              aria-describedby={!available ? "login-offline" : undefined}
            >
              {busy === "login" ? (
                <>
                  <span className="spinner" /> Validando acceso…
                </>
              ) : (
                <>
                  Iniciar sesión <ArrowRight size={18} />
                </>
              )}
            </button>
          </form>
          <div className="login-help">
            <span>¿Necesitas acceso?</span>
            <p>
              Solicita tu cuenta o ayuda con tu contraseña al administrador de
              Nettplus.
            </p>
          </div>
          <div className="login-card-bottom">
            <span>
              <ShieldCheck size={14} /> Acceso con cuenta autorizada
            </span>
            <button
              type="button"
              className="login-retry"
              disabled={Boolean(busy)}
              onClick={() => void run("connection", onCheck)}
              aria-label="Comprobar conexión con el servidor"
            >
              <RefreshCw
                size={13}
                className={busy === "connection" ? "is-checking" : ""}
              />
              {busy === "connection" ? "Comprobando…" : "Comprobar conexión"}
            </button>
          </div>
        </section>
      </main>
      <footer className="login-footer">
        <span>
          Nettplus <ChevronRight size={12} /> Gestión de asistencia
        </span>
        <span>Información que acompaña a tu equipo.</span>
      </footer>
    </div>
  );
}
