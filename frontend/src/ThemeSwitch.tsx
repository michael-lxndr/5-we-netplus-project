import { useEffect, useState } from "react";

type Mode = "auto" | "light" | "dark";

const STORAGE_KEY = "netplus-theme";

const OPTIONS: { value: Mode; label: string; icon: string }[] = [
  { value: "auto", label: "Auto", icon: "◐" },
  { value: "light", label: "Día", icon: "☀" },
  { value: "dark", label: "Noche", icon: "☾" },
];

function readStored(): Mode {
  try {
    const v = localStorage.getItem(STORAGE_KEY);
    if (v === "light" || v === "dark" || v === "auto") return v;
  } catch {
    /* almacenamiento no disponible */
  }
  return "auto";
}

function apply(mode: Mode) {
  const root = document.documentElement;
  if (mode === "auto") root.removeAttribute("data-theme");
  else root.setAttribute("data-theme", mode);
}

export function ThemeSwitch({ inline = false }: { inline?: boolean }) {
  const [mode, setMode] = useState<Mode>(readStored);

  useEffect(() => {
    apply(mode);
    try {
      localStorage.setItem(STORAGE_KEY, mode);
    } catch {
      /* ignorar */
    }
  }, [mode]);

  return (
    <div className={inline ? "theme-switch theme-switch--inline" : "theme-switch"} role="group" aria-label="Tema de color">
      {OPTIONS.map((o) => (
        <button
          key={o.value}
          type="button"
          aria-pressed={mode === o.value}
          title={o.label}
          onClick={() => setMode(o.value)}
        >
          <span aria-hidden="true">{o.icon}</span>
          <span>{o.label}</span>
        </button>
      ))}
    </div>
  );
}
