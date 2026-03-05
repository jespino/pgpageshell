import { StrictMode } from "react";
import { createRoot } from "react-dom/client";
import "@shared/index.css";
import { App } from "@shared/components/App";
import { createStaticBackend } from "@shared/backends/static";

const backend = createStaticBackend(import.meta.env.BASE_URL + "data.json");

createRoot(document.getElementById("root")!).render(
  <StrictMode>
    <App backend={backend} />
  </StrictMode>
);
