#!/usr/bin/env python3
"""Record the README demo as an asciicast, with mocked tool responses.

    scripts/demo-record.py
    agg --cols 100 --rows 28 --font-size 16 --theme dracula demo.cast docs/demo.gif

The demo is scripted rather than hand-recorded so it can be re-shot verbatim
when behavior changes. Since artex is an MCP tool server driven by Claude
Code, we cannot run a real Claude Code session in a recording. Instead we
simulate the tool-call cycle: a small helper prints JSON in the shape
artex actually returns, run against canned responses so the demo needs
no database or live target.

**Why tmux and not a bare pty.** Same reason as wsmancer's demo-record.py:
programs that query cursor position (ESC[6n]) get correct answers because
tmux genuinely models the screen. This script drives keys with send-keys and
samples capture-pane, emitting a frame whenever the screen changes. Idle
time costs nothing.

Conventions:
  * No real credentials or target IPs appear on camera. The target domain
    and IP come from env vars ($ARTEX_TARGET, $ARTEX_DOMAIN) so the
    GIF shows generic commands.
  * The PostgreSQL DSN is referenced as $ARTEX_PG, never expanded.
"""
import json, os, shutil, stat, subprocess, sys, tempfile, textwrap, time

SESSION = os.environ.get("ARTEX_TMUX_SESSION", "artex-demo")
CAST = os.environ.get("ARTEX_CAST", "demo.cast")
COLS, ROWS = 100, 28
REPO = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
TARGET = os.environ.get("ARTEX_TARGET", "10.129.x.x")
DOMAIN = os.environ.get("ARTEX_DOMAIN", "mgmt.htb")


# ---------------------------------------------------------------------------
# tmux + Recorder (same pattern as wsmancer/scripts/demo-record.py)
# ---------------------------------------------------------------------------

def tmux(*args, capture=False):
    cmd = ["tmux", *args]
    if capture:
        return subprocess.run(cmd, capture_output=True, text=True).stdout
    subprocess.run(cmd, check=True, capture_output=True)


class Recorder:
    def __init__(self):
        subprocess.run(["tmux", "kill-session", "-t", SESSION],
                       capture_output=True)
        env = {
            "TERM": "xterm-256color",
            "ARTEX_TARGET": TARGET,
            "ARTEX_DOMAIN": DOMAIN,
            "ARTEX_PG": "postgres://localhost/ccartex",
        }
        args = ["new-session", "-d", "-s", SESSION,
                "-x", str(COLS), "-y", str(ROWS)]
        for k, v in env.items():
            args += ["-e", f"{k}={v}"]
        args += ["bash", "--norc", "--noprofile", "-i"]
        tmux(*args)
        tmux("set-option", "-t", SESSION, "status", "off")
        self.t0 = time.time()
        self.events = []
        self.last = None

    def frame(self):
        pane = tmux("capture-pane", "-t", SESSION, "-p", "-e", capture=True)
        pos = tmux("display-message", "-t", SESSION, "-p",
                   "#{cursor_x} #{cursor_y}", capture=True).split()
        body = pane.rstrip("\n").replace("\n", "\r\n")
        cx, cy = (int(pos[0]), int(pos[1])) if len(pos) == 2 else (0, 0)
        painted = f"\x1b[H\x1b[2J{body}\x1b[{cy + 1};{cx + 1}H"
        if painted != self.last:
            self.last = painted
            self.events.append([round(time.time() - self.t0, 6), "o", painted])

    def pump(self, seconds):
        end = time.time() + seconds
        while time.time() < end:
            self.frame()
            time.sleep(0.05)

    def wait_for(self, needle, timeout=30.0):
        end = time.time() + timeout
        while time.time() < end:
            if needle in tmux("capture-pane", "-t", SESSION, "-p",
                              capture=True):
                return True
            self.pump(0.15)
        return False

    def send(self, *keys):
        tmux("send-keys", "-t", SESSION, *keys)

    def type(self, text, cps=18, settle=0.3):
        for ch in text:
            self.send("-H", *[f"{b:02x}" for b in ch.encode()])
            self.pump(1.0 / cps)
        self.pump(settle)

    def key(self, name, settle=0.9):
        self.send(name)
        self.pump(settle)

    def run(self, line, wait=3.0):
        self.type(line)
        self.key("Enter", 0.2)
        self.pump(wait)

    def beat(self, seconds=1.0):
        self.pump(seconds)

    def setup(self, extra_cmds=""):
        """Prepare the shell off camera, then start the clock on a clean
        screen. Everything here is noise in a demo (PATH, PS1, aliases).
        """
        setup = f"export PATH={REPO}:$PATH; export PS1='$ '"
        if extra_cmds:
            setup += "; " + extra_cmds
        self.send("-H", *[f"{b:02x}" for b in setup.encode()])
        self.send("Enter")
        time.sleep(0.5)
        self.send("-H", *[f"{b:02x}" for b in b"clear"])
        self.send("Enter")
        time.sleep(0.5)
        self.events.clear()
        self.last = None
        self.t0 = time.time()

    def save(self, path):
        header = {"version": 2, "width": COLS, "height": ROWS,
                  "timestamp": int(self.t0),
                  "env": {"TERM": "xterm-256color", "SHELL": "/bin/bash"}}
        with open(path, "w") as f:
            f.write(json.dumps(header) + "\n")
            for e in self.events:
                f.write(json.dumps(e) + "\n")
        subprocess.run(["tmux", "kill-session", "-t", SESSION],
                       capture_output=True)


