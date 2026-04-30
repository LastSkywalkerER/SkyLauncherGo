// Wails runtime is loaded by the host (main.go's bundled asset server)
// at /wails/runtime.js. We declare the surface we use so TypeScript stays
// happy without pulling the full @wailsio/runtime types.
//
// Wails v3 generates per-service bindings under window.go.<ServicePkg>.<Service>
// — those are typed in ./bindings.ts.

declare global {
  interface Window {
    wails: {
      Events: {
        On<T = unknown>(name: string, handler: (data: T) => void): () => void;
        Off(name: string): void;
      };
    };
    runtime?: {
      EventsOn?: <T = unknown>(name: string, handler: (data: T) => void) => () => void;
    };
  }
}

export type Unsubscribe = () => void;

export function onEvent<T = unknown>(name: string, handler: (data: T) => void): Unsubscribe {
  if (typeof window === "undefined") return () => {};
  if (window.wails?.Events?.On) {
    return window.wails.Events.On<T>(name, handler);
  }
  if (window.runtime?.EventsOn) {
    return window.runtime.EventsOn<T>(name, handler);
  }
  return () => {};
}
