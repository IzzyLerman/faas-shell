package main

import (
	"context"
	"funky/fnutils"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

func TestUbuntuContainerCommand(t *testing.T) {
	ctx := context.Background()

	req := testcontainers.ContainerRequest{
		Image:        "ubuntu-systemd",
		Privileged:   true,
		ExposedPorts: []string{"80/tcp"},
		Cmd: []string{
			"/bin/bash",
			"-c",
			"systemctl mask systemd-logind.service getty.service getty.target && /sbin/init",
			"echo 'Container Ready'",
			"sleep 3600",
		},
		Env: map[string]string{
			"container": "docker",
		},

		Mounts: testcontainers.Mounts(
			testcontainers.BindMount("/sys/fs/cgroup", "/sys/fs/cgroup"),
			testcontainers.VolumeMount("run", "/run"),
			testcontainers.VolumeMount("run-lock", "/run/lock"),
			testcontainers.VolumeMount("tmp", "/tmp"),
		),
		WaitingFor: wait.ForListeningPort("80/tcp").WithStartupTimeout(60 * time.Second),
	}

	ctr, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	require.NoError(t, err)

	t.Cleanup(func() {
		if err := ctr.Terminate(ctx); err != nil {
			t.Logf("failed to terminate container: %s", err)
		}
	})

	fs := &fnutils.ContainerFS{Context: ctx, Container: ctr}

	fnutils.RegisterFnService(fs, true)

	err = fnutils.GetFnStatus(fs, true)
	require.NoError(t, err)
}
