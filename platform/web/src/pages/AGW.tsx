import { useState } from "react";
import { Button, Message, Select, Space, Table, Tag, Typography } from "@arco-design/web-react";
import { useRequest } from "ahooks";
import { api, errMsg, type IdlMethod, type Publish } from "../api";

function parseRoutes(s: string): IdlMethod[] {
  try {
    const v = JSON.parse(s) as IdlMethod[] | null;
    return v || [];
  } catch {
    return [];
  }
}

export default function AGW() {
  const [name, setName] = useState("order");

  const { data, loading, error, refresh } = useRequest(async () => {
    const idls = (await api.idls()) || [];
    const pubs = (await api.publishes()) || [];
    return { idls, pubs };
  });
  const idls = data?.idls || [];
  const pubs = data?.pubs || [];

  const { run: publish, loading: busy } = useRequest((idl: string) => api.publish(idl), {
    manual: true,
    onSuccess: (_d, [idl]) => {
      Message.success("已发布 " + idl + "，gateway 已重启加载 IDL。HTTP 口：18080");
      void refresh();
    },
    onError: (e) => Message.error(errMsg(e)),
  });

  const cur = idls.find((x) => x.name === name) || idls[0];
  const selected = cur?.name || name;
  const latest = pubs[0];
  const live = latest ? parseRoutes(latest.routesJson) : [];
  const draftHttp = (cur?.methods || []).filter((m) => m.uri);
  const sampleUri = live.find((m) => m.uri)?.uri || "";

  return (
    <Space direction="vertical" size="medium" className="w-full">
      <div>
        <Typography.Title heading={4} className="!mb-1">
          AGW 网关
        </Typography.Title>
        <Typography.Text type="secondary">从 BAM 拉 IDL，把带 agw.uri 的方法开通成 HTTP。发布 = 重启 gateway 读挂载的 thrift。</Typography.Text>
      </div>
      {error ? <Typography.Text type="error">{errMsg(error)}</Typography.Text> : null}
      <Space>
        <Select value={selected} onChange={setName} style={{ width: 220 }} loading={loading}>
          {idls.map((x) => (
            <Select.Option key={x.name} value={x.name}>
              {x.name}.thrift
            </Select.Option>
          ))}
        </Select>
        <Button type="primary" loading={busy} onClick={() => publish(selected)}>
          发布到网关
        </Button>
      </Space>

      <Typography.Title heading={5}>BAM 里待发布的 HTTP</Typography.Title>
      <Table
        rowKey="name"
        pagination={false}
        loading={loading}
        data={draftHttp}
        noDataElement="这份 IDL 没有 agw.uri，发布后也不会开通 HTTP。"
        columns={[
          { title: "RPC", dataIndex: "name" },
          {
            title: "HTTP",
            render: (_: unknown, m: IdlMethod) => (
              <span className="font-mono">
                {m.httpMethod} {m.uri}
              </span>
            ),
          },
          {
            title: "入参",
            render: (_: unknown, m: IdlMethod) => (m.reqFields || []).map((f) => f.name).join(", ") || "—",
          },
          {
            title: "出参",
            render: (_: unknown, m: IdlMethod) => (m.respFields || []).map((f) => f.name).join(", ") || "—",
          },
        ]}
      />

      <Typography.Title heading={5}>最近一次发布</Typography.Title>
      <Table
        rowKey="id"
        pagination={false}
        loading={loading}
        data={pubs}
        columns={[
          { title: "IDL", dataIndex: "idlName" },
          { title: "时间", dataIndex: "createdAt" },
          {
            title: "状态",
            dataIndex: "status",
            render: (v: string) => <Tag color={v === "ok" ? "green" : "red"}>{v}</Tag>,
          },
          {
            title: "路由",
            render: (_: unknown, p: Publish) =>
              parseRoutes(p.routesJson)
                .filter((m) => m.uri)
                .map((m) => m.httpMethod + " " + m.uri)
                .join(" · ") || "无 HTTP",
          },
        ]}
      />
      {sampleUri ? (
        <Typography.Text type="secondary" className="font-mono">
          {`curl -s -d '{"id":1001}' -H 'Content-Type: application/json' http://127.0.0.1:18080${sampleUri}`}
        </Typography.Text>
      ) : null}
    </Space>
  );
}
