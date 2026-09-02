import { useState } from "react";
import { Button, Card, Message, Space, Table, Tag, Typography } from "@arco-design/web-react";
import { useRequest } from "ahooks";
import { useLocation, useNavigate, useParams } from "react-router-dom";
import { api, errMsg, type Build } from "../../api";
import Crumbs from "./Crumbs";
import LogBox from "./LogBox";

export default function JobPage() {
  const { name = "" } = useParams();
  const loc = useLocation();
  const navigate = useNavigate();
  const [liveLog, setLiveLog] = useState("");
  const [err, setErr] = useState("");

  const { data, loading, error, refresh } = useRequest(
    async () => ({
      job: await api.scmJob(name),
      builds: (await api.builds(name)) || [],
    }),
    { refreshDeps: [name, loc.key] },
  );
  const job = data?.job;
  const builds = data?.builds || [];

  const { run: compile, loading: busy } = useRequest(
    async () => {
      setErr("");
      setLiveLog("");
      await api.build(name, (text) => setLiveLog((cur) => cur + text));
    },
    {
      manual: true,
      onSuccess: () => {
        Message.success(name + " 编译完成");
        void refresh();
      },
      onError: (e) => {
        const msg = errMsg(e);
        setErr(msg);
        Message.error(msg);
      },
    },
  );

  return (
    <Space direction="vertical" size="large" className="w-full">
      <Crumbs jobName={name} />
      <div className="flex items-start justify-between gap-3">
        <div>
          <Typography.Title heading={4} className="!mb-1">
            {job?.name || name}
          </Typography.Title>
          <div className="text-xs text-slate-500">Git {job?.gitUrl}</div>
          <div className="text-xs text-slate-500">脚本 {job?.scriptPath}</div>
        </div>
        <Button type="primary" loading={busy} disabled={busy} onClick={() => compile()}>
          编译
        </Button>
      </div>
      {error ? <Typography.Text type="error">{errMsg(error)}</Typography.Text> : null}
      {err ? <Typography.Text type="error">{err}</Typography.Text> : null}
      {busy || liveLog ? (
        <Card title="本次编译">
          <LogBox text={liveLog} live />
        </Card>
      ) : null}
      <Typography.Title heading={5}>编译历史</Typography.Title>
      <Table
        rowKey="id"
        pagination={false}
        loading={loading}
        data={builds}
        onRow={(row) => ({
          onClick: () => navigate("/scm/" + name + "/builds/" + (row as Build).id),
          style: { cursor: "pointer" },
        })}
        columns={[
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
