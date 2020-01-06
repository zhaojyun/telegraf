package smprocstat

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"path/filepath"
	"strconv"
	"time"

	"github.com/influxdata/telegraf"
	"github.com/influxdata/telegraf/plugins/inputs"
)

var (
	defaultPIDFinder = NewPgrep
	defaultProcess   = NewProc
)

type PID int32

type SMProcstat struct {
	PidFinder   string `toml:"pid_finder"`
	PidFile     string `toml:"pid_file"`
	Exe         string
	Pattern     string
	Prefix      string
	CmdLineTag  bool `toml:"cmdline_tag"`
	ProcessName string
	User        string
	CGroup      string `toml:"cgroup"`
	PidTag      bool

	finder PIDFinder

	createPIDFinder func() (PIDFinder, error)
	procs           map[PID]Process
	createProcess   func(PID) (Process, error)
}

var sampleConfig = `
  ## PID file to monitor process
  pid_file = "/var/run/nginx.pid"
  ## executable name (ie, pgrep <exe>)
  # exe = "nginx"
  ## pattern as argument for pgrep (ie, pgrep -f <pattern>)
  # pattern = "nginx"
  ## user as argument for pgrep (ie, pgrep -u <user>)
  # user = "nginx"
  ## Systemd unit name
  # systemd_unit = "nginx.service"
  ## CGroup name or path
  # cgroup = "systemd/system.slice/nginx.service"

  ## override for process_name
  ## This is optional; default is sourced from /proc/<pid>/status
  # process_name = "bar"

  ## Field name prefix
  # prefix = ""

  ## When true add the full cmdline as a tag.
  # cmdline_tag = false

  ## Add PID as a tag instead of a field; useful to differentiate between
  ## processes whose tags are otherwise the same.  Can create a large number
  ## of series, use judiciously.
  # pid_tag = false

  ## Method to use when finding process IDs.  Can be one of 'pgrep', or
  ## 'native'.  The pgrep finder calls the pgrep executable in the PATH while
  ## the native finder performs the search directly in a manor dependent on the
  ## platform.  Default is 'pgrep'
  # pid_finder = "pgrep"
`

func (_ *SMProcstat) SampleConfig() string {
	return sampleConfig
}

func (_ *SMProcstat) Description() string {
	return "Monitor process cpu and memory usage"
}

func (p *SMProcstat) Gather(acc telegraf.Accumulator) error {
	if p.createPIDFinder == nil {
		switch p.PidFinder {
		case "native":
			p.createPIDFinder = NewNativeFinder
		case "pgrep":
			p.createPIDFinder = NewPgrep
		default:
			p.PidFinder = "pgrep"
			p.createPIDFinder = defaultPIDFinder
		}

	}
	if p.createProcess == nil {
		p.createProcess = defaultProcess
	}

	pids, tags, err := p.findPids(acc)
	if err != nil {
		fields := map[string]interface{}{
			"pid_count":   0,
			"running":     0,
			"result_code": 1,
		}
		tags := map[string]string{
			"pid_finder": p.PidFinder,
			"result":     "lookup_error",
		}
		acc.AddFields("smprocstat_lookup", fields, tags)
		return err
	}

	procs, err := p.updateProcesses(pids, tags, p.procs)
	if err != nil {
		acc.AddError(fmt.Errorf("E! Error: smprocstat getting process, exe: [%s] pidfile: [%s] pattern: [%s] user: [%s] %s",
			p.Exe, p.PidFile, p.Pattern, p.User, err.Error()))
	}
	p.procs = procs

	for _, proc := range p.procs {
		p.addMetric(proc, acc)
	}

	fields := map[string]interface{}{
		"pid_count":   len(pids),
		"running":     len(procs),
		"result_code": 0,
	}
	tags["pid_finder"] = p.PidFinder
	tags["result"] = "success"
	acc.AddFields("smprocstat_lookup", fields, tags)

	return nil
}

