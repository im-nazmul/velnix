export type Page =
    | "overview"
    | "projects"
    | "runtimes"
    | "services"
    | "certificates"
    | "mail"
    | "diagnostics";
export interface Selection {
    path: string;
    version: string;
    sha256: string;
}
export interface RuntimeSelection {
    executable: Selection;
}
export interface Project {
    name: string;
    root: string;
    document_root: string;
    domain: string;
    php_cli?: Selection;
    node?: RuntimeSelection;
    bun?: RuntimeSelection;
    python?: RuntimeSelection;
    proxy_port?: number;
}
export interface Workload {
    kind: string;
    name: string;
    phase: string;
    alive: boolean;
    recovery_required: boolean;
    port?: number;
    reason?: string;
}
export interface Check {
    name: string;
    status: string;
    detail: string;
}
export interface Candidate {
    path: string;
    authority: string;
    origin: string;
    probed: boolean;
}
export interface Snapshot {
    state?: { initialized: boolean; recovery_required: boolean };
    projects?: Project[];
    workloads?: Workload[];
    runtimes?: {
        kind: string;
        value:
            | Candidate[]
            | {
                  candidates: Candidate[];
                  issues: { directory: string; reason: string }[];
              };
    }[];
    certificate?: {
        phase: string;
        names: string[];
        expires: string;
        fingerprint: string;
    };
    checks?: Check[];
    platform?: {
        distribution: string;
        distribution_version: string;
        architecture: string;
        wsl: boolean;
        os: string;
    };
    issues: { area: string; message: string }[];
    truncated: boolean;
}
export interface Change {
    operation: string;
    name?: string;
    root?: string;
    document_root?: string;
    kind?: string;
    executable?: string;
    version?: string;
}
export interface Plan {
    ticket: string;
    expires: string;
    review: {
        id: string;
        change: Change;
        detail: Record<string, unknown>;
        notes: string[];
    };
}
export interface MailSummary {
    ID: string;
    Subject: string;
    From: { Name: string; Address: string };
    Created: string;
    Size: number;
    Read: boolean;
}
export interface MailPage {
    total: number;
    start: number;
    messages: MailSummary[];
}
export interface Message {
    id: string;
    headers: Record<string, string>;
    text: string;
    notice?: string;
    truncated: boolean;
    attachments: {
        id: string;
        name: string;
        content_type: string;
        size: number;
    }[];
}
export function version(p: Project, kind: string): string {
    return kind === "php"
        ? (p.php_cli?.version ?? "")
        : (p[kind as "node" | "bun" | "python"]?.executable?.version ?? "");
}
export function bytes(n: number): string {
    return n < 1024
        ? `${n} B`
        : n < 1048576
          ? `${(n / 1024).toFixed(1)} KB`
          : `${(n / 1048576).toFixed(1)} MB`;
}
