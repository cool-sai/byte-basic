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

async function req<T>(path: string, opts: RequestInit = {}): Promise<T> {
  const res = await fetch(path, {
    headers: { "Content-Type": "application/json", ...(opts.headers || {}) },
    ...opts,
  });
  const text = await res.text();
  let data: unknown = null;
  try {
    data = text ? JSON.parse(text) : null;
  } catch {
    data = { error: text };
  }
  if (!res.ok) {
    const err = data as { error?: string } | null;
    throw new Error(err?.error || res.statusText);
  }
  return data as T;
}

function errMsg(e: unknown): string {
  return e instanceof Error ? e.message : String(e);
}

async function readSSE(res: Response, onLog: (text: string) => void): Promise<BuildResult> {
  const ct = res.headers.get("content-type") || "";
  if (!ct.includes("text/event-stream")) {
    const text = await res.text();
    let data: { error?: string } = {};
    try {
      data = text ? (JSON.parse(text) as { error?: string }) : {};
    } catch {
      data = { error: text };
    }
    throw new Error(data.error || res.statusText);
  }
  if (!res.body) {
    throw new Error("no stream");
  }
  const reader = res.body.getReader();
  const dec = new TextDecoder();
  let buf = "";
  let done: BuildResult | null = null;
  while (true) {
    const chunk = await reader.read();
    if (chunk.done) {
      break;
    }
    buf += dec.decode(chunk.value, { stream: true });
    const parts = buf.split("\n\n");
    buf = parts.pop() || "";
    for (const block of parts) {
      let event = "message";
      let data = "";
      for (const line of block.split("\n")) {
        if (line.startsWith("event:")) {
          event = line.slice(6).trim();
        } else if (line.startsWith("data:")) {
          data += line.slice(5).trim();
        }
      }
      if (!data) {
        continue;
      }
      if (event === "log") {
        const j = JSON.parse(data) as { text?: string };
        onLog(j.text || "");
      } else if (event === "done") {
        done = JSON.parse(data) as BuildResult;
      }
    }
  }
  if (!done) {
    throw new Error("stream ended");
  }
  if (done.status !== "ok") {
    throw new Error(done.error || "fail");
  }
  return done;
}

async function streamSSE(path: string, body: unknown, onLog: (text: string) => void): Promise<BuildResult> {
  const res = await fetch(path, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(body),
  });
  return readSSE(res, onLog);
}

export { errMsg };

export const api = {
  services: () => req<Service[]>("/api/services"),
  scmJobs: () => req<ScmJobs>("/api/scm/jobs"),
  scmJob: (name: string) => req<ScmJob>(`/api/scm/jobs/${name}`),
  createScmJob: (name: string, gitUrl: string, scriptPath: string, label: string) =>
    req<ScmJob>("/api/scm/jobs", {
      method: "POST",
      body: JSON.stringify({ name, gitUrl, scriptPath, label }),
    }),
  updateScmJob: (name: string, gitUrl: string, scriptPath: string, label: string) =>
    req<ScmJob>(`/api/scm/jobs/${name}`, {
      method: "PUT",
      body: JSON.stringify({ gitUrl, scriptPath, label }),
    }),
  deleteScmJob: (name: string) =>
    req<{ name: string }>(`/api/scm/jobs/${name}`, { method: "DELETE" }),
  branches: (name: string) => req<BranchList>(`/api/scm/jobs/${name}/branches`),
  builds: (service?: string) =>
    req<Build[] | null>("/api/scm/builds" + (service ? `?service=${service}` : "")),
  buildDetail: (id: number) => req<Build>(`/api/scm/builds/${id}`),
  createBuild: (name: string, branch: string) =>
    req<Build>("/api/scm/builds", {
      method: "POST",
      body: JSON.stringify({ name, branch }),
    }),
  watchBuild: (id: number, onLog: (text: string) => void) =>
    fetch("/api/scm/builds/" + id + "/stream").then((res) => readSSE(res, onLog)),
  idls: () => req<IdlView[]>("/api/bam/idls"),
  idl: (name: string) => req<IdlView>(`/api/bam/idls/${name}`),
  saveIdl: (name: string, content: string) =>
    req<IdlView>(`/api/bam/idls/${name}`, { method: "PUT", body: JSON.stringify({ content }) }),
  publishes: () => req<Publish[] | null>("/api/agw/publishes"),
  publish: (name: string) =>
    req<{ name: string; status: string; methods: IdlMethod[] }>("/api/agw/publish", {
      method: "POST",
      body: JSON.stringify({ name }),
    }),
  deployApps: () => req<DeployApps>("/api/deploy/apps"),
  deployApp: (name: string) => req<DeployApp>(`/api/deploy/apps/${name}`),
  createDeployApp: (name: string, scmName: string, compose: string) =>
    req<DeployApp>("/api/deploy/apps", {
      method: "POST",
      body: JSON.stringify({ name, scmName, compose }),
    }),
  deploys: (service?: string) =>
    req<DeployRecord[] | null>("/api/deploys" + (service ? `?service=${service}` : "")),
  deployDetail: (id: number) => req<DeployRecord>(`/api/deploys/${id}`),
  deploy: (service: string, version: string, onLog: (text: string) => void) =>
    streamSSE("/api/deploys", { service, version }, onLog),
  runtime: () => req<Container[] | null>("/api/runtime"),
  tables: () => req<DbTable[] | null>("/api/db/tables"),
  table: (name: string) => req<TableDetail>(`/api/db/tables/${name}`),
  tlbUpstreams: () => req<{ upstreams: { name: string; target: string }[] | null }>("/api/tlb/upstreams"),
  tlbSites: () => req<{ sites: TlbSite[] | null; zone: string }>("/api/tlb/sites"),
  tlbSite: (name: string) => req<TlbSiteDetail>("/api/tlb/sites/" + name),
  createTlbSite: (name: string) =>
    req<TlbSite>("/api/tlb/sites", { method: "POST", body: JSON.stringify({ name }) }),
  deleteTlbSite: (name: string) =>
    req<{ name: string }>("/api/tlb/sites/" + name, { method: "DELETE" }),
  createTlbRoute: (site: string, name: string, path: string, target: string) =>
    req<TlbRoute>("/api/tlb/sites/" + site + "/routes", {
      method: "POST",
      body: JSON.stringify({ name, path, target }),
    }),
  updateTlbRoute: (site: string, id: number, name: string, path: string, target: string) =>
    req<TlbRoute>("/api/tlb/sites/" + site + "/routes/" + id, {
      method: "PUT",
      body: JSON.stringify({ name, path, target }),
    }),
  deleteTlbRoute: (site: string, id: number) =>
    req<{ id: number }>("/api/tlb/sites/" + site + "/routes/" + id, { method: "DELETE" }),
  publishTlb: () => req<{ status: string; sites: number; routes: number }>("/api/tlb/publish", { method: "POST" }),
};
