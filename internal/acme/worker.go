package acme

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/go-acme/lego/v5/challenge"
	"github.com/go-acme/lego/v5/challenge/dns01"
)

type workerMessage struct {
	Stage      string  `json:"stage,omitempty"`
	Record     *Record `json:"record,omitempty"`
	Checkpoint bool    `json:"checkpoint,omitempty"`
	Done       bool    `json:"done,omitempty"`
	OK         bool    `json:"ok,omitempty"`
}

// Every native factory runs in a separate process because upstream reads process
// environment variables. Credentials never enter command arguments or logs.
func workerEnvironment(in Input) []string {
	blocked := map[string]bool{}
	for _, p := range catalog.Providers {
		for _, f := range p.Fields {
			blocked[f.Key] = true
			blocked[f.Key+"_FILE"] = true
		}
	}
	env := []string{}
	for _, pair := range os.Environ() {
		k, _, _ := strings.Cut(pair, "=")
		if !blocked[k] {
			env = append(env, pair)
		}
	}
	values := map[string]string{}
	for k, v := range in.DNSConfig {
		if v != "" {
			values[k] = v
		}
	}
	// Apply the shared DNS wait setting unless a provider-specific override is set.
	if p := providerDefinition(in.Provider); p != nil {
		for _, f := range p.Fields {
			if strings.HasSuffix(f.Key, "_PROPAGATION_TIMEOUT") && values[f.Key] == "" {
				values[f.Key] = strconv.Itoa(in.PropagationSeconds)
			}
		}
	}
	for k, v := range values {
		env = append(env, k+"="+v)
	}
	return env
}

func issueInWorker(ctx context.Context, r *Record, checkpoint func() error, progress func(string)) error {
	executable, err := os.Executable()
	if err != nil {
		return err
	}
	out, writer, err := os.Pipe()
	if err != nil {
		return err
	}
	defer out.Close()
	defer writer.Close()
	childCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	cmd := exec.CommandContext(childCtx, executable, "acme-worker")
	cmd.Env = workerEnvironment(r.Input)
	cmd.ExtraFiles = []*os.File{writer}
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return err
	}
	defer stdin.Close()
	if err = cmd.Start(); err != nil {
		return err
	}
	defer func() { cancel(); _ = cmd.Wait() }()
	_ = writer.Close()
	input := json.NewEncoder(stdin)
	if err = input.Encode(r); err != nil {
		return err
	}
	output := json.NewDecoder(out)
	for {
		var message workerMessage
		if err = output.Decode(&message); err != nil {
			return errors.New("DNS 申请子进程中断")
		}
		if message.Stage != "" {
			progress(message.Stage)
		}
		if message.Record != nil {
			job := r.Job
			*r = *message.Record
			r.Job = job
		}
		if message.Checkpoint {
			if err = checkpoint(); err != nil {
				return err
			}
			if err = input.Encode(true); err != nil {
				return err
			}
		}
		if message.Done {
			if !message.OK {
				return errors.New("DNS 服务商认证或证书申请失败，请检查服务商配置")
			}
			return nil
		}
	}
}

// RunWorker is an internal executable mode. A dedicated pipe (fd 3) keeps
// third-party stdout/stderr separate from private checkpoint messages.
func RunWorker() int { return runWorker(nil) }

func runWorker(options []dns01.ChallengeOption) int {
	out := os.NewFile(3, "acme-checkpoints")
	if out == nil {
		return 1
	}
	defer out.Close()
	input := json.NewDecoder(io.LimitReader(os.Stdin, 16<<20))
	var r Record
	if err := input.Decode(&r); err != nil {
		return 1
	}
	if err := validate(&r.Input); err != nil {
		return 1
	}
	output := json.NewEncoder(out)
	ctx, cancel := context.WithTimeout(context.Background(), 35*time.Minute)
	defer cancel()
	checkpoint := func() error {
		if err := output.Encode(workerMessage{Record: &r, Checkpoint: true}); err != nil {
			return err
		}
		var ok bool
		if err := input.Decode(&ok); err != nil {
			return err
		}
		if !ok {
			return errors.New("checkpoint failed")
		}
		return nil
	}
	progress := func(stage string) { _ = output.Encode(workerMessage{Stage: stage}) }
	factory := func(in Input) (challenge.Provider, error) {
		progress("初始化 DNS 服务商")
		return newNativeDNSProvider(in.Provider)
	}
	err := issueWithDNS(ctx, &r, checkpoint, progress, factory, options)
	if output.Encode(workerMessage{Record: &r, Done: true, OK: err == nil}) != nil {
		return 1
	}
	if err != nil {
		return 1
	}
	return 0
}
