// Package process provides process management tools for agents.
package process

import (
	"context"
	"encoding/json"
	"os"
	"runtime"
	"strings"

	"github.com/felixgeelhaar/agent-go/domain/agent"
	"github.com/felixgeelhaar/agent-go/domain/pack"
	"github.com/felixgeelhaar/agent-go/domain/tool"
	"github.com/shirou/gopsutil/v3/process"
)

// Pack returns the process management tools pack.
func Pack() *pack.Pack {
	return pack.NewBuilder("process").
		WithDescription("Process management and monitoring tools").
		AddTools(
			listTool(),
			infoTool(),
			findByNameTool(),
			findByPIDTool(),
			currentTool(),
			parentTool(),
			childrenTool(),
			memoryUsageTool(),
			cpuUsageTool(),
			connectionsTool(),
			openFilesTool(),
			countTool(),
		).
		AllowAllInState(agent.StateExplore).
		AllowAllInState(agent.StateAct).
		Build()
}

func listTool() tool.Tool {
	return tool.NewBuilder("process_list").
		WithDescription("List all running processes").
		ReadOnly().
		Idempotent().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Limit int `json:"limit,omitempty"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			procs, err := process.ProcessesWithContext(ctx)
			if err != nil {
				return tool.Result{}, err
			}

			limit := params.Limit
			if limit <= 0 || limit > len(procs) {
				limit = len(procs)
			}

			var processes []map[string]any
			for i := 0; i < limit; i++ {
				p := procs[i]
				name, _ := p.NameWithContext(ctx)
				status, _ := p.StatusWithContext(ctx)

				processes = append(processes, map[string]any{
					"pid":    p.Pid,
					"name":   name,
					"status": status,
				})
			}

			result := map[string]any{
				"processes": processes,
				"total":     len(procs),
				"returned":  len(processes),
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func infoTool() tool.Tool {
	return tool.NewBuilder("process_info").
		WithDescription("Get detailed information about a process").
		ReadOnly().
		Idempotent().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				PID int32 `json:"pid"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			p, err := process.NewProcessWithContext(ctx, params.PID)
			if err != nil {
				return tool.Result{}, err
			}

			name, _ := p.NameWithContext(ctx)
			exe, _ := p.ExeWithContext(ctx)
			cmdline, _ := p.CmdlineWithContext(ctx)
			cwd, _ := p.CwdWithContext(ctx)
			username, _ := p.UsernameWithContext(ctx)
			createTime, _ := p.CreateTimeWithContext(ctx)
			status, _ := p.StatusWithContext(ctx)
			ppid, _ := p.PpidWithContext(ctx)
			numThreads, _ := p.NumThreadsWithContext(ctx)

			result := map[string]any{
				"pid":         params.PID,
				"name":        name,
				"exe":         exe,
				"cmdline":     cmdline,
				"cwd":         cwd,
				"username":    username,
				"create_time": createTime,
				"status":      status,
				"ppid":        ppid,
				"num_threads": numThreads,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func findByNameTool() tool.Tool {
	return tool.NewBuilder("process_find_by_name").
		WithDescription("Find processes by name").
		ReadOnly().
		Idempotent().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Name  string `json:"name"`
				Exact bool   `json:"exact,omitempty"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			procs, err := process.ProcessesWithContext(ctx)
			if err != nil {
				return tool.Result{}, err
			}

			var matches []map[string]any
			searchName := strings.ToLower(params.Name)

			for _, p := range procs {
				name, err := p.NameWithContext(ctx)
				if err != nil {
					continue
				}

				match := false
				if params.Exact {
					match = strings.ToLower(name) == searchName
				} else {
					match = strings.Contains(strings.ToLower(name), searchName)
				}

				if match {
					cmdline, _ := p.CmdlineWithContext(ctx)
					matches = append(matches, map[string]any{
						"pid":     p.Pid,
						"name":    name,
						"cmdline": cmdline,
					})
				}
			}

			result := map[string]any{
				"query":   params.Name,
				"matches": matches,
				"count":   len(matches),
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func findByPIDTool() tool.Tool {
	return tool.NewBuilder("process_find_by_pid").
		WithDescription("Check if a process with given PID exists").
		ReadOnly().
		Idempotent().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				PID int32 `json:"pid"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			exists, err := process.PidExistsWithContext(ctx, params.PID)
			if err != nil {
				return tool.Result{}, err
			}

			result := map[string]any{
				"pid":    params.PID,
				"exists": exists,
			}

			if exists {
				p, _ := process.NewProcessWithContext(ctx, params.PID)
				if p != nil {
					name, _ := p.NameWithContext(ctx)
					result["name"] = name
				}
			}

			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func currentTool() tool.Tool {
	return tool.NewBuilder("process_current").
		WithDescription("Get information about the current process").
		ReadOnly().
		Idempotent().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			pid := int32(os.Getpid()) // #nosec G115 -- PID values are bounded by OS limits (max 2^22 on Linux)
			p, err := process.NewProcessWithContext(ctx, pid)
			if err != nil {
				return tool.Result{}, err
			}

			name, _ := p.NameWithContext(ctx)
			exe, _ := p.ExeWithContext(ctx)
			cwd, _ := p.CwdWithContext(ctx)
			username, _ := p.UsernameWithContext(ctx)
			memInfo, _ := p.MemoryInfoWithContext(ctx)
			cpuPercent, _ := p.CPUPercentWithContext(ctx)

			result := map[string]any{
				"pid":         pid,
				"name":        name,
				"exe":         exe,
				"cwd":         cwd,
				"username":    username,
				"cpu_percent": cpuPercent,
				"go_version":  runtime.Version(),
				"go_routines": runtime.NumGoroutine(),
			}

			if memInfo != nil {
				result["memory_rss"] = memInfo.RSS
				result["memory_vms"] = memInfo.VMS
			}

			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func parentTool() tool.Tool {
	return tool.NewBuilder("process_parent").
		WithDescription("Get parent process information").
		ReadOnly().
		Idempotent().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				PID int32 `json:"pid,omitempty"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			pid := params.PID
			if pid == 0 {
				pid = int32(os.Getpid()) // #nosec G115 -- PID values are bounded by OS limits (max 2^22 on Linux)
			}

			p, err := process.NewProcessWithContext(ctx, pid)
			if err != nil {
				return tool.Result{}, err
			}

			parent, err := p.ParentWithContext(ctx)
			if err != nil {
				return tool.Result{}, err
			}

			name, _ := parent.NameWithContext(ctx)
			cmdline, _ := parent.CmdlineWithContext(ctx)

			result := map[string]any{
				"child_pid":  pid,
				"parent_pid": parent.Pid,
				"name":       name,
				"cmdline":    cmdline,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func childrenTool() tool.Tool {
	return tool.NewBuilder("process_children").
		WithDescription("Get child processes").
		ReadOnly().
		Idempotent().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				PID int32 `json:"pid,omitempty"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			pid := params.PID
			if pid == 0 {
				pid = int32(os.Getpid()) // #nosec G115 -- PID values are bounded by OS limits (max 2^22 on Linux)
			}

			p, err := process.NewProcessWithContext(ctx, pid)
			if err != nil {
				return tool.Result{}, err
			}

			children, err := p.ChildrenWithContext(ctx)
			if err != nil {
				return tool.Result{}, err
			}

			var childList []map[string]any
			for _, child := range children {
				name, _ := child.NameWithContext(ctx)
				childList = append(childList, map[string]any{
					"pid":  child.Pid,
					"name": name,
				})
			}

			result := map[string]any{
				"parent_pid": pid,
				"children":   childList,
				"count":      len(childList),
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func memoryUsageTool() tool.Tool {
	return tool.NewBuilder("process_memory_usage").
		WithDescription("Get memory usage of a process").
		ReadOnly().
		Idempotent().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				PID int32 `json:"pid,omitempty"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			pid := params.PID
			if pid == 0 {
				pid = int32(os.Getpid()) // #nosec G115 -- PID values are bounded by OS limits (max 2^22 on Linux)
			}

			p, err := process.NewProcessWithContext(ctx, pid)
			if err != nil {
				return tool.Result{}, err
			}

			memInfo, err := p.MemoryInfoWithContext(ctx)
			if err != nil {
				return tool.Result{}, err
			}

			memPercent, _ := p.MemoryPercentWithContext(ctx)

			result := map[string]any{
				"pid":     pid,
				"rss":     memInfo.RSS,
				"vms":     memInfo.VMS,
				"rss_mb":  float64(memInfo.RSS) / 1024 / 1024,
				"vms_mb":  float64(memInfo.VMS) / 1024 / 1024,
				"percent": memPercent,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func cpuUsageTool() tool.Tool {
	return tool.NewBuilder("process_cpu_usage").
		WithDescription("Get CPU usage of a process").
		ReadOnly().
		Idempotent().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				PID int32 `json:"pid,omitempty"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			pid := params.PID
			if pid == 0 {
				pid = int32(os.Getpid()) // #nosec G115 -- PID values are bounded by OS limits (max 2^22 on Linux)
			}

			p, err := process.NewProcessWithContext(ctx, pid)
			if err != nil {
				return tool.Result{}, err
			}

			cpuPercent, err := p.CPUPercentWithContext(ctx)
			if err != nil {
				return tool.Result{}, err
			}

			times, _ := p.TimesWithContext(ctx)

			result := map[string]any{
				"pid":     pid,
				"percent": cpuPercent,
			}

			if times != nil {
				result["user_time"] = times.User
				result["system_time"] = times.System
			}

			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func connectionsTool() tool.Tool {
	return tool.NewBuilder("process_connections").
		WithDescription("Get network connections of a process").
		ReadOnly().
		Idempotent().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				PID int32 `json:"pid,omitempty"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			pid := params.PID
			if pid == 0 {
				pid = int32(os.Getpid()) // #nosec G115 -- PID values are bounded by OS limits (max 2^22 on Linux)
			}

			p, err := process.NewProcessWithContext(ctx, pid)
			if err != nil {
				return tool.Result{}, err
			}

			conns, err := p.ConnectionsWithContext(ctx)
			if err != nil {
				return tool.Result{}, err
			}

			var connections []map[string]any
			for _, conn := range conns {
				connections = append(connections, map[string]any{
					"type":        conn.Type,
					"local_addr":  conn.Laddr.IP,
					"local_port":  conn.Laddr.Port,
					"remote_addr": conn.Raddr.IP,
					"remote_port": conn.Raddr.Port,
					"status":      conn.Status,
				})
			}

			result := map[string]any{
				"pid":         pid,
				"connections": connections,
				"count":       len(connections),
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func openFilesTool() tool.Tool {
	return tool.NewBuilder("process_open_files").
		WithDescription("Get open files of a process").
		ReadOnly().
		Idempotent().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				PID int32 `json:"pid,omitempty"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			pid := params.PID
			if pid == 0 {
				pid = int32(os.Getpid()) // #nosec G115 -- PID values are bounded by OS limits (max 2^22 on Linux)
			}

			p, err := process.NewProcessWithContext(ctx, pid)
			if err != nil {
				return tool.Result{}, err
			}

			files, err := p.OpenFilesWithContext(ctx)
			if err != nil {
				return tool.Result{}, err
			}

			var openFiles []map[string]any
			for _, f := range files {
				openFiles = append(openFiles, map[string]any{
					"fd":   f.Fd,
					"path": f.Path,
				})
			}

			result := map[string]any{
				"pid":   pid,
				"files": openFiles,
				"count": len(openFiles),
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func countTool() tool.Tool {
	return tool.NewBuilder("process_count").
		WithDescription("Count running processes").
		ReadOnly().
		Idempotent().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			pids, err := process.PidsWithContext(ctx)
			if err != nil {
				return tool.Result{}, err
			}

			result := map[string]any{
				"count": len(pids),
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}
