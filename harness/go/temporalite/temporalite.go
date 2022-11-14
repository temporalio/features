package temporalite

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"go.temporal.io/sdk-features/harness/go/harness"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/log"
)

// DefaultVersion is the default Temporalite version when not provided.
const DefaultVersion = "v0.2.0"

// Temporalite is a running Temporalite instance.
type Temporalite struct {
	// The frontend host:port for use with Temporal SDK client.
	FrontendHostPort string
	cmd              *exec.Cmd
}

// Temporalite start options.
type Options struct {
	// This logger is only used by this process, not Temporalite
	Log log.Logger
	// Defaults to random free port
	GetFrontendPort func() (int, error)
	// Defaults to "default"
	Namespace string
	// Defaults to DefaultVersion
	Version string
	// Defaults to unset
	LogLevel string
	// TODO(cretz): Other Temporalite options?
}

// Start a Temporalite server. This may download the server if not already
// downloaded.
func Start(options Options) (*Temporalite, error) {
	if options.Log == nil {
		options.Log = harness.DefaultLogger
	}
	if options.GetFrontendPort == nil {
		options.GetFrontendPort = func() (port int, err error) {
			prov := newPortProvider()
			defer prov.Close()
			port, err = prov.GetFreePort()
			return
		}
	}
	if options.Namespace == "" {
		options.Namespace = "default"
	}
	if options.Version == "" {
		options.Version = DefaultVersion
	} else if !strings.HasPrefix(options.Version, "v") {
		return nil, fmt.Errorf("version must have 'v' prefix")
	}

	// Download if necessary
	exePath, err := options.loadExePath()
	if err != nil {
		return nil, err
	}

	// Temporalite has no way to give us the port they chose, so we have to find
	// our own free port
	port, err := options.GetFrontendPort()
	if err != nil {
		return nil, fmt.Errorf("failed getting free port: %w", err)
	}
	portStr := strconv.Itoa(port)

	// Start
	args := []string{
		"start",
		"--ephemeral", "--headless", "--namespace", options.Namespace, "--port", portStr,
		"--dynamic-config-value", "system.forceSearchAttributesCacheRefreshOnRead=true",
		"--dynamic-config-value", "system.enableActivityEagerExecution=true",
	}
	if options.LogLevel != "" {
		args = append(args, "--log-level", options.LogLevel)
	}
	cmd := exec.Command(exePath, args...)
	cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
	options.Log.Info("Starting Temporalite", "ExePath", exePath, "Args", args)
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed starting: %w", err)
	}

	// Try every 100ms for 5s to connect
	var clientErr error
	for i := 0; i < 50; i++ {
		client, err := client.Dial(client.Options{HostPort: "127.0.0.1:" + portStr, Namespace: options.Namespace})
		if err != nil {
			clientErr = err
		} else {
			clientErr = nil
			client.Close()
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	if clientErr != nil {
		return nil, fmt.Errorf("failed connecting after 5s, last error: %w", clientErr)
	}
	return &Temporalite{FrontendHostPort: "127.0.0.1:" + portStr, cmd: cmd}, nil
}

// Stop the running Temporalite server and wait for it to stop. This errors if
// Temporalite returned a failed exit code.
func (t *Temporalite) Stop() error {
	if err := t.cmd.Process.Kill(); err != nil {
		return err
	}
	return t.cmd.Wait()
}

func (o *Options) loadExePath() (string, error) {
	// Build path based on version and check if already present
	exePath := filepath.Join(os.TempDir(), "sdk-features-temporalite-"+o.Version)
	if runtime.GOOS == "windows" {
		exePath += ".exe"
	}
	if _, err := os.Stat(exePath); err == nil {
		return exePath, nil
	}

	// Build info URL
	platform := runtime.GOOS
	if platform != "windows" && platform != "darwin" && platform != "linux" {
		return "", fmt.Errorf("unrecognized platform %v", platform)
	}
	arch := runtime.GOARCH
	if arch != "amd64" && arch != "arm64" {
		return "", fmt.Errorf("unrecognized architecture %v", arch)
	}
	infoURL := fmt.Sprintf("https://temporal.download/temporalite/%v?platform=%v&arch=%v", o.Version, platform, arch)

	// Get info
	info := struct {
		ArchiveURL    string `json:"archiveUrl"`
		FileToExtract string `json:"fileToExtract"`
	}{}
	resp, err := http.Get(infoURL)
	if err != nil {
		return "", fmt.Errorf("failed fetching info: %w", err)
	}
	b, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		return "", fmt.Errorf("failed fetching info body: %w", err)
	} else if resp.StatusCode != 200 {
		return "", fmt.Errorf("failed fetching info, status: %v, body: %s", resp.Status, b)
	} else if err = json.Unmarshal(b, &info); err != nil {
		return "", fmt.Errorf("failed unmarshaling info: %w", err)
	}

	// Download and extract
	o.Log.Info("Downloading Temporalite", "Url", info.ArchiveURL, "ExePath", exePath)
	resp, err = http.Get(info.ArchiveURL)
	if err != nil {
		return "", fmt.Errorf("failed downloading: %w", err)
	}
	defer resp.Body.Close()
	// We want to download to a temporary file then rename. A better system-wide
	// atomic downloader would use a common temp file and check whether it exists
	// and wait on it, but doing multiple downloads in racy situations is
	// good/simple enough for now.
	f, err := os.CreateTemp("", "temporalite-downloading-")
	if err != nil {
		return "", fmt.Errorf("failed creating temp file: %w", err)
	}
	if strings.HasSuffix(info.ArchiveURL, ".tar.gz") {
		err = extractTarball(resp.Body, info.FileToExtract, f)
	} else if strings.HasSuffix(info.ArchiveURL, ".zip") {
		err = extractZip(resp.Body, info.FileToExtract, f)
	} else {
		err = fmt.Errorf("unrecognized file extension on %v", info.ArchiveURL)
	}
	f.Close()
	if err != nil {
		return "", err
	}
	// Chmod it if not Windows
	if runtime.GOOS != "windows" {
		if err := os.Chmod(f.Name(), 0777); err != nil {
			return "", fmt.Errorf("failed chmod'ing file: %w", err)
		}
	}
	if err = os.Rename(f.Name(), exePath); err != nil {
		return "", fmt.Errorf("failed moving file: %w", err)
	}
	return exePath, nil
}

func extractTarball(r io.Reader, toExtract string, w io.Writer) error {
	r, err := gzip.NewReader(r)
	if err != nil {
		return err
	}
	tarRead := tar.NewReader(r)
	for {
		h, err := tarRead.Next()
		if err != nil {
			// This can be EOF which means we never found our file
			return err
		} else if h.Name == toExtract {
			_, err = io.Copy(w, tarRead)
			return err
		}
	}
}

func extractZip(r io.Reader, toExtract string, w io.Writer) error {
	// Instead of using a third party zip streamer, and since Go stdlib doesn't
	// support streaming read, we'll just put the entire archive in memory for now
	b, err := io.ReadAll(r)
	if err != nil {
		return err
	}
	zipRead, err := zip.NewReader(bytes.NewReader(b), int64(len(b)))
	if err != nil {
		return err
	}
	for _, file := range zipRead.File {
		if file.Name == toExtract {
			r, err := file.Open()
			if err != nil {
				return err
			}
			_, err = io.Copy(w, r)
			return err
		}
	}
	return fmt.Errorf("could not find file in zip archive")
}
