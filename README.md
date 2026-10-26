# eurofish

**eurofish** is an open-source, self-hosted budgeting calendar. Put your income
and expenses on a calendar — one-off or repeating — and see your projected
balance for every single day ahead. Share a calendar with your partner or
household so everyone plans with the same numbers.

- 📅 Month view with planned transactions and the end-of-day balance on every day
- 🔁 Repeating transactions: daily, weekly, every 2 weeks, monthly, quarterly, yearly (with optional end date), plus "remove this day only"
- 🚨 Days that would go negative are highlighted; monthly start/income/expenses/end/lowest summary
- 👥 Multiple calendars per user, shared with others as *editor* or *viewer*
- 🔐 Self-service accounts (e-mail + password), optionally limited to your home network; Google/Apple sign-in is prepared in the data model
- 📦 One static binary with the web UI embedded — runs as a `FROM scratch` container, a systemd service or as PID 1 in a microVM
- 📈 Scales from a laptop (SQLite + in-process cache) to many stateless replicas (MariaDB + Valkey) by changing environment variables only

## Quick start (home laptop / home-lab PC)

### Option A — Docker (recommended)

```sh
cp .env.example .env        # optional: restrict sign-up to your LAN, etc.
docker compose up -d --build
```

Open `http://<ip-of-this-machine>:8080` from any device on your home network,
click **Create account**, and start planning. Data lives in the
`eurofish-data` volume (a single SQLite file).

### Option B — plain binary (Go 1.23+)

```sh
make build                  # -> bin/eurofish (static, CGO-free)
./bin/eurofish              # listens on :8080, stores data in ./data
```

Install it as a hardened systemd service on any Linux machine or VM:

```sh
sudo ./scripts/install.sh   # binary -> /usr/local/bin, data -> /var/lib/eurofish
```

> Find your machine's LAN address with `ip addr` (Linux), `ipconfig` (Windows)
> or `ipconfig getifaddr en0` (macOS) and allow TCP 8080 in its firewall.

## Deployment options

| Target | How | Storage / cache |
|---|---|---|
| Laptop, home lab | `docker compose up -d --build` or `make run` | SQLite file + in-process cache |
| Any Linux VM / bare metal | `sudo ./scripts/install.sh` (systemd) | SQLite file + in-process cache |
| MicroVM (Firecracker, Cloud Hypervisor) | `make rootfs` → `dist/eurofish-rootfs.ext4`, see `deploy/microvm/` | SQLite inside the rootfs (or MariaDB) |
| Scaled-out containers | `docker compose -f compose.scale.yaml up -d --build` | MariaDB + Valkey, nginx load balancer, 3 replicas |
| Kubernetes / cloud | `deploy/k8s/eurofish.yaml` | Managed MariaDB/MySQL + Valkey/Redis |

The container image is built `FROM scratch`: it contains only the eurofish
binary and CA certificates, runs as a non-root user and has a built-in
`HEALTHCHECK` (`eurofish healthcheck`).

## Configuration

Everything is configured through environment variables; all are optional for
a single node.

| Variable | Default | Description |
|---|---|---|
| `EUROFISH_ADDR` | `:8080` | Listen address (all interfaces → reachable from the LAN) |
| `EUROFISH_DATA_DIR` | `./data` (`/data` in the container) | Directory for the SQLite database |
| `EUROFISH_DB_DRIVER` | `sqlite` | `sqlite` or `mariadb` (alias `mysql`) |
| `EUROFISH_DB_DSN` | `file:<data dir>/eurofish.db` | e.g. `user:pass@tcp(host:3306)/eurofish?charset=utf8mb4` for MariaDB |
| `EUROFISH_CACHE_URL` | `memory` | `memory` or `redis://host:6379/0` (Valkey or Redis; `valkey://` also accepted) |
| `EUROFISH_ALLOW_SIGNUP` | `true` | Allow self-service account creation |
| `EUROFISH_SIGNUP_CIDRS` | *(empty = anyone)* | Comma-separated networks allowed to sign up, e.g. `192.168.0.0/16,10.0.0.0/8` |
| `EUROFISH_SESSION_TTL` | `720h` | Login session lifetime |
| `EUROFISH_COOKIE_SECURE` | `false` | Set `true` when served over HTTPS |
| `EUROFISH_PROXY_HEADER` | *(empty)* | Client-IP header when behind a trusted reverse proxy, e.g. `X-Forwarded-For` |
| `EUROFISH_LOG_REQUESTS` | `true` | Access log on stdout |

## Commands

```
eurofish serve        # default: migrate the database and start the server
eurofish migrate      # only apply database migrations, then exit
eurofish healthcheck  # exit 0 if the local server answers /healthz
eurofish version
```

## Development

```sh
make run      # build and start on :8080
make test     # unit tests (recurrence + balance engine)
```

The web UI is plain HTML/CSS/JavaScript in `web/static/` (no build step) and is
embedded into the binary with `go:embed`. Rebuild the binary after editing it.

See [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md) for the design, the API and
the scaling model.

## License

MIT — see [LICENSE](LICENSE).

This is a test line.
