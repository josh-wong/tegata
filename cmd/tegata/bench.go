package main

import (
	"crypto/rand"
	"fmt"
	"time"

	"github.com/josh-wong/tegata/internal/crypto"
	"github.com/spf13/cobra"
)

func newBenchCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "bench",
		Short:   "Benchmark vault unlock time on this machine",
		Args:    cobra.NoArgs,
		Example: `  tegata bench`,
		RunE: func(cmd *cobra.Command, args []string) error {
			params := crypto.DefaultParams

			salt := make([]byte, 32)
			if _, err := rand.Read(salt); err != nil {
				return fmt.Errorf("generating salt: %w", err)
			}
			passphrase := make([]byte, 16)
			if _, err := rand.Read(passphrase); err != nil {
				return fmt.Errorf("generating passphrase: %w", err)
			}

			fmt.Printf("Argon2id benchmark (t=%d, m=%dMiB, p=%d):\n",
				params.Time, params.Memory/1024, params.Threads)

			var total time.Duration
			const runs = 3

			for i := 1; i <= runs; i++ {
				start := time.Now()
				key := crypto.DeriveKey(passphrase, salt, params)
				elapsed := time.Since(start)
				key.Destroy()
				total += elapsed
				fmt.Printf("  Run %d: %dms\n", i, elapsed.Milliseconds())
			}

			avg := total / runs
			fmt.Printf("  Average: %dms\n\n", avg.Milliseconds())
			fmt.Printf("Vault unlock will take approximately %dms on this machine.\n", avg.Milliseconds())
			fmt.Println("Target: under 3000ms")

			if avg > 3*time.Second {
				fmt.Println("\nWarning: unlock time exceeds 3s target. Consider reducing Argon2id parameters.")
			}

			return nil
		},
	}
}
