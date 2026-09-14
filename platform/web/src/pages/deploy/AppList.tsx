import { useState } from "react";
import { Button, Card, Message, Modal, Spin, Typography } from "@arco-design/web-react";
import { useRequest } from "ahooks";
import { useLocation, useNavigate } from "react-router-dom";
import { api, errMsg, type DeployApp } from "../../api";
import LabelIcon from "../scm/LabelIcon";
import AppForm from "./AppForm";
import Crumbs from "./Crumbs";

export default function AppList() {
  const loc = useLocation();
  const navigate = useNavigate();
  const [formOpen, setFormOpen] = useState(false);
  const [editing, setEditing] = useState<DeployApp | null>(null);

  const { data, loading, error, refresh } = useRequest(
    async () => ({
      apps: (await api.deployApps()).apps || [],
      jobs: (await api.scmJobs()).jobs || [],
      services: (await api.services()) || [],
    }),
    { refreshDeps: [loc.key] },
  );
  const apps = data?.apps || [];
  const jobs = data?.jobs || [];
  const services = data?.services || [];

  const { runAsync: remove } = useRequest((n: string) => api.deleteDeployApp(n), {
    manual: true,
    onSuccess: (_d, [n]) => {
      Message.success("已删除 " + n);
      void refresh();
    },
    onError: (e) => Message.error(errMsg(e)),
  });

  const { runAsync: save, loading: saving } = useRequest(
    (n: string, scm: string, c: string, isEdit: boolean) =>
      isEdit ? api.updateDeployApp(n, scm, c) : api.createDeployApp(n, scm, c),
    {
      manual: true,
      onSuccess: (_d, [n, _scm, _c, isEdit]) => {
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
            部署项目
          </Typography.Title>
          <Typography.Text type="secondary">
            注册服务名并关联 SCM。golang 产物当二进制启动；node 产物打成 nginx 静态站点；python 产物跑 Flask。不在 compose 里的服务，部署时会注册进去。
          </Typography.Text>
        </div>
        <Button
          type="primary"
          onClick={() => {
            setEditing(null);
            setFormOpen(true);
          }}
        >
          新建项目
        </Button>
      </div>
      {error ? <Typography.Text type="error">{errMsg(error)}</Typography.Text> : null}

      <Spin loading={loading} className="w-full">
        <div className="grid grid-cols-1 gap-3 md:grid-cols-2">
          {apps.map((a) => (
            <Card
              key={a.name}
              title={
                <span className="inline-flex items-center gap-2">
                  <LabelIcon label={a.label} />
                  {a.name}
                </span>
              }
              hoverable
              className="cursor-pointer"
              onClick={() => navigate("/deploy/" + a.name)}
              extra={
                <div className="flex items-center gap-2">
                  <Button
                    size="mini"
                    onClick={(e) => {
                      e.stopPropagation();
                      setEditing(a);
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
                        title: "删除项目 " + a.name + "？",
                        content: "部署记录会删掉。不在主 compose 里的容器会停掉。",
                        okButtonProps: { status: "danger" },
                        onOk: () => remove(a.name),
                      });
                    }}
                  >
                    删除
                  </Button>
                </div>
              }
            >
              <div className="meta">SCM {a.scmName}</div>
              <div className="meta">Compose {(a.compose || []).join(", ")}</div>
            </Card>
          ))}
        </div>
      </Spin>
      <AppForm
        visible={formOpen}
        app={editing}
        jobs={jobs}
        services={services}
        loading={saving}
        onCancel={() => {
          setFormOpen(false);
          setEditing(null);
        }}
        onOk={(n, scm, c) => void save(n, scm, c, Boolean(editing))}
      />
    </div>
  );
}
