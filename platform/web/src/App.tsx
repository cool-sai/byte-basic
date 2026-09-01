import { useState } from "react";
import { Layout, Menu, Typography } from "@arco-design/web-react";
import SCM from "./pages/SCM";
import BAM from "./pages/BAM";
import AGW from "./pages/AGW";
import Deploy from "./pages/Deploy";
import DB from "./pages/DB";

const TABS = [
  ["scm", "SCM 编译"],
  ["bam", "BAM IDL"],
  ["agw", "AGW 网关"],
  ["deploy", "部署"],
  ["db", "MySQL"],
] as const;

type Tab = (typeof TABS)[number][0];

export default function App() {
  const [tab, setTab] = useState<Tab>("scm");
  return (
    <Layout className="h-screen">
      <Layout.Sider theme="dark" width={220}>
        <div className="px-4 py-4 text-white">
          <Typography.Title heading={5} className="!mb-0 !text-white">
            minikitex
          </Typography.Title>
          <Typography.Text className="text-xs text-white/55">SCM · BAM · AGW · 部署</Typography.Text>
        </div>
        <Menu
          theme="dark"
          selectedKeys={[tab]}
          onClickMenuItem={(key) => setTab(key as Tab)}
        >
          {TABS.map(([id, label]) => (
            <Menu.Item key={id}>{label}</Menu.Item>
          ))}
        </Menu>
      </Layout.Sider>
      <Layout.Content className="overflow-auto bg-slate-50 p-6">
        {tab === "scm" && <SCM />}
        {tab === "bam" && <BAM />}
        {tab === "agw" && <AGW />}
        {tab === "deploy" && <Deploy />}
        {tab === "db" && <DB />}
      </Layout.Content>
    </Layout>
  );
}
