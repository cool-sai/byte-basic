import { useState } from "react";
import { Button, Card, Message, Modal, Radio, Select, Table, Tag, Typography } from "@arco-design/web-react";
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
  const [open, setOpen] = useState(false);
  const [mode, setMode] = useState<"artifact" | "build">("artifact");
  const [version, setVersion] = useState("");
  const [branch, setBranch] = useState("");
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
  const scmName = app?.scmName || "";

  const { data: br, loading: brLoading, error: brErr } = useRequest(() => api.branches(scmName), {
    ready: open && !!scmName,
    refreshDeps: [scmName, open],
    onSuccess: (d) => {
      const first = d.default || d.branches?.[0]?.name || "";
      setBranch((cur) => ((d.branches || []).some((x) => x.name === cur) ? cur : first));
    },
  });
  const branches = br?.branches || [];

  const { run: deploy, loading: busy } = useRequest(
    async (ver: string, doBuild: boolean, brName: string) => {
      setLiveLog("");
      let version = ver;
      if (doBuild) {
        if (!scmName) {
          throw new Error("未关联 SCM");
        }
        const row = await api.createBuild(scmName, brName);
        version = row.version;
        await api.watchBuild(row.id, (text) => setLiveLog((cur) => cur + text));
      }
      await api.deploy(name, version, (text) => setLiveLog((cur) => cur + text));
      return version;
    },
    {
      manual: true,
      onSuccess: (ver) => {
        Message.success("已部署 " + name + " @ " + ver);
        setOpen(false);
        void refresh();
      },
      onError: (e) => Message.error(errMsg(e)),
    },
  );

  const canOk = mode === "build" ? Boolean(branch) : Boolean(version);

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
          {app?.label === "python" ? (
            <div className="text-xs text-slate-500">Flask :80。发完后 TLB 把路径转到 {(compose[0] || name) + ":80"}。</div>
          ) : null}
        </div>
        <Button
          type="primary"
          disabled={!app}
          onClick={() => {
            setLiveLog("");
            setMode(okBuilds.length ? "artifact" : "build");
            setOpen(true);
          }}
        >
          部署
        </Button>
      </div>
      {error ? <Typography.Text type="error">{errMsg(error)}</Typography.Text> : null}

      {busy || liveLog ? (
        <Card title="本次部署">
          <LogBox text={liveLog} live={busy} />
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

      <Modal
        title={"部署 " + (app?.name || name)}
        visible={open}
        onCancel={() => {
          if (!busy) {
            setOpen(false);
          }
        }}
        onOk={() => void deploy(version, mode === "build", branch)}
        confirmLoading={busy}
        okButtonProps={{ disabled: !canOk || busy }}
        okText={mode === "build" ? "编译并部署" : "打镜像并启动"}
        style={{ width: 640 }}
      >
        <div className="flex w-full flex-col gap-4">
          <Radio.Group value={mode} onChange={setMode} disabled={busy}>
            <Radio value="artifact">已有产物</Radio>
            <Radio value="build">选分支编译后部署</Radio>
          </Radio.Group>
          {mode === "build" ? (
            <div className="flex w-full flex-col gap-2">
              {brErr ? <Typography.Text type="error">{errMsg(brErr)}</Typography.Text> : null}
              <Select
                className="w-full"
                loading={brLoading}
                disabled={busy}
                value={branch || undefined}
                onChange={setBranch}
                placeholder="选择要编译的分支"
                showSearch
              >
                {branches.map((b) => (
                  <Select.Option key={b.name} value={b.name}>
                    {b.name}
                  </Select.Option>
                ))}
              </Select>
            </div>
          ) : (
            <Select
              className="w-full"
              loading={loading}
              disabled={busy}
              value={version || undefined}
              onChange={setVersion}
              placeholder={okBuilds.length ? "选择 SCM 产物" : "还没有编译产物，改选分支编译"}
            >
              {okBuilds.map((b: Build) => (
                <Select.Option key={b.version} value={b.version}>
                  {b.version}
                  {b.branch ? " · " + b.branch : ""}
                </Select.Option>
              ))}
            </Select>
          )}
          {busy || liveLog ? <LogBox text={liveLog} live={busy} /> : null}
        </div>
      </Modal>
    </div>
  );
}
