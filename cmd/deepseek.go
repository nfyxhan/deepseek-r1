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
	var cmd = &cobra.Command{
		Use:   "serve",
		Short: "deepseek server",
		Long: `deepseek server
`,
		Run: func(cmd *cobra.Command, args []string) {
			if err := ollama.ChatServer(url, port, time.Second); err != nil {
				panic(err)
			}
		},
	}
	cmd.PersistentFlags().StringVarP(&url, "url", "u", "http://localhost:11434", "url")
	// cmd.PersistentFlags().StringVarP(&model, "model", "d", "deepseek-r1:7b", "model")
	cmd.PersistentFlags().StringVarP(&port, "port", "p", "1203", "port")
	rootCmd.AddCommand(cmd)
}
