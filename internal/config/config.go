package config

import (
    "errors"
    "io/ioutil"
    "os"
    "time"

    "gopkg.in/yaml.v3"
)

type RPCConfig struct {
    URL        string        `yaml:"url"`
    Commitment string        `yaml:"commitment"`
    Timeout    time.Duration `yaml:"timeout"`
}

type WalletConfig struct {
    KeypairPath   string `yaml:"keypair_path"`
    SecretKeyB58  string `yaml:"secret_key_b58"`
}

type ClaimConfig struct {
    ReferenceSignature string        `yaml:"reference_signature"`
    ProgramID          string        `yaml:"program_id"`
    TokenProgramID     string        `yaml:"token_program_id"`
    Interval           time.Duration `yaml:"interval"`
    JitterPct          float64       `yaml:"jitter_pct"`
}

type FeesConfig struct {
    PriorityMicrolamports uint64 `yaml:"priority_microlamports"`
    ComputeUnitLimit      uint32 `yaml:"compute_unit_limit"`
}

type Config struct {
    RPC       RPCConfig    `yaml:"rpc"`
    Wallet    WalletConfig `yaml:"wallet"`
    Claim     ClaimConfig  `yaml:"claim"`
    Fees      FeesConfig   `yaml:"fees"`
    Confirm   string       `yaml:"confirm"`
    MaxRetries int         `yaml:"max_retries"`
    Logging   LoggingConfig `yaml:"logging"`
}

func Load(path string) (*Config, error) {
    b, err := ioutil.ReadFile(path)
    if err != nil {
        return nil, err
    }
    var c Config
    if err := yaml.Unmarshal(b, &c); err != nil {
        return nil, err
    }
    // Env overrides
    if v := os.Getenv("RPC_URL"); v != "" {
        c.RPC.URL = v
    }
    if v := os.Getenv("SECRET_KEY_B58"); v != "" {
        c.Wallet.SecretKeyB58 = v
    }
    if c.Claim.Interval == 0 {
        c.Claim.Interval = 15 * time.Minute
    }
    if c.Claim.JitterPct == 0 {
        c.Claim.JitterPct = 0.2
    }
    if c.RPC.Commitment == "" {
        c.RPC.Commitment = "confirmed"
    }
    if c.RPC.Timeout == 0 {
        c.RPC.Timeout = 10 * time.Second
    }
    if c.MaxRetries == 0 {
        c.MaxRetries = 3
    }
    if c.Logging.Level == "" { c.Logging.Level = "info" }
    if c.Logging.Format == "" { c.Logging.Format = "json" }
    if c.Claim.ProgramID == "" {
        return nil, errors.New("claim.program_id required")
    }
    return &c, nil
}

type LoggingConfig struct {
    Level  string `yaml:"level"`  // debug|info|warn|error
    Format string `yaml:"format"` // json|text
}
