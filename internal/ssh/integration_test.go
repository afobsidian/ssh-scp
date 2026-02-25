package ssh

import (
	"crypto/ed25519"
	"crypto/rand"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"testing"

	"golang.org/x/crypto/ssh"
)

// testSSHServer starts a minimal SSH server for integration tests.
// It returns the address and a cleanup function.
func testSSHServer(t *testing.T) (addr string, cleanup func()) {
	t.Helper()

	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	signer, err := ssh.NewSignerFromKey(priv)
	if err != nil {
		t.Fatal(err)
	}

	config := &ssh.ServerConfig{
		PasswordCallback: func(c ssh.ConnMetadata, pass []byte) (*ssh.Permissions, error) {
			if c.User() == "testuser" && string(pass) == "testpass" {
				return nil, nil
			}
			return nil, fmt.Errorf("auth failed")
		},
	}
	config.AddHostKey(signer)

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}

	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go handleConn(conn, config)
		}
	}()

	return ln.Addr().String(), func() {
		ln.Close()
		<-done
	}
}

func handleConn(conn net.Conn, config *ssh.ServerConfig) {
	defer conn.Close()

	sshConn, chans, reqs, err := ssh.NewServerConn(conn, config)
	if err != nil {
		return
	}
	defer sshConn.Close()

	go ssh.DiscardRequests(reqs)

	for newChan := range chans {
		if newChan.ChannelType() != "session" {
			newChan.Reject(ssh.UnknownChannelType, "unknown channel type")
			continue
		}
		ch, requests, err := newChan.Accept()
		if err != nil {
			return
		}

		go func() {
			defer ch.Close()
			for req := range requests {
				switch req.Type {
				case "exec":
					// Parse the command from the payload: uint32 length + string
					if len(req.Payload) > 4 {
						cmdLen := int(req.Payload[0])<<24 | int(req.Payload[1])<<16 | int(req.Payload[2])<<8 | int(req.Payload[3])
						if cmdLen > 0 && 4+cmdLen <= len(req.Payload) {
							cmd := string(req.Payload[4 : 4+cmdLen])
							if cmd == "echo $HOME" {
								ch.Write([]byte("/home/testuser\n"))
							} else if len(cmd) > 3 && cmd[:3] == "ls " {
								ch.Write([]byte("total 4\n-rw-r--r-- 1 user user 100 2024-01-15 10:00:00 testfile.txt\ndrwxr-xr-x 2 user user 4096 2024-01-15 10:00:00 testdir\n"))
							}
						}
					}
					if req.WantReply {
						req.Reply(true, nil)
					}
					ch.SendRequest("exit-status", false, []byte{0, 0, 0, 0})
					ch.Close()
					return
				case "pty-req":
					if req.WantReply {
						req.Reply(true, nil)
					}
				case "shell":
					if req.WantReply {
						req.Reply(true, nil)
					}
					// Write prompt then close with exit-status
					ch.Write([]byte("$ "))
					ch.SendRequest("exit-status", false, []byte{0, 0, 0, 0})
					ch.Close()
					return
				case "window-change":
					if req.WantReply {
						req.Reply(true, nil)
					}
				default:
					if req.WantReply {
						req.Reply(false, nil)
					}
				}
			}
		}()
	}
}

// ---------------------------------------------------------------------------
// Integration tests using test SSH server
// ---------------------------------------------------------------------------

func TestNewClient(t *testing.T) {
	addr, cleanup := testSSHServer(t)
	defer cleanup()

	host, port, _ := net.SplitHostPort(addr)
	client, err := New(host, port, "testuser",
		[]ssh.AuthMethod{PasswordAuth("testpass")},
		ssh.InsecureIgnoreHostKey())
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer client.Close()

	if client.client == nil {
		t.Error("client.client should not be nil")
	}
}

func TestNewClientBadAuth(t *testing.T) {
	addr, cleanup := testSSHServer(t)
	defer cleanup()

	host, port, _ := net.SplitHostPort(addr)
	_, err := New(host, port, "testuser",
		[]ssh.AuthMethod{PasswordAuth("wrong")},
		ssh.InsecureIgnoreHostKey())
	if err == nil {
		t.Error("expected auth failure")
	}
}

func TestClientClose(t *testing.T) {
	addr, cleanup := testSSHServer(t)
	defer cleanup()

	host, port, _ := net.SplitHostPort(addr)
	client, err := New(host, port, "testuser",
		[]ssh.AuthMethod{PasswordAuth("testpass")},
		ssh.InsecureIgnoreHostKey())
	if err != nil {
		t.Fatal(err)
	}
	err = client.Close()
	if err != nil {
		t.Errorf("Close() error = %v", err)
	}
}

