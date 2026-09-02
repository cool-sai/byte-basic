import { useState } from "react";
import { Button, Card, Input, Message, Modal, Select, Spin, Typography } from "@arco-design/web-react";
import { useRequest } from "ahooks";
import { useLocation, useNavigate } from "react-router-dom";
import { api, errMsg } from "../../api";
import LabelIcon from "../scm/LabelIcon";
import Crumbs from "./Crumbs";

export default function AppList() {
  const loc = useLocation();
  const navigate = useNavigate();
  const [name, setName] = useState("");
  const [scmName, setScmName] = useState("");
  const [compose, setCompose] = useState("");
  const [creating, setCreating] = useState(false);

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

  const { runAsync: submit, loading: submitting } = useRequest(
    (n: string, scm: string, c: string) => api.createDeployApp(n, scm, c),
    {
      manual: true,
      onSuccess: (_d, [n]) => {
        Message.success("已新建 " + n);
        setName("");
        setScmName("");
        setCompose("");
        setCreating(false);
        void refresh();
      },
      onError: (e) => Message.error(errMsg(e)),
    },
  );

  const fillName = (n: string) => {
    setName(n);
    const hit = services.find((s) => s.name === n);
    if (hit) {
      setCompose(hit.compose.join(","));
    }
  };

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
        <Button type="primary" onClick={() => setCreating(true)}>
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
            >
              <div className="text-xs text-slate-500">SCM {a.scmName}</div>
              <div className="text-xs text-slate-500">Compose {(a.compose || []).join(", ")}</div>
            </Card>
          ))}
        </div>
      </Spin>
      <Modal
        title="新建部署项目"
        visible={creating}
        onCancel={() => setCreating(false)}
        onOk={() => void submit(name.trim(), scmName, compose.trim())}
        confirmLoading={submitting}
        okButtonProps={{ disabled: !name.trim() || !scmName }}
      >
        <div className="flex w-full flex-col gap-4">
          <Input addBefore="名称" value={name} onChange={fillName} placeholder="user" />
          <Select
            value={scmName || undefined}
            onChange={(v: string) => {
              setScmName(v);
              const j = jobs.find((x) => x.name === v);
              if (j && !name) {
                setName(j.name);
              }
              if (j && !compose) {
                const hit = services.find((s) => s.name === v);
                setCompose(hit ? hit.compose.join(",") : j.name);
              }
            }}
            placeholder="关联 SCM"
            className="w-full"
          >
            {jobs.map((j) => (
              <Select.Option key={j.name} value={j.name}>
                {(j.label || "golang") + " · " + j.name}
              </Select.Option>
            ))}
          </Select>
          <Input addBefore="Compose" value={compose} onChange={setCompose} placeholder="user-1,user-2" />
          <Typography.Text type="secondary">Compose 不填时：目录里有同名服务就用它，否则用服务名自己起一份（node 静态站）。</Typography.Text>
        </div>
      </Modal>
    </div>
  );
}
