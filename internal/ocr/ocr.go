package ocr

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

type Client interface {
	Recognize(ctx context.Context, imageBytes []byte) (string, error)
	Enabled() bool
}

type Settings struct {
	Enable          bool
	UseGPU          bool
	GPUDevice       string
	AutoDownload    bool
	RepoURL         string
	ModelRepoDir    string
	PythonBin       string
	BridgeScript    string
	TimeoutSec      int
	DownloadTimeout time.Duration
}

type disabledClient struct{}

func (d disabledClient) Recognize(_ context.Context, _ []byte) (string, error) {
	return "", fmt.Errorf("ocr is disabled")
}

func (d disabledClient) Enabled() bool { return false }

type pythonClient struct {
	settings Settings
}

func (p *pythonClient) Enabled() bool { return true }

func (p *pythonClient) Recognize(ctx context.Context, imageBytes []byte) (string, error) {
	if len(imageBytes) == 0 {
		return "", fmt.Errorf("empty image")
	}
	timeout := p.settings.TimeoutSec
	if timeout <= 0 {
		timeout = 30
	}
	runCtx, cancel := context.WithTimeout(ctx, time.Duration(timeout)*time.Second)
	defer cancel()

	payload := map[string]string{"image_base64": base64.StdEncoding.EncodeToString(imageBytes)}
	in, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("marshal ocr payload: %w", err)
	}

	args := []string{p.settings.BridgeScript, "--repo", p.settings.ModelRepoDir}
	if p.settings.UseGPU {
		args = append(args, "--gpu", "--gpu-id", p.settings.GPUDevice)
	}
	cmd := exec.CommandContext(runCtx, p.settings.PythonBin, args...)
	cmd.Stdin = bytes.NewReader(in)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err = cmd.Run()
	out := strings.TrimSpace(stdout.String())
	if err != nil {
		if out != "" {
			var failed struct {
				Error string `json:"error"`
			}
			if jsonErr := json.Unmarshal([]byte(out), &failed); jsonErr == nil && failed.Error != "" {
				return "", fmt.Errorf("ocr recognize: %s", failed.Error)
			}
		}
		return "", fmt.Errorf("ocr command failed: %w (stderr: %s)", err, strings.TrimSpace(stderr.String()))
	}

	var resp struct {
		Text  string `json:"text"`
		Error string `json:"error,omitempty"`
	}
	if jsonErr := json.Unmarshal([]byte(out), &resp); jsonErr != nil {
		return "", fmt.Errorf("parse ocr response: %w", jsonErr)
	}
	if resp.Error != "" {
		return "", fmt.Errorf("ocr recognize: %s", resp.Error)
	}
	return resp.Text, nil
}

func New(settings Settings) (Client, error) {
	if !settings.Enable {
		return disabledClient{}, nil
	}
	if settings.PythonBin == "" {
		settings.PythonBin = "python3"
	}
	if settings.TimeoutSec <= 0 {
		settings.TimeoutSec = 30
	}
	if strings.TrimSpace(settings.GPUDevice) == "" {
		settings.GPUDevice = "0"
	}
	if settings.DownloadTimeout <= 0 {
		settings.DownloadTimeout = 10 * time.Minute
	}
	if settings.RepoURL == "" {
		return nil, fmt.Errorf("ocr repo url is required")
	}
	if settings.ModelRepoDir == "" {
		return nil, fmt.Errorf("ocr model repo dir is required")
	}
	if settings.BridgeScript == "" {
		return nil, fmt.Errorf("ocr bridge script is required")
	}
	if settings.AutoDownload {
		if err := ensureRepo(settings); err != nil {
			return nil, err
		}
	}
	if _, err := os.Stat(settings.BridgeScript); err != nil {
		return nil, fmt.Errorf("ocr bridge script: %w", err)
	}
	if _, err := os.Stat(settings.ModelRepoDir); err != nil {
		return nil, fmt.Errorf("ocr model repo dir: %w", err)
	}
	return &pythonClient{settings: settings}, nil
}

func ensureRepo(settings Settings) error {
	if _, err := os.Stat(settings.ModelRepoDir); err == nil {
		gitDir := filepath.Join(settings.ModelRepoDir, ".git")
		if _, gitErr := os.Stat(gitDir); gitErr == nil {
			ctx, cancel := context.WithTimeout(context.Background(), settings.DownloadTimeout)
			defer cancel()
			urlCmd := exec.CommandContext(ctx, "git", "-C", settings.ModelRepoDir, "remote", "get-url", "origin")
			out, urlErr := urlCmd.CombinedOutput()
			if urlErr != nil {
				return fmt.Errorf("read ocr repo remote: %w (%s)", urlErr, strings.TrimSpace(string(out)))
			}
			remote := strings.TrimSpace(string(out))
			if !sameRepo(remote, settings.RepoURL) {
				return fmt.Errorf("ocr repo remote mismatch: got %s", remote)
			}

			cmd := exec.CommandContext(ctx, "git", "-C", settings.ModelRepoDir, "pull", "--ff-only")
			if out, runErr := cmd.CombinedOutput(); runErr != nil {
				return fmt.Errorf("update ocr repo: %w (%s)", runErr, strings.TrimSpace(string(out)))
			}
		}
		return nil
	}

	if err := os.MkdirAll(filepath.Dir(settings.ModelRepoDir), 0o755); err != nil {
		return fmt.Errorf("create model parent dir: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), settings.DownloadTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, "git", "clone", "--depth", "1", settings.RepoURL, settings.ModelRepoDir)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("clone ocr repo: %w (%s)", err, strings.TrimSpace(string(out)))
	}
	return nil
}

func sameRepo(a, b string) bool {
	norm := func(v string) string {
		v = strings.TrimSpace(v)
		v = strings.TrimSuffix(v, ".git")
		v = strings.TrimSuffix(v, "/")
		return strings.ToLower(v)
	}
	return norm(a) == norm(b)
}
