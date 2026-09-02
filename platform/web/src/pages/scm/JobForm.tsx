import { useEffect, useState } from "react";
import { Input, Modal, Select, Typography } from "@arco-design/web-react";
import type { ScmJob } from "../../api";
import LabelIcon from "./LabelIcon";

const LABELS = ["golang", "node"];

export default function JobForm({
  visible,
  job,
  loading,
  onCancel,
  onOk,
}: {
  visible: boolean;
  job?: ScmJob | null;
  loading?: boolean;
  onCancel: () => void;
  onOk: (name: string, gitUrl: string, scriptPath: string, label: string) => void;
}) {
  const [name, setName] = useState("");
  const [gitUrl, setGitUrl] = useState("");
  const [script, setScript] = useState("scripts/scm/user.sh");
  const [label, setLabel] = useState("golang");

  useEffect(() => {
    if (!visible) {
      return;
    }
    if (job) {
      setName(job.name);
      setGitUrl(job.gitUrl);
      setScript(job.scriptPath);
      setLabel(job.label || "golang");
    } else {
      setName("");
      setGitUrl("");
      setScript("scripts/scm/user.sh");
      setLabel("golang");
    }
  }, [visible, job]);

  const edit = Boolean(job);

  return (
    <Modal
      title={edit ? "编辑编译任务" : "新建编译任务"}
      visible={visible}
      onCancel={onCancel}
      onOk={() => onOk(name.trim(), gitUrl.trim(), script.trim(), label)}
      confirmLoading={loading}
      okButtonProps={{ disabled: !name.trim() || !gitUrl.trim() || !script.trim() || !label }}
    >
      <div className="flex w-full flex-col gap-4">
        <Input addBefore="名称" value={name} onChange={setName} placeholder="user" disabled={edit} />
        <Input addBefore="Git" value={gitUrl} onChange={setGitUrl} placeholder="https://github.com/coolCicada/byte-basic.git" />
        <Input addBefore="脚本" value={script} onChange={setScript} placeholder="scripts/scm/user.sh" />
        <Select className="w-full" value={label} onChange={setLabel} placeholder="label">
          {LABELS.map((x) => (
            <Select.Option key={x} value={x}>
              <span className="inline-flex items-center gap-2">
                <LabelIcon label={x} size={16} />
                {x}
              </span>
            </Select.Option>
          ))}
        </Select>
        <Typography.Text type="secondary">
          golang = 服务二进制；node = 平台前端。示例脚本：scripts/scm/user.sh、order.sh、gateway.sh、etcdui.sh、platform-api.sh、platform-web.sh
        </Typography.Text>
      </div>
    </Modal>
  );
}
