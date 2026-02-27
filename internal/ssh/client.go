package ssh

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/bramvdbogaerde/go-scp"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
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
	client     *ssh.Client
	config     *ssh.ClientConfig
	address    string
	jumpClient *ssh.Client // non-nil when connected via a jump host
}

// ConnectOptions holds per-connection SSH options parsed from ~/.ssh/config.
type ConnectOptions struct {
	HostKeyAlgorithms     string // comma-separated list, may start with +
	PubkeyAcceptedTypes   string // comma-separated list, may start with +
	StrictHostKeyChecking string // "yes", "no", or "ask"
	UserKnownHostsFile    string // path (e.g. /dev/null)
}

// New creates a new SSH client connected to host:port with the given auth methods.
func New(host, port, username string, authMethods []ssh.AuthMethod, hkCallback ssh.HostKeyCallback, opts *ConnectOptions) (*Client, error) {
	cfg := &ssh.ClientConfig{
		User:            username,
		Auth:            authMethods,
		HostKeyCallback: hkCallback,
		Timeout:         10 * time.Second,
	}

	if opts != nil {
		// Apply HostKeyAlgorithms.
		if algos := parseAlgorithms(opts.HostKeyAlgorithms); len(algos) > 0 {
			log.Printf("[SSH] applying HostKeyAlgorithms: %v", algos)
			cfg.HostKeyAlgorithms = algos
		}

		// StrictHostKeyChecking=no → accept any host key.
		if strings.EqualFold(opts.StrictHostKeyChecking, "no") {
			log.Printf("[SSH] StrictHostKeyChecking=no, accepting all host keys")
			cfg.HostKeyCallback = ssh.InsecureIgnoreHostKey()
		}
	}

	address := net.JoinHostPort(host, port)
	log.Printf("[SSH] dialing %s as %s (%d auth methods)", address, username, len(authMethods))
	client, err := ssh.Dial("tcp", address, cfg)
	if err != nil {
		log.Printf("[SSH] dial failed: %v", err)
		return nil, err
	}
	log.Printf("[SSH] connected to %s", address)
	return &Client{
		client:  client,
		config:  cfg,
		address: address,
	}, nil
}

// NewViaJump creates a new SSH client by tunnelling through an existing jump
// host connection. The jumpSSHClient is the underlying *ssh.Client of the
// bastion/jump host. The returned Client takes ownership of jumpSSHClient and
// closes it when Close is called.
func NewViaJump(jumpSSHClient *ssh.Client, host, port, username string, authMethods []ssh.AuthMethod, hkCallback ssh.HostKeyCallback, opts *ConnectOptions) (*Client, error) {
	cfg := &ssh.ClientConfig{
		User:            username,
		Auth:            authMethods,
		HostKeyCallback: hkCallback,
		Timeout:         10 * time.Second,
	}

	if opts != nil {
		if algos := parseAlgorithms(opts.HostKeyAlgorithms); len(algos) > 0 {
			log.Printf("[SSH] jump-dest applying HostKeyAlgorithms: %v", algos)
			cfg.HostKeyAlgorithms = algos
		}
		if strings.EqualFold(opts.StrictHostKeyChecking, "no") {
			log.Printf("[SSH] jump-dest StrictHostKeyChecking=no, accepting all host keys")
			cfg.HostKeyCallback = ssh.InsecureIgnoreHostKey()
		}
	}

	address := net.JoinHostPort(host, port)
	log.Printf("[SSH] dialing %s through jump host as %s", address, username)

	// Open a TCP connection through the jump host to the destination.
	netConn, err := jumpSSHClient.Dial("tcp", address)
	if err != nil {
		return nil, fmt.Errorf("dial through jump host: %w", err)
	}

	c, chans, reqs, err := ssh.NewClientConn(netConn, address, cfg)
	if err != nil {
		_ = netConn.Close()
		return nil, err
	}
	client := ssh.NewClient(c, chans, reqs)
	log.Printf("[SSH] connected to %s via jump host", address)
	return &Client{
		client:     client,
		config:     cfg,
		address:    address,
		jumpClient: jumpSSHClient,
	}, nil
}

// SSHClient returns the underlying *ssh.Client for use with jump host tunnelling.
func (c *Client) SSHClient() *ssh.Client {
	return c.client
}

// parseAlgorithms splits a comma-separated algorithm list.
// If the value starts with "+", it appends to the default set.
func parseAlgorithms(raw string) []string {
	if raw == "" {
		return nil
	}
	raw = strings.TrimSpace(raw)
	append_ := strings.HasPrefix(raw, "+")
	if append_ {
		raw = raw[1:]
	}
	var algos []string
	for _, a := range strings.Split(raw, ",") {
		a = strings.TrimSpace(a)
		if a != "" {
			algos = append(algos, a)
		}
	}
	if append_ {
		// Prepend defaults (Go's crypto/ssh defaults) so the appended ones are additive.
		defaults := []string{
			"ssh-ed25519",
			"ecdsa-sha2-nistp256",
			"ecdsa-sha2-nistp384",
			"ecdsa-sha2-nistp521",
			"rsa-sha2-256",
			"rsa-sha2-512",
		}
		algos = append(defaults, algos...)
	}
	return algos
}

