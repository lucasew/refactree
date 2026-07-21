import React from "react";
import { createRoot } from "react-dom/client";
import { RelayEnvironmentProvider } from "react-relay";
import { environment } from "./RelayEnvironment";
import { App } from "./App";

const root = document.getElementById("root");
if (root) {
  createRoot(root).render(
    <React.StrictMode>
      <RelayEnvironmentProvider environment={environment}>
        <App />
      </RelayEnvironmentProvider>
    </React.StrictMode>
  );
}
