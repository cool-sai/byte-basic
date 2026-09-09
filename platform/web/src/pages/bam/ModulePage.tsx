import { useState } from "react";
import { Button, Message, Select, Spin, Table, Tabs, Tag, Typography } from "@arco-design/web-react";
import { useRequest } from "ahooks";
import { Link, useParams } from "react-router-dom";
import { api, errMsg, type BamField, type BamRpc } from "../../api";
import Crumbs from "./Crumbs";

function fieldList(fs?: BamField[] | null) {
  if (!fs || !fs.length) return "—";
  return fs
    .map((f) => {
      const c = f.comment ? ` // ${f.comment}` : "";
      return `${f.id}:${f.type} ${f.name}${c}`;
    })
    .join("  ");
}

export default function ModulePage() {
  const { name = "" } = useParams();
  const [filePath, setFilePath] = useState("");
  const [ver, setVer] = useState(0);

  const { data: detail, loading, error } = useRequest(() => api.bamModule(name, ver || undefined), {
    refreshDeps: [name, ver],
    pollingInterval: 10000,
    onSuccess: (d) => {
      setFilePath((cur) => (d.files.some((f) => f.path === cur) ? cur : d.files[0]?.path || ""));
    },
  });

  const files = detail?.files || [];
  const cur = files.find((f) => f.path === filePath) || files[0];
  const rpcs = detail?.rpcs || [];
  const versions = detail?.versions || [];
  const yaml = [
    "endpoint: http://127.0.0.1:8081",
    "modules:",
    "  - name: " + name,
    "    out: gen",
    "    lang: go",
    "    branch: " + (detail?.module.branch || "main"),
  ].join("\n");

  return (
    <div className="flex w-full flex-col gap-4">
      <Crumbs name={name} />
      <div className="flex items-start justify-between gap-3">
        <div>
          <Typography.Title heading={4} className="!mb-1">
            {detail?.module.name || name}
          </Typography.Title>
          {detail ? (
            <Typography.Text type="secondary" className="block">
              {detail.module.scmName ? (
                <>
                  仓库 <Link to={"/scm/" + detail.module.scmName}>{detail.module.scmName}</Link>
                  {detail.module.protoDir ? " · " + detail.module.protoDir : ""}
                  {" · "}
                </>
              ) : null}
              {detail.module.branch || "默认分支"}
              {detail.module.gitCommit ? " · " + detail.module.gitCommit.slice(0, 8) : ""}
            </Typography.Text>
          ) : null}
        </div>
        {versions.length ? (
          <Select
            className="w-72"
            value={detail?.module.version}
            onChange={(v: number) => setVer(v)}
          >
            {versions.map((r) => (
              <Select.Option key={r.version} value={r.version}>
                {"v" +
                  r.version +
                  (r.branch ? " · " + r.branch : "") +
                  (r.gitCommit ? " · " + r.gitCommit.slice(0, 8) : "")}
              </Select.Option>
            ))}
          </Select>
        ) : null}
      </div>
      {error ? <Typography.Text type="error">{errMsg(error)}</Typography.Text> : null}
      {detail?.parseError ? <Typography.Text type="error">{detail.parseError}</Typography.Text> : null}
      <div className="rounded border border-solid border-gray-200 p-3">
        <div className="mb-2 flex items-center justify-between gap-2">
          <Typography.Text type="secondary">项目根目录 bam.yaml。任意分支 push 会自动出一个版本。</Typography.Text>
          <Button
            size="small"
            onClick={() => {
              void navigator.clipboard.writeText(yaml).then(
                () => Message.success("已复制 bam.yaml"),
                () => Message.error("复制失败"),
              );
            }}
          >
            复制配置
          </Button>
        </div>
        <pre className="m-0 overflow-auto font-mono text-xs">{yaml}</pre>
      </div>
      <Spin loading={loading} className="w-full">
        <div className="flex min-h-[28rem] gap-4">
          <div className="flex w-56 shrink-0 flex-col gap-1 overflow-auto">
            {files.map((f) => (
              <Button
                key={f.path}
                long
                size="small"
                type={filePath === f.path ? "primary" : "secondary"}
                className="!h-auto !whitespace-normal !text-left"
                onClick={() => setFilePath(f.path)}
              >
                {f.path}
              </Button>
            ))}
          </div>
          <div className="min-w-0 flex-1">
            <Tabs>
              <Tabs.TabPane key="proto" title="约束原文件">
                <pre className="logbox m-0 min-h-[24rem] overflow-auto font-mono text-xs">{cur?.content || "无 proto"}</pre>
              </Tabs.TabPane>
              <Tabs.TabPane key="api" title="API 定义">
                <Typography.Title heading={5}>{rpcs[0]?.service || "—"}</Typography.Title>
                <Table
                  rowKey={(r: BamRpc) => r.service + "/" + r.name}
                  pagination={false}
                  data={rpcs}
                  columns={[
                    {
                      title: "方法",
                      dataIndex: "name",
                      render: (n: string, m: BamRpc) => (
                        <div>
                          <div>
                            {n}
                            {m.stream ? (
                              <Tag color="purple" className="ml-2">
                                stream
                              </Tag>
                            ) : null}
                          </div>
                          {m.comment ? (
                            <Typography.Text type="secondary" className="text-xs">
                              {m.comment}
                            </Typography.Text>
                          ) : null}
                          {m.uri ? (
                            <Typography.Text type="secondary" className="font-mono text-xs">
                              {m.httpMethod} {m.uri}
                            </Typography.Text>
                          ) : (
                            <Typography.Text type="secondary" className="font-mono text-xs">
                              Connect / gRPC
                            </Typography.Text>
                          )}
                        </div>
                      ),
                    },
                    {
                      title: "协议",
                      render: (_: unknown, m: BamRpc) =>
                        m.uri ? <Tag color="cyan">HTTP</Tag> : <Tag color="arcoblue">RPC</Tag>,
                    },
                    {
                      title: "入参",
                      render: (_: unknown, m: BamRpc) => (
                        <div>
                          <b>{m.req}</b>
                          <div className="meta">{fieldList(m.reqFields)}</div>
                        </div>
                      ),
                    },
                    {
                      title: "出参",
                      render: (_: unknown, m: BamRpc) => (
                        <div>
                          <b>{m.resp}</b>
                          <div className="meta">{fieldList(m.respFields)}</div>
                        </div>
                      ),
                    },
                  ]}
                />
              </Tabs.TabPane>
            </Tabs>
          </div>
        </div>
      </Spin>
    </div>
  );
}
