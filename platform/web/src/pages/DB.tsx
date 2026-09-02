import { useEffect, useState } from "react";
import { Space, Table, Typography } from "@arco-design/web-react";
import { useRequest } from "ahooks";
import { api, errMsg, type DbTable } from "../api";

export default function DB() {
  const [cur, setCur] = useState("");

  const {
    data: tables = [],
    loading: tablesLoading,
    error: tablesErr,
  } = useRequest(async () => (await api.tables()) || []);

  useEffect(() => {
    if (!cur && tables[0]) {
      setCur(tables[0].name);
    }
  }, [tables, cur]);

  const {
    data: detail,
    loading: detailLoading,
    error: detailErr,
  } = useRequest(() => api.table(cur), { ready: !!cur, refreshDeps: [cur] });

  const cols = detail?.columns || [];
  const preview = (detail?.preview || []).map((row, i) => ({ ...row, _i: i }));
  const keys = preview[0] ? Object.keys(preview[0]).filter((k) => k !== "_i") : [];
  const err = tablesErr || detailErr;

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
      {err ? <Typography.Text type="error">{errMsg(err)}</Typography.Text> : null}
      <div className="grid grid-cols-1 gap-4 xl:grid-cols-[320px_1fr]">
        <Table
          rowKey="name"
          pagination={false}
          loading={tablesLoading}
          data={tables}
          onRow={(t: DbTable) => ({
            onClick: () => setCur(t.name),
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
            loading={detailLoading}
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
            loading={detailLoading}
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
