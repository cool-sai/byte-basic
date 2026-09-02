import { useEffect, useRef, useState } from "react";
import { Card, Message, Spin, Tag, Typography } from "@arco-design/web-react";
import { useRequest } from "ahooks";
import { useLocation, useParams } from "react-router-dom";
import { api, errMsg } from "../../api";
import Crumbs from "./Crumbs";
import LogBox from "./LogBox";

function statusColor(v: string) {
  if (v === "ok") {
    return "green";
  }
  if (v === "running") {
    return "arcoblue";
  }
  return "red";
}

export default function BuildPage() {
  const { name = "", id = "" } = useParams();
  const loc = useLocation();
  const [liveLog, setLiveLog] = useState("");
  const streamed = useRef(false);

  useEffect(() => {
    streamed.current = false;
    setLiveLog("");
  }, [id]);

  const { data: build, loading, error, refresh } = useRequest(() => api.buildDetail(Number(id)), {
    refreshDeps: [id, loc.key],
    onSuccess: (b) => {
      if (b.status !== "running" || streamed.current) {
        return;
      }
      streamed.current = true;
      void api
        .watchBuild(b.id, (text) => setLiveLog((cur) => cur + text))
        .catch((e) => Message.error(errMsg(e)))
        .then(() => refresh());
    },
  });

  const log = liveLog || build?.log || "";
  const live = build?.status === "running";

  return (
    <div className="flex w-full flex-col gap-6">
      <Crumbs jobName={name} version={build?.version} />
      <Spin loading={loading && !build} className="w-full">
        <div>
          <Typography.Title heading={4} className="!mb-1">
            编译 {build?.version || id}
          </Typography.Title>
          <Typography.Text type="secondary">
            {build?.branch || "—"} · {(build?.commit || "").slice(0, 12) || "commit 待写入"}
          </Typography.Text>
        </div>
        {error ? <Typography.Text type="error">{errMsg(error)}</Typography.Text> : null}
        <div className="mt-3 flex items-center gap-2">
          {build ? <Tag color={statusColor(build.status)}>{build.status}</Tag> : null}
          <Typography.Text type="secondary">{build?.createdAt}</Typography.Text>
        </div>
        <Card title="日志" className="mt-4">
          <LogBox text={log} live={live} />
        </Card>
      </Spin>
    </div>
  );
}
