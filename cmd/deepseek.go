/*
 */
package cmd

import (
	"time"

	"github.com/nfyxhan/deepseek-r1/pkg/ollama"

	"github.com/spf13/cobra"
)

func init() {
	var url string
	var port string
	var qps float64
	var debug bool
	var cmd = &cobra.Command{
		Use:   "serve",
		Short: "deepseek server",
		Long: `deepseek server
`,
		Run: func(cmd *cobra.Command, args []string) {
			if err := ollama.ChatServer(url, port, qps, time.Second); err != nil {
				panic(err)
			}
		},
	}
	cmd.PersistentFlags().StringVarP(&url, "url", "u", "http://localhost:11434", "url")
	cmd.PersistentFlags().BoolVarP(&debug, "debug", "d", true, "debug")
	cmd.PersistentFlags().StringVarP(&port, "port", "p", "1203", "port")
	cmd.PersistentFlags().Float64VarP(&qps, "qps", "q", 0.1, "qps")
	rootCmd.AddCommand(cmd)
}
