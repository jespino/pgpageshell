import { StrictMode } from "react";
import { createRoot } from "react-dom/client";
import "./index.css";
import { App } from "./components/App";
import { wailsBackend } from "./backends/wails";

createRoot(document.getElementById("root")!).render(
  <StrictMode>
    <App backend={wailsBackend} />
  </StrictMode>
);
