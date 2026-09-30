import { useState } from "react";
import { Button, Message, Modal, Radio, Select, Table, Tag, Typography } from "@arco-design/web-react";
import { useRequest } from "ahooks";
import { Link, useLocation, useNavigate, useParams } from "react-router-dom";
import { api, errMsg, type Build, type Container, type DeployRecord } from "../../api";
import LabelIcon from "../scm/LabelIcon";
import AppForm from "./AppForm";
import Crumbs from "./Crumbs";

export default function AppPage() {
  const { name = "" } = useParams();
  const loc = useLocation();
  const navigate = useNavigate();
  const [open, setOpen] = useState(false);
  const [editOpen, setEditOpen] = useState(false);
  const [mode, setMode] = useState<"artifact" | "build">("artifact");
  const [version, setVersion] = useState("");
  const [branch, setBranch] = useState("");

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
  const running = deploys.find((d) => d.status === "running");

  const { data: formMeta } = useRequest(
    async () => ({
      jobs: (await api.scmJobs()).jobs || [],
      services: (await api.services()) || [],
    }),
    { ready: editOpen, refreshDeps: [editOpen] },
  );

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
      if (doBuild) {
        if (!scmName) {
          throw new Error("未关联 SCM");
        }
        return api.createDeploy(name, "", brName);
      }
      return api.createDeploy(name, ver);
    },
    {
      manual: true,
      onSuccess: (row) => navigate("/deploy/" + name + "/runs/" + row.id),
      onError: (e) => Message.error(errMsg(e)),
    },
  );

  const building = (data?.builds || []).some((b) => b.status === "running");
  const canOk = mode === "build" ? Boolean(branch) && !building : Boolean(version);

  return (
    <div className="flex w-full flex-col gap-6">
      <Crumbs appName={name} />
      <div className="flex items-start justify-between gap-3">
        <div>
          <Typography.Title heading={4} className="!mb-1 inline-flex items-center gap-2">
            <LabelIcon label={app?.label} />
            {app?.name || name}
          </Typography.Title>
          <div className="meta">
            SCM {app?.scmName ? <Link to={"/scm/" + app.scmName}>{app.scmName}</Link> : "—"}
            {app?.label ? " · " + app.label : ""}
          </div>
          <div className="meta">Compose {compose.join(", ")}</div>
          {app?.label === "node" ? (
            <div className="meta">静态站 :80。发完后 TLB 把路径转到 {(compose[0] || name) + ":80"}。</div>
          ) : null}
          {app?.label === "python" ? (
            <div className="meta">Flask :80。发完后 TLB 把路径转到 {(compose[0] || name) + ":80"}。</div>
          ) : null}
          {app?.label === "image" ? (
            <div className="meta">按仓库 Dockerfile 打镜像，发完后替换 compose 服务 {compose.join(", ")}。</div>
          ) : null}
        </div>
        <div className="flex items-center gap-2">
          <Button disabled={!app} onClick={() => setEditOpen(true)}>
            编辑
          </Button>
          <Button
            status="danger"
            onClick={() => {
              Modal.confirm({
                title: "删除项目 " + name + "？",
                content: "部署记录会删掉。不在主 compose 里的容器会停掉。",
                okButtonProps: { status: "danger" },
                onOk: () =>
                  api.deleteDeployApp(name).then(
                    () => {
                      Message.success("已删除 " + name);
                      navigate("/deploy");
                    },
                    (e) => Message.error(errMsg(e)),
                  ),
              });
            }}
          >
            删除
          </Button>
          <Button
            type="primary"
            disabled={!app || !!running || busy}
            onClick={() => {
              setMode(okBuilds.length ? "artifact" : "build");
              setOpen(true);
            }}
          >
            部署
          </Button>
        </div>
      </div>
      {error ? <Typography.Text type="error">{errMsg(error)}</Typography.Text> : null}

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
            render: (v: string) => (
              <Tag color={v === "ok" ? "green" : v === "running" ? "arcoblue" : "red"}>{v}</Tag>
            ),
          },
          { title: "时间", dataIndex: "createdAt" },
        ]}
      />

      <AppForm
        visible={editOpen}
        app={app}
        jobs={formMeta?.jobs || []}
        services={formMeta?.services || []}
        onCancel={() => setEditOpen(false)}
        onOk={(n, scm, c) => {
          void api.updateDeployApp(n, scm, c).then(
            () => {
              Message.success("已保存");
              setEditOpen(false);
              void refresh();
            },
            (e) => Message.error(errMsg(e)),
          );
        }}
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
        okButtonProps={{ disabled: !canOk || busy || !!running }}
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
              {building ? <Typography.Text type="warning">已有编译在进行</Typography.Text> : null}
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
        </div>
      </Modal>
    </div>
  );
}