// PasswordAuth returns an AuthMethod for password authentication.
func PasswordAuth(password string) ssh.AuthMethod {
	return ssh.Password(password)
}

// PasswordCallbackAuth returns an AuthMethod that obtains the password
// on demand by calling the provided function.
func PasswordCallbackAuth(callback func() (string, error)) ssh.AuthMethod {
	return ssh.PasswordCallback(callback)
}

// KeyboardInteractiveAuth returns an AuthMethod that answers
// keyboard-interactive challenges via the given callback.
func KeyboardInteractiveAuth(challenge func(user, instruction string, questions []string, echos []bool) ([]string, error)) ssh.AuthMethod {
	return ssh.KeyboardInteractive(challenge)
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

// AgentAuth returns an AuthMethod that delegates to the running ssh-agent.
// Returns nil, err if SSH_AUTH_SOCK is not set or the agent is unreachable.
func AgentAuth() (ssh.AuthMethod, error) {
	sock := os.Getenv("SSH_AUTH_SOCK")
	if sock == "" {
		return nil, fmt.Errorf("SSH_AUTH_SOCK not set")
	}
	conn, err := net.Dial("unix", sock)
	if err != nil {
		return nil, fmt.Errorf("connect to ssh-agent: %w", err)
	}
	// Note: we intentionally don't close conn here — the agent connection
	// must remain open for the lifetime of the SSH session.
	agentClient := agent.NewClient(conn)
	return ssh.PublicKeysCallback(agentClient.Signers), nil
}

// DefaultKeyPaths returns common private key file paths that exist on disk.
func DefaultKeyPaths() []string {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil
	}
	candidates := []string{
		filepath.Join(home, ".ssh", "id_rsa"),
		filepath.Join(home, ".ssh", "id_ed25519"),
		filepath.Join(home, ".ssh", "id_ecdsa"),
		filepath.Join(home, ".ssh", "id_dsa"),
	}
	var found []string
	for _, p := range candidates {
		if _, err := os.Stat(p); err == nil {
			found = append(found, p)
		}
	}
	return found
}

// Close closes the SSH connection and any underlying jump host connection.
func (c *Client) Close() error {
	err := c.client.Close()
	if c.jumpClient != nil {
		err = errors.Join(err, c.jumpClient.Close())
	}
	return err
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
	log.Printf("[SSH] listing remote dir: %s", path)
	session, err := c.client.NewSession()
	if err != nil {
		return nil, err
	}
	defer func() {
		if cErr := session.Close(); cErr != nil && !errors.Is(cErr, io.EOF) {
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
	log.Printf("[SSH] uploading %s -> %s", localPath, remotePath)
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
	log.Printf("[SSH] downloading %s -> %s", remotePath, localDir)
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

// ReadFile reads the contents of a remote file via cat.
func (c *Client) ReadFile(path string) (string, error) {
	log.Printf("[SSH] reading remote file: %s", path)
	session, err := c.client.NewSession()
	if err != nil {
		return "", err
	}
	defer func() { _ = session.Close() }()

	cmd := fmt.Sprintf("cat %s", shellQuote(path))
	out, err := session.Output(cmd)
	if err != nil {
		return "", fmt.Errorf("read remote file: %w", err)
	}
	return string(out), nil
}

// WriteFile writes content to a remote file.
func (c *Client) WriteFile(path, content string) error {
	log.Printf("[SSH] writing remote file: %s", path)
	session, err := c.client.NewSession()
	if err != nil {
		return err
	}
	defer func() { _ = session.Close() }()

	session.Stdin = strings.NewReader(content)
	cmd := fmt.Sprintf("cat > %s", shellQuote(path))
	return session.Run(cmd)
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

	modTime := parseLSDate(fields)

	return &RemoteFile{
		Name:    name,
		Size:    size,
		Mode:    mode,
		ModTime: modTime,
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

// parseLSDate extracts the modification time from ls -la fields.
// ls -la date format for recent files:  "Jan 15 14:30" (fields[5..7])
// ls -la date format for older files:   "Jan 15  2024" (fields[5..7])
func parseLSDate(fields []string) time.Time {
	if len(fields) < 8 {
		return time.Time{}
	}
	month := fields[5]
	day := fields[6]
	timeOrYear := fields[7]

	now := time.Now()

	if strings.Contains(timeOrYear, ":") {
		// Recent file: "Jan 15 14:30" — year is current year
		dateStr := fmt.Sprintf("%s %s %d %s", month, day, now.Year(), timeOrYear)
		t, err := time.Parse("Jan 2 2006 15:04", dateStr)
		if err != nil {
			return time.Time{}
		}
		// If parsed date is in the future, it's from last year
		if t.After(now) {
			t = t.AddDate(-1, 0, 0)
		}
		return t
	}

	// Older file: "Jan 15 2024" — timeOrYear is the year
	dateStr := fmt.Sprintf("%s %s %s", month, day, timeOrYear)
	t, err := time.Parse("Jan 2 2006", dateStr)
	if err != nil {
		return time.Time{}
	}
	return t
}
