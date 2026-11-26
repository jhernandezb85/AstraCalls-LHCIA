import { create } from "zustand";

// Qué muestra la columna principal: la cuenta elegida o la página de Configuración.
export type MainView = "session" | "settings";

export const useView = create<{ view: MainView }>(() => ({ view: "session" }));

export const showSettings = (): void => useView.setState({ view: "settings" });
export const showSession = (): void => useView.setState({ view: "session" });
