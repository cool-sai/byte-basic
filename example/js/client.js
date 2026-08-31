// Generic Node client: no generated stubs. Field ids come from idl/user.idl.
const net = require("net");

const TSTOP = 0, TBOOL = 1, TI64 = 2, TSTRING = 3;
const MSG_CALL = 1, MSG_REPLY = 2, MSG_EX = 3;

const IDL = {
  GetUser: {
    req: [["id", 1, "i64"]],
    resp: [["id", 1, "i64"], ["name", 2, "string"]],
  },
  Ping: {
    req: [],
    resp: [["msg", 1, "string"]],
  },
};

function encodeStruct(fields, data) {
  const chunks = [];
  for (const [name, fid, typ] of fields) {
    if (data[name] == null) continue;
    if (typ === "i64") {
      const b = Buffer.alloc(1 + 2 + 8);
      b[0] = TI64;
      b.writeUInt16BE(fid, 1);
      b.writeBigInt64BE(BigInt(data[name]), 3);
      chunks.push(b);
    } else if (typ === "string") {
      const v = Buffer.from(String(data[name]));
      const b = Buffer.alloc(1 + 2 + 4);
      b[0] = TSTRING;
      b.writeUInt16BE(fid, 1);
      b.writeUInt32BE(v.length, 3);
      chunks.push(b, v);
    } else if (typ === "bool") {
      const b = Buffer.alloc(4);
      b[0] = TBOOL;
      b.writeUInt16BE(fid, 1);
      b[3] = data[name] ? 1 : 0;
      chunks.push(b);
    }
  }
  chunks.push(Buffer.from([TSTOP]));
  return Buffer.concat(chunks);
}

function decodeStruct(fields, body) {
  const byId = new Map(fields.map(([name, fid, typ]) => [fid, [name, typ]]));
  const out = {};
  let i = 0;
  while (true) {
    const typ = body[i++];
    if (typ === TSTOP) break;
    const fid = body.readUInt16BE(i); i += 2;
    let val;
    if (typ === TI64) { val = Number(body.readBigInt64BE(i)); i += 8; }
    else if (typ === TSTRING) {
      const n = body.readUInt32BE(i); i += 4;
      val = body.slice(i, i + n).toString(); i += n;
    } else if (typ === TBOOL) { val = body[i++] === 1; }
    const hit = byId.get(fid);
    if (hit) out[hit[0]] = val;
  }
  return out;
}

function call(sock, seq, method, req) {
  if (typeof req === "string") req = JSON.parse(req);
  const spec = IDL[method];
  const body = encodeStruct(spec.req, req);
  const mb = Buffer.from(method);
  const msg = Buffer.alloc(1 + 4 + 2 + mb.length + body.length);
  msg[0] = MSG_CALL;
  msg.writeUInt32BE(seq, 1);
  msg.writeUInt16BE(mb.length, 5);
  mb.copy(msg, 7);
  body.copy(msg, 7 + mb.length);
  const frame = Buffer.alloc(4 + msg.length);
  frame.writeUInt32BE(msg.length, 0);
  msg.copy(frame, 4);
  sock.write(frame);

  return readFrame(sock).then((raw) => {
    const typ = raw[0];
    const mlen = raw.readUInt16BE(5);
    const rbody = raw.slice(7 + mlen);
    if (typ === MSG_EX) throw new Error(rbody.toString());
    return decodeStruct(spec.resp, rbody);
  });
}

function readFrame(sock) {
  return new Promise((resolve, reject) => {
    let buf = Buffer.alloc(0);
    const onData = (chunk) => {
      buf = Buffer.concat([buf, chunk]);
      if (buf.length < 4) return;
      const n = buf.readUInt32BE(0);
      if (buf.length < 4 + n) return;
      sock.off("data", onData);
      sock.pause();
      const rest = buf.slice(4 + n);
      if (rest.length) sock.unshift(rest);
      resolve(buf.slice(4, 4 + n));
    };
    sock.on("data", onData);
    sock.resume();
    sock.once("error", reject);
  });
}

const addr = process.argv[2] || "127.0.0.1:8888";
const [host, port] = addr.split(":");
const sock = net.connect({ host, port: Number(port) }, async () => {
  try {
    sock.pause();
    console.log("js map  GetUser ->", await call(sock, 1, "GetUser", { id: 1 }));
    console.log("js json GetUser ->", await call(sock, 2, "GetUser", '{"id":2}'));
    console.log("js map  Ping    ->", await call(sock, 3, "Ping", {}));
  } catch (e) {
    console.error(e);
    process.exitCode = 1;
  } finally {
    sock.end();
  }
});
