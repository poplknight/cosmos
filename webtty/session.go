package webtty

import (
	"crypto/ecdh"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/creack/pty"
	"golang.org/x/crypto/ssh"
)

type session interface {
	Read([]byte) (int, error)
	Write([]byte) (int, error)
	Resize(cols, rows int) error
	Close() error
}

func parseConnect(priv *ecdh.PrivateKey, data []byte) (Connect, error) {
	var raw wireConnect
	if err := json.Unmarshal(data, &raw); err != nil {
		return Connect{}, errors.New("连接信息无效")
	}
	if raw.Type != "connect" {
		return Connect{}, errors.New("请先发送连接信息")
	}
	if raw.Username != "" || raw.Password != "" {
		return Connect{}, errors.New("凭据不能明文传输")
	}
	req := Connect{Mode: raw.Mode, Host: raw.Host, Cols: raw.Cols, Rows: raw.Rows}
	if raw.Mode == "ssh" || raw.Key != "" || raw.Nonce != "" || raw.Secret != "" {
		username, password, err := decryptLogin(priv, raw.Key, raw.Nonce, raw.Secret)
		if err != nil {
			return Connect{}, err
		}
		req.Username = username
		req.Password = password
	}
	if req.Cols < 2 || req.Cols > 1000 {
		req.Cols = 80
	}
	if req.Rows < 2 || req.Rows > 1000 {
		req.Rows = 24
	}
	switch req.Mode {
	case "local":
		return req, nil
	case "ssh":
		req.Host = strings.TrimSpace(req.Host)
		req.Username = strings.TrimSpace(req.Username)
		if req.Host == "" || req.Username == "" {
			return Connect{}, errors.New("请填写主机地址和用户名")
		}
		if len(req.Host) > 255 || len(req.Username) > 128 || len(req.Password) > 1024 {
			return Connect{}, errors.New("参数过长")
		}
		if strings.ContainsAny(req.Host, " \t\r\n") || strings.ContainsAny(req.Username, "\r\n") {
			return Connect{}, errors.New("主机地址或用户名无效")
		}
		addr, err := sshAddress(req.Host)
		if err != nil {
			return Connect{}, err
		}
		req.Host = addr
		return req, nil
	default:
		return Connect{}, errors.New("未知连接方式")
	}
}

func sshAddress(host string) (string, error) {
	if h, port, err := net.SplitHostPort(host); err == nil && h != "" && port != "" {
		return host, nil
	}
	host = strings.Trim(host, "[]")
	if host == "" {
		return "", errors.New("主机地址无效")
	}
	return net.JoinHostPort(host, "22"), nil
}

func openSSH(req Connect) (session, error) {
	config := &ssh.ClientConfig{
		User: req.Username,
		Auth: []ssh.AuthMethod{
			ssh.Password(req.Password),
			ssh.KeyboardInteractive(func(_, _ string, questions []string, _ []bool) ([]string, error) {
				answers := make([]string, len(questions))
				for i := range questions {
					answers[i] = req.Password
				}
				return answers, nil
			}),
		},
		HostKeyCallback: acceptHostKey,
		Timeout:         12 * time.Second,
	}
	client, err := ssh.Dial("tcp", req.Host, config)
	if err != nil {
		return nil, friendlySSHError(err)
	}
	shell, err := client.NewSession()
	if err != nil {
		_ = client.Close()
		return nil, errors.New("无法打开会话")
	}
	stdin, err := shell.StdinPipe()
	if err != nil {
		_ = shell.Close()
		_ = client.Close()
		return nil, errors.New("无法打开会话")
	}
	stdout, err := shell.StdoutPipe()
	if err != nil {
		_ = shell.Close()
		_ = client.Close()
		return nil, errors.New("无法打开会话")
	}
	modes := ssh.TerminalModes{
		ssh.ECHO:          1,
		ssh.TTY_OP_ISPEED: 14400,
		ssh.TTY_OP_OSPEED: 14400,
	}
	if err = shell.RequestPty("xterm-256color", req.Rows, req.Cols, modes); err != nil {
		_ = shell.Close()
		_ = client.Close()
		return nil, errors.New("无法分配终端")
	}
	if err = shell.Shell(); err != nil {
		_ = shell.Close()
		_ = client.Close()
		return nil, errors.New("无法启动 shell")
	}
	return &sshTerm{client: client, session: shell, stdin: stdin, stdout: stdout}, nil
}

func acceptHostKey(string, net.Addr, ssh.PublicKey) error { return nil }

func friendlySSHError(err error) error {
	msg := strings.ToLower(err.Error())
	switch {
	case strings.Contains(msg, "unable to authenticate"),
		strings.Contains(msg, "permission denied"),
		strings.Contains(msg, "no supported methods"):
		return errors.New("认证失败")
	case strings.Contains(msg, "timeout"):
		return errors.New("连接超时")
	case strings.Contains(msg, "connection refused"):
		return errors.New("连接被拒绝")
	default:
		return fmt.Errorf("无法连接")
	}
}

type sshTerm struct {
	client  *ssh.Client
	session *ssh.Session
	stdin   io.WriteCloser
	stdout  io.Reader
}

func (t *sshTerm) Read(p []byte) (int, error)  { return t.stdout.Read(p) }
func (t *sshTerm) Write(p []byte) (int, error) { return t.stdin.Write(p) }

func (t *sshTerm) Resize(cols, rows int) error {
	return t.session.WindowChange(rows, cols)
}

func (t *sshTerm) Close() error {
	_ = t.stdin.Close()
	err := t.session.Close()
	_ = t.client.Close()
	return err
}

type localTerm struct {
	file *os.File
	cmd  *exec.Cmd
}

func openLocal(cols, rows int) (session, error) {
	cmd := newShell()
	ptmx, err := pty.StartWithSize(cmd, &pty.Winsize{Cols: uint16(cols), Rows: uint16(rows)})
	if err != nil {
		return nil, errors.New("无法打开本机终端")
	}
	return &localTerm{file: ptmx, cmd: cmd}, nil
}

func (t *localTerm) Read(p []byte) (int, error)  { return t.file.Read(p) }
func (t *localTerm) Write(p []byte) (int, error) { return t.file.Write(p) }

func (t *localTerm) Resize(cols, rows int) error {
	return pty.Setsize(t.file, &pty.Winsize{Cols: uint16(cols), Rows: uint16(rows)})
}

func (t *localTerm) Close() error {
	err := t.file.Close()
	if t.cmd.Process != nil {
		_ = t.cmd.Process.Kill()
	}
	_ = t.cmd.Wait()
	return err
}

func newShell() *exec.Cmd {
	shell := os.Getenv("SHELL")
	if shell == "" {
		shell = "/bin/zsh"
	}
	cmd := exec.Command(shell)
	if home, err := os.UserHomeDir(); err == nil {
		cmd.Dir = home
	}
	cmd.Env = shellEnv()
	return cmd
}

func shellEnv() []string {
	env := os.Environ()
	overrides := map[string]string{
		"TERM":      "xterm-256color",
		"COLORTERM": "truecolor",
	}
	hasLocale := false
	for i, entry := range env {
		key, _, ok := strings.Cut(entry, "=")
		if !ok {
			continue
		}
		if key == "LANG" || key == "LC_ALL" || key == "LC_CTYPE" {
			hasLocale = true
		}
		if value, ok := overrides[key]; ok {
			env[i] = key + "=" + value
			delete(overrides, key)
		}
	}
	for key, value := range overrides {
		env = append(env, key+"="+value)
	}
	if !hasLocale {
		env = append(env, "LANG=en_US.UTF-8")
	}
	return env
}
