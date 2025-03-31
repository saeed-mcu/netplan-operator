/*
Copyright The Kubernetes NMState Authors.


Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package client

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
)

func ExecuteCommand(command string, arguments ...string) (string, error) {
	cmd := exec.Command(command, arguments...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("failed to execute %s: '%s', '%s', '%s'", command, err.Error(), stdout.String(), stderr.String())
	}

	return string(bytes.Trim(stdout.Bytes(), "\n")), nil
}

func RunWithNsenter(command string, args ...string) (string, error) {

	os.Setenv("DBUS_SYSTEM_BUS_ADDRESS", "unix:path=/host/run/dbus/system_bus_socket")

	nsenterArgs := []string{
		"--target", "1", // Target the host's PID 1 (systemd)
		"--mount", "--uts", // Ensure we're using the host's mount and UTS namespaces
		"--ipc", "--net", // Use host IPC and network namespaces
		"--pid", // Run in the host's PID namespace
	}

	nsenterArgs = append(nsenterArgs, command)
	nsenterArgs = append(nsenterArgs, args...)

	cmd := exec.Command("nsenter", nsenterArgs...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("failed to execute %s: '%s', '%s', '%s'", command, err.Error(), stdout.String(), stderr.String())
	}

	return string(bytes.Trim(stdout.Bytes(), "\n")), nil
}
