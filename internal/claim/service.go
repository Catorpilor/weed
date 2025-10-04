package claim

import (
    "context"
    "encoding/base64"
    "errors"
    "fmt"
    "strings"
    "time"

    cfgpkg "github.com/Catorpilor/weed/internal/config"
    rpcpkg "github.com/Catorpilor/weed/internal/rpcclient"
    "github.com/gagliardetto/solana-go"
    "github.com/gagliardetto/solana-go/programs/compute-budget"
    lookup "github.com/gagliardetto/solana-go/programs/address-lookup-table"
    "github.com/gagliardetto/solana-go/rpc"
)

type ClaimOptions struct {
    SimulateOnly bool
}

type Service struct {
    rpc        *rpcpkg.Client
    wallet     solana.PrivateKey
    cfg        *cfgpkg.Config
    // Learned from the reference transaction
    claimProgram solana.PublicKey
    claimIxData  []byte
    claimIxAccts []*solana.AccountMeta
    tokenProgram solana.PublicKey
    mint         solana.PublicKey
}

func NewService(ctx context.Context, rpc *rpcpkg.Client, wallet solana.PrivateKey, cfg *cfgpkg.Config) (*Service, error) {
    s := &Service{rpc: rpc, wallet: wallet, cfg: cfg}
    // Parse required IDs
    var err error
    s.claimProgram, err = solana.PublicKeyFromBase58(cfg.Claim.ProgramID)
    if err != nil {
        return nil, fmt.Errorf("invalid claim.program_id: %w", err)
    }
    if cfg.Claim.TokenProgramID != "" {
        if s.tokenProgram, err = solana.PublicKeyFromBase58(cfg.Claim.TokenProgramID); err != nil {
            return nil, fmt.Errorf("invalid claim.token_program_id: %w", err)
        }
    }

    if err := s.learnFromReference(ctx, cfg.Claim.ReferenceSignature); err != nil {
        return nil, err
    }
    return s, nil
}

