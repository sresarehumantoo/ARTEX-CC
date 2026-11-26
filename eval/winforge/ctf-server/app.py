"""
Internal monitoring dashboard - INTERNAL USE ONLY
Tracks service health and user accounts for the forge environment.
"""

import sqlite3
import hashlib
import json
import os
import sys
from http.server import HTTPServer, BaseHTTPRequestHandler
from urllib.parse import urlparse, parse_qs

DB_PATH = os.path.join(os.path.dirname(os.path.abspath(__file__)), "monitor.db")
LISTEN_HOST = "127.0.0.1"
LISTEN_PORT = 8080

def init_db():
    conn = sqlite3.connect(DB_PATH)
    c = conn.cursor()

    c.execute("""
        CREATE TABLE IF NOT EXISTS users (
            id INTEGER PRIMARY KEY,
            username TEXT NOT NULL,
            password_hash TEXT NOT NULL,
            role TEXT NOT NULL,
            email TEXT
        )
    """)

    c.execute("""
        CREATE TABLE IF NOT EXISTS services (
            id INTEGER PRIMARY KEY,
            name TEXT NOT NULL,
            endpoint TEXT NOT NULL,
            status TEXT NOT NULL,
            last_check TEXT
        )
    """)

    c.execute("""
        CREATE TABLE IF NOT EXISTS api_tokens (
            id INTEGER PRIMARY KEY,
            token TEXT NOT NULL,
            owner TEXT NOT NULL,
            scope TEXT NOT NULL,
            created TEXT NOT NULL
        )
    """)

    c.execute("""
        CREATE TABLE IF NOT EXISTS audit_log (
            id INTEGER PRIMARY KEY,
            timestamp TEXT NOT NULL,
            user TEXT NOT NULL,
            action TEXT NOT NULL,
            detail TEXT
        )
    """)

    if c.execute("SELECT COUNT(*) FROM users").fetchone()[0] == 0:
        users = [
            ("admin", hashlib.sha256(b"F0rg3M@st3r!2025").hexdigest(), "admin", "admin@winforge.local"),
            ("forge", hashlib.sha256(b"hammertime").hexdigest(), "user", "forge@winforge.local"),
            ("monitor", hashlib.sha256(b"m0n1t0r_svc").hexdigest(), "service", "monitor@winforge.local"),
            ("backup", hashlib.sha256(b"bkup_r0t@te").hexdigest(), "service", "backup@winforge.local"),
        ]
        c.executemany("INSERT INTO users (username, password_hash, role, email) VALUES (?, ?, ?, ?)", users)

    if c.execute("SELECT COUNT(*) FROM services").fetchone()[0] == 0:
        services = [
            ("web-dashboard", "http://127.0.0.1:8080/health", "healthy", "2026-11-24T08:00:00Z"),
            ("postgres-primary", "tcp://127.0.0.1:5432", "healthy", "2026-11-24T08:00:00Z"),
            ("redis-cache", "tcp://127.0.0.1:6379", "stopped", "2026-11-24T07:45:00Z"),
            ("backup-agent", "http://127.0.0.1:9090/status", "healthy", "2026-11-24T08:00:00Z"),
        ]
        c.executemany("INSERT INTO services (name, endpoint, status, last_check) VALUES (?, ?, ?, ?)", services)

    if c.execute("SELECT COUNT(*) FROM api_tokens").fetchone()[0] == 0:
        tokens = [
            ("wf-adm-7f3a9b2c4e1d8065", "admin", "full", "2026-10-01T00:00:00Z"),
            ("wf-mon-e4c8a1b3d7f20956", "monitor", "read", "2026-10-15T00:00:00Z"),
            ("wf-bak-2d6f8e1a9c3b7504", "backup", "backup", "2026-11-01T00:00:00Z"),
        ]
        c.executemany("INSERT INTO api_tokens (token, owner, scope, created) VALUES (?, ?, ?, ?)", tokens)

    if c.execute("SELECT COUNT(*) FROM audit_log").fetchone()[0] == 0:
        logs = [
            ("2026-11-24T07:30:00Z", "admin", "login", "Dashboard login from 127.0.0.1"),
            ("2026-11-24T07:31:00Z", "admin", "config_change", "Updated backup schedule"),
            ("2026-11-24T07:45:00Z", "monitor", "alert", "redis-cache health check failed"),
            ("2026-11-24T08:00:00Z", "backup", "backup_start", "Daily backup initiated"),
        ]
        c.executemany("INSERT INTO audit_log (timestamp, user, action, detail) VALUES (?, ?, ?, ?)", logs)

    conn.commit()
    conn.close()


ADMIN_TOKEN = "wf-adm-7f3a9b2c4e1d8065"
FLAG_PATH = r"C:\ProgramData\winforge\flag.txt"

def _read_flag():
    try:
        with open(FLAG_PATH, "r") as f:
            return f.read().strip()
    except Exception:
        return "FLAG_READ_ERROR"


