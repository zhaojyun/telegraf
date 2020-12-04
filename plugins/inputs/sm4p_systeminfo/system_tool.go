package sm4p_systeminfo

import (
	"bytes"
	"fmt"
	"io"
	"os/exec"
	"strings"
)

var netNum = 0
var isRead = false

func NetInterfaceNum() int {

	//只做一次读取
	if isRead {
		return netNum
	}

	fmt.Println("-------------------sss----------")

	lspci := exec.Command("lspci")
	grep := exec.Command("grep", "Ethernet")

	r, w := io.Pipe() // 创建一个管道

	lspci.Stdout = w // ps向管道的一端写
	grep.Stdin = r   // grep从管道的一端读

	var buffer bytes.Buffer
	grep.Stdout = &buffer // grep的输出为buffer

	err := lspci.Start()
	if err != nil {
		//return netNum,fmt.Errorf("error getting list of interfaces: %s", err)
	}
	_ = grep.Start()
	_ = lspci.Wait()
	_ = w.Close()
	_ = grep.Wait()

	sss := strings.TrimSpace(buffer.String())

	lines := strings.Split(strings.TrimSpace(sss), "\n")
	isRead = true
	netNum = len(lines)
	return netNum
}
