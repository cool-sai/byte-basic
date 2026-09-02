import { useState } from "react";
import { Button, Card, Input, Message, Modal, Select, Space, Spin, Typography } from "@arco-design/web-react";
import { useRequest } from "ahooks";
import { useLocation, useNavigate } from "react-router-dom";
import { api, errMsg } from "../../api";
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
    <Space direction="vertical" size="large" className="w-full">
      <Crumbs />
      <div className="flex items-start justify-between gap-3">
        <div>
          <Typography.Title heading={4} className="!mb-1">
            部署项目
          </Typography.Title>
          <Typography.Text type="secondary">
            先建项目并关联 SCM，点进去选产物打镜像启动。名称要对上 compose 镜像：user / order / gateway / etcdui。
          </Typography.Text>
        </div>
        <Button type="primary" onClick={() => setCreating(true)}>
          新建项目
        </Button>
      </div>
      {error ? <Typography.Text type="error">{errMsg(error)}</Typography.Text> : null}

      <Modal
        title="新建部署项目"
        visible={creating}
        onCancel={() => setCreating(false)}
        onOk={() => void submit(name.trim(), scmName, compose.trim())}
        confirmLoading={submitting}
        okButtonProps={{ disabled: !name.trim() || !scmName }}
      >
        <Space direction="vertical" className="w-full" size="medium">
          <Input addBefore="名称" value={name} onChange={fillName} placeholder="user" />
          <Select value={scmName || undefined} onChange={setScmName} placeholder="关联 SCM" className="w-full">
            {jobs.map((j) => (
              <Select.Option key={j.name} value={j.name}>
                {j.name}
              </Select.Option>
            ))}
          </Select>
          <Input addBefore="Compose" value={compose} onChange={setCompose} placeholder="user-1,user-2" />
          <Typography.Text type="secondary">Compose 不填时：名称能对上目录就用目录里的服务，否则就起同名服务。</Typography.Text>
        </Space>
      </Modal>

      <Spin loading={loading} className="w-full">
        <div className="grid grid-cols-1 gap-3 md:grid-cols-2">
          {apps.map((a) => (
            <Card key={a.name} title={a.name} hoverable className="cursor-pointer" onClick={() => navigate("/deploy/" + a.name)}>
              <div className="text-xs text-slate-500">SCM {a.scmName}</div>
              <div className="text-xs text-slate-500">Compose {(a.compose || []).join(", ")}</div>
            </Card>
          ))}
        </div>
      </Spin>
    </Space>
  );
}
