import { useEffect, useState } from "react";
import { Space, Table, Typography } from "@arco-design/web-react";
import { api, errMsg, type DbTable, type TableDetail } from "../api";

export default function DB() {
  const [tables, setTables] = useState<DbTable[]>([]);
  const [cur, setCur] = useState("");
  const [detail, setDetail] = useState<TableDetail | null>(null);
  const [err, setErr] = useState("");

  async function load() {
    setErr("");
    const list = (await api.tables()) || [];
    setTables(list);
    const pick = cur && list.find((t) => t.name === cur) ? cur : list[0]?.name;
    if (pick) {
      setCur(pick);
      setDetail(await api.table(pick));
    }
  }
  useEffect(() => {
    load().catch((e) => setErr(errMsg(e)));
  }, []);

  async function open(name: string) {
    setCur(name);
    setErr("");
    try {
      setDetail(await api.table(name));
    } catch (e) {
      setErr(errMsg(e));
    }
  }

  const cols = detail?.columns || [];
  const preview = (detail?.preview || []).map((row, i) => ({ ...row, _i: i }));
  const keys = preview[0] ? Object.keys(preview[0]).filter((k) => k !== "_i") : [];

  return (
    <Space direction="vertical" size="medium" className="w-full">
      <div>
        <Typography.Title heading={4} className="!mb-1">
          MySQL
        </Typography.Title>
        <Typography.Text type="secondary">
          看 compose 里 minikitex 库的表结构和前 20 行。更完整的客户端用 DBeaver / Sequel Ace 连 127.0.0.1:3306。
        </Typography.Text>
      </div>
      {err && <Typography.Text type="error">{err}</Typography.Text>}
      <div className="grid grid-cols-1 gap-4 xl:grid-cols-[320px_1fr]">
        <Table
          rowKey="name"
          pagination={false}
          data={tables}
          onRow={(t) => ({
            onClick: () => void open(t.name),
            className: t.name === cur ? "bg-cyan-50" : "cursor-pointer",
          })}
          columns={[
            { title: "表", dataIndex: "name" },
            { title: "大约行数", dataIndex: "rows", render: (v: unknown) => v ?? "—" },
            { title: "引擎", dataIndex: "engine" },
          ]}
        />
        <Space direction="vertical" className="w-full">
          <Table
            rowKey="name"
            pagination={false}
            data={cols}
            columns={[
              { title: "列", dataIndex: "name" },
              { title: "类型", dataIndex: "type" },
              { title: "空", dataIndex: "nullable" },
              { title: "键", dataIndex: "key", render: (v: string) => v || "—" },
              { title: "默认", dataIndex: "default", render: (v: string | null) => v ?? "—" },
              { title: "额外", dataIndex: "extra", render: (v: string) => v || "—" },
            ]}
          />
          <Table
            rowKey="_i"
            pagination={false}
            data={preview}
            noDataElement="没有数据"
            columns={keys.map((k) => ({
              title: k,
              dataIndex: k,
              render: (v: unknown) => (v == null ? "NULL" : String(v)),
            }))}
          />
        </Space>
      </div>
    </Space>
  );
}
