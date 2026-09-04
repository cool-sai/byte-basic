export type Service = {
  name: string;
  bin: string;
  pkg: string;
  compose: string[];
};

export type ScmJob = {
  id: number;
  name: string;
  gitUrl: string;
  scriptPath: string;
  branch?: string;
  label?: string;
  createdAt: string;
};

export type ScmJobs = {
  jobs: ScmJob[] | null;
};

export type BuildResult = {
  service?: string;
  version?: string;
  binPath?: string;
  status: string;
  error?: string;
};

export type Build = {
  id: number;
  service: string;
  version: string;
  binPath: string;
  status: string;
  branch?: string;
  commit?: string;
  log?: string;
  createdAt: string;
};

export type GitBranch = {
  name: string;
  commit: string;
};

export type BranchList = {
  default?: string;
  branches: GitBranch[] | null;
};

export type Field = {
  id: number;
  type: string;
  name: string;
};

export type IdlMethod = {
  name: string;
  req: string;
  resp: string;
  httpMethod: string;
  uri: string;
  reqFields?: Field[] | null;
  respFields?: Field[] | null;
};

export type IdlView = {
  name: string;
  service?: string;
  content: string;
  methods?: IdlMethod[];
  httpApis?: number;
  parseError?: string;
};

export type Publish = {
  id: number;
  idlName: string;
  routesJson: string;
  status: string;
  log?: string;
  createdAt: string;
};

export type DeployApp = {
  id: number;
  name: string;
  scmName: string;
  compose: string[] | null;
  label?: string;
  createdAt?: string;
};

export type DeployApps = {
  apps: DeployApp[] | null;
};

export type DeployRecord = {
  id: number;
  service: string;
  version: string;
  status: string;
  log?: string;
  createdAt: string;
};

export type Container = {
  ID?: string;
  Name?: string;
  Service?: string;
  Image?: string;
  Status?: string;
  State?: string;
};

export type TlbRoute = {
  id: number;
  name: string;
  path: string;
  target: string;
  createdAt?: string;
};

export type TlbSite = {
  id: number;
  name: string;
  host: string;
  routes: number;
  createdAt?: string;
};

export type TlbSiteDetail = {
  id: number;
  name: string;
  host: string;
  routes: TlbRoute[];
  zone?: string;
};

export type DbTable = {
  name: string;
  rows?: number | string | null;
  engine?: string;
};

export type DbColumn = {
  name: string;
  type: string;
  nullable: string;
  key: string;
  default: string | null;
  extra: string;
};

export type TableDetail = {
  name: string;
  columns: DbColumn[];
  preview: Record<string, unknown>[];
};

import { Code, ConnectError, createClient, type Interceptor } from "@connectrpc/connect";
import { createConnectTransport } from "@connectrpc/connect-web";
import type { Idl as ProtoIdl, Job as ProtoJob, Build as ProtoBuild, App as ProtoApp, Deploy as ProtoDeploy, TlbRoute as ProtoRoute, TlbSite as ProtoSite } from "./gen/platform/v1/platform_pb";
import { PlatformService } from "./gen/platform/v1/platform_pb";

function errMsg(e: unknown): string {
  return e instanceof Error ? e.message : String(e);
}

export { errMsg };

function dropSession() {
  localStorage.removeItem("token");
  localStorage.removeItem("user");
  if (window.location.pathname !== "/login") {
    window.location.assign("/login");
  }
}

const auth: Interceptor = (next) => async (req) => {
  const token = localStorage.getItem("token") || "";
  if (token) {
    req.header.set("Authorization", "Bearer " + token);
  }
  try {
    return await next(req);
  } catch (e) {
    if (e instanceof ConnectError && e.code === Code.Unauthenticated && !req.url.endsWith("/Login")) {
      dropSession();
    }
    throw e;
  }
};

const client = createClient(
  PlatformService,
  createConnectTransport({
    baseUrl: "",
    interceptors: [auth],
  }),
);

const num = (v: bigint | number) => Number(v);

function job(j: ProtoJob): ScmJob {
  return {
    id: num(j.id),
    name: j.name,
    gitUrl: j.gitUrl,
    scriptPath: j.scriptPath,
    branch: j.branch,
    label: j.label,
    createdAt: j.createdAt,
  };
}

function build(b: ProtoBuild): Build {
  return {
    id: num(b.id),
    service: b.service,
    version: b.version,
    binPath: b.binPath,
    status: b.status,
    branch: b.branch,
    commit: b.commit,
    log: b.log,
    createdAt: b.createdAt,
  };
}

function idl(v: ProtoIdl): IdlView {
  return {
    name: v.name,
    service: v.service,
    content: v.content,
    httpApis: v.httpApis,
    parseError: v.parseError || undefined,
    methods: v.methods.map((m) => ({
      name: m.name,
      req: m.req,
      resp: m.resp,
      httpMethod: m.httpMethod,
      uri: m.uri,
      reqFields: m.reqFields.map((f) => ({ id: f.id, type: f.type, name: f.name })),
      respFields: m.respFields.map((f) => ({ id: f.id, type: f.type, name: f.name })),
    })),
  };
}

function app(a: ProtoApp): DeployApp {
  return {
    id: num(a.id),
    name: a.name,
    scmName: a.scmName,
    compose: a.compose,
    label: a.label,
    createdAt: a.createdAt,
  };
}

function deploy(d: ProtoDeploy): DeployRecord {
  return {
    id: num(d.id),
    service: d.service,
    version: d.version,
    status: d.status,
    log: d.log,
    createdAt: d.createdAt,
  };
}

