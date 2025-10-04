package wallet

import (
    "crypto/ed25519"
    "encoding/json"
    "fmt"
    "io/ioutil"
    "os"

    "github.com/gagliardetto/solana-go"
    "github.com/mr-tron/base58"
    cfg "github.com/Catorpilor/weed/internal/config"
)

// Load returns a solana.PrivateKey from config.
func Load(c cfg.WalletConfig) (solana.PrivateKey, error) {
    if sk := c.SecretKeyB58; sk != "" {
        bytes, err := base58.Decode(sk)
        if err != nil {
            return nil, fmt.Errorf("decode base58: %w", err)
        }
        if l := len(bytes); l != ed25519.PrivateKeySize {
            return nil, fmt.Errorf("secret_key_b58 has %d bytes; want %d", l, ed25519.PrivateKeySize)
        }
        return solana.PrivateKey(bytes), nil
    }
    if c.KeypairPath != "" {
        path := expandHome(os.ExpandEnv(c.KeypairPath))
        b, err := ioutil.ReadFile(path)
        if err != nil {
            return nil, err
        }
        var arr []byte
        if err := json.Unmarshal(b, &arr); err != nil {
            return nil, fmt.Errorf("parse keypair file: %w", err)
        }
        if l := len(arr); l != ed25519.PrivateKeySize {
            return nil, fmt.Errorf("keypair file has %d bytes; want %d", l, ed25519.PrivateKeySize)
        }
        return solana.PrivateKey(arr), nil
    }
    return nil, fmt.Errorf("no wallet configured: set keypair_path or SECRET_KEY_B58")
}

func expandHome(p string) string {
    if p == "" { return p }
    if p[0] == '~' {
        if len(p) == 1 { return os.Getenv("HOME") }
        if p[1] == '/' { return os.Getenv("HOME") + p[1:] }
    }
    return p
}
