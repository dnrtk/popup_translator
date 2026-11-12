package llama

import (
	"reflect"
	"testing"
)

func TestParseDevices(t *testing.T) {
	out := "load_backend: loaded Vulkan backend\r\nAvailable devices:\r\n" +
		"  Vulkan0: NVIDIA GeForce RTX 3050 (8256 MiB, 6909 MiB free)\r\n" +
		"  Vulkan1: AMD Radeon(TM) Graphics (2048 MiB, 1900 MiB free)\n"
	want := []Device{
		{ID: "Vulkan0", Name: "NVIDIA GeForce RTX 3050", TotalMB: 8256, FreeMB: 6909},
		{ID: "Vulkan1", Name: "AMD Radeon(TM) Graphics", TotalMB: 2048, FreeMB: 1900},
	}
	if got := ParseDevices(out); !reflect.DeepEqual(got, want) {
		t.Errorf("got %+v", got)
	}
	if got := ParseDevices("Available devices:\n"); len(got) != 0 {
		t.Errorf("no devices: got %+v", got)
	}
}
