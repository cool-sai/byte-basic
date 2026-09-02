import { Breadcrumb } from "@arco-design/web-react";
import { Link } from "react-router-dom";

export default function Crumbs({ jobName, version }: { jobName?: string; version?: string }) {
  return (
    <Breadcrumb>
      <Breadcrumb.Item>
        <Link to="/scm">SCM</Link>
      </Breadcrumb.Item>
      {jobName ? (
        <Breadcrumb.Item>
          <Link to={"/scm/" + jobName}>{jobName}</Link>
        </Breadcrumb.Item>
      ) : null}
      {version ? <Breadcrumb.Item>{version}</Breadcrumb.Item> : null}
    </Breadcrumb>
  );
}
