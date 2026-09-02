import { Layout, Menu, Typography } from "@arco-design/web-react";
import { Navigate, Route, Routes, useLocation, useNavigate } from "react-router-dom";
import SCM from "./pages/SCM";
import JobList from "./pages/scm/JobList";
import JobPage from "./pages/scm/JobPage";
import BuildPage from "./pages/scm/BuildPage";
import BAM from "./pages/BAM";
import AGW from "./pages/AGW";
import Deploy from "./pages/Deploy";
import AppList from "./pages/deploy/AppList";
import AppPage from "./pages/deploy/AppPage";
import RunPage from "./pages/deploy/RunPage";
import DB from "./pages/DB";

const TABS = [
  ["scm", "SCM 编译"],
  ["bam", "BAM IDL"],
  ["agw", "AGW 网关"],
  ["deploy", "部署"],
  ["db", "MySQL"],
] as const;

export default function App() {
  const loc = useLocation();
  const navigate = useNavigate();
  const tab = loc.pathname.split("/")[1] || "scm";
  return (
    <Layout className="h-screen">
      <Layout.Sider theme="dark" width={220}>
        <div className="px-4 py-4 text-white">
          <Typography.Title heading={5} className="!mb-0 !text-white">
            minikitex
          </Typography.Title>
          <Typography.Text className="text-xs text-white/55">SCM · BAM · AGW · 部署</Typography.Text>
        </div>
        <Menu theme="dark" selectedKeys={[tab]} onClickMenuItem={(key) => navigate("/" + key)}>
          {TABS.map(([id, label]) => (
            <Menu.Item key={id}>{label}</Menu.Item>
          ))}
        </Menu>
      </Layout.Sider>
      <Layout.Content className="overflow-auto bg-slate-50 p-6">
        <Routes>
          <Route path="/" element={<Navigate to="/scm" replace />} />
          <Route path="/scm" element={<SCM />}>
            <Route index element={<JobList />} />
            <Route path=":name" element={<JobPage />} />
            <Route path=":name/builds/:id" element={<BuildPage />} />
          </Route>
          <Route path="/bam" element={<BAM />} />
          <Route path="/agw" element={<AGW />} />
          <Route path="/deploy" element={<Deploy />}>
            <Route index element={<AppList />} />
            <Route path=":name" element={<AppPage />} />
            <Route path=":name/runs/:id" element={<RunPage />} />
          </Route>
          <Route path="/db" element={<DB />} />
        </Routes>
      </Layout.Content>
    </Layout>
  );
}
