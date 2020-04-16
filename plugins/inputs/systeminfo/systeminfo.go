// +build linux

package systeminfo

import (
	"io/ioutil"
	"strings"

	"github.com/influxdata/telegraf"
	"github.com/influxdata/telegraf/plugins/inputs"
	"github.com/influxdata/telegraf/plugins/inputs/system"
)

type SysInfoStats struct {
	ps system.PS
}

func (_ *SysInfoStats) Description() string {
	return "Read metrics about /etc/.systeminfo"
}

func (_ *SysInfoStats) SampleConfig() string { return "" }

func (_ *SysInfoStats) Gather(acc telegraf.Accumulator) error {

	//读取文件内容
	bytes, err := ioutil.ReadFile("/etc/.systeminfo")

	if err != nil {
		return fmt.Errorf("error getting system info: %s", err)
	}

	lines := strings.Split(string(bytes), "\n")
	fields := make(map[string]interface{})

	for _, line := range lines {
		props := strings.Split(line, "=")
		if len(props) < 2 {
			props = strings.Split(line, "：")
			if len(props) < 2 {
				continue
			}
		}

		key := strings.TrimSpace(props[0])
		value := strings.TrimSpace(props[1])

		switch key {
		case "产品名称", "ProductName":
			fields["pro_name"] = value
		case "产品型号", "ProductModel":
			fields["pro_number"] = value
		case "标识码（产品唯一标识）", "ID":
			fields["pro_code"] = value
		case "电磁泄漏发射防护类型", "电磁泄露发射防护类型", "ShelterModel":
			fields["launch_type"] = value
		case "生产者（制造商）", "Producter":
			fields["manufacturer"] = value
		case "操作系统名称", "Name":
			fields["os_name"] = value
		case "系统版本", "Release":
			fields["sys_version"] = value
		case "内核版本", "kernel":
			fields["kernel"] = value
		case "系统位数", "Bit":
			fields["sys_number"] = value
		case "I/O保密管理模块":
			fields["io_sec_model"] = value
		case "安全卡版本", "Version":
			fields["safe_number"] = value
		case "固件版本（BIOS）", "固件版本(BIOS)", "固件版本(BIOS）", "固件版本（BIOS)", "BiosVersion":
			fields["bios"] = value
		case "处理器信息", "CPU":
			fields["cpu_info"] = value
		case "内存", "Memory":
			fields["memory"] = value
		case "硬盘序列号", "HDSerial":
			fields["disk_number"] = value
		case "硬盘容量", "HDCapacity":
			fields["disk_capacity"] = value
		case "主板版本号":
			fields["mainboard_version"] = value
		case "系统安装时间":
			fields["sys_begin_time"] = value
		case "系统更新时间", "UpdateTime":
			fields["sys_update_time"] = value
		case "三合一内核版本", "KernelVersion":
			fields["three_kernel"] = value
		case "三合一软件版本", "SoftWareVersion":
			fields["three_version"] = value
		case "硬盘2序列号", "HDSerial_1": //HOME 硬盘序列号
			fields["home_disk_number"] = value
		case "硬盘2容量", "HDCapacity_1": //HOME 硬盘容量
			fields["home_disk_capacity"] = value
		}
	}

	acc.AddGauge("systeminfo", fields, nil)
	return nil
}

func init() {
	ps := system.NewSystemPS()
	inputs.Add("systeminfo", func() telegraf.Input {
		return &SysInfoStats{ps: ps}
	})
}
