import { Card, Space, Spin, Tag, Typography } from "@arco-design/web-react";
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
    <Space direction="vertical" size="large" className="w-full">
      <Crumbs appName={name} version={run?.version} />
      <Spin loading={loading} className="w-full">
        <div>
          <Typography.Title heading={4} className="!mb-1">
            部署 {run?.version || id}
          </Typography.Title>
          <Typography.Text type="secondary">{run?.service}</Typography.Text>
        </div>
        {error ? <Typography.Text type="error">{errMsg(error)}</Typography.Text> : null}
        <Space className="mt-3">
          {run ? <Tag color={run.status === "ok" ? "green" : "red"}>{run.status}</Tag> : null}
          <Typography.Text type="secondary">{run?.createdAt}</Typography.Text>
        </Space>
        <Card title="日志" className="mt-4">
          <LogBox text={run?.log || ""} />
        </Card>
      </Spin>
    </Space>
  );
}
