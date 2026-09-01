import { useEffect, useState } from "react";
import { Button, Card, Message, Space, Table, Tag, Typography } from "@arco-design/web-react";
import { api, errMsg, type Build, type Service } from "../api";

export default function SCM() {
  const [services, setServices] = useState<Service[]>([]);
  const [builds, setBuilds] = useState<Build[]>([]);
  const [busy, setBusy] = useState("");
  const [err, setErr] = useState("");

  async function load() {
    setErr("");
    try {
      setServices(await api.services());
      setBuilds((await api.builds()) || []);
    } catch (e) {
      setErr(errMsg(e));
    }
  }
  useEffect(() => {
    void load();
  }, []);

  async function build(name: string) {
    setBusy(name);
    setErr("");
    try {
      await api.build(name);
      Message.success(name + " 编译完成");
      await load();
    } catch (e) {
      setErr(errMsg(e));
    } finally {
      setBusy("");
    }
  }

  return (
    <Space direction="vertical" size="large" className="w-full">
      <div>
        <Typography.Title heading={4} className="!mb-1">
          SCM 编译
        </Typography.Title>
        <Typography.Text type="secondary">对本机交叉编译 Linux 二进制，产出带版本号的制品。还不会替换正在跑的容器。</Typography.Text>
      </div>
      {err && <Typography.Text type="error">{err}</Typography.Text>}
      <div className="grid grid-cols-1 gap-3 md:grid-cols-2 xl:grid-cols-4">
        {services.map((s) => (
          <Card key={s.name} title={s.name} extra={<Typography.Text type="secondary">{s.pkg}</Typography.Text>}>
            <Typography.Text type="secondary">compose: {s.compose.join(", ")}</Typography.Text>
            <div className="mt-3">
              <Button type="primary" loading={busy === s.name} disabled={!!busy} onClick={() => void build(s.name)}>
                编译
              </Button>
            </div>
          </Card>
        ))}
      </div>
      <Typography.Title heading={5}>制品</Typography.Title>
      <Table
        rowKey="id"
        pagination={false}
        data={builds}
        columns={[
          { title: "服务", dataIndex: "service" },
          { title: "版本", dataIndex: "version" },
          {
            title: "状态",
            dataIndex: "status",
            render: (v: string) => <Tag color={v === "ok" ? "green" : "red"}>{v}</Tag>,
          },
          { title: "路径", dataIndex: "binPath" },
          { title: "时间", dataIndex: "createdAt" },
        ]}
      />
    </Space>
  );
}
