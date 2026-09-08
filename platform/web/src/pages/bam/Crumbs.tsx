import { Breadcrumb } from "@arco-design/web-react";
import { Link } from "react-router-dom";

export default function Crumbs({ name }: { name?: string }) {
  return (
    <Breadcrumb>
      <Breadcrumb.Item>
        <Link to="/bam">BAM</Link>
      </Breadcrumb.Item>
      {name ? (
        <Breadcrumb.Item>
          <Link to={"/bam/" + name}>{name}</Link>
        </Breadcrumb.Item>
      ) : null}
    </Breadcrumb>
  );
}
