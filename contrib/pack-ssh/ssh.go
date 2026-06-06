// Package ssh provides SSH client tools for agents.
package ssh

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"go.klarlabs.de/agent/domain/agent"
	"go.klarlabs.de/agent/domain/pack"
	"go.klarlabs.de/agent/domain/tool"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"
)

// ConnectionPool manages SSH connections.
type ConnectionPool struct {
	mu    sync.RWMutex
	conns map[string]*ssh.Client
}

var pool = &ConnectionPool{
	conns: make(map[string]*ssh.Client),
}

// Pack returns the SSH tools pack.
func Pack() *pack.Pack {
	return pack.NewBuilder("ssh").
		WithDescription("SSH client tools").
		AddTools(
			connectTool(),
			disconnectTool(),
			execTool(),
			shellTool(),
			scpUploadTool(),
			scpDownloadTool(),
			tunnelTool(),
			listTool(),
			closeAllTool(),
			keygenTool(),
			parseKeyTool(),
			hostKeyTool(),
		).
		AllowAllInState(agent.StateExplore).
		AllowAllInState(agent.StateAct).
		Build()
}

func connectTool() tool.Tool {
	return tool.NewBuilder("ssh_connect").
		WithDescription("Connect to an SSH server").
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Host           string `json:"host"`
				Port           int    `json:"port,omitempty"`
				User           string `json:"user"`
				Password       string `json:"password,omitempty"`
				KeyFile        string `json:"key_file,omitempty"`
				KeyData        string `json:"key_data,omitempty"`
				Passphrase     string `json:"passphrase,omitempty"`
				ID             string `json:"id,omitempty"`
				Timeout        int    `json:"timeout_ms,omitempty"`
				KnownHostsFile string `json:"known_hosts_file,omitempty"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			port := params.Port
			if port == 0 {
				port = 22
			}

			id := params.ID
			if id == "" {
				id = fmt.Sprintf("%s@%s:%d", params.User, params.Host, port)
			}

			timeout := time.Duration(params.Timeout) * time.Millisecond
			if timeout <= 0 {
				timeout = 30 * time.Second
			}

			var authMethods []ssh.AuthMethod

			// Password auth
			if params.Password != "" {
				authMethods = append(authMethods, ssh.Password(params.Password))
			}

			// Key-based auth
			if params.KeyFile != "" || params.KeyData != "" {
				var keyData []byte
				var err error

				if params.KeyData != "" {
					keyData = []byte(params.KeyData)
				} else {
					keyPath := params.KeyFile
					if strings.HasPrefix(keyPath, "~") {
						home, _ := os.UserHomeDir()
						keyPath = filepath.Join(home, keyPath[1:])
					}
					keyData, err = os.ReadFile(keyPath) // #nosec G304 -- intentional file read for SSH key tool
					if err != nil {
						return tool.Result{}, err
					}
				}

				var signer ssh.Signer
				if params.Passphrase != "" {
					signer, err = ssh.ParsePrivateKeyWithPassphrase(keyData, []byte(params.Passphrase))
				} else {
					signer, err = ssh.ParsePrivateKey(keyData)
				}
				if err != nil {
					return tool.Result{}, err
				}
				authMethods = append(authMethods, ssh.PublicKeys(signer))
			}

			var hostKeyCallback ssh.HostKeyCallback
			if params.KnownHostsFile != "" {
				var khErr error
				hostKeyCallback, khErr = knownhosts.New(params.KnownHostsFile)
				if khErr != nil {
					return tool.Result{}, fmt.Errorf("failed to load known_hosts: %w", khErr)
				}
			} else {
				// #nosec G106 -- InsecureIgnoreHostKey used when no known_hosts_file is provided; callers should supply known_hosts_file in production
				hostKeyCallback = ssh.InsecureIgnoreHostKey()
			}

			config := &ssh.ClientConfig{
				User:            params.User,
				Auth:            authMethods,
				HostKeyCallback: hostKeyCallback,
				Timeout:         timeout,
			}

			addr := fmt.Sprintf("%s:%d", params.Host, port)
			client, err := ssh.Dial("tcp", addr, config)
			if err != nil {
				return tool.Result{}, err
			}

			pool.mu.Lock()
			if existing, ok := pool.conns[id]; ok {
				_ = existing.Close() // #nosec G104 -- best-effort close of existing connection
			}
			pool.conns[id] = client
			pool.mu.Unlock()

			result := map[string]any{
				"id":        id,
				"host":      params.Host,
				"port":      port,
				"user":      params.User,
				"connected": true,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func disconnectTool() tool.Tool {
	return tool.NewBuilder("ssh_disconnect").
		WithDescription("Disconnect from an SSH server").
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				ID string `json:"id"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			pool.mu.Lock()
			client, ok := pool.conns[params.ID]
			if ok {
				_ = client.Close() // #nosec G104 -- best-effort close
				delete(pool.conns, params.ID)
			}
			pool.mu.Unlock()

			result := map[string]any{
				"id":           params.ID,
				"disconnected": ok,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func execTool() tool.Tool {
	return tool.NewBuilder("ssh_exec").
		WithDescription("Execute a command on the SSH server").
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				ID      string `json:"id"`
				Command string `json:"command"`
				Timeout int    `json:"timeout_ms,omitempty"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			pool.mu.RLock()
			client, ok := pool.conns[params.ID]
			pool.mu.RUnlock()

			if !ok {
				result := map[string]any{
					"error": "connection not found",
					"id":    params.ID,
				}
				output, _ := json.Marshal(result)
				return tool.Result{Output: output}, nil
			}

			session, err := client.NewSession()
			if err != nil {
				return tool.Result{}, err
			}
			defer session.Close()

			var stdout, stderr bytes.Buffer
			session.Stdout = &stdout
			session.Stderr = &stderr

			err = session.Run(params.Command)

			exitCode := 0
			if err != nil {
				if exitErr, ok := err.(*ssh.ExitError); ok {
					exitCode = exitErr.ExitStatus()
				} else {
					return tool.Result{}, err
				}
			}

			result := map[string]any{
				"id":        params.ID,
				"command":   params.Command,
				"stdout":    stdout.String(),
				"stderr":    stderr.String(),
				"exit_code": exitCode,
				"success":   exitCode == 0,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func shellTool() tool.Tool {
	return tool.NewBuilder("ssh_shell").
		WithDescription("Start an interactive shell session").
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				ID       string   `json:"id"`
				Commands []string `json:"commands"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			pool.mu.RLock()
			client, ok := pool.conns[params.ID]
			pool.mu.RUnlock()

			if !ok {
				result := map[string]any{
					"error": "connection not found",
					"id":    params.ID,
				}
				output, _ := json.Marshal(result)
				return tool.Result{Output: output}, nil
			}

			// Execute each command in sequence
			var results []map[string]any
			for _, cmd := range params.Commands {
				session, err := client.NewSession()
				if err != nil {
					return tool.Result{}, err
				}

				var stdout, stderr bytes.Buffer
				session.Stdout = &stdout
				session.Stderr = &stderr

				err = session.Run(cmd)
				exitCode := 0
				if err != nil {
					if exitErr, ok := err.(*ssh.ExitError); ok {
						exitCode = exitErr.ExitStatus()
					}
				}

				results = append(results, map[string]any{
					"command":   cmd,
					"stdout":    stdout.String(),
					"stderr":    stderr.String(),
					"exit_code": exitCode,
				})

				_ = session.Close() // #nosec G104 -- best-effort close
			}

			result := map[string]any{
				"id":       params.ID,
				"results":  results,
				"commands": len(params.Commands),
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func scpUploadTool() tool.Tool {
	return tool.NewBuilder("ssh_scp_upload").
		WithDescription("Upload a file via SCP").
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				ID         string `json:"id"`
				LocalPath  string `json:"local_path"`
				RemotePath string `json:"remote_path"`
				Content    string `json:"content,omitempty"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			pool.mu.RLock()
			client, ok := pool.conns[params.ID]
			pool.mu.RUnlock()

			if !ok {
				result := map[string]any{
					"error": "connection not found",
					"id":    params.ID,
				}
				output, _ := json.Marshal(result)
				return tool.Result{Output: output}, nil
			}

			var data []byte
			var err error

			if params.Content != "" {
				data = []byte(params.Content)
			} else {
				data, err = os.ReadFile(params.LocalPath)
				if err != nil {
					return tool.Result{}, err
				}
			}

			session, err := client.NewSession()
			if err != nil {
				return tool.Result{}, err
			}
			defer session.Close()

			// SCP protocol
			go func() {
				w, _ := session.StdinPipe()
				defer func() { _ = w.Close() }() // #nosec G104 -- best-effort close
				fmt.Fprintf(w, "C0644 %d %s\n", len(data), filepath.Base(params.RemotePath))
				_, _ = w.Write(data) // #nosec G104 -- write error handled by session.Run
				fmt.Fprint(w, "\x00")
			}()

			dir := filepath.Dir(params.RemotePath)
			err = session.Run(fmt.Sprintf("scp -t %s", dir))
			if err != nil {
				return tool.Result{}, err
			}

			result := map[string]any{
				"id":          params.ID,
				"remote_path": params.RemotePath,
				"size":        len(data),
				"uploaded":    true,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func scpDownloadTool() tool.Tool {
	return tool.NewBuilder("ssh_scp_download").
		WithDescription("Download a file via SCP").
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				ID         string `json:"id"`
				RemotePath string `json:"remote_path"`
				LocalPath  string `json:"local_path,omitempty"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			pool.mu.RLock()
			client, ok := pool.conns[params.ID]
			pool.mu.RUnlock()

			if !ok {
				result := map[string]any{
					"error": "connection not found",
					"id":    params.ID,
				}
				output, _ := json.Marshal(result)
				return tool.Result{Output: output}, nil
			}

			session, err := client.NewSession()
			if err != nil {
				return tool.Result{}, err
			}
			defer session.Close()

			var stdout bytes.Buffer
			session.Stdout = &stdout

			err = session.Run(fmt.Sprintf("cat %s", params.RemotePath))
			if err != nil {
				return tool.Result{}, err
			}

			content := stdout.Bytes()

			if params.LocalPath != "" {
				err = os.WriteFile(params.LocalPath, content, 0600) // #nosec G306 -- restrictive permissions for downloaded files
				if err != nil {
					return tool.Result{}, err
				}
			}

			result := map[string]any{
				"id":          params.ID,
				"remote_path": params.RemotePath,
				"local_path":  params.LocalPath,
				"size":        len(content),
				"content":     string(content),
				"downloaded":  true,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func tunnelTool() tool.Tool {
	return tool.NewBuilder("ssh_tunnel").
		WithDescription("Create an SSH tunnel (port forwarding)").
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				ID         string `json:"id"`
				LocalPort  int    `json:"local_port"`
				RemoteHost string `json:"remote_host"`
				RemotePort int    `json:"remote_port"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			pool.mu.RLock()
			client, ok := pool.conns[params.ID]
			pool.mu.RUnlock()

			if !ok {
				result := map[string]any{
					"error": "connection not found",
					"id":    params.ID,
				}
				output, _ := json.Marshal(result)
				return tool.Result{Output: output}, nil
			}

			// Start local listener
			localAddr := fmt.Sprintf("localhost:%d", params.LocalPort)
			listener, err := net.Listen("tcp", localAddr)
			if err != nil {
				return tool.Result{}, err
			}

			remoteAddr := fmt.Sprintf("%s:%d", params.RemoteHost, params.RemotePort)

			// Handle connections in background
			go func() {
				for {
					local, err := listener.Accept()
					if err != nil {
						return
					}

					remote, err := client.Dial("tcp", remoteAddr)
					if err != nil {
						_ = local.Close() // #nosec G104 -- best-effort close on dial failure
						continue
					}

					// Bidirectional copy
					go func() {
						defer local.Close()
						defer remote.Close()

						go func() {
							buf := make([]byte, 32*1024)
							for {
								n, err := local.Read(buf)
								if err != nil {
									return
								}
								_, _ = remote.Write(buf[:n]) // #nosec G104 -- best-effort tunnel write
							}
						}()

						buf := make([]byte, 32*1024)
						for {
							n, err := remote.Read(buf)
							if err != nil {
								return
							}
							_, _ = local.Write(buf[:n]) // #nosec G104 -- best-effort tunnel write
						}
					}()
				}
			}()

			result := map[string]any{
				"id":          params.ID,
				"local_port":  params.LocalPort,
				"remote_host": params.RemoteHost,
				"remote_port": params.RemotePort,
				"tunnel":      fmt.Sprintf("localhost:%d -> %s", params.LocalPort, remoteAddr),
				"active":      true,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func listTool() tool.Tool {
	return tool.NewBuilder("ssh_list").
		WithDescription("List active SSH connections").
		ReadOnly().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			pool.mu.RLock()
			var connections []string
			for id := range pool.conns {
				connections = append(connections, id)
			}
			pool.mu.RUnlock()

			result := map[string]any{
				"connections": connections,
				"count":       len(connections),
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func closeAllTool() tool.Tool {
	return tool.NewBuilder("ssh_close_all").
		WithDescription("Close all SSH connections").
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			pool.mu.Lock()
			count := len(pool.conns)
			for id, client := range pool.conns {
				_ = client.Close() // #nosec G104 -- best-effort close
				delete(pool.conns, id)
			}
			pool.mu.Unlock()

			result := map[string]any{
				"closed": count,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func keygenTool() tool.Tool {
	return tool.NewBuilder("ssh_keygen").
		WithDescription("Generate SSH key pair").
		ReadOnly().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Type    string `json:"type,omitempty"` // rsa, ed25519
				Bits    int    `json:"bits,omitempty"` // For RSA
				Comment string `json:"comment,omitempty"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			// Note: This is a placeholder - actual key generation would require
			// more complex implementation
			result := map[string]any{
				"type":    params.Type,
				"bits":    params.Bits,
				"comment": params.Comment,
				"note":    "Key generation requires crypto operations - use ssh-keygen command",
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func parseKeyTool() tool.Tool {
	return tool.NewBuilder("ssh_parse_key").
		WithDescription("Parse SSH public key").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Key     string `json:"key"`
				KeyFile string `json:"key_file,omitempty"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			keyData := params.Key
			if params.KeyFile != "" {
				data, err := os.ReadFile(params.KeyFile)
				if err != nil {
					return tool.Result{}, err
				}
				keyData = string(data)
			}

			pubKey, comment, _, _, err := ssh.ParseAuthorizedKey([]byte(keyData))
			if err != nil {
				return tool.Result{}, err
			}

			result := map[string]any{
				"type":        pubKey.Type(),
				"comment":     comment,
				"fingerprint": ssh.FingerprintSHA256(pubKey),
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func hostKeyTool() tool.Tool {
	return tool.NewBuilder("ssh_host_key").
		WithDescription("Get SSH server host key").
		ReadOnly().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Host    string `json:"host"`
				Port    int    `json:"port,omitempty"`
				Timeout int    `json:"timeout_ms,omitempty"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			port := params.Port
			if port == 0 {
				port = 22
			}

			timeout := time.Duration(params.Timeout) * time.Millisecond
			if timeout <= 0 {
				timeout = 10 * time.Second
			}

			var hostKey ssh.PublicKey
			config := &ssh.ClientConfig{
				// #nosec G106 -- intentional to capture host key before verification
				HostKeyCallback: func(hostname string, remote net.Addr, key ssh.PublicKey) error {
					hostKey = key
					return nil
				},
				Timeout: timeout,
			}

			addr := fmt.Sprintf("%s:%d", params.Host, port)
			conn, err := ssh.Dial("tcp", addr, config)
			if conn != nil {
				_ = conn.Close() // #nosec G104 -- best-effort close
			}

			// We expect an error (no auth), but we captured the host key
			if hostKey == nil && err != nil {
				// Try to extract host key from error
				result := map[string]any{
					"host":  params.Host,
					"port":  port,
					"error": "Could not retrieve host key",
				}
				output, _ := json.Marshal(result)
				return tool.Result{Output: output}, nil
			}

			result := map[string]any{
				"host":        params.Host,
				"port":        port,
				"type":        hostKey.Type(),
				"fingerprint": ssh.FingerprintSHA256(hostKey),
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}
