import { useEffect, useRef, useState } from "react";
import { Button, Card, Input, Message, Space, Table, Tag, Typography } from "@arco-design/web-react";
import { api, errMsg, type Build, type ScmJob } from "../api";

export default function SCM() {
  const [jobs, setJobs] = useState<ScmJob[]>([]);
  const [builds, setBuilds] = useState<Build[]>([]);
  const [name, setName] = useState("");
  const [gitUrl, setGitUrl] = useState("");
  const [script, setScript] = useState("scripts/scm/user.sh");
  const [busy, setBusy] = useState("");
  const [err, setErr] = useState("");
  const [liveLog, setLiveLog] = useState("");
  const logRef = useRef<HTMLPreElement>(null);

  async function load() {
    setErr("");
    try {
      const data = await api.scmJobs();
      setJobs(data.jobs || []);
      setBuilds((await api.builds()) || []);
    } catch (e) {
      setErr(errMsg(e));
    }
  }
  useEffect(() => {
    void load();
  }, []);

  async function create() {
    setBusy("create");
    setErr("");
    try {
      await api.createScmJob(name.trim(), gitUrl.trim(), script.trim());
      Message.success("已新建 " + name.trim());
      setName("");
      await load();
    } catch (e) {
      setErr(errMsg(e));
    } finally {
      setBusy("");
    }
  }

  useEffect(() => {
    const el = logRef.current;
    if (el) {
      el.scrollTop = el.scrollHeight;
    }
  }, [liveLog]);

  async function build(jobName: string) {
    setBusy(jobName);
    setErr("");
    setLiveLog("");
    try {
      await api.build(jobName, (text) => setLiveLog((cur) => cur + text));
      Message.success(jobName + " 编译完成");
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
        <Typography.Text type="secondary">
          新建时填 Git 地址和编译脚本。点编译会 git clone / pull 到 scm-work/，再跑脚本，产物落到 artifacts/名称/版本/。名称要和部署页服务名一致（user / order / gateway / etcdui）。
        </Typography.Text>
      </div>
      {err && <Typography.Text type="error">{err}</Typography.Text>}

      <Card title="新建编译任务">
        <Space direction="vertical" className="w-full" size="medium">
          <Input addBefore="名称" value={name} onChange={setName} placeholder="user" />
          <Input addBefore="Git" value={gitUrl} onChange={setGitUrl} placeholder="https://github.com/coolCicada/byte-basic.git" />
          <Input addBefore="脚本" value={script} onChange={setScript} placeholder="scripts/scm/user.sh" />
          <Typography.Text type="secondary">
            本仓库示例脚本：scripts/scm/user.sh、order.sh、gateway.sh、etcdui.sh
          </Typography.Text>
          <Button type="primary" loading={busy === "create"} disabled={!!busy || !name.trim() || !gitUrl.trim() || !script.trim()} onClick={() => void create()}>
            新建
          </Button>
        </Space>
      </Card>

      <div className="grid grid-cols-1 gap-3 md:grid-cols-2">
        {jobs.map((j) => (
          <Card key={j.name} title={j.name}>
            <div className="text-xs text-slate-500">Git {j.gitUrl}</div>
            <div className="text-xs text-slate-500">脚本 {j.scriptPath}</div>
            <div className="mt-3">
              <Button type="primary" loading={busy === j.name} disabled={!!busy} onClick={() => void build(j.name)}>
                编译
              </Button>
            </div>
          </Card>
        ))}
      </div>

      {(busy && busy !== "create") || liveLog ? (
        <Card title="编译日志">
          <pre
            ref={logRef}
            className="m-0 max-h-80 overflow-auto whitespace-pre-wrap bg-slate-900 p-3 font-mono text-xs text-slate-100"
          >
            {liveLog || "启动中…"}
          </pre>
        </Card>
      ) : null}

      <Typography.Title heading={5}>制品</Typography.Title>
      <Table
        rowKey="id"
        pagination={false}
        data={builds}
        columns={[
          { title: "任务", dataIndex: "service" },
          { title: "版本", dataIndex: "version" },
          {
            title: "状态",
            dataIndex: "status",
            render: (v: string) => <Tag color={v === "ok" ? "green" : "red"}>{v}</Tag>,
          },
          { title: "路径", dataIndex: "binPath" },
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
