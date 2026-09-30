//go:build windows

package plugins

import (
	"os/exec"
	"strconv"
	"sync"
	"syscall"
	"unsafe"
)

// ownGroup starts cmd in its own process group, suspended; stopping it kills
// the whole tree with taskkill /T while the leader still runs. startedGroup
// MUST follow Start: it puts the process in a Job Object before it runs a
// single instruction (so no child escapes it) and resumes it. reapGroup
// terminates the job, which also reaches children whose leader already
// exited; the job also dies with lazyagents.
func ownGroup(cmd *exec.Cmd) {
	const createSuspended = 0x00000004
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP | createSuspended}
	cmd.Cancel = func() error {
		if err := exec.Command("taskkill", "/T", "/F", "/PID", strconv.Itoa(cmd.Process.Pid)).Run(); err != nil {
			return cmd.Process.Kill()
		}
		return nil
	}
}

var (
	kernel32                = syscall.NewLazyDLL("kernel32.dll")
	procCreateJobObject     = kernel32.NewProc("CreateJobObjectW")
	procSetInformationJob   = kernel32.NewProc("SetInformationJobObject")
	procAssignProcessToJob  = kernel32.NewProc("AssignProcessToJobObject")
	procTerminateJobObject  = kernel32.NewProc("TerminateJobObject")
	procNtResumeProcess     = syscall.NewLazyDLL("ntdll.dll").NewProc("NtResumeProcess")
	jobs                    sync.Map                   // *exec.Cmd → syscall.Handle
	jobObjectExtendedLimits = 9                        // JobObjectExtendedLimitInformation
	jobLimitKillOnJobClose  = 0x2000                   // JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE
	processJobAccess        = 0x0100 | 0x0001 | 0x0800 // SET_QUOTA | TERMINATE | SUSPEND_RESUME
)

// jobLimits is JOBOBJECT_EXTENDED_LIMIT_INFORMATION; Go's field alignment
// matches the C layout on amd64 and arm64.
type jobLimits struct {
	PerProcessUserTimeLimit int64
	PerJobUserTimeLimit     int64
	LimitFlags              uint32
	MinimumWorkingSetSize   uintptr
	MaximumWorkingSetSize   uintptr
	ActiveProcessLimit      uint32
	Affinity                uintptr
	PriorityClass           uint32
	SchedulingClass         uint32
	IoCounters              [6]uint64
	ProcessMemoryLimit      uintptr
	JobMemoryLimit          uintptr
	PeakProcessMemoryUsed   uintptr
	PeakJobMemoryUsed       uintptr
}

// startedGroup puts the suspended cmd in a new kill-on-close Job Object and
// resumes it. The job is best effort (without it, taskkill /T at cancel time
// still stops the tree); the resume is not: a process left suspended hangs.
func startedGroup(cmd *exec.Cmd) {
	proc, err := syscall.OpenProcess(uint32(processJobAccess), false, uint32(cmd.Process.Pid))
	if err != nil {
		return // cannot resume either: Wait's deadline and Cancel still apply
	}
	defer syscall.CloseHandle(proc)
	defer procNtResumeProcess.Call(uintptr(proc))
	job, _, _ := procCreateJobObject.Call(0, 0)
	if job == 0 {
		return
	}
	limits := jobLimits{LimitFlags: uint32(jobLimitKillOnJobClose)}
	ok, _, _ := procSetInformationJob.Call(job, uintptr(jobObjectExtendedLimits), uintptr(unsafe.Pointer(&limits)), unsafe.Sizeof(limits))
	if ok != 0 {
		ok, _, _ = procAssignProcessToJob.Call(job, uintptr(proc))
	}
	if ok == 0 {
		_ = syscall.CloseHandle(syscall.Handle(job))
		return
	}
	jobs.Store(cmd, syscall.Handle(job))
}

// reapGroup ends the cmd's job: whatever it started is stopped too.
func reapGroup(cmd *exec.Cmd) {
	if v, ok := jobs.LoadAndDelete(cmd); ok {
		job := v.(syscall.Handle)
		_, _, _ = procTerminateJobObject.Call(uintptr(job), 1)
		_ = syscall.CloseHandle(job)
	}
}
