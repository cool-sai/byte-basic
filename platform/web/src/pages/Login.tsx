import { useState } from "react";
import { Button, Form, Input, Message } from "@arco-design/web-react";
import { useNavigate } from "react-router-dom";
import { api, errMsg } from "../api";

export default function Login() {
  const navigate = useNavigate();
  const [loading, setLoading] = useState(false);

  return (
    <div className="gate">
      <div className="gate-card">
        <div className="kicker">minikitex</div>
        <h2 className="mb-1 text-xl font-medium tracking-wide">控制台</h2>
        <p className="mb-7 text-sm" style={{ color: "var(--mute)" }}>
          用户名密码登录
        </p>
        <Form
          layout="vertical"
          onSubmit={async (v) => {
            setLoading(true);
            try {
              const d = await api.login(String(v.name || ""), String(v.password || ""));
              localStorage.setItem("token", d.token);
              localStorage.setItem("user", d.name);
              navigate("/scm", { replace: true });
            } catch (e) {
              Message.error(errMsg(e));
            } finally {
              setLoading(false);
            }
          }}
        >
          <Form.Item field="name" label="用户名" required>
            <Input placeholder="admin" autoComplete="username" />
          </Form.Item>
          <Form.Item field="password" label="密码" required>
            <Input.Password placeholder="密码" autoComplete="current-password" />
          </Form.Item>
          <Button htmlType="submit" type="primary" long loading={loading}>
            进入
          </Button>
        </Form>
      </div>
    </div>
  );
}
