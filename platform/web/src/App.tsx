import { Button } from "@arco-design/web-react";
import { Navigate, Route, Routes, useLocation, useNavigate } from "react-router-dom";
import Login from "./pages/Login";
import SCM from "./pages/SCM";
import JobList from "./pages/scm/JobList";
import JobPage from "./pages/scm/JobPage";
import BuildPage from "./pages/scm/BuildPage";
import BAM from "./pages/BAM";
import AGW from "./pages/AGW";
import TLB from "./pages/TLB";
import SiteList from "./pages/tlb/SiteList";
import SitePage from "./pages/tlb/SitePage";
import Deploy from "./pages/Deploy";
import AppList from "./pages/deploy/AppList";
import AppPage from "./pages/deploy/AppPage";
import RunPage from "./pages/deploy/RunPage";
import DB from "./pages/DB";

const TABS = [
  ["scm", "01", "SCM"],
  ["bam", "02", "BAM"],
  ["agw", "03", "AGW"],
  ["tlb", "04", "TLB"],
  ["deploy", "05", "Deploy"],
  ["db", "06", "MySQL"],
] as const;

export default function App() {
  const loc = useLocation();
  const navigate = useNavigate();
  const tab = loc.pathname.split("/")[1] || "scm";
  const token = localStorage.getItem("token");
  if (loc.pathname === "/login") {
    if (token) {
      return <Navigate to="/scm" replace />;
    }
    return <Login />;
  }
  if (!token) {
    return <Navigate to="/login" replace />;
  }
  const user = localStorage.getItem("user") || "";
  return (
    <div className="shell">
      <aside className="rail">
        <div className="brand">
          <div className="mark" />
          <div>
            <h1>minikitex</h1>
            <p>control plane</p>
          </div>
        </div>
        <nav className="nav">
          {TABS.map(([id, idx, label]) => (
            <button key={id} className={tab === id ? "on" : ""} onClick={() => navigate("/" + id)}>
              <span className="idx">{idx}</span>
              {label}
            </button>
          ))}
        </nav>
        <div className="rail-foot">
          <div className="who">{user}</div>
          <Button
            size="mini"
            long
            onClick={() => {
              localStorage.removeItem("token");
              localStorage.removeItem("user");
              navigate("/login", { replace: true });
            }}
          >
            退出
          </Button>
        </div>
      </aside>
      <main className="stage">
        <Routes>
          <Route path="/" element={<Navigate to="/scm" replace />} />
          <Route path="/scm" element={<SCM />}>
            <Route index element={<JobList />} />
            <Route path=":name" element={<JobPage />} />
            <Route path=":name/builds/:id" element={<BuildPage />} />
          </Route>
          <Route path="/bam" element={<BAM />} />
          <Route path="/agw" element={<AGW />} />
          <Route path="/tlb" element={<TLB />}>
            <Route index element={<SiteList />} />
            <Route path=":name" element={<SitePage />} />
          </Route>
          <Route path="/deploy" element={<Deploy />}>
            <Route index element={<AppList />} />
            <Route path=":name" element={<AppPage />} />
            <Route path=":name/runs/:id" element={<RunPage />} />
          </Route>
          <Route path="/db" element={<DB />} />
        </Routes>
      </main>
    </div>
  );
}
