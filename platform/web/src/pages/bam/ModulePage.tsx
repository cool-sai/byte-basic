import { useState } from "react";
import { Button, Message, Spin, Table, Tag, Typography } from "@arco-design/web-react";
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

  const { data: detail, loading, error, refresh } = useRequest(() => api.bamModule(name), {
    refreshDeps: [name],
    onSuccess: (d) => {
      setFilePath((cur) => (d.files.some((f) => f.path === cur) ? cur : d.files[0]?.path || ""));
    },
  });

  const { run: generate, loading: genBusy } = useRequest(() => api.generateBam(name), {
    manual: true,
    onSuccess: (r) => {
      if (r.status === "ok") {
        Message.success("已生成 v" + r.version + "，共 " + r.files.length + " 个文件");
      } else {
        Message.error(r.log || "生成失败");
      }
      void refresh();
    },
    onError: (e) => Message.error(errMsg(e)),
  });

  const { run: download, loading: dlBusy } = useRequest(() => api.downloadBam(name), {
    manual: true,
    onError: (e) => Message.error(errMsg(e)),
  });

  const files = detail?.files || [];
  const cur = files.find((f) => f.path === filePath) || files[0];
  const rpcs = detail?.rpcs || [];
  const yaml = ["endpoint: http://127.0.0.1:8081", "modules:", "  - name: " + name, "    out: gen"].join("\n");

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
                  {detail.module.branch ? " · " + detail.module.branch : ""}
                  {detail.module.gitCommit ? " · " + detail.module.gitCommit.slice(0, 8) : ""}
                  {" · "}
                </>
              ) : null}
              v{detail.module.version}
              {detail.genStatus ? ` · 生成 ${detail.genStatus}` : " · 未生成"}
            </Typography.Text>
          ) : null}
        </div>
        <div className="flex flex-wrap gap-2">
          <Button disabled={!detail} loading={genBusy} onClick={() => generate()}>
            生成代码
          </Button>
          <Button disabled={!detail || detail.genStatus !== "ok"} loading={dlBusy} onClick={() => download()}>
            下载 zip
          </Button>
        </div>
      </div>
      {error ? <Typography.Text type="error">{errMsg(error)}</Typography.Text> : null}
      {detail?.parseError ? <Typography.Text type="error">{detail.parseError}</Typography.Text> : null}
      <div className="rounded border border-solid border-gray-200 p-3">
        <div className="mb-2 flex items-center justify-between gap-2">
          <Typography.Text type="secondary">项目根目录 bam.yaml，然后 go run ./cmd/bam update</Typography.Text>
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
        <div className="mt-1 flex flex-wrap gap-2">
          {files.map((f) => (
            <Button key={f.path} size="small" type={filePath === f.path ? "primary" : "secondary"} onClick={() => setFilePath(f.path)}>
              {f.path}
            </Button>
          ))}
        </div>
        <div className="mt-4 grid grid-cols-1 gap-4 xl:grid-cols-2">
          <pre className="logbox m-0 min-h-[28rem] overflow-auto font-mono text-xs">{cur?.content || "无 proto"}</pre>
          <div>
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
          </div>
        </div>
      </Spin>
    </div>
  );
}
