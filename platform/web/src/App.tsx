import { lazy, Suspense } from "react";
import { Button } from "@arco-design/web-react";
import { Navigate, Route, Routes, useLocation, useNavigate } from "react-router-dom";

const Login = lazy(() => import("./pages/Login"));
const SCM = lazy(() => import("./pages/SCM"));
const JobList = lazy(() => import("./pages/scm/JobList"));
const JobPage = lazy(() => import("./pages/scm/JobPage"));
const BuildPage = lazy(() => import("./pages/scm/BuildPage"));
const BAM = lazy(() => import("./pages/BAM"));
const ModuleList = lazy(() => import("./pages/bam/ModuleList"));
const ModulePage = lazy(() => import("./pages/bam/ModulePage"));
const AGW = lazy(() => import("./pages/AGW"));
const TLB = lazy(() => import("./pages/TLB"));
const SiteList = lazy(() => import("./pages/tlb/SiteList"));
const SitePage = lazy(() => import("./pages/tlb/SitePage"));
const Deploy = lazy(() => import("./pages/Deploy"));
const AppList = lazy(() => import("./pages/deploy/AppList"));
const AppPage = lazy(() => import("./pages/deploy/AppPage"));
const RunPage = lazy(() => import("./pages/deploy/RunPage"));
const DB = lazy(() => import("./pages/DB"));

const fallback = <p className="meta">加载中</p>;

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
    return (
      <Suspense fallback={fallback}>
        <Login />
      </Suspense>
    );
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
        <Suspense fallback={fallback}>
          <Routes>
            <Route path="/" element={<Navigate to="/scm" replace />} />
            <Route path="/scm" element={<SCM />}>
              <Route index element={<JobList />} />
              <Route path=":name" element={<JobPage />} />
              <Route path=":name/builds/:id" element={<BuildPage />} />
            </Route>
            <Route path="/bam" element={<BAM />}>
              <Route index element={<ModuleList />} />
              <Route path=":name" element={<ModulePage />} />
            </Route>
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
        </Suspense>
      </main>
    </div>
  );
}
