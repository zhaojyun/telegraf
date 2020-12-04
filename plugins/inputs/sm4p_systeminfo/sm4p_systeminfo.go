package sm4p_systeminfo

import (
	"fmt"
	"github.com/influxdata/telegraf"
	"github.com/influxdata/telegraf/plugins/inputs"
	"github.com/influxdata/telegraf/plugins/inputs/smcpu"
	"io/ioutil"
	"strings"
	"time"
)

type Sm4pSysInfoStats struct {
	ResourceType  string `toml:resource_type`
	ResourceIndex string `json:"resource_index"`
}

func (_ *Sm4pSysInfoStats) Description() string {
	return "Read metrics about /etc/.systeminfo"
}

var sampleConfig = `
  ##资源类型：0-SM服务器，6-SM桌终端
  resource_type = 0
   
  ##资源索引：静态信息 index=1 ; 动态信息 index=2; CPU信息 index=3
  resource_index = 1
`

func (_ *Sm4pSysInfoStats) SampleConfig() string { return sampleConfig }

func (s *Sm4pSysInfoStats) Gather(acc telegraf.Accumulator) error {

	//读取文件内容
	bytes, err := ioutil.ReadFile("/etc/.systeminfo") //"D:/sys.systeminfo")

	if err != nil {
		return fmt.Errorf("error getting system info: %s", err)
	}

	lines := strings.Split(string(bytes), "\n")
	fields := make(map[string]interface{})

	tags := map[string]string{
		"type":  s.ResourceType,
		"index": s.ResourceIndex,
	}

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
			fields["productName"] = value
		case "产品型号", "ProductModel":
			fields["productModel"] = value
		case "标识码（产品唯一标识）", "ID":
			fields["uniqueIdent"] = value
		case "电磁泄漏发射防护类型", "电磁泄露发射防护类型", "ShelterModel":
			fields["launch_type"] = value
		case "生产者（制造商）", "Producter":
			fields["manufacturer"] = value
		case "操作系统名称", "Name":
			fields["sysName"] = value
		case "系统版本", "Release":
			fields["sysVersion"] = value
		case "内核版本", "kernel":
			fields["coreVersion"] = value
		case "系统位数", "Bit":
			fields["sysBits"] = value
		case "I/O保密管理模块":
			fields["io_sec_model"] = value
		case "安全卡版本", "Version":
			fields["socVersion"] = value
		case "固件版本（BIOS）", "固件版本(BIOS)", "固件版本(BIOS）", "固件版本（BIOS)", "BiosVersion":
			fields["biosVersion"] = value
		case "处理器信息", "CPU":
			fields["cpuInfo"] = value
		case "内存", "Memory":
			fields["memory"] = value
		case "硬盘序列号", "HDSerial":
			fields["diskSn"] = value
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
			fields["ioVersion"] = value
		case "硬盘2序列号", "HDSerial_1": //HOME 硬盘序列号
			fields["home_disk_number"] = value
		case "硬盘2容量", "HDCapacity_1": //HOME 硬盘容量
			fields["home_disk_capacity"] = value
		}
	}

	//执行lscpu，获取cpu核数等信息
	lscpuinfo, err := smcpu.ReadLscpuInfo()
	if err != nil {
		return fmt.Errorf("excute lscpu error: %s", err)
	}

	fields["sysArch"] = lscpuinfo.Arch
	fields["cpuNum"] = lscpuinfo.Sockets

	fields["netNum"] = NetInterfaceNum()

	var indexes []map[string]interface{}
	indexes = append(indexes, fields)
	if indexes != nil && len(indexes) != 0 {
		fieldsG := map[string]interface{}{
			"value": indexes,
		}
		acc.AddGauge("sm4p_systeminfo", fieldsG, tags, time.Now())
	}

	return nil
}

func init() {
	inputs.Add("sm4p_systeminfo", func() telegraf.Input {
		return &Sm4pSysInfoStats{
			ResourceType:  "0",
			ResourceIndex: "1",
		}
	})
}
