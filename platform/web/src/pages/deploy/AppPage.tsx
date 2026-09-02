import { useState } from "react";
import { Button, Card, Message, Select, Table, Tag, Typography } from "@arco-design/web-react";
import { useRequest } from "ahooks";
import { Link, useLocation, useNavigate, useParams } from "react-router-dom";
import { api, errMsg, type Build, type Container, type DeployRecord } from "../../api";
import LabelIcon from "../scm/LabelIcon";
import LogBox from "../scm/LogBox";
import Crumbs from "./Crumbs";

export default function AppPage() {
  const { name = "" } = useParams();
  const loc = useLocation();
  const navigate = useNavigate();
  const [version, setVersion] = useState("");
  const [liveLog, setLiveLog] = useState("");

  const { data, loading, error, refresh } = useRequest(
    async () => {
      const app = await api.deployApp(name);
      const builds = (await api.builds(app.scmName)) || [];
      const deploys = (await api.deploys(name)) || [];
      const runtime = (await api.runtime()) || [];
      return { app, builds, deploys, runtime };
    },
    {
      refreshDeps: [name, loc.key],
      onSuccess: (d) => {
        const okBuilds = d.builds.filter((x) => x.status === "ok");
        setVersion((v) => (okBuilds.find((x) => x.version === v) ? v : okBuilds[0]?.version || ""));
      },
    },
  );
  const app = data?.app;
  const okBuilds = (data?.builds || []).filter((b) => b.status === "ok");
  const deploys = data?.deploys || [];
  const compose = app?.compose || [];
  const runtime = (data?.runtime || []).filter((c) => compose.includes(c.Service || ""));
  const picked = okBuilds.find((b) => b.version === version);

  const { run: deploy, loading: busy } = useRequest(
    async (ver: string) => {
      setLiveLog("");
      await api.deploy(name, ver, (text) => setLiveLog((cur) => cur + text));
    },
    {
      manual: true,
      onSuccess: (_d, [ver]) => {
        Message.success("已部署 " + name + " @ " + ver);
        void refresh();
      },
      onError: (e) => Message.error(errMsg(e)),
    },
  );

  return (
    <div className="flex w-full flex-col gap-6">
      <Crumbs appName={name} />
      <div className="flex items-start justify-between gap-3">
        <div>
          <Typography.Title heading={4} className="!mb-1 inline-flex items-center gap-2">
            <LabelIcon label={app?.label} />
            {app?.name || name}
          </Typography.Title>
          <div className="text-xs text-slate-500">
            SCM {app?.scmName ? <Link to={"/scm/" + app.scmName}>{app.scmName}</Link> : "—"}
            {app?.label ? " · " + app.label : ""}
          </div>
          <div className="text-xs text-slate-500">Compose {compose.join(", ")}</div>
          {app?.label === "node" ? (
            <div className="text-xs text-slate-500">静态站 :80。发完后 TLB 把路径转到 {(compose[0] || name) + ":80"}。</div>
          ) : null}
        </div>
      </div>
      {error ? <Typography.Text type="error">{errMsg(error)}</Typography.Text> : null}

      <div className="flex items-center gap-2">
        <Select
          value={version}
          onChange={setVersion}
          style={{ width: 280 }}
          loading={loading}
          placeholder={okBuilds.length ? "选择 SCM 产物" : "先去 SCM 编译"}
        >
          {okBuilds.map((b: Build) => (
            <Select.Option key={b.version} value={b.version}>
              {b.version}
            </Select.Option>
          ))}
        </Select>
        <Button type="primary" loading={busy} disabled={!version || busy} onClick={() => deploy(version)}>
          打镜像并启动
        </Button>
      </div>
      {picked ? (
        <Typography.Text type="secondary" className="font-mono text-xs">
          {picked.binPath}
        </Typography.Text>
      ) : null}

      {busy || liveLog ? (
        <Card title="本次部署">
          <LogBox text={liveLog} live />
        </Card>
      ) : null}

      <Typography.Title heading={5}>运行中的容器</Typography.Title>
      <Table
        rowKey={(c) => c.ID || c.Name || ""}
        pagination={false}
        loading={loading}
        data={runtime}
        columns={[
          { title: "容器", render: (_: unknown, c: Container) => c.Name || c.Service },
          { title: "镜像", dataIndex: "Image" },
          { title: "状态", render: (_: unknown, c: Container) => c.Status || c.State },
        ]}
      />

      <Typography.Title heading={5}>部署历史</Typography.Title>
      <Table
        rowKey="id"
        pagination={false}
        loading={loading}
        data={deploys}
        onRow={(row) => ({
          onClick: () => navigate("/deploy/" + name + "/runs/" + (row as DeployRecord).id),
          style: { cursor: "pointer" },
        })}
        columns={[
          { title: "版本", dataIndex: "version" },
          {
            title: "状态",
            dataIndex: "status",
            render: (v: string) => <Tag color={v === "ok" ? "green" : "red"}>{v}</Tag>,
          },
          { title: "时间", dataIndex: "createdAt" },
        ]}
      />
    </div>
  );
}
