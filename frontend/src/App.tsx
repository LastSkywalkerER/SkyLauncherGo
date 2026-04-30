import { Route, Routes } from "react-router-dom";

import { Sidebar } from "./components/Sidebar";
import { TopBar } from "./components/TopBar";
import { BrowsePage } from "./pages/BrowsePage";
import { InstancesPage } from "./pages/InstancesPage";
import { LoginPage } from "./pages/LoginPage";
import { SettingsPage } from "./pages/SettingsPage";

export function App() {
  return (
    <div className="flex h-screen bg-bg text-white">
      <Sidebar />
      <div className="flex flex-1 flex-col overflow-hidden">
        <TopBar />
        <main className="flex-1 overflow-auto p-6">
          <Routes>
            <Route path="/" element={<InstancesPage />} />
            <Route path="/browse" element={<BrowsePage />} />
            <Route path="/settings" element={<SettingsPage />} />
            <Route path="/login" element={<LoginPage />} />
          </Routes>
        </main>
      </div>
    </div>
  );
}