// Add metrics a single Process
func (p *SMProcstat) addMetric(proc Process, acc telegraf.Accumulator) {
	var prefix string
	if p.Prefix != "" {
		prefix = p.Prefix + "_"
	}

	fields := map[string]interface{}{}

	//If process_name tag is not already set, set to actual name
	if _, nameInTags := proc.Tags()["process_name"]; !nameInTags {
		name, err := proc.Name()
		if err == nil {
			proc.Tags()["process_name"] = name
		}
	}

	//If user tag is not already set, set to actual name
	if _, ok := proc.Tags()["user"]; !ok {
		user, err := proc.Username()
		if err == nil {
			proc.Tags()["user"] = user
		}
	}

	//If pid is not present as a tag, include it as a field.
	if _, pidInTags := proc.Tags()["pid"]; !pidInTags {
		fields["pid"] = int32(proc.PID())
	}

	//If cmd_line tag is true and it is not already set add cmdline as a tag
	if p.CmdLineTag {
		if _, ok := proc.Tags()["cmdline"]; !ok {
			Cmdline, err := proc.Cmdline()
			if err == nil {
				proc.Tags()["cmdline"] = Cmdline
			}
		}
	}

	status, err := proc.Status()
	if err == nil {
		fields[prefix+"status"] = status
	}

	exe, err := proc.Exe()
	if err == nil {
		fields[prefix+"exe"] = exe
	}

	cpu_time, err := proc.Times()
	if err == nil {
		fields[prefix+"cpu_time_user"] = cpu_time.User
		fields[prefix+"cpu_time_system"] = cpu_time.System
		fields[prefix+"cpu_time_idle"] = cpu_time.Idle
		fields[prefix+"cpu_time_nice"] = cpu_time.Nice
		fields[prefix+"cpu_time_iowait"] = cpu_time.Iowait
	}

	cpu_perc, err := proc.Percent(time.Duration(0))
	if err == nil {
		fields[prefix+"cpu_usage"] = cpu_perc
	}

	mem, err := proc.MemoryInfo()
	if err == nil {
		fields[prefix+"memory_rss"] = mem.RSS
		fields[prefix+"memory_vms"] = mem.VMS
		fields[prefix+"memory_swap"] = mem.Swap
	}

	mem_perc, err := proc.MemoryPercent()
	if err == nil {
		fields[prefix+"memory_usage"] = mem_perc
	}

	acc.AddFields("smprocstat", fields, proc.Tags())
}

// Update monitored Processes
func (p *SMProcstat) updateProcesses(pids []PID, tags map[string]string, prevInfo map[PID]Process) (map[PID]Process, error) {
	procs := make(map[PID]Process, len(prevInfo))

	for _, pid := range pids {
		info, ok := prevInfo[pid]
		if ok {
			// Assumption: if a process has no name, it probably does not exist
			if name, _ := info.Name(); name == "" {
				continue
			}
			procs[pid] = info
		} else {
			proc, err := p.createProcess(pid)
			if err != nil {
				// No problem; process may have ended after we found it
				continue
			}
			// Assumption: if a process has no name, it probably does not exist
			if name, _ := proc.Name(); name == "" {
				continue
			}
			procs[pid] = proc

			// Add initial tags
			for k, v := range tags {
				proc.Tags()[k] = v
			}

			// Add pid tag if needed
			if p.PidTag {
				proc.Tags()["pid"] = strconv.Itoa(int(pid))
			}
			if p.ProcessName != "" {
				proc.Tags()["process_name"] = p.ProcessName
			}
		}
	}
	return procs, nil
}

// Create and return PIDGatherer lazily
func (p *SMProcstat) getPIDFinder() (PIDFinder, error) {
	if p.finder == nil {
		f, err := p.createPIDFinder()
		if err != nil {
			return nil, err
		}
		p.finder = f
	}
	return p.finder, nil
}

// Get matching PIDs and their initial tags
func (p *SMProcstat) findPids(acc telegraf.Accumulator) ([]PID, map[string]string, error) {
	var pids []PID
	tags := make(map[string]string)
	var err error

	f, err := p.getPIDFinder()
	if err != nil {
		return nil, nil, err
	}

	if p.PidFile != "" {
		pids, err = f.PidFile(p.PidFile)
		tags = map[string]string{"pidfile": p.PidFile}
	} else if p.Exe != "" {
		pids, err = f.Pattern(p.Exe)
		tags = map[string]string{"exe": p.Exe}
	} else if p.Pattern != "" {
		pids, err = f.FullPattern(p.Pattern)
		tags = map[string]string{"pattern": p.Pattern}
	} else if p.User != "" {
		pids, err = f.Uid(p.User)
		tags = map[string]string{"user": p.User}
	} else if p.CGroup != "" {
		pids, err = p.cgroupPIDs()
		tags = map[string]string{"cgroup": p.CGroup}
	} else {
		err = fmt.Errorf("Either exe, pid_file, user, pattern, systemd_unit,or wcgroup must be specified")
	}

	return pids, tags, err
}

func (p *SMProcstat) cgroupPIDs() ([]PID, error) {
	var pids []PID

	procsPath := p.CGroup
	if procsPath[0] != '/' {
		procsPath = "/sys/fs/cgroup/" + procsPath
	}
	procsPath = filepath.Join(procsPath, "cgroup.procs")
	out, err := ioutil.ReadFile(procsPath)
	if err != nil {
		return nil, err
	}
	for _, pidBS := range bytes.Split(out, []byte{'\n'}) {
		if len(pidBS) == 0 {
			continue
		}
		pid, err := strconv.Atoi(string(pidBS))
		if err != nil {
			return nil, fmt.Errorf("invalid pid '%s'", pidBS)
		}
		pids = append(pids, PID(pid))
	}

	return pids, nil
}

func init() {
	inputs.Add("smprocstat", func() telegraf.Input {
		return &SMProcstat{}
	})
}
