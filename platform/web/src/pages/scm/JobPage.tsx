import { useState } from "react";
import { Button, Message, Modal, Select, Table, Tag, Typography } from "@arco-design/web-react";
import { useRequest } from "ahooks";
import { useLocation, useNavigate, useParams } from "react-router-dom";
import { api, errMsg, type Build } from "../../api";
import Crumbs from "./Crumbs";
import JobForm from "./JobForm";
import LabelIcon from "./LabelIcon";

function statusColor(v: string) {
  if (v === "ok") {
    return "green";
  }
  if (v === "running") {
    return "arcoblue";
  }
  return "red";
}

export default function JobPage() {
  const { name = "" } = useParams();
  const loc = useLocation();
  const navigate = useNavigate();
  const [open, setOpen] = useState(false);
  const [editOpen, setEditOpen] = useState(false);
  const [branch, setBranch] = useState("");

  const { data, loading, error, refresh } = useRequest(
    async () => ({
      job: await api.scmJob(name),
      builds: (await api.builds(name)) || [],
    }),
    { refreshDeps: [name, loc.key] },
  );
  const job = data?.job;
  const builds = data?.builds || [];

  const { data: br, loading: brLoading, error: brErr } = useRequest(() => api.branches(name), {
    ready: open,
    refreshDeps: [name, open],
    onSuccess: (d) => {
      const first = d.default || d.branches?.[0]?.name || "";
      setBranch((cur) => (d.branches || []).some((x) => x.name === cur) ? cur : first);
    },
  });
  const branches = br?.branches || [];

  const { runAsync: start, loading: starting } = useRequest((b: string) => api.createBuild(name, b), {
    manual: true,
    onSuccess: (row) => {
      setOpen(false);
      navigate("/scm/" + name + "/builds/" + row.id);
    },
    onError: (e) => Message.error(errMsg(e)),
  });

  return (
    <div className="flex w-full flex-col gap-6">
      <Crumbs jobName={name} />
      <div className="flex items-start justify-between gap-3">
        <div>
          <Typography.Title heading={4} className="!mb-1 inline-flex items-center gap-2">
            <LabelIcon label={job?.label} />
            {job?.name || name}
          </Typography.Title>
          <div className="meta">Git {job?.gitUrl}</div>
          <div className="meta">脚本 {job?.scriptPath}</div>
        </div>
        <div className="flex items-center gap-2">
          <Button disabled={!job} onClick={() => setEditOpen(true)}>
            编辑
          </Button>
          <Button
            status="danger"
            onClick={() => {
              Modal.confirm({
                title: "删除任务 " + name + "？",
                content: "编译记录、clone、产物都会删掉。",
                okButtonProps: { status: "danger" },
                onOk: () =>
                  api.deleteScmJob(name).then(
                    () => {
                      Message.success("已删除 " + name);
                      navigate("/scm");
                    },
                    (e) => Message.error(errMsg(e)),
                  ),
              });
            }}
          >
            删除
          </Button>
          <Button type="primary" onClick={() => setOpen(true)}>
            编译
          </Button>
        </div>
      </div>
      {error ? <Typography.Text type="error">{errMsg(error)}</Typography.Text> : null}

      <Typography.Title heading={5}>编译历史</Typography.Title>
      <Table
        rowKey="id"
        pagination={false}
        loading={loading}
        data={builds}
        onRow={(row) => ({
          onClick: () => navigate("/scm/" + name + "/builds/" + (row as Build).id),
          style: { cursor: "pointer" },
        })}
        columns={[
          { title: "版本", dataIndex: "version" },
          { title: "分支", dataIndex: "branch", render: (v: string) => v || "—" },
          {
            title: "Commit",
            dataIndex: "commit",
            render: (v: string) => (v ? v.slice(0, 7) : "—"),
          },
          {
            title: "状态",
            dataIndex: "status",
            render: (v: string) => <Tag color={statusColor(v)}>{v}</Tag>,
          },
          { title: "时间", dataIndex: "createdAt" },
        ]}
      />
      <JobForm
        visible={editOpen}
        job={job}
        onCancel={() => setEditOpen(false)}
        onOk={(n, g, s, label) => {
          void api.updateScmJob(n, g, s, label).then(
            () => {
              Message.success("已保存");
              setEditOpen(false);
              void refresh();
            },
            (e) => Message.error(errMsg(e)),
          );
        }}
      />
      <Modal
        title="选择分支"
        visible={open}
        onCancel={() => setOpen(false)}
        onOk={() => void start(branch)}
        confirmLoading={starting}
        okButtonProps={{ disabled: !branch }}
      >
        {brErr ? <Typography.Text type="error">{errMsg(brErr)}</Typography.Text> : null}
        <Select
          className="w-full"
          loading={brLoading}
          value={branch || undefined}
          onChange={setBranch}
          placeholder="选择要编译的分支"
          showSearch
        >
          {branches.map((b) => (
            <Select.Option key={b.name} value={b.name}>
              {b.name}
            </Select.Option>
          ))}
        </Select>
      </Modal>
    </div>
  );
}
