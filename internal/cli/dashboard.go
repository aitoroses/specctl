package cli

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"

	specctl "github.com/aitoroses/specctl"
	"github.com/aitoroses/specctl/internal/application"
	"github.com/aitoroses/specctl/internal/dashboard"
)

func newDashboardCmd() *cobra.Command {
	var serve bool
	var port int
	var output string

	cmd := &cobra.Command{
		Use:   "dashboard",
		Short: "Open the spec governance dashboard",
		Args:  cobra.NoArgs,
		Long: commandLong(`Open the specctl spec governance dashboard.

Without --serve (default), generates a self-contained static site in the
output directory with all spec data pre-injected.

With --serve, starts a live HTTP server that reloads when .yaml files change.

Stdin:
  This command does not read stdin.

Output:
  Without --serve: writes static files to the output directory.
  With --serve:    serves HTTP on the given port until Ctrl+C.`,
			"INVALID_INPUT",
		),
		RunE: func(cmd *cobra.Command, args []string) error {
			svc, err := application.OpenFromWorkingDir()
			if err != nil {
				return err
			}

			if serve {
				addr := fmt.Sprintf(":%d", port)
				srv := dashboard.NewServer(svc, specctl.DashboardFS, svc.SpecsDir())

				ctx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
				defer stop()

				fmt.Fprintf(cmd.OutOrStdout(), `{"url":"http://localhost:%d","status":"serving"}`+"\n", port)

				errCh := make(chan error, 1)
				go func() {
					errCh <- srv.Start(addr)
				}()

				select {
				case <-ctx.Done():
					return nil
				case serveErr := <-errCh:
					return serveErr
				}
			}

			if err := dashboard.GenerateStatic(svc, specctl.DashboardFS, output); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), `{"output":%q,"status":"generated"}`+"\n", output)
			return nil
		},
	}

	cmd.Flags().BoolVar(&serve, "serve", false, "serve the dashboard as a live HTTP server")
	cmd.Flags().IntVar(&port, "port", 3847, "port to listen on (with --serve)")
	cmd.Flags().StringVar(&output, "output", ".specs/dashboard/", "output directory for static generation")

	return cmd
}
