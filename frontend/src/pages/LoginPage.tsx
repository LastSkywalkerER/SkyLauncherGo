import { Button } from "primereact/button";
import { InputText } from "primereact/inputtext";
import { useEffect, useState } from "react";

import { Auth, type UserProfile } from "../wails/bindings";

export function LoginPage() {
  const [profile, setProfile] = useState<UserProfile | null>(null);
  const [nick, setNick] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);

  useEffect(() => {
    Auth.current().then((p) => (p?.name ? setProfile(p) : setProfile(null)));
  }, []);

  const offline = async () => {
    setError(null);
    setBusy(true);
    try {
      const p = await Auth.loginOffline(nick);
      setProfile(p);
    } catch (e) {
      setError(String(e));
    } finally {
      setBusy(false);
    }
  };

  const microsoft = async () => {
    setError(null);
    setBusy(true);
    try {
      const p = await Auth.loginMicrosoft();
      setProfile(p);
    } catch (e) {
      setError(String(e));
    } finally {
      setBusy(false);
    }
  };

  const logout = async () => {
    await Auth.logout();
    setProfile(null);
  };

  return (
    <div className="mx-auto max-w-md space-y-6 rounded-md border border-white/10 bg-surface p-6">
      <h1 className="text-xl font-semibold">Account</h1>

      {profile ? (
        <div className="space-y-3">
          <p>
            Signed in as <span className="font-semibold">{profile.name}</span>{" "}
            <span className="text-white/40">({profile.type})</span>
          </p>
          <p className="break-all text-xs text-white/40">UUID: {profile.id}</p>
          <Button label="Sign out" icon="pi pi-sign-out" severity="secondary" onClick={logout} />
        </div>
      ) : (
        <>
          <div className="space-y-2">
            <label className="text-sm">Offline username</label>
            <div className="flex gap-2">
              <InputText
                value={nick}
                onChange={(e) => setNick(e.target.value)}
                placeholder="Steve"
                className="flex-1"
              />
              <Button label="Use offline" icon="pi pi-user" loading={busy} onClick={offline} />
            </div>
            <p className="text-xs text-white/40">3-16 chars, letters/digits/underscore.</p>
          </div>

          <div className="border-t border-white/10 pt-4">
            <Button
              label="Sign in with Microsoft"
              icon="pi pi-microsoft"
              loading={busy}
              onClick={microsoft}
            />
            <p className="mt-2 text-xs text-white/40">
              Opens your default browser. The launcher waits for the redirect to localhost.
            </p>
          </div>
        </>
      )}

      {error && <div className="rounded-md border border-red-700 bg-red-900/40 p-3 text-sm">{error}</div>}
    </div>
  );
}
