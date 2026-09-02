import { Card, Spin, Tag, Typography } from "@arco-design/web-react";
import { useRequest } from "ahooks";
import { useLocation, useParams } from "react-router-dom";
import { api, errMsg } from "../../api";
import LogBox from "../scm/LogBox";
import Crumbs from "./Crumbs";

export default function RunPage() {
  const { name = "", id = "" } = useParams();
  const loc = useLocation();
  const { data: run, loading, error } = useRequest(() => api.deployDetail(Number(id)), {
    refreshDeps: [id, loc.key],
  });

  return (
    <div className="flex w-full flex-col gap-6">
      <Crumbs appName={name} version={run?.version} />
      <Spin loading={loading} className="w-full">
        <div>
          <Typography.Title heading={4} className="!mb-1">
            部署 {run?.version || id}
          </Typography.Title>
          <Typography.Text type="secondary">{run?.service}</Typography.Text>
        </div>
        {error ? <Typography.Text type="error">{errMsg(error)}</Typography.Text> : null}
        <div className="mt-3 flex items-center gap-2">
          {run ? <Tag color={run.status === "ok" ? "green" : "red"}>{run.status}</Tag> : null}
          <Typography.Text type="secondary">{run?.createdAt}</Typography.Text>
        </div>
        <Card title="日志" className="mt-4">
          <LogBox text={run?.log || ""} />
        </Card>
      </Spin>
    </div>
  );
}
