import { useState } from "react";
import { Button, Card, Input, Message, Modal, Select, Spin, Typography } from "@arco-design/web-react";
import { useRequest } from "ahooks";
import { useLocation, useNavigate } from "react-router-dom";
import { api, errMsg } from "../../api";
import Crumbs from "./Crumbs";

export default function ModuleList() {
  const loc = useLocation();
  const navigate = useNavigate();
  const [newOpen, setNewOpen] = useState(false);
  const [newName, setNewName] = useState("");
  const [newScm, setNewScm] = useState("");
  const [newDir, setNewDir] = useState("");
  const [newBranch, setNewBranch] = useState("");

  const { data: modules = [], loading, error, refresh } = useRequest(() => api.bamModules(), { refreshDeps: [loc.key] });
  const { data: jobs = [] } = useRequest(async () => (await api.scmJobs()).jobs || [], { ready: newOpen });
  const { data: br, loading: brLoading } = useRequest(() => api.branches(newScm), {
    ready: newOpen && !!newScm,
    refreshDeps: [newScm, newOpen],
    onSuccess: (d) => {
      const first = d.default || d.branches?.[0]?.name || "";
      setNewBranch((cur) => ((d.branches || []).some((x) => x.name === cur) ? cur : first));
    },
  });
  const branches = br?.branches || [];

  const { run: create, loading: creating } = useRequest(
    (n: string, scm: string, dir: string, branch: string) => api.createBamModule(n, scm, dir, branch),
    {
      manual: true,
      onSuccess: (m) => {
        Message.success("已创建 " + m.name);
        setNewOpen(false);
        setNewName("");
        setNewScm("");
        setNewDir("");
        setNewBranch("");
        void refresh();
        navigate("/bam/" + m.name);
      },
      onError: (e) => Message.error(errMsg(e)),
    },
  );

  return (
    <div className="flex w-full flex-col gap-6">
      <Crumbs />
      <div className="flex items-start justify-between gap-3">
        <div>
          <Typography.Title heading={4} className="!mb-1">
            BAM 服务
          </Typography.Title>
          <Typography.Text type="secondary">
            一个模块绑仓库里的一个 proto 目录。点进去看接口。项目里 bam.yaml 配语言和分支，bam update 拉最新。
          </Typography.Text>
        </div>
        <Button type="primary" onClick={() => setNewOpen(true)}>
          新建模块
        </Button>
      </div>
      {error ? <Typography.Text type="error">{errMsg(error)}</Typography.Text> : null}
      <Spin loading={loading} className="w-full">
        <div className="grid grid-cols-1 gap-3 md:grid-cols-2">
          {modules.map((m) => (
            <Card key={m.name} title={m.name} hoverable className="cursor-pointer" onClick={() => navigate("/bam/" + m.name)}>
              <div className="meta">v{m.version} · RPC {m.rpcs}{m.httpApis ? " · HTTP " + m.httpApis : ""}</div>
              <div className="meta">
                {m.scmName ? "仓库 " + m.scmName : "未绑仓库"}
                {m.protoDir ? " · " + m.protoDir : ""}
              </div>
            </Card>
          ))}
        </div>
      </Spin>
      <Modal
        title="新建 proto 模块"
        visible={newOpen}
        onCancel={() => setNewOpen(false)}
        onOk={() => create(newName.trim(), newScm, newDir.trim(), newBranch)}
        confirmLoading={creating}
        okButtonProps={{ disabled: !newName.trim() || !newScm || !newDir.trim() }}
      >
        <div className="flex w-full flex-col gap-4">
          <Input addBefore="名称" placeholder="platform" value={newName} onChange={setNewName} />
          <Select
            value={newScm || undefined}
            onChange={(v: string) => setNewScm(v)}
            placeholder={jobs.length ? "选择 SCM 仓库" : "先去 SCM 登记仓库"}
            className="w-full"
          >
            {jobs.map((j) => (
              <Select.Option key={j.name} value={j.name}>
                {j.name + " · " + j.gitUrl}
              </Select.Option>
            ))}
          </Select>
          <Select
            value={newBranch || undefined}
            onChange={setNewBranch}
            placeholder="默认分支"
            loading={brLoading}
            allowClear
            className="w-full"
          >
            {branches.map((b) => (
              <Select.Option key={b.name} value={b.name}>
                {b.name}
              </Select.Option>
            ))}
          </Select>
          <Input addBefore="目录" placeholder="proto/platform/v1" value={newDir} onChange={setNewDir} />
          <Typography.Text type="secondary">同一仓库可以建多个模块，目录不同即可。</Typography.Text>
        </div>
      </Modal>
    </div>
  );
}
