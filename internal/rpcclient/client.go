package rpcclient

import (
    "github.com/gagliardetto/solana-go/rpc"
    cfg "github.com/Catorpilor/weed/internal/config"
)

type Client struct { RPC *rpc.Client }

func New(c cfg.RPCConfig) *Client { return &Client{RPC: rpc.New(c.URL)} }
