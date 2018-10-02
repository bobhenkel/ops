package main

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"strconv"
	"strings"

	api "github.com/nanovms/nvm/lepton"
	"github.com/spf13/cobra"
)

func copy(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, in)
	if err != nil {
		return err
	}
	return out.Close()
}

func checkExists(key string) bool {
	_, err := exec.LookPath(key)
	if err != nil {
		return false
	}
	return true
}

func startHypervisor(image string, port int) {
	for k := range hypervisors {
		if checkExists(k) {
			hypervisor := hypervisors[k]()
			hypervisor.start(image, port)
			break
		}
	}
}

func panicOnError(err error) {
	if err != nil {
		panic(err)
	}
}

func runCommandHandler(cmd *cobra.Command, args []string) {
	force, err := strconv.ParseBool(cmd.Flag("force").Value.String())
	if err != nil {
		panic(err)
	}
	buildImages(args[0], force)
	fmt.Printf("booting %s ...\n", api.FinalImg)
	port, err := strconv.Atoi(cmd.Flag("port").Value.String())
	if err != nil {
		panic(err)
	}
	startHypervisor(api.FinalImg, port)
}

func buildImages(userBin string, useLatest bool) {
	var err error
	if useLatest {
		err =  api.DownloadImages(callback{}, api.DevBaseUrl)
	} else {
		err =  api.DownloadImages(callback{}, api.ReleaseBaseUrl)
	}
	panicOnError(err)
	err = api.BuildImage(userBin, api.FinalImg)
	panicOnError(err)
}

func buildCommandHandler(cmd *cobra.Command, args []string) {
	buildImages(args[0], false)
}

type callback struct {
	total uint64
}

func (bc callback) Write(p []byte) (int, error) {
	n := len(p)
	bc.total += uint64(n)
	bc.printProgress()
	return n, nil
}

func (bc callback) printProgress() {
	// clear the previous line
	fmt.Printf("\r%s", strings.Repeat(" ", 35))
	fmt.Printf("\rDownloading... %v complete", bc.total)
}

func runningAsRoot() bool {
	cmd := exec.Command("id", "-u")
	output, _ := cmd.Output()
	i, _ := strconv.Atoi(string(output[:len(output)-1]))
	return i == 0
}

func netCommandHandler(cmd *cobra.Command, args []string) {
	if !runningAsRoot() {
		fmt.Println("net command needs root permission")
		return
	}
	if len(args) < 1 {
		fmt.Println("Not enough arguments.")
		return
	}
	if args[0] == "setup" {
		if err := setupBridgeNetwork(); err != nil {
			panic(err)
		}
	} else {
		if err := resetBridgeNetwork(); err != nil {
			panic(err)
		}
	}
}

func main() {
	var cmdRun = &cobra.Command{
		Use:   "run [ELF file]",
		Short: "run ELF as unikernel",
		Args:  cobra.MinimumNArgs(1),
		Run:   runCommandHandler,
	}
	var port int
	var force bool
	cmdRun.PersistentFlags().IntVarP(&port, "port", "p", -1, "port to forward")
	cmdRun.PersistentFlags().BoolVarP(&force, "force", "f", false, "use latest dev images")

	var cmdNet = &cobra.Command{
		Use:       "net",
		Args:      cobra.OnlyValidArgs,
		ValidArgs: []string{"setup", "reset"},
		Short:     "configure bridge network",
		Run:       netCommandHandler,
	}

	var cmdBuild = &cobra.Command{
		Use:   "build [ELF file]",
		Short: "build an image from ELF",
		Args:  cobra.MinimumNArgs(1),
		Run:   buildCommandHandler,
	}

	var rootCmd = &cobra.Command{Use: "nvm"}
	rootCmd.AddCommand(cmdRun)
	rootCmd.AddCommand(cmdNet)
	rootCmd.AddCommand(cmdBuild)
	rootCmd.Execute()
}