// learnFromReference downloads the provided transaction and extracts:
// - raw instruction data and account metas for the claim program
// - token program id used (Token-2022 vs Tokenkeg)
// - token mint address
func (s *Service) learnFromReference(ctx context.Context, sig string) error {
    signature, err := solana.SignatureFromBase58(strings.TrimSpace(sig))
    if err != nil {
        return fmt.Errorf("invalid reference_signature: %w", err)
    }
    // Use JSON encoding so we can resolve v0 messages cleanly.
    res, err := s.rpc.RPC.GetTransaction(ctx, signature, &rpc.GetTransactionOpts{
        Encoding:   solana.EncodingBase64,
        Commitment: rpc.CommitmentConfirmed,
    })
    if err != nil {
        return fmt.Errorf("getTransaction: %w", err)
    }
    tx, err := res.Transaction.GetTransaction()
    if err != nil {
        return fmt.Errorf("decode tx: %w", err)
    }
    msg := tx.Message
    // If the transaction is v0 and uses address lookups, resolve them.
    if msg.IsVersioned() && msg.AddressTableLookups.NumLookups() > 0 {
        // Fetch each address table and set it in the message.
        tableIDs := msg.AddressTableLookups.GetTableIDs()
        resolutions := make(map[solana.PublicKey]solana.PublicKeySlice)
        for _, id := range tableIDs {
            acc, err := s.rpc.RPC.GetAccountInfo(ctx, id)
            if err != nil || acc == nil {
                return fmt.Errorf("fetch address table %s: %w", id, err)
            }
            table, derr := lookup.DecodeAddressLookupTableState(acc.GetBinary())
            if derr != nil { return fmt.Errorf("decode address table %s: %w", id, derr) }
            resolutions[id] = table.Addresses
        }
        if err := msg.SetAddressTables(resolutions); err != nil {
            return err
        }
        if err := msg.ResolveLookups(); err != nil {
            return err
        }
    }

    // Iterate top-level instructions; find our claim program and token program (if any top-level usage).
    for _, ci := range msg.Instructions {
        // Resolve program id pubkey
        prog := msg.AccountKeys[ci.ProgramIDIndex]
        if prog.Equals(s.claimProgram) {
            // Resolve account metas with proper signer/writable flags
            metas, err := ci.ResolveInstructionAccounts(&msg)
            if err != nil {
                return fmt.Errorf("resolve claim accounts: %w", err)
            }
            s.claimIxAccts = metas
            // ci.Data is base58-decoded bytes via JSON decoding in solana-go types.
            s.claimIxData = append([]byte(nil), ci.Data...)
        }
        // Token program (Token-2022 or Tokenkeg)
        // Well-known program IDs
        if prog.String() == "TokenzQdBNbLqP5VEhdkAS6EPFLC1PHnBqCXEpPxuEb" ||
            prog.String() == "TokenkegQfeZyiNwAJbNbGKPFXCWuBvf9Ss623VQ5DA" {
            s.tokenProgram = prog
        }
    }
    // If token program wasn’t found at the top level, scan inner instructions in meta.
    if s.tokenProgram.IsZero() && res.Meta != nil {
        for _, inner := range res.Meta.InnerInstructions {
            for _, ici := range inner.Instructions {
                prog, _ := msg.Program(ici.ProgramIDIndex)
                if prog.String() == "TokenzQdBNbLqP5VEhdkAS6EPFLC1PHnBqCXEpPxuEb" ||
                    prog.String() == "TokenkegQfeZyiNwAJbNbGKPFXCWuBvf9Ss623VQ5DA" {
                    s.tokenProgram = prog
                    // Try to infer mint from first account if present
                    if len(ici.Accounts) > 0 {
                        if mk, err := msg.Account(ici.Accounts[0]); err == nil {
                            s.mint = mk
                        }
                    }
                }
            }
        }
    }
    // As a fallback, infer mint from post token balances.
    if s.mint.IsZero() && res.Meta != nil && len(res.Meta.PostTokenBalances) > 0 {
        s.mint = res.Meta.PostTokenBalances[0].Mint
    }
    if len(s.claimIxAccts) == 0 || len(s.claimIxData) == 0 {
        return errors.New("could not extract claim instruction from reference transaction")
    }
    if s.tokenProgram.IsZero() && s.cfg.Claim.TokenProgramID == "" {
        return errors.New("could not detect token program id; set claim.token_program_id")
    }
    return nil
}

