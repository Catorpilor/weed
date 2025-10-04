Weed (Auto-Claimer)

Simple CLI that reconstructs and submits a “claim” transaction on Solana by learning from a reference transaction. It can run once or on an interval with jitter, simulate before sending, and tweak compute budget/priority fees.

— Built with Go 1.21+, lives under `cmd/claimer`.

**What It Does**
- Loads config from `configs/config.yaml` (or `--config` flag) and env overrides.
- Loads a wallet from a keypair JSON file or a base58 private key.
- Fetches the `claim.reference_signature` transaction and extracts:
  - the claim program id, instruction data, and accounts
  - token program (Token Program 2022 or Tokenkeg) and token mint when possible
- Optionally creates the associated token account if missing.
- Applies compute-unit limit and priority fee when configured.
- Simulates, then optionally sends a fresh claim transaction and prints the result.
- When not `--once`, schedules repeat runs at an interval with jitter.

**Repo Layout**
- `cmd/claimer`: CLI entrypoint (flags, loop) — see `cmd/claimer/main.go:1`.
- `internal/config`: YAML + env config loader and defaults — see `internal/config/config.go:1`.
- `internal/wallet`: wallet loading from keypair file or base58 — `internal/wallet/wallet.go:1`.
- `internal/rpcclient`: thin wrapper around `solana-go` RPC client.
- `internal/claim`: reference-tx analysis and claim transaction builder.
- `internal/schedule`: jittered interval scheduler.
- `configs/`: example configuration (added by this commit).

**Build**
- `go build ./cmd/claimer`
- Or run directly: `go run ./cmd/claimer --help`

**Run (Local)**
- First edit `configs/config.yaml` (added below). At minimum set:
  - `claim.program_id` (required)
  - `claim.reference_signature` (required)
  - one of `wallet.keypair_path` or `SECRET_KEY_B58` (see Security)

- Examples:
  - Single run, simulate only: `go run ./cmd/claimer --simulate --once`
  - Override RPC at runtime: `go run ./cmd/claimer --rpc-url=https://api.mainnet-beta.solana.com`
  - Override interval: `go run ./cmd/claimer --interval=20m`

**CLI Flags**
- `--config` (string): path to YAML, default `configs/config.yaml`.
- `--once` (bool): run a single claim and exit.
- `--simulate` (bool): simulate only (no send).
- `--rpc-url` (string): override `rpc.url` from config/env.
- `--interval` (string): override `claim.interval` (e.g., `15m`).

**Environment Overrides**
- `RPC_URL`: sets `rpc.url`.
- `SECRET_KEY_B58`: base58-encoded ed25519 private key (see Security).

**Docker**
- Build image: `docker build -t weed-claimer:local .`
- Run (mount config):
  - `docker run --rm -v "$(pwd)/configs:/configs:ro" weed-claimer:local --config=/configs/config.yaml --once`
- Compose: see `docker-compose.yml` (uses `ghcr.io/catorpilor/weed:main`). Edit `configs/config.yaml` and uncomment flags as needed.

**Configuration**
- A starter `configs/config.yaml` is included. Key fields and defaults are enforced by `internal/config`:
  - `rpc.commitment`: defaults to `confirmed`
  - `rpc.timeout`: defaults to `10s`
  - `claim.interval`: defaults to `15m`
  - `claim.jitter_pct`: defaults to `0.2`
  - `max_retries`: defaults to `3`
  - `logging.level`: defaults to `info`
  - `logging.format`: defaults to `json`
- Required values:
  - `claim.program_id` must be set
  - `claim.reference_signature` must be set
- Optional but sometimes required:
  - `claim.token_program_id` — set to Tokenkeg or Token-2022 if auto-detection from the reference tx fails

Example config snippet with your reference signature and program id:

```
claim:
  reference_signature: "2xS8BchUy4GJffLEPb5mRtfwz7Gyh6YEfX75U45jh8rVuPXxyTfZBeFt6Fo4LB2FmDMmqj6x2ixE52MMdan1TF8J"
  program_id: "5f6jnqJUNkUvWvwuqvTvmbQS1REjSsgTtZ75KercWNnG"
  # token_program_id: "TokenkegQfeZyiNwAJbNbGKPFXCWuBvf9Ss623VQ5DA"  # optional override
  interval: "15m"
  jitter_pct: 0.2
```

Note on configs/config.yaml (gitignored)
- `configs/config.yaml` is intentionally listed in `.gitignore` so you don’t commit secrets.
- Create it locally before running. You can use the snippet above or generate a minimal file with:

```
mkdir -p configs
cat > configs/config.yaml <<'YAML'
rpc:
  url: "https://api.mainnet-beta.solana.com"
  commitment: "confirmed"
  timeout: "10s"
wallet:
  keypair_path: ""         # set to your keypair path, preferred
  secret_key_b58: ""       # or set via env SECRET_KEY_B58
claim:
  reference_signature: "2xS8BchUy4GJffLEPb5mRtfwz7Gyh6YEfX75U45jh8rVuPXxyTfZBeFt6Fo4LB2FmDMmqj6x2ixE52MMdan1TF8J"
  program_id: "5f6jnqJUNkUvWvwuqvTvmbQS1REjSsgTtZ75KercWNnG"
  token_program_id: ""      # optional override (Tokenkeg or Token-2022)
  interval: "15m"
  jitter_pct: 0.2
fees:
  priority_microlamports: 0
  compute_unit_limit: 0
confirm: "confirmed"
max_retries: 3
logging:
  level: "info"
  format: "json"
YAML
```

**Security Notes**
- Prefer `wallet.keypair_path` (file-mounted ed25519 array) over `SECRET_KEY_B58`.
- Never commit private keys; keep `configs/` world-readable but not secrets.
- Logs are minimal by default (`json`, `info`). Avoid raising log level in production.

**Testing & Dev**
- Lint/format: `gofmt -s -w . && go vet ./...`
- Unit tests (if present): `go test ./... -race -cover`
- Integration (RPC required): `go test -tags=integration ./...`

**Troubleshooting**
- "claim.program_id required": set `claim.program_id` in config.
- "claim.reference_signature is required": set a valid signature in config.
- "could not detect token program id": set `claim.token_program_id` explicitly to either:
  - Token Program: `TokenkegQfeZyiNwAJbNbGKPFXCWuBvf9Ss623VQ5DA`
  - Token-2022: `TokenzQdBNbLqP5VEhdkAS6EPFLC1PHnBqCXEpPxuEb`
- Simulation errors: run with `--simulate` and inspect printed logs; adjust compute budget or fees in `fees.*`.
