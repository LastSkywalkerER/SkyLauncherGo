import { Button } from "primereact/button";
import { Dropdown } from "primereact/dropdown";
import { InputText } from "primereact/inputtext";
import { useEffect, useState } from "react";
import { useNavigate } from "react-router-dom";

import { CurseForge, Launcher, type ModpackSummary } from "../wails/bindings";

const loaderOptions = [
  { label: "Auto (from manifest)", value: "" },
  { label: "Forge", value: "forge" },
  { label: "Fabric", value: "fabric" },
  { label: "NeoForge", value: "neoforge" },
];

export function BrowsePage() {
  const [query, setQuery] = useState("");
  const [results, setResults] = useState<ModpackSummary[]>([]);
  const [loading, setLoading] = useState(false);
  const [installing, setInstalling] = useState<number | null>(null);
  const [error, setError] = useState<string | null>(null);
  const navigate = useNavigate();

  const search = async () => {
    setLoading(true);
    setError(null);
    try {
      const res = await CurseForge.searchModpacks({
        search: query, pageSize: 20, sortField: 2, sortOrder: "desc",
      });
      setResults(res.data ?? []);
    } catch (e) {
      setError(String(e));
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    void search();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  const install = async (mp: ModpackSummary, loader: string) => {
    const file = mp.latestFiles?.[0];
    if (!file) {
      setError("Modpack has no downloadable files");
      return;
    }
    setInstalling(mp.id);
    setError(null);
    try {
      await Launcher.createAndInstall({
        name: mp.name,
        mcVersion: file.gameVersions?.find((v) => /^\d+\.\d+/.test(v)) ?? "1.20.1",
        loader: (loader || "vanilla") as "vanilla" | "fabric" | "forge" | "neoforge",
        modpackModId: mp.id,
        modpackFileId: file.id,
        iconUrl: mp.logo?.thumbnailUrl,
      });
      navigate("/");
    } catch (e) {
      setError(String(e));
    } finally {
      setInstalling(null);
    }
  };

  return (
    <div className="space-y-4">
      <div className="flex items-center gap-2">
        <InputText
          value={query}
          onChange={(e) => setQuery(e.target.value)}
          placeholder="Search modpacks…"
          onKeyDown={(e) => e.key === "Enter" && search()}
          className="flex-1"
        />
        <Button label="Search" icon="pi pi-search" onClick={search} loading={loading} />
      </div>

      {error && <div className="rounded-md border border-red-700 bg-red-900/40 p-3 text-sm">{error}</div>}

      <ul className="grid grid-cols-1 gap-3 md:grid-cols-2 xl:grid-cols-3">
        {results.map((mp) => (
          <ModpackCard
            key={mp.id}
            modpack={mp}
            installing={installing === mp.id}
            onInstall={(loader) => install(mp, loader)}
          />
        ))}
      </ul>
    </div>
  );
}

function ModpackCard(props: {
  modpack: ModpackSummary;
  installing: boolean;
  onInstall: (loader: string) => void;
}) {
  const [loader, setLoader] = useState("");
  const mp = props.modpack;

  return (
    <li className="rounded-md border border-white/10 bg-surface p-4">
      <div className="flex items-start gap-3">
        {mp.logo?.thumbnailUrl && <img src={mp.logo.thumbnailUrl} className="h-12 w-12 rounded" alt="" />}
        <div className="flex-1">
          <div className="font-semibold">{mp.name}</div>
          <div className="text-xs text-white/40">
            {mp.authors?.[0]?.name} · {mp.downloadCount.toLocaleString()} downloads
          </div>
          <p className="mt-1 line-clamp-3 text-sm text-white/80">{mp.summary}</p>
        </div>
      </div>
      <div className="mt-3 flex items-center gap-2">
        <Dropdown
          value={loader}
          onChange={(e) => setLoader(e.value)}
          options={loaderOptions}
          placeholder="Loader"
          className="w-44"
        />
        <Button
          label="Install"
          icon="pi pi-download"
          loading={props.installing}
          onClick={() => props.onInstall(loader)}
        />
      </div>
    </li>
  );
}
