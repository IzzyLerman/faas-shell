package fnutils

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"strings"

	"github.com/testcontainers/testcontainers-go"
)

var fnservicename string = "fnserver"

type FS interface {
	Mkdir(path string, perm os.FileMode) error
	Chmod(path string, mode os.FileMode) error
	Exec(cmd []string) (int, string, error)
	UserHomeDir() (string, error)
	Create(path string) (*os.File, error)
}

type LocalFS struct{}

func (l *LocalFS) Exec(cmd []string) (int, string, error) {
	output, err := exec.Command(cmd[0], cmd[1:]...).Output()
	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			log.Fatal(err)
			return -1, "", err
		}
	}
	return exitCode, string(output), nil
}

func (l *LocalFS) Chmod(path string, mode os.FileMode) error {
	return os.Chmod(path, mode)
}

// Create a folder and update its permissions if it doesn't already exist
func (l *LocalFS) Mkdir(path string, perm os.FileMode) error {
	if info, err := os.Stat(path); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			if err := os.MkdirAll(path, perm); err != nil {
				return fmt.Errorf("failed to create directory %q: %w", path, err)
			}

			if err := os.Chmod(path, perm); err != nil {
				return fmt.Errorf("failed to chmod directory %q: %w", path, err)
			}
		} else {
			return fmt.Errorf("error checking directory %q: %w", path, err)
		}
	} else if !info.IsDir() {
		return fmt.Errorf("path %q exists but is not a directory", path)
	}
	return nil
}

func (l *LocalFS) UserHomeDir() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("exec error: %w — %s", err, homeDir)
	}
	return homeDir, err
}

func (l *LocalFS) Create(path string) (*os.File, error) {
	f, err := os.Create(path)
	if err != nil {
		return nil, fmt.Errorf("local create %q failed: %w", path, err)
	}
	return f, nil
}

type ContainerFS struct {
	Container testcontainers.Container
	Context   context.Context
}

func (c *ContainerFS) Exec(cmd []string) (int, string, error) {
	// Attach stdout+stderr by default
	exitCode, reader, err := c.Container.Exec(c.Context, cmd)
	if err != nil {
		return -1, "", err
	}
	buf := new(strings.Builder)
	_, err = io.Copy(buf, reader)

	if err != nil {
		return -1, "", err
	}
	return exitCode, buf.String(), nil
}

func (c *ContainerFS) Mkdir(path string, perm os.FileMode) error {
	cmd := []string{"sh", "-c", fmt.Sprintf("mkdir -p %s && chmod %o %s", path, perm, path)}
	code, out, err := c.Container.Exec(c.Context, cmd)
	if err != nil {
		return fmt.Errorf("exec error: %w — %s", err, out)
	}
	if code != 0 {
		return fmt.Errorf("non-zero exit (%d): %s", code, out)
	}
	return nil
}

func (c *ContainerFS) Chmod(path string, mode os.FileMode) error {
	cmd := []string{"chmod", fmt.Sprintf("%o", mode), path}
	code, out, err := c.Container.Exec(c.Context, cmd)
	if err != nil {
		return fmt.Errorf("exec error: %w — %s", err, out)
	}
	if code != 0 {
		return fmt.Errorf("non-zero exit (%d): %s", code, out)
	}
	return nil
}

func (c *ContainerFS) UserHomeDir() (string, error) {
	cmd := []string{"sh", "-c", "echo", "$HOME"}
	_, reader, err := c.Container.Exec(c.Context, cmd)
	if err != nil {
		return "", fmt.Errorf("Error getting home dir: %w", err)
	}
	buf := new(strings.Builder)
	_, err = io.Copy(buf, reader)

	if err != nil {
		return "", err
	}
	return buf.String(), nil
}

func (c *ContainerFS) Create(path string) (*os.File, error) {
	cmd := []string{"sh", "-c", fmt.Sprintf("touch %s", path)}
	exitCode, out, err := c.Container.Exec(c.Context, cmd)
	if err != nil {
		return nil, fmt.Errorf("container exec error: %w – output: %s", err, out)
	}
	if exitCode != 0 {
		return nil, fmt.Errorf("container touch failed (%d): %s", exitCode, out)
	}

	return nil, nil
}

func GetFnStatus(fs FS, verbose bool) error {
	if verbose {
		fmt.Println("checking the status of the fn daemon...")
	}

	_, output, _ := fs.Exec([]string{"systemctl", "--user", "is-active", fnservicename})

	status := strings.TrimSpace(output)
	if status == "active" {
		if verbose {
			fmt.Println("Fn daemon is active!")
		}
		return nil
	} else {
		if verbose {
			fmt.Printf("Fn daemon is not active: %s\n", status)
		}
		return errors.New("Fn Daemon Inactive")
	}

}

func RegisterFnService(fs FS, verbose bool) error {
	if verbose {
		fmt.Println("Registering the fn daemon...")
	}

	userHomeDir, err := fs.UserHomeDir()
	fs.Mkdir(userHomeDir+"/.config", 0o755)
	fs.Mkdir(userHomeDir+"/.config/systemd", 0o755)
	fs.Mkdir(userHomeDir+"/.config/systemd/user", 0o755)

	file, err := fs.Create(userHomeDir + "/.config/systemd/user/" + fnservicename + ".service")
	if err != nil {
		log.Fatal(err)
	}
	defer file.Close()

	_, err = file.WriteString(`
	[Unit]
	Description=Fn Server Daemon

	[Service]
	ExecStart=/usr/local/bin/fn start 
	Restart=on-failure
	WorkingDirectory=%h/.fn
	Environment=FN_HOME=%h/.fn

	[Install]
	WantedBy=default.target
	`)
	if err != nil && verbose {
		fmt.Println("Failed to write to the fn service config. Exiting...")
		return err
	}
	fs.Exec([]string{"systemctl", "--user", "daemon-reload"})
	fs.Exec([]string{"systemctl", "--user", "enable", fnservicename})
	_, out, err := fs.Exec([]string{"systemctl", "--user", "start", fnservicename})

	if verbose {
		log.Println(out)
	}

	return nil

}
