package cmd

import (
	"mohh2-tools/internal/server"

	"github.com/spf13/cobra"
)

var (
	tcp1Port       string
	tcp2Port       string
	udpPort        string
	logLevel       string
	tcpHostIP      string
	uhsProxyListen string
	uhsProxyTarget string
)

// serveCmd represents the serve command
var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start the game server, allowing to play online. The game must be patched with No SSL and Serverless patches.",
	Run: func(cmd *cobra.Command, args []string) {
		server.Serve(tcp1Port, tcp2Port, udpPort, logLevel, tcpHostIP, uhsProxyListen, uhsProxyTarget)
	},
}

func init() {
	rootCmd.AddCommand(serveCmd)

	serveCmd.Flags().StringVarP(&tcp1Port, "tcp1-port", "t", "21171", "TCP port for the first server (default: 21171)")
	serveCmd.Flags().StringVarP(&tcp2Port, "tcp2-port", "T", "21172", "TCP port for the second server (default: 21172)")
	serveCmd.Flags().StringVarP(&udpPort, "udp-port", "u", "1", "UDP port for the server (default: 1)")
	serveCmd.Flags().StringVarP(&logLevel, "log-level", "l", "debug", "Log level (debug, info, warn, error) (default: debug)")
	serveCmd.Flags().StringVarP(&tcpHostIP, "tcp-host-ip", "H", "", "TCP host IP to send in @dir responses (defaults to TCP_HOST_IP env var, then detected local IP)")
	serveCmd.Flags().StringVarP(&uhsProxyListen, "uhs-proxy-listen", "L", ":3660", "UDP address where the game connects when UHS proxy is enabled")
	serveCmd.Flags().StringVarP(&uhsProxyTarget, "uhs-proxy-target", "p", "", "Enable UHS UDP proxy using the mohz.exe -port address")
}