# ---------------------------------------------------------------------------
# Mock response data
# ---------------------------------------------------------------------------

RESPONSES = {
    "init": textwrap.dedent("""\
        Engagement created:
          Task ID:        1
          Exploration ID: 1
          Config written: ./.artex.json

        Add to your Claude Code MCP config (~/.claude.json or project .mcp.json):

        Minimal (no proxy, data in cwd):

          {
            "mcpServers": {
              "artex": {
                "command": "artex",
                "args": ["serve", "--stdio", "--pg", "$ARTEX_PG"]
              }
            }
          }"""),

    "graph_overview": json.dumps({
        "task": {"id": 1, "goal": f"Pentest {DOMAIN} ({TARGET})",
                 "status": "active"},
        "findings_total": 0, "facts_total": 0,
        "frontier_open": 0, "done_intents": 0,
        "goals": [], "open_intents": [], "recent_facts": [],
    }, indent=2),

    "insert_assets": json.dumps({
        "results": [
            {"index": 0, "id": 1, "type": "root_domain"},
            {"index": 1, "id": 2, "type": "ip"},
        ],
        "errors": [],
    }, indent=2),

    "enrich_domain": json.dumps({
        "domain": DOMAIN,
        "records": {
            "A": [TARGET],
            "MX": [f"mail.{DOMAIN}"],
            "NS": ["ns1.htb", "ns2.htb"],
        },
        "new_assets": [
            {"type": "ip", "ip": TARGET, "source": "A record"},
        ],
    }, indent=2),

    "list_assets": json.dumps({
        "count": 4,
        "assets": [
            {"id": 1, "type": "root_domain", "domain": DOMAIN},
            {"id": 2, "type": "ip", "ip": TARGET,
             "open_ports": [{"port": 22, "service": "ssh"},
                            {"port": 80, "service": "http"},
                            {"port": 443, "service": "https"}]},
            {"id": 3, "type": "service", "url": f"http://{DOMAIN}",
             "status_code": 200, "page_title": "Management Portal"},
            {"id": 4, "type": "service", "url": f"https://{DOMAIN}",
             "status_code": 200, "page_title": "Management Portal"},
        ],
    }, indent=2),

    "add_intent": json.dumps({
        "id": 5, "kind": "intent", "state": "open",
        "summary": "Test management portal for default credentials",
    }, indent=2),

    "report_finding": json.dumps({"finding_id": 6}, indent=2),

    "generate_report": textwrap.dedent(f"""\
        # Penetration Test Report -- {DOMAIN}

        - **Task goal**: Pentest {DOMAIN} ({TARGET})
        - **Generated at**: 2026-11-21 14:30:00

        ## Summary

        - Confirmed findings: **1**
        - Assets: ip 1, root_domain 1, service 2

        ## Findings

        ### 1. [HIGH] Default Credentials on Management Portal

        The management portal at https://{DOMAIN} accepts default
        vendor credentials (admin/admin), granting full administrative
        access to the appliance.

        **PoC / Evidence:**

        ```
        POST /api/login HTTP/1.1
        Host: {DOMAIN}
        Content-Type: application/json

        {{"username":"admin","password":"admin"}}

        HTTP/1.1 200 OK
        Set-Cookie: session=eyJ...
        {{"status":"authenticated","role":"administrator"}}
        ```"""),
}


