"""Disposable HTTP and UDP upstream used only by docker-integration.py."""
import http.server
import socket
import threading


class Handler(http.server.BaseHTTPRequestHandler):
    def do_GET(self):
        self.send_response(200)
        self.end_headers()
        self.wfile.write(b"DOCKER_PROXY_OK")

    def log_message(self, *args):
        pass


def echo():
    with socket.socket(socket.AF_INET, socket.SOCK_DGRAM) as server:
        server.bind(("0.0.0.0", 9002))
        while True:
            data, address = server.recvfrom(4096)
            server.sendto(data, address)


threading.Thread(target=echo, daemon=True).start()
http.server.ThreadingHTTPServer(("0.0.0.0", 8000), Handler).serve_forever()
