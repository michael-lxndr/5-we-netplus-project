import type { AppData, DemoUser, Rules } from "../domain/types";
import type { ColumnMap } from "./excel";
import type { DocumentKind } from "./sourceDocuments";

export class ApiError extends Error {
  constructor(
    readonly status: number,
    message: string,
  ) {
    super(message);
  }
}
export async function api<T>(
  path: string,
  method = "GET",
  body?: unknown,
): Promise<T> {
  const response = await fetch(`/api${path}`, {
    method,
    signal:
      path.startsWith("/auth/") || path === "/session"
        ? AbortSignal.timeout(15000)
        : undefined,
    credentials: "same-origin",
    headers: body ? { "Content-Type": "application/json" } : undefined,
    body: body ? JSON.stringify(body) : undefined,
  }).catch(() => {
    throw new ApiError(
      503,
      "No se pudo conectar con el servidor. Comprueba la conexión y vuelve a intentar.",
    );
  });
  if (!response.headers.get("content-type")?.includes("application/json"))
    throw new ApiError(503, "El servidor local no está disponible.");
  const result = await response.json();
  if (!response.ok)
    throw new ApiError(
      response.status,
      result.error || "No se pudo completar la solicitud.",
    );
  return result as T;
}
export async function serverAvailable() {
  try {
    const response = await fetch("/api/health", {
      signal: AbortSignal.timeout(3500),
    });
    const result = response.headers
      .get("content-type")
      ?.includes("application/json")
      ? await response.json()
      : null;
    return Boolean(
      response.ok &&
      result?.service === "nettplus-api" &&
      result?.database === "ok",
    );
  } catch {
    return false;
  }
}
export interface Session {
  user: DemoUser;
  data: AppData;
}
export type ApiCommand =
  | { type: "rules"; rules: Rules }
  | { type: "reset"; mode: "seed" | "clear" }
  | {
      type: "user";
      user: Omit<DemoUser, "id"> & { id?: string };
      password?: string;
    };
export async function commandApi(
  command: ApiCommand,
  revision: number | undefined,
) {
  if (command.type === "rules")
    return api<AppData>("/rules", "PUT", { rules: command.rules, revision });
  if (command.type === "reset")
    return api<AppData>("/reset", "POST", { mode: command.mode, revision });
  return api<AppData>("/users", "POST", {
    user: command.user,
    password: command.password,
    revision,
  });
}
export interface WorkbookImport {
  file: File;
  sheetIndex: number;
  header: number;
  map: ColumnMap;
  kind: DocumentKind;
  month: string;
}
export async function uploadWorkbook(
  input: WorkbookImport,
  revision: number | undefined,
) {
  const bytes = new Uint8Array(await input.file.arrayBuffer());
  let binary = "";
  for (let i = 0; i < bytes.length; i += 32768)
    binary += String.fromCharCode(...bytes.subarray(i, i + 32768));
  return api<AppData>("/imports", "POST", {
    filename: input.file.name,
    base64: btoa(binary),
    sheetIndex: input.sheetIndex,
    header: input.header,
    map: input.map,
    kind: input.kind,
    month: input.month,
    revision,
  });
}
