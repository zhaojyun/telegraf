package smcpu

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
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

type SMCpuInfo struct {
	ModelName string
	Cores     int32
	Mhz       float64
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

	//获取逻辑cpu信息
	cpuInfos, err := cpu.Info()
	if err != nil {
		fmt.Println(err)
	}

	cpuInfosByName := map[string]cpu.InfoStat{}
	for _, cpuInfo := range cpuInfos {
		cpuInfosByName["cpu"+strconv.FormatInt(int64(cpuInfo.CPU), 10)] = cpuInfo
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

		var smcpuinfo SMCpuInfo
		cinfo, ok := cpuInfosByName[cts.CPU]

		if ok {
			index := strings.Index(cinfo.ModelName, "@")
			modleName := cinfo.ModelName
			if index > 0 {
				modleName = deleteExtraSpace(cinfo.ModelName[:index])
			}
			smcpuinfo.ModelName = modleName
			smcpuinfo.Cores = cinfo.Cores
			smcpuinfo.Mhz = cinfo.Mhz
		}

		fieldsG := map[string]interface{}{
			"model_name":   smcpuinfo.ModelName,
			"cores":        smcpuinfo.Cores,
			"mhz":          smcpuinfo.Mhz,
			"usage_active": 100 * (active - lastActive) / totalDelta,
			"usage_user":   100 * (cts.User - lastCts.User - (cts.Guest - lastCts.Guest)) / totalDelta,
			"usage_system": 100 * (cts.System - lastCts.System) / totalDelta,
			"usage_idle":   100 * (cts.Idle - lastCts.Idle) / totalDelta,
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

/*
 * 函数名：delete_extra_space(s string) string
 * 功  能:删除字符串中多余的空格(含tab)，有多个空格时，仅保留一个空格，同时将字符串中的tab换为空格
 * 参  数:s string:原始字符串
 * 返回值:string:删除多余空格后的字符串
 */
func deleteExtraSpace(s string) string {
	//删除字符串中的多余空格，有多个空格时，仅保留一个空格
	s1 := strings.Replace(s, "	", " ", -1)       //替换tab为空格
	regstr := "\\s{2,}"                          //两个及两个以上空格的正则表达式
	reg, _ := regexp.Compile(regstr)             //编译正则表达式
	s2 := make([]byte, len(s1))                  //定义字符数组切片
	copy(s2, s1)                                 //将字符串复制到切片
	spc_index := reg.FindStringIndex(string(s2)) //在字符串中搜索
	for len(spc_index) > 0 {                     //找到适配项
		s2 = append(s2[:spc_index[0]+1], s2[spc_index[1]:]...) //删除多余空格
		spc_index = reg.FindStringIndex(string(s2))            //继续在字符串中搜索
	}
	return string(s2)
}
