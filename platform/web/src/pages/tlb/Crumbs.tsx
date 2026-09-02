import { Breadcrumb } from "@arco-design/web-react";
import { Link } from "react-router-dom";

export default function Crumbs({ siteName }: { siteName?: string }) {
  return (
    <Breadcrumb>
      <Breadcrumb.Item>
        <Link to="/tlb">TLB</Link>
      </Breadcrumb.Item>
      {siteName ? (
        <Breadcrumb.Item>
          <Link to={"/tlb/" + siteName}>{siteName}</Link>
        </Breadcrumb.Item>
      ) : null}
    </Breadcrumb>
  );
}
