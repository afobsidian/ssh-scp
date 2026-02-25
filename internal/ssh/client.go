package ssh

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/bramvdbogaerde/go-scp"
	"golang.org/x/crypto/ssh"
)

// RemoteFile represents a file entry on the remote filesystem.
type RemoteFile struct {
	Name    string
	Size    int64
	Mode    os.FileMode
	ModTime time.Time
	IsDir   bool
}

// Client wraps an SSH connection.
type Client struct {
	client  *ssh.Client
	config  *ssh.ClientConfig
	address string
}

// New creates a new SSH client connected to host:port with the given auth methods.
func New(host, port, username string, authMethods []ssh.AuthMethod, hkCallback ssh.HostKeyCallback) (*Client, error) {
	cfg := &ssh.ClientConfig{
		User:            username,
		Auth:            authMethods,
		HostKeyCallback: hkCallback,
		Timeout:         10 * time.Second,
	}
	address := net.JoinHostPort(host, port)
	client, err := ssh.Dial("tcp", address, cfg)
	if err != nil {
		return nil, err
	}
	return &Client{
		client:  client,
		config:  cfg,
		address: address,
	}, nil
}

// PasswordAuth returns an AuthMethod for password authentication.
func PasswordAuth(password string) ssh.AuthMethod {
	return ssh.Password(password)
}

// PubKeyAuth returns an AuthMethod for public key authentication from a key file.
func PubKeyAuth(keyPath string) (ssh.AuthMethod, error) {
	key, err := os.ReadFile(keyPath)
	if err != nil {
		return nil, err
	}
	signer, err := ssh.ParsePrivateKey(key)
	if err != nil {
		return nil, err
	}
	return ssh.PublicKeys(signer), nil
}

// Close closes the SSH connection.
func (c *Client) Close() error {
	return c.client.Close()
}

// NewSession creates a new SSH session.
func (c *Client) NewSession() (*ssh.Session, error) {
	return c.client.NewSession()
}

// StartTerminal starts an interactive PTY session over the given session,
// wiring stdin/stdout/stderr to the provided reader/writers.
func (c *Client) StartTerminal(session *ssh.Session, stdin io.Reader, stdout, stderr io.Writer) error {
	session.Stdin = stdin
	session.Stdout = stdout
	session.Stderr = stderr

	modes := ssh.TerminalModes{
		ssh.ECHO:          1,
		ssh.TTY_OP_ISPEED: 14400,
		ssh.TTY_OP_OSPEED: 14400,
	}
	if err := session.RequestPty("xterm-256color", 40, 80, modes); err != nil {
		return fmt.Errorf("request pty: %w", err)
	}
	if err := session.Shell(); err != nil {
		return fmt.Errorf("start shell: %w", err)
	}
	return nil
}

// ResizePty resizes the PTY for the given session.
func (c *Client) ResizePty(session *ssh.Session, width, height int) error {
	return session.WindowChange(height, width)
}

// ListDir lists the contents of a remote directory.
func (c *Client) ListDir(path string) (files []RemoteFile, retErr error) {
	session, err := c.client.NewSession()
	if err != nil {
		return nil, err
	}
	defer func() {
		if cErr := session.Close(); cErr != nil {
			retErr = errors.Join(retErr, fmt.Errorf("close session: %w", cErr))
		}
	}()

	// Use printf %q to safely single-quote the path, preventing shell injection.
	escapedPath := shellQuote(path)
	cmd := fmt.Sprintf("ls -la --time-style='+%%Y-%%m-%%d %%H:%%M:%%S' %s 2>/dev/null || ls -la %s", escapedPath, escapedPath)
	out, err := session.Output(cmd)
	if err != nil {
		return nil, err
	}
	return parseLS(string(out)), nil
}

