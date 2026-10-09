#!/usr/bin/env python3
"""Exercise the actual image, isolated ports/network/volume, and clean up.

Requires Python 3 and Docker. --keep preserves a fixture for browser QA.
Never prints bootstrap passwords, session cookies, CSRF tokens or private keys.
"""
import argparse
import http.cookiejar
import http.client
import json
import os
import pathlib
import secrets
import shutil
import socket
import ssl
import subprocess
import tempfile
import time
import urllib.error
import urllib.request
import uuid

ROOT = pathlib.Path(__file__).resolve().parents[2]
UPSTREAM = pathlib.Path(__file__).resolve().with_name("upstream.py")


def docker(*args, check=True):
    result = subprocess.run(["docker", *args], capture_output=True, text=True)
    if check and result.returncode:
        raise RuntimeError(f"Docker {args[0]} failed: {result.stderr.strip()}")
    return result.stdout.strip()


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--image", default="nginx-web:docker")
    parser.add_argument("--platform")
    parser.add_argument("--keep", action="store_true")
    args = parser.parse_args()
    fixture = "nginx-web-docker-" + uuid.uuid4().hex[:10]
    network, volume, upstream = fixture + "-net", fixture + "-data", fixture + "-upstream"
    cache = ROOT / ".cache"
    cache.mkdir(exist_ok=True)
    temp = pathlib.Path(tempfile.mkdtemp(prefix=fixture + "-", dir=cache))
    os.chmod(temp, 0o755)
    secret = temp / "admin-password"
    password = secrets.token_urlsafe(24)
    secret.write_text(password + "\n")
    secret.chmod(0o444)
    site = temp / "www"
    site.mkdir(mode=0o755)
    (site / "index.html").write_text("DOCKER_STATIC_OK")
    docker("network", "create", network)
    docker("volume", "create", volume)
    jar = http.cookiejar.CookieJar()
    opener = urllib.request.build_opener(urllib.request.ProxyHandler({}), urllib.request.HTTPCookieProcessor(jar))
    csrf = ""
    base = ""

    def request(path, method="GET", body=None, status=200, auth=True, token=True, headers=None):
        values = {"Content-Type": "application/json", "X-FnProxy-Request": "1"}
        if token and csrf:
            values["X-CSRF-Token"] = csrf
        if headers:
            values.update(headers)
        data = json.dumps(body).encode() if body is not None else None
        req = urllib.request.Request(base + path, data=data, headers=values, method=method)
        client = opener if auth else urllib.request.build_opener(urllib.request.ProxyHandler({}))
        try:
            response = client.open(req, timeout=20)
        except urllib.error.HTTPError as error:
            response = error
        with response:
            raw = response.read()
            assert response.code == status, f"{method} {path}: HTTP {response.code}, expected {status}"
            return json.loads(raw) if "application/json" in response.headers.get("Content-Type", "") else raw

    def login():
        nonlocal csrf
        value = request("/api/auth/login", "POST", {"username": "admin", "password": password})
        assert value["authenticated"] and value["username"] == "admin"
        csrf = value["csrf_token"]

    def port(number, protocol="tcp"):
        value = docker("port", fixture, f"{number}/{protocol}")
        return int(value.splitlines()[0].rsplit(":", 1)[1])

    def wait_ready():
        for _ in range(150):
            try:
                request("/healthz", auth=False)
                return
            except (OSError, AssertionError):
                time.sleep(0.2)
        raise RuntimeError("Management service did not become ready")

    def proxy(number, domain, expected, tls=False):
        connection = http.client.HTTPConnection("127.0.0.1", port(number), timeout=5)
        if tls:
            context = ssl.create_default_context(cadata=certificate)
            with socket.create_connection(("127.0.0.1", port(number)), timeout=5) as plain:
                with context.wrap_socket(plain, server_hostname=domain) as secure:
                    secure.sendall(f"GET / HTTP/1.0\r\nHost: {domain}\r\n\r\n".encode())
                    response = b""
                    while chunk := secure.recv(4096):
                        response += chunk
                    assert expected in response, "HTTPS response mismatch"
            return
        try:
            connection.request("GET", "/", headers={"Host": domain})
            response = connection.getresponse()
            assert response.status == 200 and response.read() == expected, "HTTP/TCP response mismatch"
        finally:
            connection.close()

    success = False
    try:
        docker("run", "-d", "--name", upstream, "--network", network, "--network-alias", "upstream",
               "--mount", f"type=bind,src={UPSTREAM},dst=/upstream.py,readonly",
               "python:3.12-alpine", "python", "/upstream.py")
        run = ["run", "-d", "--name", fixture, "--network", network, "--read-only", "--cap-drop=ALL",
               "--security-opt=no-new-privileges:true", "--sysctl=net.ipv4.ip_unprivileged_port_start=0",
               "--tmpfs", "/tmp:rw,nosuid,nodev,size=256m,mode=1777", "--tmpfs",
               "/run/nginx-web:rw,nosuid,nodev,size=16m,uid=10001,gid=10001,mode=0750",
               "--mount", f"type=volume,src={volume},dst=/data", "--mount",
               f"type=bind,src={secret},dst=/run/secrets/admin_password,readonly", "--mount",
               f"type=bind,src={site},dst=/mnt/www,readonly", "-e", "FNPROXY_ADMIN_PASSWORD_FILE=/run/secrets/admin_password",
               "-e", "FNPROXY_ALLOWED_PATHS=/mnt/www", "-e", "FNPROXY_DEV_ALLOW=1"]
        for mapping in ["8080/tcp", "80/tcp", "443/tcp", "9001/tcp", "9002/udp"]:
            run += ["-p", "127.0.0.1::" + mapping]
        if args.platform:
            run += ["--platform", args.platform]
        run += [args.image]
        docker(*run)
        base = f"http://127.0.0.1:{port(8080)}"
        wait_ready()
        assert request("/api/auth/session", auth=False)["authenticated"] is False
        request("/api/state", status=401, auth=False, headers={"X-Trim-Isadmin": "true"})
        request("/api/auth/login", "POST", {"username": "admin", "password": "incorrect"}, status=401)
        login()
        request("/api/apply", "POST", {}, status=403, token=False)
        request("/api/apply", "POST", {}, status=403, headers={"Origin": "http://evil.invalid"})
        request("/internal/basic-auth/unknown", status=404)
        assert request("/api/overview")["nginx"]["running"]
        assert docker("exec", fixture, "id", "-u") == "10001"
        assert docker("exec", fixture, "sh", "-c", "awk '/CapEff:/ { print $2 }' /proc/self/status") == "0000000000000000"
        docker("exec", fixture, "/opt/nginx-web/bin/nginx-web-server", "healthcheck")
        docker("exec", fixture, "openssl", "req", "-x509", "-newkey", "rsa:2048", "-nodes", "-days", "1",
               "-subj", "/CN=docker.test", "-addext", "subjectAltName=DNS:docker.test",
               "-keyout", "/tmp/key.pem", "-out", "/tmp/cert.pem")
        certificate = docker("exec", fixture, "cat", "/tmp/cert.pem")
        private_key = docker("exec", fixture, "cat", "/tmp/key.pem")
        certificate_id = request("/api/certificates", "POST", {"name": "Docker fixture", "certificate": certificate, "private_key": private_key}, status=201)["id"]
        common = {"enabled": True, "upstream_scheme": "http", "upstream_host": "upstream", "upstream_port": 8000}
        rule = request("/api/rules", "POST", dict(common, name="Docker HTTP", listen_port=80, domains=["proxy.test"]), status=201)
        request("/api/rules", "POST", dict(common, name="Docker HTTPS", listen_port=443, domains=["docker.test"], tls=True, certificate_id=certificate_id), status=201)
        request("/api/rules", "POST", dict(common, name="Docker static", listen_port=80, domains=["static.test"],
                root_location={"backend_type": "static", "static_path": "/mnt/www", "index_files": ["index.html"]}), status=201)
        request("/api/rules", "POST", dict(common, name="Denied directory", listen_port=80, domains=["denied.test"],
                root_location={"backend_type": "static", "static_path": "/data", "index_files": ["index.html"]}), status=400)
        for protocol, number, target in [("tcp", 9001, 8000), ("udp", 9002, 9002)]:
            request("/api/streams", "POST", {"name": "Docker " + protocol, "enabled": True, "protocol": protocol,
                    "listen_address": "0.0.0.0", "listen_port": number, "upstream_host": "upstream", "upstream_port": target, "udp_responses": 1}, status=201)
        request("/api/apply", "POST", {"summary": "Docker integration"})
        proxy(80, "proxy.test", b"DOCKER_PROXY_OK")
        proxy(443, "docker.test", b"DOCKER_PROXY_OK", tls=True)
        proxy(80, "static.test", b"DOCKER_STATIC_OK")
        proxy(9001, "proxy.test", b"DOCKER_PROXY_OK")
        # Docker's Linux host namespace exercises the real published UDP
        # port. macOS VM forwarders may only forward TCP to the desktop host.
        udp_port = port(9002, "udp")
        docker("run", "--rm", "--network", "host", "python:3.12-alpine", "python", "-c",
               "import socket; s=socket.socket(socket.AF_INET,socket.SOCK_DGRAM); s.settimeout(5); "
               f"s.sendto(b'DOCKER_UDP_OK',('127.0.0.1',{udp_port})); "
               "assert s.recv(4096)==b'DOCKER_UDP_OK'; s.close()")
        profile = request("/api/auth-profiles", "POST", {"name": "Docker Basic Auth", "users": [
            {"username": "proxy-user", "password": "proxy-password", "enabled": True}]}, status=201)
        rule["authentication"] = {"enabled": True, "mode": "basic", "profile_id": profile["id"], "forward_authorization": False}
        request("/api/rules/" + rule["id"], "PUT", rule)
        request("/api/apply", "POST", {"summary": "Docker Basic Auth"})
        connection = http.client.HTTPConnection("127.0.0.1", port(80), timeout=5)
        connection.request("GET", "/", headers={"Host": "proxy.test"})
        response = connection.getresponse()
        assert response.status == 401
        response.read()
        connection.close()
        import base64
        connection = http.client.HTTPConnection("127.0.0.1", port(80), timeout=5)
        connection.request("GET", "/", headers={"Host": "proxy.test", "Authorization": "Basic " + base64.b64encode(b"proxy-user:proxy-password").decode()})
        response = connection.getresponse()
        assert response.status == 200 and response.read() == b"DOCKER_PROXY_OK"
        connection.close()
        backup = request("/api/backup")
        assert "admin.json" not in json.dumps(backup)
        # Leave a saved draft; a restart must resume the applied version.
        rule["name"] = "Unapplied draft"
        request("/api/rules/" + rule["id"], "PUT", rule)
        docker("restart", "--time", "30", fixture)
        # Docker may allocate new ephemeral published ports on restart.
        base = f"http://127.0.0.1:{port(8080)}"
        wait_ready()
        request("/api/state", status=401)  # Sessions are revoked at process restart.
        login()
        state = request("/api/state")
        assert state["dirty"] and any(item["name"] == "Unapplied draft" for item in state["rules"])
        proxy(443, "docker.test", b"DOCKER_PROXY_OK", tls=True)
        request("/api/nginx/stop", "POST", {})
        assert request("/api/overview")["nginx"]["running"] is False
        docker("exec", fixture, "/opt/nginx-web/bin/nginx-web-server", "healthcheck")
        request("/api/nginx/start", "POST", {})
        proxy(443, "docker.test", b"DOCKER_PROXY_OK", tls=True)
        request("/api/auth/logout", "POST", {})
        request("/api/state", status=401)
        success = True
        print("PASS: admin login/CSRF/logout, UID/capabilities, HTTP 80, HTTPS 443, TCP, UDP, static mount, Basic Auth, backup, persisted drafts/restart, Nginx stop/start, healthcheck")
        if args.keep:
            (temp / "fixture.json").write_text(json.dumps({"container": fixture, "network": network, "volume": volume, "upstream": upstream, "url": base, "secret_file": str(secret)}, indent=2))
            (temp / "fixture.json").chmod(0o600)
            print(f"Browser QA fixture: {base}; metadata: {temp / 'fixture.json'}")
        else:
            docker("stop", "--time", "30", fixture)
            assert docker("inspect", "--format", "{{.State.ExitCode}}", fixture) == "0", "Container did not exit gracefully"
    finally:
        if not success:
            diagnostics = docker("logs", "--tail", "12", fixture, check=False)
            if diagnostics:
                print("Management diagnostics:\n" + diagnostics)
        if not (success and args.keep):
            docker("rm", "-f", fixture, upstream, check=False)
            docker("volume", "rm", volume, check=False)
            docker("network", "rm", network, check=False)
            shutil.rmtree(temp)


if __name__ == "__main__":
    main()
