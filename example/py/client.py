"""Generic client in Python: no generated stubs.

Field ids and types come from idl/user.thrift — same contract the Go server uses.
"""
from __future__ import annotations

import json
import socket
import struct
import sys

TSTOP, TBOOL, TI64, TSTRING = 0, 1, 2, 3
MSG_CALL, MSG_REPLY, MSG_EX = 1, 2, 3

# Runtime IDL, same idea as generic.Encode walking the Spec.
IDL = {
    "GetUser": {
        "req": [("id", 1, "i64")],
        "resp": [("id", 1, "i64"), ("name", 2, "string")],
    },
    "Ping": {
        "req": [],
        "resp": [("msg", 1, "string")],
    },
}


def encode_struct(fields, data: dict) -> bytes:
    out = bytearray()
    for name, fid, typ in fields:
        if name not in data or data[name] is None:
            continue
        v = data[name]
        if typ == "i64":
            out += struct.pack(">BHq", TI64, fid, int(v))
        elif typ == "string":
            b = str(v).encode()
            out += struct.pack(">BHI", TSTRING, fid, len(b)) + b
        elif typ == "bool":
            out += struct.pack(">BHB", TBOOL, fid, 1 if v else 0)
        else:
            raise ValueError(typ)
    out.append(TSTOP)
    return bytes(out)


def decode_struct(fields, body: bytes) -> dict:
    by_id = {fid: (name, typ) for name, fid, typ in fields}
    i = 0
    out = {}
    while True:
        if i >= len(body):
            raise ValueError("eof")
        typ = body[i]
        i += 1
        if typ == TSTOP:
            break
        fid = struct.unpack_from(">H", body, i)[0]
        i += 2
        name_typ = by_id.get(fid)
        if typ == TI64:
            val = struct.unpack_from(">q", body, i)[0]
            i += 8
        elif typ == TSTRING:
            n = struct.unpack_from(">I", body, i)[0]
            i += 4
            val = body[i : i + n].decode()
            i += n
        elif typ == TBOOL:
            val = body[i] == 1
            i += 1
        else:
            raise ValueError(typ)
        if name_typ:
            out[name_typ[0]] = val
    return out


class Client:
    def __init__(self, addr: str):
        host, port = addr.rsplit(":", 1)
        self.sock = socket.create_connection((host, int(port)))
        self.seq = 0

    def close(self):
        self.sock.close()

    def call(self, method: str, req) -> dict:
        if isinstance(req, str):
            req = json.loads(req)
        spec = IDL[method]
        body = encode_struct(spec["req"], req)
        self.seq += 1
        mb = method.encode()
        msg = struct.pack(">BIH", MSG_CALL, self.seq, len(mb)) + mb + struct.pack(">H", 0) + body
        self.sock.sendall(struct.pack(">I", len(msg)) + msg)
        hdr = recvall(self.sock, 4)
        n = struct.unpack(">I", hdr)[0]
        raw = recvall(self.sock, n)
        typ, seq, mlen = struct.unpack_from(">BIH", raw, 0)
        hlen = struct.unpack_from(">H", raw, 7 + mlen)[0]
        rbody = raw[7 + mlen + 2 + hlen :]
        if typ == MSG_EX:
            raise RuntimeError(rbody.decode())
        return decode_struct(spec["resp"], rbody)


def recvall(sock, n: int) -> bytes:
    buf = bytearray()
    while len(buf) < n:
        chunk = sock.recv(n - len(buf))
        if not chunk:
            raise EOFError
        buf += chunk
    return bytes(buf)


def main():
    addr = sys.argv[1] if len(sys.argv) > 1 else "127.0.0.1:8888"
    c = Client(addr)
    try:
        print("py map  GetUser ->", c.call("GetUser", {"id": 1}))
        print("py json GetUser ->", c.call("GetUser", '{"id": 2}'))
        print("py map  Ping    ->", c.call("Ping", {}))
    finally:
        c.close()


if __name__ == "__main__":
    main()
