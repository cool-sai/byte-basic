import { useState } from "react";
import { Button, Card, Message, Modal, Spin, Typography } from "@arco-design/web-react";
import { useRequest } from "ahooks";
import { useLocation, useNavigate } from "react-router-dom";
import { api, errMsg, type ScmJob } from "../../api";
import Crumbs from "./Crumbs";
import JobForm from "./JobForm";
import LabelIcon from "./LabelIcon";

export default function JobList() {
  const loc = useLocation();
  const navigate = useNavigate();
  const [formOpen, setFormOpen] = useState(false);
  const [editing, setEditing] = useState<ScmJob | null>(null);

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

  const { runAsync: save, loading: saving } = useRequest(
    (n: string, g: string, s: string, label: string, isEdit: boolean) =>
      isEdit ? api.updateScmJob(n, g, s, label) : api.createScmJob(n, g, s, label),
    {
      manual: true,
      onSuccess: (_d, [n, _g, _s, _l, isEdit]) => {
        Message.success(isEdit ? "已保存 " + n : "已新建 " + n);
        setFormOpen(false);
        setEditing(null);
        void refresh();
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
            SCM 编译任务
          </Typography.Title>
          <Typography.Text type="secondary">
            点进任务编译；编译时选分支，进当次记录看实时日志。label 用来区分 golang / node / python。
          </Typography.Text>
        </div>
        <Button
          type="primary"
          onClick={() => {
            setEditing(null);
            setFormOpen(true);
          }}
        >
          新建任务
        </Button>
      </div>
      {error ? <Typography.Text type="error">{errMsg(error)}</Typography.Text> : null}

      <Spin loading={loading} className="w-full">
        <div className="grid grid-cols-1 gap-3 md:grid-cols-2">
          {jobs.map((j) => (
            <Card
              key={j.name}
              title={
                <span className="inline-flex items-center gap-2">
                  <LabelIcon label={j.label} />
                  {j.name}
                </span>
              }
              hoverable
              className="cursor-pointer"
              onClick={() => navigate("/scm/" + j.name)}
              extra={
                <div className="flex items-center gap-2">
                  <Button
                    size="mini"
                    onClick={(e) => {
                      e.stopPropagation();
                      setEditing(j);
                      setFormOpen(true);
                    }}
                  >
                    编辑
                  </Button>
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
                </div>
              }
            >
              <div className="meta">Git {j.gitUrl}</div>
              <div className="meta">脚本 {j.scriptPath}</div>
            </Card>
          ))}
        </div>
      </Spin>
      <JobForm
        visible={formOpen}
        job={editing}
        loading={saving}
        onCancel={() => {
          setFormOpen(false);
          setEditing(null);
        }}
        onOk={(n, g, s, label) => void save(n, g, s, label, Boolean(editing))}
      />
    </div>
  );
}
