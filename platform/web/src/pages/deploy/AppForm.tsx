import { useEffect, useState } from "react";
import { Input, Modal, Select, Typography } from "@arco-design/web-react";
import type { DeployApp, ScmJob, Service } from "../../api";

export default function AppForm({
  visible,
  app,
  jobs,
  services,
  loading,
  onCancel,
  onOk,
}: {
  visible: boolean;
  app?: DeployApp | null;
  jobs: ScmJob[];
  services: Service[];
  loading?: boolean;
  onCancel: () => void;
  onOk: (name: string, scmName: string, compose: string) => void;
}) {
  const [name, setName] = useState("");
  const [scmName, setScmName] = useState("");
  const [compose, setCompose] = useState("");

  useEffect(() => {
    if (!visible) {
      return;
    }
    if (app) {
      setName(app.name);
      setScmName(app.scmName);
      setCompose((app.compose || []).join(","));
    } else {
      setName("");
      setScmName("");
      setCompose("");
    }
  }, [visible, app]);

  const edit = Boolean(app);

  const fillName = (n: string) => {
    setName(n);
    if (edit) {
      return;
    }
    const hit = services.find((s) => s.name === n);
    if (hit) {
      setCompose(hit.compose.join(","));
    }
  };

  return (
    <Modal
      title={edit ? "编辑部署项目" : "新建部署项目"}
      visible={visible}
      onCancel={onCancel}
      onOk={() => onOk(name.trim(), scmName, compose.trim())}
      confirmLoading={loading}
      okButtonProps={{ disabled: !name.trim() || !scmName }}
    >
      <div className="flex w-full flex-col gap-4">
        <Input addBefore="名称" value={name} onChange={fillName} placeholder="user" disabled={edit} />
        <Select
          value={scmName || undefined}
          onChange={(v: string) => {
            setScmName(v);
            const j = jobs.find((x) => x.name === v);
            if (!edit && j && !name) {
              setName(j.name);
            }
            if (!edit && j && !compose) {
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
        <Input addBefore="Compose" value={compose} onChange={setCompose} placeholder="user" />
        <Typography.Text type="secondary">
          Compose 不填时：目录里有同名服务就用它，否则用服务名自己起一份。
        </Typography.Text>
      </div>
    </Modal>
  );
}
