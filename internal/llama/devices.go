package llama

import (
	"bufio"
	"context"
	"fmt"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// Device is a GPU backend device reported by `llama-server --list-devices`.
type Device struct {
	ID      string `json:"id"`   // e.g. "Vulkan0"
	Name    string `json:"name"` // e.g. "NVIDIA GeForce RTX 3050"
	TotalMB int    `json:"totalMB"`
	FreeMB  int    `json:"freeMB"`
}

var deviceLine = regexp.MustCompile(`^\s*(\S+):\s+(.+?)\s+\((\d+) MiB, (\d+) MiB free\)\s*$`)

// ParseDevices parses the output of `llama-server --list-devices`.
func ParseDevices(out string) []Device {
	var devs []Device
	sc := bufio.NewScanner(strings.NewReader(out))
	for sc.Scan() {
		m := deviceLine.FindStringSubmatch(sc.Text())
		if m == nil {
			continue
		}
		total, _ := strconv.Atoi(m[3])
		free, _ := strconv.Atoi(m[4])
		devs = append(devs, Device{ID: m[1], Name: m[2], TotalMB: total, FreeMB: free})
	}
	return devs
}

// ListDevices runs `exe --list-devices` and returns the GPU devices.
func ListDevices(ctx context.Context, exe string) ([]Device, error) {
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, exe, "--list-devices")
	cmd.SysProcAttr = sysProcAttr()
	out, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("llama-server --list-devices: %w", err)
	}
	return ParseDevices(string(out)), nil
}
