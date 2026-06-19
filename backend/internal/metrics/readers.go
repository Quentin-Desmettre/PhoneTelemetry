package metrics

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/shirou/gopsutil/v4/cpu"
	"github.com/shirou/gopsutil/v4/disk"
	"github.com/shirou/gopsutil/v4/mem"
)

// hostSys returns the sysfs root, honouring HOST_SYS so the container can read
// the real device metrics from a mounted host /sys.
func hostSys() string {
	if p := os.Getenv("HOST_SYS"); p != "" {
		return p
	}
	return "/sys"
}

// diskPath is the filesystem whose usage we report (the host root by default).
func diskPath() string {
	if p := os.Getenv("DISK_PATH"); p != "" {
		return p
	}
	return "/"
}

// ReadCPU returns the busy CPU percentage averaged across cores since the last
// call. The first call (interval 0, nil prior) returns the since-boot average.
func ReadCPU() (float64, error) {
	pcts, err := cpu.Percent(0, false)
	if err != nil || len(pcts) == 0 {
		return 0, err
	}
	return pcts[0], nil
}

// PrimeCPU establishes the baseline so the next ReadCPU is a real delta.
func PrimeCPU() { _, _ = cpu.Percent(0, false) }

type MemInfo struct {
	Pct   float64
	Used  uint64
	Total uint64
}

func ReadMem() (MemInfo, error) {
	v, err := mem.VirtualMemory()
	if err != nil {
		return MemInfo{}, err
	}
	return MemInfo{Pct: v.UsedPercent, Used: v.Used, Total: v.Total}, nil
}

type DiskInfo struct {
	Pct   float64
	Used  uint64
	Total uint64
}

func ReadDisk() (DiskInfo, error) {
	u, err := disk.Usage(diskPath())
	if err != nil {
		return DiskInfo{}, err
	}
	return DiskInfo{Pct: u.UsedPercent, Used: u.Used, Total: u.Total}, nil
}

type BatteryInfo struct {
	Present  bool
	Pct      float64
	Charging bool
}

// ReadBattery scans /sys/class/power_supply for a battery and reports its
// charge level and whether it is currently charging. Absent on most desktops.
func ReadBattery() BatteryInfo {
	base := filepath.Join(hostSys(), "class", "power_supply")
	entries, err := os.ReadDir(base)
	if err != nil {
		return BatteryInfo{}
	}
	for _, e := range entries {
		dir := filepath.Join(base, e.Name())
		typ := strings.TrimSpace(readFile(filepath.Join(dir, "type")))
		if !strings.EqualFold(typ, "Battery") {
			continue
		}
		capStr := strings.TrimSpace(readFile(filepath.Join(dir, "capacity")))
		if capStr == "" {
			continue
		}
		pct, err := strconv.ParseFloat(capStr, 64)
		if err != nil {
			continue
		}
		status := strings.TrimSpace(readFile(filepath.Join(dir, "status")))
		charging := strings.EqualFold(status, "Charging") || strings.EqualFold(status, "Full")
		return BatteryInfo{Present: true, Pct: pct, Charging: charging}
	}
	return BatteryInfo{}
}

// ReadTemps returns a map of sensor name -> temperature in Celsius, gathered
// from the thermal zones and hwmon devices exposed by the kernel.
func ReadTemps() map[string]float64 {
	out := map[string]float64{}
	sys := hostSys()

	// /sys/class/thermal/thermal_zone*/temp (+ type for a friendly name).
	zones, _ := filepath.Glob(filepath.Join(sys, "class", "thermal", "thermal_zone*"))
	for _, z := range zones {
		raw := strings.TrimSpace(readFile(filepath.Join(z, "temp")))
		v, err := strconv.ParseFloat(raw, 64)
		if err != nil {
			continue
		}
		name := strings.TrimSpace(readFile(filepath.Join(z, "type")))
		if name == "" {
			name = filepath.Base(z)
		}
		out[uniqueName(out, name)] = milliToC(v)
	}

	// /sys/class/hwmon/hwmon*/temp*_input (named via tempN_label or name).
	hwmons, _ := filepath.Glob(filepath.Join(sys, "class", "hwmon", "hwmon*"))
	for _, hw := range hwmons {
		chip := strings.TrimSpace(readFile(filepath.Join(hw, "name")))
		inputs, _ := filepath.Glob(filepath.Join(hw, "temp*_input"))
		for _, in := range inputs {
			raw := strings.TrimSpace(readFile(in))
			v, err := strconv.ParseFloat(raw, 64)
			if err != nil {
				continue
			}
			label := strings.TrimSpace(readFile(strings.Replace(in, "_input", "_label", 1)))
			name := strings.TrimSpace(strings.Join([]string{chip, label}, " "))
			if name == "" {
				name = filepath.Base(in)
			}
			out[uniqueName(out, name)] = milliToC(v)
		}
	}
	return out
}

// kernel temps are reported in milli-degrees Celsius.
func milliToC(v float64) float64 {
	c := v / 1000.0
	return float64(int64(c*10+0.5)) / 10
}

// uniqueName avoids collisions when several sensors share a label.
func uniqueName(m map[string]float64, name string) string {
	if _, ok := m[name]; !ok {
		return name
	}
	for i := 2; ; i++ {
		cand := name + " " + strconv.Itoa(i)
		if _, ok := m[cand]; !ok {
			return cand
		}
	}
}

func readFile(path string) string {
	b, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return string(b)
}
