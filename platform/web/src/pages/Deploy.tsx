import { useState } from "react";
import { Button, Message, Select, Space, Table, Tag, Typography } from "@arco-design/web-react";
import { useRequest } from "ahooks";
import { api, errMsg, type Container } from "../api";

export default function Deploy() {
  const [service, setService] = useState("gateway");
  const [version, setVersion] = useState("");

  const { data, loading, error, refresh } = useRequest(
    async () => {
      const services = (await api.services()) || [];
      const builds = (await api.builds(service)) || [];
      const deploys = (await api.deploys(service)) || [];
      const runtime = (await api.runtime()) || [];
      return { services, builds, deploys, runtime };
    },
    {
      refreshDeps: [service],
      onSuccess: (d) => {
        const okBuilds = d.builds.filter((x) => x.status === "ok");
        setVersion((v) => (okBuilds.find((x) => x.version === v) ? v : okBuilds[0]?.version || ""));
      },
    },
  );
  const services = data?.services || [];
  const builds = data?.builds || [];
  const deploys = data?.deploys || [];
  const runtime = data?.runtime || [];

  const { run: deploy, loading: busy } = useRequest((svc: string, ver: string) => api.deploy(svc, ver), {
    manual: true,
    onSuccess: (_d, [svc, ver]) => {
      Message.success("已部署 " + svc + " @ " + ver);
      void refresh();
    },
    onError: (e) => Message.error(errMsg(e)),
  });

  return (
    <Space direction="vertical" size="medium" className="w-full">
      <div>
        <Typography.Title heading={4} className="!mb-1">
          部署
        </Typography.Title>
        <Typography.Text type="secondary">选 SCM 版本，把制品拷到 bin/ 再 compose build + up --no-deps。只动这一个服务。</Typography.Text>
      </div>
      {error ? <Typography.Text type="error">{errMsg(error)}</Typography.Text> : null}
      <Space>
        <Select value={service} onChange={setService} style={{ width: 160 }} loading={loading}>
          {services.map((s) => (
            <Select.Option key={s.name} value={s.name}>
              {s.name}
            </Select.Option>
          ))}
        </Select>
        <Select value={version} onChange={setVersion} style={{ width: 220 }} loading={loading}>
          {builds
            .filter((b) => b.status === "ok")
            .map((b) => (
              <Select.Option key={b.version} value={b.version}>
                {b.version}
              </Select.Option>
            ))}
        </Select>
        <Button type="primary" loading={busy} disabled={!version} onClick={() => deploy(service, version)}>
          构建并部署
        </Button>
      </Space>

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

      <Typography.Title heading={5}>部署记录</Typography.Title>
      <Table
        rowKey="id"
        pagination={false}
        loading={loading}
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
