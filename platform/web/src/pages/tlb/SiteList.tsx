import { useState } from "react";
import { Button, Card, Input, Message, Modal, Spin, Typography } from "@arco-design/web-react";
import { useRequest } from "ahooks";
import { useLocation, useNavigate } from "react-router-dom";
import { api, errMsg, type TlbSite } from "../../api";
import Crumbs from "./Crumbs";

export default function SiteList() {
  const loc = useLocation();
  const navigate = useNavigate();
  const [open, setOpen] = useState(false);
  const [host, setHost] = useState("");

  const { data, loading, error, refresh } = useRequest(
    async () => {
      const d = await api.tlbSites();
      return { sites: d.sites || [], zone: d.zone || "ls-byte-basic.com" };
    },
    { refreshDeps: [loc.key] },
  );
  const sites = data?.sites || [];
  const zone = data?.zone || "ls-byte-basic.com";

  const { runAsync: create, loading: saving } = useRequest((h: string) => api.createTlbSite(h), {
    manual: true,
    onSuccess: (d) => {
      Message.success("已新建 " + d.host);
      setOpen(false);
      setHost("");
      void refresh();
    },
    onError: (e) => Message.error(errMsg(e)),
  });

  const { runAsync: remove } = useRequest((n: string) => api.deleteTlbSite(n), {
    manual: true,
    onSuccess: (_d, [n]) => {
      Message.success("已删除 " + n);
      void refresh();
    },
    onError: (e) => Message.error(errMsg(e)),
  });

  const { run: publish, loading: busy } = useRequest(() => api.publishTlb(), {
    manual: true,
    onSuccess: (d) => {
      Message.success("已发布 " + d.sites + " 份配置，nginx 已 reload");
      void refresh();
    },
    onError: (e) => Message.error(errMsg(e)),
  });

  return (
    <div className="flex w-full flex-col gap-6">
      <Crumbs />
      <div className="flex items-start justify-between gap-3">
        <div>
          <Typography.Title heading={4} className="!mb-1">
            TLB 配置
          </Typography.Title>
          <Typography.Text type="secondary">
            一份配置对应一个域名（nginx server_name）。完整域名直接填，例如 lssl.work；只填一段会拼成 xxx.{zone}。没匹配到的 Host 走第一份配置。
          </Typography.Text>
        </div>
        <div className="flex items-center gap-2">
          <Button onClick={() => setOpen(true)}>新建配置</Button>
          <Button type="primary" loading={busy} onClick={() => publish()}>
            发布到 TLB
          </Button>
        </div>
      </div>
      {error ? <Typography.Text type="error">{errMsg(error)}</Typography.Text> : null}

      <Spin loading={loading} className="w-full">
        <div className="grid grid-cols-1 gap-3 md:grid-cols-2">
          {sites.map((s: TlbSite) => (
            <Card
              key={s.name}
              title={s.host}
              hoverable
              className="cursor-pointer"
              onClick={() => navigate("/tlb/" + s.name)}
              extra={
                <Button
                  size="mini"
                  status="danger"
                  onClick={(e) => {
                    e.stopPropagation();
                    Modal.confirm({
                      title: "删除配置 " + s.host + "？",
                      content: "下面的路径规则一起删。要点发布才从 nginx 拿掉。",
                      okButtonProps: { status: "danger" },
                      onOk: () => remove(s.name),
                    });
                  }}
                >
                  删除
                </Button>
              }
            >
              <div className="meta">名称 {s.name}</div>
              <div className="meta">路由 {s.routes} 条</div>
            </Card>
          ))}
        </div>
      </Spin>

      <Modal
        title="新建配置"
        visible={open}
        onCancel={() => setOpen(false)}
        onOk={() => void create(host.trim())}
        confirmLoading={saving}
        okButtonProps={{ disabled: !host.trim() }}
      >
        <div className="flex w-full flex-col gap-4">
          <Input addBefore="域名" value={host} onChange={setHost} placeholder={"lssl.work 或 order." + zone} />
          <Typography.Text type="secondary">
            {host.includes(".") || !host.trim()
              ? "点进配置里再加路径转到哪个服务。"
              : `会生成 ${host.trim()}.${zone}，点进配置里再加路径转到哪个服务。`}
          </Typography.Text>
        </div>
      </Modal>
    </div>
  );
}
