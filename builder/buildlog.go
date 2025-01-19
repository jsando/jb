package builder

import (
	"fmt"
	"github.com/jsando/jb/project"
	"github.com/pterm/pterm"
	"time"
)

type buildLog struct {
	buildStartTime  time.Time
	moduleStartTime time.Time
	warnCount       int
	errorCount      int
}

type taskLog struct {
	buildLog  *buildLog
	startTime time.Time
	name      string
}

func (t *taskLog) Info(msg string) {
	pterm.Info.Println(msg)
}

func (t *taskLog) Warn(msg string) {
	t.buildLog.warnCount++
	pterm.Warning.Println(msg)
}

func (t *taskLog) Error(msg string) {
	t.buildLog.errorCount++
	pterm.Error.Println(msg)
}

func formatSeconds(t time.Time) string {
	return fmt.Sprintf("%.1fs", time.Since(t).Seconds())
}

func NewBuildLog() *buildLog {
	bl := &buildLog{}
	bl.BuildStart()
	return bl
}

func (b *buildLog) BuildStart() {
	b.buildStartTime = time.Now()
	fmt.Println("JB 1.0.0 - Build Started")
}

func (b *buildLog) BuildFinish() {
	totalTime := formatSeconds(b.buildStartTime)
	result := "completed"
	if b.errorCount > 0 {
		result = "FAILED"
	}
	msg := fmt.Sprintf("Build %s in %s (%d Warnings, %d Errors)\n", result, totalTime, b.warnCount, b.errorCount)
	if b.errorCount > 0 {
		pterm.Fatal.Println(msg)
	} else {
		pterm.Success.Println(msg)
	}
}

func (b *buildLog) ModuleStart(name string) {
	b.moduleStartTime = time.Now()
	fmt.Printf("  Module: %s\n", name)
}

func (b *buildLog) CheckError(task string, err error) bool {
	if err == nil {
		return false
	}
	b.errorCount++
	pterm.Fatal.Printf("ERROR %s: %s\n", task, err)
	//fmt.Printf("ERROR %s: %s\n", task, err)
	return true
}

func (b *buildLog) Failed() bool {
	return b.errorCount > 0
}

func (b *buildLog) TaskStart(name string) project.TaskLog {
	return &taskLog{
		buildLog:  b,
		startTime: time.Now(),
		name:      name,
	}
}

func (t *taskLog) Done(err error) bool {
	taskDuration := formatSeconds(t.startTime)
	if err != nil {
		t.buildLog.errorCount++
		pterm.Error.Printf("    ✖ %s FAILED (Time: %s)\n", t.name, taskDuration)
		pterm.Error.Printf("      └─ Cause: %s\n", err)
	} else {
		fmt.Printf("    ✔ %s (Time: %s)\n", t.name, taskDuration)
	}
	return err != nil
}
