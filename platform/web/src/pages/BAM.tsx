import { useEffect, useState } from "react";
import { Button, Input, Message, Space, Table, Tag, Typography } from "@arco-design/web-react";
import { api, errMsg, type Field, type IdlView } from "../api";

function fieldList(fs?: Field[] | null) {
  if (!fs || !fs.length) return "—";
  return fs.map((f) => `${f.id}:${f.type} ${f.name}`).join("  ");
}

export default function BAM() {
  const [idls, setIdls] = useState<IdlView[]>([]);
  const [cur, setCur] = useState<IdlView | null>(null);
  const [content, setContent] = useState("");
  const [err, setErr] = useState("");
  const [busy, setBusy] = useState(false);

  async function load(name?: string) {
    setErr("");
    const list = (await api.idls()) || [];
    setIdls(list);
    const pick = name ? list.find((x) => x.name === name) : list[0];
    if (pick) {
      setCur(pick);
      setContent(pick.content || "");
    }
  }
  useEffect(() => {
    load().catch((e) => setErr(errMsg(e)));
  }, []);

  async function save() {
    if (!cur) return;
    setBusy(true);
    setErr("");
    try {
      const saved = await api.saveIdl(cur.name, content);
      Message.success("已保存到 idl/" + cur.name + ".thrift，网关还不会换路由，要去 AGW 发布。");
      await load(cur.name);
      setContent(saved.content);
    } catch (e) {
      setErr(errMsg(e));
    } finally {
      setBusy(false);
    }
  }

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
        <Button type="primary" loading={busy} disabled={!cur} onClick={() => void save()}>
          保存 IDL
        </Button>
      </div>
      {err && <Typography.Text type="error">{err}</Typography.Text>}
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
      <div className="grid grid-cols-1 gap-4 xl:grid-cols-2">
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
    </Space>
  );
}
