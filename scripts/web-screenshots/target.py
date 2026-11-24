import json
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer

USERS = {1: {"id": 1, "email": "admin@acme.example", "role": "admin", "mfa": False},
         2: {"id": 2, "email": "j.doe@acme.example", "role": "user", "mfa": True},
         3: {"id": 3, "email": "svc-backup@acme.example", "role": "service", "mfa": False}}

class H(BaseHTTPRequestHandler):
    server_version = "nginx/1.25.3"
    sys_version = ""
    def send(self, code, body, ctype="application/json", extra=None):
        b = body.encode() if isinstance(body, str) else json.dumps(body, indent=2).encode()
        self.send_response(code)
        self.send_header("Content-Type", ctype)
        self.send_header("X-Powered-By", "Express")
        for k, v in (extra or {}).items(): self.send_header(k, v)
        self.send_header("Content-Length", str(len(b)))
        self.end_headers(); self.wfile.write(b)
    def do_GET(self):
        p = self.path.split("?")[0]
        if p == "/":
            self.send(200, "<html><head><title>ACME Customer Portal</title></head><body><h1>ACME Portal</h1><a href='/login'>Sign in</a></body></html>", "text/html")
        elif p == "/login":
            self.send(200, "<html><head><title>Sign in | ACME</title></head><body><form method=post action=/api/v2/auth/login><input name=email><input name=password type=password></form></body></html>", "text/html")
        elif p.startswith("/api/v2/users/"):
            uid = int(p.rsplit("/", 1)[1] or 0)
            u = USERS.get(uid)
            self.send(200 if u else 404, u or {"error": "not found"})
        elif p == "/api/v2/users":
            self.send(200, {"users": list(USERS.values()), "total": 3})
        elif p == "/.git/config":
            self.send(200, "[core]\n\trepositoryformatversion = 0\n[remote \"origin\"]\n\turl = git@git.acme.example:web/portal.git\n", "text/plain")
        elif p == "/robots.txt":
            self.send(200, "User-agent: *\nDisallow: /admin\nDisallow: /api/v2/internal\n", "text/plain")
        elif p.startswith("/admin"):
            self.send(302, "", "text/html", {"Location": "/login?next=/admin"})
        elif p == "/api/v2/internal/health":
            self.send(200, {"status": "ok", "db": "postgres://portal@db01.int.acme.example:5432/portal", "version": "4.2.1"})
        else:
            self.send(404, {"error": "not found"})
    def do_POST(self):
        n = int(self.headers.get("Content-Length") or 0); body = self.rfile.read(n).decode()
        if self.path.startswith("/api/v2/auth/login"):
            if "Spring2026" in body:
                self.send(200, {"token": "eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiIzIiwicm9sZSI6InNlcnZpY2UifQ.c2lnbmF0dXJl", "user": USERS[3]}, extra={"Set-Cookie": "session=8f2c1d; HttpOnly"})
            else:
                self.send(401, {"error": "invalid credentials"})
        elif self.path.startswith("/api/v2/search"):
            if "'" in body:
                self.send(500, {"error": "pq: syntax error at or near \"'\" at character 41"})
            else:
                self.send(200, {"results": [], "query": body})
        else:
            self.send(404, {"error": "not found"})
    def log_message(self, *a): pass

ThreadingHTTPServer(("0.0.0.0", 80), H).serve_forever()
