import { Button } from "primereact/button";
import { InputNumber } from "primereact/inputnumber";
import { useEffect, useState } from "react";

import { Config, Hardware, type AppConfig, type HardwareInfo } from "../wails/bindings";

export function SettingsPage() {
  const [cfg, setCfg] = useState<AppConfig | null>(null);
  const [hw, setHw] = useState<HardwareInfo | null>(null);
  const [memory, setMemory] = useState(4096);
  const [savedAt, setSavedAt] = useState<number | null>(null);

  useEffect(() => {
    Config.get().then((c) => {
      setCfg(c);
      setMemory(c.java.memory || 4096);
    });
    Hardware.get().then(setHw);
  }, []);

  const save = async () => {
    await Config.setJava({ memory, path: cfg?.java.path });
    setSavedAt(Date.now());
  };

  return (
    <div className="mx-auto max-w-xl space-y-6 rounded-md border border-white/10 bg-surface p-6">
      <h1 className="text-xl font-semibold">Settings</h1>

      {hw && (
        <section className="rounded-md border border-white/10 p-4 text-sm">
          <h2 className="mb-2 font-semibold">Host</h2>
          <ul className="space-y-1 text-white/80">
            <li>OS: {hw.os} / {hw.arch}</li>
            <li>CPU cores: {hw.cpuCount}</li>
            <li>RAM: {(hw.totalRamMB / 1024).toFixed(1)} GB</li>
          </ul>
        </section>
      )}

      <section className="space-y-2">
        <label className="text-sm">JVM heap (MB)</label>
        <div className="flex items-center gap-2">
          <InputNumber
            value={memory}
            onValueChange={(e) => setMemory(e.value ?? 0)}
            min={1024}
            max={hw ? hw.totalRamMB - 1024 : 32768}
            step={512}
            showButtons
            className="w-40"
          />
          <Button label="Save" icon="pi pi-check" onClick={save} />
          {savedAt && (
            <span className="text-xs text-white/40">Saved {new Date(savedAt).toLocaleTimeString()}</span>
          )}
        </div>
        <p className="text-xs text-white/40">
          Recommended: 4 GB for vanilla, 6+ GB for heavy modpacks.
        </p>
      </section>
    </div>
  );
}