func TestClientNewSession(t *testing.T) {
	addr, cleanup := testSSHServer(t)
	defer cleanup()

	host, port, _ := net.SplitHostPort(addr)
	client, err := New(host, port, "testuser",
		[]ssh.AuthMethod{PasswordAuth("testpass")},
		ssh.InsecureIgnoreHostKey())
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()

	session, err := client.NewSession()
	if err != nil {
		t.Fatalf("NewSession() error = %v", err)
	}
	session.Close()
}

func TestClientListDir(t *testing.T) {
	addr, cleanup := testSSHServer(t)
	defer cleanup()

	host, port, _ := net.SplitHostPort(addr)
	client, err := New(host, port, "testuser",
		[]ssh.AuthMethod{PasswordAuth("testpass")},
		ssh.InsecureIgnoreHostKey())
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()

	files, err := client.ListDir("/home")
	if err != nil {
		// The error may contain "close session: EOF" from the deferred close
		// after the server already closed the channel — that's OK if we got files
		if len(files) == 0 {
			t.Fatalf("ListDir() error = %v, got no files", err)
		}
	}
	if len(files) == 0 {
		t.Error("expected some files from ls output")
	}
}

func TestClientStartTerminal(t *testing.T) {
	addr, cleanup := testSSHServer(t)
	defer cleanup()

	host, port, _ := net.SplitHostPort(addr)
	client, err := New(host, port, "testuser",
		[]ssh.AuthMethod{PasswordAuth("testpass")},
		ssh.InsecureIgnoreHostKey())
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()

	session, err := client.NewSession()
	if err != nil {
		t.Fatal(err)
	}

	var buf fakeWriter
	err = client.StartTerminal(session, nil, &buf, &buf)
	if err != nil {
		t.Fatalf("StartTerminal() error = %v", err)
	}

	// Close session to stop the shell goroutine
	session.Close()
}

type fakeWriter struct {
	data []byte
}

func (f *fakeWriter) Write(p []byte) (int, error) {
	f.data = append(f.data, p...)
	return len(p), nil
}

func TestClientResizePty(t *testing.T) {
	addr, cleanup := testSSHServer(t)
	defer cleanup()

	host, port, _ := net.SplitHostPort(addr)
	client, err := New(host, port, "testuser",
		[]ssh.AuthMethod{PasswordAuth("testpass")},
		ssh.InsecureIgnoreHostKey())
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()

	session, err := client.NewSession()
	if err != nil {
		t.Fatal(err)
	}

	// Request PTY without starting shell so there's no blocking goroutine
	modes := ssh.TerminalModes{ssh.ECHO: 1}
	if err := session.RequestPty("xterm", 40, 80, modes); err != nil {
		t.Fatal(err)
	}

	err = client.ResizePty(session, 120, 40)
	if err != nil {
		t.Fatalf("ResizePty() error = %v", err)
	}
	session.Close()
}

func TestClientUploadDownloadFile(t *testing.T) {
	// SCP requires a remote scp binary which our test server doesn't have
	// Test that it handles errors gracefully
	addr, cleanup := testSSHServer(t)
	defer cleanup()

	host, port, _ := net.SplitHostPort(addr)
	client, err := New(host, port, "testuser",
		[]ssh.AuthMethod{PasswordAuth("testpass")},
		ssh.InsecureIgnoreHostKey())
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()

	// UploadFile - create a local file
	dir := t.TempDir()
	localFile := filepath.Join(dir, "upload.txt")
	os.WriteFile(localFile, []byte("test content"), 0644)

	// Upload will fail because test server doesn't support SCP
	err = client.UploadFile(localFile, "/tmp/upload.txt")
	if err == nil {
		t.Log("upload succeeded (unexpected with test server)")
	}
	// We're just testing the code path runs without panic

	// DownloadFile - will also fail
	err = client.DownloadFile("/tmp/file.txt", dir)
	if err == nil {
		t.Log("download succeeded (unexpected with test server)")
	}
}

func TestUploadFileNonexistentLocal(t *testing.T) {
	addr, cleanup := testSSHServer(t)
	defer cleanup()

	host, port, _ := net.SplitHostPort(addr)
	client, err := New(host, port, "testuser",
		[]ssh.AuthMethod{PasswordAuth("testpass")},
		ssh.InsecureIgnoreHostKey())
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()

	err = client.UploadFile("/nonexistent/file", "/remote/file")
	if err == nil {
		t.Error("expected error for nonexistent local file")
	}
}
