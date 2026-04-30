// Hand-written client for the Wails-bound Go services.
//
// Wails v3 normally code-generates these bindings via `wails3 generate`. To
// avoid a generation step in the dev loop and keep TypeScript types under our
// control, we maintain a typed wrapper around the underlying CallByID/
// CallByName JSON-RPC bridge that wails injects on window.
//
// The methods marshal arguments to JSON and resolve to the typed return value.

type Caller = (method: string, args: unknown[]) => Promise<unknown>;

declare global {
  interface Window {
    _skylauncher_call?: Caller;
    go?: Record<string, Record<string, Record<string, (...args: any[]) => Promise<any>>>>;
  }
}

function callerFor(servicePkg: string, serviceName: string) {
  return async function call<T>(method: string, ...args: unknown[]): Promise<T> {
    if (typeof window === "undefined") {
      throw new Error("wails bridge unavailable");
    }
    const svc = window.go?.[servicePkg]?.[serviceName];
    if (svc && typeof svc[method] === "function") {
      return (await svc[method](...args)) as T;
    }
    if (window._skylauncher_call) {
      const id = `${servicePkg}.${serviceName}.${method}`;
      return (await window._skylauncher_call(id, args)) as T;
    }
    throw new Error(`wails service ${servicePkg}.${serviceName}.${method} not bound`);
  };
}

// ---- Types mirrored from Go DTOs ----

export interface JavaConfig {
  memory: number;
  path?: string;
}

export interface UserProfile {
  id: string;
  name: string;
  accessToken?: string;
  refreshToken?: string;
  type: "microsoft" | "offline";
  expiresAt?: number;
}

export interface AppConfig {
  locale: string;
  java: JavaConfig;
  user: UserProfile;
  lastInstance?: string;
}

export interface HardwareInfo {
  os: string;
  arch: string;
  cpuCount: number;
  totalRamMB: number;
}

export interface Instance {
  id: string;
  name: string;
  mcVersion: string;
  loader?: string;
  loaderVersion?: string;
  versionId: string;
  javaPath?: string;
  heapMB?: number;
  iconUrl?: string;
  modpack?: string;
  createdAt: string;
  lastPlayed?: string;
}

export interface ModpackSummary {
  id: number;
  name: string;
  slug: string;
  summary: string;
  downloadCount: number;
  logo?: { url: string; thumbnailUrl: string };
  authors: { name: string }[];
  latestFiles: ModpackFile[];
}

export interface ModpackFile {
  id: number;
  modId: number;
  displayName: string;
  fileName: string;
  fileLength: number;
  gameVersions: string[];
  downloadUrl: string;
}

export interface SearchResponse {
  data: ModpackSummary[];
  pagination: { index: number; pageSize: number; resultCount: number; totalCount: number };
}

// ---- Service surfaces ----

const cfg = callerFor("config", "Service");
export const Config = {
  get: () => cfg<AppConfig>("Get"),
  setLocale: (locale: string) => cfg<void>("SetLocale", locale),
  setJava: (j: JavaConfig) => cfg<void>("SetJava", j),
  setLastInstance: (id: string) => cfg<void>("SetLastInstance", id),
};

const hw = callerFor("hardware", "Service");
export const Hardware = {
  get: () => hw<HardwareInfo>("Get"),
};

const authSvc = callerFor("auth", "Service");
export const Auth = {
  current: () => authSvc<UserProfile>("Current"),
  loginOffline: (nick: string) => authSvc<UserProfile>("LoginOffline", nick),
  loginMicrosoft: () => authSvc<UserProfile>("LoginMicrosoft"),
  logout: () => authSvc<void>("Logout"),
};

const cf = callerFor("curseforge", "Client");
export const CurseForge = {
  searchModpacks: (params: { search?: string; gameVersion?: string; index?: number; pageSize?: number; sortField?: number; sortOrder?: "asc" | "desc" }) =>
    cf<SearchResponse>("SearchModpacks", params),
  getMod: (id: number) => cf<ModpackSummary>("GetMod", id),
};

const launch = callerFor("launcher", "Service");
export const Launcher = {
  list: () => launch<Instance[]>("List"),
  running: () => launch<string[]>("Running"),
  createAndInstall: (req: {
    name: string;
    mcVersion: string;
    loader: "vanilla" | "fabric" | "forge" | "neoforge";
    loaderVersion?: string;
    modpackModId?: number;
    modpackFileId?: number;
    heapMB?: number;
    iconUrl?: string;
  }) => launch<Instance>("CreateAndInstall", req),
  launch: (id: string) => launch<void>("Launch", id),
  stop: (id: string) => launch<void>("Stop", id),
};
