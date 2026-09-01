export type Service = {
  name: string;
  bin: string;
  pkg: string;
  compose: string[];
};

export type Build = {
  id: number;
  service: string;
  version: string;
  binPath: string;
  status: string;
  log?: string;
  createdAt: string;
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

export { errMsg };

export const api = {
  services: () => req<Service[]>("/api/services"),
  builds: (service?: string) =>
    req<Build[] | null>("/api/scm/builds" + (service ? `?service=${service}` : "")),
  build: (service: string) =>
    req<{ service: string; version: string; status: string }>("/api/scm/builds", {
      method: "POST",
      body: JSON.stringify({ service }),
    }),
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
  deploys: (service?: string) =>
    req<DeployRecord[] | null>("/api/deploys" + (service ? `?service=${service}` : "")),
  deploy: (service: string, version: string) =>
    req<{ service: string; version: string; status: string }>("/api/deploys", {
      method: "POST",
      body: JSON.stringify({ service, version }),
    }),
  runtime: () => req<Container[] | null>("/api/runtime"),
  tables: () => req<DbTable[] | null>("/api/db/tables"),
  table: (name: string) => req<TableDetail>(`/api/db/tables/${name}`),
};
