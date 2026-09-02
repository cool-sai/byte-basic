import { useEffect, useState } from "react";
import { Button, Input, Message, Space, Spin, Table, Tag, Typography } from "@arco-design/web-react";
import { useRequest } from "ahooks";
import { api, errMsg, type Field, type IdlView } from "../api";

function fieldList(fs?: Field[] | null) {
  if (!fs || !fs.length) return "—";
  return fs.map((f) => `${f.id}:${f.type} ${f.name}`).join("  ");
}

export default function BAM() {
  const [cur, setCur] = useState<IdlView | null>(null);
  const [content, setContent] = useState("");

  const { data: idls = [], loading, error, refresh } = useRequest(async () => (await api.idls()) || []);

  useEffect(() => {
    if (!idls.length) {
      return;
    }
    const pick = cur ? idls.find((x) => x.name === cur.name) : idls[0];
    if (pick && pick.name !== cur?.name) {
      setCur(pick);
      setContent(pick.content || "");
    }
  }, [idls, cur]);

  const { run: save, loading: busy } = useRequest((name: string, body: string) => api.saveIdl(name, body), {
    manual: true,
    onSuccess: (saved) => {
      Message.success("已保存到 idl/" + saved.name + ".thrift，网关还不会换路由，要去 AGW 发布。");
      setContent(saved.content);
      void refresh();
    },
    onError: (e) => Message.error(errMsg(e)),
  });

  const methods = cur?.methods || [];

  return (
    <Space direction="vertical" size="medium" className="w-full">
      <div className="flex items-start justify-between gap-3">
        <div>
          <Typography.Title heading={4} className="!mb-1">
            BAM IDL
          </Typography.Title>
          <Typography.Text type="secondary">托管 Thrift 契约。带 agw.uri 的方法会显示成 HTTP 入参 / 出参；没标的只走 RPC。</Typography.Text>
        </div>
        <Button type="primary" loading={busy} disabled={!cur} onClick={() => cur && save(cur.name, content)}>
          保存 IDL
        </Button>
      </div>
      {error ? <Typography.Text type="error">{errMsg(error)}</Typography.Text> : null}
      <Spin loading={loading} className="w-full">
        <Space wrap>
          {idls.map((x) => (
            <Button
              key={x.name}
              type={cur?.name === x.name ? "primary" : "secondary"}
              onClick={() => {
                setCur(x);
                setContent(x.content || "");
              }}
            >
              {x.name}.thrift {x.httpApis ? `(HTTP ${x.httpApis})` : "(RPC)"}
            </Button>
          ))}
        </Space>
        <div className="mt-4 grid grid-cols-1 gap-4 xl:grid-cols-2">
          <Input.TextArea
            value={content}
            onChange={setContent}
            autoSize={{ minRows: 22 }}
            className="font-mono text-xs"
          />
          <div>
            <Typography.Title heading={5}>{cur?.service || "—"}</Typography.Title>
            {cur?.parseError && <Typography.Text type="error">{cur.parseError}</Typography.Text>}
            <Table
              rowKey="name"
              pagination={false}
              data={methods}
              columns={[
                {
                  title: "方法",
                  dataIndex: "name",
                  render: (name: string, m: (typeof methods)[0]) => (
                    <div>
                      <div>{name}</div>
                      {m.uri ? (
                        <Typography.Text type="secondary" className="font-mono text-xs">
                          {m.httpMethod} {m.uri}
                        </Typography.Text>
                      ) : null}
                    </div>
                  ),
                },
                {
                  title: "协议",
                  render: (_: unknown, m: (typeof methods)[0]) =>
                    m.uri ? <Tag color="cyan">HTTP</Tag> : <Tag color="arcoblue">RPC</Tag>,
                },
                {
                  title: "入参",
                  render: (_: unknown, m: (typeof methods)[0]) => (
                    <div>
                      <b>{m.req}</b>
                      <div className="text-slate-500">{fieldList(m.reqFields)}</div>
                    </div>
                  ),
                },
                {
                  title: "出参",
                  render: (_: unknown, m: (typeof methods)[0]) => (
                    <div>
                      <b>{m.resp}</b>
                      <div className="text-slate-500">{fieldList(m.respFields)}</div>
                    </div>
                  ),
                },
              ]}
            />
          </div>
        </div>
      </Spin>
    </Space>
  );
}
