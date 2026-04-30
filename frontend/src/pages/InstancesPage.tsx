import { Button } from "primereact/button";
import { useEffect, useState } from "react";

import { onEvent } from "../wails/runtime";
import { Launcher, type Instance } from "../wails/bindings";

interface ProgressEvent {
  id: string;
  stage: string;
  message: string;
  fraction: number;
  done?: boolean;
  error?: string;
}

export function InstancesPage() {
  const [items, setItems] = useState<Instance[]>([]);
  const [running, setRunning] = useState<string[]>([]);
  const [progress, setProgress] = useState<ProgressEvent | null>(null);
  const [error, setError] = useState<string | null>(null);

  const reload = () => {
    Launcher.list().then(setItems).catch((e) => setError(String(e)));
    Launcher.running().then(setRunning).catch(() => setRunning([]));
  };

  useEffect(() => {
    reload();
    return onEvent<ProgressEvent>("process-progress", setProgress);
  }, []);

  const handleLaunch = async (id: string) => {
    setError(null);
    try {
      await Launcher.launch(id);
      setRunning((prev) => [...prev, id]);
    } catch (e) {
      setError(String(e));
    }
  };

  const handleStop = async (id: string) => {
    try {
      await Launcher.stop(id);
      setRunning((prev) => prev.filter((x) => x !== id));
    } catch (e) {
      setError(String(e));
    }
  };

  const handleDelete = async (id: string, name: string) => {
    if (!window.confirm(`Delete "${name}"? Your saves will be backed up.`)) return;
    try {
      await Launcher.delete(id, true);
      reload();
    } catch (e) {
      setError(String(e));
    }
  };

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <h1 className="text-2xl font-semibold">Instances</h1>
        <Button label="Refresh" icon="pi pi-refresh" outlined onClick={reload} />
      </div>

      {progress && !progress.done && (
        <div className="rounded-md border border-white/10 bg-surface p-3 text-sm">
          <div className="mb-1 flex justify-between">
            <span>{progress.stage}: {progress.message}</span>
            <span>{Math.round(progress.fraction * 100)}%</span>
          </div>
          <div className="h-1 overflow-hidden rounded bg-white/10">
            <div className="h-full bg-accent" style={{ width: `${Math.round(progress.fraction * 100)}%` }} />
          </div>
        </div>
      )}

      {error && <div className="rounded-md border border-red-700 bg-red-900/40 p-3 text-sm">{error}</div>}

      {items.length === 0 ? (
        <div className="rounded-md border border-white/10 bg-surface p-6 text-center text-white/60">
          No instances yet — pick a modpack from the Browse tab to get started.
        </div>
      ) : (
        <ul className="grid grid-cols-1 gap-3 md:grid-cols-2 xl:grid-cols-3">
          {items.map((it) => {
            const live = running.includes(it.id);
            return (
              <li key={it.id} className="rounded-md border border-white/10 bg-surface p-4">
                <div className="flex items-start gap-3">
                  {it.iconUrl && <img src={it.iconUrl} className="h-12 w-12 rounded" alt="" />}
                  <div className="flex-1">
                    <div className="font-semibold">{it.name}</div>
                    <div className="text-xs text-white/60">
                      {it.mcVersion}
                      {it.loader && it.loader !== "vanilla" ? ` · ${it.loader} ${it.loaderVersion ?? ""}` : ""}
                    </div>
                    {it.modpack && <div className="mt-1 text-xs text-white/40">{it.modpack}</div>}
                  </div>
                </div>
                <div className="mt-3 flex gap-2">
                  {live ? (
                    <Button label="Stop" icon="pi pi-stop" severity="danger" onClick={() => handleStop(it.id)} />
                  ) : (
                    <Button label="Play" icon="pi pi-play" onClick={() => handleLaunch(it.id)} />
                  )}
                  <Button
                    icon="pi pi-trash"
                    severity="secondary"
                    outlined
                    disabled={live}
                    onClick={() => handleDelete(it.id, it.name)}
                    aria-label="Delete"
                  />
                </div>
              </li>
            );
          })}
        </ul>
      )}
    </div>
  );
}
