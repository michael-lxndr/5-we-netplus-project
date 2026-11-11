import type { User } from "./api";
import { ThemeSwitch } from "./ThemeSwitch";
import icon from "../media/cropped-cropped-1-2-1-192x192.png";

type Props = {
  user: User;
  onLogout: () => void;
};

const MENU = ["Resumen", "Colaboradores", "Importaciones", "Reportes"];

// Estructura base según el prototipo (menú lateral + barra superior).
// El contenido real de cada sección se construye en las siguientes etapas.
export function Shell({ user, onLogout }: Props) {
  const initials = user.username.slice(0, 2).toUpperCase();
  const roleLabel = user.role === "admin" ? "Administración" : "Consulta";

  return (
    <div className="shell">
      <aside className="sidebar">
        <div className="logo">
          <img className="logo__icon" src={icon} alt="" width={40} height={40} />
          <div className="logo__text">
            <strong>NETPLUS</strong>
            <small>ASISTENCIA</small>
          </div>
        </div>

        <nav aria-label="Menú principal">
          <p className="sidebar__label">MENÚ PRINCIPAL</p>
          <ul>
            {MENU.map((item, i) => (
              <li key={item}>
                <a className={i === 0 ? "navitem navitem--active" : "navitem"} aria-current={i === 0 ? "page" : undefined}>
                  {item}
                </a>
              </li>
            ))}
          </ul>
        </nav>

        <div className="sidebar__user">
          <span className="avatar avatar--dark">{initials}</span>
          <div>
            <strong>{user.username}</strong>
            <small>{roleLabel}</small>
          </div>
        </div>
      </aside>

      <div className="content">
        <header className="topbar">
          <p className="crumbs">
            NetPlus <span>/</span> <strong>Resumen de asistencia</strong>
          </p>
          <div className="topbar__actions">
            <ThemeSwitch inline />
            <button type="button" className="btn btn--ghost" onClick={onLogout}>
              Cerrar sesión
            </button>
            <span className="avatar" title={user.username}>{initials}</span>
          </div>
        </header>

        <main>
          <h1 className="page__title">Resumen de asistencia</h1>
          <p className="page__subtitle">Hola, {user.username}. Tu sesión está activa.</p>

          <section className="card empty">
            <h2>Aún no hay datos para mostrar</h2>
            <p>Cuando se importen archivos de Z-BioTime, aquí verás los indicadores del periodo.</p>
          </section>
        </main>
      </div>
    </div>
  );
}
