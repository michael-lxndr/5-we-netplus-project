import { useEffect, useState } from "react";
import { api, ApiError, type User } from "./api";
import { Login } from "./Login";
import { Shell } from "./Shell";

type State =
  | { status: "loading" }
  | { status: "anonymous"; setupRequired: boolean }
  | { status: "authenticated"; user: User };

export function App() {
  const [state, setState] = useState<State>({ status: "loading" });

  useEffect(() => {
    let cancelled = false;

    async function bootstrap() {
      try {
        const user = await api.me();
        if (!cancelled) setState({ status: "authenticated", user });
        return;
      } catch (e) {
        if (!(e instanceof ApiError && e.status === 401)) {
          // Servidor caído: se muestra el login igualmente, que informa el estado.
        }
      }
      const setupRequired = await api.setupRequired().catch(() => false);
      if (!cancelled) setState({ status: "anonymous", setupRequired });
    }

    bootstrap();
    return () => {
      cancelled = true;
    };
  }, []);

  if (state.status === "loading") return <div className="splash" aria-busy="true" />;

  if (state.status === "authenticated") {
    return (
      <Shell
        user={state.user}
        onLogout={async () => {
          await api.logout().catch(() => undefined);
          setState({ status: "anonymous", setupRequired: false });
        }}
      />
    );
  }

  return (
    <Login
      setupRequired={state.setupRequired}
      onAuthenticated={(user) => setState({ status: "authenticated", user })}
    />
  );
}
