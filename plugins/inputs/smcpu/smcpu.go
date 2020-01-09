package smcpu

import (
	"fmt"
	"time"

	"github.com/influxdata/telegraf"
	"github.com/influxdata/telegraf/plugins/inputs"
	"github.com/influxdata/telegraf/plugins/inputs/system"
	"github.com/shirou/gopsutil/cpu"
)

type SMCPUStats struct {
	ps        system.PS
	lastStats map[string]cpu.TimesStat

	PerCPU   bool `toml:"percpu"`
	TotalCPU bool `toml:"totalcpu"`
}

func (_ *SMCPUStats) Description() string {
	return "Read metrics about cpu usage"
}

var sampleConfig = `
  ## Whether to report per-cpu stats or not
  percpu = true
  ## Whether to report total system cpu stats or not
  totalcpu = true
`

func (_ *SMCPUStats) SampleConfig() string {
	return sampleConfig
}

func (s *SMCPUStats) Gather(acc telegraf.Accumulator) error {

	//执行lscpu，获取cpu核数等信息
	lscpuinfo, err := ReadLscpuInfo()
	if err != nil {
		return fmt.Errorf("excute lscpu error: %s", err)
	}

	times, err := s.ps.CPUTimes(s.PerCPU, s.TotalCPU)
	if err != nil {
		return fmt.Errorf("error getting CPU info: %s", err)
	}
	now := time.Now()

	for _, cts := range times {
		tags := map[string]string{
			"cpu": cts.CPU,
		}

		total := totalCpuTime(cts)
		active := activeCpuTime(cts)

		// Add in percentage
		if len(s.lastStats) == 0 {
			// If it's the 1st gather, can't get CPU Usage stats yet
			continue
		}

		lastCts, ok := s.lastStats[cts.CPU]
		if !ok {
			continue
		}
		lastTotal := totalCpuTime(lastCts)
		lastActive := activeCpuTime(lastCts)
		totalDelta := total - lastTotal

		if totalDelta < 0 {
			err = fmt.Errorf("Error: current total CPU time is less than previous total CPU time")
			break
		}

		if totalDelta == 0 {
			continue
		}

		fieldsG := map[string]interface{}{
			"cpus":             lscpuinfo.CPUs,
			"model_name":       lscpuinfo.ModelName,
			"threads_per_core": lscpuinfo.ThreadsPerCore,
			"cores_per_socket": lscpuinfo.CoresPerSocket,
			"sockets":          lscpuinfo.Sockets,
			"mhz":              lscpuinfo.Mhz,
			"usage_active":     100 * (active - lastActive) / totalDelta,
			"usage_user":       100 * (cts.User - lastCts.User - (cts.Guest - lastCts.Guest)) / totalDelta,
			"usage_system":     100 * (cts.System - lastCts.System) / totalDelta,
			"usage_idle":       100 * (cts.Idle - lastCts.Idle) / totalDelta,
		}
		acc.AddGauge("smcpu", fieldsG, tags, now)
	}

	s.lastStats = make(map[string]cpu.TimesStat)
	for _, cts := range times {
		s.lastStats[cts.CPU] = cts
	}

	return err
}

func totalCpuTime(t cpu.TimesStat) float64 {
	total := t.User + t.System + t.Nice + t.Iowait + t.Irq + t.Softirq + t.Steal +
		t.Idle
	return total
}

func activeCpuTime(t cpu.TimesStat) float64 {
	active := totalCpuTime(t) - t.Idle
	return active
}

func init() {
	inputs.Add("smcpu", func() telegraf.Input {
		return &SMCPUStats{
			PerCPU:   true,
			TotalCPU: true,
			ps:       system.NewSystemPS(),
		}
	})
}
