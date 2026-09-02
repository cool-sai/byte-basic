import { useState } from "react";
import { Button, Input, Message, Modal, Select, Table, Typography } from "@arco-design/web-react";
import { useRequest } from "ahooks";
import { useLocation, useParams } from "react-router-dom";
import { api, errMsg, type TlbRoute } from "../../api";
import Crumbs from "./Crumbs";

export default function SitePage() {
  const { name = "" } = useParams();
  const loc = useLocation();
  const [open, setOpen] = useState(false);
  const [editing, setEditing] = useState<TlbRoute | null>(null);
  const [routeName, setRouteName] = useState("");
  const [path, setPath] = useState("");
  const [target, setTarget] = useState("");

  const { data, loading, error, refresh } = useRequest(() => api.tlbSite(name), {
    refreshDeps: [name, loc.key],
  });
  const { data: upData } = useRequest(() => api.tlbUpstreams());
  const routes = data?.routes || [];
  const host = data?.host || name + ".ls-byte-basic.com";
  const upstreams = upData?.upstreams || [];
  const targetOpts = target && !upstreams.some((u) => u.target === target)
    ? [{ name: target, target }, ...upstreams]
    : upstreams;

  const { runAsync: save, loading: saving } = useRequest(
    (n: string, p: string, t: string, row: TlbRoute | null) =>
      row ? api.updateTlbRoute(name, row.id, n, p, t) : api.createTlbRoute(name, n, p, t),
    {
      manual: true,
      onSuccess: () => {
        Message.success("已保存");
        setOpen(false);
        setEditing(null);
        void refresh();
      },
      onError: (e) => Message.error(errMsg(e)),
    },
  );

  const { runAsync: remove } = useRequest((id: number) => api.deleteTlbRoute(name, id), {
    manual: true,
    onSuccess: () => {
      Message.success("已删除");
      void refresh();
    },
    onError: (e) => Message.error(errMsg(e)),
  });

  const { run: publish, loading: busy } = useRequest(() => api.publishTlb(), {
    manual: true,
    onSuccess: (d) => Message.success("已发布 " + d.sites + " 份配置"),
    onError: (e) => Message.error(errMsg(e)),
  });

  const openCreate = () => {
    setEditing(null);
    setRouteName("");
    setPath("");
    setTarget("");
    setOpen(true);
  };

  const openEdit = (r: TlbRoute) => {
    setEditing(r);
    setRouteName(r.name);
    setPath(r.path);
    setTarget(r.target);
    setOpen(true);
  };

  return (
    <div className="flex w-full flex-col gap-4">
      <Crumbs siteName={name} />
      <div className="flex items-start justify-between gap-3">
        <div>
          <Typography.Title heading={4} className="!mb-1">
            {host}
          </Typography.Title>
          <Typography.Text type="secondary">
            这个域名下的路径转到哪些服务。改完回列表或这里点发布。
          </Typography.Text>
        </div>
        <div className="flex items-center gap-2">
          <Button onClick={openCreate}>新建路由</Button>
          <Button type="primary" loading={busy} onClick={() => publish()}>
            发布到 TLB
          </Button>
        </div>
      </div>
      {error ? <Typography.Text type="error">{errMsg(error)}</Typography.Text> : null}

      <Table
        rowKey="id"
        pagination={false}
        loading={loading}
        data={routes}
        columns={[
          { title: "名称", dataIndex: "name" },
          { title: "路径", dataIndex: "path", render: (v: string) => <span className="font-mono">{v}</span> },
          { title: "转到", dataIndex: "target", render: (v: string) => <span className="font-mono">{v}</span> },
          {
            title: "",
            width: 160,
            render: (_: unknown, r: TlbRoute) => (
              <div className="flex items-center gap-2">
                <Button size="mini" onClick={() => openEdit(r)}>
                  编辑
                </Button>
                <Button
                  size="mini"
                  status="danger"
                  onClick={() => {
                    Modal.confirm({
                      title: "删除路由 " + r.path + "？",
                      content: "要点发布才从 nginx 拿掉。",
                      okButtonProps: { status: "danger" },
                      onOk: () => remove(r.id),
                    });
                  }}
                >
                  删除
                </Button>
              </div>
            ),
          },
        ]}
      />
      <Typography.Text type="secondary" className="font-mono">
        {`curl -s -H 'Host: ${host}' http://127.0.0.1/`}
      </Typography.Text>

      <Modal
        title={editing ? "编辑路由" : "新建路由"}
        visible={open}
        onCancel={() => setOpen(false)}
        onOk={() => void save(routeName.trim(), path.trim(), target.trim(), editing)}
        confirmLoading={saving}
        okButtonProps={{ disabled: !path.trim() || !target.trim() }}
      >
        <div className="flex w-full flex-col gap-4">
          <Input addBefore="名称" value={routeName} onChange={setRouteName} placeholder="agent-web" />
          <Input addBefore="路径" value={path} onChange={setPath} placeholder="/" />
          <Select
            className="w-full"
            value={target || undefined}
            onChange={setTarget}
            placeholder="选择服务"
            showSearch
          >
            {targetOpts.map((u) => (
              <Select.Option key={u.target} value={u.target}>
                {u.name}
              </Select.Option>
            ))}
          </Select>
        </div>
      </Modal>
    </div>
  );
}
