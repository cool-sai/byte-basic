import { useEffect, useRef, useState } from "react";
import { Card, Message, Spin, Tag, Typography } from "@arco-design/web-react";
import { useRequest } from "ahooks";
import { useLocation, useParams } from "react-router-dom";
import { api, errMsg } from "../../api";
import LogBox from "../scm/LogBox";
import Crumbs from "./Crumbs";

function statusColor(v: string) {
  if (v === "ok") {
    return "green";
  }
  if (v === "running") {
    return "arcoblue";
  }
  return "red";
}

export default function RunPage() {
  const { name = "", id = "" } = useParams();
  const loc = useLocation();
  const [liveLog, setLiveLog] = useState("");
  const streamed = useRef(false);

  useEffect(() => {
    streamed.current = false;
    setLiveLog("");
  }, [id]);

  const { data: run, loading, error, refresh } = useRequest(() => api.deployDetail(Number(id)), {
    refreshDeps: [id, loc.key],
    onSuccess: (row) => {
      if (row.status !== "running" || streamed.current) {
        return;
      }
      streamed.current = true;
      void api
        .watchDeploy(row.id, (text) => setLiveLog((cur) => cur + text))
        .catch((e) => Message.error(errMsg(e)))
        .then(() => refresh());
    },
  });

  const log = liveLog || run?.log || "";
  const live = run?.status === "running";

  return (
    <div className="flex w-full flex-col gap-6">
      <Crumbs appName={name} version={run?.version} />
      <Spin loading={loading && !run} className="w-full">
        <div>
          <Typography.Title heading={4} className="!mb-1">
            部署 {run?.version || id}
          </Typography.Title>
          <Typography.Text type="secondary">{run?.service}</Typography.Text>
        </div>
        {error ? <Typography.Text type="error">{errMsg(error)}</Typography.Text> : null}
        <div className="mt-3 flex items-center gap-2">
          {run ? <Tag color={statusColor(run.status)}>{run.status}</Tag> : null}
          <Typography.Text type="secondary">{run?.createdAt}</Typography.Text>
        </div>
        <Card title="日志" className="mt-4">
          <LogBox text={log} live={live} />
        </Card>
      </Spin>
    </div>
  );
}