// UploadFile uploads a local file to the remote destination path.
func (c *Client) UploadFile(localPath, remotePath string) (retErr error) {
	scpClient, err := scp.NewClientBySSH(c.client)
	if err != nil {
		return err
	}
	defer scpClient.Close()

	f, err := os.Open(localPath)
	if err != nil {
		return err
	}
	defer func() {
		if cErr := f.Close(); cErr != nil {
			retErr = errors.Join(retErr, fmt.Errorf("close local file: %w", cErr))
		}
	}()

	info, err := f.Stat()
	if err != nil {
		return err
	}

	return scpClient.CopyFile(context.Background(), f, remotePath, fmt.Sprintf("0%o", info.Mode()))
}

// DownloadFile downloads a remote file to a local destination path.
func (c *Client) DownloadFile(remotePath, localDir string) (retErr error) {
	scpClient, err := scp.NewClientBySSH(c.client)
	if err != nil {
		return err
	}
	defer scpClient.Close()

	fileName := filepath.Base(remotePath)
	localPath := filepath.Join(localDir, fileName)

	f, err := os.Create(localPath)
	if err != nil {
		return err
	}
	defer func() {
		if cErr := f.Close(); cErr != nil {
			retErr = errors.Join(retErr, fmt.Errorf("close local file: %w", cErr))
		}
	}()

	return scpClient.CopyFromRemote(context.Background(), f, remotePath)
}

// parseLS parses `ls -la` output into RemoteFile entries.
func parseLS(output string) []RemoteFile {
	var files []RemoteFile
	lines := splitLines(output)
	for _, line := range lines {
		if line == "" || len(line) >= 5 && line[:5] == "total" {
			continue
		}
		f := parseLSLine(line)
		if f == nil {
			continue
		}
		if f.Name == "." || f.Name == ".." {
			continue
		}
		files = append(files, *f)
	}
	return files
}

// shellQuote wraps a path in single quotes and escapes any single quotes within it,
// preventing shell injection when the path is used in a remote command.
func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'"
}

func splitLines(s string) []string {
	var lines []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			lines = append(lines, s[start:i])
			start = i + 1
		}
	}
	if start < len(s) {
		lines = append(lines, s[start:])
	}
	return lines
}

func parseLSLine(line string) *RemoteFile {
	fields := splitFields(line)
	if len(fields) < 5 {
		return nil
	}
	perm := fields[0]
	isDir := len(perm) > 0 && perm[0] == 'd'

	name := ""
	if len(fields) >= 9 {
		name = fields[8]
	} else if len(fields) >= 5 {
		name = fields[len(fields)-1]
	}

	if name == "" {
		return nil
	}

	var size int64
	_, _ = fmt.Sscanf(fields[4], "%d", &size)

	mode := parsePerm(perm)

	return &RemoteFile{
		Name:    name,
		Size:    size,
		Mode:    mode,
		ModTime: time.Now(),
		IsDir:   isDir,
	}
}

func splitFields(s string) []string {
	var fields []string
	inField := false
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == ' ' || s[i] == '\t' {
			if inField {
				fields = append(fields, s[start:i])
				inField = false
			}
		} else {
			if !inField {
				start = i
				inField = true
			}
		}
	}
	if inField {
		fields = append(fields, s[start:])
	}
	return fields
}

func parsePerm(perm string) os.FileMode {
	if len(perm) < 10 {
		return 0
	}
	var mode os.FileMode
	if perm[1] == 'r' {
		mode |= 0400
	}
	if perm[2] == 'w' {
		mode |= 0200
	}
	if perm[3] == 'x' {
		mode |= 0100
	}
	if perm[4] == 'r' {
		mode |= 0040
	}
	if perm[5] == 'w' {
		mode |= 0020
	}
	if perm[6] == 'x' {
		mode |= 0010
	}
	if perm[7] == 'r' {
		mode |= 0004
	}
	if perm[8] == 'w' {
		mode |= 0002
	}
	if perm[9] == 'x' {
		mode |= 0001
	}
	return mode
}
