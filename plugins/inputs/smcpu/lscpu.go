package smcpu

import (
	"fmt"
	"io/ioutil"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
)

type LscpuInfo struct {
	Arch           string  `json:"arch"`
	CPUs           int     `json:"cpus"`
	ThreadsPerCore int     `json:"threads_per_core"`
	CoresPerSocket int     `json:"cores_per_socket"`
	Sockets        int     `json:"sockets"`
	ModelName      string  `json:"model_name"`
	Mhz            float64 `json:"mhz"`
}

func (c LscpuInfo) String() string {
	v := []string{
		`"arch":"` + c.Arch + `"`,
		`"cpus":` + strconv.FormatInt(int64(c.CPUs), 10),
		`"threads_per_core":` + strconv.FormatInt(int64(c.ThreadsPerCore), 10),
		`"cores_per_socket":` + strconv.FormatInt(int64(c.CoresPerSocket), 10),
		`"sockets":` + strconv.FormatInt(int64(c.Sockets), 10),
		`"model_name":` + c.ModelName,
		`"mhz":` + strconv.FormatFloat(c.Mhz, 'f', 2, 64),
	}

	return `{` + strings.Join(v, ",") + `}`
}

func ReadLscpuInfo() (LscpuInfo, error) {

	var lscpuinfo LscpuInfo
	cmd := exec.Command("lscpu")

	//创建获取命令输出管道
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return lscpuinfo, fmt.Errorf("Error:can not obtain stdout pipe for command:%s\n", err)
	}

	//执行命令
	if err := cmd.Start(); err != nil {
		return lscpuinfo, fmt.Errorf("The command is err:%s\n", err)
	}

	//读取所有输出
	bytes, err := ioutil.ReadAll(stdout)
	if err != nil {
		return lscpuinfo, fmt.Errorf("ReadAll Stdout:%s\n", err)
	}

	lines := strings.Split(string(bytes), "\n")

	for _, line := range lines {
		fields := strings.Split(line, ":")
		if len(fields) < 2 {
			fields = strings.Split(line, "：")
			if len(fields) < 2 {
				continue
			}
		}

		key := strings.TrimSpace(fields[0])
		value := strings.TrimSpace(fields[1])

		switch key {
		case "Architecture", "架构":
			lscpuinfo.Arch = value
		case "CPU(s)", "CPU":
			lscpuinfo.CPUs, _ = strconv.Atoi(value)
		case "Thread(s) per core", "每个核的线程数":
			lscpuinfo.ThreadsPerCore, _ = strconv.Atoi(value)
		case "Core(s) per socket", "每个座的核数":
			lscpuinfo.CoresPerSocket, _ = strconv.Atoi(value)
		case "Socket(s)", "座":
			lscpuinfo.Sockets, _ = strconv.Atoi(value)
		case "Model name", "型号名称":
			lscpuinfo.ModelName = parseModelName(value)
		case "CPU MHz":
			lscpuinfo.Mhz, _ = strconv.ParseFloat(value, 64)
		case "CPU max MHz", "CPU 最大 MHz":
			if lscpuinfo.Mhz == 0 {
				lscpuinfo.Mhz, _ = strconv.ParseFloat(value, 64)
			}
		}
	}

	if err := cmd.Wait(); err != nil {
		return lscpuinfo, fmt.Errorf("wait:%s\n", err)
	}

	return lscpuinfo, nil
}

/**
 * 解析Model Name
 */
func parseModelName(value string) string {
	modelName := value
	index := strings.Index(value, "@")
	if index > 0 {
		modelName = deleteExtraSpace(value[:index])
	}

	return strings.TrimSpace(modelName)
}

/*
 * 函数名：delete_extra_space(s string) string
 * 功  能:删除字符串中多余的空格(含tab)，有多个空格时，仅保留一个空格，同时将字符串中的tab换为空格
 * 参  数:s string:原始字符串
 * 返回值:string:删除多余空格后的字符串
 */
func deleteExtraSpace(s string) string {
	//删除字符串中的多余空格，有多个空格时，仅保留一个空格
	s1 := strings.Replace(s, "	", " ", -1)      //替换tab为空格
	regs := "\\s{2,}"                           //两个及两个以上空格的正则表达式
	reg, _ := regexp.Compile(regs)              //编译正则表达式
	s2 := make([]byte, len(s1))                 //定义字符数组切片
	copy(s2, s1)                                //将字符串复制到切片
	spcIndex := reg.FindStringIndex(string(s2)) //在字符串中搜索
	for len(spcIndex) > 0 {                     //找到适配项
		s2 = append(s2[:spcIndex[0]+1], s2[spcIndex[1]:]...) //删除多余空格
		spcIndex = reg.FindStringIndex(string(s2))           //继续在字符串中搜索
	}
	return string(s2)
}
