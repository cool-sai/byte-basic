import { useEffect, useRef, useState } from "react";
import { Breadcrumb, Button, Card, Input, Message, Modal, Space, Table, Tag, Typography } from "@arco-design/web-react";
import { api, errMsg, type Build, type ScmJob } from "../api";

function LogBox({ text, live }: { text: string; live?: boolean }) {
  const ref = useRef<HTMLPreElement>(null);
  useEffect(() => {
    const el = ref.current;
    if (live && el) {
      el.scrollTop = el.scrollHeight;
    }
  }, [text, live]);
  return (
    <pre
      ref={ref}
      className="m-0 max-h-[28rem] overflow-auto whitespace-pre-wrap bg-slate-900 p-3 font-mono text-xs text-slate-100"
    >
      {text || (live ? "启动中…" : "无日志")}
    </pre>
  );
}

export default function SCM() {
  const [jobs, setJobs] = useState<ScmJob[]>([]);
  const [builds, setBuilds] = useState<Build[]>([]);
  const [job, setJob] = useState<ScmJob | null>(null);
  const [build, setBuild] = useState<Build | null>(null);
  const [name, setName] = useState("");
  const [gitUrl, setGitUrl] = useState("");
  const [script, setScript] = useState("scripts/scm/user.sh");
  const [busy, setBusy] = useState("");
  const [err, setErr] = useState("");
  const [liveLog, setLiveLog] = useState("");
  const [creating, setCreating] = useState(false);

  async function loadJobs() {
    setErr("");
    try {
      const data = await api.scmJobs();
      setJobs(data.jobs || []);
    } catch (e) {
      setErr(errMsg(e));
    }
  }

  async function loadBuilds(jobName: string) {
    setErr("");
    try {
      setBuilds((await api.builds(jobName)) || []);
    } catch (e) {
      setErr(errMsg(e));
    }
  }

  useEffect(() => {
    void loadJobs();
  }, []);

  async function create() {
    setBusy("create");
    setErr("");
    try {
      await api.createScmJob(name.trim(), gitUrl.trim(), script.trim());
      Message.success("已新建 " + name.trim());
      setName("");
      setCreating(false);
      await loadJobs();
    } catch (e) {
      setErr(errMsg(e));
    } finally {
      setBusy("");
    }
  }

  async function openJob(j: ScmJob) {
    setJob(j);
    setBuild(null);
    await loadBuilds(j.name);
  }

  async function openBuild(row: Build) {
    setErr("");
    try {
      setBuild(await api.buildDetail(row.id));
    } catch (e) {
      setErr(errMsg(e));
    }
  }

  async function runBuild() {
    if (!job) {
      return;
    }
    setBusy(job.name);
    setErr("");
    setLiveLog("");
    try {
      await api.build(job.name, (text) => setLiveLog((cur) => cur + text));
      Message.success(job.name + " 编译完成");
      await loadBuilds(job.name);
    } catch (e) {
      setErr(errMsg(e));
    } finally {
      setBusy("");
    }
  }

  function backToJobs() {
    setJob(null);
    setBuild(null);
    setLiveLog("");
    void loadJobs();
  }

  function backToJob() {
    setBuild(null);
  }

  const crumbs = (
    <Breadcrumb>
      <Breadcrumb.Item onClick={backToJobs}>SCM</Breadcrumb.Item>
      {job ? (
        <Breadcrumb.Item onClick={backToJob}>{job.name}</Breadcrumb.Item>
      ) : null}
      {build ? <Breadcrumb.Item>{build.version}</Breadcrumb.Item> : null}
    </Breadcrumb>
  );

  if (job && build) {
    return (
      <Space direction="vertical" size="large" className="w-full">
        {crumbs}
        <div>
          <Typography.Title heading={4} className="!mb-1">
            编译 {build.version}
          </Typography.Title>
          <Typography.Text type="secondary">
            {build.service} · {build.binPath}
          </Typography.Text>
        </div>
        {err && <Typography.Text type="error">{err}</Typography.Text>}
        <Space>
          <Tag color={build.status === "ok" ? "green" : "red"}>{build.status}</Tag>
          <Typography.Text type="secondary">{build.createdAt}</Typography.Text>
        </Space>
        <Card title="日志">
          <LogBox text={build.log || ""} />
        </Card>
      </Space>
    );
  }

  if (job) {
    return (
      <Space direction="vertical" size="large" className="w-full">
        {crumbs}
        <div className="flex items-start justify-between gap-3">
          <div>
            <Typography.Title heading={4} className="!mb-1">
              {job.name}
            </Typography.Title>
            <div className="text-xs text-slate-500">Git {job.gitUrl}</div>
            <div className="text-xs text-slate-500">脚本 {job.scriptPath}</div>
          </div>
          <Button type="primary" loading={busy === job.name} disabled={!!busy} onClick={() => void runBuild()}>
            编译
          </Button>
        </div>
        {err && <Typography.Text type="error">{err}</Typography.Text>}
        {busy === job.name || liveLog ? (
          <Card title="本次编译">
            <LogBox text={liveLog} live />
          </Card>
        ) : null}
        <Typography.Title heading={5}>编译历史</Typography.Title>
        <Table
          rowKey="id"
          pagination={false}
          data={builds}
          onRow={(row) => ({
            onClick: () => void openBuild(row as Build),
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

  return (
    <Space direction="vertical" size="large" className="w-full">
      {crumbs}
      <div className="flex items-start justify-between gap-3">
        <div>
          <Typography.Title heading={4} className="!mb-1">
            SCM 编译任务
          </Typography.Title>
          <Typography.Text type="secondary">
            点进任务编译；历史记录点进去看当次日志。名称要和部署页服务名一致（user / order / gateway / etcdui）。
          </Typography.Text>
        </div>
        <Button type="primary" onClick={() => setCreating(true)}>
          新建任务
        </Button>
      </div>
      {err && <Typography.Text type="error">{err}</Typography.Text>}

      <Modal
        title="新建编译任务"
        visible={creating}
        onCancel={() => setCreating(false)}
        onOk={() => void create()}
        confirmLoading={busy === "create"}
        okButtonProps={{ disabled: !name.trim() || !gitUrl.trim() || !script.trim() }}
      >
        <Space direction="vertical" className="w-full" size="medium">
          <Input addBefore="名称" value={name} onChange={setName} placeholder="user" />
          <Input addBefore="Git" value={gitUrl} onChange={setGitUrl} placeholder="https://github.com/coolCicada/byte-basic.git" />
          <Input addBefore="脚本" value={script} onChange={setScript} placeholder="scripts/scm/user.sh" />
          <Typography.Text type="secondary">
            本仓库示例脚本：scripts/scm/user.sh、order.sh、gateway.sh、etcdui.sh
          </Typography.Text>
        </Space>
      </Modal>

      <div className="grid grid-cols-1 gap-3 md:grid-cols-2">
        {jobs.map((j) => (
          <Card key={j.name} title={j.name} hoverable className="cursor-pointer" onClick={() => void openJob(j)}>
            <div className="text-xs text-slate-500">Git {j.gitUrl}</div>
            <div className="text-xs text-slate-500">脚本 {j.scriptPath}</div>
          </Card>
        ))}
      </div>
    </Space>
  );
}
