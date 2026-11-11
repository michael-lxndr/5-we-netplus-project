import { StrictMode } from "react";
import { createRoot } from "react-dom/client";
import { Login } from "./Login";

const stylesheet = document.createElement("link");

stylesheet.rel = "stylesheet";
stylesheet.href = new URL("./styles.css", import.meta.url).href;
document.head.appendChild(stylesheet);

createRoot(document.getElementById("root")!).render(
  <StrictMode>
    <Login />
  </StrictMode>,
);
