package main

import (
    "context"
    "flag"
    "fmt"
    "log/slog"
    "os"
    "os/signal"
    "syscall"
    "time"

    cfgpkg "github.com/Catorpilor/weed/internal/config"
    claimpkg "github.com/Catorpilor/weed/internal/claim"
    rpcpkg "github.com/Catorpilor/weed/internal/rpcclient"
    "github.com/Catorpilor/weed/internal/schedule"
    logpkg "github.com/Catorpilor/weed/internal/logging"
    "github.com/Catorpilor/weed/internal/wallet"
)

func main() {
    var (
        configPath string
        once        bool
        simulate    bool
        rpcURL      string
        intervalStr string
    )

    flag.StringVar(&configPath, "config", "configs/config.yaml", "Path to config file")
    flag.BoolVar(&once, "once", false, "Run a single claim and exit")
    flag.BoolVar(&simulate, "simulate", false, "Simulate only (no send)")
    flag.StringVar(&rpcURL, "rpc-url", "", "Override RPC URL")
    flag.StringVar(&intervalStr, "interval", "", "Override interval (e.g., 15m)")
    flag.Parse()

    cfg, err := cfgpkg.Load(configPath)
    if err != nil { fatalf("load config: %v", err) }
    // logging setup
    logpkg.Setup(cfg.Logging)
    if rpcURL != "" {
        cfg.RPC.URL = rpcURL
    }
    if intervalStr != "" {
        d, err := time.ParseDuration(intervalStr)
        if err != nil {
            fatalf("invalid interval override: %v", err)
        }
        cfg.Claim.Interval = d
    }

    if cfg.Claim.ReferenceSignature == "" {
        fatalf("claim.reference_signature is required for MVP")
    }

    ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
    defer cancel()

    // Wallet
    kp, err := wallet.Load(cfg.Wallet)
    if err != nil { fatalf("load wallet: %v", err) }

    // RPC
    rpc := rpcpkg.New(cfg.RPC)

    // Claim service: resolves accounts/data from reference signature
    claimer, err := claimpkg.NewService(ctx, rpc, kp, cfg)
    if err != nil { fatalf("init claim service: %v", err) }

    runOnce := func(ctx context.Context) error {
        res, err := claimer.Claim(ctx, claimpkg.ClaimOptions{SimulateOnly: simulate})
        if err != nil {
            return err
        }
        fmt.Println(res)
        return nil
    }

    if once {
        if err := runOnce(ctx); err != nil { fatalf("claim failed: %v", err) }
        return
    }

    // Scheduler loop.
    sched := schedule.New(cfg.Claim.Interval, cfg.Claim.JitterPct)
    slog.Info("starting auto-claimer", "interval", cfg.Claim.Interval.String(), "jitter_pct", cfg.Claim.JitterPct)
    for {
        select {
        case <-ctx.Done():
            slog.Info("shutting down")
            return
        case <-sched.Next():
            if err := runOnce(ctx); err != nil { slog.Error("claim error", "err", err) }
        }
    }
}

func fatalf(msg string, args ...any) {
    slog.Error(fmt.Sprintf(msg, args...))
    os.Exit(1)
}
