import os

from flask import Flask, jsonify

app = Flask(__name__)


def ok(data):
    return jsonify(error="", data=data)


@app.get("/api/hello")
@app.get("/python/api/hello")
def hello():
    return ok({"service": "agent-python", "msg": "hello from agent-python"})


@app.get("/health")
def health():
    return ok("ok")


if __name__ == "__main__":
    listen = os.environ.get("LISTEN", "0.0.0.0:80")
    host, port = listen.rsplit(":", 1)
    app.run(host=host, port=int(port))