class MonitorHandler(BaseHTTPRequestHandler):

    def log_message(self, format, *args):
        pass

    def do_GET(self):
        parsed = urlparse(self.path)
        path = parsed.path
        params = parse_qs(parsed.query)

        if path == "/":
            self.send_json(200, {
                "service": "winforge-monitor",
                "version": "1.3.2",
                "endpoints": ["/health", "/api/services", "/api/search", "/api/admin"],
            })
        elif path == "/health":
            self.send_json(200, {"status": "ok"})
        elif path == "/api/services":
            self.handle_services()
        elif path == "/api/search":
            q = params.get("q", [""])[0]
            category = params.get("category", ["users"])[0]
            self.handle_search(q, category)
        elif path == "/api/logs":
            self.handle_logs(params)
        elif path == "/api/admin":
            self.handle_admin()
        elif path.startswith("/files"):
            name = params.get("name", [""])[0]
            self.handle_files(name)
        else:
            self.send_json(404, {"error": "not found"})

    def handle_services(self):
        conn = sqlite3.connect(DB_PATH)
        conn.row_factory = sqlite3.Row
        rows = conn.execute("SELECT name, status, last_check FROM services").fetchall()
        conn.close()
        self.send_json(200, {"services": [dict(r) for r in rows]})

    def handle_search(self, q, category):
        if not q:
            self.send_json(400, {"error": "missing 'q' parameter"})
            return

        conn = sqlite3.connect(DB_PATH)
        conn.row_factory = sqlite3.Row

        # Intentionally vulnerable to SQL injection
        if category == "users":
            query = "SELECT id, username, role, email FROM users WHERE username LIKE '%" + q + "%' OR email LIKE '%" + q + "%'"
        elif category == "services":
            query = "SELECT id, name, endpoint, status FROM services WHERE name LIKE '%" + q + "%'"
        elif category == "logs":
            query = "SELECT id, timestamp, user, action, detail FROM audit_log WHERE user LIKE '%" + q + "%' OR action LIKE '%" + q + "%'"
        else:
            self.send_json(400, {"error": "invalid category, use: users, services, logs"})
            conn.close()
            return

        try:
            rows = conn.execute(query).fetchall()
            results = [dict(r) for r in rows]
            self.send_json(200, {"query": q, "category": category, "count": len(results), "results": results})
        except Exception as e:
            self.send_json(500, {"error": str(e)})
        finally:
            conn.close()

    def handle_logs(self, params):
        limit = params.get("limit", ["20"])[0]
        conn = sqlite3.connect(DB_PATH)
        conn.row_factory = sqlite3.Row
        try:
            rows = conn.execute("SELECT * FROM audit_log ORDER BY id DESC LIMIT ?", (int(limit),)).fetchall()
            self.send_json(200, {"logs": [dict(r) for r in rows]})
        except Exception:
            self.send_json(400, {"error": "invalid limit"})
        finally:
            conn.close()

    def handle_admin(self):
        token = self.headers.get("X-Api-Token", "")
        if token != ADMIN_TOKEN:
            self.send_json(403, {"error": "invalid or missing API token", "hint": "Provide a valid token via X-Api-Token header"})
            return

        self.send_json(200, {
            "admin_panel": True,
            "flag": _read_flag(),
            "message": "Congratulations! You've completed the attack chain.",
            "system_info": {
                "hostname": "winforge",
                "os": "Windows Server 2025 Core",
                "uptime": "14d 6h 32m",
            },
        })

    def handle_files(self, name):
        if not name:
            self.send_json(200, {
                "available_reports": [
                    "health-report.txt",
                    "service-map.txt",
                    "changelog.txt",
                ],
            })
            return

        # Rejected before any filesystem call: realpath on a UNC name would make
        # the SYSTEM service connect to an attacker-chosen SMB host.
        if ":" in name or name.startswith(("/", "\\")) or os.path.isabs(name):
            self.send_json(403, {"error": "access denied"})
            return

        # Intentionally vulnerable to directory traversal
        base_dir = os.path.join(os.path.dirname(os.path.abspath(__file__)), "reports")
        file_path = os.path.realpath(os.path.join(base_dir, name))

        # The traversal may reach the app source, but never files outside the
        # app directory: the service runs as SYSTEM and the other CTF tracks
        # keep their flags in SYSTEM-readable places.
        app_dir = os.path.dirname(os.path.realpath(__file__))
        if os.path.commonpath([app_dir.lower(), file_path.lower()]) != app_dir.lower():
            self.send_json(403, {"error": "access denied"})
            return

        try:
            with open(file_path, "r") as f:
                content = f.read()
            self.send_response(200)
            self.send_header("Content-Type", "text/plain")
            self.end_headers()
            self.wfile.write(content.encode())
        except FileNotFoundError:
            self.send_json(404, {"error": "file not found"})
        except Exception as e:
            self.send_json(500, {"error": str(e)})

    def send_json(self, code, data):
        body = json.dumps(data, indent=2).encode()
        self.send_response(code)
        self.send_header("Content-Type", "application/json")
        self.send_header("Content-Length", str(len(body)))
        self.end_headers()
        self.wfile.write(body)


def main():
    init_db()
    server = HTTPServer((LISTEN_HOST, LISTEN_PORT), MonitorHandler)
    print(f"Monitor dashboard listening on {LISTEN_HOST}:{LISTEN_PORT}", flush=True)
    try:
        server.serve_forever()
    except KeyboardInterrupt:
        pass
    server.server_close()


if __name__ == "__main__":
    main()
