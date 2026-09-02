import { useState } from "react";
import { Button, Card, Input, Message, Modal, Space, Spin, Typography } from "@arco-design/web-react";
import { useRequest } from "ahooks";
import { useLocation, useNavigate } from "react-router-dom";
import { api, errMsg } from "../../api";
import Crumbs from "./Crumbs";

export default function JobList() {
  const loc = useLocation();
  const navigate = useNavigate();
  const [name, setName] = useState("");
  const [gitUrl, setGitUrl] = useState("");
  const [script, setScript] = useState("scripts/scm/user.sh");
  const [creating, setCreating] = useState(false);

  const {
    data: jobs = [],
    loading,
    error,
    refresh,
  } = useRequest(async () => (await api.scmJobs()).jobs || [], { refreshDeps: [loc.key] });

  const { runAsync: remove } = useRequest((n: string) => api.deleteScmJob(n), {
    manual: true,
    onSuccess: (_d, [n]) => {
      Message.success("已删除 " + n);
      void refresh();
    },
    onError: (e) => Message.error(errMsg(e)),
  });

  const { runAsync: submit, loading: submitting } = useRequest(
    (n: string, g: string, s: string) => api.createScmJob(n, g, s),
    {
      manual: true,
      onSuccess: (_d, [n]) => {
        Message.success("已新建 " + n);
        setName("");
        setCreating(false);
        void refresh();
      },
      onError: (e) => Message.error(errMsg(e)),
    },
  );

  return (
    <Space direction="vertical" size="large" className="w-full">
      <Crumbs />
      <div className="flex items-start justify-between gap-3">
        <div>
          <Typography.Title heading={4} className="!mb-1">
            SCM 编译任务
          </Typography.Title>
          <Typography.Text type="secondary">
            点进任务编译；编译时选分支，进当次记录看实时日志。
          </Typography.Text>
        </div>
        <Button type="primary" onClick={() => setCreating(true)}>
          新建任务
        </Button>
      </div>
      {error ? <Typography.Text type="error">{errMsg(error)}</Typography.Text> : null}

      <Modal
        title="新建编译任务"
        visible={creating}
        onCancel={() => setCreating(false)}
        onOk={() => void submit(name.trim(), gitUrl.trim(), script.trim())}
        confirmLoading={submitting}
        okButtonProps={{ disabled: !name.trim() || !gitUrl.trim() || !script.trim() }}
      >
        <Space direction="vertical" className="w-full" size="medium">
          <Input addBefore="名称" value={name} onChange={setName} placeholder="user" />
          <Input addBefore="Git" value={gitUrl} onChange={setGitUrl} placeholder="https://github.com/coolCicada/byte-basic.git" />
          <Input addBefore="脚本" value={script} onChange={setScript} placeholder="scripts/scm/user.sh" />
          <Typography.Text type="secondary">
            本仓库示例脚本：scripts/scm/user.sh、order.sh、gateway.sh、etcdui.sh、platform-web.sh
          </Typography.Text>
        </Space>
      </Modal>

      <Spin loading={loading} className="w-full">
        <div className="grid grid-cols-1 gap-3 md:grid-cols-2">
          {jobs.map((j) => (
            <Card
              key={j.name}
              title={j.name}
              hoverable
              className="cursor-pointer"
              onClick={() => navigate("/scm/" + j.name)}
              extra={
                <Button
                  size="mini"
                  status="danger"
                  onClick={(e) => {
                    e.stopPropagation();
                    Modal.confirm({
                      title: "删除任务 " + j.name + "？",
                      content: "编译记录、clone、产物都会删掉。",
                      okButtonProps: { status: "danger" },
                      onOk: () => remove(j.name),
                    });
                  }}
                >
                  删除
                </Button>
              }
            >
              <div className="text-xs text-slate-500">Git {j.gitUrl}</div>
              <div className="text-xs text-slate-500">脚本 {j.scriptPath}</div>
            </Card>
          ))}
        </div>
      </Spin>
    </Space>
  );
}