// Claim builds, simulates and optionally sends a new claim transaction.
func (s *Service) Claim(ctx context.Context, opts ClaimOptions) (string, error) {
    // Latest blockhash
    lb, err := s.rpc.RPC.GetLatestBlockhash(ctx, rpc.CommitmentType(s.cfg.RPC.Commitment))
    if err != nil {
        return "", fmt.Errorf("getLatestBlockhash: %w", err)
    }

    // Build instruction list
    var ixs []solana.Instruction

    // Compute budget knobs
    if s.cfg.Fees.ComputeUnitLimit > 0 {
        ixs = append(ixs, computebudget.NewSetComputeUnitLimitInstruction(s.cfg.Fees.ComputeUnitLimit).Build())
    }
    if s.cfg.Fees.PriorityMicrolamports > 0 {
        ixs = append(ixs, computebudget.NewSetComputeUnitPriceInstruction(s.cfg.Fees.PriorityMicrolamports).Build())
    }

    // Ensure ATA exists (create if missing) when we know the mint.
    if !s.mint.IsZero() {
        // Derive ATA for current wallet and detected token program.
        tokenProg := s.tokenProgram
        if tokenProg.IsZero() {
            // fallback to config override
            tokenProg, _ = solana.PublicKeyFromBase58(s.cfg.Claim.TokenProgramID)
        }
        ataAddr, _ := findATAWithProgram(s.wallet.PublicKey(), s.mint, tokenProg)
        // Check if ATA account exists; create only if missing.
        accInfo, _ := s.rpc.RPC.GetAccountInfo(ctx, ataAddr)
        if accInfo == nil || accInfo.Value == nil || accInfo.Value.Lamports == 0 {
            accs := []*solana.AccountMeta{
                {PublicKey: s.wallet.PublicKey(), IsSigner: true, IsWritable: true},
                {PublicKey: ataAddr, IsSigner: false, IsWritable: true},
                {PublicKey: s.wallet.PublicKey(), IsSigner: false, IsWritable: false},
                {PublicKey: s.mint, IsSigner: false, IsWritable: false},
                {PublicKey: solana.SystemProgramID, IsSigner: false, IsWritable: false},
                {PublicKey: tokenProg, IsSigner: false, IsWritable: false},
            }
            ixs = append(ixs, solana.NewInstruction(solana.SPLAssociatedTokenAccountProgramID, accs, []byte{}))
        }
    }

    // Claim instruction reconstructed from template
    claimIx := solana.NewInstruction(s.claimProgram, s.claimIxAccts, s.claimIxData)
    ixs = append(ixs, claimIx)

    // Build transaction
    tx, err := solana.NewTransaction(ixs, lb.Value.Blockhash, solana.TransactionPayer(s.wallet.PublicKey()))
    if err != nil {
        return "", fmt.Errorf("new tx: %w", err)
    }
    // Sign
    _, err = tx.Sign(func(key solana.PublicKey) *solana.PrivateKey { if key.Equals(s.wallet.PublicKey()) { return &s.wallet }; return nil })
    if err != nil {
        return "", fmt.Errorf("sign: %w", err)
    }

    // Simulate first
    sim, err := s.rpc.RPC.SimulateTransactionWithOpts(ctx, tx, &rpc.SimulateTransactionOpts{
        SigVerify:              false,
        ReplaceRecentBlockhash: true,
    })
    if err != nil {
        return "", fmt.Errorf("simulate: %w", err)
    }
    if sim.Value.Err != nil {
        // include logs when available
        logs := strings.Join(sim.Value.Logs, "\n")
        return "", fmt.Errorf("simulation error: %v\nlogs:\n%s", sim.Value.Err, logs)
    }
    if opts.SimulateOnly {
        return "simulation OK", nil
    }

    // Send
    sig, err := s.rpc.RPC.SendTransactionWithOpts(ctx, tx, rpc.TransactionOpts{
        SkipPreflight:       false,
        PreflightCommitment: rpc.CommitmentType(s.cfg.RPC.Commitment),
        MaxRetries:          &[]uint{uint(s.cfg.MaxRetries)}[0],
    })
    if err != nil {
        return "", fmt.Errorf("send: %w", err)
    }

    // Optionally confirm by polling
    // Basic naive confirmation loop
    deadline := time.Now().Add(30 * time.Second)
    for time.Now().Before(deadline) {
        st, err := s.rpc.RPC.GetSignatureStatuses(ctx, false, sig)
        if err == nil && st != nil && len(st.Value) > 0 && st.Value[0] != nil {
            cs := st.Value[0].ConfirmationStatus
            if cs == rpc.ConfirmationStatusFinalized || cs == rpc.ConfirmationStatusConfirmed {
                return fmt.Sprintf("submitted: %s", sig.String()), nil
            }
        }
        time.Sleep(1 * time.Second)
    }
    // If not confirmed in time, still return signature
    return fmt.Sprintf("submitted (pending): %s", sig.String()), nil
}

// helper for debugging base64 data prints
func b64(b []byte) string { return base64.StdEncoding.EncodeToString(b) }

// findATAWithProgram derives the associated token account using a specific token program ID (Token-2022 or Tokenkeg).
func findATAWithProgram(owner, mint, tokenProgram solana.PublicKey) (solana.PublicKey, uint8) {
    addr, bump, _ := solana.FindProgramAddress([][]byte{
        owner[:],
        tokenProgram[:],
        mint[:],
    }, solana.SPLAssociatedTokenAccountProgramID)
    return addr, bump
}