function site(s: ProtoSite): TlbSite {
  return { id: num(s.id), name: s.name, host: s.host, routes: s.routes, createdAt: s.createdAt };
}

function route(r: ProtoRoute): TlbRoute {
  return { id: num(r.id), name: r.name, path: r.path, target: r.target, createdAt: r.createdAt };
}

async function consumeRun(
  stream: AsyncIterable<{ text: string; done: boolean; status: string; error: string }>,
  onLog: (text: string) => void,
): Promise<BuildResult> {
  let result: BuildResult = { status: "fail" };
  for await (const ev of stream) {
    if (ev.text) {
      onLog(ev.text);
    }
    if (ev.done) {
      result = { status: ev.status || "ok", error: ev.error || undefined };
    }
  }
  if (result.status !== "ok") {
    throw new Error(result.error || "fail");
  }
  return result;
}

export const api = {
  services: () => client.listServices({}).then((r) => r.services as Service[]),
  scmJobs: () => client.listJobs({}).then((r) => ({ jobs: r.jobs.map(job) })),
  scmJob: (name: string) => client.showJob({ name }).then(job),
  createScmJob: (name: string, gitUrl: string, scriptPath: string, label: string) =>
    client.createJob({ name, gitUrl, scriptPath, label }).then(job),
  updateScmJob: (name: string, gitUrl: string, scriptPath: string, label: string) =>
    client.updateJob({ name, gitUrl, scriptPath, label }).then(job),
  deleteScmJob: (name: string) => client.deleteJob({ name }),
  branches: (name: string) =>
    client.listBranches({ name }).then((r) => ({ default: r.defaultBranch, branches: r.branches })),
  builds: (service?: string) =>
    client.listBuilds(service ? { service } : {}).then((r) => r.builds.map(build)),
  buildDetail: (id: number) => client.getBuild({ id: BigInt(id) }).then(build),
  createBuild: (name: string, branch: string) => client.createBuild({ name, branch }).then(build),
  watchBuild: (id: number, onLog: (text: string) => void) => consumeRun(client.watchBuild({ id: BigInt(id) }), onLog),
  login: (name: string, password: string) => client.login({ name, password }),
  idls: () => client.listIdls({}).then((r) => r.idls.map(idl)),
  idl: (name: string) => client.getIdl({ name }).then(idl),
  saveIdl: (name: string, content: string) => client.saveIdl({ name, content }).then(idl),
  publishes: () =>
    client.listPublishes({}).then((r) =>
      r.publishes.map((p) => ({
        id: num(p.id),
        idlName: p.idlName,
        routesJson: p.routesJson,
        status: p.status,
        log: p.log,
        createdAt: p.createdAt,
      })),
    ),
  publish: (name: string) =>
    client.publishAgw({ name }).then((r) => ({
      name: r.name,
      status: r.status,
      methods: r.methods.map((m) => ({
        name: m.name,
        req: m.req,
        resp: m.resp,
        httpMethod: m.httpMethod,
        uri: m.uri,
      })),
    })),
  deployApps: () => client.listApps({}).then((r) => ({ apps: r.apps.map(app) })),
  deployApp: (name: string) => client.showApp({ name }).then(app),
  createDeployApp: (name: string, scmName: string, compose: string) =>
    client.createApp({ name, scmName, compose }).then(app),
  deploys: (service?: string) =>
    client.listDeploys(service ? { service } : {}).then((r) => r.deploys.map(deploy)),
  deployDetail: (id: number) => client.getDeploy({ id: BigInt(id) }).then(deploy),
  deploy: async (service: string, version: string, onLog: (text: string) => void) => {
    const d = await client.createDeploy({ service, version });
    return consumeRun(client.watchDeploy({ id: d.id }), onLog);
  },
  runtime: () =>
    client.runtime({}).then((r) =>
      r.containers.map((c) => ({
        ID: c.id,
        Name: c.name,
        Service: c.service,
        Image: c.image,
        Status: c.status,
        State: c.state,
      })),
    ),
  tables: () => client.listTables({}).then((r) => r.tables as DbTable[]),
  table: (name: string) =>
    client.getTable({ name }).then((t) => ({
      name: t.name,
      columns: t.columns.map((c) => ({
        name: c.name,
        type: c.type,
        nullable: c.nullable,
        key: c.key,
        default: c.defaultValue || null,
        extra: c.extra,
      })),
      preview: t.preview.map((row) => ({ ...row.cells })),
    })),
  tlbUpstreams: () => client.listTlbUpstreams({}).then((r) => ({ upstreams: r.upstreams })),
  tlbSites: () => client.listTlbSites({}).then((r) => ({ sites: r.sites.map(site), zone: r.zone })),
  tlbSite: (name: string) =>
    client.showTlbSite({ name }).then((d) => ({
      id: num(d.id),
      name: d.name,
      host: d.host,
      routes: d.routes.map(route),
      zone: d.zone,
    })),
  createTlbSite: (name: string) => client.createTlbSite({ name }).then(site),
  deleteTlbSite: (name: string) => client.deleteTlbSite({ name }),
  createTlbRoute: (siteName: string, name: string, path: string, target: string) =>
    client.createTlbRoute({ site: siteName, name, path, target }).then(route),
  updateTlbRoute: (siteName: string, id: number, name: string, path: string, target: string) =>
    client.updateTlbRoute({ site: siteName, id: BigInt(id), name, path, target }).then(route),
  deleteTlbRoute: (siteName: string, id: number) => client.deleteTlbRoute({ site: siteName, id: BigInt(id) }),
  publishTlb: () => client.publishTlb({}),
};
