import { Breadcrumb } from "@arco-design/web-react";
import { Link } from "react-router-dom";

export default function Crumbs({ appName, version }: { appName?: string; version?: string }) {
  return (
    <Breadcrumb>
      <Breadcrumb.Item>
        <Link to="/deploy">部署</Link>
      </Breadcrumb.Item>
      {appName ? (
        <Breadcrumb.Item>
          <Link to={"/deploy/" + appName}>{appName}</Link>
        </Breadcrumb.Item>
      ) : null}
      {version ? <Breadcrumb.Item>{version}</Breadcrumb.Item> : null}
    </Breadcrumb>
  );
}