def write_mock_helper(tmpdir):
    """Write a Python mock helper and a JSON data file into tmpdir.

    The helper handles both ``artex init`` (via alias) and
    ``artex-call <tool>`` for simulated MCP tool invocations.
    """
    data_path = os.path.join(tmpdir, "responses.json")
    with open(data_path, "w") as f:
        json.dump(RESPONSES, f)

    helper_path = os.path.join(tmpdir, "demo-helper")
    helper = textwrap.dedent(f"""\
        #!/usr/bin/env python3
        import json, sys, time
        with open({data_path!r}) as f:
            R = json.load(f)
        cmd = sys.argv[1] if len(sys.argv) > 1 else ""
        if cmd == "--init":
            time.sleep(0.3)
            print(R.get("init", ""))
        elif cmd in R:
            time.sleep(0.2)
            print(R[cmd])
        else:
            print(json.dumps({{"error": "unknown: " + cmd}}))
    """)
    with open(helper_path, "w") as f:
        f.write(helper)
    os.chmod(helper_path, stat.S_IRWXU)
    return helper_path


# ---------------------------------------------------------------------------
# Record
# ---------------------------------------------------------------------------

tmpdir = tempfile.mkdtemp(prefix="artex-demo-")
helper = write_mock_helper(tmpdir)

r = Recorder()

aliases = (f"alias artex='{helper} --init #'; "
           f"alias artex-call='{helper}'")
r.setup(extra_cmds=aliases)
r.beat(0.6)

# Step 1: Initialize engagement.
r.type("# create a new pentest engagement")
r.key("Enter", 0.3)
r.type(
    'artex init --goal "Pentest $ARTEX_DOMAIN ($ARTEX_TARGET)" '
    '--description "External pentest" --pg "$ARTEX_PG"'
)
r.key("Enter", 0.2)
r.pump(4.0)
r.beat(2.5)

# Step 2: graph_overview (blank slate).
r.type("# Claude sees the empty exploration graph")
r.key("Enter", 0.3)
r.run("artex-call graph_overview", 3.0)
r.beat(1.5)

# Step 3: insert_assets.
r.type("# register the target domain and IP")
r.key("Enter", 0.3)
r.run("artex-call insert_assets", 3.0)
r.beat(1.0)

# Step 4: enrich_domain.
r.type("# DNS enrichment discovers records and services")
r.key("Enter", 0.3)
r.run("artex-call enrich_domain", 3.5)
r.beat(1.2)

# Step 5: list_assets.
r.type("# review all discovered assets")
r.key("Enter", 0.3)
r.run("artex-call list_assets", 4.0)
r.beat(1.5)

# Step 6: add_intent.
r.type("# plan an attack direction")
r.key("Enter", 0.3)
r.run("artex-call add_intent", 3.0)
r.beat(1.0)

# Step 7: report_finding.
r.type("# document a confirmed vulnerability")
r.key("Enter", 0.3)
r.run("artex-call report_finding", 3.0)
r.beat(1.0)

# Step 8: generate_report.
r.type("# generate the final pentest report")
r.key("Enter", 0.3)
r.run("artex-call generate_report", 5.0)
r.beat(3.0)

# Cleanup.
shutil.rmtree(tmpdir, ignore_errors=True)

r.save(CAST)
print(f"wrote {CAST}: {len(r.events)} frames, "
      f"{r.events[-1][0] if r.events else 0:.1f}s")
