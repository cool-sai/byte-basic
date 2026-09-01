import { useEffect, useState } from "react";
import { Button, Message, Select, Space, Table, Tag, Typography } from "@arco-design/web-react";
import { api, errMsg, type Build, type Container, type DeployRecord, type Service } from "../api";

export default function Deploy() {
  const [services, setServices] = useState<Service[]>([]);
  const [service, setService] = useState("gateway");
  const [builds, setBuilds] = useState<Build[]>([]);
  const [version, setVersion] = useState("");
  const [deploys, setDeploys] = useState<DeployRecord[]>([]);
  const [runtime, setRuntime] = useState<Container[]>([]);
  const [err, setErr] = useState("");
  const [busy, setBusy] = useState(false);

  async function load(svc: string) {
    setErr("");
    const svcs = (await api.services()) || [];
    setServices(svcs);
    const b = (await api.builds(svc)) || [];
    setBuilds(b);
    const okBuilds = b.filter((x) => x.status === "ok");
    setVersion((v) => (okBuilds.find((x) => x.version === v) ? v : okBuilds[0]?.version || ""));
    setDeploys((await api.deploys(svc)) || []);
    setRuntime((await api.runtime()) || []);
  }
  useEffect(() => {
    load(service).catch((e) => setErr(errMsg(e)));
  }, [service]);

  async function deploy() {
    if (!version) return;
    setBusy(true);
    setErr("");
    try {
      await api.deploy(service, version);
      Message.success("已部署 " + service + " @ " + version);
      await load(service);
    } catch (e) {
      setErr(errMsg(e));
    } finally {
      setBusy(false);
    }
  }

  return (
    <Space direction="vertical" size="medium" className="w-full">
      <div>
        <Typography.Title heading={4} className="!mb-1">
          部署
        </Typography.Title>
        <Typography.Text type="secondary">选 SCM 版本，把制品拷到 bin/ 再 compose build + up --no-deps。只动这一个服务。</Typography.Text>
      </div>
      {err && <Typography.Text type="error">{err}</Typography.Text>}
      <Space>
        <Select value={service} onChange={setService} style={{ width: 160 }}>
          {services.map((s) => (
            <Select.Option key={s.name} value={s.name}>
              {s.name}
            </Select.Option>
          ))}
        </Select>
        <Select value={version} onChange={setVersion} style={{ width: 220 }}>
          {builds
            .filter((b) => b.status === "ok")
            .map((b) => (
              <Select.Option key={b.version} value={b.version}>
                {b.version}
              </Select.Option>
            ))}
        </Select>
        <Button type="primary" loading={busy} disabled={!version} onClick={() => void deploy()}>
          构建并部署
        </Button>
      </Space>

      <Typography.Title heading={5}>运行中的容器</Typography.Title>
      <Table
        rowKey={(c) => c.ID || c.Name || ""}
        pagination={false}
        data={runtime}
        columns={[
          { title: "容器", render: (_: unknown, c: Container) => c.Name || c.Service },
          { title: "镜像", dataIndex: "Image" },
          { title: "状态", render: (_: unknown, c: Container) => c.Status || c.State },
        ]}
      />

      <Typography.Title heading={5}>部署记录</Typography.Title>
      <Table
        rowKey="id"
        pagination={false}
        data={deploys}
        columns={[
          { title: "服务", dataIndex: "service" },
          { title: "版本", dataIndex: "version" },
          {
            title: "状态",
            dataIndex: "status",
            render: (v: string) => <Tag color={v === "ok" ? "green" : "red"}>{v}</Tag>,
          },
          { title: "时间", dataIndex: "createdAt" },
          {
            title: "日志",
            dataIndex: "log",
            render: (v: string) => <pre className="m-0 max-h-20 max-w-md overflow-auto text-xs text-slate-500">{v}</pre>,
          },
        ]}
      />
    </Space>
  );
}
