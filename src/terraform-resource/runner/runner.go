package runner

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"syscall"
)

type Runner struct {
	Stdout io.Writer
	Stderr io.Writer
	cmd    *exec.Cmd
	sigs   chan os.Signal
	logger io.Writer
}

func New(cmd *exec.Cmd, logger io.Writer) *Runner {
	// Ensure that child is started in process group
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	r := &Runner{
		cmd:    cmd,
		sigs:   make(chan os.Signal, 1),
		logger: logger,
	}
	r.startSignalHandler()

	return r
}

func (r *Runner) Run() error {
	r.cmd.Stdout = r.Stdout
	r.cmd.Stderr = r.Stderr
	err := r.cmd.Run()

	r.stopSignalHandler()
	return err
}

func (r *Runner) CombinedOutput() ([]byte, error) {
	out, err := r.cmd.CombinedOutput()
	r.stopSignalHandler()
	return out, err
}

func (r *Runner) Output() ([]byte, error) {
	out, err := r.cmd.Output()
	r.stopSignalHandler()
	return out, err
}

func (r *Runner) terminate() {
	if r.cmd.Process != nil {
		processGroup := -r.cmd.Process.Pid
		if err := syscall.Kill(processGroup, syscall.SIGKILL); err != nil {
			fmt.Fprintf(r.logger, "** Error signaling process group %d: %s\n", processGroup, err)
		}
	} else {
		fmt.Fprintln(r.logger, "** Process already terminated.")
	}

	fmt.Fprintln(r.logger, "** Exiting due to signal")
	os.Exit(1)
}

func (r *Runner) startSignalHandler() {
	signal.Notify(r.sigs, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		for s := range r.sigs {
			fmt.Fprintf(r.logger, "** Received signal: %s\n", s)
			r.terminate()
		}
	}()
}

func (r *Runner) stopSignalHandler() {
	signal.Reset(syscall.SIGINT, syscall.SIGTERM)
	close(r.sigs)
}
