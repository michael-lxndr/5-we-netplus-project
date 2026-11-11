import { useState, type FormEvent } from "react";
import { ThemeSwitch } from "./ThemeSwitch";

export function Login() {
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [showPassword, setShowPassword] = useState(false);
  const [remember, setRemember] = useState(true);
  const [error, setError] = useState("");

  function handleSubmit(e: FormEvent) {
    e.preventDefault();
    if (!email.trim() || !password) {
      setError("Ingresa tu correo y contraseña para continuar.");
      return;
    }
    setError("");
    // TODO: conectar con la autenticación real cuando se implemente el backend.
  }

  return (
    <div className="stage">
      <ThemeSwitch />
      <div className="islands">
        {/* Isla de marca */}
        <section className="island island--brand">
          <div className="logo">
            <span className="logo__mark" aria-hidden="true">N</span>
            <span className="logo__name">Netplus</span>
          </div>
          <div className="brand__copy">
            <h1>Tus datos, en un solo panorama.</h1>
            <p>Carga tus archivos Excel y transfórmalos en dashboards claros para decidir con rapidez.</p>
          </div>
        </section>

        {/* Isla del formulario */}
        <section className="island island--form" aria-labelledby="login-title">
          <header>
            <p className="eyebrow">Bienvenido</p>
            <h2 id="login-title">Iniciar sesión</h2>
          </header>

          <form onSubmit={handleSubmit} noValidate>
            <label className="field">
              <span>Correo electrónico</span>
              <input
                type="email"
                autoComplete="email"
                placeholder="nombre@empresa.com"
                value={email}
                onChange={(e) => setEmail(e.target.value)}
              />
            </label>

            <label className="field">
              <span>Contraseña</span>
              <div className="field__password">
                <input
                  type={showPassword ? "text" : "password"}
                  autoComplete="current-password"
                  placeholder="••••••••"
                  value={password}
                  onChange={(e) => setPassword(e.target.value)}
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

            <div className="row">
              <label className="check">
                <input
                  type="checkbox"
                  checked={remember}
                  onChange={(e) => setRemember(e.target.checked)}
                />
                <span>Recordarme</span>
              </label>
              <a className="link" href="#">¿Olvidaste tu contraseña?</a>
            </div>

            {error && <p className="error" role="alert">{error}</p>}

            <button type="submit" className="btn">Ingresar</button>
          </form>
        </section>

        {/* Islas pequeñas */}
        <aside className="island island--stat">
          <span className="stat__icon" aria-hidden="true">▤</span>
          <div>
            <strong>Excel → Dashboard</strong>
            <small>Carga y visualiza en segundos</small>
          </div>
        </aside>

        <aside className="island island--status">
          <span className="dot" aria-hidden="true" />
          <span>Sistema operativo</span>
        </aside>
      </div>

      <p className="footer">© {new Date().getFullYear()} Netplus</p>
    </div>
  );
}
